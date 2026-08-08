package httpx

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					switch v := rec.(type) {
					case string:
						logger.Error("panic recovered", "error", v, "stack", slog.String("stack", string(debug.Stack())))
						WriteError(w, r, errors.New(v), logger)
					case error:
						if errors.Is(v, http.ErrAbortHandler) {
							panic(v)
						}
						logger.Error("panic recovered", "error", v, "stack", slog.String("stack", string(debug.Stack())))
						WriteError(w, r, v, logger)
					default:
						logger.Error("panic recovered", "error", v, "stack", slog.String("stack", string(debug.Stack())))
						WriteError(w, r, errors.New("unknown panic"), logger)
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
