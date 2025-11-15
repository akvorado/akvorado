// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// Logger implements kgo.Logger interface (and in tests kfake.Logger).
type Logger struct {
	r *reporter.Reporter
}

var _ kgo.Logger = &Logger{}

// NewLogger creates a new kafka logger using the provided reporter.
func NewLogger(r *reporter.Reporter) *Logger {
	return &Logger{r: r}
}

// Level returns the current log level.
func (l *Logger) Level() kgo.LogLevel {
	if !helpers.Testing() {
		return kgo.LogLevelInfo
	}
	return kgo.LogLevelDebug
}

// Log logs a message at the specified level.
func (l *Logger) Log(level kgo.LogLevel, msg string, keyvals ...any) {
	switch level {
	case kgo.LogLevelError:
		l.r.Error().Fields(keyvals).Msg(msg)
	case kgo.LogLevelWarn:
		l.r.Warn().Fields(keyvals).Msg(msg)
	case kgo.LogLevelInfo:
		l.r.Info().Fields(keyvals).Msg(msg)
	case kgo.LogLevelDebug:
		l.r.Debug().Fields(keyvals).Msg(msg)
	}
}

// Logf logs a message at the specified level for kfake.
func (l *Logger) Logf(level kfake.LogLevel, msg string, args ...any) {
	switch level {
	case kfake.LogLevelError:
		l.r.Error().Msgf(msg, args...)
	case kfake.LogLevelWarn:
		l.r.Warn().Msgf(msg, args...)
	case kfake.LogLevelInfo:
		l.r.Info().Msgf(msg, args...)
	case kfake.LogLevelDebug:
		l.r.Debug().Msgf(msg, args...)
	}
}
