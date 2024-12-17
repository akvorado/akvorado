// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"akvorado/common/schema"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/vm"
)

// Global cache for regular expressions. No boundary.
var (
	regexCacheLock sync.RWMutex
	regexCache     = make(map[string]*regexp.Regexp)
)

type classifierContextKey string

// ExporterClassifierRule defines a classification rule for a exporter.
type ExporterClassifierRule struct {
	program *vm.Program
}

// exporterInfo contains the information we want to expose about a exporter.
type exporterInfo struct {
	IP   string
	Name string
}

// exporterClassification contains the information about an exporter classification
type exporterClassification struct {
	Group  string
	Role   string
	Site   string
	Region string
	Tenant string
	Reject bool
}

// exporterClassifierEnvironment defines the environment used by the exporter classifier
type exporterClassifierEnvironment struct {
	Exporter              exporterInfo
	CurrentClassification *exporterClassification
}

// exec executes the exporter classifier with the provided exporter.
func (scr *ExporterClassifierRule) exec(si exporterInfo, ec *exporterClassification) error {
	env := exporterClassifierEnvironment{
		Exporter:              si,
		CurrentClassification: ec,
	}
	if _, err := expr.Run(scr.program, env); err != nil {
		return fmt.Errorf("unable to execute classifier %q: %w", scr, err)
	}
	return nil
}

// UnmarshalText compiles a classification rule for a exporter.
func (scr *ExporterClassifierRule) UnmarshalText(text []byte) error {
	regexValidator := regexValidator{}
	withClassificationPatcher := withClassificationPatcher{}
	options := []expr.Option{
		expr.Env(exporterClassifierEnvironment{}),
		expr.WithContext("Context"),
		expr.AsBool(),
		expr.Patch(&withClassificationPatcher),
		expr.Patch(&regexValidator),
		expr.Function(
			"Format",
			func(params ...any) (any, error) {
				return fmt.Sprintf(params[0].(string), params[1:]...), nil
			},
			new(func(string, ...any) string),
		),
		expr.Function(
			"Reject",
			func(params ...any) (any, error) {
				ec := params[0].(*exporterClassification)
				ec.Reject = true
				return false, nil
			},
			new(func(*exporterClassification) bool),
		),
	}
	options = addExporterClassifyStringFunction(options,
		"Classify", func(ec *exporterClassification) *string { return &ec.Group })
	options = addExporterClassifyStringFunction(options,
		"ClassifyGroup", func(ec *exporterClassification) *string { return &ec.Group })
	options = addExporterClassifyStringFunction(options,
		"ClassifyRole", func(ec *exporterClassification) *string { return &ec.Role })
	options = addExporterClassifyStringFunction(options,
		"ClassifySite", func(ec *exporterClassification) *string { return &ec.Site })
	options = addExporterClassifyStringFunction(options,
		"ClassifyRegion", func(ec *exporterClassification) *string { return &ec.Region })
	options = addExporterClassifyStringFunction(options,
		"ClassifyTenant", func(ec *exporterClassification) *string { return &ec.Tenant })

	program, err := expr.Compile(string(text), options...)
	if err != nil {
		return fmt.Errorf("cannot compile exporter classifier rule %q: %w", string(text), err)
	}
	if len(regexValidator.invalidRegexes) > 0 {
		return fmt.Errorf("invalid regular expression %q", regexValidator.invalidRegexes[0])
	}
	scr.program = program
	return nil
}

// String turns a exporter classifier rule into a string
func (scr ExporterClassifierRule) String() string {
	return scr.program.Source().String()
}

// MarshalText turns a exporter classifier rule into a string
func (scr ExporterClassifierRule) MarshalText() ([]byte, error) {
	return []byte(scr.String()), nil
}

// InterfaceClassifierRule defines a classification rule for an interface.
type InterfaceClassifierRule struct {
	program *vm.Program
}

// interfaceInfo contains the information we want to expose about an interface.
type interfaceInfo struct {
	Index       uint32
	Name        string
	Description string
	Speed       uint32
	VLAN        uint16
}

// interfaceClassification contains the information about an interface classification
type interfaceClassification struct {
	Connectivity string
	Provider     string
	Boundary     schema.InterfaceBoundary
	Reject       bool
	Name         string
	Description  string
}

// interfaceClassifierEnvironment defines the environment used by the interface classifier
type interfaceClassifierEnvironment struct {
	Exporter              exporterInfo
	Interface             interfaceInfo
	CurrentClassification *interfaceClassification
}

// exec executes the exporter classifier with the provided interface.
func (scr *InterfaceClassifierRule) exec(si exporterInfo, ii interfaceInfo, ic *interfaceClassification) error {
	env := interfaceClassifierEnvironment{
		Exporter:              si,
		Interface:             ii,
		CurrentClassification: ic,
	}
	if _, err := expr.Run(scr.program, env); err != nil {
		return fmt.Errorf("unable to execute classifier %q: %w", scr, err)
	}
	return nil
}

// UnmarshalText compiles a classification rule for an interface.
func (scr *InterfaceClassifierRule) UnmarshalText(text []byte) error {
	regexValidator := regexValidator{}
	withClassificationPatcher := withClassificationPatcher{}
	options := []expr.Option{
		expr.Env(interfaceClassifierEnvironment{}),
		expr.WithContext("Context"),
		expr.AsBool(),
		expr.Patch(&withClassificationPatcher),
		expr.Patch(&regexValidator),
		expr.Function(
			"Format",
			func(params ...any) (any, error) {
				return fmt.Sprintf(params[0].(string), params[1:]...), nil
			},
			new(func(string, ...any) string),
		),
		expr.Function(
			"Reject",
			func(params ...any) (any, error) {
				ic := params[0].(*interfaceClassification)
				ic.Reject = true
				return false, nil
			},
			new(func(*interfaceClassification) bool),
		),
		expr.Function(
			"ClassifyExternal",
			func(params ...any) (any, error) {
				ic := params[0].(*interfaceClassification)
				if ic.Boundary == schema.InterfaceBoundaryUndefined {
					ic.Boundary = schema.InterfaceBoundaryExternal
				}
				return true, nil
			},
			new(func(*interfaceClassification) bool),
		),
		expr.Function(
			"ClassifyInternal",
			func(params ...any) (any, error) {
				ic := params[0].(*interfaceClassification)
				if ic.Boundary == schema.InterfaceBoundaryUndefined {
					ic.Boundary = schema.InterfaceBoundaryInternal
				}
				return true, nil
			},
			new(func(*interfaceClassification) bool),
		),
		expr.Function(
			"SetName",
			func(params ...any) (any, error) {
				ic := params[0].(*interfaceClassification)
				if ic.Name == "" {
					ic.Name = params[1].(string)
				}
				return true, nil
			},
			new(func(*interfaceClassification, string) bool),
		),
		expr.Function(
			"SetDescription",
			func(params ...any) (any, error) {
				ic := params[0].(*interfaceClassification)
				if ic.Description == "" {
					ic.Description = params[1].(string)
				}
				return true, nil
			},
			new(func(*interfaceClassification, string) bool),
		),
	}
	options = addInterfaceClassifyStringFunction(options,
		"ClassifyProvider", func(ic *interfaceClassification) *string { return &ic.Provider })
	options = addInterfaceClassifyStringFunction(options,
		"ClassifyConnectivity", func(ic *interfaceClassification) *string { return &ic.Connectivity })
	program, err := expr.Compile(string(text), options...)
	if err != nil {
		return fmt.Errorf("cannot compile interface classifier rule %q: %w", string(text), err)
	}
	if len(regexValidator.invalidRegexes) > 0 {
		return fmt.Errorf("invalid regular expression %q", regexValidator.invalidRegexes[0])
	}
	scr.program = program
	return nil
}

// String turns a interface classifier rule into a string
func (scr InterfaceClassifierRule) String() string {
	return scr.program.Source().String()
}

// MarshalText turns a interface classifier rule into a string
func (scr InterfaceClassifierRule) MarshalText() ([]byte, error) {
	return []byte(scr.String()), nil
}

var normalizeRegex = regexp.MustCompile("[^a-z0-9.+-]+")

// Normalize a string by putting it lowercase and only keeping safe characters
func normalize(str string) string {
	return normalizeRegex.ReplaceAllString(strings.ToLower(str), "")
}

// classifyString is an helper to classify from string to string
func classifyString(input string, output *string) (bool, error) {
	if *output == "" {
		*output = normalize(input)
	}
	return true, nil
}

// classifyStringWithRegex is an helper to classify from string and regex to string
func classifyStringWithRegex(input, regex, template string, output *string) (bool, error) {
	if *output == "" {
		// We may have several readers trying to compile the
		// regex the first time. It's not really important.
		regexCacheLock.RLock()
		compiledRegex, ok := regexCache[regex]
		regexCacheLock.RUnlock()
		if !ok {
			var err error
			compiledRegex, err = regexp.Compile(regex)
			if err != nil {
				return false, fmt.Errorf("cannot compile regex %q: %w", regex, err)
			}
			regexCacheLock.Lock()
			regexCache[regex] = compiledRegex
			regexCacheLock.Unlock()
		}

		result := []byte{}
		indexes := compiledRegex.FindSubmatchIndex([]byte(input))
		if indexes == nil {
			return false, nil
		}
		result = compiledRegex.ExpandString(result, template, input, indexes)
		*output = normalize(string(result))
	}
	return true, nil
}

// addExporterClassifyStringFunction adds to the list of compile options two
// functions for classifying an aspect of an exporter.
func addExporterClassifyStringFunction(options []expr.Option, name string, fn func(*exporterClassification) *string) []expr.Option {
	options = append(options,
		expr.Function(
			name,
			func(params ...any) (any, error) {
				ec := params[0].(*exporterClassification)
				return classifyString(params[1].(string), fn(ec))
			},
			new(func(*exporterClassification, string) bool),
		),
		expr.Function(
			fmt.Sprintf("%sRegex", name),
			func(params ...any) (any, error) {
				ec := params[0].(*exporterClassification)
				return classifyStringWithRegex(params[1].(string), params[2].(string), params[3].(string), fn(ec))
			},
			new(func(*exporterClassification, string, string, string) (bool, error)),
		),
	)
	return options
}

// addInterfaceClassifyStringFunction adds to the list of compile options two
// functions for classifying an aspect of an interface.
func addInterfaceClassifyStringFunction(options []expr.Option, name string, fn func(*interfaceClassification) *string) []expr.Option {
	options = append(options,
		expr.Function(
			name,
			func(params ...any) (any, error) {
				ic := params[0].(*interfaceClassification)
				return classifyString(params[1].(string), fn(ic))
			},
			new(func(*interfaceClassification, string) bool),
		),
		expr.Function(
			fmt.Sprintf("%sRegex", name),
			func(params ...any) (any, error) {
				ic := params[0].(*interfaceClassification)
				return classifyStringWithRegex(params[1].(string), params[2].(string), params[3].(string), fn(ic))
			},
			new(func(*interfaceClassification, string, string, string) (bool, error)),
		),
	)
	return options
}

// regexValidator is a patch validate regular expressions at compile-time
type regexValidator struct {
	invalidRegexes []string
}

func (r *regexValidator) Visit(node *ast.Node) {
	n, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	identifier, ok := n.Callee.(*ast.IdentifierNode)
	if !ok {
		return
	}
	if !strings.HasSuffix(identifier.Value, "Regex") || len(n.Arguments) != 4 {
		return
	}
	str, ok := n.Arguments[2].(*ast.StringNode)
	if !ok {
		return
	}
	if _, err := regexp.Compile(str.Value); err != nil {
		r.invalidRegexes = append(r.invalidRegexes, str.Value)
	}
}

// withClassificationPatcher is a patch to add the current classification as the
// first argument when the function called expects one.
type withClassificationPatcher struct{}

func (a *withClassificationPatcher) Visit(node *ast.Node) {
	switch call := (*node).(type) {
	case *ast.CallNode:
		fn := call.Callee.Type()
		if fn == nil || fn.Kind() != reflect.Func || fn.NumIn() < 1 {
			return
		}
		if fn.In(0).String() == "*core.exporterClassification" || fn.In(0).String() == "*core.interfaceClassification" {
			ast.Patch(node, &ast.CallNode{
				Callee: call.Callee,
				Arguments: append([]ast.Node{
					&ast.IdentifierNode{Value: "CurrentClassification"},
				}, call.Arguments...),
			})
		}
	}
}
