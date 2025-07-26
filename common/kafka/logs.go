// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// kafkaLogger implements kgo.Logger interface.
type kafkaLogger struct {
	r *reporter.Reporter
}

// NewLogger creates a new kafka logger using the provided reporter.
func NewLogger(r *reporter.Reporter) kgo.Logger {
	return &kafkaLogger{r: r}
}

// Level returns the current log level.
func (l *kafkaLogger) Level() kgo.LogLevel {
	if !helpers.Testing() {
		return kgo.LogLevelInfo
	}
	return kgo.LogLevelDebug
}

// Log logs a message at the specified level.
func (l *kafkaLogger) Log(level kgo.LogLevel, msg string, keyvals ...any) {
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
