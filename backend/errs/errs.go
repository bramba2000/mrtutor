package errs

import "errors"

var (
	NotFound           = errors.New("not found")
	Conflict           = errors.New("conflict")
	Invalid            = errors.New("invalid")
	Unauthenticated    = errors.New("unauthenticated")
	Forbidden          = errors.New("forbidden")
	FailedPrecondition = errors.New("failed precondition")
)
