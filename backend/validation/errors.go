package validation

import "strings"

// Errors is a map of field names to slices of errors.
// It is used to represent validation errors for multiple fields.
//
// When created manually with a map, consider using the Filters() method to remove any nil errors and empty slices.
type Errors map[string][]error

func filterNilErrors(errs []error) []error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

// Filters removes any nil errors and empty slices from the Errors map.
// If the resulting map is empty, it returns nil.
func (in Errors) Filters() Errors {
	for key, errs := range in {
		filtered := filterNilErrors(errs)
		if len(filtered) > 0 {
			in[key] = filtered
		} else {
			delete(in, key)
		}
	}
	if len(in) == 0 {
		return nil
	}
	return in
}

func (in Errors) Error() string {
	var result strings.Builder
	for key, errs := range in {
		for _, err := range errs {
			result.WriteString(key)
			result.WriteString(": ")
			result.WriteString(err.Error())
			result.WriteString("\n")
		}
	}
	return result.String()
}

// Validator is a function that takes a value of type T and
// returns an error if the value is invalid.
type Validator[T any] func(T) error

// Validate applies a list of validators to a value of type T
// and returns a slice of errors. 
func Validate[T any](value T, validators ...Validator[T]) []error {
	errs := make([]error, 0, len(validators))
	for _, validator := range validators {
		if err := validator(value); err != nil {
			errs = append(errs, err)
		}
	}
	return filterNilErrors(errs)
}
