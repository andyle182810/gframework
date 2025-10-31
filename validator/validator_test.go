package validator_test

import (
	"errors"
	"testing"

	"github.com/andyle182810/gframework/validator"
	gvalidator "github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestStruct struct {
	Name  string `validate:"required,min=3,max=20"`
	Email string `validate:"required,email"`
	Age   int    `validate:"gte=18,lte=100"`
}

func TestDefaultRestValidator(t *testing.T) {
	t.Parallel()

	validatorInstance := validator.DefaultRestValidator()
	assert.NotNil(t, validatorInstance)
	assert.NotNil(t, validatorInstance.Validator)
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
	assert.NoError(t, err)
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

	var validationErrors gvalidator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	assert.True(t, ok)
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

	var validationErrors gvalidator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	assert.True(t, ok)
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

	var validationErrors gvalidator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	assert.True(t, ok)
}
