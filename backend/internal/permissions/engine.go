package permissions

import (
	"fmt"

	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/models"
)

// Engine checks permissions server-side.
type Engine struct {
	store db.Store
}

// NewEngine creates a permission engine.
func NewEngine(store db.Store) *Engine {
	return &Engine{store: store}
}

// Action is the type of access being requested.
type Action string

const (
	ActionView     Action = "view"
	ActionCreate   Action = "create"
	ActionEdit     Action = "edit"
	ActionDelete   Action = "delete"
	ActionExecute  Action = "execute"
	ActionManage   Action = "manage"
)

// Resource is a protectable resource type.
type Resource string

const (
	ResourceServers              Resource = "servers"
	ResourceMonitoring           Resource = "monitoring"
	ResourceAnalytics            Resource = "analytics"
	ResourceLogs                 Resource = "logs"
	ResourceProcesses            Resource = "processes"
	ResourceNetwork              Resource = "network"
	ResourceStorage              Resource = "storage"
	ResourceDocker               Resource = "docker"
	ResourceUserManagement       Resource = "user_management"
	ResourceServerManagement     Resource = "server_management"
	ResourceSettings             Resource = "settings"
	ResourceUptime               Resource = "uptime"
)

// Check returns true if the user has the given action on the resource, optionally scoped to a server.
// Super admins are granted everything. Admins get broad access. Regular users need explicit grants.
func (e *Engine) Check(userID int64, role models.Role, resource Resource, action Action, serverID ...int64) (bool, error) {
	// Super admin: unrestricted.
	if role == models.RoleSuperAdmin {
		return true, nil
	}

	var sid *int64
	if len(serverID) > 0 && serverID[0] > 0 {
		sid = &serverID[0]
	}

	// 1) Role-based permission.
	roleAllowed, err := e.roleAllows(role, resource, action)
	if err != nil {
		return false, err
	}
	if roleAllowed {
		if sid != nil {
			return e.store.ServerAccessible(userID, *sid)
		}
		return true, nil
	}

	// 2) Explicit user permission grant (can grant or deny).
	userAllowed, denied, err := e.userAllows(userID, resource, action, sid)
	if err != nil {
		return false, err
	}
	if denied {
		return false, nil
	}
	if userAllowed {
		if sid != nil {
			return e.store.ServerAccessible(userID, *sid)
		}
		return true, nil
	}

	// 3) Admin role: broad default allow for monitoring/view on servers they have access to.
	if role == models.RoleAdmin {
		if action == ActionView || action == ActionExecute {
			if sid != nil {
				return e.store.ServerAccessible(userID, *sid)
			}
			return true, nil
		}
		if resource == ResourceUserManagement || resource == ResourceServerManagement || resource == ResourceSettings {
			return false, nil
		}
		return true, nil
	}

	// Regular user: deny by default.
	return false, nil
}

// MustCheck checks and returns an error if not allowed.
func (e *Engine) MustCheck(userID int64, role models.Role, resource Resource, action Action, serverID ...int64) error {
	allowed, err := e.Check(userID, role, resource, action, serverID...)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("permission denied: %s:%s", resource, action)
	}
	return nil
}

func (e *Engine) roleAllows(role models.Role, resource Resource, action Action) (bool, error) {
	perms, err := e.store.ListRolePermissions(role)
	if err != nil {
		return false, err
	}
	for _, p := range perms {
		if Resource(p.Resource) == resource && Action(p.Action) == action {
			return true, nil
		}
	}
	return false, nil
}

func (e *Engine) userAllows(userID int64, resource Resource, action Action, serverID *int64) (allowed bool, denied bool, err error) {
	ups, err := e.store.ListUserPermissions(userID)
	if err != nil {
		return false, false, err
	}
	for _, up := range ups {
		if up.Permission == nil {
			continue
		}
		if Resource(up.Permission.Resource) != resource || Action(up.Permission.Action) != action {
			continue
		}
		// Server scope: match only if server_id is NULL (global) or equals the requested server.
		if serverID == nil {
			if up.ServerID != nil {
				continue // server-scoped grant does not apply to a global check
			}
		} else {
			if up.ServerID == nil || *up.ServerID != *serverID {
				continue
			}
		}
		if up.Granted {
			return true, false, nil
		}
		return false, true, nil
	}
	return false, false, nil
}

// EnsureBasePermissions inserts the standard permission set if missing.
func (e *Engine) EnsureBasePermissions() error {
	base := []struct {
		name, desc string
		resource   Resource
		action     Action
	}{
		{"servers.view", "View servers and monitoring overview", ResourceServers, ActionView},
		{"servers.manage", "Add, edit, remove servers", ResourceServerManagement, ActionManage},
		{"analytics.view", "View web analytics", ResourceAnalytics, ActionView},
		{"logs.view", "View system and application logs", ResourceLogs, ActionView},
		{"logs.download", "Download log files", ResourceLogs, ActionExecute},
		{"processes.view", "View running processes", ResourceProcesses, ActionView},
		{"processes.execute", "Kill/manage processes", ResourceProcesses, ActionExecute},
		{"network.view", "View network metrics", ResourceNetwork, ActionView},
		{"storage.view", "View storage/disk metrics", ResourceStorage, ActionView},
		{"docker.view", "View Docker containers", ResourceDocker, ActionView},
		{"docker.execute", "Manage Docker containers", ResourceDocker, ActionExecute},
		{"users.manage", "Create, edit, delete users", ResourceUserManagement, ActionManage},
		{"users.view", "View users", ResourceUserManagement, ActionView},
		{"permissions.manage", "Manage permissions and roles", ResourceUserManagement, ActionManage},
		{"settings.view", "View settings", ResourceSettings, ActionView},
		{"settings.edit", "Edit settings", ResourceSettings, ActionEdit},
		{"audit.view", "View audit logs", ResourceLogs, ActionView},
		{"uptime.view", "View uptime monitoring", ResourceUptime, ActionView},
		{"uptime.manage", "Manage uptime checks", ResourceUptime, ActionManage},
	}
	for _, b := range base {
		if err := e.store.EnsureDefaultPermission(b.name, b.desc, string(b.resource), string(b.action)); err != nil {
			return err
		}
	}
	return nil
}

// EnsureDefaultRoleGrants assigns baseline permissions to roles.
func (e *Engine) EnsureDefaultRoleGrants() error {
	type permRef struct {
		name     string
		resource string
		action   string
	}
	perms := []permRef{
		{"servers.view", string(ResourceServers), string(ActionView)},
		{"servers.manage", string(ResourceServerManagement), string(ActionManage)},
		{"analytics.view", string(ResourceAnalytics), string(ActionView)},
		{"logs.view", string(ResourceLogs), string(ActionView)},
		{"logs.download", string(ResourceLogs), string(ActionExecute)},
		{"processes.view", string(ResourceProcesses), string(ActionView)},
		{"processes.execute", string(ResourceProcesses), string(ActionExecute)},
		{"network.view", string(ResourceNetwork), string(ActionView)},
		{"storage.view", string(ResourceStorage), string(ActionView)},
		{"docker.view", string(ResourceDocker), string(ActionView)},
		{"docker.execute", string(ResourceDocker), string(ActionExecute)},
		{"users.manage", string(ResourceUserManagement), string(ActionManage)},
		{"users.view", string(ResourceUserManagement), string(ActionView)},
		{"permissions.manage", string(ResourceUserManagement), string(ActionManage)},
		{"settings.view", string(ResourceSettings), string(ActionView)},
		{"settings.edit", string(ResourceSettings), string(ActionEdit)},
		{"audit.view", string(ResourceLogs), string(ActionView)},
		{"uptime.view", string(ResourceUptime), string(ActionView)},
		{"uptime.manage", string(ResourceUptime), string(ActionManage)},
	}
	idByName := make(map[string]int64)
	for _, p := range perms {
		perm, err := e.store.GetPermissionByName(p.name)
		if err != nil {
			return fmt.Errorf("lookup permission %s: %w", p.name, err)
		}
		idByName[p.name] = perm.ID
	}

	// Super admin gets all.
	for _, id := range idByName {
		if err := e.store.EnsureRolePermission(models.RoleSuperAdmin, id); err != nil {
			return err
		}
	}

	// Admin: broad view/manage on servers + monitoring + analytics + logs + processes + network + storage + users view
	adminPerms := []string{
		"servers.view", "servers.manage", "analytics.view", "logs.view", "processes.view",
		"network.view", "storage.view", "docker.view", "users.view", "settings.view", "audit.view",
		"uptime.view", "uptime.manage",
	}
	for _, name := range adminPerms {
		if err := e.store.EnsureRolePermission(models.RoleAdmin, idByName[name]); err != nil {
			return err
		}
	}

	// User: baseline view on servers + analytics + processes + network + storage
	userPerms := []string{
		"servers.view", "analytics.view", "processes.view", "network.view", "storage.view",
		"uptime.view",
	}
	for _, name := range userPerms {
		if err := e.store.EnsureRolePermission(models.RoleUser, idByName[name]); err != nil {
			return err
		}
	}
	return nil
}
