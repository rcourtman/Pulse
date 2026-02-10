package handoff

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func newTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func TestHandoffHandler(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()

	accountID := "a_TEST"
	tenantID := "t-TEST"
	userID := "u_TEST"

	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: userID, Email: "tech@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: userID, Role: registry.MemberRoleTech}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&registry.Tenant{ID: tenantID, AccountID: accountID, DisplayName: "Client", State: registry.TenantStateActive}); err != nil {
		t.Fatal(err)
	}

	secret := []byte("0123456789abcdef0123456789abcdef")
	keyPath := filepath.Join(tenantsDir, tenantID, "secrets", "handoff.key")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, secret, 0o600); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	h := HandleHandoff(reg, tenantsDir)
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}/handoff", admin.AdminKeyMiddleware("secret-key", h))

	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants/"+tenantID+"/handoff", nil)
	req.Host = "cloud.example.com"
	req.Header.Set("X-Admin-Key", "secret-key")
	req.Header.Set("X-User-ID", userID)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	wantAction := "https://" + tenantID + ".cloud.example.com/api/cloud/handoff/exchange"
	if !regexp.MustCompile(regexp.QuoteMeta(wantAction)).MatchString(body) {
		t.Fatalf("missing form action %q in body", wantAction)
	}

	re := regexp.MustCompile(`name="token" value="([^"]+)"`)
	m := re.FindStringSubmatch(body)
	if len(m) != 2 {
		t.Fatalf("failed to extract token from HTML")
	}
	tokenStr := m[1]

	var got jwtHandoffClaims
	parsed, err := jwt.ParseWithClaims(
		tokenStr,
		&got,
		func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(tenantID),
	)
	if err != nil {
		t.Fatalf("ParseWithClaims: %v", err)
	}
	if !parsed.Valid {
		t.Fatalf("token valid = false, want true")
	}
	if got.Subject != userID {
		t.Fatalf("sub = %q, want %q", got.Subject, userID)
	}
	if got.AccountID != accountID {
		t.Fatalf("account_id = %q, want %q", got.AccountID, accountID)
	}
	if got.Email != "tech@example.com" {
		t.Fatalf("email = %q, want %q", got.Email, "tech@example.com")
	}
	if got.Role != registry.MemberRoleTech {
		t.Fatalf("role = %q, want %q", got.Role, registry.MemberRoleTech)
	}
	if got.ExpiresAt == nil || time.Until(got.ExpiresAt.Time) > 60*time.Second+2*time.Second {
		t.Fatalf("exp looks wrong: %v", got.ExpiresAt)
	}
}
