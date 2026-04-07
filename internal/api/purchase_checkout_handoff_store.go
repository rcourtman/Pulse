package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const purchaseCheckoutHandoffTTL = 2 * time.Hour

type purchaseCheckoutHandoffRecord struct {
	Feature               string
	ActivationURLTemplate string
}

type purchaseCheckoutHandoffStore struct {
	once sync.Once
	db   *sql.DB
	mu   sync.Mutex

	configDir string
	initErr   error
}

func (s *purchaseCheckoutHandoffStore) init() {
	s.once.Do(func() {
		dir := filepath.Clean(s.configDir)
		if strings.TrimSpace(dir) == "" {
			s.initErr = fmt.Errorf("configDir is required")
			return
		}
		secretsDir := filepath.Join(dir, "secrets")
		if err := os.MkdirAll(secretsDir, handoffPrivateDirPerm); err != nil {
			s.initErr = fmt.Errorf("create purchase handoff secrets dir: %w", err)
			return
		}
		if err := os.Chmod(secretsDir, handoffPrivateDirPerm); err != nil {
			s.initErr = fmt.Errorf("chmod purchase handoff secrets dir: %w", err)
			return
		}

		dbPath := filepath.Join(secretsDir, "purchase_checkout_handoff.db")
		dsn := dbPath + "?" + url.Values{
			"_pragma": []string{
				"busy_timeout(30000)",
				"journal_mode(WAL)",
				"synchronous(NORMAL)",
			},
		}.Encode()

		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			s.initErr = fmt.Errorf("open purchase handoff db: %w", err)
			return
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)

		schema := `
		CREATE TABLE IF NOT EXISTS purchase_checkout_handoffs (
			handoff_hash TEXT PRIMARY KEY,
			feature TEXT NOT NULL,
			activation_url_template TEXT NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_purchase_checkout_handoffs_expires_at
			ON purchase_checkout_handoffs(expires_at);
		`
		if _, err := db.Exec(schema); err != nil {
			_ = db.Close()
			s.initErr = fmt.Errorf("init purchase handoff schema: %w", err)
			return
		}
		for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
			if err := hardenPrivateFile(path, handoffPrivateFilePerm); err != nil {
				_ = db.Close()
				s.initErr = fmt.Errorf("harden purchase handoff file permissions: %w", err)
				return
			}
		}

		s.db = db
	})
}

func (s *purchaseCheckoutHandoffStore) issue(
	feature string,
	activationURLTemplate string,
	expiresAt time.Time,
) (string, error) {
	s.init()
	if s.initErr != nil {
		return "", s.initErr
	}
	if s.db == nil {
		return "", fmt.Errorf("purchase handoff store not initialized")
	}
	activationURLTemplate = strings.TrimSpace(activationURLTemplate)
	if activationURLTemplate == "" {
		return "", fmt.Errorf("activation_url_template is required")
	}
	expiresAt = expiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(purchaseCheckoutHandoffTTL)
	}

	rawID, err := randomPurchaseCheckoutHandoffID()
	if err != nil {
		return "", err
	}
	handoffHash := hashPurchaseCheckoutHandoffID(rawID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cleanupExpiredLocked(time.Now().UTC()); err != nil {
		return "", err
	}
	if _, err := s.db.Exec(
		`INSERT INTO purchase_checkout_handoffs (handoff_hash, feature, activation_url_template, expires_at)
		 VALUES (?, ?, ?, ?)`,
		handoffHash,
		strings.TrimSpace(feature),
		activationURLTemplate,
		expiresAt.Unix(),
	); err != nil {
		return "", fmt.Errorf("insert purchase handoff: %w", err)
	}
	return rawID, nil
}

func (s *purchaseCheckoutHandoffStore) resolve(
	rawID string,
	now time.Time,
) (*purchaseCheckoutHandoffRecord, bool, error) {
	s.init()
	if s.initErr != nil {
		return nil, false, s.initErr
	}
	if s.db == nil {
		return nil, false, fmt.Errorf("purchase handoff store not initialized")
	}
	handoffHash := hashPurchaseCheckoutHandoffID(rawID)
	if handoffHash == "" {
		return nil, false, nil
	}
	now = now.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cleanupExpiredLocked(now); err != nil {
		return nil, false, err
	}

	record := &purchaseCheckoutHandoffRecord{}
	var expiresAt int64
	err := s.db.QueryRow(
		`SELECT feature, activation_url_template, expires_at
		 FROM purchase_checkout_handoffs
		 WHERE handoff_hash = ?`,
		handoffHash,
	).Scan(&record.Feature, &record.ActivationURLTemplate, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lookup purchase handoff: %w", err)
	}
	if expiresAt <= now.Unix() {
		if _, deleteErr := s.db.Exec(
			`DELETE FROM purchase_checkout_handoffs WHERE handoff_hash = ?`,
			handoffHash,
		); deleteErr != nil {
			return nil, false, fmt.Errorf("delete expired purchase handoff: %w", deleteErr)
		}
		return nil, false, nil
	}
	return record, true, nil
}

func (s *purchaseCheckoutHandoffStore) cleanupExpiredLocked(now time.Time) error {
	if _, err := s.db.Exec(
		`DELETE FROM purchase_checkout_handoffs WHERE expires_at <= ?`,
		now.UTC().Unix(),
	); err != nil {
		return fmt.Errorf("cleanup purchase handoffs: %w", err)
	}
	return nil
}

func hashPurchaseCheckoutHandoffID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomPurchaseCheckoutHandoffID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate purchase handoff id: %w", err)
	}
	return "pch1_" + base64.RawURLEncoding.EncodeToString(buf), nil
}
