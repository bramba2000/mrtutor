package errs

type domainError struct {
	code    string
	message string
	kind    error
}

func (e *domainError) Error() string  { return e.message }
func (e *domainError) Code() string   { return e.code }
func (e *domainError) Unwrap() error  { return e.kind }
func (e *domainError) Public() string { return e.message }

func Domain(code, msg string, kind error) *domainError {
	return &domainError{code: code, message: msg, kind: kind}
}
