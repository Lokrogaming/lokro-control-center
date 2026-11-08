package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lokro-control-center/backend/internal/models"
)

func TestStoreUserLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	store, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	id, err := store.CreateUser("alice", "alice@example.com", "hash1", models.RoleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("expected id 1, got %d", id)
	}

	u, err := store.GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != id || u.Username != "alice" {
		t.Fatalf("unexpected user: %+v", u)
	}

	u2, err := store.GetUserByID(id)
	if err != nil || u2.Username != "alice" {
		t.Fatal("GetUserByID failed")
	}

	if err := store.UpdateUser(id, map[string]interface{}{"email": "alice2@example.com", "is_active": 0}); err != nil {
		t.Fatal(err)
	}
	u3, _ := store.GetUserByID(id)
	if u3.Email != "alice2@example.com" || u3.IsActive {
		t.Fatalf("update not applied: %+v", u3)
	}

	users, err := store.ListUsers()
	if err != nil || len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	count, _ := store.CountUsers()
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	if err := store.DeleteUser(id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetUserByID(id); err == nil {
		t.Fatal("user should be deleted")
	}
	count, _ = store.CountUsers()
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}
}

func TestStoreDuplicateUser(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewJSONFileStore(filepath.Join(dir, "dup.json"))
	defer store.Close()
	if _, err := store.CreateUser("bob", "bob@example.com", "h", models.RoleUser, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser("bob", "bob2@example.com", "h", models.RoleUser, true); err == nil {
		t.Fatal("expected duplicate username error")
	}
	if _, err := store.CreateUser("bob2", "bob@example.com", "h", models.RoleUser, true); err == nil {
		t.Fatal("expected duplicate email error")
	}
}

func TestStoreSessions(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewJSONFileStore(filepath.Join(dir, "sess.json"))
	defer store.Close()

	_, err := store.CreateUser("sessuser", "sess@example.com", "x", models.RoleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateSession(1, "hashA", time.Now().Add(time.Hour), "1.2.3.4", "ua")
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := store.ListSessions(1)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	user, err := store.ValidateSession("hashA")
	if err != nil || user.ID != 1 {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if err := store.RevokeSession("hashA"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ValidateSession("hashA"); err == nil {
		t.Fatal("revoked session should not validate")
	}
	if err := store.CleanupExpired(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreServerAndMetric(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewJSONFileStore(filepath.Join(dir, "srv.json"))
	defer store.Close()

	id, err := store.CreateServer("uuid-1", "Server One", "host1", "10.0.0.1", "ubuntu", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	sv, err := store.GetServer(id)
	if err != nil || sv.Name != "Server One" {
		t.Fatal(err)
	}
	if err := store.UpdateServerHeartbeat(id); err != nil {
		t.Fatal(err)
	}
	sv, _ = store.GetServer(id)
	if sv.Status != "online" {
		t.Fatalf("expected online, got %s", sv.Status)
	}

	_, err = store.InsertServerMetric(models.ServerMetric{
		ServerID:     id,
		RecordedAt:   time.Now(),
		CPUPercent:   12.5,
		MemTotal:     16000,
		MemUsed:      4000,
		ProcessCount: 130,
	})
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := store.ListServerMetrics(id, 10)
	if err != nil || len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].CPUPercent != 12.5 {
		t.Fatalf("unexpected cpu: %v", metrics[0].CPUPercent)
	}
}

func TestStoreServerAccess(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewJSONFileStore(filepath.Join(dir, "access.json"))
	defer store.Close()

	sid, _ := store.CreateServer("uuid-s", "S", "", "", "", "")
	uid, _ := store.CreateUser("u", "u@example.com", "h", models.RoleUser, true)

	if err := store.GrantUserServerAccess(uid, sid); err != nil {
		t.Fatal(err)
	}
	ok, err := store.ServerAccessible(uid, sid)
	if err != nil || !ok {
		t.Fatalf("expected accessible, got %v %v", ok, err)
	}
	servers, _ := store.ListUserServers(uid)
	if len(servers) != 1 || servers[0] != sid {
		t.Fatalf("unexpected user servers: %v", servers)
	}
	if err := store.RevokeUserServerAccess(uid, sid); err != nil {
		t.Fatal(err)
	}
	ok, _ = store.ServerAccessible(uid, sid)
	if ok {
		t.Fatal("should not be accessible after revoke")
	}
}

func TestStoreAuditAndSettings(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewJSONFileStore(filepath.Join(dir, "audit.json"))
	defer store.Close()

	if err := store.InsertAuditLog(nil, "test_action", "test_resource", "1", "details", "1.2.3.4", "ua"); err != nil {
		t.Fatal(err)
	}
	logs, err := store.ListAuditLogs(10, 0)
	if err != nil || len(logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(logs))
	}
	if logs[0].Action != "test_action" {
		t.Fatalf("unexpected action: %s", logs[0].Action)
	}

	if err := store.UpsertSetting("theme", "dark"); err != nil {
		t.Fatal(err)
	}
	val, found, err := store.GetSetting("theme")
	if err != nil || !found || val != "dark" {
		t.Fatalf("unexpected setting: %q %v", val, found)
	}
	val, found, _ = store.GetSetting("missing")
	if found {
		t.Fatal("missing setting should not be found")
	}
	_ = val
}

func TestStorePermissions(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewJSONFileStore(filepath.Join(dir, "perm.json"))
	defer store.Close()

	if err := store.EnsureDefaultPermission("servers.view", "desc", "servers", "view"); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDefaultPermission("servers.view", "desc", "servers", "view"); err != nil {
		t.Fatal("second insert should be no-op")
	}
	p, err := store.GetPermissionByName("servers.view")
	if err != nil || p.Resource != "servers" || p.Action != "view" {
		t.Fatal(err)
	}
	if err := store.EnsureRolePermission(models.RoleAdmin, p.ID); err != nil {
		t.Fatal(err)
	}
	perms, err := store.ListRolePermissions(models.RoleAdmin)
	if err != nil || len(perms) != 1 {
		t.Fatalf("expected 1 role perm, got %d", len(perms))
	}

	if err := store.GrantUserPermission(1, p.ID, nil, true); err != nil {
		t.Fatal(err)
	}
	ups, err := store.ListUserPermissions(1)
	if err != nil || len(ups) != 1 || !ups[0].Granted {
		t.Fatalf("unexpected user perms: %+v", ups)
	}
	if err := store.RevokeUserPermission(1, p.ID); err != nil {
		t.Fatal(err)
	}
	ups, _ = store.ListUserPermissions(1)
	if len(ups) != 0 {
		t.Fatal("expected 0 user perms after revoke")
	}
}

func TestStorePasswordUpdate(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewJSONFileStore(filepath.Join(dir, "pw.json"))
	defer store.Close()
	id, _ := store.CreateUser("pwuser", "pw@example.com", "oldhash", models.RoleUser, true)
	if err := store.UpdateUserPassword(id, "newhash"); err != nil {
		t.Fatal(err)
	}
	u, _ := store.GetUserByID(id)
	if u.PasswordHash != "newhash" {
		t.Fatal("password not updated")
	}
}

var _ = os.Rename
