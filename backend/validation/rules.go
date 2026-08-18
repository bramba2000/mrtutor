package validation

import (
	"cmp"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
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

// Min checks if the value is at least min.
func Min[T cmp.Ordered](min T) Validator[T] {
	return func(value T) error {
		if value < min {
			return fmt.Errorf("must be at least %v", min)
		}
		return nil
	}
}

// Max checks if the value is at most max.
func Max[T cmp.Ordered](max T) Validator[T] {
	return func(value T) error {
		if value > max {
			return fmt.Errorf("must be at most %v", max)
		}
		return nil
	}
}

// MinMax checks if the value is between min and max (inclusive).
func MinMax[T cmp.Ordered](min, max T) Validator[T] {
	return func(value T) error {
		if value < min || value > max {
			return fmt.Errorf("must be between %v and %v", min, max)
		}
		return nil
	}
}

// OneOf checks if the value is one of the allowed values.
func OneOf[T comparable](allowed ...T) Validator[T] {
	return func(value T) error {
		if !slices.Contains(allowed, value) {
			return fmt.Errorf("must be one of %v", allowed)
		}
		return nil
	}
}

// Phone checks if the string value is a valid phone number in E.164 format.
func Phone(value string) error {
	// Parse E.164 phone number format: +[country code][subscriber number including area code]
	regex := regexp.MustCompile(`^\+(3[0-469]|4[0-13-9]|7|8[1-4629]|9[0-58])\d{6,14}$`)
	matched := regex.MatchString(value)
	if !matched {
		return errors.New("must be a valid phone number")
	}
	return nil
}

// Date checks if the string value is a valid date in YYYY-MM-DD format.
func Date(value string) error {
	// Parse date format: YYYY-MM-DD
	regex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	matched := regex.MatchString(value)
	if !matched {
		return errors.New("must be a valid date in YYYY-MM-DD format")
	}
	return nil
}
