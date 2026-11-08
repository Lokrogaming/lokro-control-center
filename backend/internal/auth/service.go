package auth

import (
	"crypto/rand"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"

	"lokro-control-center/backend/internal/config"
	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/models"
)

// Service handles authentication and session management.
type Service struct {
	store db.Store
	cfg   *config.Config
}

// NewService creates an auth service.
func NewService(store db.Store, cfg *config.Config) *Service {
	return &Service{store: store, cfg: cfg}
}

// Store returns the underlying storage (useful for delegates).
func (s *Service) Store() db.Store {
	return s.store
}

// Credentials carries login input.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// TokenPair returns an access token string and its hash for storage.
type TokenPair struct {
	Token string
	Hash  string
}

// Errors returned by the auth service.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrAccountInactive    = errors.New("account inactive")
	ErrSessionInvalid     = errors.New("invalid session")
)

// hashPassword hashes a password using PBKDF2-HMAC-SHA256 (stdlib).
// Encoded form: salt_hex $ hash_hex
func hashPassword(password string, cfg *config.Config) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, cfg.PBKDF2Iterations, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(hash), nil
}

// verifyPassword checks a password against an encoded PBKDF2 hash.
func verifyPassword(password, encoded string, cfg *config.Config) bool {
	saltHex, hashHex, ok := splitEncoded(encoded)
	if !ok {
		return false
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}
	computed, err := pbkdf2.Key(sha256.New, password, salt, cfg.PBKDF2Iterations, 32)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(hash, computed) == 1
}

// splitEncoded splits "salt$hash" into the two hex parts.
func splitEncoded(encoded string) (string, string, bool) {
	idx := -1
	for i := len(encoded) - 1; i >= 0; i-- {
		if encoded[i] == '$' {
			idx = i
			break
		}
	}
	if idx <= 0 || idx >= len(encoded)-1 {
		return "", "", false
	}
	return encoded[:idx], encoded[idx+1:], true
}

// generateToken creates a random session token and its SHA-256 hash.
func (s *Service) generateToken() (TokenPair, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return TokenPair{}, err
	}
	token := hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return TokenPair{Token: token, Hash: hex.EncodeToString(sum[:])}, nil
}

// Authenticate validates credentials and returns the user.
func (s *Service) Authenticate(username, password string) (*models.User, error) {
	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, ErrAccountLocked
	}

	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	if !verifyPassword(password, user.PasswordHash, s.cfg) {
		_ = s.store.RecordFailedLogin(user.ID, s.cfg.MaxFailedLogins, s.cfg.LockoutDuration)
		return nil, ErrInvalidCredentials
	}

	_ = s.store.ClearFailedLogins(user.ID)
	return user, nil
}

// CreateSession creates a new session for a user and returns the plaintext token.
func (s *Service) CreateSession(userID int64, ipAddress, userAgent string) (string, error) {
	pair, err := s.generateToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(s.cfg.SessionDuration)
	_, err = s.store.CreateSession(userID, pair.Hash, expiresAt, ipAddress, userAgent)
	if err != nil {
		return "", err
	}
	return pair.Token, nil
}

// ValidateSession validates a token hash and returns the user if active and not expired/revoked.
func (s *Service) ValidateSession(tokenHash string) (*models.User, error) {
	user, err := s.store.ValidateSession(tokenHash)
	if err != nil {
		return nil, ErrSessionInvalid
	}
	if !user.IsActive {
		return nil, ErrAccountInactive
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, ErrAccountLocked
	}
	return user, nil
}

// RevokeSession revokes a session by token hash.
func (s *Service) RevokeSession(tokenHash string) error {
	return s.store.RevokeSession(tokenHash)
}

// RevokeAllSessionsForUser revokes all sessions of a user.
func (s *Service) RevokeAllSessionsForUser(userID int64) error {
	return s.store.RevokeAllSessionsForUser(userID)
}

// ListSessions returns active sessions for a user.
func (s *Service) ListSessions(userID int64) ([]models.Session, error) {
	return s.store.ListSessions(userID)
}

// HashPassword is a helper exposed for seeding/admin use.
func (s *Service) HashPassword(password string) (string, error) {
	return hashPassword(password, s.cfg)
}

// VerifyPassword checks a password against an encoded PBKDF2 hash.
func (s *Service) VerifyPassword(password, encoded string) bool {
	return verifyPassword(password, encoded, s.cfg)
}

// CleanupExpired removes expired and revoked sessions.
func (s *Service) CleanupExpired() error {
	return s.store.CleanupExpired()
}
