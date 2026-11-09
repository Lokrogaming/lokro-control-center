package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFallbackIsUsedForUnmatchedPaths(t *testing.T) {
	router := NewRouter()
	router.GET("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy"))
	})
	router.SetNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("spa:" + r.URL.Path))
	}))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/servers/7", nil))
	if resp.Code != http.StatusOK || resp.Body.String() != "spa:/servers/7" {
		t.Fatalf("expected SPA fallback, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestFallbackIsNotUsedForMatchedRoutes(t *testing.T) {
	router := NewRouter()
	router.GET("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy"))
	})
	router.SetNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("fallback must not be called for a matched route")
	}))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/health", nil))
	if resp.Code != http.StatusOK || resp.Body.String() != "healthy" {
		t.Fatalf("unexpected response %d: %s", resp.Code, resp.Body.String())
	}
}

func TestWrongMethodReturns405WithoutFallback(t *testing.T) {
	router := NewRouter()
	router.GET("/health", func(w http.ResponseWriter, r *http.Request) {})
	router.SetNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("fallback must not be called when the path matches another method")
	}))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/health", nil))
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
	if allow := resp.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected Allow: GET, got %q", allow)
	}
}
