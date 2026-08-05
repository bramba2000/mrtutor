package validation

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

// Validator is a function that takes a value of type T and
// returns an error if the value is invalid.
type Validator[T any] func(T) error

// Validate applies a list of validators to a value of type T
// and returns a slice of errors.
func Validate[T any](value T, validators ...Validator[T]) []error {
	var errs []error
	for _, validator := range validators {
		if err := validator(value); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Required checks if the value is not the zero value for its type.
//
// Consider using [NotNil] for non comparable types and pointers.
func Required[T comparable](value T) error {
	var zero T
	if value == zero {
		return errors.New("is required")
	}
	return nil
}

// NotNil checks if the pointer value is not nil.
//
// This is useful for non-comparable types and pointers. Consider using [Required]
// for comparable types.
func NotNil[T any](value *T) error {
	if value == nil {
		return errors.New("must not be nil")
	}
	return nil
}

// MinLength checks if the length of the value is at least min.
func MinLength[T ~string](min int) Validator[T] {
	return func(value T) error {
		if len(value) < min {
			return fmt.Errorf("must be at least %d characters long", min)
		}
		return nil
	}
}

// MaxLength checks if the length of the value is at most max.
func MaxLength[T ~string](max int) Validator[T] {
	return func(value T) error {
		if len(value) > max {
			return fmt.Errorf("must be at most %d characters long", max)
		}
		return nil
	}
}

// MinMaxLength checks if the length of the value is between min and max (inclusive).
func MinMaxLength[T ~string](min, max int) Validator[T] {
	return func(value T) error {
		if len(value) < min || len(value) > max {
			return fmt.Errorf("must be between %d and %d characters long", min, max)
		}
		return nil
	}
}

// NotEmpty checks if the value (string or slice) is not empty (len is zero).
func NotEmpty[T ~string | ~[]any](value T) error {
	if len(value) == 0 {
		return errors.New("must not be empty")
	}
	return nil
}

// NotBlank checks if the string value is not blank (len is zero or only whitespace).
func NotBlank(value string) error {
	if len(value) == 0 || len(strings.TrimSpace(value)) == 0 {
		return errors.New("must not be blank")
	}
	return nil
}

// Email checks if the string value is a valid email address.
func Email(value string) error {
	_, err := mail.ParseAddress(value)
	if err != nil {
		return errors.New("must be a valid email address")
	}
	return nil
}
