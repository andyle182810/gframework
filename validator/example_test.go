package validator_test

import (
	"fmt"

	"github.com/andyle182810/gframework/validator"
)

func ExampleDefaultRestValidator() {
	type CreateUserRequest struct {
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age"   validate:"gte=18"`
	}

	v := validator.DefaultRestValidator()

	err := v.Validate(&CreateUserRequest{Email: "not-an-email", Age: 16})
	fmt.Println(err)
	// Output: email must be a valid email address; age must be greater than or equal to 18
}
