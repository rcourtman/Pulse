package audit

import (
	"fmt"
	"path/filepath"
)

// SQLiteLoggerFactory creates SQLite-backed audit loggers for tenant databases.
//
// The TenantLoggerManager passes a dbPath like "<base>/orgs/<org>/audit.db".
// NewSQLiteLogger expects a DataDir, and creates the database at:
//
//	<DataDir>/audit/audit.db
//
// This factory bridges that mismatch by extracting the directory from dbPath.
type SQLiteLoggerFactory struct {
	// CryptoMgr is used to enable HMAC signing (optional).
	// If nil, signing is disabled and signatures will be empty.
	CryptoMgr CryptoEncryptor

	// CryptoMgrForDataDir optionally provides a per-tenant crypto manager based on DataDir.
	// If set, it takes precedence over CryptoMgr.
	CryptoMgrForDataDir func(dataDir string) (CryptoEncryptor, error)

	// RetentionDays controls how long audit events are retained (0 uses SQLiteLogger defaults).
	RetentionDays int
}

func (f *SQLiteLoggerFactory) CreateLogger(dbPath string) (Logger, error) {
	if filepath.Clean(dbPath) == "." || dbPath == "" {
		return nil, fmt.Errorf("db path is required")
	}

	dataDir := filepath.Dir(dbPath)

	cfg := SQLiteLoggerConfig{
		DataDir:       dataDir,
		RetentionDays: f.RetentionDays,
	}

	if f.CryptoMgrForDataDir != nil {
		cm, err := f.CryptoMgrForDataDir(dataDir)
		if err != nil {
			return nil, err
		}
		cfg.CryptoMgr = cm
	} else {
		cfg.CryptoMgr = f.CryptoMgr
	}

	return NewSQLiteLogger(cfg)
}
