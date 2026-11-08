package main

import (
	"log"
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
		log.Printf("configuration error: %v", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Printf("application init error: %v", err)
		os.Exit(1)
	}
	defer application.Close()

	// Seed default admin if none exists.
	if err := seedDefaultAdmin(application); err != nil {
		log.Printf("admin seeding skipped: %v", err)
	}

	go func() {
		if err := application.Start(); err != nil {
			log.Printf("server error: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for termination signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("shutting down")
	if err := application.Stop(); err != nil {
		log.Printf("shutdown error: %v", err)
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
