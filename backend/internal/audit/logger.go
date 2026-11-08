package audit

import (
	"fmt"

	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/models"
)

// Logger records security-relevant and administrative actions.
type Logger struct {
	store db.Store
}

// NewLogger creates an audit logger.
func NewLogger(store db.Store) *Logger {
	return &Logger{store: store}
}

// Record writes an audit log entry.
func (l *Logger) Record(userID int64, action, resource, resourceID, details, ipAddress, userAgent string) error {
	if err := l.store.InsertAuditLog(&userID, action, resource, resourceID, details, ipAddress, userAgent); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

// List returns audit log entries with optional filtering and pagination.
func (l *Logger) List(limit, offset int) ([]models.AuditLog, error) {
	return l.store.ListAuditLogs(limit, offset)
}
