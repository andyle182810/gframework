package notifylog

import (
	"os"
	"strings"
	"time"

	"github.com/andyle182810/gframework/notifylog/notifier"
	"github.com/rs/zerolog"
)

type Encoding int64

const (
	CBOR Encoding = iota
	JSON
)

type NotifyLog struct {
	zerolog.Logger
}

func New(name string, encoding Encoding, notifiers ...notifier.Notifier) NotifyLog {
	zerolog.SetGlobalLevel(getLogLevel())

	var logger zerolog.Logger

	if encoding == CBOR {
		logger = cbor(name)
	} else {
		logger = json(name)
	}

	for _, not := range notifiers {
		logger = logger.Hook(not)
	}

	return NotifyLog{
		Logger: logger,
	}
}

func cbor(name string) zerolog.Logger {
	output := zerolog.ConsoleWriter{
		Out:                 os.Stdout,
		TimeFormat:          time.RFC3339Nano,
		NoColor:             false,
		TimeLocation:        nil,
		PartsOrder:          nil,
		PartsExclude:        nil,
		FieldsOrder:         nil,
		FieldsExclude:       nil,
		FormatTimestamp:     nil,
		FormatLevel:         nil,
		FormatCaller:        nil,
		FormatMessage:       nil,
		FormatFieldName:     nil,
		FormatFieldValue:    nil,
		FormatErrFieldName:  nil,
		FormatErrFieldValue: nil,
		FormatExtra:         nil,
		FormatPrepare:       nil,
	}

	logger := zerolog.New(output).With().Caller().Stack().Timestamp().Str("logger", name).Logger()

	return logger
}

func json(name string) zerolog.Logger {
	timestampFunc := func() time.Time {
		return time.Now().UTC()
	}

	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimestampFunc = timestampFunc
	zerolog.TimeFieldFormat = time.RFC3339Nano

	logger := zerolog.New(os.Stderr).With().Caller().Stack().Timestamp().Str("logger", name).Logger()

	return logger
}

func getLogLevel() zerolog.Level {
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))

	switch logLevel {
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
		return zerolog.DebugLevel
	}
}
