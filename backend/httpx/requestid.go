package httpx

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type requestIDKey struct{}

var startPrefix string

func RequestID() Middleware {
	startPrefix = strconv.FormatInt(time.Now().Unix(), 36)
	var counter atomic.Uint64

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var id string
			if rid := r.Header.Get("X-Request-ID"); validateRequestID(rid) {
				id = rid
			} else {
				id = generateRequestID(&counter)
			}
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	return id, ok
}

const maxRequestIDLength = 64

func validateRequestID(id string) bool {
	if len(id) == 0 || len(id) > maxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == '-' || c == '_' || c == '.':
		default:
			return false
		}
	}
	return true
}

func generateRequestID(counter *atomic.Uint64) string {
	id := counter.Add(1)
	buf := make([]byte, 0, len(startPrefix)+1+20)
	buf = append(buf, startPrefix...)
	buf = append(buf, '-')
	buf = strconv.AppendUint(buf, id, 10)
	return string(buf)
}
