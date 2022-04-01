// Package logger handles logging for akvorado.
//
// This is a thin wrapper around zerolog. It is currently not
// configurable as we don't need anything fancy yet for configuration.
//
// It also brings some conventions like the presence of "module" in
// each context to be able to filter logs more easily. However, this
// convention is not really enforced. Once you have a root logger,
// create sublogger with New and provide a new value for "module".
package logger

import (
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"akvorado/common/reporter/stack"
)

// Logger is a logger instance. It is compatible with the interface
// from zerolog by design.
type Logger struct {
	zerolog.Logger
}

// New creates a new logger
func New(config Configuration) (Logger, error) {
	// Initialize the logger
	var logger = log.Logger.Hook(contextHook{})
	return Logger{logger}, nil
}

type contextHook struct{}

// Run adds more context to an event, including "module" and "caller".
func (h contextHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	callStack := stack.Callers()
	callStack = callStack[3:] // Trial and error, there is a test to check it works
	caller := callStack[0].SourceFile(true)
	e.Str("caller", caller)
	for _, call := range callStack {
		module := call.FunctionName()
		if !strings.HasPrefix(module, stack.ModuleName) {
			continue
		}
		module = strings.SplitN(module, ".", 2)[0]
		e.Str("module", module)
		break
	}
}
