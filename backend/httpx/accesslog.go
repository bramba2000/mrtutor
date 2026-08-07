package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

type recorder struct {
	http.ResponseWriter
	code int
}

func (w *recorder) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

// AccessLog creates a middleware that logs HTTP requests using the provided logger.
func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start, rec := time.Now(), &recorder{ResponseWriter: w, code: 0}
			next.ServeHTTP(rec, r)
			id, _ := RequestIDFrom(r.Context())
			logger.Info("request",
				slog.String("requestID", id),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.code),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}
