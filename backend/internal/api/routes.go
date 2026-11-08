package api

import (
	"net/http"

	"lokro-control-center/backend/internal/auth"
	"lokro-control-center/backend/internal/config"
	"lokro-control-center/backend/internal/db"
	mux "lokro-control-center/backend/internal/mux"
	"lokro-control-center/backend/internal/audit"
	"lokro-control-center/backend/internal/permissions"
	"lokro-control-center/backend/internal/uptime"
	"log/slog"
)

// Register mounts all routes on the stdlib mux router.
func Register(router *mux.Router, store db.Store, authSvc *auth.Service, permEngine *permissions.Engine, auditLog *audit.Logger, uptimeSvc *uptime.Service, cfg *config.Config, logger *slog.Logger) {
	// Health (public).
	router.GET("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = writeJSON(w, http.StatusOK, ginH{"status": "ok"})
	})

	// Auth (public).
	router.POST("/api/auth/login", loginHandler(authSvc, auditLog, cfg))
	router.POST("/api/auth/logout", chain(logoutHandler(authSvc, auditLog), AuthMiddleware(authSvc, cfg)))
	router.GET("/api/auth/me", chain(meHandler(store), AuthMiddleware(authSvc, cfg)))
	router.POST("/api/auth/change-password", chain(changePasswordHandler(authSvc, auditLog), AuthMiddleware(authSvc, cfg)))
	router.GET("/api/auth/sessions", chain(sessionsHandler(authSvc), AuthMiddleware(authSvc, cfg)))
	router.DELETE("/api/auth/sessions/:id", chain(revokeSessionHandler(authSvc), AuthMiddleware(authSvc, cfg)))

	// Users (admin).
	router.GET("/api/users", chain(listUsersHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionView, "")))
	router.POST("/api/users", chain(createUserHandler(store, authSvc, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionCreate, "")))
	router.GET("/api/users/:id", chain(getUserHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionView, "")))
	router.GET("/api/users/:id/permissions", chain(listUserPermissionsHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionView, "id")))
	router.PUT("/api/users/:id", chain(updateUserHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionEdit, "")))
	router.DELETE("/api/users/:id", chain(deleteUserHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionDelete, "")))

	// Permissions (admin).
	router.GET("/api/permissions", chain(listPermissionsHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionView, "")))
	router.POST("/api/permissions/grant", chain(grantPermissionHandler(store, permEngine), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionManage, "")))
	router.POST("/api/permissions/revoke", chain(revokePermissionHandler(store, permEngine), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUserManagement, permissions.ActionManage, "")))

	// Servers.
	router.GET("/api/servers", chain(listServersHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceServers, permissions.ActionView, "")))
	router.POST("/api/servers", chain(createServerHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceServerManagement, permissions.ActionCreate, "")))
	router.GET("/api/servers/:id", chain(getServerHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceServers, permissions.ActionView, "id")))
	router.PUT("/api/servers/:id", chain(updateServerHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceServerManagement, permissions.ActionEdit, "id")))
	router.DELETE("/api/servers/:id", chain(deleteServerHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceServerManagement, permissions.ActionDelete, "id")))

	// Metrics (read-only).
	router.GET("/api/metrics/servers/:id", chain(serverMetricsHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceMonitoring, permissions.ActionView, "id")))

	// Audit (admin).
	router.GET("/api/audit", chain(listAuditHandler(store, auditLog), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceLogs, permissions.ActionView, "")))

	// Uptime monitoring.
	router.GET("/api/uptime/checks", chain(listUptimeChecksHandler(uptimeSvc, store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUptime, permissions.ActionView, "")))
	router.POST("/api/uptime/checks", chain(createUptimeCheckHandler(uptimeSvc, store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUptime, permissions.ActionManage, "")))
	router.GET("/api/uptime/checks/:id", chain(getUptimeCheckHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUptime, permissions.ActionView, "")))
	router.PUT("/api/uptime/checks/:id", chain(updateUptimeCheckHandler(uptimeSvc, store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUptime, permissions.ActionManage, "")))
	router.DELETE("/api/uptime/checks/:id", chain(deleteUptimeCheckHandler(uptimeSvc, store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUptime, permissions.ActionManage, "")))
	router.GET("/api/uptime/results/:id", chain(listUptimeResultsHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUptime, permissions.ActionView, "")))
	router.GET("/api/uptime/summary/:id", chain(getUptimeSummaryHandler(store), AuthMiddleware(authSvc, cfg), RequirePermission(permEngine, permissions.ResourceUptime, permissions.ActionView, "")))
}

// chain composes an http.HandlerFunc with middleware applied in order before reaching the handler.
func chain(handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) http.HandlerFunc {
	var h http.Handler = handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h.ServeHTTP
}
