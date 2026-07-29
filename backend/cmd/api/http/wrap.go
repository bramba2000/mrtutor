package http

import (
	"context"
	"log/slog"
	"net/http"
)

// Response bodies for the failures whose cause must not reach the client.
const (
	msgInvalidBody   = "invalid request body"
	msgInternalError = "internal server error"
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
// Failures map to responses as follows:
//
//	decode error     400, body "invalid request body"
//	Validate error   400, body carrying the validation message
//	fn error         500, body "internal server error"
//	encode error     500, body "internal server error"
//
// Only validation messages are forwarded, being written for clients in the first
// place. Decoder errors are withheld because they name Go types and struct
// fields, and fn and encode errors because they are internal; both are logged
// instead. Passing a nil logger disables that logging.
func wrap[In, Out any](
	decode func(*http.Request) (In, error),
	fn func(context.Context, In) (Out, error),
	encode func(http.ResponseWriter, Out) error,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := decode(r)
		if err != nil {
			logError(r.Context(), logger, "failed to decode request", err)
			http.Error(w, msgInvalidBody, http.StatusBadRequest)
			return
		}

		if v, ok := any(in).(Validable); ok {
			if err := v.Validate(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		out, err := fn(r.Context(), in)
		if err != nil {
			logError(r.Context(), logger, "request handler failed", err)
			http.Error(w, msgInternalError, http.StatusInternalServerError)
			return
		}

		err = encode(w, out)
		if err != nil {
			logError(r.Context(), logger, "failed to encode response", err)
			// A no-op when the encoder already committed a status, which is
			// unavoidable: the response is on its way out by then.
			http.Error(w, msgInternalError, http.StatusInternalServerError)
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

// logError reports err at error level, tolerating a nil logger.
func logError(ctx context.Context, logger *slog.Logger, msg string, err error) {
	if logger == nil {
		return
	}
	logger.ErrorContext(ctx, msg, "error", err)
}
