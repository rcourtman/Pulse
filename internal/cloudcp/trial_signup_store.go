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
	"regexp"
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
	ErrTrialSignupOrganizationUsed    = errors.New("trial signup organization already used")
)

const (
	trialSignupStoreCleanupInterval = 10 * time.Minute
	trialSignupStorePrivateDirPerm  = 0o700
	trialSignupTokenPrefix          = "tsv1_"
)

var (
	trialSignupCompanyNoiseTokens = map[string]struct{}{
		"and": {}, "co": {}, "company": {}, "corp": {}, "corporation": {}, "gmbh": {}, "group": {},
		"holding": {}, "holdings": {}, "inc": {}, "incorporated": {}, "limited": {}, "llc": {},
		"llp": {}, "ltd": {}, "plc": {}, "pte": {}, "pty": {}, "sa": {}, "sas": {}, "sarl": {},
	}
	trialSignupPublicEmailDomains = map[string]struct{}{
		"aol.com": {}, "fastmail.com": {}, "gmail.com": {}, "gmx.com": {}, "googlemail.com": {},
		"hey.com": {}, "hotmail.com": {}, "icloud.com": {}, "live.com": {}, "mac.com": {},
		"mail.com": {}, "me.com": {}, "msn.com": {}, "outlook.com": {}, "pm.me": {},
		"proton.me": {}, "protonmail.com": {}, "rocketmail.com": {}, "yahoo.com": {},
		"ymail.com": {}, "zoho.com": {},
	}
	trialSignupNonAlnumPattern = regexp.MustCompile(`[^a-z0-9]+`)
)

type trialSignupIssuanceConflictKind string

const (
	trialSignupConflictNone          trialSignupIssuanceConflictKind = ""
	trialSignupConflictEmail         trialSignupIssuanceConflictKind = "email"
	trialSignupConflictDomain        trialSignupIssuanceConflictKind = "domain"
	trialSignupConflictPublicCompany trialSignupIssuanceConflictKind = "public_company"
)

type TrialSignupIssuanceConflict struct {
	RequestID string
	Kind      trialSignupIssuanceConflictKind
}

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
		email_domain TEXT NOT NULL DEFAULT '',
		company_key TEXT NOT NULL DEFAULT '',
		issued_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_trial_signup_issuances_request_id ON trial_signup_issuances(request_id);
	CREATE INDEX IF NOT EXISTS idx_trial_signup_issuances_domain ON trial_signup_issuances(email_domain);
	CREATE INDEX IF NOT EXISTS idx_trial_signup_issuances_company_key ON trial_signup_issuances(company_key);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("init trial signup schema: %w", err)
	}
	hasDomain, err := s.tableHasColumn("trial_signup_issuances", "email_domain")
	if err != nil {
		return fmt.Errorf("check trial_signup_issuances schema for email_domain: %w", err)
	}
	if !hasDomain {
		if _, err := s.db.Exec(`ALTER TABLE trial_signup_issuances ADD COLUMN email_domain TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate trial_signup_issuances: add email_domain: %w", err)
		}
	}
	hasCompanyKey, err := s.tableHasColumn("trial_signup_issuances", "company_key")
	if err != nil {
		return fmt.Errorf("check trial_signup_issuances schema for company_key: %w", err)
	}
	if !hasCompanyKey {
		if _, err := s.db.Exec(`ALTER TABLE trial_signup_issuances ADD COLUMN company_key TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate trial_signup_issuances: add company_key: %w", err)
		}
	}
	if err := s.backfillTrialSignupIssuanceIdentities(); err != nil {
		return fmt.Errorf("backfill trial signup issuance identities: %w", err)
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

func (s *TrialSignupStore) FindIssuedTrialConflict(email, company string) (*TrialSignupIssuanceConflict, error) {
	if s == nil {
		return nil, fmt.Errorf("trial signup store not configured")
	}
	email = normalizeTrialSignupEmail(email)
	if email == "" {
		return nil, nil
	}

	s.mu.Lock()
	db := s.db
	if db == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("trial signup store not configured")
	}
	defer s.mu.Unlock()

	return findTrialSignupIssuanceConflict(db, email, company, "")
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
	conflict, err := findTrialSignupIssuanceConflict(tx, email, rec.Company, rec.ID)
	if err != nil {
		return fmt.Errorf("find trial issuance conflict: %w", err)
	}
	if conflict != nil {
		switch conflict.Kind {
		case trialSignupConflictEmail:
			return ErrTrialSignupEmailAlreadyUsed
		case trialSignupConflictDomain, trialSignupConflictPublicCompany:
			return ErrTrialSignupOrganizationUsed
		default:
			return ErrTrialSignupOrganizationUsed
		}
	}
	emailDomain := normalizeTrialSignupEmailDomain(email)
	companyKey := normalizeTrialSignupCompany(rec.Company)

	res, err := tx.Exec(
		`INSERT OR IGNORE INTO trial_signup_issuances (email, request_id, email_domain, company_key, issued_at)
		 VALUES (?, ?, ?, ?, ?)`,
		email,
		rec.ID,
		emailDomain,
		companyKey,
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
		conflict, conflictErr := findTrialSignupIssuanceConflict(tx, email, rec.Company, rec.ID)
		if conflictErr != nil {
			return fmt.Errorf("lookup existing trial issuance: %w", conflictErr)
		}
		if conflict != nil {
			switch conflict.Kind {
			case trialSignupConflictEmail:
				return ErrTrialSignupEmailAlreadyUsed
			default:
				return ErrTrialSignupOrganizationUsed
			}
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

func normalizeTrialSignupEmailDomain(email string) string {
	email = normalizeTrialSignupEmail(email)
	at := strings.LastIndex(email, "@")
	if at == -1 || at == len(email)-1 {
		return ""
	}
	return strings.TrimSpace(email[at+1:])
}

func normalizeTrialSignupCompany(company string) string {
	raw := strings.ToLower(strings.TrimSpace(company))
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "&", " and ")
	raw = trialSignupNonAlnumPattern.ReplaceAllString(raw, " ")
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, noise := trialSignupCompanyNoiseTokens[field]; noise {
			continue
		}
		normalized = append(normalized, field)
	}
	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, " ")
}

func isPublicTrialSignupEmailDomain(domain string) bool {
	_, ok := trialSignupPublicEmailDomains[strings.ToLower(strings.TrimSpace(domain))]
	return ok
}

func findTrialSignupIssuanceConflict(
	queryable interface {
		QueryRow(query string, args ...any) *sql.Row
	},
	email, company, excludeRequestID string,
) (*TrialSignupIssuanceConflict, error) {
	email = normalizeTrialSignupEmail(email)
	if email == "" {
		return nil, nil
	}
	excludeRequestID = strings.TrimSpace(excludeRequestID)

	requestID, err := queryIssuanceConflictRequestID(queryable,
		`SELECT request_id FROM trial_signup_issuances
		 WHERE email = ? AND (? = '' OR request_id <> ?)
		 LIMIT 1`,
		email, excludeRequestID, excludeRequestID,
	)
	if err != nil {
		return nil, err
	}
	if requestID != "" {
		return &TrialSignupIssuanceConflict{RequestID: requestID, Kind: trialSignupConflictEmail}, nil
	}

	domain := normalizeTrialSignupEmailDomain(email)
	if domain != "" && !isPublicTrialSignupEmailDomain(domain) {
		requestID, err = queryIssuanceConflictRequestID(queryable,
			`SELECT request_id FROM trial_signup_issuances
			 WHERE email_domain = ? AND (? = '' OR request_id <> ?)
			 LIMIT 1`,
			domain, excludeRequestID, excludeRequestID,
		)
		if err != nil {
			return nil, err
		}
		if requestID != "" {
			return &TrialSignupIssuanceConflict{RequestID: requestID, Kind: trialSignupConflictDomain}, nil
		}
		return nil, nil
	}

	companyKey := normalizeTrialSignupCompany(company)
	if companyKey == "" {
		return nil, nil
	}
	requestID, err = queryIssuanceConflictRequestID(queryable,
		`SELECT request_id FROM trial_signup_issuances
		 WHERE company_key = ? AND company_key <> '' AND (? = '' OR request_id <> ?)
		 LIMIT 1`,
		companyKey, excludeRequestID, excludeRequestID,
	)
	if err != nil {
		return nil, err
	}
	if requestID != "" {
		return &TrialSignupIssuanceConflict{RequestID: requestID, Kind: trialSignupConflictPublicCompany}, nil
	}
	return nil, nil
}

func queryIssuanceConflictRequestID(
	queryable interface {
		QueryRow(query string, args ...any) *sql.Row
	},
	query string,
	args ...any,
) (string, error) {
	var requestID string
	err := queryable.QueryRow(query, args...).Scan(&requestID)
	if err == nil {
		return strings.TrimSpace(requestID), nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return "", err
}

func (s *TrialSignupStore) tableHasColumn(tableName, columnName string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typeName string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(columnName)) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *TrialSignupStore) backfillTrialSignupIssuanceIdentities() error {
	rows, err := s.db.Query(`
		SELECT i.request_id, i.email, i.email_domain, i.company_key, COALESCE(r.company, '')
		FROM trial_signup_issuances i
		LEFT JOIN trial_signup_requests r ON r.request_id = i.request_id
		WHERE i.email_domain = '' OR i.company_key = ''
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type rowData struct {
		requestID   string
		email       string
		emailDomain string
		companyKey  string
		company     string
	}
	var updates []rowData
	for rows.Next() {
		var row rowData
		if err := rows.Scan(&row.requestID, &row.email, &row.emailDomain, &row.companyKey, &row.company); err != nil {
			return err
		}
		updates = append(updates, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, row := range updates {
		emailDomain := row.emailDomain
		if strings.TrimSpace(emailDomain) == "" {
			emailDomain = normalizeTrialSignupEmailDomain(row.email)
		}
		companyKey := row.companyKey
		if strings.TrimSpace(companyKey) == "" {
			companyKey = normalizeTrialSignupCompany(row.company)
		}
		if _, err := s.db.Exec(
			`UPDATE trial_signup_issuances
			 SET email_domain = ?, company_key = ?
			 WHERE request_id = ?`,
			emailDomain,
			companyKey,
			row.requestID,
		); err != nil {
			return err
		}
	}
	return nil
}
