package registry

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// TenantRegistry provides CRUD operations for tenant records backed by SQLite.
type TenantRegistry struct {
	db *sql.DB
}

// NewTenantRegistry opens (or creates) the tenant registry database in dir.
func NewTenantRegistry(dir string) (*TenantRegistry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create registry dir: %w", err)
	}

	dbPath := filepath.Join(dir, "tenants.db")
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open tenant registry db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	r := &TenantRegistry{db: db}
	if err := r.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *TenantRegistry) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id                    TEXT PRIMARY KEY,
		account_id            TEXT NOT NULL DEFAULT '',
		email                 TEXT NOT NULL DEFAULT '',
		display_name          TEXT NOT NULL DEFAULT '',
		state                 TEXT NOT NULL DEFAULT 'provisioning',
		stripe_customer_id    TEXT NOT NULL DEFAULT '',
		stripe_subscription_id TEXT NOT NULL DEFAULT '',
		stripe_price_id       TEXT NOT NULL DEFAULT '',
		plan_version          TEXT NOT NULL DEFAULT '',
		container_id          TEXT NOT NULL DEFAULT '',
		current_image_digest  TEXT NOT NULL DEFAULT '',
		desired_image_digest  TEXT NOT NULL DEFAULT '',
		created_at            INTEGER NOT NULL,
		updated_at            INTEGER NOT NULL,
		last_health_check     INTEGER,
		health_check_ok       INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_tenants_state ON tenants(state);
	CREATE INDEX IF NOT EXISTS idx_tenants_stripe_customer_id ON tenants(stripe_customer_id);

	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL DEFAULT 'individual',
		display_name TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS stripe_accounts (
		account_id TEXT PRIMARY KEY,
		stripe_customer_id TEXT NOT NULL UNIQUE,
		stripe_subscription_id TEXT,
		stripe_sub_item_workspaces_id TEXT,
		plan_version TEXT NOT NULL DEFAULT '',
		subscription_state TEXT NOT NULL DEFAULT 'trial',
		trial_ends_at INTEGER,
		current_period_end INTEGER,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY (account_id) REFERENCES accounts(id)
	);
	CREATE INDEX IF NOT EXISTS idx_stripe_accounts_customer ON stripe_accounts(stripe_customer_id);

	CREATE TABLE IF NOT EXISTS stripe_events (
		stripe_event_id TEXT PRIMARY KEY,
		event_type TEXT NOT NULL,
		received_at INTEGER NOT NULL,
		processed_at INTEGER,
		processing_error TEXT
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		created_at INTEGER NOT NULL,
		last_login_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS account_memberships (
		account_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'tech',
		created_at INTEGER NOT NULL,
		PRIMARY KEY (account_id, user_id),
		FOREIGN KEY (account_id) REFERENCES accounts(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON account_memberships(user_id);
	`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("init tenant registry schema: %w", err)
	}

	// Migration: add account_id to tenants if not present.
	// (SQLite makes it awkward to add FK constraints via ALTER TABLE, and FK
	// enforcement is off by default; this keeps the change backwards-compatible.)
	hasAccountID, err := r.tenantsHasColumn("account_id")
	if err != nil {
		return err
	}
	if !hasAccountID {
		if _, err := r.db.Exec(`ALTER TABLE tenants ADD COLUMN account_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate tenants: add account_id: %w", err)
		}
	}
	if _, err := r.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tenants_account_id ON tenants(account_id)`); err != nil {
		return fmt.Errorf("init tenant registry schema: create idx_tenants_account_id: %w", err)
	}
	return nil
}

func (r *TenantRegistry) tenantsHasColumn(name string) (bool, error) {
	rows, err := r.db.Query(`PRAGMA table_info(tenants)`)
	if err != nil {
		return false, fmt.Errorf("pragma table_info(tenants): %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid     int
			colName string
			colType string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scan table_info(tenants): %w", err)
		}
		if colName == name {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table_info(tenants): %w", err)
	}
	return false, nil
}

// Ping checks database connectivity (used for readiness probes).
func (r *TenantRegistry) Ping() error {
	return r.db.Ping()
}

// Close closes the underlying database connection.
func (r *TenantRegistry) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// Create inserts a new tenant record.
func (r *TenantRegistry) Create(t *Tenant) error {
	if t == nil {
		return fmt.Errorf("tenant is nil")
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now

	_, err := r.db.Exec(`
		INSERT INTO tenants (
			id, account_id, email, display_name, state,
			stripe_customer_id, stripe_subscription_id, stripe_price_id,
			plan_version, container_id, current_image_digest, desired_image_digest,
			created_at, updated_at, last_health_check, health_check_ok
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.AccountID, t.Email, t.DisplayName, string(t.State),
		t.StripeCustomerID, t.StripeSubscriptionID, t.StripePriceID,
		t.PlanVersion, t.ContainerID, t.CurrentImageDigest, t.DesiredImageDigest,
		t.CreatedAt.Unix(), t.UpdatedAt.Unix(), nullableTimeUnix(t.LastHealthCheck), boolToInt(t.HealthCheckOK),
	)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

// Get retrieves a tenant by ID.
func (r *TenantRegistry) Get(id string) (*Tenant, error) {
	row := r.db.QueryRow(`SELECT
		id, account_id, email, display_name, state,
		stripe_customer_id, stripe_subscription_id, stripe_price_id,
		plan_version, container_id, current_image_digest, desired_image_digest,
		created_at, updated_at, last_health_check, health_check_ok
		FROM tenants WHERE id = ?`, id)
	return scanTenant(row)
}

// GetByStripeCustomerID retrieves a tenant by Stripe customer ID.
func (r *TenantRegistry) GetByStripeCustomerID(customerID string) (*Tenant, error) {
	row := r.db.QueryRow(`SELECT
		id, account_id, email, display_name, state,
		stripe_customer_id, stripe_subscription_id, stripe_price_id,
		plan_version, container_id, current_image_digest, desired_image_digest,
		created_at, updated_at, last_health_check, health_check_ok
		FROM tenants WHERE stripe_customer_id = ?`, customerID)
	return scanTenant(row)
}

// Update modifies an existing tenant record.
func (r *TenantRegistry) Update(t *Tenant) error {
	if t == nil {
		return fmt.Errorf("tenant is nil")
	}
	t.UpdatedAt = time.Now().UTC()

	res, err := r.db.Exec(`
		UPDATE tenants SET
			account_id = ?, email = ?, display_name = ?, state = ?,
			stripe_customer_id = ?, stripe_subscription_id = ?, stripe_price_id = ?,
			plan_version = ?, container_id = ?, current_image_digest = ?, desired_image_digest = ?,
			updated_at = ?, last_health_check = ?, health_check_ok = ?
		WHERE id = ?`,
		t.AccountID, t.Email, t.DisplayName, string(t.State),
		t.StripeCustomerID, t.StripeSubscriptionID, t.StripePriceID,
		t.PlanVersion, t.ContainerID, t.CurrentImageDigest, t.DesiredImageDigest,
		t.UpdatedAt.Unix(), nullableTimeUnix(t.LastHealthCheck), boolToInt(t.HealthCheckOK),
		t.ID,
	)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("tenant %q not found", t.ID)
	}
	return nil
}

// List returns all tenants.
func (r *TenantRegistry) List() ([]*Tenant, error) {
	rows, err := r.db.Query(`SELECT
		id, account_id, email, display_name, state,
		stripe_customer_id, stripe_subscription_id, stripe_price_id,
		plan_version, container_id, current_image_digest, desired_image_digest,
		created_at, updated_at, last_health_check, health_check_ok
		FROM tenants ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()
	return scanTenants(rows)
}

// ListByState returns all tenants matching the given state.
func (r *TenantRegistry) ListByState(state TenantState) ([]*Tenant, error) {
	rows, err := r.db.Query(`SELECT
		id, account_id, email, display_name, state,
		stripe_customer_id, stripe_subscription_id, stripe_price_id,
		plan_version, container_id, current_image_digest, desired_image_digest,
		created_at, updated_at, last_health_check, health_check_ok
		FROM tenants WHERE state = ? ORDER BY created_at DESC`, string(state))
	if err != nil {
		return nil, fmt.Errorf("list tenants by state: %w", err)
	}
	defer rows.Close()
	return scanTenants(rows)
}

// ListByAccountID returns all tenants belonging to the given account ID.
func (r *TenantRegistry) ListByAccountID(accountID string) ([]*Tenant, error) {
	rows, err := r.db.Query(`SELECT
		id, account_id, email, display_name, state,
		stripe_customer_id, stripe_subscription_id, stripe_price_id,
		plan_version, container_id, current_image_digest, desired_image_digest,
		created_at, updated_at, last_health_check, health_check_ok
		FROM tenants WHERE account_id = ? ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list tenants by account id: %w", err)
	}
	defer rows.Close()
	return scanTenants(rows)
}

// CountByState returns a map of state -> count.
func (r *TenantRegistry) CountByState() (map[TenantState]int, error) {
	rows, err := r.db.Query(`SELECT state, COUNT(*) FROM tenants GROUP BY state`)
	if err != nil {
		return nil, fmt.Errorf("count tenants by state: %w", err)
	}
	defer rows.Close()

	counts := make(map[TenantState]int)
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return nil, fmt.Errorf("scan count: %w", err)
		}
		counts[TenantState(state)] = count
	}
	return counts, rows.Err()
}

// HealthSummary returns the number of healthy and unhealthy active tenants.
func (r *TenantRegistry) HealthSummary() (healthy, unhealthy int, err error) {
	row := r.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN health_check_ok = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN health_check_ok = 0 THEN 1 ELSE 0 END), 0)
		FROM tenants WHERE state = ?`, string(TenantStateActive))
	if err := row.Scan(&healthy, &unhealthy); err != nil {
		return 0, 0, fmt.Errorf("health summary: %w", err)
	}
	return healthy, unhealthy, nil
}

// scanner is an interface satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanTenant(s scanner) (*Tenant, error) {
	var t Tenant
	var state string
	var createdAt, updatedAt int64
	var lastHealthCheck sql.NullInt64
	var healthOK int

	err := s.Scan(
		&t.ID, &t.AccountID, &t.Email, &t.DisplayName, &state,
		&t.StripeCustomerID, &t.StripeSubscriptionID, &t.StripePriceID,
		&t.PlanVersion, &t.ContainerID, &t.CurrentImageDigest, &t.DesiredImageDigest,
		&createdAt, &updatedAt, &lastHealthCheck, &healthOK,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan tenant: %w", err)
	}

	t.State = TenantState(state)
	t.CreatedAt = time.Unix(createdAt, 0).UTC()
	t.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	if lastHealthCheck.Valid {
		ts := time.Unix(lastHealthCheck.Int64, 0).UTC()
		t.LastHealthCheck = &ts
	}
	t.HealthCheckOK = healthOK != 0
	return &t, nil
}

func scanTenants(rows *sql.Rows) ([]*Tenant, error) {
	var tenants []*Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// CreateAccount inserts a new account record.
func (r *TenantRegistry) CreateAccount(a *Account) error {
	if a == nil {
		return fmt.Errorf("account is nil")
	}
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	kind := string(a.Kind)
	if kind == "" {
		kind = string(AccountKindIndividual)
	}

	_, err := r.db.Exec(`
		INSERT INTO accounts (
			id, kind, display_name, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?)`,
		a.ID, kind, a.DisplayName, a.CreatedAt.Unix(), a.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	a.Kind = AccountKind(kind)
	return nil
}

// GetAccount retrieves an account by ID.
func (r *TenantRegistry) GetAccount(id string) (*Account, error) {
	row := r.db.QueryRow(`SELECT
		id, kind, display_name, created_at, updated_at
		FROM accounts WHERE id = ?`, id)
	return scanAccount(row)
}

// UpdateAccount modifies an existing account record.
func (r *TenantRegistry) UpdateAccount(a *Account) error {
	if a == nil {
		return fmt.Errorf("account is nil")
	}
	a.UpdatedAt = time.Now().UTC()

	kind := string(a.Kind)
	if kind == "" {
		kind = string(AccountKindIndividual)
	}

	res, err := r.db.Exec(`
		UPDATE accounts SET
			kind = ?, display_name = ?, updated_at = ?
		WHERE id = ?`,
		kind, a.DisplayName, a.UpdatedAt.Unix(),
		a.ID,
	)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("account %q not found", a.ID)
	}
	a.Kind = AccountKind(kind)
	return nil
}

// ListAccounts returns all accounts.
func (r *TenantRegistry) ListAccounts() ([]*Account, error) {
	rows, err := r.db.Query(`SELECT
		id, kind, display_name, created_at, updated_at
		FROM accounts ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()
	return scanAccounts(rows)
}

// CreateUser inserts a new user record.
func (r *TenantRegistry) CreateUser(u *User) error {
	if u == nil {
		return fmt.Errorf("user is nil")
	}
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}

	_, err := r.db.Exec(`
		INSERT INTO users (
			id, email, created_at, last_login_at
		) VALUES (?, ?, ?, ?)`,
		u.ID, u.Email, u.CreatedAt.Unix(), nullableTimeUnix(u.LastLoginAt),
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUser retrieves a user by ID.
func (r *TenantRegistry) GetUser(id string) (*User, error) {
	row := r.db.QueryRow(`SELECT
		id, email, created_at, last_login_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// GetUserByEmail retrieves a user by email.
func (r *TenantRegistry) GetUserByEmail(email string) (*User, error) {
	row := r.db.QueryRow(`SELECT
		id, email, created_at, last_login_at
		FROM users WHERE email = ?`, email)
	return scanUser(row)
}

// UpdateUserLastLogin sets last_login_at for the given user ID to the current time.
func (r *TenantRegistry) UpdateUserLastLogin(id string) error {
	now := time.Now().UTC()
	res, err := r.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, now.Unix(), id)
	if err != nil {
		return fmt.Errorf("update user last login: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("user %q not found", id)
	}
	return nil
}

// CreateMembership inserts a new membership record.
func (r *TenantRegistry) CreateMembership(m *AccountMembership) error {
	if m == nil {
		return fmt.Errorf("membership is nil")
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	role := string(m.Role)
	if role == "" {
		role = string(MemberRoleTech)
	}

	_, err := r.db.Exec(`
		INSERT INTO account_memberships (
			account_id, user_id, role, created_at
		) VALUES (?, ?, ?, ?)`,
		m.AccountID, m.UserID, role, m.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}
	m.Role = MemberRole(role)
	return nil
}

// GetMembership retrieves a membership record by account ID and user ID.
func (r *TenantRegistry) GetMembership(accountID, userID string) (*AccountMembership, error) {
	row := r.db.QueryRow(`SELECT
		account_id, user_id, role, created_at
		FROM account_memberships
		WHERE account_id = ? AND user_id = ?`, accountID, userID)
	return scanMembership(row)
}

// ListMembersByAccount returns all membership records for a given account ID.
func (r *TenantRegistry) ListMembersByAccount(accountID string) ([]*AccountMembership, error) {
	rows, err := r.db.Query(`SELECT
		account_id, user_id, role, created_at
		FROM account_memberships
		WHERE account_id = ?
		ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list members by account: %w", err)
	}
	defer rows.Close()
	return scanMemberships(rows)
}

// ListAccountsByUser returns account IDs for all accounts the given user belongs to.
func (r *TenantRegistry) ListAccountsByUser(userID string) ([]string, error) {
	rows, err := r.db.Query(`SELECT account_id FROM account_memberships WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts by user: %w", err)
	}
	defer rows.Close()

	var accountIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan account id: %w", err)
		}
		accountIDs = append(accountIDs, id)
	}
	return accountIDs, rows.Err()
}

// UpdateMembershipRole updates a membership role.
func (r *TenantRegistry) UpdateMembershipRole(accountID, userID string, role MemberRole) error {
	res, err := r.db.Exec(`UPDATE account_memberships SET role = ? WHERE account_id = ? AND user_id = ?`, string(role), accountID, userID)
	if err != nil {
		return fmt.Errorf("update membership role: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("membership (%q, %q) not found", accountID, userID)
	}
	return nil
}

// DeleteMembership deletes a membership record.
func (r *TenantRegistry) DeleteMembership(accountID, userID string) error {
	res, err := r.db.Exec(`DELETE FROM account_memberships WHERE account_id = ? AND user_id = ?`, accountID, userID)
	if err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("membership (%q, %q) not found", accountID, userID)
	}
	return nil
}

// CreateStripeAccount inserts a new StripeAccount mapping row.
func (r *TenantRegistry) CreateStripeAccount(sa *StripeAccount) error {
	if sa == nil {
		return fmt.Errorf("stripe account is nil")
	}
	sa.AccountID = strings.TrimSpace(sa.AccountID)
	sa.StripeCustomerID = strings.TrimSpace(sa.StripeCustomerID)
	sa.StripeSubscriptionID = strings.TrimSpace(sa.StripeSubscriptionID)
	sa.StripeSubItemWorkspacesID = strings.TrimSpace(sa.StripeSubItemWorkspacesID)
	sa.PlanVersion = strings.TrimSpace(sa.PlanVersion)
	sa.SubscriptionState = strings.TrimSpace(sa.SubscriptionState)

	if sa.AccountID == "" {
		return fmt.Errorf("missing account id")
	}
	if sa.StripeCustomerID == "" {
		return fmt.Errorf("missing stripe customer id")
	}
	if sa.SubscriptionState == "" {
		sa.SubscriptionState = "trial"
	}
	if sa.UpdatedAt == 0 {
		sa.UpdatedAt = time.Now().UTC().Unix()
	}

	_, err := r.db.Exec(`
		INSERT INTO stripe_accounts (
			account_id, stripe_customer_id, stripe_subscription_id, stripe_sub_item_workspaces_id,
			plan_version, subscription_state, trial_ends_at, current_period_end, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sa.AccountID,
		sa.StripeCustomerID,
		nullableString(sa.StripeSubscriptionID),
		nullableString(sa.StripeSubItemWorkspacesID),
		sa.PlanVersion,
		sa.SubscriptionState,
		nullableInt64Ptr(sa.TrialEndsAt),
		nullableInt64Ptr(sa.CurrentPeriodEnd),
		sa.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create stripe account: %w", err)
	}
	return nil
}

// GetStripeAccount retrieves the StripeAccount row by account ID.
func (r *TenantRegistry) GetStripeAccount(accountID string) (*StripeAccount, error) {
	row := r.db.QueryRow(`SELECT
		account_id, stripe_customer_id, stripe_subscription_id, stripe_sub_item_workspaces_id,
		plan_version, subscription_state, trial_ends_at, current_period_end, updated_at
		FROM stripe_accounts WHERE account_id = ?`, strings.TrimSpace(accountID))
	return scanStripeAccount(row)
}

// GetStripeAccountByCustomerID retrieves the StripeAccount row by Stripe customer ID.
func (r *TenantRegistry) GetStripeAccountByCustomerID(customerID string) (*StripeAccount, error) {
	row := r.db.QueryRow(`SELECT
		account_id, stripe_customer_id, stripe_subscription_id, stripe_sub_item_workspaces_id,
		plan_version, subscription_state, trial_ends_at, current_period_end, updated_at
		FROM stripe_accounts WHERE stripe_customer_id = ?`, strings.TrimSpace(customerID))
	return scanStripeAccount(row)
}

// UpdateStripeAccount modifies an existing StripeAccount row.
func (r *TenantRegistry) UpdateStripeAccount(sa *StripeAccount) error {
	if sa == nil {
		return fmt.Errorf("stripe account is nil")
	}
	sa.AccountID = strings.TrimSpace(sa.AccountID)
	sa.StripeCustomerID = strings.TrimSpace(sa.StripeCustomerID)
	sa.StripeSubscriptionID = strings.TrimSpace(sa.StripeSubscriptionID)
	sa.StripeSubItemWorkspacesID = strings.TrimSpace(sa.StripeSubItemWorkspacesID)
	sa.PlanVersion = strings.TrimSpace(sa.PlanVersion)
	sa.SubscriptionState = strings.TrimSpace(sa.SubscriptionState)

	if sa.AccountID == "" {
		return fmt.Errorf("missing account id")
	}
	if sa.StripeCustomerID == "" {
		return fmt.Errorf("missing stripe customer id")
	}
	if sa.SubscriptionState == "" {
		sa.SubscriptionState = "trial"
	}

	sa.UpdatedAt = time.Now().UTC().Unix()

	res, err := r.db.Exec(`
		UPDATE stripe_accounts SET
			stripe_customer_id = ?, stripe_subscription_id = ?, stripe_sub_item_workspaces_id = ?,
			plan_version = ?, subscription_state = ?, trial_ends_at = ?, current_period_end = ?, updated_at = ?
		WHERE account_id = ?`,
		sa.StripeCustomerID,
		nullableString(sa.StripeSubscriptionID),
		nullableString(sa.StripeSubItemWorkspacesID),
		sa.PlanVersion,
		sa.SubscriptionState,
		nullableInt64Ptr(sa.TrialEndsAt),
		nullableInt64Ptr(sa.CurrentPeriodEnd),
		sa.UpdatedAt,
		sa.AccountID,
	)
	if err != nil {
		return fmt.Errorf("update stripe account: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("stripe account %q not found", sa.AccountID)
	}
	return nil
}

// RecordStripeEvent inserts a webhook event record and returns true if the
// event was already recorded (duplicate Stripe delivery).
func (r *TenantRegistry) RecordStripeEvent(eventID, eventType string) (alreadyProcessed bool, err error) {
	eventID = strings.TrimSpace(eventID)
	eventType = strings.TrimSpace(eventType)
	if eventID == "" {
		return false, fmt.Errorf("missing stripe event id")
	}
	if eventType == "" {
		return false, fmt.Errorf("missing stripe event type")
	}

	// INSERT OR IGNORE avoids driver-specific error parsing for duplicates.
	res, err := r.db.Exec(`
		INSERT OR IGNORE INTO stripe_events (
			stripe_event_id, event_type, received_at, processed_at, processing_error
		) VALUES (?, ?, ?, NULL, NULL)`,
		eventID, eventType, time.Now().UTC().Unix(),
	)
	if err != nil {
		return false, fmt.Errorf("record stripe event: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return true, nil
	}
	return false, nil
}

// MarkStripeEventProcessed marks a previously recorded event as processed.
// processingError is stored (nullable) for troubleshooting.
func (r *TenantRegistry) MarkStripeEventProcessed(eventID string, processingError string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return fmt.Errorf("missing stripe event id")
	}
	processingError = strings.TrimSpace(processingError)

	res, err := r.db.Exec(`
		UPDATE stripe_events SET
			processed_at = ?, processing_error = ?
		WHERE stripe_event_id = ?`,
		time.Now().UTC().Unix(),
		nullableString(processingError),
		eventID,
	)
	if err != nil {
		return fmt.Errorf("mark stripe event processed: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("stripe event %q not found", eventID)
	}
	return nil
}

func scanAccount(s scanner) (*Account, error) {
	var a Account
	var kind string
	var createdAt, updatedAt int64
	if err := s.Scan(&a.ID, &kind, &a.DisplayName, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan account: %w", err)
	}
	a.Kind = AccountKind(kind)
	a.CreatedAt = time.Unix(createdAt, 0).UTC()
	a.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &a, nil
}

func scanAccounts(rows *sql.Rows) ([]*Account, error) {
	var accounts []*Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func scanUser(s scanner) (*User, error) {
	var u User
	var createdAt int64
	var lastLogin sql.NullInt64
	if err := s.Scan(&u.ID, &u.Email, &createdAt, &lastLogin); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.CreatedAt = time.Unix(createdAt, 0).UTC()
	if lastLogin.Valid {
		ts := time.Unix(lastLogin.Int64, 0).UTC()
		u.LastLoginAt = &ts
	}
	return &u, nil
}

func scanMembership(s scanner) (*AccountMembership, error) {
	var m AccountMembership
	var role string
	var createdAt int64
	if err := s.Scan(&m.AccountID, &m.UserID, &role, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan membership: %w", err)
	}
	m.Role = MemberRole(role)
	m.CreatedAt = time.Unix(createdAt, 0).UTC()
	return &m, nil
}

func scanMemberships(rows *sql.Rows) ([]*AccountMembership, error) {
	var memberships []*AccountMembership
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

func nullableTimeUnix(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func nullableInt64Ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanStripeAccount(s scanner) (*StripeAccount, error) {
	var sa StripeAccount
	var subID, subItemID sql.NullString
	var trialEnds, periodEnd sql.NullInt64
	if err := s.Scan(
		&sa.AccountID,
		&sa.StripeCustomerID,
		&subID,
		&subItemID,
		&sa.PlanVersion,
		&sa.SubscriptionState,
		&trialEnds,
		&periodEnd,
		&sa.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan stripe account: %w", err)
	}
	if subID.Valid {
		sa.StripeSubscriptionID = subID.String
	}
	if subItemID.Valid {
		sa.StripeSubItemWorkspacesID = subItemID.String
	}
	if trialEnds.Valid {
		v := trialEnds.Int64
		sa.TrialEndsAt = &v
	}
	if periodEnd.Valid {
		v := periodEnd.Int64
		sa.CurrentPeriodEnd = &v
	}
	return &sa, nil
}
