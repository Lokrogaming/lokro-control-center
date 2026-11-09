package app

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"lokro-control-center/backend/internal/api"
	"lokro-control-center/backend/internal/audit"
	"lokro-control-center/backend/internal/auth"
	"lokro-control-center/backend/internal/config"
	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/models"
	mux "lokro-control-center/backend/internal/mux"
	"lokro-control-center/backend/internal/permissions"
	"lokro-control-center/backend/internal/uptime"
	"lokro-control-center/backend/internal/web"
)

// App holds the wired application.
type App struct {
	Handler http.Handler
	Store   db.Store
	Auth    *auth.Service
	Perms   *permissions.Engine
	Audit   *audit.Logger
	Uptime  *uptime.Service
	Cfg     *config.Config
	Logger  *log.Logger
	server  *http.Server
}

// New builds and wires the application.
func New(cfg *config.Config) (*App, error) {
	logger := log.New(io.Discard, "", log.LstdFlags)

	store, err := db.NewJSONFileStore(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	authSvc := auth.NewService(store, cfg)
	permEngine := permissions.NewEngine(store)
	auditLogger := audit.NewLogger(store)
	uptimeSvc := uptime.NewService(store, logger)

	// Ensure base permissions and role grants.
	if err := permEngine.EnsureBasePermissions(); err != nil {
		return nil, fmt.Errorf("base permissions: %w", err)
	}
	if err := permEngine.EnsureDefaultRoleGrants(); err != nil {
		return nil, fmt.Errorf("default role grants: %w", err)
	}

	router := mux.NewRouter()
	api.Register(router, store, authSvc, permEngine, auditLogger, uptimeSvc, cfg, logger)

	// Serve the compiled React frontend for every non-API route. The router
	// only invokes this fallback when no registered route matched, so /api/*
	// and /health keep their normal handlers.
	router.SetNotFoundHandler(web.NewFrontend(cfg.FrontendDir, logger))

	// Start uptime checker in background.
	if err := uptimeSvc.Start(); err != nil {
		return nil, fmt.Errorf("uptime service: %w", err)
	}

	return &App{
		Handler: router,
		Store:   store,
		Auth:    authSvc,
		Perms:   permEngine,
		Audit:   auditLogger,
		Uptime:  uptimeSvc,
		Cfg:     cfg,
		Logger:  logger,
	}, nil
}

// Start begins listening on the configured port.
func (a *App) Start() error {
	addr := a.Cfg.Host + ":" + a.Cfg.Port
	a.server = &http.Server{
		Addr:         addr,
		Handler:      a.Handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	a.Logger.Printf("server listening address=%s", addr)
	return a.server.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (a *App) Stop() error {
	if a.server != nil {
		return a.server.Close()
	}
	return nil
}

// Close persists the JSON store.
func (a *App) Close() error {
	return a.Store.Close()
}

// seedDefaultAdmin creates an initial super admin when no users exist.
func seedDefaultAdmin(a *App) error {
	count, err := a.Store.CountUsers()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	adminUser := os.Getenv("INITIAL_ADMIN_USERNAME")
	adminPass := os.Getenv("INITIAL_ADMIN_PASSWORD")
	adminEmail := os.Getenv("INITIAL_ADMIN_EMAIL")
	if adminUser == "" || adminPass == "" {
		log.Printf("no initial admin credentials provided; skipping default admin creation. Set INITIAL_ADMIN_USERNAME and INITIAL_ADMIN_PASSWORD.")
		return nil
	}

	hash, err := a.Auth.HashPassword(adminPass)
	if err != nil {
		return err
	}

	_, err = a.Store.CreateUser(adminUser, adminEmail, hash, models.RoleSuperAdmin, true)
	if err != nil {
		return err
	}
	log.Printf("default super admin created username=%s", adminUser)
	return nil
}
