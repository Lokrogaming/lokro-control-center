package web

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testIndex = "<!doctype html><html><body><div id=\"root\"></div></body></html>"

func newTestFrontend(t *testing.T) (*Frontend, string) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(testIndex), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app-abc123.js"), []byte("console.log('app');"), 0o644); err != nil {
		t.Fatal(err)
	}

	return NewFrontend(root, log.New(io.Discard, "", 0)), root
}

func doRequest(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestServesIndexAtRoot(t *testing.T) {
	frontend, _ := newTestFrontend(t)

	resp := doRequest(frontend, http.MethodGet, "/")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "id=\"root\"") {
		t.Fatalf("expected index.html, got: %s", resp.Body.String())
	}
	if ct := resp.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html, got %q", ct)
	}
	if cc := resp.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("expected no-cache for index.html, got %q", cc)
	}
}

func TestServesSPAFallbackForClientRoutes(t *testing.T) {
	frontend, _ := newTestFrontend(t)

	for _, target := range []string{"/login", "/servers/42", "/uptime"} {
		resp := doRequest(frontend, http.MethodGet, target)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", target, resp.Code)
		}
		if !strings.Contains(resp.Body.String(), "id=\"root\"") {
			t.Fatalf("%s: expected SPA index, got: %s", target, resp.Body.String())
		}
	}
}

func TestServesStaticAssets(t *testing.T) {
	frontend, _ := newTestFrontend(t)

	resp := doRequest(frontend, http.MethodGet, "/assets/app-abc123.js")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "console.log('app')") {
		t.Fatalf("unexpected asset body: %s", resp.Body.String())
	}
	if ct := resp.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("expected javascript content type, got %q", ct)
	}
	if cc := resp.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("expected immutable cache header, got %q", cc)
	}
	if ct := resp.Header().Get("X-Content-Type-Options"); ct != "nosniff" {
		t.Fatalf("expected nosniff, got %q", ct)
	}
}

func TestMissingAssetIsNotSPAFallback(t *testing.T) {
	frontend, _ := newTestFrontend(t)

	for _, target := range []string{"/assets/missing-123.js", "/favicon.ico"} {
		resp := doRequest(frontend, http.MethodGet, target)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404, got %d: %s", target, resp.Code, resp.Body.String())
		}
		if strings.Contains(resp.Body.String(), "id=\"root\"") {
			t.Fatalf("%s: missing asset must not return index.html", target)
		}
	}
}

func TestAPIPathsNeverFallBackToSPA(t *testing.T) {
	frontend, _ := newTestFrontend(t)

	for _, target := range []string{"/api", "/api/unknown", "/api/users/1", "/health"} {
		resp := doRequest(frontend, http.MethodGet, target)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404, got %d: %s", target, resp.Code, resp.Body.String())
		}
		if strings.Contains(resp.Body.String(), "id=\"root\"") {
			t.Fatalf("%s: backend path must not return index.html", target)
		}
		if ct := resp.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("%s: expected JSON 404, got content type %q", target, ct)
		}
	}
}

func TestRejectsTraversal(t *testing.T) {
	frontend, root := newTestFrontend(t)
	outsideDir := filepath.Dir(root)
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	targets := []string{"/../secret.txt", "/%2e%2e/secret.txt", "/..\\secret.txt", "/assets/../../secret.txt"}
	for _, target := range targets {
		resp := doRequest(frontend, http.MethodGet, target)
		if resp.Code == http.StatusOK {
			t.Fatalf("%s: traversal must not succeed, got body: %s", target, resp.Body.String())
		}
		if strings.Contains(resp.Body.String(), "top secret") {
			t.Fatalf("%s: traversal leaked file contents", target)
		}
	}
}

func TestRejectsSymlinkEscape(t *testing.T) {
	frontend, root := newTestFrontend(t)
	outside := filepath.Join(filepath.Dir(root), "secret.txt")
	if err := os.WriteFile(outside, []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	resp := doRequest(frontend, http.MethodGet, "/link.txt")
	if resp.Code == http.StatusOK || strings.Contains(resp.Body.String(), "top secret") {
		t.Fatalf("symlink escape must not be served, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestRejectsUnsupportedMethods(t *testing.T) {
	frontend, _ := newTestFrontend(t)

	resp := doRequest(frontend, http.MethodPost, "/login")
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
	if allow := resp.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("expected Allow header, got %q", allow)
	}
}

func TestMissingBuildReturnsServiceUnavailableButKeepsAPIContract(t *testing.T) {
	frontend := NewFrontend(filepath.Join(t.TempDir(), "does-not-exist"), log.New(io.Discard, "", 0))

	resp := doRequest(frontend, http.MethodGet, "/")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing build, got %d", resp.Code)
	}

	apiResp := doRequest(frontend, http.MethodGet, "/api/unknown")
	if apiResp.Code != http.StatusNotFound {
		t.Fatalf("expected API 404, got %d", apiResp.Code)
	}
}
