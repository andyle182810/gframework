package notifier

import "github.com/rs/zerolog"

type Notifier interface {
	Enabled(level zerolog.Level) bool
	Run(e *zerolog.Event, level zerolog.Level, message string)
}
