package notifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nikoksr/notify"
	"github.com/rs/zerolog"
)

const TelegramTimeout = 30 * time.Second

type TelegramClient interface {
	AddReceivers(chatIDs ...int64)
	Send(ctx context.Context, subject string, message string) error
}

var _ Notifier = (*TelegramNotifier)(nil)

type TelegramNotifier struct {
	level    zerolog.Level
	channel  int64
	notifier *notify.Notify
}

func NewTelegramNotifier(level zerolog.Level, channel int64, telegram TelegramClient) *TelegramNotifier {
	if telegram == nil {
		panic("Telegram client cannot be empty")
	}

	telegram.AddReceivers(channel)

	notifier := notify.New()
	notifier.UseServices(telegram)

	return &TelegramNotifier{
		level:    level,
		channel:  channel,
		notifier: notifier,
	}
}

func (n *TelegramNotifier) Enabled(level zerolog.Level) bool {
	return level >= n.level
}

func (n *TelegramNotifier) Run(_ *zerolog.Event, level zerolog.Level, message string) {
	if !n.Enabled(level) {
		return
	}

	_ = n.notify(level, message)
}

func (n *TelegramNotifier) notify(level zerolog.Level, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), TelegramTimeout)
	defer cancel()

	title := fmt.Sprint(strings.ToUpper(level.String()), " ALERT!")

	return n.notifier.Send(ctx, title, message)
}
