package util

import (
	"errors"
	"time"
)

const dateLayout = "2006-01-02"

var ErrNotSunday = errors.New("date must be a Sunday")

func IsSundayDate(value string) (bool, error) {
	t, err := time.Parse(dateLayout, value)
	if err != nil {
		return false, err
	}

	return t.Weekday() == time.Sunday, nil
}

func ValidateSunday(value string) error {
	is, err := IsSundayDate(value)
	if err != nil {
		return err
	}

	if !is {
		return ErrNotSunday
	}

	return nil
}
