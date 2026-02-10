package registry

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("init tenant registry schema: %w", err)
	}
	return nil
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
			id, email, display_name, state,
			stripe_customer_id, stripe_subscription_id, stripe_price_id,
			plan_version, container_id, current_image_digest, desired_image_digest,
			created_at, updated_at, last_health_check, health_check_ok
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Email, t.DisplayName, string(t.State),
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
		id, email, display_name, state,
		stripe_customer_id, stripe_subscription_id, stripe_price_id,
		plan_version, container_id, current_image_digest, desired_image_digest,
		created_at, updated_at, last_health_check, health_check_ok
		FROM tenants WHERE id = ?`, id)
	return scanTenant(row)
}

// GetByStripeCustomerID retrieves a tenant by Stripe customer ID.
func (r *TenantRegistry) GetByStripeCustomerID(customerID string) (*Tenant, error) {
	row := r.db.QueryRow(`SELECT
		id, email, display_name, state,
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
			email = ?, display_name = ?, state = ?,
			stripe_customer_id = ?, stripe_subscription_id = ?, stripe_price_id = ?,
			plan_version = ?, container_id = ?, current_image_digest = ?, desired_image_digest = ?,
			updated_at = ?, last_health_check = ?, health_check_ok = ?
		WHERE id = ?`,
		t.Email, t.DisplayName, string(t.State),
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
		id, email, display_name, state,
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
		id, email, display_name, state,
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
		&t.ID, &t.Email, &t.DisplayName, &state,
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

func nullableTimeUnix(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
