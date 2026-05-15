package util

import (
	"errors"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

var (
	ErrNotSunday         = errors.New("date must be a Sunday")
	ErrFromAfterTo       = errors.New("'from' must be before 'to'")
	ErrDateRangeExceeded = errors.New("date range exceeded")
)

func ParseSundayDate(value string) (time.Time, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, err
	}

	if parsed.Weekday() != time.Sunday {
		return time.Time{}, ErrNotSunday
	}

	return parsed, nil
}

func ValidateSunday(value string) error {
	_, err := ParseSundayDate(value)

	return err
}

const MaxDateRangeDays = 93

type DateRange struct {
	From    time.Time
	To      time.Time
	MaxDays int
}

func (dr DateRange) Validate() error {
	maxDays := dr.MaxDays
	if maxDays == 0 {
		maxDays = MaxDateRangeDays
	}

	return ValidateDateRange(dr.From, dr.To, maxDays)
}

func ValidateDateRange(from, to time.Time, maxDays int) error {
	if to.Before(from) {
		return fmt.Errorf("from=%s is after to=%s: %w", from.Format("2006-01-02"), to.Format("2006-01-02"), ErrFromAfterTo)
	}

	if to.Sub(from) > time.Duration(maxDays)*24*time.Hour {
		return fmt.Errorf("date range must not exceed %d days: %w", maxDays, ErrDateRangeExceeded)
	}

	return nil
}
