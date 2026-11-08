package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"lokro-control-center/backend/internal/auth"
	"lokro-control-center/backend/internal/config"
	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/models"
	"lokro-control-center/backend/internal/permissions"
	"lokro-control-center/backend/internal/audit"
	"lokro-control-center/backend/internal/uptime"
)

/* ---------- Auth ---------- */

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func loginHandler(authSvc *auth.Service, al *audit.Logger, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := decodeJSON(r, &req); err != nil || req.Username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		user, err := authSvc.Authenticate(req.Username, req.Password)
		if err != nil {
			al.Record(0, "login_failed", "auth", req.Username, "failed login attempt", r.RemoteAddr, r.UserAgent())
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := authSvc.CreateSession(user.ID, r.RemoteAddr, r.UserAgent())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create session")
			return
		}

		setSessionCookie(w, cfg, token)
		al.Record(user.ID, "login", "auth", fmt.Sprintf("%d", user.ID), "successful login", r.RemoteAddr, r.UserAgent())

		_ = writeJSON(w, http.StatusOK, ginH{
			"user": ginH{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
				"role":     string(user.Role),
			},
		})
	}
}

func logoutHandler(authSvc *auth.Service, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
			_ = authSvc.RevokeSession(hashToken(cookie.Value))
			clearSessionCookie(w, &config.Config{})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"message": "logged out"})
	}
}

func meHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := userIDFromCtx(r)
		user, err := store.GetUserByID(uid)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		_ = writeJSON(w, http.StatusOK, ginH{
			"user": ginH{
				"id":           user.ID,
				"username":     user.Username,
				"email":        user.Email,
				"role":         string(user.Role),
				"is_active":    user.IsActive,
				"last_login_at": user.LastLoginAt,
			},
		})
	}
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func changePasswordHandler(authSvc *auth.Service, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := userIDFromCtx(r)
		var req changePasswordRequest
		if err := decodeJSON(r, &req); err != nil || req.CurrentPassword == "" || req.NewPassword == "" {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		user, err := authSvc.Store().GetUserByID(uid)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		if !authSvc.VerifyPassword(req.CurrentPassword, user.PasswordHash) {
			al.Record(uid, "change_password_failed", "users", fmt.Sprintf("%d", uid), "wrong current password", "", "")
			writeError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}
		newHash, err := authSvc.HashPassword(req.NewPassword)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not hash password")
			return
		}
		if err := authSvc.Store().UpdateUserPassword(uid, newHash); err != nil {
			writeError(w, http.StatusInternalServerError, "could not update password")
			return
		}
		_ = authSvc.RevokeAllSessionsForUser(uid)
		al.Record(uid, "change_password", "users", fmt.Sprintf("%d", uid), "password changed", "", "")
		_ = writeJSON(w, http.StatusOK, ginH{"message": "password changed"})
	}
}

func sessionsHandler(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := userIDFromCtx(r)
		sessions, err := authSvc.ListSessions(uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list sessions")
			return
		}
		type sessionView struct {
			ID         int64     `json:"id"`
			ExpiresAt  time.Time `json:"expires_at"`
			CreatedAt  time.Time `json:"created_at"`
			IPAddress  string    `json:"ip_address"`
			UserAgent  string    `json:"user_agent"`
			IsRevoked  bool      `json:"is_revoked"`
			Current    bool      `json:"current"`
		}
		views := make([]sessionView, 0, len(sessions))
		currentToken := ""
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			currentToken = cookie.Value
		}
		currentHash := hashToken(currentToken)
		for _, s := range sessions {
			views = append(views, sessionView{
				ID:         s.ID,
				ExpiresAt:  s.ExpiresAt,
				CreatedAt:  s.CreatedAt,
				IPAddress:  s.IPAddress,
				UserAgent:  s.UserAgent,
				IsRevoked:  s.IsRevoked,
				Current:    s.TokenHash == currentHash,
			})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"sessions": views})
	}
}

func revokeSessionHandler(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := userIDFromCtx(r)
		sid, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		if err := authSvc.Store().RevokeSessionByID(sid, uid); err != nil {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		_ = writeJSON(w, http.StatusOK, ginH{"message": "session revoked"})
	}
}

/* ---------- Users ---------- */

func listUsersHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID := userIDFromCtx(r)
		users, err := store.ListUsers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		out := make([]ginH, 0, len(users))
		for _, u := range users {
			out = append(out, ginH{
				"id":            u.ID,
				"username":      u.Username,
				"email":         u.Email,
				"role":          string(u.Role),
				"is_active":     u.IsActive,
				"failed_logins": u.FailedLogins,
				"last_login_at": u.LastLoginAt,
				"created_at":    u.CreatedAt,
				"updated_at":    u.UpdatedAt,
				"is_self":       u.ID == actorID,
			})
		}
		al.Record(actorID, "list_users", "users", "", "listed users", "", "")
		_ = writeJSON(w, http.StatusOK, ginH{"users": out})
	}
}

type createUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
}

func createUserHandler(store db.Store, authSvc *auth.Service, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID := userIDFromCtx(r)
		var req createUserRequest
		if err := decodeJSON(r, &req); err != nil || req.Username == "" || req.Email == "" || req.Password == "" || req.Role == "" {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}

		hash, err := authSvc.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not hash password")
			return
		}
		id, err := store.CreateUser(req.Username, req.Email, hash, models.Role(req.Role), req.IsActive)
		if err != nil {
			writeError(w, http.StatusConflict, "username or email already exists")
			return
		}
		al.Record(actorID, "create_user", "users", fmt.Sprintf("%d", id), "created user "+req.Username, "", "")
		_ = writeJSON(w, http.StatusCreated, ginH{"id": id, "message": "user created"})
	}
}

func getUserHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		user, err := store.GetUserByID(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		al.Record(0, "get_user", "users", fmt.Sprintf("%d", id), "viewed user", "", "")
		_ = writeJSON(w, http.StatusOK, ginH{
			"user": ginH{
				"id":            user.ID,
				"username":      user.Username,
				"email":         user.Email,
				"role":          string(user.Role),
				"is_active":     user.IsActive,
				"failed_logins": user.FailedLogins,
				"last_login_at": user.LastLoginAt,
				"created_at":    user.CreatedAt,
				"updated_at":    user.UpdatedAt,
			},
		})
	}
}

type updateUserRequest struct {
	Email    string `json:"email"`
	Role     string `json:"role"`
	IsActive *bool  `json:"is_active"`
}

func updateUserHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		actorID := userIDFromCtx(r)
		var req updateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		updates := map[string]interface{}{}
		if req.Email != "" {
			updates["email"] = req.Email
		}
		if req.Role != "" {
			updates["role"] = req.Role
		}
		if req.IsActive != nil {
			val := 0
			if *req.IsActive {
				val = 1
			}
			updates["is_active"] = val
		}
		if len(updates) == 0 {
			writeError(w, http.StatusBadRequest, "nothing to update")
			return
		}
		if err := store.UpdateUser(id, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		al.Record(actorID, "update_user", "users", fmt.Sprintf("%d", id), "updated user", "", "")
		_ = writeJSON(w, http.StatusOK, ginH{"message": "user updated"})
	}
}

func deleteUserHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		actorID := userIDFromCtx(r)
		if id == actorID {
			writeError(w, http.StatusBadRequest, "cannot delete yourself")
			return
		}
		if err := store.DeleteUser(id); err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		al.Record(actorID, "delete_user", "users", fmt.Sprintf("%d", id), "deleted user", "", "")
		_ = writeJSON(w, http.StatusOK, ginH{"message": "user deleted"})
	}
}

func listUserPermissionsHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		ups, err := store.ListUserPermissions(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		out := make([]ginH, 0, len(ups))
		for _, up := range ups {
			out = append(out, ginH{
				"id":           up.ID,
				"user_id":      up.UserID,
				"permission_id": up.PermissionID,
				"server_id":    up.ServerID,
				"granted":      up.Granted,
				"permission":   up.Permission,
			})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"permissions": out})
	}
}

/* ---------- Permissions ---------- */

func listPermissionsHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		perms, err := store.ListPermissions()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		out := make([]ginH, 0, len(perms))
		for _, p := range perms {
			out = append(out, ginH{
				"id":          p.ID,
				"name":        p.Name,
				"description": p.Description,
				"resource":    p.Resource,
				"action":      p.Action,
			})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"permissions": out})
	}
}

type grantRequest struct {
	UserID       int64  `json:"user_id"`
	PermissionID int64  `json:"permission_id"`
	ServerID     *int64 `json:"server_id"`
	Granted      bool   `json:"granted"`
}

func grantPermissionHandler(store db.Store, pe *permissions.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req grantRequest
		if err := decodeJSON(r, &req); err != nil || req.UserID == 0 || req.PermissionID == 0 {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := store.GrantUserPermission(req.UserID, req.PermissionID, req.ServerID, req.Granted); err != nil {
			writeError(w, http.StatusInternalServerError, "could not grant permission")
			return
		}
		_ = pe
		_ = writeJSON(w, http.StatusOK, ginH{"message": "permission granted"})
	}
}

func revokePermissionHandler(store db.Store, pe *permissions.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req grantRequest
		if err := decodeJSON(r, &req); err != nil || req.UserID == 0 || req.PermissionID == 0 {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := store.RevokeUserPermission(req.UserID, req.PermissionID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not revoke")
			return
		}
		_ = pe
		_ = writeJSON(w, http.StatusOK, ginH{"message": "permission revoked"})
	}
}

/* ---------- Servers ---------- */

type serverRequest struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Hostname     string `json:"hostname"`
	IPAddress    string `json:"ip_address"`
	OS           string `json:"os"`
	AgentVersion string `json:"agent_version"`
}

func listServersHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		servers, err := store.ListServers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		out := make([]ginH, 0, len(servers))
		for _, sv := range servers {
			out = append(out, ginH{
				"id":             sv.ID,
				"uuid":           sv.UUID,
				"name":           sv.Name,
				"hostname":       sv.Hostname,
				"ip_address":     sv.IPAddress,
				"os":             sv.OS,
				"agent_version":  sv.AgentVersion,
				"status":         sv.Status,
				"last_heartbeat": sv.LastHeartbeat,
				"created_at":     sv.CreatedAt,
				"updated_at":     sv.UpdatedAt,
			})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"servers": out})
	}
}

func createServerHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req serverRequest
		if err := decodeJSON(r, &req); err != nil || req.UUID == "" || req.Name == "" {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		actorID := userIDFromCtx(r)
		id, err := store.CreateServer(req.UUID, req.Name, req.Hostname, req.IPAddress, req.OS, req.AgentVersion)
		if err != nil {
			writeError(w, http.StatusConflict, "server uuid already exists")
			return
		}
		al.Record(actorID, "create_server", "servers", fmt.Sprintf("%d", id), "created server "+req.Name, "", "")
		_ = writeJSON(w, http.StatusCreated, ginH{"id": id, "message": "server created"})
	}
}

func getServerHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		sv, err := store.GetServer(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "server not found")
			return
		}
		_ = writeJSON(w, http.StatusOK, ginH{"server": ginH{
			"id":             sv.ID,
			"uuid":           sv.UUID,
			"name":           sv.Name,
			"hostname":       sv.Hostname,
			"ip_address":     sv.IPAddress,
			"os":             sv.OS,
			"agent_version":  sv.AgentVersion,
			"status":         sv.Status,
			"last_heartbeat": sv.LastHeartbeat,
			"created_at":     sv.CreatedAt,
			"updated_at":     sv.UpdatedAt,
		}})
	}
}

func updateServerHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		actorID := userIDFromCtx(r)
		var req serverRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		updates := map[string]interface{}{
			"uuid":          req.UUID,
			"name":          req.Name,
			"hostname":      req.Hostname,
			"ip_address":    req.IPAddress,
			"os":            req.OS,
			"agent_version": req.AgentVersion,
		}
		if err := store.UpdateServer(id, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		al.Record(actorID, "update_server", "servers", fmt.Sprintf("%d", id), "updated server "+req.Name, "", "")
		_ = writeJSON(w, http.StatusOK, ginH{"message": "server updated"})
	}
}

func deleteServerHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		actorID := userIDFromCtx(r)
		if err := store.DeleteServer(id); err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		al.Record(actorID, "delete_server", "servers", fmt.Sprintf("%d", id), "deleted server", "", "")
		_ = writeJSON(w, http.StatusOK, ginH{"message": "server deleted"})
	}
}

/* ---------- Metrics ---------- */

func serverMetricsHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		limit := 60
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		metrics, err := store.ListServerMetrics(id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		out := make([]ginH, 0, len(metrics))
		for _, m := range metrics {
			out = append(out, ginH{
				"id":           m.ID,
				"server_id":    m.ServerID,
				"recorded_at":  m.RecordedAt,
				"cpu_percent":  m.CPUPercent,
				"load_1":       m.Load1,
				"load_5":       m.Load5,
				"load_15":      m.Load15,
				"mem_total":    m.MemTotal,
				"mem_used":     m.MemUsed,
				"mem_free":     m.MemFree,
				"mem_cached":   m.MemCached,
				"swap_total":   m.SwapTotal,
				"swap_used":    m.SwapUsed,
				"uptime":       m.Uptime,
				"process_count": m.ProcessCount,
			})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"metrics": out})
	}
}

/* ---------- Audit ---------- */

func listAuditHandler(store db.Store, al *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		offset := 0
		if o := r.URL.Query().Get("offset"); o != "" {
			if n, err := strconv.Atoi(o); err == nil && n >= 0 {
				offset = n
			}
		}
		logs, err := al.List(limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		_ = writeJSON(w, http.StatusOK, ginH{"audit": logs})
	}
}

/* ---------- Uptime ---------- */

type uptimeCheckRequest struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Type           string `json:"type"`
	Interval       int    `json:"interval"`
	Timeout        int    `json:"timeout"`
	ExpectedStatus int    `json:"expected_status"`
	IsActive       bool   `json:"is_active"`
}

func listUptimeChecksHandler(uptimeSvc *uptime.Service, store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks, err := store.ListUptimeChecks()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		out := make([]ginH, 0, len(checks))
		for _, c := range checks {
			out = append(out, ginH{
				"id":              c.ID,
				"name":            c.Name,
				"url":             c.URL,
				"type":            c.Type,
				"interval":        c.Interval,
				"timeout":         c.Timeout,
				"expected_status": c.ExpectedStatus,
				"is_active":       c.IsActive,
				"created_at":      c.CreatedAt,
				"updated_at":      c.UpdatedAt,
			})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"checks": out})
	}
}

func createUptimeCheckHandler(uptimeSvc *uptime.Service, store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req uptimeCheckRequest
		if err := decodeJSON(r, &req); err != nil || req.Name == "" || req.URL == "" || req.Type == "" {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if req.Interval <= 0 {
			req.Interval = 60
		}
		if req.Timeout <= 0 {
			req.Timeout = 10
		}
		if req.ExpectedStatus <= 0 {
			req.ExpectedStatus = 200
		}
		id, err := store.CreateUptimeCheck(req.Name, req.URL, req.Type, req.Interval, req.Timeout, req.ExpectedStatus, req.IsActive)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create check")
			return
		}
		_ = uptimeSvc
		_ = writeJSON(w, http.StatusCreated, ginH{"id": id, "message": "uptime check created"})
	}
}

func getUptimeCheckHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		c, err := store.GetUptimeCheck(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "check not found")
			return
		}
		_ = writeJSON(w, http.StatusOK, ginH{"check": ginH{
			"id":              c.ID,
			"name":            c.Name,
			"url":             c.URL,
			"type":            c.Type,
			"interval":        c.Interval,
			"timeout":         c.Timeout,
			"expected_status": c.ExpectedStatus,
			"is_active":       c.IsActive,
			"created_at":      c.CreatedAt,
			"updated_at":      c.UpdatedAt,
		}})
	}
}

func updateUptimeCheckHandler(uptimeSvc *uptime.Service, store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		var req uptimeCheckRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		updates := map[string]interface{}{
			"name":            req.Name,
			"url":             req.URL,
			"type":            req.Type,
			"interval":        req.Interval,
			"timeout":         req.Timeout,
			"expected_status": req.ExpectedStatus,
			"is_active":       req.IsActive,
		}
		if err := store.UpdateUptimeCheck(id, updates); err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		_ = uptimeSvc
		_ = writeJSON(w, http.StatusOK, ginH{"message": "uptime check updated"})
	}
}

func deleteUptimeCheckHandler(uptimeSvc *uptime.Service, store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := store.DeleteUptimeCheck(id); err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		_ = uptimeSvc
		_ = writeJSON(w, http.StatusOK, ginH{"message": "uptime check deleted"})
	}
}

func listUptimeResultsHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		results, err := store.ListUptimeResults(id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		out := make([]ginH, 0, len(results))
		for _, res := range results {
			out = append(out, ginH{
				"id":           res.ID,
				"check_id":     res.CheckID,
				"status":       res.Status,
				"response_time_ms": res.ResponseTime,
				"status_code":  res.StatusCode,
				"error":        res.Error,
				"checked_at":   res.CheckedAt,
			})
		}
		_ = writeJSON(w, http.StatusOK, ginH{"results": out})
	}
}

func getUptimeSummaryHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(param(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		summary, err := store.GetUptimeSummary(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "check not found")
			return
		}
		_ = writeJSON(w, http.StatusOK, ginH{"summary": ginH{
			"check_id":      summary.CheckID,
			"name":          summary.Name,
			"url":           summary.URL,
			"up_percent":    summary.UpPercent,
			"last_status":   summary.LastStatus,
			"last_checked":  summary.LastChecked,
			"avg_response":  summary.AvgResponse,
		}})
	}
}

