package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

type SlackClient interface {
	PostMessage(channel string, options ...slack.MsgOption) (string, string, error)
}

var _ Notifier = (*SlackNotifier)(nil)

type SlackNotifier struct {
	level   zerolog.Level
	channel string
	slack   SlackClient
}

func NewSlackNotifier(level zerolog.Level, channel string, slack SlackClient) *SlackNotifier {
	if channel == "" {
		panic("Slack channel cannot be empty")
	}

	if slack == nil {
		panic("Slack client cannot be empty")
	}

	return &SlackNotifier{
		level:   level,
		channel: channel,
		slack:   slack,
	}
}

func (n *SlackNotifier) Enabled(level zerolog.Level) bool {
	return level >= n.level
}

func (n *SlackNotifier) Run(_ *zerolog.Event, level zerolog.Level, message string) {
	if !n.Enabled(level) {
		return
	}

	_ = n.notify(level, message)
}

func (n *SlackNotifier) notify(level zerolog.Level, message string) error {
	alertText := fmt.Sprintf(":red_circle: *%s ALERT!*", strings.ToUpper(level.String()))
	timeText := fmt.Sprint("*Timestamp:* ", time.Now().Format(time.RubyDate))
	messageText := fmt.Sprint("*Message:* ", message)

	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", alertText, false, false), nil, nil),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", messageText, false, false), nil, nil),
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", timeText, false, false), nil, nil),
	}

	msg := slack.MsgOptionBlocks(blocks...)

	_, _, err := n.slack.PostMessage(n.channel, msg)

	return err
}
