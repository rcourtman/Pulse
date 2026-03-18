package api

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

	_ "modernc.org/sqlite"
)

// SQLiteMagicLinkStore persists short-lived magic link tokens in SQLite.
//
// Tokens are identified by token_hash (HMAC-SHA256(token)), encoded as hex text.
// The raw token is never stored.
type SQLiteMagicLinkStore struct {
	db     *sql.DB
	dbPath string

	stopCleanup chan struct{}
	mu          sync.Mutex
}

func NewSQLiteMagicLinkStore(dataPath string) (*SQLiteMagicLinkStore, error) {
	dataPath = filepath.Clean(dataPath)
	if strings.TrimSpace(dataPath) == "" {
		return nil, fmt.Errorf("dataPath is required")
	}
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("create magic link data dir: %w", err)
	}

	dbPath := filepath.Join(dataPath, "magic_links.db")
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

	s := &SQLiteMagicLinkStore{
		db:          db,
		dbPath:      dbPath,
		stopCleanup: make(chan struct{}),
	}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	go s.cleanupLoop()

	return s, nil
}

func (s *SQLiteMagicLinkStore) initSchema() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite magic link store not initialized")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS magic_link_tokens (
		token_hash TEXT PRIMARY KEY,
		email TEXT NOT NULL,
		org_id TEXT NOT NULL,
		expires_at INTEGER NOT NULL,
		used INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		used_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_expires_at ON magic_link_tokens(expires_at);
	CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_email ON magic_link_tokens(email);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("init magic link schema: %w", err)
	}
	return nil
}

func (s *SQLiteMagicLinkStore) cleanupLoop() {
	ticker := time.NewTicker(magicLinkStoreCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.DeleteExpired(time.Now().UTC())
		case <-s.stopCleanup:
			return
		}
	}
}

func (s *SQLiteMagicLinkStore) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.stopCleanup:
		// already stopped
	default:
		close(s.stopCleanup)
	}
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
}

func (s *SQLiteMagicLinkStore) Put(tokenHash []byte, token *MagicLinkToken) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("magic link store not configured")
	}
	if len(tokenHash) == 0 {
		return fmt.Errorf("tokenHash is required")
	}
	if token == nil || token.Email == "" || token.OrgID == "" || token.ExpiresAt.IsZero() {
		return fmt.Errorf("token record is required")
	}

	key := hex.EncodeToString(tokenHash)
	now := time.Now().UTC().Unix()
	expiresAt := token.ExpiresAt.UTC().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO magic_link_tokens (token_hash, email, org_id, expires_at, used, created_at, used_at)
		 VALUES (?, ?, ?, ?, 0, ?, NULL)`,
		key,
		token.Email,
		token.OrgID,
		expiresAt,
		now,
	)
	if err != nil {
		return fmt.Errorf("put magic link token: %w", err)
	}
	return nil
}

func (s *SQLiteMagicLinkStore) Consume(tokenHash []byte, now time.Time) (*MagicLinkToken, error) {
	if s == nil || s.db == nil {
		return nil, ErrMagicLinkInvalidToken
	}
	if len(tokenHash) == 0 {
		return nil, ErrMagicLinkInvalidToken
	}
	key := hex.EncodeToString(tokenHash)
	now = now.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin consume tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var email, orgID string
	var expiresAtUnix int64
	var usedInt int

	row := tx.QueryRow(`SELECT email, org_id, expires_at, used FROM magic_link_tokens WHERE token_hash = ?`, key)
	if scanErr := row.Scan(&email, &orgID, &expiresAtUnix, &usedInt); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil, ErrMagicLinkInvalidToken
		}
		return nil, fmt.Errorf("load magic link token: %w", scanErr)
	}

	expiresAt := time.Unix(expiresAtUnix, 0).UTC()
	if now.After(expiresAt) {
		return nil, ErrMagicLinkExpired
	}
	if usedInt != 0 {
		return nil, ErrMagicLinkUsed
	}

	res, err := tx.Exec(`UPDATE magic_link_tokens SET used = 1, used_at = ? WHERE token_hash = ? AND used = 0`, now.Unix(), key)
	if err != nil {
		return nil, fmt.Errorf("mark magic link token used: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, ErrMagicLinkUsed
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit consume tx: %w", err)
	}

	return &MagicLinkToken{
		Email:     email,
		OrgID:     orgID,
		ExpiresAt: expiresAt,
		Used:      true,
		Token:     "",
	}, nil
}

func (s *SQLiteMagicLinkStore) DeleteExpired(now time.Time) {
	if s == nil || s.db == nil {
		return
	}
	now = now.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`DELETE FROM magic_link_tokens WHERE expires_at < ?`, now.Unix())
}
