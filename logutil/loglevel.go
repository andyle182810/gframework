// Package logutil provides log-level string parsing for zerolog and PostgreSQL query tracing.
//
// It offers a single, consistent parsing point for log-level configuration strings used throughout
// the application. Unknown levels default to Info.
//
// Basic usage:
//
//	level := logutil.ParseZerologLevel("debug")
//	pgLevel := logutil.ParsePostgresLogLevel("info")
//
// Supported levels: "trace", "debug", "info", "warn", "error", "fatal", "panic" (for zerolog).
// For PostgreSQL: "trace", "debug", "info", "warn", "error".
package logutil

import (
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
)

func ParseZerologLevel(level string) zerolog.Level {
	switch level {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

func ParsePostgresLogLevel(level string) tracelog.LogLevel {
	switch level {
	case "trace":
		return tracelog.LogLevelTrace
	case "debug":
		return tracelog.LogLevelDebug
	case "info":
		return tracelog.LogLevelInfo
	case "warn":
		return tracelog.LogLevelWarn
	case "error":
		return tracelog.LogLevelError
	default:
		return tracelog.LogLevelInfo
	}
}
