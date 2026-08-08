package web

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":           {Data: []byte("<html>index</html>")},
		"favicon.ico":          {Data: []byte("icon")},
		"assets/index-abc.js":  {Data: []byte("console.log(1)")},
		"assets/index-abc.css": {Data: []byte("body{}")},
	}
}

func TestHandlerFS_NotBuilt(t *testing.T) {
	discard := slog.New(slog.DiscardHandler)
	h, err := handlerFS(fstest.MapFS{}, discard)
	if err == nil {
		t.Fatal("expected ErrNotBuilt, got nil")
	}
	if err != ErrNotBuilt {
		t.Fatalf("error = %v, want ErrNotBuilt", err)
	}
	if h == nil {
		t.Fatal("expected a usable handler even when the frontend isn't built")
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandlerFS(t *testing.T) {
	discard := slog.New(slog.DiscardHandler)
	h, err := handlerFS(testFS(), discard)
	if err != nil {
		t.Fatalf("handlerFS: %v", err)
	}

	t.Run("serves index at root", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if w.Body.String() != "<html>index</html>" {
			t.Errorf("body = %q", w.Body.String())
		}
		if got := w.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", got)
		}
	})

	t.Run("SPA fallback serves index for an unknown client route", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/courses/algebra-1.0", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if w.Body.String() != "<html>index</html>" {
			t.Errorf("body = %q, want the SPA index", w.Body.String())
		}
	})

	t.Run("existing asset is served with an immutable cache header", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/index-abc.js", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Errorf("Cache-Control = %q", got)
		}
	})

	t.Run("missing asset 404s instead of falling back to index", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/gone.js", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		if w.Body.String() == "<html>index</html>" {
			t.Error("a missing asset must never be answered with the SPA index")
		}
	})

	t.Run("non-asset file is served with a no-cache header", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", got)
		}
	})

	t.Run("POST is rejected with 405 and Allow", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
		if got := w.Header().Get("Allow"); got != "GET, HEAD" {
			t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
		}
	})

	t.Run("matching If-None-Match returns 304", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		etag := w.Header().Get("ETag")
		if etag == "" {
			t.Fatal("expected an ETag on the index response")
		}

		w2 := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", etag)
		h.ServeHTTP(w2, req)
		if w2.Code != http.StatusNotModified {
			t.Errorf("status = %d, want %d", w2.Code, http.StatusNotModified)
		}
	})
}
