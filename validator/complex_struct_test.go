package validator_test

import (
	"strings"
	"testing"
	"time"

	"github.com/andyle182810/gframework/validator"
	"github.com/stretchr/testify/require"
)

type ComplexRequest struct {
	RequiredID   string    `json:"requiredId"           validate:"required,uuid"`
	RequiredID2  string    `json:"requiredId2"          validate:"required,uuid"`
	OptionalID   *string   `json:"optionalId,omitempty" validate:"omitempty,uuid"`
	RequiredTime time.Time `json:"requiredTime"         validate:"required"`
	RequiredEnum string    `json:"requiredEnum"         validate:"required,oneof=alpha beta"`
	// 0x2C is go-playground/validator's escape for a literal comma inside a tag value.
	// Without it the tag parser splits "\\d{4,5}" on the comma and panics.
	PatternField    string     `json:"patternField"              validate:"required,regexp=^\\d{2}[a-zA-Z]-?\\d{40x2C5}$"`
	OptionalText    string     `json:"optionalText"              validate:"omitempty,max=2000"`
	OptionalShort   *string    `json:"optionalShort,omitempty"   validate:"omitempty,max=100"`
	OptionalNumber1 *float64   `json:"optionalNumber1,omitempty" validate:"omitempty,min=0"`
	OptionalNumber2 *float64   `json:"optionalNumber2,omitempty" validate:"omitempty,min=0"`
	OptionalTime    *time.Time `json:"optionalTime,omitempty"`
}

func validComplexRequest() ComplexRequest {
	optID := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	short := "SHORT-001"
	num1 := 1.5
	num2 := 250.75
	optTime := time.Date(2026, time.May, 18, 9, 30, 0, 0, time.UTC)

	return ComplexRequest{
		RequiredID:      "9b2d6ed7-2c1f-4e51-8b0a-1a6b6f3e9a01",
		RequiredID2:     "1d8e8b3a-7b1d-4f2c-9b0a-2a6b6f3e9a02",
		OptionalID:      &optID,
		RequiredTime:    time.Date(2026, time.May, 20, 8, 0, 0, 0, time.UTC),
		RequiredEnum:    "alpha",
		PatternField:    "29A-12345",
		OptionalText:    "some descriptive text",
		OptionalShort:   &short,
		OptionalNumber1: &num1,
		OptionalNumber2: &num2,
		OptionalTime:    &optTime,
	}
}

func findComplexError(t *testing.T, err error, field string) validator.ValidationError {
	t.Helper()

	var validationErrors validator.ValidationErrors

	require.ErrorAs(t, err, &validationErrors)

	for _, ve := range validationErrors {
		if ve.Field == field {
			return ve
		}
	}

	t.Fatalf("no validation error found for field %q; errors: %+v", field, validationErrors)

	return validator.ValidationError{} //nolint:exhaustruct
}

func TestComplexStruct_Valid(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	require.NoError(t, v.Validate(validComplexRequest()))
}

func TestComplexStruct_ValidWithOptionalFieldsOmitted(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()

	input := ComplexRequest{ //nolint:exhaustruct
		RequiredID:   "9b2d6ed7-2c1f-4e51-8b0a-1a6b6f3e9a01",
		RequiredID2:  "1d8e8b3a-7b1d-4f2c-9b0a-2a6b6f3e9a02",
		RequiredTime: time.Date(2026, time.May, 20, 8, 0, 0, 0, time.UTC),
		RequiredEnum: "beta",
		PatternField: "51F1234",
	}

	require.NoError(t, v.Validate(input))
}

func TestComplexStruct_RequiredIDMissing(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.RequiredID = ""

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "requiredId")
	require.Equal(t, "required", ve.Tag)
	require.Equal(t, "requiredId is required", ve.Message)
}

func TestComplexStruct_RequiredIDInvalidUUID(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.RequiredID = "not-a-uuid"

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "requiredId")
	require.Equal(t, "uuid", ve.Tag)
	require.Equal(t, "requiredId must be a valid UUID", ve.Message)
}

func TestComplexStruct_RequiredID2Missing(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.RequiredID2 = ""

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "requiredId2")
	require.Equal(t, "required", ve.Tag)
}

func TestComplexStruct_RequiredID2InvalidUUID(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.RequiredID2 = "12345"

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "requiredId2")
	require.Equal(t, "uuid", ve.Tag)
}

func TestComplexStruct_OptionalIDOmittedIsValid(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.OptionalID = nil

	require.NoError(t, v.Validate(input))
}

func TestComplexStruct_OptionalIDInvalidUUID(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	bad := "not-a-uuid"
	input := validComplexRequest()
	input.OptionalID = &bad

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "optionalId")
	require.Equal(t, "uuid", ve.Tag)
}

func TestComplexStruct_RequiredTimeMissing(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.RequiredTime = time.Time{}

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "requiredTime")
	require.Equal(t, "required", ve.Tag)
	require.Equal(t, "requiredTime is required", ve.Message)
}

func TestComplexStruct_RequiredEnumMissing(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.RequiredEnum = ""

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "requiredEnum")
	require.Equal(t, "required", ve.Tag)
}

func TestComplexStruct_RequiredEnumInvalidValue(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.RequiredEnum = "gamma"

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "requiredEnum")
	require.Equal(t, "oneof", ve.Tag)
	require.Contains(t, ve.Message, "alpha beta")
}

func TestComplexStruct_RequiredEnumAcceptsAllValidValues(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()

	for _, value := range []string{"alpha", "beta"} {
		input := validComplexRequest()
		input.RequiredEnum = value
		require.NoErrorf(t, v.Validate(input), "expected %q to be valid", value)
	}
}

func TestComplexStruct_PatternFieldMissing(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.PatternField = ""

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "patternField")
	require.Equal(t, "required", ve.Tag)
}

func TestComplexStruct_PatternFieldValidFormats(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()

	validValues := []string{
		"29A-12345",
		"29A12345",
		"51F-1234",
		"51F1234",
		"30b-12345",
	}

	for _, val := range validValues {
		input := validComplexRequest()
		input.PatternField = val
		require.NoErrorf(t, v.Validate(input), "expected %q to be valid", val)
	}
}

func TestComplexStruct_PatternFieldInvalidFormats(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()

	invalidValues := []string{
		"ABC-12345",
		"2A-12345",
		"29-12345",
		"29A-123",
		"29A-123456",
		"29A_12345",
		"29AB-12345",
	}

	for _, val := range invalidValues {
		input := validComplexRequest()
		input.PatternField = val

		err := v.Validate(input)
		require.Errorf(t, err, "expected %q to be invalid", val)

		ve := findComplexError(t, err, "patternField")
		require.Equal(t, "regexp", ve.Tag)
		require.Contains(t, ve.Message, "patternField must match the required pattern")
	}
}

func TestComplexStruct_OptionalTextEmptyIsValid(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.OptionalText = ""

	require.NoError(t, v.Validate(input))
}

func TestComplexStruct_OptionalTextExceedsMax(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.OptionalText = strings.Repeat("a", 2001)

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "optionalText")
	require.Equal(t, "max", ve.Tag)
	require.Equal(t, "optionalText must be at most 2000", ve.Message)
}

func TestComplexStruct_OptionalTextAtMaxBoundary(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.OptionalText = strings.Repeat("a", 2000)

	require.NoError(t, v.Validate(input))
}

func TestComplexStruct_OptionalShortExceedsMax(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	long := strings.Repeat("L", 101)
	input := validComplexRequest()
	input.OptionalShort = &long

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "optionalShort")
	require.Equal(t, "max", ve.Tag)
}

func TestComplexStruct_OptionalShortAtMaxBoundary(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	val := strings.Repeat("L", 100)
	input := validComplexRequest()
	input.OptionalShort = &val

	require.NoError(t, v.Validate(input))
}

func TestComplexStruct_OptionalNumber1Negative(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	val := -0.01
	input := validComplexRequest()
	input.OptionalNumber1 = &val

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "optionalNumber1")
	require.Equal(t, "min", ve.Tag)
	require.Equal(t, "optionalNumber1 must be at least 0", ve.Message)
}

func TestComplexStruct_OptionalNumber1ZeroIsValid(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	val := 0.0
	input := validComplexRequest()
	input.OptionalNumber1 = &val

	require.NoError(t, v.Validate(input))
}

func TestComplexStruct_OptionalNumber2Negative(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	val := -100.5
	input := validComplexRequest()
	input.OptionalNumber2 = &val

	err := v.Validate(input)
	require.Error(t, err)

	ve := findComplexError(t, err, "optionalNumber2")
	require.Equal(t, "min", ve.Tag)
}

func TestComplexStruct_OptionalTimeNilIsValid(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()
	input := validComplexRequest()
	input.OptionalTime = nil

	require.NoError(t, v.Validate(input))
}

func TestComplexStruct_MultipleErrorsAggregated(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()

	input := ComplexRequest{ //nolint:exhaustruct
		RequiredID:   "",
		RequiredID2:  "not-uuid",
		RequiredTime: time.Time{},
		RequiredEnum: "bogus",
		PatternField: "bad-value",
	}

	err := v.Validate(input)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors

	require.ErrorAs(t, err, &validationErrors)
	require.GreaterOrEqual(t, len(validationErrors), 5)

	fields := map[string]bool{}
	for _, ve := range validationErrors {
		fields[ve.Field] = true
	}

	require.True(t, fields["requiredId"])
	require.True(t, fields["requiredId2"])
	require.True(t, fields["requiredTime"])
	require.True(t, fields["requiredEnum"])
	require.True(t, fields["patternField"])
}

func TestComplexStruct_RegexpCommaEscapedQuantifier(t *testing.T) {
	t.Parallel()

	v := validator.DefaultRestValidator()

	// 0x2C is go-playground/validator's escape for a literal comma inside a tag value.
	// Without it the tag parser splits "\\d{4,5}" on the comma and panics.
	type PlateStruct struct {
		TruckPlate string `json:"truckPlate" validate:"required,regexp=^\\d{2}[a-zA-Z]-?\\d{40x2C5}$"`
	}

	t.Run("valid plates match the {4,5} quantifier", func(t *testing.T) {
		t.Parallel()

		validPlates := []string{
			"29A-12345",
			"29A12345",
			"51F-1234",
			"51F1234",
			"30b-12345",
		}

		for _, plate := range validPlates {
			err := v.Validate(PlateStruct{TruckPlate: plate})
			require.NoErrorf(t, err, "expected plate %q to be valid", plate)
		}
	})

	t.Run("invalid plates fail the regexp", func(t *testing.T) {
		t.Parallel()

		invalidPlates := []string{
			"ABC-12345",
			"2A-12345",
			"29-12345",
			"29A-123",
			"29A-123456",
			"29A_12345",
			"29AB-12345",
		}

		for _, plate := range invalidPlates {
			err := v.Validate(PlateStruct{TruckPlate: plate})
			require.Errorf(t, err, "expected plate %q to be invalid", plate)

			ve := findComplexError(t, err, "truckPlate")
			require.Equal(t, "regexp", ve.Tag)
			require.Equal(t, "truckPlate must match the required pattern", ve.Message)
		}
	})

	t.Run("required tag still fires when value is empty", func(t *testing.T) {
		t.Parallel()

		err := v.Validate(PlateStruct{TruckPlate: ""})
		require.Error(t, err)

		ve := findComplexError(t, err, "truckPlate")
		require.Equal(t, "required", ve.Tag)
	})
}
