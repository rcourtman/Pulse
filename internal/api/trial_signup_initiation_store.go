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

const trialSignupInitiationTTL = 2 * time.Hour

type trialSignupInitiationStore struct {
	once sync.Once
	db   *sql.DB
	mu   sync.Mutex

	configDir string
	initErr   error
}

func (s *trialSignupInitiationStore) init() {
	s.once.Do(func() {
		dir := filepath.Clean(s.configDir)
		if strings.TrimSpace(dir) == "" {
			s.initErr = fmt.Errorf("configDir is required")
			return
		}
		secretsDir := filepath.Join(dir, "secrets")
		if err := os.MkdirAll(secretsDir, handoffPrivateDirPerm); err != nil {
			s.initErr = fmt.Errorf("create trial signup secrets dir: %w", err)
			return
		}
		if err := os.Chmod(secretsDir, handoffPrivateDirPerm); err != nil {
			s.initErr = fmt.Errorf("chmod trial signup secrets dir: %w", err)
			return
		}

		dbPath := filepath.Join(secretsDir, "trial_signup_state.db")
		dsn := dbPath + "?" + url.Values{
			"_pragma": []string{
				"busy_timeout(30000)",
				"journal_mode(WAL)",
				"synchronous(NORMAL)",
			},
		}.Encode()

		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			s.initErr = fmt.Errorf("open trial signup initiation db: %w", err)
			return
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)

		schema := `
		CREATE TABLE IF NOT EXISTS trial_signup_initiations (
			token_hash TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_trial_signup_initiations_expires_at ON trial_signup_initiations(expires_at);
		`
		if _, err := db.Exec(schema); err != nil {
			_ = db.Close()
			s.initErr = fmt.Errorf("init trial signup initiation schema: %w", err)
			return
		}
		for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
			if err := hardenPrivateFile(path, handoffPrivateFilePerm); err != nil {
				_ = db.Close()
				s.initErr = fmt.Errorf("harden trial signup initiation file permissions: %w", err)
				return
			}
		}
		s.db = db
	})
}

func (s *trialSignupInitiationStore) issue(orgID string, expiresAt time.Time) (string, error) {
	s.init()
	if s.initErr != nil {
		return "", s.initErr
	}
	if s.db == nil {
		return "", fmt.Errorf("trial signup initiation store not initialized")
	}
	orgID = normalizeTrialSignupInitiationOrgID(orgID)
	expiresAt = expiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(trialSignupInitiationTTL)
	}

	rawToken, err := randomTrialSignupInitiationToken()
	if err != nil {
		return "", err
	}
	tokenHash := hashTrialSignupInitiationToken(rawToken)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cleanupExpiredLocked(time.Now().UTC()); err != nil {
		return "", err
	}
	if _, err := s.db.Exec(
		`INSERT INTO trial_signup_initiations (token_hash, org_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash,
		orgID,
		expiresAt.Unix(),
	); err != nil {
		return "", fmt.Errorf("insert trial signup initiation: %w", err)
	}
	return rawToken, nil
}

func (s *trialSignupInitiationStore) validate(orgID, rawToken string, now time.Time) (bool, error) {
	s.init()
	if s.initErr != nil {
		return false, s.initErr
	}
	if s.db == nil {
		return false, fmt.Errorf("trial signup initiation store not initialized")
	}
	orgID = normalizeTrialSignupInitiationOrgID(orgID)
	tokenHash := hashTrialSignupInitiationToken(rawToken)
	if tokenHash == "" {
		return false, nil
	}
	now = now.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cleanupExpiredLocked(now); err != nil {
		return false, err
	}
	var expiresAt int64
	err := s.db.QueryRow(
		`SELECT expires_at FROM trial_signup_initiations WHERE token_hash = ? AND org_id = ?`,
		tokenHash,
		orgID,
	).Scan(&expiresAt)
	if err == nil {
		return expiresAt > now.Unix(), nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("lookup trial signup initiation: %w", err)
}

func (s *trialSignupInitiationStore) cleanupExpiredLocked(now time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM trial_signup_initiations WHERE expires_at <= ?`, now.UTC().Unix()); err != nil {
		return fmt.Errorf("cleanup trial signup initiations: %w", err)
	}
	return nil
}

func normalizeTrialSignupInitiationOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "default"
	}
	return orgID
}

func hashTrialSignupInitiationToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomTrialSignupInitiationToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate trial signup initiation token: %w", err)
	}
	return "tsi1_" + base64.RawURLEncoding.EncodeToString(buf), nil
}
