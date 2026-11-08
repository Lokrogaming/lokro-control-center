package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"lokro-control-center/backend/internal/auth"
	"lokro-control-center/backend/internal/config"
	mux "lokro-control-center/backend/internal/mux"
	"lokro-control-center/backend/internal/models"
	"lokro-control-center/backend/internal/permissions"
)

const (
	sessionCookieName = "lokro_session"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxUser
	ctxRole
)

type ginH map[string]interface{}

// hashToken returns the SHA-256 hex of a token for storage comparison.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// setSessionCookie sets the HttpOnly session cookie.
func setSessionCookie(w http.ResponseWriter, cfg *config.Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: parseSameSite(cfg.CookieSameSite),
		Domain:   cfg.CookieDomain,
		MaxAge:   int(cfg.SessionDuration.Seconds()),
	})
}

// clearSessionCookie clears the session cookie.
func clearSessionCookie(w http.ResponseWriter, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: parseSameSite(cfg.CookieSameSite),
		Domain:   cfg.CookieDomain,
	})
}

func parseSameSite(s string) http.SameSite {
	switch s {
	case "Strict":
		return http.SameSiteStrictMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func userIDFromCtx(r *http.Request) int64 {
	if v := r.Context().Value(ctxUserID); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

func param(r *http.Request, name string) string {
	return mux.Params(r)[name]
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	_ = writeJSON(w, status, ginH{"error": message})
}

// LoggerMiddleware logs incoming requests.
func LoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"ip", r.RemoteAddr,
				"latency", time.Since(start),
			)
		})
	}
}

// AuthMiddleware authenticates via the session cookie and sets user in context.
func AuthMiddleware(authSvc *auth.Service, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			user, err := authSvc.ValidateSession(hashToken(cookie.Value))
			if err != nil {
				clearSessionCookie(w, cfg)
				writeError(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, user.ID)
			ctx = context.WithValue(ctx, ctxUser, user.Username)
			ctx = context.WithValue(ctx, ctxRole, string(user.Role))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission enforces a resource:action permission, optionally scoped to a server ID path param.
func RequirePermission(permEngine *permissions.Engine, resource permissions.Resource, action permissions.Action, serverParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid := userIDFromCtx(r)
			if uid == 0 {
				writeError(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			roleVal := r.Context().Value(ctxRole)
			role := models.RoleUser
			if rs, ok := roleVal.(string); ok {
				role = models.Role(rs)
			}

			var serverIDs []int64
			if serverParam != "" {
				if id, err := strconv.Atoi(param(r, serverParam)); err == nil && id > 0 {
					serverIDs = []int64{int64(id)}
				}
			}

			allowed, err := permEngine.Check(uid, role, resource, action, serverIDs...)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "permission check failed")
				return
			}
			if !allowed {
				writeError(w, http.StatusForbidden, "permission denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

var _ = fmt.Sprintf
var _ = io.Discard
