package validation

import (
	"errors"
	"fmt"
	"strings"
)

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
func MinLength[T ~string | ~[]any](value T, min int) Validator[T] {
	return func(value T) error {
		if len(value) < min {
			return fmt.Errorf("must be at least %d characters long", min)
		}
		return nil
	}
}

// MaxLength checks if the length of the value is at most max.
func MaxLength[T ~string | ~[]any](value T, max int) Validator[T] {
	return func(value T) error {
		if len(value) > max {
			return fmt.Errorf("must be at most %d characters long", max)
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
