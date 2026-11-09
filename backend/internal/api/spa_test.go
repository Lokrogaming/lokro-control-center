package api

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lokro-control-center/backend/internal/web"
)

func setupFrontendFallback(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<!doctype html><div id=\"root\"></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app-abc.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFrontendFallbackWithAPIRoutes(t *testing.T) {
	_, _, router := setupRouter(t)
	root := t.TempDir()
	setupFrontendFallback(t, root)
	router.SetNotFoundHandler(web.NewFrontend(root, log.New(io.Discard, "", 0)))

	tests := []struct {
		name         string
		path         string
		wantCode     int
		wantContains string
	}{
		{name: "root serves SPA", path: "/", wantCode: http.StatusOK, wantContains: "id=\"root\""},
		{name: "client route serves SPA", path: "/servers/7", wantCode: http.StatusOK, wantContains: "id=\"root\""},
		{name: "asset served", path: "/assets/app-abc.js", wantCode: http.StatusOK, wantContains: "console.log('ok')"},
		{name: "missing asset is 404", path: "/assets/missing.js", wantCode: http.StatusNotFound, wantContains: "not found"},
		{name: "unknown API is JSON 404", path: "/api/does-not-exist", wantCode: http.StatusNotFound, wantContains: "not found"},
		{name: "unauthenticated API stays API", path: "/api/auth/me", wantCode: http.StatusUnauthorized, wantContains: "not authenticated"},
		{name: "health stays healthy", path: "/health", wantCode: http.StatusOK, wantContains: "ok"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := execute(router, httptestRequest(http.MethodGet, tc.path, nil))
			if resp.Code != tc.wantCode {
				t.Fatalf("%s: expected %d, got %d: %s", tc.path, tc.wantCode, resp.Code, resp.Body.String())
			}
			if !strings.Contains(strings.ToLower(resp.Body.String()), strings.ToLower(tc.wantContains)) {
				t.Fatalf("%s: expected body to contain %q, got %q", tc.path, tc.wantContains, resp.Body.String())
			}
			if strings.Contains(resp.Body.String(), "id=\"root\"") &&
				(strings.HasPrefix(tc.path, "/api/") || tc.path == "/health") {
				t.Fatalf("%s: backend route must not return the SPA", tc.path)
			}
		})
	}
}
