// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
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

type (
	classifyStringFunc      = func(string) bool
	classifyStringRegexFunc = func(string, string, string) (bool, error)
)

// exporterClassifierEnvironment defines the environment used by the exporter classifier
type exporterClassifierEnvironment struct {
	Format              func(string, ...any) string
	Exporter            exporterInfo
	Classify            classifyStringFunc
	ClassifyRegex       classifyStringRegexFunc
	ClassifyGroup       classifyStringFunc
	ClassifyGroupRegex  classifyStringRegexFunc
	ClassifyRole        classifyStringFunc
	ClassifyRoleRegex   classifyStringRegexFunc
	ClassifySite        classifyStringFunc
	ClassifySiteRegex   classifyStringRegexFunc
	ClassifyRegion      classifyStringFunc
	ClassifyRegionRegex classifyStringRegexFunc
	ClassifyTenant      classifyStringFunc
	ClassifyTenantRegex classifyStringRegexFunc
	Reject              func() bool
}

func format(format string, a ...any) string {
	return fmt.Sprintf(format, a...)
}

// exec executes the exporter classifier with the provided exporter.
func (scr *ExporterClassifierRule) exec(si exporterInfo, ec *exporterClassification) error {
	classifyGroup := classifyString(&ec.Group)
	classifyRole := classifyString(&ec.Role)
	classifySite := classifyString(&ec.Site)
	classifyRegion := classifyString(&ec.Region)
	classifyTenant := classifyString(&ec.Tenant)
	env := exporterClassifierEnvironment{
		Format:              format,
		Exporter:            si,
		Classify:            classifyGroup,
		ClassifyRegex:       withRegex(classifyGroup),
		ClassifyGroup:       classifyGroup,
		ClassifyGroupRegex:  withRegex(classifyGroup),
		ClassifyRole:        classifyRole,
		ClassifyRoleRegex:   withRegex(classifyRole),
		ClassifySite:        classifySite,
		ClassifySiteRegex:   withRegex(classifySite),
		ClassifyRegion:      classifyRegion,
		ClassifyRegionRegex: withRegex(classifyRegion),
		ClassifyTenant:      classifyTenant,
		ClassifyTenantRegex: withRegex(classifyTenant),
		Reject: func() bool {
			ec.Reject = true
			return false
		},
	}
	if _, err := expr.Run(scr.program, env); err != nil {
		return fmt.Errorf("unable to execute classifier %q: %w", scr, err)
	}
	return nil
}

// UnmarshalText compiles a classification rule for a exporter.
func (scr *ExporterClassifierRule) UnmarshalText(text []byte) error {
	regexValidator := regexValidator{}
	program, err := expr.Compile(string(text),
		expr.Env(exporterClassifierEnvironment{}),
		expr.AsBool(),
		expr.Patch(&regexValidator))
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
	return scr.program.Source().Content()
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
	Format                    func(string, ...any) string
	Exporter                  exporterInfo
	Interface                 interfaceInfo
	ClassifyConnectivity      classifyStringFunc
	ClassifyConnectivityRegex classifyStringRegexFunc
	ClassifyProvider          classifyStringFunc
	ClassifyProviderRegex     classifyStringRegexFunc
	ClassifyExternal          func() bool
	ClassifyInternal          func() bool
	SetName                   func(string) bool
	SetDescription            func(string) bool
	Reject                    func() bool
}

// exec executes the exporter classifier with the provided interface.
func (scr *InterfaceClassifierRule) exec(si exporterInfo, ii interfaceInfo, ic *interfaceClassification) error {
	classifyConnectivity := classifyString(&ic.Connectivity)
	classifyProvider := classifyString(&ic.Provider)
	classifyExternal := func() bool {
		if ic.Boundary == schema.InterfaceBoundaryUndefined {
			ic.Boundary = schema.InterfaceBoundaryExternal
		}
		return true
	}
	classifyInternal := func() bool {
		if ic.Boundary == schema.InterfaceBoundaryUndefined {
			ic.Boundary = schema.InterfaceBoundaryInternal
		}
		return true
	}
	setName := func(name string) bool {
		if ic.Name == "" {
			ic.Name = name
		}
		return true
	}
	setDescription := func(description string) bool {
		if ic.Description == "" {
			ic.Description = description
		}
		return true
	}
	env := interfaceClassifierEnvironment{
		Format:                    format,
		Exporter:                  si,
		Interface:                 ii,
		ClassifyConnectivity:      classifyConnectivity,
		ClassifyProvider:          classifyProvider,
		ClassifyExternal:          classifyExternal,
		ClassifyInternal:          classifyInternal,
		ClassifyConnectivityRegex: withRegex(classifyConnectivity),
		ClassifyProviderRegex:     withRegex(classifyProvider),
		SetName:                   setName,
		SetDescription:            setDescription,
		Reject: func() bool {
			ic.Reject = true
			return false
		},
	}
	if _, err := expr.Run(scr.program, env); err != nil {
		return fmt.Errorf("unable to execute classifier %q: %w", scr, err)
	}
	return nil
}

// UnmarshalText compiles a classification rule for an interface.
func (scr *InterfaceClassifierRule) UnmarshalText(text []byte) error {
	regexValidator := regexValidator{}
	program, err := expr.Compile(string(text),
		expr.Env(interfaceClassifierEnvironment{}),
		expr.AsBool(),
		expr.Patch(&regexValidator))
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
	return scr.program.Source().Content()
}

// MarshalText turns a interface classifier rule into a string
func (scr InterfaceClassifierRule) MarshalText() ([]byte, error) {
	return []byte(scr.String()), nil
}

// withRegex turns a function taking a string into a function taking a
// string to match a regex with, a regex and a template to be expanded
// with the result of the regex.
func withRegex(fn func(string) bool) func(string, string, string) (bool, error) {
	return func(str string, regex string, template string) (bool, error) {
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
		indexes := compiledRegex.FindSubmatchIndex([]byte(str))
		if indexes == nil {
			return false, nil
		}
		result = compiledRegex.ExpandString(result, template, str, indexes)
		return fn(string(result)), nil
	}
}

var normalizeRegex = regexp.MustCompile("[^a-z0-9.+-]+")

// Normalize a string by putting it lowercase and only keeping safe characters
func normalize(str string) string {
	return normalizeRegex.ReplaceAllString(strings.ToLower(str), "")
}

// classifyString is an helper to classify from string to string
func classifyString(output *string) func(string) bool {
	return func(input string) bool {
		if *output == "" {
			*output = normalize(input)
		}
		return true
	}
}

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
	if !strings.HasSuffix(identifier.Value, "Regex") || len(n.Arguments) != 3 {
		return
	}
	str, ok := n.Arguments[1].(*ast.StringNode)
	if !ok {
		return
	}
	if _, err := regexp.Compile(str.Value); err != nil {
		r.invalidRegexes = append(r.invalidRegexes, str.Value)
	}
}
