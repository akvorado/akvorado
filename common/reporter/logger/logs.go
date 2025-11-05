// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package logger

// IntermediateLogger logs the provided message with the provided logger. This
// is used as part of testing.
func IntermediateLogger(l Logger, msg string) {
	l.Info().Msg(msg)
}
