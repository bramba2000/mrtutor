package httpx

import (
	"net/http"
	"slices"
	"strings"
)

type Router struct {
	mux    *http.ServeMux
	prefix string
	mws    []Middleware
}

func NewRouter(prefix string, mws ...Middleware) *Router {
	return &Router{
		mux:    http.NewServeMux(),
		prefix: prefix,
		mws:    mws,
	}
}

// Handle ovverrides the [http.ServeMux.Handle] method, applying the router prefix and middleware chan.
func (r *Router) Handle(pattern string, handler http.Handler) {
	method := http.MethodGet
	path := pattern
	if strings.Contains(pattern, " ") {
		parts := strings.Split(pattern, " ")
		method = parts[0]
		path = parts[1]
	}

	r.mux.Handle(method+" "+r.prefix+path, Chain(handler, r.mws...))
}

// Group creates a new sub-router with the given prefix and middleware chain.
//
// The new router will inherit the parent router's prefix and middleware chain, allowing
// for nested routing and middleware application.
func (r *Router) Group(prefix string, mws ...Middleware) *Router {
	return &Router{
		mux:    r.mux,
		prefix: r.prefix + prefix,
		mws:    append(slices.Clone(r.mws), mws...),
	}
}

// ServeHTTP implements the [http.Handler] interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// HandleFunc ovverides the [http.ServeMux.HandleFunc] method, applying the router prefix and middleware chain.
func (r *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.Handle(pattern, http.HandlerFunc(handler))
}

func (r *Router) Use(mws ...Middleware) {
	r.mws = append(r.mws, mws...)
}
