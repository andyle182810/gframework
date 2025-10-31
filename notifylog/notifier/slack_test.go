package notifier_test

import (
	"errors"
	"testing"

	"github.com/andyle182810/gframework/notifylog/notifier"
	mocks "github.com/andyle182810/gframework/notifylog/notifier/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

var ErrMockError = errors.New("mock error")

func TestSlackNotifier_Run_Success(t *testing.T) {
	t.Parallel()

	slack := mocks.NewMockSlackClient(t)
	slack.EXPECT().PostMessage("test-channel", mock.Anything).Return("mockTS", "mockChannel", nil)

	notifier := notifier.NewSlackNotifier(zerolog.InfoLevel, "test-channel", slack)

	notifier.Run(nil, zerolog.InfoLevel, "Test Message")

	// Verify PostMessage was called with expected arguments
	slack.AssertCalled(t, "PostMessage", "test-channel", mock.Anything)
}

func TestSlackNotifier_Run_Error(t *testing.T) {
	t.Parallel()

	slack := mocks.NewMockSlackClient(t)
	slack.EXPECT().PostMessage("test-channel", mock.Anything).Return("", "", ErrMockError)

	notifier := notifier.NewSlackNotifier(zerolog.InfoLevel, "test-channel", slack)

	notifier.Run(nil, zerolog.InfoLevel, "Test Error Message")

	// Verify PostMessage was called
	slack.AssertCalled(t, "PostMessage", "test-channel", mock.Anything)
}

func BenchmarkSlackNotifier_Run(b *testing.B) {
	slack := mocks.NewMockSlackClient(b)
	slack.EXPECT().PostMessage("test-channel", mock.Anything).Return("mockTS", "mockChannel", nil)

	notifier := notifier.NewSlackNotifier(zerolog.InfoLevel, "test-channel", slack)

	message := "Benchmark test message"

	b.ResetTimer() // Reset timer before loop starts

	for range make([]struct{}, b.N) {
		notifier.Run(nil, zerolog.InfoLevel, message)
	}
}
