package auth

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

var (
	ErrTokenInvalid = errors.New("magic link token is invalid")
	ErrTokenExpired = errors.New("magic link token is expired")
	ErrTokenUsed    = errors.New("magic link token already used")
)

const storeCleanupInterval = 5 * time.Minute
const privateDirPerm = 0o700

func ensureOwnerOnlyDir(dir string) error {
	if err := os.MkdirAll(dir, privateDirPerm); err != nil {
		return err
	}
	return os.Chmod(dir, privateDirPerm)
}

// TokenRecord holds the data associated with a stored magic link token.
type TokenRecord struct {
	Email     string
	TenantID  string
	ExpiresAt time.Time
	Used      bool
}

// Store persists magic link tokens in SQLite.
// Tokens are identified by HMAC-SHA256(rawToken) stored as hex.
type Store struct {
	db          *sql.DB
	stopCleanup chan struct{}
	mu          sync.Mutex
}

// NewStore opens (or creates) the magic link token database in dir.
func NewStore(dir string) (*Store, error) {
	dir = filepath.Clean(dir)
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("dir is required")
	}
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return nil, fmt.Errorf("create magic link store dir: %w", err)
	}

	dbPath := filepath.Join(dir, "cp_magic_links.db")
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open magic link db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &Store{
		db:          db,
		stopCleanup: make(chan struct{}),
	}
	if err := s.initSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close magic link db after schema init failure: %w", closeErr))
		}
		return nil, err
	}

	go s.cleanupLoop()
	return s, nil
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS magic_link_tokens (
		token_hash TEXT PRIMARY KEY,
		email TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		expires_at INTEGER NOT NULL,
		used INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		used_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_cp_ml_expires_at ON magic_link_tokens(expires_at);
	CREATE INDEX IF NOT EXISTS idx_cp_ml_email ON magic_link_tokens(email);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("init magic link schema: %w", err)
	}
	return nil
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(storeCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.DeleteExpired(time.Now().UTC()); err != nil {
				log.Warn().Err(err).Msg("Failed to delete expired magic link tokens")
			}
		case <-s.stopCleanup:
			return
		}
	}
}

// Put inserts or replaces a token record.
func (s *Store) Put(tokenHash []byte, rec *TokenRecord) error {
	if s == nil {
		return fmt.Errorf("store not configured")
	}
	if len(tokenHash) == 0 {
		return fmt.Errorf("tokenHash is required")
	}
	if rec == nil || rec.Email == "" || rec.TenantID == "" || rec.ExpiresAt.IsZero() {
		return fmt.Errorf("token record is required")
	}

	key := hex.EncodeToString(tokenHash)
	now := time.Now().UTC().Unix()

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return fmt.Errorf("store not configured")
	}
	defer s.mu.Unlock()
	_, err := db.Exec(
		`INSERT OR REPLACE INTO magic_link_tokens (token_hash, email, tenant_id, expires_at, used, created_at, used_at)
		 VALUES (?, ?, ?, ?, 0, ?, NULL)`,
		key, rec.Email, rec.TenantID, rec.ExpiresAt.UTC().Unix(), now,
	)
	if err != nil {
		return fmt.Errorf("put magic link token: %w", err)
	}
	return nil
}

// Consume atomically validates and marks a token as used. Returns the token record on success.
func (s *Store) Consume(tokenHash []byte, now time.Time) (*TokenRecord, error) {
	if s == nil {
		return nil, ErrTokenInvalid
	}
	if len(tokenHash) == 0 {
		return nil, ErrTokenInvalid
	}

	key := hex.EncodeToString(tokenHash)
	now = now.UTC()

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return nil, ErrTokenInvalid
	}
	defer s.mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin consume tx: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			log.Warn().Err(rollbackErr).Msg("Failed to rollback magic link consume transaction")
		}
	}()

	var email, tenantID string
	var expiresAtUnix int64
	var usedInt int

	row := tx.QueryRow(`SELECT email, tenant_id, expires_at, used FROM magic_link_tokens WHERE token_hash = ?`, key)
	if scanErr := row.Scan(&email, &tenantID, &expiresAtUnix, &usedInt); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		return nil, fmt.Errorf("load magic link token: %w", scanErr)
	}

	expiresAt := time.Unix(expiresAtUnix, 0).UTC()
	if now.After(expiresAt) {
		return nil, ErrTokenExpired
	}
	if usedInt != 0 {
		return nil, ErrTokenUsed
	}

	res, err := tx.Exec(`UPDATE magic_link_tokens SET used = 1, used_at = ? WHERE token_hash = ? AND used = 0`, now.Unix(), key)
	if err != nil {
		return nil, fmt.Errorf("mark magic link token used: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get consume update rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrTokenUsed
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit consume tx: %w", err)
	}

	return &TokenRecord{
		Email:     email,
		TenantID:  tenantID,
		ExpiresAt: expiresAt,
		Used:      true,
	}, nil
}

// DeleteExpired removes tokens that have passed their expiry time.
func (s *Store) DeleteExpired(now time.Time) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return nil
	}
	defer s.mu.Unlock()
	if _, err := db.Exec(`DELETE FROM magic_link_tokens WHERE expires_at < ?`, now.UTC().Unix()); err != nil {
		return fmt.Errorf("delete expired magic link tokens: %w", err)
	}
	return nil
}

// Close stops the background cleanup goroutine and closes the database.
func (s *Store) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.stopCleanup:
	default:
		close(s.stopCleanup)
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close magic link store database")
		}
		s.db = nil
	}
}
