package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lokro-control-center/backend/internal/auth"
	"lokro-control-center/backend/internal/config"
	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/mux"
	"lokro-control-center/backend/internal/permissions"
	"lokro-control-center/backend/internal/models"
	"lokro-control-center/backend/internal/audit"
	"lokro-control-center/backend/internal/uptime"
	"log"
)

func setupRouter(t *testing.T) (db.Store, *auth.Service, *mux.Router) {
	store, err := db.NewJSONFileStore(t.TempDir() + "/api_test.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		SessionSecret:     "testsecret123456789012345678901234567890ab",
		SessionDuration:   24 * time.Hour,
		CookieSecure:      false,
		CookieSameSite:    "Lax",
		PBKDF2Iterations:  1000,
		MaxFailedLogins:   5,
		LockoutDuration:   15 * time.Minute,
	}
	authSvc := auth.NewService(store, cfg)
	permEngine := permissions.NewEngine(store)
	if err := permEngine.EnsureBasePermissions(); err != nil {
		t.Fatal(err)
	}
	if err := permEngine.EnsureDefaultRoleGrants(); err != nil {
		t.Fatal(err)
	}
	auditLog := audit.NewLogger(store)
	uptimeSvc := uptime.NewService(store, log.New(io.Discard, "", 0))
	logger := log.New(io.Discard, "", 0)
	router := mux.NewRouter()
	Register(router, store, authSvc, permEngine, auditLog, uptimeSvc, cfg, logger)

	hash, _ := authSvc.HashPassword("SuperSecret123!")
	if _, err := store.CreateUser("admin", "admin@example.com", hash, models.RoleSuperAdmin, true); err != nil {
		t.Fatal(err)
	}
	return store, authSvc, router
}

func httptestRequest(method, path string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func execute(router *mux.Router, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func setCookieHeader(resp *httptest.ResponseRecorder) string {
	return resp.Header().Get("Set-Cookie")
}

func reqWithCookie(method, path, cookie string, body io.Reader) *http.Request {
	req := httptestRequest(method, path, body)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	return req
}

func extractSessionValue(setCookie string) string {
	// Set-Cookie: lokro_session=<value>; Path=/; ...
	parts := strings.Split(setCookie, ";")
	if len(parts) == 0 {
		return ""
	}
	first := parts[0]
	kv := strings.SplitN(first, "=", 2)
	if len(kv) != 2 {
		return ""
	}
	return kv[1]
}

func TestLoginAndMe(t *testing.T) {
	_, authSvc, router := setupRouter(t)

	body := strings.NewReader(`{"username":"admin","password":"SuperSecret123!"}`)
	resp := execute(router, httptestRequest(http.MethodPost, "/api/auth/login", body))
	if resp.Code != 200 {
		t.Fatalf("login expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	cookie := setCookieHeader(resp)
	if cookie == "" {
		t.Fatal("expected Set-Cookie on login")
	}

	req2 := reqWithCookie(http.MethodGet, "/api/auth/me", cookie, nil)
	resp2 := execute(router, req2)
	if resp2.Code != 200 {
		t.Fatalf("me expected 200, got %d: %s", resp2.Code, resp2.Body.String())
	}

	body3 := strings.NewReader(`{"username":"admin","password":"wrong"}`)
	resp3 := execute(router, httptestRequest(http.MethodPost, "/api/auth/login", body3))
	if resp3.Code != 401 {
		t.Fatalf("wrong password expected 401, got %d", resp3.Code)
	}

	resp4 := execute(router, httptestRequest(http.MethodGet, "/api/auth/me", nil))
	if resp4.Code != 401 {
		t.Fatalf("me without cookie expected 401, got %d", resp4.Code)
	}
	_ = authSvc
}

func TestUsersRequirePermission(t *testing.T) {
	store, authSvc, router := setupRouter(t)

	// create a regular viewer user
	hash, _ := authSvc.HashPassword("pw")
	if _, err := store.CreateUser("viewer", "viewer@example.com", hash, models.RoleUser, true); err != nil {
		t.Fatal(err)
	}

	// login as viewer
	body := strings.NewReader(`{"username":"viewer","password":"pw"}`)
	resp := execute(router, httptestRequest(http.MethodPost, "/api/auth/login", body))
	if resp.Code != 200 {
		t.Fatalf("viewer login expected 200, got %d", resp.Code)
	}
	cookie := setCookieHeader(resp)

	// viewer tries to list users -> forbidden
	req := reqWithCookie(http.MethodGet, "/api/users", cookie, nil)
	resp2 := execute(router, req)
	if resp2.Code != 403 {
		t.Fatalf("viewer list users expected 403, got %d: %s", resp2.Code, resp2.Body.String())
	}
}

func TestHealth(t *testing.T) {
	_, _, router := setupRouter(t)
	resp := execute(router, httptestRequest(http.MethodGet, "/health", nil))
	if resp.Code != 200 {
		t.Fatalf("health expected 200, got %d", resp.Code)
	}
}
