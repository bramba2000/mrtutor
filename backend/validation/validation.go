package validation

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

func Optional[T comparable](value T, validators ...Validator[T]) []error {
	var zero T
	if value == zero {
		return nil
	}
	return Validate(value, validators...)
}
