// SPDX-FileCopyrightText: 2026 Gabriel ROUSSEAU (bolemo)
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/vm"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/authentication"
	"akvorado/console/filter"
)

// AuthorizationConfiguration describes what an authenticated user is allowed
// to see.
type AuthorizationConfiguration struct {
	// Rules decide what an authenticated user can see. They are evaluated in
	// order and the first one calling Allow(), Deny() or SetFilter() decides.
	// When rules are present and none decides, the user is denied. Without any
	// rule, every user sees every flow.
	Rules []AuthorizationRule
}

// AuthorizationRule is a rule deciding what an authenticated user can see.
type AuthorizationRule struct {
	program *vm.Program
	// filters are the filter templates found in the SetFilter() calls of the
	// rule. They can only be checked against the schema once it is known.
	filters []string
}

// authorizationEnvironment is what a rule has access to.
type authorizationEnvironment struct {
	Login           string
	Name            string
	Email           string
	Groups          []string
	CurrentDecision *authorizationDecision
}

// authorizationDecision is the outcome of the evaluation of the rules.
type authorizationDecision struct {
	// taken tells if a rule has decided.
	taken bool
	// denied tells if the user is denied access.
	denied bool
	// filter is the filter restricting what the user sees, in the console
	// filter language, with placeholders bound. Empty means no restriction.
	filter string
}

// decide evaluates the rules for the provided user. The returned decision is
// not taken when no rule decided.
func (config AuthorizationConfiguration) decide(user authentication.UserInformation) (authorizationDecision, error) {
	decision := authorizationDecision{}
	env := authorizationEnvironment{
		Login:           user.Login,
		Name:            user.Name,
		Email:           user.Email,
		Groups:          user.Groups,
		CurrentDecision: &decision,
	}
	for _, rule := range config.Rules {
		if _, err := expr.Run(rule.program, env); err != nil {
			return decision, fmt.Errorf("unable to execute authorization rule %q: %w", rule, err)
		}
		if decision.taken {
			break
		}
	}
	return decision, nil
}

// UnmarshalText compiles an authorization rule.
func (rule *AuthorizationRule) UnmarshalText(text []byte) error {
	// The validator has to run before the decision patcher: once patched, the
	// calls carry the decision as their first argument.
	validator := setFilterValidator{}
	options := []expr.Option{
		expr.Env(authorizationEnvironment{}),
		expr.AsBool(),
		expr.Patch(&validator),
		expr.Patch(&withDecisionPatcher{}),
		expr.Function(
			"Allow",
			func(params ...any) (any, error) {
				decision := params[0].(*authorizationDecision)
				decision.taken = true
				return true, nil
			},
			new(func(*authorizationDecision) bool),
		),
		expr.Function(
			"Deny",
			func(params ...any) (any, error) {
				decision := params[0].(*authorizationDecision)
				decision.taken = true
				decision.denied = true
				return true, nil
			},
			new(func(*authorizationDecision) bool),
		),
		expr.Function(
			"SetFilter",
			func(params ...any) (any, error) {
				decision := params[0].(*authorizationDecision)
				bound, err := bindFilter(params[1].(string), params[2:])
				if err != nil {
					return false, err
				}
				decision.taken = true
				decision.filter = bound
				return true, nil
			},
			new(func(*authorizationDecision, string, ...any) bool),
		),
	}
	program, err := expr.Compile(string(text), options...)
	if err != nil {
		return fmt.Errorf("cannot compile authorization rule %q: %w", string(text), err)
	}
	if validator.err != nil {
		return fmt.Errorf("cannot compile authorization rule %q: %w", string(text), validator.err)
	}
	rule.program = program
	rule.filters = validator.filters
	return nil
}

// String turns an authorization rule into a string.
func (rule AuthorizationRule) String() string {
	return rule.program.Source().String()
}

// MarshalText turns an authorization rule into a string.
func (rule AuthorizationRule) MarshalText() ([]byte, error) {
	return []byte(rule.String()), nil
}

// validate checks the filters used by the rule against the schema. A filter
// with a placeholder is checked with an empty string in its place: the
// placeholders are bound to string literals, so this is what the parser will
// get. The filter is sent along queries on every flows table, so it cannot
// use a column only present in the main one.
func (rule AuthorizationRule) validate(sch *schema.Component, database string) error {
	for _, template := range rule.filters {
		args := make([]any, placeholders(template))
		for idx := range args {
			args[idx] = ""
		}
		bound, err := bindFilter(template, args)
		if err != nil {
			return err
		}
		meta := filter.Meta{Schema: sch, Database: database}
		if _, err := filter.Parse("", []byte(bound), filter.GlobalStore("meta", &meta)); err != nil {
			return fmt.Errorf("cannot parse filter %q: %s", template, filter.HumanError(err))
		}
		if meta.MainTableRequired {
			return fmt.Errorf("filter %q uses a column only available in the main flows table", template)
		}
	}
	return nil
}

// splitFilterTemplate splits a filter template around its placeholders: a `?'
// outside of a string literal. The result has one more element than there are
// placeholders.
func splitFilterTemplate(template string) []string {
	pieces := []string{}
	var quote byte
	start := 0
	for idx := 0; idx < len(template); idx++ {
		switch ch := template[idx]; {
		case quote != 0 && ch == '\\':
			// Skip the escaped character.
			idx++
		case quote != 0 && ch == quote:
			quote = 0
		case quote == 0 && (ch == '"' || ch == '\''):
			quote = ch
		case quote == 0 && ch == '?':
			pieces = append(pieces, template[start:idx])
			start = idx + 1
		}
	}
	return append(pieces, template[start:])
}

// placeholders counts the placeholders of a filter template.
func placeholders(template string) int {
	return len(splitFilterTemplate(template)) - 1
}

// filterStringEscaper escapes a value to put it inside a string literal of the
// filter language.
var filterStringEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// quoteFilterString turns a value into a string literal of the filter language.
func quoteFilterString(value string) string {
	return "'" + filterStringEscaper.Replace(value) + "'"
}

// bindFilter replaces each placeholder of a filter template with the matching
// argument, quoted as a string literal of the filter language. A list of
// strings becomes a comma-separated list of literals, to be used with `IN'.
// Nothing coming from the user can reach the filter unquoted.
func bindFilter(template string, args []any) (string, error) {
	pieces := splitFilterTemplate(template)
	if len(pieces)-1 != len(args) {
		return "", fmt.Errorf("filter %q has %d placeholders but %d arguments were provided",
			template, len(pieces)-1, len(args))
	}
	var result strings.Builder
	for idx, piece := range pieces {
		result.WriteString(piece)
		if idx == len(args) {
			break
		}
		switch arg := args[idx].(type) {
		case string:
			result.WriteString(quoteFilterString(arg))
		case []string:
			quoted := make([]string, len(arg))
			for i, value := range arg {
				quoted[i] = quoteFilterString(value)
			}
			result.WriteString(strings.Join(quoted, ", "))
		default:
			return "", fmt.Errorf("cannot bind argument %d of filter %q: unsupported type %T",
				idx+1, template, args[idx])
		}
	}
	return result.String(), nil
}

// setFilterValidator is a patch checking the SetFilter() calls at compile
// time: the filter has to be a string literal, with as many placeholders as
// arguments. It collects the filters, for a later check against the schema.
type setFilterValidator struct {
	filters []string
	err     error
}

func (v *setFilterValidator) Visit(node *ast.Node) {
	call, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	callee, ok := call.Callee.(*ast.IdentifierNode)
	if !ok || callee.Value != "SetFilter" || v.err != nil {
		return
	}
	if len(call.Arguments) == 0 {
		// The type check reports it once the patches are applied.
		return
	}
	template, ok := call.Arguments[0].(*ast.StringNode)
	if !ok {
		v.err = errors.New("the filter of SetFilter() must be a string literal")
		return
	}
	if got, expected := len(call.Arguments)-1, placeholders(template.Value); got != expected {
		v.err = fmt.Errorf("filter %q has %d placeholders but %d arguments were provided",
			template.Value, expected, got)
		return
	}
	v.filters = append(v.filters, template.Value)
}

// withDecisionPatcher is a patch adding the current decision as the first
// argument of the functions expecting one.
type withDecisionPatcher struct{}

var decisionType = reflect.TypeFor[*authorizationDecision]()

func (withDecisionPatcher) Visit(node *ast.Node) {
	call, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	fn := call.Callee.Type()
	if fn == nil || fn.Kind() != reflect.Func || fn.NumIn() < 1 || fn.In(0) != decisionType {
		return
	}
	ast.Patch(node, &ast.CallNode{
		Callee: call.Callee,
		Arguments: append([]ast.Node{
			&ast.IdentifierNode{Value: "CurrentDecision"},
		}, call.Arguments...),
	})
}

// renderFilter turns a bound filter into a ClickHouse expression.
func (c *Component) renderFilter(bound string) (string, error) {
	meta := filter.Meta{Schema: c.d.Schema, Database: c.d.ClickHouseDB.DatabaseName()}
	got, err := filter.Parse("", []byte(bound), filter.GlobalStore("meta", &meta))
	if err != nil {
		return "", fmt.Errorf("cannot parse filter %q: %s", bound, filter.HumanError(err))
	}
	return got.(sb.Expr).String(), nil
}

// additionalTableFilters builds the value of the `additional_table_filters'
// ClickHouse setting applying the provided expression to every flows table. The
// keys are qualified with the database: ClickHouse resolves a bare one against
// the current database and silently ignores a key matching no table. The
// tables come from the request context, so they are the same ones the handlers
// pick from.
func (c *Component) additionalTableFilters(ctx context.Context, expression string) string {
	database := c.d.ClickHouseDB.DatabaseName()
	flowsTables := flowsTablesFromContext(ctx)
	entries := make([]string, 0, len(flowsTables))
	for _, table := range flowsTables {
		entries = append(entries, fmt.Sprintf("%s: %s",
			sb.String(database+"."+table.Name), sb.String(expression)))
	}
	return "{" + strings.Join(entries, ", ") + "}"
}

// authorization is a middleware applying the authorization rules to the
// authenticated user. A denied user gets a 403. A restricted user gets the
// filter attached to every ClickHouse query made while serving the request,
// through the `additional_table_filters' setting. The cache is keyed on the
// rendered filter: users whose rules resolve to the same filter share the
// cached responses, users with another filter never do.
//
// It does nothing when there is no rule.
func (c *Component) authorization() httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		if len(c.config.Authorization.Rules) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			user := authentication.UserFromContext(req.Context())
			decision, err := c.config.Authorization.decide(user)
			if err != nil {
				c.r.Err(err).Str("login", user.Login).Msg("cannot evaluate authorization rules")
				httpserver.WriteJSON(w, http.StatusInternalServerError,
					helpers.M{"message": "Cannot evaluate authorization rules."})
				return
			}
			if !decision.taken || decision.denied {
				httpserver.WriteJSON(w, http.StatusForbidden,
					helpers.M{"message": "Access denied."})
				return
			}
			if decision.filter == "" {
				next.ServeHTTP(w, req)
				return
			}
			expression, err := c.renderFilter(decision.filter)
			if err != nil {
				c.r.Err(err).Str("login", user.Login).Msg("cannot render authorization filter")
				httpserver.WriteJSON(w, http.StatusInternalServerError,
					helpers.M{"message": "Cannot apply authorization rules."})
				return
			}
			ctx := clickhouse.Context(req.Context(), clickhouse.WithSettings(clickhouse.Settings{
				"additional_table_filters": c.additionalTableFilters(req.Context(), expression),
			}))
			ctx = httpserver.WithCacheKeyDiscriminator(ctx, expression)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}
