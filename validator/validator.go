package validator

import "github.com/go-playground/validator/v10"

type Validator struct {
	Validator *validator.Validate
}

func DefaultRestValidator() *Validator {
	r := &Validator{Validator: validator.New()}

	return r
}

func (v *Validator) Validate(i any) error {
	if err := v.Validator.Struct(i); err != nil {
		return err
	}

	return nil
}
