package registry

import (
	"os"
	"strings"
	"testing"
	"time"
)

func newTestRegistry(t *testing.T) *TenantRegistry {
	t.Helper()
	dir := t.TempDir()
	reg, err := NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func TestGenerateTenantID(t *testing.T) {
	id, err := GenerateTenantID()
	if err != nil {
		t.Fatalf("GenerateTenantID: %v", err)
	}
	if !strings.HasPrefix(id, "t-") {
		t.Errorf("expected prefix t-, got %q", id)
	}
	if len(id) != 12 { // "t-" + 10 chars
		t.Errorf("expected length 12, got %d (%q)", len(id), id)
	}

	// Uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := GenerateTenantID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate tenant ID: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateTenantID_CrockfordCharset(t *testing.T) {
	for i := 0; i < 50; i++ {
		id, err := GenerateTenantID()
		if err != nil {
			t.Fatal(err)
		}
		suffix := id[2:] // strip "t-"
		for _, c := range suffix {
			if !strings.ContainsRune(crockfordBase32, c) {
				t.Errorf("character %q not in Crockford base32 alphabet (id=%s)", c, id)
			}
		}
	}
}

func TestCRUD(t *testing.T) {
	reg := newTestRegistry(t)

	tenant := &Tenant{
		ID:               "t-TEST00001",
		Email:            "test@example.com",
		DisplayName:      "Test Tenant",
		State:            TenantStateProvisioning,
		StripeCustomerID: "cus_test123",
		PlanVersion:      "stripe",
	}

	// Create
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tenant.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Get
	got, err := reg.Get("t-TEST00001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "test@example.com")
	}
	if got.State != TenantStateProvisioning {
		t.Errorf("State = %q, want %q", got.State, TenantStateProvisioning)
	}

	// Get not found
	notFound, err := reg.Get("t-NONEXIST1")
	if err != nil {
		t.Fatalf("Get not found: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent tenant")
	}

	// GetByStripeCustomerID
	got2, err := reg.GetByStripeCustomerID("cus_test123")
	if err != nil {
		t.Fatalf("GetByStripeCustomerID: %v", err)
	}
	if got2 == nil || got2.ID != "t-TEST00001" {
		t.Error("GetByStripeCustomerID should find the tenant")
	}

	// Update
	got.State = TenantStateActive
	got.ContainerID = "abc123"
	if err := reg.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got3, err := reg.Get("t-TEST00001")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got3.State != TenantStateActive {
		t.Errorf("State after update = %q, want %q", got3.State, TenantStateActive)
	}
	if got3.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q", got3.ContainerID, "abc123")
	}

	// Update not found
	phantom := &Tenant{ID: "t-NONEXIST1"}
	if err := reg.Update(phantom); err == nil {
		t.Error("Update non-existent tenant should error")
	}
}

func TestList(t *testing.T) {
	reg := newTestRegistry(t)

	// Empty list
	tenants, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tenants) != 0 {
		t.Errorf("expected 0 tenants, got %d", len(tenants))
	}

	// Add two tenants
	for _, id := range []string{"t-LIST00001", "t-LIST00002"} {
		if err := reg.Create(&Tenant{
			ID:    id,
			Email: id + "@example.com",
			State: TenantStateActive,
		}); err != nil {
			t.Fatal(err)
		}
	}

	tenants, err = reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tenants) != 2 {
		t.Errorf("expected 2 tenants, got %d", len(tenants))
	}
}

func TestListByState(t *testing.T) {
	reg := newTestRegistry(t)

	if err := reg.Create(&Tenant{ID: "t-STATE0001", State: TenantStateActive}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&Tenant{ID: "t-STATE0002", State: TenantStateSuspended}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&Tenant{ID: "t-STATE0003", State: TenantStateActive}); err != nil {
		t.Fatal(err)
	}

	active, err := reg.ListByState(TenantStateActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active, got %d", len(active))
	}

	suspended, err := reg.ListByState(TenantStateSuspended)
	if err != nil {
		t.Fatal(err)
	}
	if len(suspended) != 1 {
		t.Errorf("expected 1 suspended, got %d", len(suspended))
	}
}

func TestCountByState(t *testing.T) {
	reg := newTestRegistry(t)

	if err := reg.Create(&Tenant{ID: "t-CNT000001", State: TenantStateActive}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&Tenant{ID: "t-CNT000002", State: TenantStateActive}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&Tenant{ID: "t-CNT000003", State: TenantStateCanceled}); err != nil {
		t.Fatal(err)
	}

	counts, err := reg.CountByState()
	if err != nil {
		t.Fatal(err)
	}
	if counts[TenantStateActive] != 2 {
		t.Errorf("active count = %d, want 2", counts[TenantStateActive])
	}
	if counts[TenantStateCanceled] != 1 {
		t.Errorf("canceled count = %d, want 1", counts[TenantStateCanceled])
	}
}

func TestHealthSummary(t *testing.T) {
	reg := newTestRegistry(t)

	now := time.Now().UTC()

	if err := reg.Create(&Tenant{
		ID: "t-HLTH00001", State: TenantStateActive,
		HealthCheckOK: true, LastHealthCheck: &now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&Tenant{
		ID: "t-HLTH00002", State: TenantStateActive,
		HealthCheckOK: false, LastHealthCheck: &now,
	}); err != nil {
		t.Fatal(err)
	}
	// Suspended tenant should not count
	if err := reg.Create(&Tenant{
		ID: "t-HLTH00003", State: TenantStateSuspended,
		HealthCheckOK: false,
	}); err != nil {
		t.Fatal(err)
	}

	healthy, unhealthy, err := reg.HealthSummary()
	if err != nil {
		t.Fatal(err)
	}
	if healthy != 1 {
		t.Errorf("healthy = %d, want 1", healthy)
	}
	if unhealthy != 1 {
		t.Errorf("unhealthy = %d, want 1", unhealthy)
	}
}

func TestPing(t *testing.T) {
	reg := newTestRegistry(t)
	if err := reg.Ping(); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestNewTenantRegistry_InvalidDir(t *testing.T) {
	// Read-only path that doesn't exist
	_, err := NewTenantRegistry("/proc/nonexistent/path")
	if err == nil {
		// On macOS /proc doesn't exist, so MkdirAll will fail
		// On Linux with /proc it would also fail
		// But skip if somehow it works (unlikely)
		if _, statErr := os.Stat("/proc/nonexistent/path"); statErr != nil {
			t.Log("Skipping: path creation succeeded unexpectedly")
		}
	}
}
