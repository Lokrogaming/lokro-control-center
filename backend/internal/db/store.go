package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"lokro-control-center/backend/internal/models"
)

// Store is the application storage interface. All persistence goes through it.
// A JSON-file implementation is provided here (JSONFileStore). A SQL-backed
// implementation (SQLite / PostgreSQL) can be added later behind the same
// interface without changing the application or API layer.
type Store interface {
	Close() error

	// ---- Users ----
	CreateUser(username, email, passwordHash string, role models.Role, isActive bool) (int64, error)
	GetUserByUsername(username string) (*models.User, error)
	GetUserByID(id int64) (*models.User, error)
	UpdateUser(id int64, updates map[string]interface{}) error
	RecordFailedLogin(userID int64, maxFailed int, lockout time.Duration) error
	ClearFailedLogins(userID int64) error
	UpdateUserPassword(userID int64, passwordHash string) error
	DeleteUser(id int64) error
	ListUsers() ([]*models.User, error)
	CountUsers() (int, error)

	// ---- Sessions ----
	CreateSession(userID int64, tokenHash string, expiresAt time.Time, ipAddress, userAgent string) (int64, error)
	ValidateSession(tokenHash string) (*models.User, error)
	RevokeSession(tokenHash string) error
	RevokeSessionByID(sessionID, userID int64) error
	RevokeAllSessionsForUser(userID int64) error
	ListSessions(userID int64) ([]models.Session, error)
	CleanupExpired() error

	// ---- Permissions ----
	EnsureDefaultPermission(name, description, resource, action string) error
	GetPermissionByName(name string) (*models.Permission, error)
	ListPermissions() ([]*models.Permission, error)
	EnsureRolePermission(role models.Role, permissionID int64) error
	ListRolePermissions(role models.Role) ([]*models.Permission, error)

	// ---- User permissions (grants) ----
	GrantUserPermission(userID, permissionID int64, serverID *int64, granted bool) error
	RevokeUserPermission(userID, permissionID int64) error
	ListUserPermissions(userID int64) ([]*models.UserPermission, error)

	// ---- Servers ----
	CreateServer(uuid, name, hostname, ipAddress, os, agentVersion string) (int64, error)
	GetServer(id int64) (*models.Server, error)
	ListServers() ([]*models.Server, error)
	UpdateServer(id int64, updates map[string]interface{}) error
	DeleteServer(id int64) error
	UpdateServerHeartbeat(id int64) error

	// ---- User <-> Server access ----
	GrantUserServerAccess(userID, serverID int64) error
	RevokeUserServerAccess(userID, serverID int64) error
	ListUserServers(userID int64) ([]int64, error)
	ServerAccessible(userID, serverID int64) (bool, error)

	// ---- Metrics ----
	InsertServerMetric(m models.ServerMetric) (int64, error)
	ListServerMetrics(serverID int64, limit int) ([]models.ServerMetric, error)

	// ---- Audit ----
	InsertAuditLog(userID *int64, action, resource, resourceID, details, ipAddress, userAgent string) error
	ListAuditLogs(limit, offset int) ([]models.AuditLog, error)

	// ---- Settings ----
	UpsertSetting(key, value string) error
	GetSetting(key string) (string, bool, error)

	// ---- Notifications ----
	InsertNotification(userID int64, title, message, typ string) error
	ListNotifications(userID int64, unreadOnly bool) ([]models.Notification, error)

	// ---- Uptime Monitoring ----
	CreateUptimeCheck(name, url, typ string, interval, timeout, expectedStatus int, isActive bool) (int64, error)
	GetUptimeCheck(id int64) (*models.UptimeCheck, error)
	ListUptimeChecks() ([]*models.UptimeCheck, error)
	UpdateUptimeCheck(id int64, updates map[string]interface{}) error
	DeleteUptimeCheck(id int64) error
	InsertUptimeResult(checkID int64, status string, responseTime, statusCode int, err string) error
	ListUptimeResults(checkID int64, limit int) ([]models.UptimeResult, error)
	GetUptimeSummary(checkID int64) (*models.UptimeSummary, error)
}

// JSONFileStore implements Store using a single JSON file with atomic writes.
// All data is held in memory and persisted on mutation. Suitable for
// single-process deployments (the standard installation).
type JSONFileStore struct {
	mu   sync.RWMutex
	path string
	data *jsonData
}

type jsonData struct {
	Users           []*models.User           `json:"users"`
	Sessions        []*models.Session        `json:"sessions"`
	Permissions     []*models.Permission     `json:"permissions"`
	RolePermissions []*rolePermission        `json:"role_permissions"`
	UserPermissions []*userPermission        `json:"user_permissions"`
	Servers         []*models.Server         `json:"servers"`
	UserServers     []*userServer            `json:"user_servers"`
	Metrics         []*models.ServerMetric   `json:"metrics"`
	AuditLogs       []*models.AuditLog       `json:"audit_logs"`
	Settings        []*settingEntry          `json:"settings"`
	Notifications   []*models.Notification   `json:"notifications"`
	UptimeChecks    []*models.UptimeCheck    `json:"uptime_checks"`
	UptimeResults   []*models.UptimeResult   `json:"uptime_results"`
}

type rolePermission struct {
	Role            string `json:"role"`
	PermissionID    int64  `json:"permission_id"`
}
type userPermission struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"user_id"`
	PermissionID int64  `json:"permission_id"`
	ServerID    *int64  `json:"server_id"`
	Granted     bool    `json:"granted"`
}
type userServer struct {
	UserID   int64 `json:"user_id"`
	ServerID int64 `json:"server_id"`
}
type settingEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var nextID int64 = 1

func nextIDValue() int64 {
	id := nextID
	nextID++
	return id
}

// NewJSONFileStore opens (or creates) the JSON store at path.
func NewJSONFileStore(path string) (*JSONFileStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &JSONFileStore{path: path, data: &jsonData{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	s.recomputeIDs()
	return s, nil
}

func (s *JSONFileStore) load() error {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.save()
		}
		return err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(s.data); err != nil {
		return fmt.Errorf("decode store: %w", err)
	}
	if s.data == nil {
		s.data = &jsonData{}
	}
	// Initialize slices if nil.
	if s.data.Users == nil {
		s.data.Users = []*models.User{}
	}
	if s.data.Sessions == nil {
		s.data.Sessions = []*models.Session{}
	}
	if s.data.Permissions == nil {
		s.data.Permissions = []*models.Permission{}
	}
	if s.data.RolePermissions == nil {
		s.data.RolePermissions = []*rolePermission{}
	}
	if s.data.UserPermissions == nil {
		s.data.UserPermissions = []*userPermission{}
	}
	if s.data.Servers == nil {
		s.data.Servers = []*models.Server{}
	}
	if s.data.UserServers == nil {
		s.data.UserServers = []*userServer{}
	}
	if s.data.Metrics == nil {
		s.data.Metrics = []*models.ServerMetric{}
	}
	if s.data.AuditLogs == nil {
		s.data.AuditLogs = []*models.AuditLog{}
	}
	if s.data.Settings == nil {
		s.data.Settings = []*settingEntry{}
	}
	if s.data.Notifications == nil {
		s.data.Notifications = []*models.Notification{}
	}
	if s.data.UptimeChecks == nil {
		s.data.UptimeChecks = []*models.UptimeCheck{}
	}
	if s.data.UptimeResults == nil {
		s.data.UptimeResults = []*models.UptimeResult{}
	}
	return nil
}

func (s *JSONFileStore) save() error {
	tmp := s.path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	file.Close()
	return os.Rename(tmp, s.path)
}

func (s *JSONFileStore) recomputeIDs() {
	max := int64(0)
	updateMax := func(id int64) { if id > max { max = id } }
	for _, u := range s.data.Users { updateMax(u.ID) }
	for _, ss := range s.data.Sessions { updateMax(ss.ID) }
	for _, p := range s.data.Permissions { updateMax(p.ID) }
	for _, up := range s.data.UserPermissions { updateMax(up.ID) }
	for _, sv := range s.data.Servers { updateMax(sv.ID) }
	for _, m := range s.data.Metrics { updateMax(m.ID) }
	for _, a := range s.data.AuditLogs { updateMax(a.ID) }
	for _, n := range s.data.Notifications { updateMax(n.ID) }
	for _, c := range s.data.UptimeChecks { updateMax(c.ID) }
	for _, r := range s.data.UptimeResults { updateMax(r.ID) }
	nextID = max + 1
}

func (s *JSONFileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

// ---- Users ----

func (s *JSONFileStore) CreateUser(username, email, passwordHash string, role models.Role, isActive bool) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.data.Users {
		if u.Username == username || u.Email == email {
			return 0, fmt.Errorf("duplicate username or email")
		}
	}
	id := nextIDValue()
	now := time.Now()
	u := &models.User{
		ID:          id,
		Username:    username,
		Email:       email,
		PasswordHash: passwordHash,
		Role:        role,
		IsActive:    isActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.data.Users = append(s.data.Users, u)
	if err := s.save(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *JSONFileStore) GetUserByUsername(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.data.Users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (s *JSONFileStore) GetUserByID(id int64) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.data.Users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (s *JSONFileStore) UpdateUser(id int64, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.data.Users {
		if u.ID == id {
			if v, ok := updates["email"]; ok { u.Email = v.(string) }
			if v, ok := updates["role"]; ok { u.Role = models.Role(v.(string)) }
			if v, ok := updates["is_active"]; ok { u.IsActive = v.(int) == 1 }
			if v, ok := updates["failed_logins"]; ok { u.FailedLogins = v.(int) }
			if v, ok := updates["locked_until"]; ok {
				if v == nil { u.LockedUntil = nil } else { u.LockedUntil = v.(*time.Time) }
			}
			u.UpdatedAt = time.Now()
			if err := s.save(); err != nil { return err }
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

// RecordFailedLogin increments failed_logins and locks the account when the threshold is reached.
func (s *JSONFileStore) RecordFailedLogin(userID int64, maxFailed int, lockout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.data.Users {
		if u.ID == userID {
			u.FailedLogins++
			if u.FailedLogins >= maxFailed {
				until := time.Now().Add(lockout)
				u.LockedUntil = &until
			}
			u.UpdatedAt = time.Now()
			return s.save()
		}
	}
	return fmt.Errorf("user not found")
}

// ClearFailedLogins resets failed_logins and clears the lock.
func (s *JSONFileStore) ClearFailedLogins(userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.data.Users {
		if u.ID == userID {
			u.FailedLogins = 0
			u.LockedUntil = nil
			u.UpdatedAt = time.Now()
			return s.save()
		}
	}
	return fmt.Errorf("user not found")
}

func (s *JSONFileStore) UpdateUserPassword(userID int64, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.data.Users {
		if u.ID == userID {
			u.PasswordHash = passwordHash
			u.UpdatedAt = time.Now()
			return s.save()
		}
	}
	return fmt.Errorf("user not found")
}



func (s *JSONFileStore) DeleteUser(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.data.Users {
		if u.ID == id {
			s.data.Users = append(s.data.Users[:i], s.data.Users[i+1:]...)
			// Cascade: sessions, user_permissions, user_servers, notifications.
			s.data.Sessions = filterSessions(s.data.Sessions, func(ss *models.Session) bool { return ss.UserID != id })
			s.data.UserPermissions = filterUserPerms(s.data.UserPermissions, func(up *userPermission) bool { return up.UserID != id })
			s.data.UserServers = filterUserServers(s.data.UserServers, func(us *userServer) bool { return us.UserID != id })
			s.data.Notifications = filterNotifications(s.data.Notifications, func(n *models.Notification) bool { return n.UserID != id })
			if err := s.save(); err != nil { return err }
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

func (s *JSONFileStore) ListUsers() ([]*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.User, len(s.data.Users))
	copy(out, s.data.Users)
	return out, nil
}

func (s *JSONFileStore) CountUsers() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data.Users), nil
}

// ---- Sessions ----

func (s *JSONFileStore) CreateSession(userID int64, tokenHash string, expiresAt time.Time, ipAddress, userAgent string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := nextIDValue()
	ss := &models.Session{
		ID:         id,
		UserID:     userID,
		TokenHash:  tokenHash,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now(),
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		IsRevoked:  false,
	}
	s.data.Sessions = append(s.data.Sessions, ss)
	if err := s.save(); err != nil { return 0, err }
	return id, nil
}

func (s *JSONFileStore) ValidateSession(tokenHash string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sess *models.Session
	for _, ss := range s.data.Sessions {
		if ss.TokenHash == tokenHash && !ss.IsRevoked && ss.ExpiresAt.After(time.Now()) {
			sess = ss
			break
		}
	}
	if sess == nil {
		return nil, fmt.Errorf("session not found")
	}
	for _, u := range s.data.Users {
		if u.ID == sess.UserID {
			if !u.IsActive {
				return nil, fmt.Errorf("inactive user")
			}
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (s *JSONFileStore) RevokeSession(tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ss := range s.data.Sessions {
		if ss.TokenHash == tokenHash {
			ss.IsRevoked = true
			return s.save()
		}
	}
	return nil
}

func (s *JSONFileStore) RevokeSessionByID(sessionID, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ss := range s.data.Sessions {
		if ss.ID == sessionID && ss.UserID == userID {
			ss.IsRevoked = true
			return s.save()
		}
	}
	return fmt.Errorf("session not found or not owned by user")
}

func (s *JSONFileStore) RevokeAllSessionsForUser(userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, ss := range s.data.Sessions {
		if ss.UserID == userID && !ss.IsRevoked {
			ss.IsRevoked = true
			changed = true
		}
	}
	if changed {
		return s.save()
	}
	return nil
}

func (s *JSONFileStore) ListSessions(userID int64) ([]models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Session
	for _, ss := range s.data.Sessions {
		if ss.UserID == userID && ss.ExpiresAt.After(time.Now()) && !ss.IsRevoked {
			out = append(out, *ss)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *JSONFileStore) CleanupExpired() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	before := len(s.data.Sessions)
	filtered := s.data.Sessions[:0]
	for _, ss := range s.data.Sessions {
		if ss.ExpiresAt.After(now) && !ss.IsRevoked {
			filtered = append(filtered, ss)
		}
	}
	s.data.Sessions = filtered
	if len(s.data.Sessions) != before {
		return s.save()
	}
	return nil
}

// ---- Permissions ----

func (s *JSONFileStore) EnsureDefaultPermission(name, description, resource, action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.data.Permissions {
		if p.Name == name {
			return nil
		}
	}
	id := nextIDValue()
	s.data.Permissions = append(s.data.Permissions, &models.Permission{
		ID:          id,
		Name:        name,
		Description: description,
		Resource:    resource,
		Action:      action,
	})
	return s.save()
}

func (s *JSONFileStore) GetPermissionByName(name string) (*models.Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.data.Permissions {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("permission not found")
}

func (s *JSONFileStore) ListPermissions() ([]*models.Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.Permission, len(s.data.Permissions))
	copy(out, s.data.Permissions)
	return out, nil
}

func (s *JSONFileStore) EnsureRolePermission(role models.Role, permissionID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rp := range s.data.RolePermissions {
		if rp.Role == string(role) && rp.PermissionID == permissionID {
			return nil
		}
	}
	s.data.RolePermissions = append(s.data.RolePermissions, &rolePermission{Role: string(role), PermissionID: permissionID})
	return s.save()
}

func (s *JSONFileStore) ListRolePermissions(role models.Role) ([]*models.Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*models.Permission
	permByID := make(map[int64]*models.Permission)
	for _, p := range s.data.Permissions {
		permByID[p.ID] = p
	}
	for _, rp := range s.data.RolePermissions {
		if rp.Role == string(role) {
			if p, ok := permByID[rp.PermissionID]; ok {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// ---- User permissions ----

func (s *JSONFileStore) GrantUserPermission(userID, permissionID int64, serverID *int64, granted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Upsert by (user_id, permission_id, server_id).
	for _, up := range s.data.UserPermissions {
		if up.UserID == userID && up.PermissionID == permissionID &&
			((up.ServerID == nil && serverID == nil) || (up.ServerID != nil && serverID != nil && *up.ServerID == *serverID)) {
			up.Granted = granted
			return s.save()
		}
	}
	id := nextIDValue()
	s.data.UserPermissions = append(s.data.UserPermissions, &userPermission{
		ID:           id,
		UserID:       userID,
		PermissionID: permissionID,
		ServerID:     serverID,
		Granted:      granted,
	})
	return s.save()
}

func (s *JSONFileStore) RevokeUserPermission(userID, permissionID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, up := range s.data.UserPermissions {
		if up.UserID == userID && up.PermissionID == permissionID {
			s.data.UserPermissions = append(s.data.UserPermissions[:i], s.data.UserPermissions[i+1:]...)
			return s.save()
		}
	}
	return nil
}

func (s *JSONFileStore) ListUserPermissions(userID int64) ([]*models.UserPermission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*models.UserPermission
	permByID := make(map[int64]*models.Permission)
	for _, p := range s.data.Permissions {
		permByID[p.ID] = p
	}
	for _, up := range s.data.UserPermissions {
		if up.UserID == userID {
			out = append(out, &models.UserPermission{
				ID:           up.ID,
				UserID:       up.UserID,
				PermissionID: up.PermissionID,
				ServerID:     up.ServerID,
				Granted:      up.Granted,
				Permission:   permByID[up.PermissionID],
			})
		}
	}
	return out, nil
}

// ---- Servers ----

func (s *JSONFileStore) CreateServer(uuid, name, hostname, ipAddress, os, agentVersion string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sv := range s.data.Servers {
		if sv.UUID == uuid {
			return 0, fmt.Errorf("server uuid already exists")
		}
	}
	id := nextIDValue()
	now := time.Now()
	sv := &models.Server{
		ID:           id,
		UUID:         uuid,
		Name:         name,
		Hostname:     hostname,
		IPAddress:    ipAddress,
		OS:           os,
		AgentVersion: agentVersion,
		Status:       "unknown",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.data.Servers = append(s.data.Servers, sv)
	if err := s.save(); err != nil { return 0, err }
	return id, nil
}

func (s *JSONFileStore) GetServer(id int64) (*models.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sv := range s.data.Servers {
		if sv.ID == id {
			return sv, nil
		}
	}
	return nil, fmt.Errorf("server not found")
}

func (s *JSONFileStore) ListServers() ([]*models.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.Server, len(s.data.Servers))
	copy(out, s.data.Servers)
	return out, nil
}

func (s *JSONFileStore) UpdateServer(id int64, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sv := range s.data.Servers {
		if sv.ID == id {
			if v, ok := updates["uuid"]; ok { sv.UUID = v.(string) }
			if v, ok := updates["name"]; ok { sv.Name = v.(string) }
			if v, ok := updates["hostname"]; ok { sv.Hostname = v.(string) }
			if v, ok := updates["ip_address"]; ok { sv.IPAddress = v.(string) }
			if v, ok := updates["os"]; ok { sv.OS = v.(string) }
			if v, ok := updates["agent_version"]; ok { sv.AgentVersion = v.(string) }
			sv.UpdatedAt = time.Now()
			return s.save()
		}
	}
	return fmt.Errorf("server not found")
}

func (s *JSONFileStore) DeleteServer(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sv := range s.data.Servers {
		if sv.ID == id {
			s.data.Servers = append(s.data.Servers[:i], s.data.Servers[i+1:]...)
			s.data.Metrics = filterMetrics(s.data.Metrics, func(m *models.ServerMetric) bool { return m.ServerID != id })
			s.data.UserServers = filterUserServers(s.data.UserServers, func(us *userServer) bool { return us.ServerID != id })
			return s.save()
		}
	}
	return fmt.Errorf("server not found")
}

func (s *JSONFileStore) UpdateServerHeartbeat(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sv := range s.data.Servers {
		if sv.ID == id {
			now := time.Now()
			sv.LastHeartbeat = &now
			sv.Status = "online"
			sv.UpdatedAt = now
			return s.save()
		}
	}
	return fmt.Errorf("server not found")
}

// ---- User <-> Server ----

func (s *JSONFileStore) GrantUserServerAccess(userID, serverID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, us := range s.data.UserServers {
		if us.UserID == userID && us.ServerID == serverID {
			return nil
		}
	}
	s.data.UserServers = append(s.data.UserServers, &userServer{UserID: userID, ServerID: serverID})
	return s.save()
}

func (s *JSONFileStore) RevokeUserServerAccess(userID, serverID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, us := range s.data.UserServers {
		if us.UserID == userID && us.ServerID == serverID {
			s.data.UserServers = append(s.data.UserServers[:i], s.data.UserServers[i+1:]...)
			return s.save()
		}
	}
	return nil
}

func (s *JSONFileStore) ListUserServers(userID int64) ([]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []int64
	for _, us := range s.data.UserServers {
		if us.UserID == userID {
			out = append(out, us.ServerID)
		}
	}
	return out, nil
}

func (s *JSONFileStore) ServerAccessible(userID, serverID int64) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, us := range s.data.UserServers {
		if us.UserID == userID && us.ServerID == serverID {
			return true, nil
		}
	}
	return false, nil
}

// ---- Metrics ----

func (s *JSONFileStore) InsertServerMetric(m models.ServerMetric) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := nextIDValue()
	m.ID = id
	cp := m
	s.data.Metrics = append(s.data.Metrics, &cp)
	return id, s.save()
}

func (s *JSONFileStore) ListServerMetrics(serverID int64, limit int) ([]models.ServerMetric, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.ServerMetric
	for i := len(s.data.Metrics) - 1; i >= 0; i-- {
		m := s.data.Metrics[i]
		if m.ServerID == serverID {
			out = append(out, *m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ---- Audit ----

func (s *JSONFileStore) InsertAuditLog(userID *int64, action, resource, resourceID, details, ipAddress, userAgent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := nextIDValue()
	a := models.AuditLog{
		ID:         id,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		CreatedAt:  time.Now(),
	}
	if userID != nil {
		a.UserID = *userID
	}
	s.data.AuditLogs = append(s.data.AuditLogs, &a)
	return s.save()
}

func (s *JSONFileStore) ListAuditLogs(limit, offset int) ([]models.AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.AuditLog
	for i := len(s.data.AuditLogs) - 1; i >= 0; i-- {
		out = append(out, *s.data.AuditLogs[i])
		if len(out) >= limit {
			break
		}
	}
	// Apply offset
	if offset > len(out) {
		out = out[:0]
	} else {
		out = out[offset:]
	}
	return out, nil
}

// ---- Settings ----

func (s *JSONFileStore) UpsertSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.data.Settings {
		if st.Key == key {
			st.Value = value
			return s.save()
		}
	}
	s.data.Settings = append(s.data.Settings, &settingEntry{Key: key, Value: value})
	return s.save()
}

func (s *JSONFileStore) GetSetting(key string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, st := range s.data.Settings {
		if st.Key == key {
			return st.Value, true, nil
		}
	}
	return "", false, nil
}

// ---- Notifications ----

func (s *JSONFileStore) InsertNotification(userID int64, title, message, typ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := nextIDValue()
	n := models.Notification{
		ID:        id,
		UserID:    userID,
		Title:     title,
		Message:   message,
		Type:      typ,
		Read:      false,
		CreatedAt: time.Now(),
	}
	s.data.Notifications = append(s.data.Notifications, &n)
	return s.save()
}

func (s *JSONFileStore) ListNotifications(userID int64, unreadOnly bool) ([]models.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Notification
	for _, n := range s.data.Notifications {
		if n.UserID == userID && (!unreadOnly || !n.Read) {
			out = append(out, *n)
		}
	}
	return out, nil
}

// ---- Uptime Monitoring ----

func (s *JSONFileStore) CreateUptimeCheck(name, url, typ string, interval, timeout, expectedStatus int, isActive bool) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := nextIDValue()
	now := time.Now()
	c := &models.UptimeCheck{
		ID:              id,
		Name:            name,
		URL:             url,
		Type:            typ,
		Interval:        interval,
		Timeout:         timeout,
		ExpectedStatus:  expectedStatus,
		IsActive:        isActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.data.UptimeChecks = append(s.data.UptimeChecks, c)
	return id, s.save()
}

func (s *JSONFileStore) GetUptimeCheck(id int64) (*models.UptimeCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.data.UptimeChecks {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("uptime check not found")
}

func (s *JSONFileStore) ListUptimeChecks() ([]*models.UptimeCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.UptimeCheck, len(s.data.UptimeChecks))
	copy(out, s.data.UptimeChecks)
	return out, nil
}

func (s *JSONFileStore) UpdateUptimeCheck(id int64, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.data.UptimeChecks {
		if c.ID == id {
			if v, ok := updates["name"]; ok { c.Name = v.(string) }
			if v, ok := updates["url"]; ok { c.URL = v.(string) }
			if v, ok := updates["type"]; ok { c.Type = v.(string) }
			if v, ok := updates["interval"]; ok { c.Interval = v.(int) }
			if v, ok := updates["timeout"]; ok { c.Timeout = v.(int) }
			if v, ok := updates["expected_status"]; ok { c.ExpectedStatus = v.(int) }
			if v, ok := updates["is_active"]; ok { c.IsActive = v.(bool) }
			c.UpdatedAt = time.Now()
			return s.save()
		}
	}
	return fmt.Errorf("uptime check not found")
}

func (s *JSONFileStore) DeleteUptimeCheck(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.data.UptimeChecks {
		if c.ID == id {
			s.data.UptimeChecks = append(s.data.UptimeChecks[:i], s.data.UptimeChecks[i+1:]...)
			// cascade delete results
			s.data.UptimeResults = filterUptimeResults(s.data.UptimeResults, func(r *models.UptimeResult) bool { return r.CheckID != id })
			return s.save()
		}
	}
	return fmt.Errorf("uptime check not found")
}

func (s *JSONFileStore) InsertUptimeResult(checkID int64, status string, responseTime, statusCode int, err string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := nextIDValue()
	r := models.UptimeResult{
		ID:           id,
		CheckID:      checkID,
		Status:       status,
		ResponseTime: responseTime,
		StatusCode:   statusCode,
		Error:        err,
		CheckedAt:    time.Now(),
	}
	s.data.UptimeResults = append(s.data.UptimeResults, &r)
	return s.save()
}

func (s *JSONFileStore) ListUptimeResults(checkID int64, limit int) ([]models.UptimeResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.UptimeResult
	for i := len(s.data.UptimeResults) - 1; i >= 0; i-- {
		r := s.data.UptimeResults[i]
		if r.CheckID == checkID {
			out = append(out, *r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *JSONFileStore) GetUptimeSummary(checkID int64) (*models.UptimeSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var check *models.UptimeCheck
	for _, c := range s.data.UptimeChecks {
		if c.ID == checkID {
			check = c
			break
		}
	}
	if check == nil {
		return nil, fmt.Errorf("uptime check not found")
	}
	var results []models.UptimeResult
	for _, r := range s.data.UptimeResults {
		if r.CheckID == checkID {
			results = append(results, *r)
		}
	}
	summary := &models.UptimeSummary{
		CheckID:   checkID,
		Name:      check.Name,
		URL:       check.URL,
		UpPercent: 100.0,
	}
	if len(results) > 0 {
		upCount := 0
		totalResp := 0
		for _, r := range results {
			if r.Status == "up" {
				upCount++
			}
			totalResp += r.ResponseTime
		}
		summary.UpPercent = float64(upCount) / float64(len(results)) * 100.0
		summary.AvgResponse = float64(totalResp) / float64(len(results))
		summary.LastStatus = results[0].Status
		summary.LastChecked = results[0].CheckedAt.Format("15:04:05")
	}
	return summary, nil
}

// ---- filters ----

func filterSessions(in []*models.Session, keep func(*models.Session) bool) []*models.Session {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}
func filterUserPerms(in []*userPermission, keep func(*userPermission) bool) []*userPermission {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}
func filterUserServers(in []*userServer, keep func(*userServer) bool) []*userServer {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}
func filterMetrics(in []*models.ServerMetric, keep func(*models.ServerMetric) bool) []*models.ServerMetric {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}
func filterNotifications(in []*models.Notification, keep func(*models.Notification) bool) []*models.Notification {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}
func filterUptimeResults(in []*models.UptimeResult, keep func(*models.UptimeResult) bool) []*models.UptimeResult {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

var _ = fmt.Sprintf
