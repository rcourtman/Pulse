package migration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type managedLicenseServer struct {
	cmd     *exec.Cmd
	logBuf  *bytes.Buffer
	baseURL string
}

func TestV5PaidLicenseUpgrade_RealLicenseServerExchange(t *testing.T) {
	licenseServerDir := managedLicenseServerDir(t)
	licenseServerBinary := buildManagedLicenseServerBinary(t, licenseServerDir)
	plansJSON := managedLicenseServerPlansJSON(t)

	tests := []struct {
		name           string
		email          string
		tier           pkglicensing.Tier
		planKey        string
		licenseID      string
		installIDHint  string
		expiresIn      time.Duration
		wantIsLifetime bool
	}{
		{
			name:           "lifetime grandfathered",
			email:          "legacy-lifetime@example.com",
			tier:           pkglicensing.TierLifetime,
			planKey:        "v5_lifetime_grandfathered",
			licenseID:      "lic_v5_real_exchange_lifetime",
			installIDHint:  "lifetime",
			expiresIn:      365 * 24 * time.Hour,
			wantIsLifetime: true,
		},
		{
			name:           "monthly grandfathered",
			email:          "legacy-monthly@example.com",
			tier:           pkglicensing.TierPro,
			planKey:        "v5_pro_monthly_grandfathered",
			licenseID:      "lic_v5_real_exchange_monthly",
			installIDHint:  "monthly",
			expiresIn:      365 * 24 * time.Hour,
			wantIsLifetime: false,
		},
		{
			name:           "annual grandfathered",
			email:          "legacy-annual@example.com",
			tier:           pkglicensing.TierPro,
			planKey:        "v5_pro_annual_grandfathered",
			licenseID:      "lic_v5_real_exchange_annual",
			installIDHint:  "annual",
			expiresIn:      365 * 24 * time.Hour,
			wantIsLifetime: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

			dataDir, _, _, _, _ := buildV5DataDir(t)
			grantPublicKey, grantPrivateKey, err := ed25519.GenerateKey(nil)
			require.NoError(t, err)

			legacyLicense := signManagedLegacyLicenseJWT(t, grantPrivateKey, pkglicensing.Claims{
				LicenseID: tc.licenseID,
				Email:     tc.email,
				Tier:      tc.tier,
				IssuedAt:  time.Now().Add(-6 * time.Hour).Unix(),
				ExpiresAt: time.Now().Add(tc.expiresIn).Unix(),
			})

			licenseServerDataDir := t.TempDir()
			writeManagedLegacyLicenseRecord(t, licenseServerDataDir, pkglicensing.Claims{
				LicenseID: tc.licenseID,
				Email:     tc.email,
				Tier:      tc.tier,
				IssuedAt:  time.Now().Add(-6 * time.Hour).Unix(),
				ExpiresAt: time.Now().Add(tc.expiresIn).Unix(),
			}, legacyLicense, tc.planKey)

			server := startManagedLicenseServer(t, licenseServerBinary, licenseServerDataDir, grantPrivateKey, plansJSON)
			defer server.stopKill()

			pkglicensing.SetPublicKey(grantPublicKey)
			t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

			persistence, err := pkglicensing.NewPersistence(dataDir)
			require.NoError(t, err)
			require.NoError(t, persistence.Save(legacyLicense))

			t.Setenv("PULSE_LICENSE_SERVER_URL", server.baseURL)

			mtp := config.NewMultiTenantPersistence(dataDir)
			handlers := api.NewLicenseHandlers(mtp, false)
			t.Cleanup(handlers.StopAllBackgroundLoops)

			ctx := context.WithValue(context.Background(), api.OrgIDContextKey, "default")
			svc := handlers.Service(ctx)
			require.NotNil(t, svc)
			require.True(t, svc.IsActivated(), "persisted v5 license must auto-exchange through the real license server")

			current := svc.Current()
			require.NotNil(t, current)
			assert.NotEmpty(t, current.Claims.LicenseID)
			assert.NotEqual(t, tc.licenseID, current.Claims.LicenseID, "real exchange should promote the legacy token into a new canonical v6 license id")
			assert.Equal(t, tc.tier, current.Claims.Tier)
			assert.Equal(t, tc.planKey, current.Claims.PlanVersion)
			assert.Equal(t, pkglicensing.TierAgentLimits[tc.tier], current.Claims.MaxAgents)

			activationState, err := persistence.LoadActivationState()
			require.NoError(t, err)
			require.NotNil(t, activationState, "real exchange must persist activation state")
			assert.Equal(t, current.Claims.LicenseID, activationState.LicenseID)
			assert.Equal(t, server.baseURL, activationState.LicenseServerURL)
			assert.NotEmpty(t, activationState.InstallationID)
			assert.NotEmpty(t, activationState.InstallationToken)
			assert.NotEmpty(t, activationState.GrantJWT)

			legacyLeft, err := persistence.Load()
			require.NoError(t, err)
			assert.Equal(t, legacyLicense, legacyLeft, "legacy v5 license must remain available for downgrade")

			statusReq := httpRequestWithOrg(http.MethodGet, "/api/license/status", ctx)
			statusRec := responseRecorder()
			handlers.HandleLicenseStatus(statusRec, statusReq)
			require.Equal(t, http.StatusOK, statusRec.Code)

			var status pkglicensing.LicenseStatus
			require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &status))
			assert.True(t, status.Valid)
			assert.Equal(t, tc.tier, status.Tier)
			assert.Equal(t, tc.planKey, status.PlanVersion)
			assert.Equal(t, tc.wantIsLifetime, status.IsLifetime)
			assert.Equal(t, pkglicensing.TierAgentLimits[tc.tier], status.MaxAgents)

			entReq := httpRequestWithOrg(http.MethodGet, "/api/license/entitlements", ctx)
			entRec := responseRecorder()
			handlers.HandleEntitlements(entRec, entReq)
			require.Equal(t, http.StatusOK, entRec.Code)

			var entitlements pkglicensing.EntitlementPayload
			require.NoError(t, json.Unmarshal(entRec.Body.Bytes(), &entitlements))
			assert.Equal(t, tc.planKey, entitlements.PlanVersion)
			assert.Equal(t, "active", entitlements.SubscriptionState)
			assert.Equal(t, string(tc.tier), entitlements.Tier)
			assert.Equal(t, tc.wantIsLifetime, entitlements.IsLifetime)
			assert.Equal(t, int64(pkglicensing.TierAgentLimits[tc.tier]), entitlementLimitByKey(entitlements.Limits, pkglicensing.MaxAgentsLicenseGateKey))
		})
	}
}

func httpRequestWithOrg(method string, path string, ctx context.Context) *http.Request {
	return httptest.NewRequest(method, path, nil).WithContext(ctx)
}

func responseRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

func entitlementLimitByKey(limits []pkglicensing.LimitStatus, key string) int64 {
	for _, limit := range limits {
		if limit.Key == key {
			return limit.Limit
		}
	}
	return 0
}

func managedLicenseServerDir(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller for managed license server test")
	}
	pulseRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	pulseProRoot := os.Getenv("PULSE_REPO_ROOT_PULSE_PRO")
	if pulseProRoot == "" {
		pulseProRoot = filepath.Join(filepath.Dir(pulseRoot), "pulse-pro")
	}
	licenseServerDir := filepath.Join(pulseProRoot, "license-server")
	if _, err := os.Stat(filepath.Join(licenseServerDir, "main.go")); err != nil {
		if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("PULSE_REPO_ROOT_PULSE_PRO") == "" {
			t.Skipf("managed license-server proof requires sibling pulse-pro license-server; skipping in GitHub Actions without PULSE_REPO_ROOT_PULSE_PRO override: %v", err)
		}
		t.Fatalf("managed license-server proof requires sibling pulse-pro license-server at %s: %v", licenseServerDir, err)
	}
	return licenseServerDir
}

func buildManagedLicenseServerBinary(t *testing.T, licenseServerDir string) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "pulse-license-server-managed-runtime")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = licenseServerDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build managed license server binary: %v\n%s", err, string(output))
	}
	return binaryPath
}

func managedLicenseServerPlansJSON(t *testing.T) string {
	t.Helper()
	plans := map[string]map[string]any{
		"v5_lifetime_grandfathered": {
			"tier":          string(pkglicensing.TierLifetime),
			"duration_days": 0,
			"features":      pkglicensing.TierFeatures[pkglicensing.TierLifetime],
			"max_agents":    pkglicensing.TierAgentLimits[pkglicensing.TierLifetime],
			"max_guests":    5,
		},
		"v5_pro_monthly_grandfathered": {
			"tier":          string(pkglicensing.TierPro),
			"duration_days": 30,
			"features":      pkglicensing.TierFeatures[pkglicensing.TierPro],
			"max_agents":    pkglicensing.TierAgentLimits[pkglicensing.TierPro],
			"max_guests":    5,
		},
		"v5_pro_annual_grandfathered": {
			"tier":          string(pkglicensing.TierPro),
			"duration_days": 365,
			"features":      pkglicensing.TierFeatures[pkglicensing.TierPro],
			"max_agents":    pkglicensing.TierAgentLimits[pkglicensing.TierPro],
			"max_guests":    5,
		},
	}
	raw, err := json.Marshal(plans)
	require.NoError(t, err)
	return string(raw)
}

func signManagedLegacyLicenseJWT(t *testing.T, privateKey ed25519.PrivateKey, claims pkglicensing.Claims) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	require.NoError(t, err)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := []byte(header + "." + payload)
	signature := ed25519.Sign(privateKey, signingInput)
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func writeManagedLegacyLicenseRecord(t *testing.T, dataDir string, claims pkglicensing.Claims, token string, planKey string) {
	t.Helper()
	issuedAt := time.Unix(claims.IssuedAt, 0).UTC().Format(time.RFC3339)
	var expiresAt any
	if claims.ExpiresAt > 0 {
		expiresAt = time.Unix(claims.ExpiresAt, 0).UTC().Format(time.RFC3339)
	}

	payload := map[string]any{
		"licenses": []map[string]any{
			{
				"id":                 claims.LicenseID,
				"email":              claims.Email,
				"tier":               string(claims.Tier),
				"features":           claims.Features,
				"issued_at":          issuedAt,
				"expires_at":         expiresAt,
				"token":              token,
				"plan_key":           planKey,
				"stripe_session_id":  "",
				"stripe_customer_id": "",
				"revoked":            false,
			},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "licenses.json"), raw, 0o600))
}

func startManagedLicenseServer(
	t *testing.T,
	binaryPath string,
	dataDir string,
	privateKey ed25519.PrivateKey,
	plansJSON string,
) *managedLicenseServer {
	t.Helper()
	addr := allocateManagedLicenseServerAddr(t)
	logBuf := &bytes.Buffer{}
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		"PULSE_LICENSE_ADDR="+addr,
		"PULSE_LICENSE_DATA_DIR="+dataDir,
		"PULSE_LICENSE_PRIVATE_KEY="+base64.StdEncoding.EncodeToString(privateKey.Seed()),
		"PULSE_LICENSE_PLANS="+plansJSON,
		"PULSE_LICENSE_V6_ENABLED=true",
	)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start managed license server: %v", err)
	}

	server := &managedLicenseServer{
		cmd:     cmd,
		logBuf:  logBuf,
		baseURL: "http://" + addr,
	}
	t.Cleanup(func() {
		server.stopKill()
	})
	waitForManagedLicenseServerHealth(t, server.baseURL, logBuf, 10*time.Second)
	return server
}

func (s *managedLicenseServer) stopKill() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
		return
	}
	_ = s.cmd.Process.Kill()
	_, _ = s.cmd.Process.Wait()
}

func allocateManagedLicenseServerAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return addr
}

func waitForManagedLicenseServerHealth(t *testing.T, baseURL string, logBuf *bytes.Buffer, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("managed license server at %s never became healthy\nlogs:\n%s", baseURL, logBuf.String())
}
