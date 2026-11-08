package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"lokro-control-center/backend/internal/config"
	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/models"
)

func newTestStore(t *testing.T) db.Store {
	dir := t.TempDir()
	s, err := db.NewJSONFileStore(dir + "/auth.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newTestConfig() *config.Config {
	return &config.Config{
		SessionDuration:  24 * time.Hour,
		PBKDF2Iterations: 1000,
		MaxFailedLogins:  3,
		LockoutDuration:  15 * time.Minute,
	}
}

func sha256Str(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestAuthenticateRoundTrip(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, newTestConfig())

	_, err := store.CreateUser("bob", "bob@example.com", "", models.RoleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := svc.HashPassword("Secret123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateUserPassword(1, hash); err != nil {
		t.Fatal(err)
	}

	user, err := svc.Authenticate("bob", "Secret123!")
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if user.Username != "bob" || user.Role != models.RoleUser {
		t.Fatalf("unexpected user: %+v", user)
	}

	if _, err := svc.Authenticate("bob", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateLockout(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, newTestConfig())
	_, err := store.CreateUser("lock", "lock@example.com", "", models.RoleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := svc.HashPassword("pw")
	_ = store.UpdateUserPassword(1, hash)

	for i := 0; i < 3; i++ {
		if _, err := svc.Authenticate("lock", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected invalid credentials, got %v", err)
		}
	}
	if _, err := svc.Authenticate("lock", "pw"); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("expected account locked, got %v", err)
	}
}

func TestSessionCreateAndValidate(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, newTestConfig())
	userID, err := store.CreateUser("sessuser", "sess@example.com", "x", models.RoleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.CreateSession(userID, "1.2.3.4", "ua")
	if err != nil {
		t.Fatal(err)
	}
	user, err := svc.ValidateSession(sha256Str(token))
	if err != nil || user.ID != userID {
		t.Fatalf("validate session failed: %v (userID=%d)", err, userID)
	}
	if err := svc.RevokeSession(sha256Str(token)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateSession(sha256Str(token)); err == nil {
		t.Fatal("revoked session should not validate")
	}
	sessions, err := svc.ListSessions(userID)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("expected 0 active sessions, got %d", len(sessions))
	}
}
