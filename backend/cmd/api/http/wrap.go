package http

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

// wrap wraps a function with input and output types into an http.HandlerFunc.
//
// Before fn runs, the decoded input is validated whenever it satisfies
// Validable. Note that the check is a plain type assertion on In, so a Validate
// method declared on *T is not reached when In is T.
//
// Every failure — from decode, Validate, fn, or encode — is handed to
// writeError, which derives the response status and body from the error's errs
// classification (see errs and writeError) and logs the failure. A Validate
// error is classified as errs.Invalid first, unless it is already classified as
// something else, so any Validable is guaranteed a 400 response; return
// validation.Errors or an errs.Domain error from Validate for a client-visible
// message. Passing a nil logger is safe; writeError disables logging in that
// case.
func wrap[In, Out any](
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
			writeError(w, r, err, logger)
			return
		}

		if v, ok := any(in).(Validable); ok {
			if err := v.Validate(); err != nil {
				if !errors.Is(err, errs.Invalid) {
					err = fmt.Errorf("%w: %w", errs.Invalid, err)
				}
				writeError(w, r, err, logger)
				return
			}
		}

		out, err := fn(r.Context(), in)
		if err != nil {
			writeError(w, r, err, logger)
			return
		}

		err = encode(w, out)
		if err != nil {
			writeError(w, r, err, logger)
			return
		}
	}
}

// wrapNoInput wraps a function with no input and output types into an
// http.HandlerFunc. The request body is never read. See wrap for how failures
// map to responses.
func wrapNoInput[Out any](
	fn func(context.Context) (Out, error),
	encode func(http.ResponseWriter, Out) error,
	logger *slog.Logger,
) http.HandlerFunc {
	return wrap(
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
