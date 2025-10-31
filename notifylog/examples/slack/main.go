package main

import (
	"os"

	"github.com/andyle182810/gframework/notifylog"
	"github.com/andyle182810/gframework/notifylog/notifier"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

func main() {
	slack := notifier.NewSlackNotifier(
		zerolog.InfoLevel,
		os.Getenv("SLACK_CHANNEL"),
		slack.New(os.Getenv("SLACK_TOKEN")),
	)
	log := notifylog.New("test", notifylog.JSON, slack)

	log.Info().Str("foo", "bar").Msg("Hello world ddd")
}
