package redispub_test

import (
	"testing"
	"time"

	"github.com/andyle182810/gframework/redispub"
	"github.com/stretchr/testify/assert"
)

const baseStreamID = "1789121150000-0"

func TestOlderStreamID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{"earlier millisecond", baseStreamID, "1789121180000-0", true},
		{"later millisecond", "1789121180000-0", baseStreamID, false},
		{"same millisecond, lower sequence", "1789121150000-3", "1789121150000-7", true},
		{"same millisecond, higher sequence", "1789121150000-7", "1789121150000-3", false},
		{"identical ids", baseStreamID, baseStreamID, false},
		{"shorter millisecond string is older", "9999999999999-0", "10000000000000-0", true},
		{"longer millisecond string is newer", "10000000000000-0", "9999999999999-0", false},
		{"missing sequence reads as zero", "1789121150000", "1789121150000-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, redispub.OlderStreamID(tt.left, tt.right))
		})
	}
}

func TestStreamIDAt(t *testing.T) {
	t.Parallel()

	instant := time.UnixMilli(1789121150000).UTC()

	assert.Equal(t, baseStreamID, redispub.StreamIDAt(instant))
	assert.True(t, redispub.OlderStreamID(
		redispub.StreamIDAt(instant.Add(-time.Hour)),
		redispub.StreamIDAt(instant),
	))
}
