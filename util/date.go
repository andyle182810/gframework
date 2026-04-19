package util

import (
	"errors"
	"time"
)

const dateLayout = "2006-01-02"

var ErrNotSunday = errors.New("date must be a Sunday")

func ParseSundayDate(value string) (time.Time, error) {
	t, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, err
	}

	if t.Weekday() != time.Sunday {
		return time.Time{}, ErrNotSunday
	}

	return t, nil
}

func ValidateSunday(value string) error {
	_, err := ParseSundayDate(value)
	return err
}
