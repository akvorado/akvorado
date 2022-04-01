package logger

import (
	"testing"
)

func TestNew(t *testing.T) {
	logger, err := New(Configuration{})
	if err != nil {
		t.Fatalf("New({}) error:\n%+v", err)
	}
	logger.Info().Int("integer", 15).Msg("log message")
}
