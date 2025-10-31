package notifier_test

import (
	"testing"

	"github.com/andyle182810/gframework/notifylog/notifier"
	mocks "github.com/andyle182810/gframework/notifylog/notifier/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_Run_Success(t *testing.T) {
	t.Parallel() // Enables parallel test execution

	telegram := mocks.NewMockTelegramClient(t)
	telegram.EXPECT().AddReceivers(int64(123456)).Return()
	telegram.EXPECT().Send(mock.Anything, mock.Anything, "Test Message").Return(nil)

	notifier := notifier.NewTelegramNotifier(zerolog.InfoLevel, 123456, telegram)

	notifier.Run(nil, zerolog.InfoLevel, "Test Message")

	// Verify Send was called with expected arguments
	telegram.AssertCalled(t, "Send", mock.Anything, mock.Anything, "Test Message")
}

func TestTelegramNotifier_Run_Error(t *testing.T) {
	t.Parallel()

	telegram := mocks.NewMockTelegramClient(t)
	telegram.EXPECT().AddReceivers(int64(123456)).Return()
	telegram.EXPECT().Send(mock.Anything, mock.Anything, "Test Error Message").Return(ErrMockError)

	notifier := notifier.NewTelegramNotifier(zerolog.InfoLevel, 123456, telegram)

	notifier.Run(nil, zerolog.InfoLevel, "Test Error Message")

	// Verify Send was called
	telegram.AssertCalled(t, "Send", mock.Anything, mock.Anything, "Test Error Message")
}

func BenchmarkTelegramNotifier_Run(b *testing.B) {
	telegram := mocks.NewMockTelegramClient(b)
	telegram.EXPECT().AddReceivers(int64(123456)).Return()
	telegram.EXPECT().Send(mock.Anything, mock.Anything, "Benchmark test message").Return(nil)

	notifier := notifier.NewTelegramNotifier(zerolog.InfoLevel, 123456, telegram)

	b.ResetTimer() // Reset timer before loop starts

	for range make([]struct{}, b.N) {
		notifier.Run(nil, zerolog.InfoLevel, "Benchmark test message")
	}
}
