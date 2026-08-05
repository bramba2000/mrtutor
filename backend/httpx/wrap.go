package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bramba2000/mrtutor/backend/errs"
)

type Validable interface {
	Validate() error
}

// Wrap turns a function with input and output types into an http.HandlerFunc,
// validating the decoded input before fn runs.
//
// In is constrained to Validable so validation is enforced by the compiler
// rather than by a runtime assertion that can fail open. If Validate is
// declared on *T, Wrap[T] does not compile — which is the point; use
// Wrap[*T] (and decode to a *T) when the receiver must be a pointer.
//
// Every failure — from decode, Validate, fn, or encode — is handed to
// WriteError, which derives the response status and body from the error's errs
// classification (see errs and WriteError) and logs the failure. A Validate
// error is classified as errs.Invalid first, unless it is already classified
// as something else, so any Validable is guaranteed a 400 response; return
// validation.Errors or an errs.Domain error from Validate for a client-visible
// message. Passing a nil logger is safe; WriteError disables logging in that
// case.
func Wrap[In Validable, Out any](
	decode func(*http.Request) (In, error),
	fn func(context.Context, In) (Out, error),
	encode func(http.ResponseWriter, Out) error,
	logger *slog.Logger,
) http.HandlerFunc {
	return WrapUnvalidated(
		func(r *http.Request) (In, error) {
			in, err := decode(r)
			if err != nil {
				return in, err
			}
			if err := in.Validate(); err != nil {
				if !errors.Is(err, errs.Invalid) {
					err = fmt.Errorf("%w: %w", errs.Invalid, err)
				}
				return in, err
			}
			return in, nil
		},
		fn,
		encode,
		logger,
	)
}

// WrapUnvalidated is Wrap for inputs with nothing to validate. The name is
// deliberately unpleasant: reach for Wrap unless you can say why not.
func WrapUnvalidated[In, Out any](
	decode func(*http.Request) (In, error),
	fn func(context.Context, In) (Out, error),
	encode func(http.ResponseWriter, Out) error,
	logger *slog.Logger,
) http.HandlerFunc {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := decode(r)
		if err != nil {
			WriteError(w, r, err, logger)
			return
		}

		out, err := fn(r.Context(), in)
		if err != nil {
			WriteError(w, r, err, logger)
			return
		}

		err = encode(w, out)
		if err != nil {
			WriteError(w, r, err, logger)
			return
		}
	}
}

// WrapNoInput wraps a function with no input and output types into an
// http.HandlerFunc. The request body is never read. See Wrap for how failures
// map to responses.
func WrapNoInput[Out any](
	fn func(context.Context) (Out, error),
	encode func(http.ResponseWriter, Out) error,
	logger *slog.Logger,
) http.HandlerFunc {
	return WrapUnvalidated(
		func(r *http.Request) (struct{}, error) {
			return struct{}{}, nil
		},
		func(ctx context.Context, _ struct{}) (Out, error) {
			return fn(ctx)
		},
		encode,
		logger,
	)
}
