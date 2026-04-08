package api

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	purchaseReturnRedemptionStateStarted   = "started"
	purchaseReturnRedemptionStateActivated = "activated"
	purchaseReturnRedemptionStateFailed    = "failed"

	purchaseReturnRedemptionDecisionStarted          = "started"
	purchaseReturnRedemptionDecisionAlreadyActivated = "already_activated"
	purchaseReturnRedemptionDecisionInProgress       = "in_progress"
	purchaseReturnRedemptionDecisionConflict         = "conflict"

	purchaseReturnRedemptionStaleAfter = 30 * time.Second
)

type purchaseReturnRedemptionStore struct {
	once sync.Once
	db   *sql.DB
	mu   sync.Mutex

	configDir string
	initErr   error
}

type purchaseReturnRedemptionAttempt struct {
	PortalHandoffID     string
	PurchaseReturnJTI   string
	CheckoutSessionID   string
	LicenseID           string
	ActivationKeyPrefix string
	ExpiresAt           time.Time
}

type purchaseReturnRedemptionRecord struct {
	PortalHandoffID     string
	PurchaseReturnJTI   string
	CheckoutSessionID   string
	LicenseID           string
	ActivationKeyPrefix string
	Status              string
	FailureReason       string
	FailureMessage      string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ExpiresAt           time.Time
	RedeemedAt          *time.Time
}

func (s *purchaseReturnRedemptionStore) init() {
	s.once.Do(func() {
		dir := filepath.Clean(s.configDir)
		if strings.TrimSpace(dir) == "" {
			s.initErr = fmt.Errorf("configDir is required")
			return
		}
		secretsDir := filepath.Join(dir, "secrets")
		if err := os.MkdirAll(secretsDir, handoffPrivateDirPerm); err != nil {
			s.initErr = fmt.Errorf("create purchase return secrets dir: %w", err)
			return
		}
		if err := os.Chmod(secretsDir, handoffPrivateDirPerm); err != nil {
			s.initErr = fmt.Errorf("chmod purchase return secrets dir: %w", err)
			return
		}

		dbPath := filepath.Join(secretsDir, "purchase_return_redemptions.db")
		dsn := dbPath + "?" + url.Values{
			"_pragma": []string{
				"busy_timeout(30000)",
				"journal_mode(WAL)",
				"synchronous(NORMAL)",
			},
		}.Encode()

		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			s.initErr = fmt.Errorf("open purchase return redemption db: %w", err)
			return
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)

		schema := `
		CREATE TABLE IF NOT EXISTS purchase_return_redemptions (
			portal_handoff_id TEXT NOT NULL,
			purchase_return_jti TEXT NOT NULL,
			checkout_session_id TEXT NOT NULL,
			license_id TEXT NOT NULL DEFAULT '',
			activation_key_prefix TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			failure_reason TEXT NOT NULL DEFAULT '',
			failure_message TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			redeemed_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (portal_handoff_id, purchase_return_jti)
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_purchase_return_redemptions_portal_handoff
			ON purchase_return_redemptions(portal_handoff_id);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_purchase_return_redemptions_checkout_session
			ON purchase_return_redemptions(checkout_session_id);
		CREATE INDEX IF NOT EXISTS idx_purchase_return_redemptions_status
			ON purchase_return_redemptions(status);
		`
		if _, err := db.Exec(schema); err != nil {
			_ = db.Close()
			s.initErr = fmt.Errorf("init purchase return redemption schema: %w", err)
			return
		}
		for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
			if err := hardenPrivateFile(path, handoffPrivateFilePerm); err != nil {
				_ = db.Close()
				s.initErr = fmt.Errorf("harden purchase return redemption file permissions: %w", err)
				return
			}
		}

		s.db = db
	})
}

func (s *purchaseReturnRedemptionStore) begin(attempt purchaseReturnRedemptionAttempt) (string, *purchaseReturnRedemptionRecord, error) {
	s.init()
	if s.initErr != nil {
		return "", nil, s.initErr
	}
	if s.db == nil {
		return "", nil, fmt.Errorf("purchase return redemption store not initialized")
	}

	attempt.PortalHandoffID = strings.TrimSpace(attempt.PortalHandoffID)
	attempt.PurchaseReturnJTI = strings.TrimSpace(attempt.PurchaseReturnJTI)
	attempt.CheckoutSessionID = strings.TrimSpace(attempt.CheckoutSessionID)
	attempt.LicenseID = strings.TrimSpace(attempt.LicenseID)
	attempt.ActivationKeyPrefix = strings.TrimSpace(attempt.ActivationKeyPrefix)
	if attempt.PortalHandoffID == "" {
		return "", nil, fmt.Errorf("portal handoff id is required")
	}
	if attempt.PurchaseReturnJTI == "" {
		return "", nil, fmt.Errorf("purchase return jti is required")
	}
	if attempt.CheckoutSessionID == "" {
		return "", nil, fmt.Errorf("checkout session id is required")
	}
	if attempt.ExpiresAt.IsZero() {
		return "", nil, fmt.Errorf("expires at is required")
	}
	attempt.ExpiresAt = attempt.ExpiresAt.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return "", nil, fmt.Errorf("begin purchase return redemption tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()

	existing, err := loadPurchaseReturnRedemptionExactTx(tx, attempt.PortalHandoffID, attempt.PurchaseReturnJTI)
	if err != nil {
		return "", nil, err
	}
	if existing != nil {
		if existing.CheckoutSessionID != attempt.CheckoutSessionID {
			return purchaseReturnRedemptionDecisionConflict, existing, nil
		}
		switch existing.Status {
		case purchaseReturnRedemptionStateActivated:
			committed = true
			if err := tx.Commit(); err != nil {
				return "", nil, fmt.Errorf("commit activated purchase redemption lookup: %w", err)
			}
			return purchaseReturnRedemptionDecisionAlreadyActivated, existing, nil
		case purchaseReturnRedemptionStateStarted:
			if now.Sub(existing.UpdatedAt) < purchaseReturnRedemptionStaleAfter {
				committed = true
				if err := tx.Commit(); err != nil {
					return "", nil, fmt.Errorf("commit in-progress purchase redemption lookup: %w", err)
				}
				return purchaseReturnRedemptionDecisionInProgress, existing, nil
			}
		case purchaseReturnRedemptionStateFailed:
		default:
			return "", nil, fmt.Errorf("unexpected purchase return redemption status %q", existing.Status)
		}
		if err := updatePurchaseReturnRedemptionStartedTx(tx, attempt, now); err != nil {
			return "", nil, err
		}
		record, err := loadPurchaseReturnRedemptionExactTx(tx, attempt.PortalHandoffID, attempt.PurchaseReturnJTI)
		if err != nil {
			return "", nil, err
		}
		if record == nil {
			return "", nil, fmt.Errorf("purchase return redemption row disappeared after restart")
		}
		if err := tx.Commit(); err != nil {
			return "", nil, fmt.Errorf("commit restarted purchase return redemption: %w", err)
		}
		committed = true
		return purchaseReturnRedemptionDecisionStarted, record, nil
	}

	byPortalHandoffID, err := loadPurchaseReturnRedemptionByPortalHandoffTx(tx, attempt.PortalHandoffID)
	if err != nil {
		return "", nil, err
	}
	if byPortalHandoffID != nil {
		committed = true
		if err := tx.Commit(); err != nil {
			return "", nil, fmt.Errorf("commit portal handoff conflict lookup: %w", err)
		}
		return purchaseReturnRedemptionDecisionConflict, byPortalHandoffID, nil
	}

	bySessionID, err := loadPurchaseReturnRedemptionBySessionTx(tx, attempt.CheckoutSessionID)
	if err != nil {
		return "", nil, err
	}
	if bySessionID != nil {
		committed = true
		if err := tx.Commit(); err != nil {
			return "", nil, fmt.Errorf("commit checkout session conflict lookup: %w", err)
		}
		return purchaseReturnRedemptionDecisionConflict, bySessionID, nil
	}

	if err := insertPurchaseReturnRedemptionStartedTx(tx, attempt, now); err != nil {
		return "", nil, err
	}
	record, err := loadPurchaseReturnRedemptionExactTx(tx, attempt.PortalHandoffID, attempt.PurchaseReturnJTI)
	if err != nil {
		return "", nil, err
	}
	if record == nil {
		return "", nil, fmt.Errorf("purchase return redemption row missing after insert")
	}

	if err := tx.Commit(); err != nil {
		return "", nil, fmt.Errorf("commit purchase return redemption insert: %w", err)
	}
	committed = true
	return purchaseReturnRedemptionDecisionStarted, record, nil
}

func (s *purchaseReturnRedemptionStore) markFailed(portalHandoffID, purchaseReturnJTI, reason, message string) error {
	return s.update(portalHandoffID, purchaseReturnJTI, func(tx *sql.Tx, now time.Time, existing *purchaseReturnRedemptionRecord) error {
		if existing.Status == purchaseReturnRedemptionStateActivated {
			return nil
		}
		if _, err := tx.Exec(
			`UPDATE purchase_return_redemptions
				SET status = ?, failure_reason = ?, failure_message = ?, updated_at = ?
				WHERE portal_handoff_id = ? AND purchase_return_jti = ?`,
			purchaseReturnRedemptionStateFailed,
			strings.TrimSpace(reason),
			strings.TrimSpace(message),
			now.Unix(),
			strings.TrimSpace(portalHandoffID),
			strings.TrimSpace(purchaseReturnJTI),
		); err != nil {
			return fmt.Errorf("mark purchase return redemption failed: %w", err)
		}
		return nil
	})
}

func (s *purchaseReturnRedemptionStore) markActivated(portalHandoffID, purchaseReturnJTI, licenseID, activationKeyPrefix string) error {
	return s.update(portalHandoffID, purchaseReturnJTI, func(tx *sql.Tx, now time.Time, existing *purchaseReturnRedemptionRecord) error {
		if existing.Status == purchaseReturnRedemptionStateActivated {
			return nil
		}
		if _, err := tx.Exec(
			`UPDATE purchase_return_redemptions
				SET status = ?, failure_reason = '', failure_message = '', license_id = ?, activation_key_prefix = ?, updated_at = ?, redeemed_at = ?
				WHERE portal_handoff_id = ? AND purchase_return_jti = ?`,
			purchaseReturnRedemptionStateActivated,
			strings.TrimSpace(licenseID),
			strings.TrimSpace(activationKeyPrefix),
			now.Unix(),
			now.Unix(),
			strings.TrimSpace(portalHandoffID),
			strings.TrimSpace(purchaseReturnJTI),
		); err != nil {
			return fmt.Errorf("mark purchase return redemption activated: %w", err)
		}
		return nil
	})
}

func (s *purchaseReturnRedemptionStore) get(portalHandoffID, purchaseReturnJTI string) (*purchaseReturnRedemptionRecord, error) {
	s.init()
	if s.initErr != nil {
		return nil, s.initErr
	}
	if s.db == nil {
		return nil, fmt.Errorf("purchase return redemption store not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return loadPurchaseReturnRedemptionExactTx(nil, strings.TrimSpace(portalHandoffID), strings.TrimSpace(purchaseReturnJTI), s.db)
}

func (s *purchaseReturnRedemptionStore) update(portalHandoffID, purchaseReturnJTI string, apply func(tx *sql.Tx, now time.Time, existing *purchaseReturnRedemptionRecord) error) error {
	s.init()
	if s.initErr != nil {
		return s.initErr
	}
	if s.db == nil {
		return fmt.Errorf("purchase return redemption store not initialized")
	}
	portalHandoffID = strings.TrimSpace(portalHandoffID)
	purchaseReturnJTI = strings.TrimSpace(purchaseReturnJTI)
	if portalHandoffID == "" {
		return fmt.Errorf("portal handoff id is required")
	}
	if purchaseReturnJTI == "" {
		return fmt.Errorf("purchase return jti is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin purchase return redemption update tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	existing, err := loadPurchaseReturnRedemptionExactTx(tx, portalHandoffID, purchaseReturnJTI)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("purchase return redemption not found")
	}
	if err := apply(tx, time.Now().UTC(), existing); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit purchase return redemption update: %w", err)
	}
	committed = true
	return nil
}

func insertPurchaseReturnRedemptionStartedTx(tx *sql.Tx, attempt purchaseReturnRedemptionAttempt, now time.Time) error {
	_, err := tx.Exec(
		`INSERT INTO purchase_return_redemptions (
			portal_handoff_id, purchase_return_jti, checkout_session_id, license_id, activation_key_prefix,
			status, failure_reason, failure_message, created_at, updated_at, expires_at, redeemed_at
		) VALUES (?, ?, ?, ?, ?, ?, '', '', ?, ?, ?, 0)`,
		attempt.PortalHandoffID,
		attempt.PurchaseReturnJTI,
		attempt.CheckoutSessionID,
		attempt.LicenseID,
		attempt.ActivationKeyPrefix,
		purchaseReturnRedemptionStateStarted,
		now.Unix(),
		now.Unix(),
		attempt.ExpiresAt.Unix(),
	)
	if err != nil {
		if isSQLiteUniqueViolation(err) {
			return fmt.Errorf("purchase return redemption binding already exists: %w", err)
		}
		return fmt.Errorf("insert purchase return redemption: %w", err)
	}
	return nil
}

func updatePurchaseReturnRedemptionStartedTx(tx *sql.Tx, attempt purchaseReturnRedemptionAttempt, now time.Time) error {
	_, err := tx.Exec(
		`UPDATE purchase_return_redemptions
			SET checkout_session_id = ?, license_id = ?, activation_key_prefix = ?, status = ?, failure_reason = '', failure_message = '', updated_at = ?, expires_at = ?, redeemed_at = 0
			WHERE portal_handoff_id = ? AND purchase_return_jti = ?`,
		attempt.CheckoutSessionID,
		attempt.LicenseID,
		attempt.ActivationKeyPrefix,
		purchaseReturnRedemptionStateStarted,
		now.Unix(),
		attempt.ExpiresAt.Unix(),
		attempt.PortalHandoffID,
		attempt.PurchaseReturnJTI,
	)
	if err != nil {
		return fmt.Errorf("restart purchase return redemption: %w", err)
	}
	return nil
}

func loadPurchaseReturnRedemptionExactTx(tx *sql.Tx, portalHandoffID, purchaseReturnJTI string, dbs ...*sql.DB) (*purchaseReturnRedemptionRecord, error) {
	const query = `SELECT
		portal_handoff_id,
		purchase_return_jti,
		checkout_session_id,
		license_id,
		activation_key_prefix,
		status,
		failure_reason,
		failure_message,
		created_at,
		updated_at,
		expires_at,
		redeemed_at
	FROM purchase_return_redemptions
	WHERE portal_handoff_id = ? AND purchase_return_jti = ?`

	var row *sql.Row
	switch {
	case tx != nil:
		row = tx.QueryRow(query, portalHandoffID, purchaseReturnJTI)
	case len(dbs) > 0 && dbs[0] != nil:
		row = dbs[0].QueryRow(query, portalHandoffID, purchaseReturnJTI)
	default:
		return nil, fmt.Errorf("database handle is required")
	}
	return scanPurchaseReturnRedemptionRow(row)
}

func loadPurchaseReturnRedemptionByPortalHandoffTx(tx *sql.Tx, portalHandoffID string) (*purchaseReturnRedemptionRecord, error) {
	return scanPurchaseReturnRedemptionRow(tx.QueryRow(
		`SELECT
			portal_handoff_id,
			purchase_return_jti,
			checkout_session_id,
			license_id,
			activation_key_prefix,
			status,
			failure_reason,
			failure_message,
			created_at,
			updated_at,
			expires_at,
			redeemed_at
		FROM purchase_return_redemptions
		WHERE portal_handoff_id = ?`,
		portalHandoffID,
	))
}

func loadPurchaseReturnRedemptionBySessionTx(tx *sql.Tx, checkoutSessionID string) (*purchaseReturnRedemptionRecord, error) {
	return scanPurchaseReturnRedemptionRow(tx.QueryRow(
		`SELECT
			portal_handoff_id,
			purchase_return_jti,
			checkout_session_id,
			license_id,
			activation_key_prefix,
			status,
			failure_reason,
			failure_message,
			created_at,
			updated_at,
			expires_at,
			redeemed_at
		FROM purchase_return_redemptions
		WHERE checkout_session_id = ?`,
		checkoutSessionID,
	))
}

func scanPurchaseReturnRedemptionRow(row *sql.Row) (*purchaseReturnRedemptionRecord, error) {
	if row == nil {
		return nil, fmt.Errorf("purchase return redemption row is required")
	}
	record := purchaseReturnRedemptionRecord{}
	var createdAt, updatedAt, expiresAt, redeemedAt int64
	err := row.Scan(
		&record.PortalHandoffID,
		&record.PurchaseReturnJTI,
		&record.CheckoutSessionID,
		&record.LicenseID,
		&record.ActivationKeyPrefix,
		&record.Status,
		&record.FailureReason,
		&record.FailureMessage,
		&createdAt,
		&updatedAt,
		&expiresAt,
		&redeemedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan purchase return redemption: %w", err)
	}
	record.CreatedAt = time.Unix(createdAt, 0).UTC()
	record.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	record.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	if redeemedAt > 0 {
		redeemedAtTime := time.Unix(redeemedAt, 0).UTC()
		record.RedeemedAt = &redeemedAtTime
	}
	return &record, nil
}
