package config

import "github.com/bramba2000/mrtutor/backend/validation"

// loader accumulates parse and validation failures across every setting so
// Load can report all of them at once instead of failing on the first bad
// value.
type loader struct {
	lookup func(string) (string, bool)
	errs   validation.Errors
}

// get reads key from the loader's lookup function, parses it, and validates
// it, recording any problem under key. An unset key yields def unparsed and
// unvalidated: defaults are compile-time constants the caller controls, so
// they are trusted rather than checked. This is what lets a setting default
// to its zero value while still rejecting that same zero value when a caller
// supplies it explicitly.
func get[T any](l *loader, key string, def T, parse func(string) (T, error), validators ...validation.Validator[T]) T {
	raw, ok := l.lookup(key)
	if !ok {
		return def
	}

	v, err := parse(raw)
	if err != nil {
		l.errs.Add(key, err)
		return def
	}

	l.errs.Add(key, validation.Validate(v, validators...)...)
	return v
}
