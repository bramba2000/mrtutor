package web

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"
)

// ErrNotBuilt reports that the embedded assets carry no index.html — nobody
// has run the frontend build (the root Taskfile's "build" task).
var ErrNotBuilt = errors.New("frontend assets not built")

// Handler returns an http.Handler serving the embedded single-page
// application.
//
// When the build output is absent it still returns a non-nil handler — one
// that answers every request with 503 — together with ErrNotBuilt. That lets
// the caller decide whether a missing frontend is fatal (production) or
// merely a warning (development) without having to build its own
// placeholder handler for the "not built yet" case.
func Handler(logger *slog.Logger) (http.Handler, error) {
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		// Unreachable: "dist" is a literal matching the embed directive
		// above, not user input.
		return nil, fmt.Errorf("sub dist: %w", err)
	}
	return handlerFS(fsys, logger)
}

// handlerFS is Handler's testable core: unexported and fs.FS-parameterised
// so tests drive it with fstest.MapFS instead of depending on whether the
// real frontend happens to have been built.
func handlerFS(fsys fs.FS, logger *slog.Logger) (http.Handler, error) {
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		if logger != nil {
			logger.Warn("frontend assets not built", "error", err)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, ErrNotBuilt.Error(), http.StatusServiceUnavailable)
		}), ErrNotBuilt
	}

	sum := sha256.Sum256(index)
	etag := fmt.Sprintf(`"%x"`, sum)

	// serveIndex answers with index.html: the initial page load, and the SPA
	// fallback for any client-side route the router owns, not the file
	// system. no-cache (not immutable) because, unlike everything under
	// assets/, this file's name never changes when its content does — a
	// browser must always revalidate it.
	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", etag)
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		if name == "index.html" {
			serveIndex(w, r)
			return
		}

		info, err := fs.Stat(fsys, name)
		if err != nil || info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				// Never answer a missing script or stylesheet with HTML —
				// that turns a build/deploy bug into a silent, confusing
				// failure in the browser instead of a loud 404.
				http.NotFound(w, r)
				return
			}
			// Any other unknown path is a client-side route (e.g.
			// /courses/algebra-1.0), not a missing resource.
			serveIndex(w, r)
			return
		}

		if strings.HasPrefix(name, "assets/") {
			// Vite content-hashes every filename under assets/, so a given
			// name's bytes never change — safe to cache for a year.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeFileFS(w, r, fsys, name)
	}), nil
}
