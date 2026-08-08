package httpx

import (
	"context"
	"net/http"
	"time"
)

// Timeout bounds request-scoped work by attaching a deadline to the request
// context passed downstream. It does not itself abort a handler that never
// checks ctx.Done() or ctx.Err() — it makes context-aware calls (database
// queries, outbound HTTP) fail fast with context.DeadlineExceeded once the
// deadline passes, rather than being unbounded. d <= 0 disables the
// middleware entirely: context.WithTimeout with a non-positive duration
// would produce an already-expired context, cancelling every request
// immediately rather than "no limit".
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		if d <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
