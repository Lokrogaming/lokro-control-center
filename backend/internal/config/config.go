package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Env            string
	Port           string
	Host           string
	DatabaseURL    string // file path for the JSON store (or DSN for a future SQL driver)
	DatabaseDriver string // "json" (default) or future "sqlite"/"postgres"

	// FrontendDir is the directory containing the compiled Vite build
	// (index.html + assets/). Relative paths are resolved against the process
	// working directory (systemd: WorkingDirectory=/opt/lokro-control-center).
	FrontendDir string

	// Session / Auth
	SessionSecret   string
	SessionDuration time.Duration
	CookieSecure    bool
	CookieSameSite  string
	CookieDomain    string

	// Password hashing (PBKDF2-HMAC-SHA256, stdlib-only)
	PBKDF2Iterations int

	// Security
	MaxFailedLogins int
	LockoutDuration time.Duration

	// Monitoring
	MetricsInterval    time.Duration
	MetricsRetention   time.Duration
	AnalyticsRetention time.Duration
	AnonymizeIP        bool

	// Server
	ServerName string

	// Feature flags
	DockerEnabled bool

	// Notification (placeholders for future phases)
	DiscordWebhookURL string
	SMTPHost          string
	SMTPPort          int
	SMTPUser          string
	SMTPPass          string
	SMTPFrom          string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:             getEnv("APP_ENV", "production"),
		Port:            getEnv("APP_PORT", "8080"),
		Host:            getEnv("APP_HOST", "0.0.0.0"),
		DatabaseURL:     getEnv("DATABASE_URL", "/opt/lokro-control-center/data/lokro.json"),
		DatabaseDriver:  getEnv("DATABASE_DRIVER", "json"),
		FrontendDir:     getEnv("FRONTEND_DIR", "frontend/dist"),
		SessionSecret:   getEnv("SESSION_SECRET", ""),
		SessionDuration: parseDuration(getEnv("SESSION_DURATION", "24h"), 24*time.Hour),
		CookieSecure:    getEnv("COOKIE_SECURE", "true") == "true",
		CookieSameSite:  getEnv("COOKIE_SAMESITE", "Lax"),
		CookieDomain:    getEnv("COOKIE_DOMAIN", ""),

		PBKDF2Iterations: parseInt(getEnv("PBKDF2_ITERATIONS", "200000"), 200000),

		MaxFailedLogins: parseInt(getEnv("MAX_FAILED_LOGINS", "5"), 5),
		LockoutDuration: parseDuration(getEnv("LOCKOUT_DURATION", "15m"), 15*time.Minute),

		MetricsInterval:    parseDuration(getEnv("METRICS_INTERVAL", "5s"), 5*time.Second),
		MetricsRetention:   parseDuration(getEnv("METRICS_RETENTION", "72h"), 72*time.Hour),
		AnalyticsRetention: parseDuration(getEnv("ANALYTICS_RETENTION", "90d"), 90*24*time.Hour),
		AnonymizeIP:        getEnv("ANONYMIZE_IP", "true") == "true",

		ServerName:    getEnv("SERVER_NAME", "LOKRO Server Control Center"),
		DockerEnabled: getEnv("DOCKER_ENABLED", "false") == "true",

		DiscordWebhookURL: getEnv("DISCORD_WEBHOOK_URL", ""),
		SMTPHost:          getEnv("SMTP_HOST", ""),
		SMTPPort:          parseInt(getEnv("SMTP_PORT", "587"), 587),
		SMTPUser:          getEnv("SMTP_USER", ""),
		SMTPPass:          getEnv("SMTP_PASS", ""),
		SMTPFrom:          getEnv("SMTP_FROM", ""),
	}

	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required; generate one with: openssl rand -hex 32")
	}

	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// IsProduction returns true when running in production mode.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production")
}
