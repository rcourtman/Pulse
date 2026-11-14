package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpdocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/proxytrust"
)

// Exercise LoadConfig -> actual Docker create payload -> tenant trust consumer.
// The Docker API is an in-process fixture; no daemon or tenant is started.
func TestHostedTenantProxyTrustPropagation(t *testing.T) {
	const ingress = "172.30.0.2/32,173.245.48.0/20,2400:cb00::/32"
	for _, tc := range []struct {
		name, primary, fallback string
		wantEnv                 string
	}{
		{"explicit CP excludes app-only trust", ingress, "172.30.8.0/21,2001:db8:30::/64", "172.30.0.2/32,173.245.48.0/20,2400:cb00::/32"},
		{"legacy fallback retains ingress", "", ingress, "172.30.0.2/32,173.245.48.0/20,2400:cb00::/32"},
		{"no configured trust", "", "", ""},
		{"invalid primary cannot activate fallback", "not-a-proxy", ingress, ""},
		{"empty-entry primary cannot activate fallback", ", ,", ingress, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredCloudTrustEnv(t)
			t.Setenv("CP_CONTROL_PLANE_MODE", string(cloudcp.ControlPlaneModePulseHosted))
			t.Setenv("CP_TRUSTED_PROXY_CIDRS", tc.primary)
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", tc.fallback)
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			proxytrust.ResetForTesting()
			t.Cleanup(proxytrust.ResetForTesting)
			if proxytrust.IsTrustedProxyIP("172.30.8.7") || proxytrust.IsTrustedProxyIP("2001:db8:30::7") {
				t.Error("control plane trusts app-only or network-only tenant addresses")
			}
			if got := proxytrust.IsTrustedProxyIP("172.30.0.2"); got != (tc.wantEnv != "") {
				t.Errorf("control-plane ingress trust = %v, want %v", got, tc.wantEnv != "")
			}

			const runtimeID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			var createdEnv []string
			var engineMu sync.Mutex
			networkInspections, creates, starts := 0, 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				engineMu.Lock()
				defer engineMu.Unlock()
				w.Header().Set("Api-Version", "1.47")
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/_ping"):
					_, _ = w.Write([]byte("OK"))
				case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/images/"):
					_ = json.NewEncoder(w).Encode(map[string]string{"Id": "sha256:synthetic-image"})
				case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/networks/"):
					networkInspections++
					// These subnets include unrelated tenants. Supplying them in
					// the old response makes this a sensitive regression control.
					_, _ = w.Write([]byte(`{"Name":"pulse-cloud","IPAM":{"Config":[{"Subnet":"172.30.0.0/20"},{"Subnet":"2001:db8:30::/64"}]}}`))
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/create"):
					var body struct {
						Env              []string
						NetworkingConfig struct{ EndpointsConfig map[string]json.RawMessage }
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode create: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if len(body.NetworkingConfig.EndpointsConfig) != 1 || body.NetworkingConfig.EndpointsConfig["pulse-cloud"] == nil {
						t.Errorf("runtime network changed: %v", body.NetworkingConfig.EndpointsConfig)
					}
					creates++
					createdEnv = body.Env
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]string{"Id": runtimeID})
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/containers/"+runtimeID+"/json"):
					_, _ = w.Write([]byte(`{"Config":{"Labels":{}}}`))
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/"+runtimeID+"/start"):
					starts++
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected synthetic Docker request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(srv.Close)
			t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
			t.Setenv("DOCKER_TLS_VERIFY", "")
			t.Setenv("DOCKER_CERT_PATH", "")
			mgr, err := cpdocker.NewManager(cpdocker.ManagerConfig{
				Image: "pulse:synthetic", Network: "pulse-cloud",
				IsolateTenantNetworks: cfg.IsMSPControlPlane(), TrustedProxyCIDRs: cfg.TrustedProxyCIDRs,
				TenantRuntimeUID: os.Getuid(), TenantRuntimeGID: os.Getgid(),
			})
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}
			t.Cleanup(func() { _ = mgr.Close() })
			if _, err := mgr.CreateAndStart(context.Background(), "t-synthetic", t.TempDir()); err != nil {
				t.Fatalf("CreateAndStart: %v", err)
			}
			engineMu.Lock()
			if networkInspections != 0 || creates != 1 || starts != 1 {
				t.Errorf("network inspections/create/start = %d/%d/%d, want 0/1/1", networkInspections, creates, starts)
			}
			emittedEnv := append([]string(nil), createdEnv...)
			engineMu.Unlock()
			trustEnv, trustEntries := "", 0
			for _, entry := range emittedEnv {
				if value, ok := strings.CutPrefix(entry, "PULSE_TRUSTED_PROXY_CIDRS="); ok {
					trustEnv = value
					trustEntries++
				}
			}
			if trustEnv != tc.wantEnv || trustEntries > 1 || (tc.wantEnv == "" && trustEntries != 0) {
				t.Errorf("emitted trust = %q (%d entries), want %q", trustEnv, trustEntries, tc.wantEnv)
			}

			// Consume precisely the create payload, not the expected string.
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", trustEnv)
			ResetTrustedProxyConfigForTests()
			t.Cleanup(ResetTrustedProxyConfigForTests)
			if err := ValidateTrustedProxyCIDRsFromEnv(); err != nil {
				t.Fatalf("tenant startup validation: %v", err)
			}
			for _, peer := range []string{"172.30.8.7", "172.30.0.3", "2001:db8:30::7"} {
				if IsTrustedProxyIP(peer) {
					t.Errorf("tenant trusts unconfigured peer %s", peer)
				}
			}
			for _, peer := range []string{"172.30.0.2", "173.245.48.1", "2400:cb00::1"} {
				if got := IsTrustedProxyIP(peer); got != (tc.wantEnv != "") {
					t.Errorf("tenant ingress trust for %s = %v, want %v", peer, got, tc.wantEnv != "")
				}
			}
			r := httptest.NewRequest(http.MethodGet, "https://tenant.example.test/api/health", nil)
			r.RemoteAddr = "172.30.8.7:1234"
			r.Header.Set("X-Forwarded-For", "198.51.100.9")
			if got := GetClientIP(r); got != "172.30.8.7" {
				t.Errorf("unconfigured peer client IP = %q, want its actual address", got)
			}
			r.RemoteAddr = "172.30.0.2:1234"
			r.Header.Set("X-Forwarded-For", "198.51.100.9, 173.245.48.1")
			wantClient := "172.30.0.2"
			if tc.wantEnv != "" {
				wantClient = "198.51.100.9"
			}
			if got := GetClientIP(r); got != wantClient {
				t.Errorf("configured ingress client IP = %q, want %q", got, wantClient)
			}
		})
	}
}

func setRequiredCloudTrustEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CP_ADMIN_KEY", "synthetic-cloud-key")
	t.Setenv("CP_BASE_URL", "https://cloud.example.test")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test")
	t.Setenv("CP_ENV", "development")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")
}
