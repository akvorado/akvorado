package core

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
)

// Global cache for regular expressions. No boundary.
var (
	regexCacheLock sync.RWMutex
	regexCache     = make(map[string]*regexp.Regexp)
)

// SamplerClassifierRule defines a classification rule for a sampler.
type SamplerClassifierRule struct {
	program *vm.Program
}

// samplerInfo contains the information we want to expose about a sampler.
type samplerInfo struct {
	IP   string
	Name string
}

// samplerClassifierEnvironment defines the environment used by the sampler classifier
type samplerClassifierEnvironment struct {
	Sampler       samplerInfo
	Classify      func(group string) bool
	ClassifyRegex func(str string, regex string, template string) (bool, error)
}

// exec executes the sampler classifier with the provided sampler.
func (scr *SamplerClassifierRule) exec(si samplerInfo) (string, error) {
	var group string
	classify := func(g string) bool {
		group = g
		return true
	}
	env := samplerClassifierEnvironment{
		Sampler:       si,
		Classify:      classify,
		ClassifyRegex: withRegex(classify),
	}
	if _, err := expr.Run(scr.program, env); err != nil {
		return "", fmt.Errorf("unable to execute classifier %q: %w", scr, err)
	}
	return group, nil
}

// UnmarshalText compiles a classification rule for a sampler.
func (scr *SamplerClassifierRule) UnmarshalText(text []byte) error {
	program, err := expr.Compile(string(text),
		expr.Env(samplerClassifierEnvironment{}),
		expr.AsBool())
	if err != nil {
		return fmt.Errorf("cannot compile sampler classifier rule %q: %w", string(text), err)
	}
	scr.program = program
	return nil
}

// String turns a sampler classifier rule into a string
func (scr SamplerClassifierRule) String() string {
	return scr.program.Source.Content()
}

// MarshalText turns a sampler classifier rule into a string
func (scr SamplerClassifierRule) MarshalText() ([]byte, error) {
	return []byte(scr.String()), nil
}

// InterfaceClassifierRule defines a classification rule for an interface.
type InterfaceClassifierRule struct {
	program *vm.Program
}

// interfaceInfo contains the information we want to expose about a sampler.
type interfaceInfo struct {
	Name        string
	Description string
	Speed       uint32
}

// interfaceBoundary tells if an interface is internal or external
type interfaceBoundary uint

const (
	undefinedBoundary interfaceBoundary = iota
	externalBoundary
	internalBoundary
)

// interfaceClassification contains the information about an interface classification
type interfaceClassification struct {
	Connectivity string
	Provider     string
	Boundary     interfaceBoundary
}

// interfaceClassifierEnvironment defines the environment used by the interface classifier
type interfaceClassifierEnvironment struct {
	Sampler                   samplerInfo
	Interface                 interfaceInfo
	ClassifyConnectivity      func(connectivity string) bool
	ClassifyConnectivityRegex func(str string, regex string, template string) (bool, error)
	ClassifyProvider          func(provider string) bool
	ClassifyProviderRegex     func(str string, regex string, template string) (bool, error)
	ClassifyExternal          func() bool
	ClassifyInternal          func() bool
}

// exec executes the sampler classifier with the provided interface.
func (scr *InterfaceClassifierRule) exec(si samplerInfo, ii interfaceInfo, ic *interfaceClassification) error {
	classifyConnectivity := func(connectivity string) bool {
		if ic.Connectivity == "" {
			ic.Connectivity = normalize(connectivity)
		}
		return true
	}
	classifyProvider := func(provider string) bool {
		if ic.Provider == "" {
			ic.Provider = normalize(provider)
		}
		return true
	}
	classifyExternal := func() bool {
		if ic.Boundary == undefinedBoundary {
			ic.Boundary = externalBoundary
		}
		return true
	}
	classifyInternal := func() bool {
		if ic.Boundary == undefinedBoundary {
			ic.Boundary = internalBoundary
		}
		return true
	}
	env := interfaceClassifierEnvironment{
		Sampler:                   si,
		Interface:                 ii,
		ClassifyConnectivity:      classifyConnectivity,
		ClassifyProvider:          classifyProvider,
		ClassifyExternal:          classifyExternal,
		ClassifyInternal:          classifyInternal,
		ClassifyConnectivityRegex: withRegex(classifyConnectivity),
		ClassifyProviderRegex:     withRegex(classifyProvider),
	}
	if _, err := expr.Run(scr.program, env); err != nil {
		return fmt.Errorf("unable to execute classifier %q: %w", scr, err)
	}
	return nil
}

// UnmarshalText compiles a classification rule for an interface.
func (scr *InterfaceClassifierRule) UnmarshalText(text []byte) error {
	program, err := expr.Compile(string(text),
		expr.Env(interfaceClassifierEnvironment{}),
		expr.AsBool())
	if err != nil {
		return fmt.Errorf("cannot compile interface classifier rule %q: %w", string(text), err)
	}
	scr.program = program
	return nil
}

// String turns a interface classifier rule into a string
func (scr InterfaceClassifierRule) String() string {
	return scr.program.Source.Content()
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

// Normalize a string (provider or connectivity)
func normalize(str string) string {
	return normalizeRegex.ReplaceAllString(strings.ToLower(str), "")
}
