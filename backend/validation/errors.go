package validation

import (
	"slices"
	"strings"

	"github.com/bramba2000/mrtutor/backend/errs"
)

// Errors is a map of field names to slices of errors.
// It is used to represent validation errors for multiple fields.
//
// When created manually with a map, consider using the Filters() method to remove any nil errors and empty slices.
type Errors map[string][]error

func (in Errors) Add(key string, errs ...error) {
	f := filterNil(errs)
	if len(f) == 0 {
		return
	}
	if in[key] == nil {
		in[key] = f
	} else {
		in[key] = append(in[key], f...)
	}
}

func (in Errors) Filtered() Errors {
	out := make(Errors, len(in))
	for key, errs := range in {
		f := filterNil(errs)
		if len(f) > 0 {
			out[key] = f
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (in Errors) Err() error {
	filtered := in.Filtered()
	if filtered == nil {
		return nil
	}
	return filtered
}

func (in Errors) Fields() []string {
	fields := make([]string, 0, len(in))
	for key := range in {
		fields = append(fields, key)
	}
	slices.Sort(fields)
	return fields
}

func (in Errors) Error() string {
	if len(in) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("validation errors: ")
	for i, field := range in.Fields() {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(field)
		b.WriteString(": ")
		for j, err := range in[field] {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString(err.Error())
		}
	}
	return b.String()
}

func (in Errors) Is(target error) bool {
	return target == errs.Invalid
}

func (in Errors) Merge(other Errors) {
	if other == nil {
		return
	}
	for key, errs := range other {
		in.Add(key, errs...)
	}
}

func filterNil(errs []error) []error {
	out := errs[:0:0]
	for _, err := range errs {
		if err != nil {
			out = append(out, err)
		}
	}
	return out
}
