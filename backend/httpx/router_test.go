package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bramba2000/mrtutor/backend/httpx"
)

func TestRouter_ServeHTTP(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	m := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" && r.URL.Path == "/test" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	tt := []struct {
		name    string
		router  *httpx.Router
		request *http.Request
	}{
		{
			name: "Serve handler with no prefix",
			router: func() *httpx.Router {
				r := httpx.NewRouter("")
				r.Handle("POST /test", h)
				return r
			}(),
			request: httptest.NewRequest("POST", "/test", http.NoBody),
		},
		{
			name: "Serve handler with prefix",
			router: func() *httpx.Router {
				r := httpx.NewRouter("/auth")
				r.Handle("POST /test", h)
				return r
			}(),
			request: httptest.NewRequest("POST", "/auth/test", http.NoBody),
		},
		{
			name: "Serve handler with middlewares",
			router: func() *httpx.Router {
				r := httpx.NewRouter("", m)
				r.Handle("POST /test", http.NotFoundHandler())
				return r
			}(),
			request: httptest.NewRequest("POST", "/test", http.NoBody),
		},
		{
			name: "Serve handler with correct order",
			router: func() *httpx.Router {
				mws := []httpx.Middleware{
					func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							r = r.WithContext(context.WithValue(r.Context(), "key", true))
							next.ServeHTTP(w, r)
						})
					},
					func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if r.Context().Value("key") == true {
								w.WriteHeader(http.StatusOK)
								return
							}
							next.ServeHTTP(w, r)
						})
					},
				}
				r := httpx.NewRouter("", mws...)
				r.Handle("POST /test", http.NotFoundHandler())
				return r
			}(),
			request: httptest.NewRequest("POST", "/test", http.NoBody),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Create a ResponseRecorder to record the response
			rr := httptest.NewRecorder()

			// Call the ServeHTTP method of the router with the request and ResponseRecorder
			tc.router.ServeHTTP(rr, tc.request)

			// You can add assertions here to check the response status code, headers, body, etc.
			// For example:
			if rr.Code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
			}
		})
	}
}
