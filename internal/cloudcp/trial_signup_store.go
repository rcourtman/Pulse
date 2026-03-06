package cloudcp

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
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
	ErrTrialSignupVerificationInvalid = errors.New("trial signup verification token is invalid")
	ErrTrialSignupVerificationExpired = errors.New("trial signup verification token is expired")
	ErrTrialSignupVerificationUsed    = errors.New("trial signup verification token already used")
	ErrTrialSignupRecordNotFound      = errors.New("trial signup record not found")
	ErrTrialSignupEmailAlreadyUsed    = errors.New("trial signup email already used")
)

const (
	trialSignupStoreCleanupInterval = 10 * time.Minute
	trialSignupStorePrivateDirPerm  = 0o700
	trialSignupTokenPrefix          = "tsv1_"
)

type TrialSignupRecord struct {
	ID                    string
	OrgID                 string
	ReturnURL             string
	Name                  string
	Email                 string
	Company               string
	VerificationExpiresAt time.Time
	CreatedAt             time.Time
	VerifiedAt            time.Time
	CheckoutStartedAt     time.Time
}

type TrialSignupStore struct {
	db          *sql.DB
	stopCleanup chan struct{}
	mu          sync.Mutex
}

func NewTrialSignupStore(dir string) (*TrialSignupStore, error) {
	dir = filepath.Clean(dir)
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("dir is required")
	}
	if err := ensureTrialSignupStoreDir(dir); err != nil {
		return nil, fmt.Errorf("create trial signup store dir: %w", err)
	}

	dbPath := filepath.Join(dir, "trial_signup.db")
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open trial signup db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &TrialSignupStore{
		db:          db,
		stopCleanup: make(chan struct{}),
	}
	if err := s.initSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close trial signup db after schema init failure: %w", closeErr))
		}
		return nil, err
	}

	go s.cleanupLoop()
	return s, nil
}

func ensureTrialSignupStoreDir(dir string) error {
	if err := os.MkdirAll(dir, trialSignupStorePrivateDirPerm); err != nil {
		return err
	}
	return os.Chmod(dir, trialSignupStorePrivateDirPerm)
}

func (s *TrialSignupStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS trial_signup_requests (
		request_id TEXT PRIMARY KEY,
		verification_token_hash TEXT NOT NULL UNIQUE,
		org_id TEXT NOT NULL,
		return_url TEXT NOT NULL,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		company TEXT NOT NULL DEFAULT '',
		verification_expires_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		verified_at INTEGER,
		checkout_started_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_trial_signup_email ON trial_signup_requests(email);
	CREATE INDEX IF NOT EXISTS idx_trial_signup_verify_expiry ON trial_signup_requests(verification_expires_at);
	CREATE INDEX IF NOT EXISTS idx_trial_signup_verified_at ON trial_signup_requests(verified_at);
	CREATE TABLE IF NOT EXISTS trial_signup_issuances (
		email TEXT PRIMARY KEY,
		request_id TEXT NOT NULL UNIQUE,
		issued_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_trial_signup_issuances_request_id ON trial_signup_issuances(request_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("init trial signup schema: %w", err)
	}
	return nil
}

func (s *TrialSignupStore) cleanupLoop() {
	ticker := time.NewTicker(trialSignupStoreCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.DeleteExpired(time.Now().UTC()); err != nil {
				log.Warn().Err(err).Msg("Failed to delete expired trial signup requests")
			}
		case <-s.stopCleanup:
			return
		}
	}
}

func (s *TrialSignupStore) CreateVerification(rec *TrialSignupRecord) (string, error) {
	if s == nil {
		return "", fmt.Errorf("trial signup store not configured")
	}
	if rec == nil {
		return "", fmt.Errorf("trial signup record is required")
	}
	requestID, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("generate request id: %w", err)
	}
	rawToken, err := randomPrefixedToken()
	if err != nil {
		return "", fmt.Errorf("generate verification token: %w", err)
	}
	tokenHash := hashTrialSignupToken(rawToken)

	now := rec.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiresAt := rec.VerificationExpiresAt.UTC()
	if expiresAt.IsZero() {
		return "", fmt.Errorf("verification expiry is required")
	}
	email := normalizeTrialSignupEmail(rec.Email)
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return "", fmt.Errorf("trial signup store not configured")
	}
	defer s.mu.Unlock()

	_, err = db.Exec(
		`INSERT INTO trial_signup_requests (
			request_id, verification_token_hash, org_id, return_url, name, email, company,
			verification_expires_at, created_at, verified_at, checkout_started_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL)`,
		requestID,
		tokenHash,
		rec.OrgID,
		rec.ReturnURL,
		rec.Name,
		email,
		rec.Company,
		expiresAt.Unix(),
		now.Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("insert trial signup request: %w", err)
	}
	rec.ID = requestID
	rec.CreatedAt = now
	rec.VerificationExpiresAt = expiresAt
	rec.Email = email
	return rawToken, nil
}

func (s *TrialSignupStore) ConsumeVerification(rawToken string, now time.Time) (*TrialSignupRecord, error) {
	if s == nil {
		return nil, ErrTrialSignupVerificationInvalid
	}
	tokenHash := hashTrialSignupToken(rawToken)
	if tokenHash == "" {
		return nil, ErrTrialSignupVerificationInvalid
	}
	now = now.UTC()

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return nil, ErrTrialSignupVerificationInvalid
	}
	defer s.mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin trial signup consume tx: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			log.Warn().Err(rollbackErr).Msg("Failed to rollback trial signup consume transaction")
		}
	}()

	rec, err := loadTrialSignupRecord(tx.QueryRow(
		`SELECT request_id, org_id, return_url, name, email, company, verification_expires_at, created_at, verified_at, checkout_started_at
		 FROM trial_signup_requests
		 WHERE verification_token_hash = ?`,
		tokenHash,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTrialSignupVerificationInvalid
		}
		return nil, fmt.Errorf("load trial signup request: %w", err)
	}
	if now.After(rec.VerificationExpiresAt) {
		return nil, ErrTrialSignupVerificationExpired
	}
	if !rec.VerifiedAt.IsZero() {
		return nil, ErrTrialSignupVerificationUsed
	}

	res, err := tx.Exec(
		`UPDATE trial_signup_requests
		 SET verified_at = ?
		 WHERE request_id = ? AND verified_at IS NULL`,
		now.Unix(),
		rec.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark trial signup verified: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get trial signup update rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrTrialSignupVerificationUsed
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit trial signup consume tx: %w", err)
	}
	rec.VerifiedAt = now
	return rec, nil
}

func (s *TrialSignupStore) GetRecord(id string) (*TrialSignupRecord, error) {
	if s == nil {
		return nil, ErrTrialSignupRecordNotFound
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrTrialSignupRecordNotFound
	}

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return nil, ErrTrialSignupRecordNotFound
	}
	defer s.mu.Unlock()

	rec, err := loadTrialSignupRecord(db.QueryRow(
		`SELECT request_id, org_id, return_url, name, email, company, verification_expires_at, created_at, verified_at, checkout_started_at
		 FROM trial_signup_requests
		 WHERE request_id = ?`,
		id,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTrialSignupRecordNotFound
		}
		return nil, fmt.Errorf("load trial signup request: %w", err)
	}
	return rec, nil
}

func (s *TrialSignupStore) MarkCheckoutStarted(id string, now time.Time) error {
	if s == nil {
		return fmt.Errorf("trial signup store not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrTrialSignupRecordNotFound
	}
	now = now.UTC()

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return fmt.Errorf("trial signup store not configured")
	}
	defer s.mu.Unlock()

	res, err := db.Exec(
		`UPDATE trial_signup_requests
		 SET checkout_started_at = COALESCE(checkout_started_at, ?)
		 WHERE request_id = ? AND verified_at IS NOT NULL`,
		now.Unix(),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark trial signup checkout started: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get trial signup checkout rows affected: %w", err)
	}
	if affected == 0 {
		return ErrTrialSignupRecordNotFound
	}
	return nil
}

func (s *TrialSignupStore) HasIssuedTrialForEmail(email string) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("trial signup store not configured")
	}
	email = normalizeTrialSignupEmail(email)
	if email == "" {
		return false, nil
	}

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return false, fmt.Errorf("trial signup store not configured")
	}
	defer s.mu.Unlock()

	var issuedAt int64
	err := db.QueryRow(
		`SELECT issued_at
		 FROM trial_signup_issuances
		 WHERE email = ?`,
		email,
	).Scan(&issuedAt)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("lookup trial signup issuance: %w", err)
}

func (s *TrialSignupStore) MarkTrialIssued(id string, now time.Time) error {
	if s == nil {
		return fmt.Errorf("trial signup store not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrTrialSignupRecordNotFound
	}
	now = now.UTC()

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return fmt.Errorf("trial signup store not configured")
	}
	defer s.mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin trial issuance tx: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			log.Warn().Err(rollbackErr).Msg("Failed to rollback trial issuance transaction")
		}
	}()

	rec, err := loadTrialSignupRecord(tx.QueryRow(
		`SELECT request_id, org_id, return_url, name, email, company, verification_expires_at, created_at, verified_at, checkout_started_at
		 FROM trial_signup_requests
		 WHERE request_id = ?`,
		id,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTrialSignupRecordNotFound
		}
		return fmt.Errorf("load trial signup request for issuance: %w", err)
	}
	if rec.VerifiedAt.IsZero() {
		return ErrTrialSignupVerificationInvalid
	}

	email := normalizeTrialSignupEmail(rec.Email)
	if email == "" {
		return ErrTrialSignupVerificationInvalid
	}

	res, err := tx.Exec(
		`INSERT OR IGNORE INTO trial_signup_issuances (email, request_id, issued_at)
		 VALUES (?, ?, ?)`,
		email,
		rec.ID,
		now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert trial issuance: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get trial issuance rows affected: %w", err)
	}
	if affected == 0 {
		var existingRequestID string
		err = tx.QueryRow(
			`SELECT request_id
			 FROM trial_signup_issuances
			 WHERE email = ?`,
			email,
		).Scan(&existingRequestID)
		if err != nil {
			return fmt.Errorf("lookup existing trial issuance: %w", err)
		}
		if strings.TrimSpace(existingRequestID) != rec.ID {
			return ErrTrialSignupEmailAlreadyUsed
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit trial issuance tx: %w", err)
	}
	return nil
}

func (s *TrialSignupStore) DeleteExpired(now time.Time) error {
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
	_, err := db.Exec(
		`DELETE FROM trial_signup_requests
		 WHERE verified_at IS NULL AND verification_expires_at < ?`,
		now.UTC().Unix(),
	)
	if err != nil {
		return fmt.Errorf("delete expired trial signup requests: %w", err)
	}
	return nil
}

func (s *TrialSignupStore) Close() {
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
			log.Warn().Err(err).Msg("Failed to close trial signup store database")
		}
		s.db = nil
	}
}

func loadTrialSignupRecord(scanner interface {
	Scan(dest ...any) error
}) (*TrialSignupRecord, error) {
	rec := &TrialSignupRecord{}
	var verificationExpiresAt, createdAt int64
	var verifiedAt, checkoutStartedAt sql.NullInt64
	if err := scanner.Scan(
		&rec.ID,
		&rec.OrgID,
		&rec.ReturnURL,
		&rec.Name,
		&rec.Email,
		&rec.Company,
		&verificationExpiresAt,
		&createdAt,
		&verifiedAt,
		&checkoutStartedAt,
	); err != nil {
		return nil, err
	}
	rec.VerificationExpiresAt = time.Unix(verificationExpiresAt, 0).UTC()
	rec.CreatedAt = time.Unix(createdAt, 0).UTC()
	if verifiedAt.Valid {
		rec.VerifiedAt = time.Unix(verifiedAt.Int64, 0).UTC()
	}
	if checkoutStartedAt.Valid {
		rec.CheckoutStartedAt = time.Unix(checkoutStartedAt.Int64, 0).UTC()
	}
	return rec, nil
}

func hashTrialSignupToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomPrefixedToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return trialSignupTokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func normalizeTrialSignupEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
