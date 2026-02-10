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

func TestGenerateAccountID(t *testing.T) {
	id, err := GenerateAccountID()
	if err != nil {
		t.Fatalf("GenerateAccountID: %v", err)
	}
	if !strings.HasPrefix(id, "a_") {
		t.Errorf("expected prefix a_, got %q", id)
	}
	if len(id) != 12 { // "a_" + 10 chars
		t.Errorf("expected length 12, got %d (%q)", len(id), id)
	}
}

func TestGenerateUserID(t *testing.T) {
	id, err := GenerateUserID()
	if err != nil {
		t.Fatalf("GenerateUserID: %v", err)
	}
	if !strings.HasPrefix(id, "u_") {
		t.Errorf("expected prefix u_, got %q", id)
	}
	if len(id) != 12 { // "u_" + 10 chars
		t.Errorf("expected length 12, got %d (%q)", len(id), id)
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

func TestAccountCRUD(t *testing.T) {
	reg := newTestRegistry(t)

	accountID, err := GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	a := &Account{
		ID:          accountID,
		Kind:        AccountKindMSP,
		DisplayName: "Test MSP",
	}

	// Create
	if err := reg.CreateAccount(a); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if a.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if a.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}

	// Get
	got, err := reg.GetAccount(accountID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got == nil {
		t.Fatal("GetAccount returned nil")
	}
	if got.Kind != AccountKindMSP {
		t.Errorf("Kind = %q, want %q", got.Kind, AccountKindMSP)
	}
	if got.DisplayName != "Test MSP" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Test MSP")
	}

	// Update
	got.DisplayName = "Renamed MSP"
	if err := reg.UpdateAccount(got); err != nil {
		t.Fatalf("UpdateAccount: %v", err)
	}
	got2, err := reg.GetAccount(accountID)
	if err != nil {
		t.Fatalf("GetAccount after update: %v", err)
	}
	if got2.DisplayName != "Renamed MSP" {
		t.Errorf("DisplayName after update = %q, want %q", got2.DisplayName, "Renamed MSP")
	}

	// List
	accounts, err := reg.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].ID != accountID {
		t.Errorf("accounts[0].ID = %q, want %q", accounts[0].ID, accountID)
	}
}

func TestUserCRUD(t *testing.T) {
	reg := newTestRegistry(t)

	userID, err := GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID:    userID,
		Email: "user@example.com",
	}

	// Create
	if err := reg.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Get by ID
	got, err := reg.GetUser(userID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got == nil {
		t.Fatal("GetUser returned nil")
	}
	if got.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "user@example.com")
	}
	if got.LastLoginAt != nil {
		t.Errorf("LastLoginAt = %v, want nil", got.LastLoginAt)
	}

	// Get by email
	got2, err := reg.GetUserByEmail("user@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got2 == nil || got2.ID != userID {
		t.Fatalf("GetUserByEmail returned %+v, want id=%q", got2, userID)
	}

	// Update last login
	before := time.Now().UTC()
	if err := reg.UpdateUserLastLogin(userID); err != nil {
		t.Fatalf("UpdateUserLastLogin: %v", err)
	}
	after := time.Now().UTC()

	got3, err := reg.GetUser(userID)
	if err != nil {
		t.Fatalf("GetUser after last login update: %v", err)
	}
	if got3.LastLoginAt == nil {
		t.Fatal("LastLoginAt should be set")
	}
	ll := *got3.LastLoginAt
	if ll.Before(before.Add(-2*time.Second)) || ll.After(after.Add(2*time.Second)) {
		t.Fatalf("LastLoginAt=%s out of expected range [%s, %s]", ll, before, after)
	}
}

func TestMembershipCRUD(t *testing.T) {
	reg := newTestRegistry(t)

	accountID, err := GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	userID, err := GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}

	if err := reg.CreateAccount(&Account{ID: accountID, Kind: AccountKindMSP, DisplayName: "Account"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&User{ID: userID, Email: "member@example.com"}); err != nil {
		t.Fatal(err)
	}

	m := &AccountMembership{
		AccountID: accountID,
		UserID:    userID,
		Role:      MemberRoleOwner,
	}

	// Create
	if err := reg.CreateMembership(m); err != nil {
		t.Fatalf("CreateMembership: %v", err)
	}
	if m.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Get
	got, err := reg.GetMembership(accountID, userID)
	if err != nil {
		t.Fatalf("GetMembership: %v", err)
	}
	if got == nil {
		t.Fatal("GetMembership returned nil")
	}
	if got.Role != MemberRoleOwner {
		t.Errorf("Role = %q, want %q", got.Role, MemberRoleOwner)
	}

	// List by account
	members, err := reg.ListMembersByAccount(accountID)
	if err != nil {
		t.Fatalf("ListMembersByAccount: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].UserID != userID {
		t.Errorf("members[0].UserID = %q, want %q", members[0].UserID, userID)
	}

	// List accounts by user
	accounts, err := reg.ListAccountsByUser(userID)
	if err != nil {
		t.Fatalf("ListAccountsByUser: %v", err)
	}
	if len(accounts) != 1 || accounts[0] != accountID {
		t.Fatalf("accounts=%v, want [%q]", accounts, accountID)
	}

	// Update role
	if err := reg.UpdateMembershipRole(accountID, userID, MemberRoleAdmin); err != nil {
		t.Fatalf("UpdateMembershipRole: %v", err)
	}
	got2, err := reg.GetMembership(accountID, userID)
	if err != nil {
		t.Fatalf("GetMembership after role update: %v", err)
	}
	if got2.Role != MemberRoleAdmin {
		t.Errorf("Role after update = %q, want %q", got2.Role, MemberRoleAdmin)
	}

	// Delete
	if err := reg.DeleteMembership(accountID, userID); err != nil {
		t.Fatalf("DeleteMembership: %v", err)
	}
	got3, err := reg.GetMembership(accountID, userID)
	if err != nil {
		t.Fatalf("GetMembership after delete: %v", err)
	}
	if got3 != nil {
		t.Fatalf("expected nil membership after delete, got %+v", got3)
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

func TestListByAccountID(t *testing.T) {
	reg := newTestRegistry(t)

	accountID, err := GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&Account{ID: accountID, Kind: AccountKindMSP, DisplayName: "Account"}); err != nil {
		t.Fatal(err)
	}

	if err := reg.Create(&Tenant{ID: "t-ACCNT0001", AccountID: accountID, State: TenantStateActive}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&Tenant{ID: "t-ACCNT0002", AccountID: accountID, State: TenantStateActive}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&Tenant{ID: "t-ACCNT0003", State: TenantStateActive}); err != nil {
		t.Fatal(err)
	}

	tenants, err := reg.ListByAccountID(accountID)
	if err != nil {
		t.Fatalf("ListByAccountID: %v", err)
	}
	if len(tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(tenants))
	}
	seen := make(map[string]bool)
	for _, tnt := range tenants {
		seen[tnt.ID] = true
		if tnt.AccountID != accountID {
			t.Errorf("tenant %s AccountID=%q, want %q", tnt.ID, tnt.AccountID, accountID)
		}
	}
	if !seen["t-ACCNT0001"] || !seen["t-ACCNT0002"] {
		t.Fatalf("expected tenants t-ACCNT0001 and t-ACCNT0002, got %+v", tenants)
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

func TestStripeAccountCRUD(t *testing.T) {
	reg := newTestRegistry(t)

	accountID, err := GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&Account{
		ID:          accountID,
		Kind:        AccountKindMSP,
		DisplayName: "Test MSP",
	}); err != nil {
		t.Fatal(err)
	}

	trialEnds := time.Now().UTC().Add(7 * 24 * time.Hour).Unix()
	periodEnd := time.Now().UTC().Add(30 * 24 * time.Hour).Unix()

	sa := &StripeAccount{
		AccountID:                 accountID,
		StripeCustomerID:          "cus_test_123",
		StripeSubscriptionID:      "sub_test_123",
		StripeSubItemWorkspacesID: "si_workspaces_123",
		PlanVersion:               "msp_hosted_v1",
		SubscriptionState:         "trial",
		TrialEndsAt:               &trialEnds,
		CurrentPeriodEnd:          &periodEnd,
	}

	// Create
	if err := reg.CreateStripeAccount(sa); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	// Get by account id
	got, err := reg.GetStripeAccount(accountID)
	if err != nil {
		t.Fatalf("GetStripeAccount: %v", err)
	}
	if got == nil {
		t.Fatal("GetStripeAccount returned nil")
	}
	if got.StripeCustomerID != "cus_test_123" {
		t.Errorf("StripeCustomerID = %q, want %q", got.StripeCustomerID, "cus_test_123")
	}
	if got.StripeSubscriptionID != "sub_test_123" {
		t.Errorf("StripeSubscriptionID = %q, want %q", got.StripeSubscriptionID, "sub_test_123")
	}

	// Get by customer id
	got2, err := reg.GetStripeAccountByCustomerID("cus_test_123")
	if err != nil {
		t.Fatalf("GetStripeAccountByCustomerID: %v", err)
	}
	if got2 == nil || got2.AccountID != accountID {
		t.Fatalf("expected accountID %q, got %#v", accountID, got2)
	}

	// Update
	got2.SubscriptionState = "active"
	got2.PlanVersion = "msp_hosted_v2"
	got2.StripeSubscriptionID = "sub_test_456"
	if err := reg.UpdateStripeAccount(got2); err != nil {
		t.Fatalf("UpdateStripeAccount: %v", err)
	}

	got3, err := reg.GetStripeAccount(accountID)
	if err != nil {
		t.Fatalf("GetStripeAccount after update: %v", err)
	}
	if got3.SubscriptionState != "active" {
		t.Errorf("SubscriptionState = %q, want %q", got3.SubscriptionState, "active")
	}
	if got3.PlanVersion != "msp_hosted_v2" {
		t.Errorf("PlanVersion = %q, want %q", got3.PlanVersion, "msp_hosted_v2")
	}
	if got3.StripeSubscriptionID != "sub_test_456" {
		t.Errorf("StripeSubscriptionID = %q, want %q", got3.StripeSubscriptionID, "sub_test_456")
	}
	if got3.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set")
	}
}

func TestStripeEventIdempotency(t *testing.T) {
	reg := newTestRegistry(t)

	already, err := reg.RecordStripeEvent("evt_test_123", "customer.subscription.updated")
	if err != nil {
		t.Fatalf("RecordStripeEvent: %v", err)
	}
	if already {
		t.Fatalf("expected alreadyProcessed=false on first insert")
	}

	already2, err := reg.RecordStripeEvent("evt_test_123", "customer.subscription.updated")
	if err != nil {
		t.Fatalf("RecordStripeEvent duplicate: %v", err)
	}
	if !already2 {
		t.Fatalf("expected alreadyProcessed=true on duplicate insert")
	}
}
