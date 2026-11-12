package util_test

import (
	"testing"
	"time"

	"github.com/andyle182810/gframework/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dateStart = "2026-01-01"

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
			from:    dateStart,
			to:      "2026-03-01",
			maxDays: 93,
			wantErr: false,
		},
		{
			name:    "same day",
			from:    dateStart,
			to:      dateStart,
			maxDays: 93,
			wantErr: false,
		},
		{
			name:    "to before from",
			from:    "2026-03-01",
			to:      dateStart,
			maxDays: 93,
			wantErr: true,
		},
		{
			name:    "exceeds max days",
			from:    dateStart,
			to:      "2026-04-04", // 94 days, counting both ends
			maxDays: 93,
			wantErr: true,
		},
		{
			name:    "exactly max days",
			from:    dateStart,
			to:      "2026-04-03", // 93 days, counting both ends
			maxDays: 93,
			wantErr: false,
		},
		{
			name:    "custom max days",
			from:    dateStart,
			to:      "2026-01-11", // 11 days
			maxDays: 7,
			wantErr: true,
		},
		{
			name:    "a week counting both ends",
			from:    dateStart,
			to:      "2026-01-07",
			maxDays: 7,
			wantErr: false,
		},
		{
			name:    "one day past a week",
			from:    dateStart,
			to:      "2026-01-08",
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

	dateRange := util.DateRange{From: day(dateStart), To: day("2026-03-01"), MaxDays: 0}
	require.NoError(t, dateRange.Validate())

	dateRange = util.DateRange{From: day("2026-03-01"), To: day(dateStart), MaxDays: 0}
	require.Error(t, dateRange.Validate())

	dateRange = util.DateRange{From: day(dateStart), To: day("2026-01-10"), MaxDays: 7}
	require.Error(t, dateRange.Validate())
}

func TestValidateDateRange_EndOfDay(t *testing.T) {
	t.Parallel()

	endOf := func(s string) time.Time { return day(s).Add(24*time.Hour - time.Nanosecond) }

	require.NoError(t, util.ValidateDateRange(day(dateStart), endOf("2026-04-03"), 93))
	require.ErrorIs(t, util.ValidateDateRange(day(dateStart), endOf("2026-04-04"), 93), util.ErrDateRangeExceeded)
}
