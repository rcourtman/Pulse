package registry

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTenantRegistryQueryPlansUseIndexes validates that the main registry list
// and count queries continue to use the intended SQLite indexes instead of
// regressing to full table scans or temp-sort plans.
func TestTenantRegistryQueryPlansUseIndexes(t *testing.T) {
	t.Parallel()

	reg := newRegistryPlanTestRegistry(t)

	tests := []struct {
		name              string
		query             string
		args              []any
		wantSearchIndexes map[string]string
		wantScanIndexes   map[string]string
	}{
		{
			name: "list tenants uses created_at index",
			query: `SELECT
				id, account_id, email, display_name, state,
				stripe_customer_id, stripe_subscription_id, stripe_price_id,
				plan_version, container_id, current_image_digest, desired_image_digest,
				created_at, updated_at, last_health_check, health_check_ok
				FROM tenants ORDER BY created_at DESC`,
			wantScanIndexes: map[string]string{
				"tenants": "idx_tenants_created_at",
			},
		},
		{
			name: "list tenants by state uses composite index",
			query: `SELECT
				id, account_id, email, display_name, state,
				stripe_customer_id, stripe_subscription_id, stripe_price_id,
				plan_version, container_id, current_image_digest, desired_image_digest,
				created_at, updated_at, last_health_check, health_check_ok
				FROM tenants WHERE state = ? ORDER BY created_at DESC`,
			args: []any{string(TenantStateActive)},
			wantSearchIndexes: map[string]string{
				"tenants": "idx_tenants_state_created_at",
			},
		},
		{
			name: "list tenants by account uses composite index",
			query: `SELECT
				id, account_id, email, display_name, state,
				stripe_customer_id, stripe_subscription_id, stripe_price_id,
				plan_version, container_id, current_image_digest, desired_image_digest,
				created_at, updated_at, last_health_check, health_check_ok
				FROM tenants WHERE account_id = ? ORDER BY created_at DESC`,
			args: []any{"account-03"},
			wantSearchIndexes: map[string]string{
				"tenants": "idx_tenants_account_id_created_at",
			},
		},
		{
			name:  "count active tenants by account uses account index",
			query: `SELECT COUNT(*) FROM tenants WHERE account_id = ? AND state NOT IN ('deleting', 'deleted', 'canceled')`,
			args:  []any{"account-03"},
			wantSearchIndexes: map[string]string{
				"tenants": "idx_tenants_account_id",
			},
		},
		{
			name: "list accounts uses created_at index",
			query: `SELECT
				id, kind, display_name, created_at, updated_at
				FROM accounts ORDER BY created_at DESC`,
			wantScanIndexes: map[string]string{
				"accounts": "idx_accounts_created_at",
			},
		},
		{
			name: "list members by account uses composite index",
			query: `SELECT
				account_id, user_id, role, created_at
				FROM account_memberships
				WHERE account_id = ?
				ORDER BY created_at DESC`,
			args: []any{"account-03"},
			wantSearchIndexes: map[string]string{
				"account_memberships": "idx_memberships_account_id_created_at",
			},
		},
		{
			name:  "list accounts by user uses composite index",
			query: `SELECT account_id FROM account_memberships WHERE user_id = ? ORDER BY created_at DESC`,
			args:  []any{"user-007"},
			wantSearchIndexes: map[string]string{
				"account_memberships": "idx_memberships_user_id_created_at",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainRegistryQueryPlan(t, reg.db, tt.query, tt.args...)

			if containsRegistryFullTableScan(plan) {
				t.Fatalf("query uses full table scan on registry tables\nPlan:\n%s", plan)
			}
			if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") {
				t.Fatalf("query regressed to temp B-tree ordering\nPlan:\n%s", plan)
			}
			for table, indexName := range tt.wantSearchIndexes {
				if !containsRegistrySearchWithIndex(plan, table, indexName) {
					t.Fatalf("expected SEARCH on %s using index %q\nPlan:\n%s", table, indexName, plan)
				}
			}
			for table, indexName := range tt.wantScanIndexes {
				if !containsRegistryScanWithIndex(plan, table, indexName) {
					t.Fatalf("expected indexed SCAN on %s using index %q\nPlan:\n%s", table, indexName, plan)
				}
			}
		})
	}
}

func newRegistryPlanTestRegistry(t *testing.T) *TenantRegistry {
	t.Helper()

	reg, err := NewTenantRegistry(filepath.Join(t.TempDir(), "registry"))
	if err != nil {
		t.Fatalf("NewTenantRegistry() error = %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	base := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 80; i++ {
		account := &Account{
			ID:          fmt.Sprintf("account-%02d", i),
			Kind:        AccountKindMSP,
			DisplayName: fmt.Sprintf("Account %02d", i),
			CreatedAt:   base.Add(time.Duration(i) * time.Minute),
		}
		if err := reg.CreateAccount(account); err != nil {
			t.Fatalf("CreateAccount(%d): %v", i, err)
		}
	}

	for i := 0; i < 160; i++ {
		user := &User{
			ID:        fmt.Sprintf("user-%03d", i),
			Email:     fmt.Sprintf("member-%03d@example.com", i),
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		if err := reg.CreateUser(user); err != nil {
			t.Fatalf("CreateUser(%d): %v", i, err)
		}
	}

	for i := 0; i < 320; i++ {
		tenant := &Tenant{
			ID:              fmt.Sprintf("tenant-%03d", i),
			AccountID:       fmt.Sprintf("account-%02d", i%40),
			Email:           fmt.Sprintf("tenant-%03d@example.com", i),
			DisplayName:     fmt.Sprintf("Tenant %03d", i),
			State:           tenantStateForIndex(i),
			CreatedAt:       base.Add(time.Duration(i) * time.Minute),
			UpdatedAt:       base.Add(time.Duration(i) * time.Minute),
			HealthCheckOK:   i%2 == 0,
			LastHealthCheck: nil,
		}
		if err := reg.Create(tenant); err != nil {
			t.Fatalf("Create tenant(%d): %v", i, err)
		}
	}

	for i := 0; i < 480; i++ {
		membership := &AccountMembership{
			AccountID: fmt.Sprintf("account-%02d", i%40),
			UserID:    fmt.Sprintf("user-%03d", i%120),
			Role:      MemberRoleTech,
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := reg.CreateMembership(membership); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			}
			t.Fatalf("CreateMembership(%d): %v", i, err)
		}
	}

	if _, err := reg.db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	return reg
}

func tenantStateForIndex(i int) TenantState {
	switch i % 3 {
	case 0:
		return TenantStateActive
	case 1:
		return TenantStateSuspended
	default:
		return TenantStateProvisioning
	}
}

func explainRegistryQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()

	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v\nQuery: %s", err, query)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var (
			id     int
			parent int
			aux    int
			detail string
		)
		if err := rows.Scan(&id, &parent, &aux, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		lines = append(lines, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("EXPLAIN QUERY PLAN returned no rows")
	}
	return strings.Join(lines, "\n")
}

func containsRegistrySearchWithIndex(plan, tableName, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") &&
			strings.Contains(line, tableName) &&
			strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsRegistryScanWithIndex(plan, tableName, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SCAN") &&
			strings.Contains(line, tableName) &&
			strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsRegistryFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") {
			continue
		}
		if !(strings.Contains(line, "tenants") || strings.Contains(line, "accounts") || strings.Contains(line, "account_memberships")) {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}
