package models

import (
	"time"
)

// Role defines the system-wide role of a user.
type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleUser       Role = "user"
)

// User represents a system user.
type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Role         Role       `json:"role"`
	IsActive     bool       `json:"is_active"`
	FailedLogins int        `json:"-"`
	LockedUntil  *time.Time `json:"-"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Session represents an authenticated user session.
type Session struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	TokenHash  string     `json:"-"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	IPAddress  string     `json:"ip_address"`
	UserAgent  string     `json:"user_agent"`
	IsRevoked  bool       `json:"is_revoked"`
}

// Permission represents a granular permission.
type Permission struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"` // e.g. servers, analytics, logs, processes
	Action      string `json:"action"`   // e.g. view, create, edit, delete, execute, manage
}

// UserPermission represents a granular user-level grant/deny.
type UserPermission struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	PermissionID int64      `json:"permission_id"`
	ServerID     *int64     `json:"server_id"`
	Granted      bool       `json:"granted"`
	Permission   *Permission `json:"permission,omitempty"`
}

// Server represents a monitored server (agent target).
type Server struct {
	ID           int64      `json:"id"`
	UUID         string     `json:"uuid"`
	Name         string     `json:"name"`
	Hostname     string     `json:"hostname"`
	IPAddress    string     `json:"ip_address"`
	OS           string     `json:"os"`
	AgentVersion string     `json:"agent_version"`
	Status       string     `json:"status"` // online, offline, unknown
	LastHeartbeat *time.Time `json:"last_heartbeat"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ServerMetric is a point-in-time system metric snapshot.
type ServerMetric struct {
	ID           int64     `json:"id"`
	ServerID     int64     `json:"server_id"`
	RecordedAt   time.Time `json:"recorded_at"`
	CPUPercent   float64   `json:"cpu_percent"`
	Load1        float64   `json:"load_1"`
	Load5        float64   `json:"load_5"`
	Load15       float64   `json:"load_15"`
	MemTotal     uint64    `json:"mem_total"`
	MemUsed      uint64    `json:"mem_used"`
	MemFree      uint64    `json:"mem_free"`
	MemCached    uint64    `json:"mem_cached"`
	SwapTotal    uint64    `json:"swap_total"`
	SwapUsed     uint64    `json:"swap_used"`
	Uptime       uint64    `json:"uptime"`
	ProcessCount int       `json:"process_count"`
}

// Process represents a running process snapshot.
type Process struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user"`
	CPUPercent float64 `json:"cpu_percent"`
	MemMB      float64 `json:"mem_mb"`
	Status     string  `json:"status"`
	StartTime  string  `json:"start_time"`
	RunTime    string  `json:"run_time"`
}

// AuditLog records security-relevant actions.
type AuditLog struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Action     string     `json:"action"`
	Resource   string     `json:"resource"`
	ResourceID string     `json:"resource_id"`
	Details    string     `json:"details"`
	IPAddress  string     `json:"ip_address"`
	UserAgent  string     `json:"user_agent"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Setting is a key/value application setting.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Notification is a user-facing notification.
type Notification struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Type      string     `json:"type"` // info, warning, error, success
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
}

// Pagination holds pagination parameters and results.
type Pagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// UptimeCheck defines a monitored endpoint.
type UptimeCheck struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Type         string    `json:"type"`       // http, https, tcp
	Interval     int       `json:"interval"`   // seconds
	Timeout      int       `json:"timeout"`    // seconds
	ExpectedStatus int     `json:"expected_status"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UptimeResult is a single check result.
type UptimeResult struct {
	ID           int64     `json:"id"`
	CheckID      int64     `json:"check_id"`
	Status       string    `json:"status"`   // up, down
	ResponseTime int       `json:"response_time_ms"`
	StatusCode   int       `json:"status_code"`
	Error        string    `json:"error"`
	CheckedAt    time.Time `json:"checked_at"`
}

// UptimeSummary aggregates check results.
type UptimeSummary struct {
	CheckID      int64   `json:"check_id"`
	Name         string  `json:"name"`
	URL          string  `json:"url"`
	UpPercent    float64 `json:"up_percent"`
	LastStatus   string  `json:"last_status"`
	LastChecked  string  `json:"last_checked"`
	AvgResponse  float64 `json:"avg_response_ms"`
}
