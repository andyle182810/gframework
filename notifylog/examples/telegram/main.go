package main

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/andyle182810/gframework/notifylog"
	"github.com/andyle182810/gframework/notifylog/notifier"
	"github.com/nikoksr/notify/service/telegram"
	"github.com/rs/zerolog"
)

func main() {
	tg, err := telegram.New(os.Getenv("TELEGRAM_API_TOKEN"))
	if err != nil {
		slog.Error("failed to initiate telegram client")
	}

	channel, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHANNEL"), 10, 64)
	if err != nil {
		slog.Error("failed to get channel")
	}

	telegram := notifier.NewTelegramNotifier(zerolog.InfoLevel, channel, tg)
	log := notifylog.New("test", notifylog.JSON, telegram)

	log.Info().Str("foo", "bar").Msg("Hello world")
}
