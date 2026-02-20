package audit

import (
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var tenantAuditOrgIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// TenantLoggerManager manages per-tenant audit loggers.
// Each tenant gets their own isolated audit database at <orgDir>/audit/audit.db
type TenantLoggerManager struct {
	mu       sync.RWMutex
	loggers  map[string]Logger
	dataPath string        // Base data path
	factory  LoggerFactory // Factory for creating tenant loggers
}

func isValidOrgID(orgID string) bool {
	if orgID == "" || orgID == "." || orgID == ".." {
		return false
	}
	if filepath.Base(orgID) != orgID {
		return false
	}
	return tenantAuditOrgIDPattern.MatchString(orgID)
}

// LoggerFactory creates audit loggers for specific paths.
type LoggerFactory interface {
	// CreateLogger creates a new audit logger at the specified path.
	CreateLogger(dbPath string) (Logger, error)
}

// DefaultLoggerFactory creates console loggers (for OSS).
type DefaultLoggerFactory struct{}

// CreateLogger creates a console logger (doesn't use the path).
func (f *DefaultLoggerFactory) CreateLogger(dbPath string) (Logger, error) {
	return NewConsoleLogger(), nil
}

// NewTenantLoggerManager creates a new tenant logger manager.
func NewTenantLoggerManager(dataPath string, factory LoggerFactory) *TenantLoggerManager {
	if factory == nil {
		factory = &DefaultLoggerFactory{}
	}
	return &TenantLoggerManager{
		loggers:  make(map[string]Logger),
		dataPath: dataPath,
		factory:  factory,
	}
}

// GetLogger returns the audit logger for a specific organization.
// It lazily initializes the logger if it doesn't exist.
// For the "default" org, it returns the global logger.
func (m *TenantLoggerManager) GetLogger(orgID string) Logger {
	// Default org uses the global logger
	if orgID == "" || orgID == "default" {
		return GetLogger()
	}
	if !isValidOrgID(orgID) {
		log.Warn().Str("org_id", orgID).Msg("Invalid organization ID for tenant audit logger; using console logger")
		return NewConsoleLogger()
	}

	m.mu.RLock()
	logger, exists := m.loggers[orgID]
	m.mu.RUnlock()

	if exists {
		return logger
	}

	// Create new logger for tenant
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if logger, exists = m.loggers[orgID]; exists {
		return logger
	}

	// Create tenant-specific logger
	dbPath := filepath.Join(m.dataPath, "orgs", orgID, "audit.db")
	logger, err := m.factory.CreateLogger(dbPath)
	if err != nil {
		log.Error().
			Err(err).
			Str("org_id", orgID).
			Str("db_path", dbPath).
			Msg("Failed to create tenant audit logger, using console logger")
		logger = NewConsoleLogger()
	}

	m.loggers[orgID] = logger
	log.Info().
		Str("org_id", orgID).
		Str("db_path", dbPath).
		Msg("Created tenant audit logger")

	return logger
}

// Log logs an audit event for a specific organization.
func (m *TenantLoggerManager) Log(orgID, eventType, user, ip, path string, success bool, details string) error {
	logger := m.GetLogger(orgID)
	event := Event{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		EventType: eventType,
		User:      user,
		IP:        ip,
		Path:      path,
		Success:   success,
		Details:   details,
	}
	return logger.Log(event)
}

// Query queries audit events for a specific organization.
func (m *TenantLoggerManager) Query(orgID string, filter QueryFilter) ([]Event, error) {
	logger := m.GetLogger(orgID)
	return logger.Query(filter)
}

// Count counts audit events for a specific organization.
func (m *TenantLoggerManager) Count(orgID string, filter QueryFilter) (int, error) {
	logger := m.GetLogger(orgID)
	return logger.Count(filter)
}

// Close closes all tenant loggers.
func (m *TenantLoggerManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for orgID, logger := range m.loggers {
		if closer, ok := logger.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				log.Error().
					Err(err).
					Str("org_id", orgID).
					Msg("Failed to close tenant audit logger")
			}
		}
	}

	m.loggers = make(map[string]Logger)
}

// GetAllLoggers returns all initialized loggers (for administrative purposes).
func (m *TenantLoggerManager) GetAllLoggers() map[string]Logger {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Logger, len(m.loggers))
	for k, v := range m.loggers {
		result[k] = v
	}
	return result
}

// RemoveTenantLogger removes a specific tenant's logger.
// Useful when an organization is deleted.
func (m *TenantLoggerManager) RemoveTenantLogger(orgID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger, exists := m.loggers[orgID]
	if !exists {
		return
	}

	if closer, ok := logger.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			log.Error().
				Err(err).
				Str("org_id", orgID).
				Msg("Failed to close tenant audit logger during removal")
		}
	}

	delete(m.loggers, orgID)
	log.Info().Str("org_id", orgID).Msg("Removed tenant audit logger")
}
