package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"lokro-control-center/backend/internal/app"
	"lokro-control-center/backend/internal/config"
	"lokro-control-center/backend/internal/models"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("application init error", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	// Seed default admin if none exists.
	if err := seedDefaultAdmin(application); err != nil {
		slog.Warn("admin seeding skipped", "error", err)
	}

	go func() {
		if err := application.Start(); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for termination signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	if err := application.Stop(); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
}

// seedDefaultAdmin creates an initial super admin when no users exist.
func seedDefaultAdmin(a *app.App) error {
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
		// Do not create a default account with guessable credentials.
		slog.Warn("no initial admin credentials provided; skipping default admin creation. Set INITIAL_ADMIN_USERNAME and INITIAL_ADMIN_PASSWORD.")
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
	slog.Info("default super admin created", "username", adminUser)
	return nil
}
