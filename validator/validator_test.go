package validator_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/andyle182810/gframework/validator"
	gvalidator "github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type TestStruct struct {
	Name  string `json:"name"  validate:"required,min=3,max=20"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age"   validate:"gte=18,lte=100"`
}

type ExtendedTestStruct struct {
	URL         string `json:"url"         validate:"required,url"`
	URI         string `json:"uri"         validate:"uri"`
	UUID        string `json:"uuid"        validate:"uuid"`
	AlphaNum    string `json:"alphaNum"    validate:"alphanum"`
	Numeric     string `json:"numeric"     validate:"numeric"`
	Length      string `json:"length"      validate:"len=5"`
	GreaterThan int    `json:"greaterThan" validate:"gt=10"`
	LessThan    int    `json:"lessThan"    validate:"lt=100"`
	OneOf       string `json:"oneOf"       validate:"oneof=red green blue"`
}

func TestDefaultRestValidator(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()
	require.NotNil(t, validatorInstance)
	require.NotNil(t, validatorInstance.Validator)
}

func TestValidate_Success(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	validInput := TestStruct{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   25,
	}

	err := validatorInstance.Validate(validInput)
	require.NoError(t, err)
}

func TestValidate_RequiredFieldMissing(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := TestStruct{
		Name:  "",
		Email: "john.doe@example.com",
		Age:   25,
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	require.Len(t, validationErrors, 1)
	require.Equal(t, "name", validationErrors[0].Field)
	require.Equal(t, "required", validationErrors[0].Tag)
	require.Equal(t, "name is required", validationErrors[0].Message)
}

func TestValidate_InvalidEmail(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := TestStruct{
		Name:  "John Doe",
		Email: "invalid-email",
		Age:   25,
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	require.Len(t, validationErrors, 1)
	require.Equal(t, "email", validationErrors[0].Field)
	require.Equal(t, "email", validationErrors[0].Tag)
	require.Equal(t, "email must be a valid email address", validationErrors[0].Message)
}

func TestValidate_AgeOutOfRange(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := TestStruct{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   17,
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	require.Len(t, validationErrors, 1)
	require.Equal(t, "age", validationErrors[0].Field)
	require.Equal(t, "gte", validationErrors[0].Tag)
	require.Equal(t, "age must be greater than or equal to 18", validationErrors[0].Message)
}

func TestValidate_MinMaxValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	tests := []struct {
		name          string
		input         TestStruct
		expectedField string
		expectedMsg   string
	}{
		{
			name: "name too short",
			input: TestStruct{
				Name:  "Jo",
				Email: "john@example.com",
				Age:   25,
			},
			expectedField: "name",
			expectedMsg:   "name must be at least 3",
		},
		{
			name: "name too long",
			input: TestStruct{
				Name:  "John Doe With Very Long Name",
				Email: "john@example.com",
				Age:   25,
			},
			expectedField: "name",
			expectedMsg:   "name must be at most 20",
		},
		{
			name: "age too high",
			input: TestStruct{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   101,
			},
			expectedField: "age",
			expectedMsg:   "age must be less than or equal to 100",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validatorInstance.Validate(testCase.input)
			require.Error(t, err)

			var validationErrors validator.ValidationErrors
			ok := errors.As(err, &validationErrors)
			require.True(t, ok)

			require.NotEmpty(t, validationErrors)
			require.Equal(t, testCase.expectedField, validationErrors[0].Field)
			require.Contains(t, validationErrors[0].Message, testCase.expectedMsg)
		})
	}
}

func TestValidate_URLValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "not-a-url",
		URI:         "",
		UUID:        "",
		AlphaNum:    "",
		Numeric:     "",
		Length:      "",
		GreaterThan: 0,
		LessThan:    0,
		OneOf:       "",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "url")
	require.NotNil(t, found)
	require.Equal(t, "url", found.Tag)
	require.Equal(t, "url must be a valid URL", found.Message)
}

func TestValidate_UUIDValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "https://example.com",
		URI:         "",
		UUID:        "not-a-uuid",
		AlphaNum:    "",
		Numeric:     "",
		Length:      "",
		GreaterThan: 0,
		LessThan:    0,
		OneOf:       "",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "uuid")
	require.NotNil(t, found)
	require.Equal(t, "uuid", found.Tag)
	require.Equal(t, "uuid must be a valid UUID", found.Message)
}

func TestValidate_AlphanumValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "https://example.com",
		URI:         "",
		UUID:        "",
		AlphaNum:    "abc123!@#",
		Numeric:     "",
		Length:      "",
		GreaterThan: 0,
		LessThan:    0,
		OneOf:       "",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "alphaNum")
	require.NotNil(t, found)
	require.Equal(t, "alphanum", found.Tag)
	require.Equal(t, "alphaNum must contain only alphanumeric characters", found.Message)
}

func TestValidate_NumericValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "https://example.com",
		URI:         "",
		UUID:        "",
		AlphaNum:    "",
		Numeric:     "abc",
		Length:      "",
		GreaterThan: 0,
		LessThan:    0,
		OneOf:       "",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "numeric")
	require.NotNil(t, found)
	require.Equal(t, "numeric", found.Tag)
	require.Equal(t, "numeric must be numeric", found.Message)
}

func TestValidate_LengthValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "https://example.com",
		URI:         "",
		UUID:        "",
		AlphaNum:    "",
		Numeric:     "",
		Length:      "abc",
		GreaterThan: 0,
		LessThan:    0,
		OneOf:       "",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "length")
	require.NotNil(t, found)
	require.Equal(t, "len", found.Tag)
	require.Equal(t, "length must be exactly 5 characters", found.Message)
}

func TestValidate_GreaterThanValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "https://example.com",
		URI:         "",
		UUID:        "",
		AlphaNum:    "",
		Numeric:     "",
		Length:      "",
		GreaterThan: 5,
		LessThan:    0,
		OneOf:       "",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "greaterThan")
	require.NotNil(t, found)
	require.Equal(t, "gt", found.Tag)
	require.Equal(t, "greaterThan must be greater than 10", found.Message)
}

func TestValidate_LessThanValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "https://example.com",
		URI:         "",
		UUID:        "",
		AlphaNum:    "",
		Numeric:     "",
		Length:      "",
		GreaterThan: 0,
		LessThan:    150,
		OneOf:       "",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "lessThan")
	require.NotNil(t, found)
	require.Equal(t, "lt", found.Tag)
	require.Equal(t, "lessThan must be less than 100", found.Message)
}

func TestValidate_OneOfValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := ExtendedTestStruct{
		URL:         "https://example.com",
		URI:         "",
		UUID:        "",
		AlphaNum:    "",
		Numeric:     "",
		Length:      "",
		GreaterThan: 0,
		LessThan:    0,
		OneOf:       "yellow",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	found := findValidationError(validationErrors, "oneOf")
	require.NotNil(t, found)
	require.Equal(t, "oneof", found.Tag)
	require.Equal(t, "oneOf must be one of [red green blue]", found.Message)
}

func TestValidate_MultipleErrors(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	invalidInput := TestStruct{
		Name:  "",
		Email: "invalid-email",
		Age:   17,
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	require.Len(t, validationErrors, 3)

	errorMsg := validationErrors.Error()
	require.Contains(t, errorMsg, "name is required")
	require.Contains(t, errorMsg, "email must be a valid email address")
	require.Contains(t, errorMsg, "age must be greater than or equal to 18")
	require.Contains(t, errorMsg, ";")
}

func TestValidate_JSONFieldNames(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	type User struct {
		FullName string `json:"fullName" validate:"required"`
		EmailID  string `json:"emailId"  validate:"required,email"`
	}

	invalidInput := User{
		FullName: "",
		EmailID:  "invalid",
	}

	err := validatorInstance.Validate(invalidInput)
	require.Error(t, err)

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	require.True(t, ok)

	fields := make(map[string]bool)
	for _, vErr := range validationErrors {
		fields[vErr.Field] = true
	}

	require.True(t, fields["fullName"], "should use JSON field name 'fullName'")
	require.True(t, fields["emailId"], "should use JSON field name 'emailId'")
	require.False(t, fields["FullName"], "should not use struct field name 'FullName'")
	require.False(t, fields["EmailID"], "should not use struct field name 'EmailID'")
}

func TestRegisterCustomValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	err := validatorInstance.RegisterCustomValidation("custom", func(fl gvalidator.FieldLevel) bool {
		return fl.Field().String() == "custom-value"
	})
	require.NoError(t, err)

	type CustomStruct struct {
		Field string `json:"field" validate:"custom"`
	}

	validInput := CustomStruct{Field: "custom-value"}
	err = validatorInstance.Validate(validInput)
	require.NoError(t, err)

	invalidInput := CustomStruct{Field: "other-value"}
	err = validatorInstance.Validate(invalidInput)
	require.Error(t, err)
}

func TestRegisterStructValidation(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	type PasswordStruct struct {
		Password        string `json:"password"`
		PasswordConfirm string `json:"passwordConfirm"`
	}

	validatorInstance.RegisterStructValidation(func(structLevel gvalidator.StructLevel) {
		pwd, ok := structLevel.Current().Interface().(PasswordStruct)
		if !ok {
			return
		}

		if pwd.Password != pwd.PasswordConfirm {
			structLevel.ReportError(pwd.PasswordConfirm, "passwordConfirm", "PasswordConfirm", "passwordmatch", "")
		}
	}, PasswordStruct{
		Password:        "",
		PasswordConfirm: "",
	})

	validInput := PasswordStruct{
		Password:        "secret123",
		PasswordConfirm: "secret123",
	}
	err := validatorInstance.Validate(validInput)
	require.NoError(t, err)

	invalidInput := PasswordStruct{
		Password:        "secret123",
		PasswordConfirm: "different",
	}
	err = validatorInstance.Validate(invalidInput)
	require.Error(t, err)
}

func TestValidationError_ErrorMethod(t *testing.T) {
	t.Parallel()

	validationErrs := validator.ValidationErrors{
		{Field: "name", Tag: "required", Value: "", Message: "name is required"},
		{Field: "email", Tag: "email", Value: "invalid", Message: "email must be a valid email address"},
		{Field: "age", Tag: "gte", Value: "17", Message: "age must be greater than or equal to 18"},
	}

	errorMsg := validationErrs.Error()
	require.Contains(t, errorMsg, "name is required")
	require.Contains(t, errorMsg, "email must be a valid email address")
	require.Contains(t, errorMsg, "age must be greater than or equal to 18")

	parts := strings.Split(errorMsg, "; ")
	require.Len(t, parts, 3)
}

type DecimalStruct struct {
	Amount decimal.Decimal  `json:"amount" validate:"required,gte=0"`
	Fee    decimal.Decimal  `json:"fee"    validate:"gte=0,lte=1000"`
	Limit  *decimal.Decimal `json:"limit"  validate:"omitempty,gte=0"`
}

func TestValidate_DecimalGteSuccess(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	tests := []struct {
		name  string
		input DecimalStruct
	}{
		{
			name: "positive decimal",
			input: DecimalStruct{
				Amount: decimal.NewFromFloat(100.50),
				Fee:    decimal.NewFromInt(10),
				Limit:  nil,
			},
		},
		{
			name: "zero fee (gte=0 without required)",
			input: DecimalStruct{
				Amount: decimal.NewFromInt(1),
				Fee:    decimal.NewFromInt(0),
				Limit:  nil,
			},
		},
		{
			name: "nil optional pointer",
			input: DecimalStruct{
				Amount: decimal.NewFromInt(1),
				Fee:    decimal.NewFromInt(0),
				Limit:  nil,
			},
		},
		{
			name: "non-nil optional pointer with valid value",
			input: DecimalStruct{
				Amount: decimal.NewFromInt(1),
				Fee:    decimal.NewFromInt(0),
				Limit:  decimalPtr(decimal.NewFromFloat(500.25)),
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validatorInstance.Validate(testCase.input)
			require.NoError(t, err)
		})
	}
}

func TestValidate_DecimalGteFailure(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()

	tests := []struct {
		name          string
		input         DecimalStruct
		expectedField string
		expectedTag   string
		expectedMsg   string
	}{
		{
			name: "negative amount",
			input: DecimalStruct{
				Amount: decimal.NewFromFloat(-1.5),
				Fee:    decimal.NewFromInt(0),
				Limit:  nil,
			},
			expectedField: "amount",
			expectedTag:   "gte",
			expectedMsg:   "amount must be greater than or equal to 0",
		},
		{
			name: "negative fee",
			input: DecimalStruct{
				Amount: decimal.NewFromInt(100),
				Fee:    decimal.NewFromFloat(-0.01),
				Limit:  nil,
			},
			expectedField: "fee",
			expectedTag:   "gte",
			expectedMsg:   "fee must be greater than or equal to 0",
		},
		{
			name: "fee exceeds lte limit",
			input: DecimalStruct{
				Amount: decimal.NewFromInt(100),
				Fee:    decimal.NewFromFloat(1000.01),
				Limit:  nil,
			},
			expectedField: "fee",
			expectedTag:   "lte",
			expectedMsg:   "fee must be less than or equal to 1000",
		},
		{
			name: "negative optional pointer",
			input: DecimalStruct{
				Amount: decimal.NewFromInt(1),
				Fee:    decimal.NewFromInt(0),
				Limit:  decimalPtr(decimal.NewFromFloat(-10)),
			},
			expectedField: "limit",
			expectedTag:   "gte",
			expectedMsg:   "limit must be greater than or equal to 0",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validatorInstance.Validate(testCase.input)
			require.Error(t, err)

			var validationErrors validator.ValidationErrors
			ok := errors.As(err, &validationErrors)
			require.True(t, ok)

			found := findValidationError(validationErrors, testCase.expectedField)
			require.NotNil(t, found, "expected validation error for field %s", testCase.expectedField)
			require.Equal(t, testCase.expectedTag, found.Tag)
			require.Equal(t, testCase.expectedMsg, found.Message)
		})
	}
}

func decimalPtr(d decimal.Decimal) *decimal.Decimal {
	return &d
}

func findValidationError(errors validator.ValidationErrors, field string) *validator.ValidationError {
	for i := range errors {
		if errors[i].Field == field {
			return &errors[i]
		}
	}

	return nil
}
