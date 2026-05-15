package util_test

import (
	"testing"
	"time"

	"github.com/andyle182810/gframework/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)

	return t
}

func TestValidateDateRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		maxDays int
		wantErr bool
	}{
		{
			name:    "valid range",
			from:    "2026-01-01",
			to:      "2026-03-01",
			maxDays: 93,
			wantErr: false,
		},
		{
			name:    "same day",
			from:    "2026-01-01",
			to:      "2026-01-01",
			maxDays: 93,
			wantErr: false,
		},
		{
			name:    "to before from",
			from:    "2026-03-01",
			to:      "2026-01-01",
			maxDays: 93,
			wantErr: true,
		},
		{
			name:    "exceeds max days",
			from:    "2026-01-01",
			to:      "2026-04-05", // 94 days
			maxDays: 93,
			wantErr: true,
		},
		{
			name:    "exactly max days",
			from:    "2026-01-01",
			to:      "2026-04-04", // 93 days
			maxDays: 93,
			wantErr: false,
		},
		{
			name:    "custom max days",
			from:    "2026-01-01",
			to:      "2026-01-11", // 10 days
			maxDays: 7,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := util.ValidateDateRange(day(tc.from), day(tc.to), tc.maxDays)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDateRange_Validate(t *testing.T) {
	t.Parallel()

	dateRange := util.DateRange{From: day("2026-01-01"), To: day("2026-03-01"), MaxDays: 0}
	require.NoError(t, dateRange.Validate())

	dateRange = util.DateRange{From: day("2026-03-01"), To: day("2026-01-01"), MaxDays: 0}
	require.Error(t, dateRange.Validate())

	dateRange = util.DateRange{From: day("2026-01-01"), To: day("2026-01-10"), MaxDays: 7}
	require.Error(t, dateRange.Validate())
}
