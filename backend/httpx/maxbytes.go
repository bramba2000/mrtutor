package httpx

import "net/http"

func MaxBytes() Middleware {
	return func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, 1<<20) // Limit request body to 1MB
	}
}
