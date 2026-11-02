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

// AuthorizationConfiguration describes what an authenticated user can see.
type AuthorizationConfiguration struct {
	// Rules are evaluated in order. The first one calling Allow(), Deny() or
	// SetFilter() decides. When none decides, the user is denied. No rule means
	// any user is allowed.
	Rules []AuthorizationRule
}

// AuthorizationRule is a compiled authorization rule.
type AuthorizationRule struct {
	program *vm.Program
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
	taken  bool
	denied bool
	// filter is the bound filter. Empty means no restriction.
	filter string
}

// decide evaluates the rules for the provided user.
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

// validate checks the filters of the rule against the schema, with an empty
// string for each placeholder. As the filter applies to every flows table, it
// cannot use a column only present in the main one.
func (rule AuthorizationRule) validate(sch *schema.Component, database string) error {
	for _, template := range rule.filterTemplates() {
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
		case []any:
			quoted := make([]string, len(arg))
			for i, value := range arg {
				s, ok := value.(string)
				if !ok {
					return "", fmt.Errorf("cannot bind argument %d of filter %q: unsupported type %T in list",
						idx+1, template, value)
				}
				quoted[i] = quoteFilterString(s)
			}
			result.WriteString(strings.Join(quoted, ", "))
		default:
			return "", fmt.Errorf("cannot bind argument %d of filter %q: unsupported type %T",
				idx+1, template, args[idx])
		}
	}
	return result.String(), nil
}

// setFilterValidator checks that the filter of each SetFilter() call is a
// string literal with as many placeholders as arguments.
type setFilterValidator struct {
	err error
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
	}
}

// filterTemplates returns the filters used by the SetFilter() calls of the
// rule.
func (rule AuthorizationRule) filterTemplates() []string {
	var collector setFilterCollector
	node := rule.program.Node()
	ast.Walk(&node, &collector)
	return collector
}

// setFilterCollector collects the filters of the SetFilter() calls in a
// compiled rule. There, the filter is the second argument, after the decision.
type setFilterCollector []string

func (templates *setFilterCollector) Visit(node *ast.Node) {
	call, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	if callee, ok := call.Callee.(*ast.IdentifierNode); !ok || callee.Value != "SetFilter" {
		return
	}
	*templates = append(*templates, call.Arguments[1].(*ast.StringNode).Value)
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
// setting for the flows tables of the request context. The keys include the
// database, as ClickHouse silently ignores a key matching no table.
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

// authorization is a middleware applying the authorization rules. The filter
// of a restricted user is added to each ClickHouse query and to the cache keys.
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
