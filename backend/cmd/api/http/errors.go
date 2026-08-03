package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/validation"
)

type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []fieldEntry `json:"fields,omitempty"`
}

type fieldEntry struct {
	Name    string `json:"field"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	status := statusFor(err)

	if status >= 500 {
		logger.Error("request failed", "error", err, "method", r.Method, "url", r.URL.Path)
	} else {
		logger.Info("request rejected", "error", err, "method", r.Method, "url", r.URL.Path)
	}

	body := errorBody{Code: "internal", Message: "internal error"}

	var d interface {
		Code() string
		Public() string
	}

	if isJsonDecodingError(err) {
		body.Code = "invalid.json"
		body.Message = "invalid JSON"
	} else if errors.As(err, &d) {
		body.Code = d.Code()
		body.Message = d.Public()
	}

	if v, ok := errors.AsType[validation.Errors](err); ok {
		body.Fields = make([]fieldEntry, 0, len(v))
		for _, field := range v.Fields() {
			for _, entry := range v[field] {
				body.Fields = append(body.Fields, fieldEntry{
					Name:    field,
					Message: entry.Error(),
				})
			}
		}
	}

	if status < 200 || status >= 300 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			logger.Error("failed to write error response", "error", err, "method", r.Method, "url", r.URL.Path)
		}
	}
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, errs.NotFound):
		return http.StatusNotFound
	case errors.Is(err, errs.Unauthenticated):
		return http.StatusUnauthorized
	case errors.Is(err, errs.Forbidden):
		return http.StatusForbidden
	case errors.Is(err, errs.Invalid):
		return http.StatusBadRequest
	case errors.Is(err, errs.Conflict):
		return http.StatusConflict
	case errors.Is(err, errs.FailedPrecondition):
		return http.StatusPreconditionFailed
	case isJsonDecodingError(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func isJsonDecodingError(err error) bool {
	var syntaxErr *json.SyntaxError
	var unmarshalTypeErr *json.UnmarshalTypeError

	return errors.As(err, &syntaxErr) || errors.As(err, &unmarshalTypeErr)
}
