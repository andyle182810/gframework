package logutil_test

import (
	"testing"

	"github.com/andyle182810/gframework/logutil"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestParseZerologLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected zerolog.Level
	}{
		{
			name:     "trace level",
			input:    "trace",
			expected: zerolog.TraceLevel,
		},
		{
			name:     "debug level",
			input:    "debug",
			expected: zerolog.DebugLevel,
		},
		{
			name:     "info level",
			input:    "info",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "warn level",
			input:    "warn",
			expected: zerolog.WarnLevel,
		},
		{
			name:     "error level",
			input:    "error",
			expected: zerolog.ErrorLevel,
		},
		{
			name:     "fatal level",
			input:    "fatal",
			expected: zerolog.FatalLevel,
		},
		{
			name:     "panic level",
			input:    "panic",
			expected: zerolog.PanicLevel,
		},
		{
			name:     "unknown level defaults to info",
			input:    "unknown",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "empty string defaults to info",
			input:    "",
			expected: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := logutil.ParseZerologLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePostgresLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected tracelog.LogLevel
	}{
		{
			name:     "trace level",
			input:    "trace",
			expected: tracelog.LogLevelTrace,
		},
		{
			name:     "debug level",
			input:    "debug",
			expected: tracelog.LogLevelDebug,
		},
		{
			name:     "info level",
			input:    "info",
			expected: tracelog.LogLevelInfo,
		},
		{
			name:     "warn level",
			input:    "warn",
			expected: tracelog.LogLevelWarn,
		},
		{
			name:     "error level",
			input:    "error",
			expected: tracelog.LogLevelError,
		},
		{
			name:     "unknown level defaults to info",
			input:    "unknown",
			expected: tracelog.LogLevelInfo,
		},
		{
			name:     "empty string defaults to info",
			input:    "",
			expected: tracelog.LogLevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := logutil.ParsePostgresLogLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
