package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

type Validator struct {
	Validator *validator.Validate
}

type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	msgs := make([]string, 0, len(v))
	for _, err := range v {
		msgs = append(msgs, err.Message)
	}

	return strings.Join(msgs, "; ")
}

func DefaultRestValidator() *Validator {
	v := validator.New()

	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		const maxSplits = 2
		name := strings.SplitN(fld.Tag.Get("json"), ",", maxSplits)[0]

		if name == "-" {
			return ""
		}

		return name
	})

	v.RegisterCustomTypeFunc(func(field reflect.Value) any {
		if val, ok := field.Interface().(decimal.Decimal); ok {
			f, _ := val.Float64()

			return f
		}

		return nil
	}, decimal.Decimal{})

	return &Validator{Validator: v}
}

func (v *Validator) Validate(i any) error {
	if err := v.Validator.Struct(i); err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			return v.formatValidationErrors(validationErrs)
		}

		return err
	}

	return nil
}

func (v *Validator) formatValidationErrors(errs validator.ValidationErrors) ValidationErrors {
	validationErrs := make(ValidationErrors, 0, len(errs))

	for _, err := range errs {
		field := err.Field()
		if field == "" {
			field = err.StructField()
		}

		validationErrs = append(validationErrs, ValidationError{
			Field:   field,
			Tag:     err.Tag(),
			Value:   fmt.Sprintf("%v", err.Value()),
			Message: v.generateErrorMessage(field, err),
		})
	}

	return validationErrs
}

func (v *Validator) generateErrorMessage(field string, err validator.FieldError) string {
	msg := v.getSimpleErrorMessage(field, err.Tag())
	if msg != "" {
		return msg
	}

	return v.getParameterizedErrorMessage(field, err)
}

func (v *Validator) getSimpleErrorMessage(field, tag string) string {
	switch tag {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "url":
		return field + " must be a valid URL"
	case "uri":
		return field + " must be a valid URI"
	case "alphanum":
		return field + " must contain only alphanumeric characters"
	case "numeric":
		return field + " must be numeric"
	case "uuid":
		return field + " must be a valid UUID"
	default:
		return ""
	}
}

func (v *Validator) getParameterizedErrorMessage(field string, err validator.FieldError) string {
	param := err.Param()
	tag := err.Tag()

	switch tag {
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, param)
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, param)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, param)
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, param)
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, param)
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, param)
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, param)
	default:
		return fmt.Sprintf("%s failed validation on '%s'", field, err.Tag())
	}
}

func (v *Validator) RegisterCustomValidation(tag string, fn validator.Func) error {
	return v.Validator.RegisterValidation(tag, fn)
}

func (v *Validator) RegisterStructValidation(fn validator.StructLevelFunc, types ...any) {
	v.Validator.RegisterStructValidation(fn, types...)
}
