package api

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// loopbackListener counts connections made to a listener on this server's own
// loopback, standing in for a service that only the instance should reach.
func loopbackListener(t *testing.T) (port int, accepted *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	accepted = &atomic.Int32{}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			conn.Close()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, accepted
}

// An organization's owner manages its own connections, but is not the
// instance operator: the routes that make this server connect somewhere must
// not reach the server's own loopback for them. The instance administrator
// still can, so single-tenant installs keep monitoring local services.
func TestTenantAdministratorCannotPointOutboundConnectionsAtLoopback(t *testing.T) {
	router := notificationAuthorityRouter(t)
	port, accepted := loopbackListener(t)
	loopback := "127.0.0.1:" + strconv.Itoa(port)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"availability check test", http.MethodPost, "/api/availability-targets/test",
			fmt.Sprintf(`{"name":"loop","targetKind":"service","address":"127.0.0.1","protocol":"tcp","port":%d}`, port)},
		{"availability check over HTTP", http.MethodPost, "/api/availability-targets/test",
			fmt.Sprintf(`{"name":"loop","targetKind":"service","address":"http://%s/api/health","protocol":"http"}`, loopback)},
		{"saved availability check", http.MethodPost, "/api/availability-targets",
			fmt.Sprintf(`{"name":"loop","targetKind":"service","address":"127.0.0.1","protocol":"tcp","port":%d}`, port)},
		{"availability check assigned to an agent", http.MethodPost, "/api/availability-targets",
			fmt.Sprintf(`{"name":"loop","targetKind":"service","address":"127.0.0.1","protocol":"tcp","port":%d,"observationLocationIds":["agent:host-1"]}`, port)},
		{"Proxmox connection test", http.MethodPost, "/api/config/nodes/test-connection",
			fmt.Sprintf(`{"type":"pve","host":"https://%s","tokenName":"root@pam!pulse","tokenValue":"secret"}`, loopback)},
		{"Proxmox credential test", http.MethodPost, "/api/config/nodes/test-config",
			fmt.Sprintf(`{"type":"pbs","host":"https://%s","tokenName":"root@pam!pulse","tokenValue":"secret"}`, loopback)},
		{"Proxmox node", http.MethodPost, "/api/config/nodes",
			fmt.Sprintf(`{"type":"pve","name":"loop","host":"https://%s","tokenName":"root@pam!pulse","tokenValue":"secret"}`, loopback)},
		{"address probe", http.MethodPost, "/api/connections/probe",
			fmt.Sprintf(`{"address":"%s"}`, loopback)},
		{"TrueNAS connection test", http.MethodPost, "/api/truenas/connections/test",
			fmt.Sprintf(`{"name":"loop","host":"127.0.0.1","port":%d,"apiKey":"secret","username":"admin"}`, port)},
		{"vCenter connection test", http.MethodPost, "/api/vmware/connections/test",
			fmt.Sprintf(`{"name":"loop","host":"127.0.0.1","port":%d,"username":"u","password":"p"}`, port)},
		{"SMTP server", http.MethodPut, "/api/notifications/email",
			fmt.Sprintf(`{"enabled":true,"server":"127.0.0.1","port":%d,"from":"a@example.com","to":["b@example.com"]}`, port)},
		{"SMTP test", http.MethodPost, "/api/notifications/test",
			fmt.Sprintf(`{"method":"email","config":{"enabled":true,"server":"127.0.0.1","port":%d,"from":"a@example.com","to":["b@example.com"]}}`, port)},
		{"cloud metadata", http.MethodPost, "/api/availability-targets/test",
			`{"name":"meta","targetKind":"service","address":"169.254.169.254","protocol":"tcp","port":80}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := notificationAuthorityRequest(t, router, tc.method, tc.path, tc.body, "org-a-owner", "org-a")
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s as tenant owner = %d, want 403: %s", tc.path, rec.Code, rec.Body.String())
			}
		})
	}
	// Give any stray dial a moment to land before asserting none did.
	time.Sleep(50 * time.Millisecond)
	if got := accepted.Load(); got != 0 {
		t.Fatalf("tenant requests reached the loopback listener %d times", got)
	}

	rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/availability-targets/test",
		fmt.Sprintf(`{"name":"loop","targetKind":"service","address":"127.0.0.1","protocol":"tcp","port":%d}`, port),
		"platform-admin", "default")
	if rec.Code != http.StatusOK {
		t.Fatalf("instance administrator availability test = %d: %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for accepted.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if accepted.Load() == 0 {
		t.Fatal("instance administrator availability test never reached the loopback listener")
	}
}

// Private networks are where a tenant's own systems live on a self-hosted
// shared-process install and behind a provider's VPN, so only Pulse Cloud,
// where they can only be the hosting network, closes them.
func TestTenantOutboundPolicyClosesPrivateNetworksOnlyOnPulseCloud(t *testing.T) {
	for _, tc := range []struct {
		name         string
		hosted       string
		provider     string
		allowPrivate bool
	}{
		{"self-hosted", "", "", true},
		{"provider client runtime", "true", "true", true},
		{"Pulse Cloud", "true", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PULSE_HOSTED_MODE", tc.hosted)
			t.Setenv("PULSE_PROVIDER_HOSTED", tc.provider)
			if got := tenantOutboundPolicyForRuntime(); got != (securityutil.TenantOutboundPolicy{AllowPrivateNetworks: tc.allowPrivate}) {
				t.Fatalf("policy = %+v, want AllowPrivateNetworks=%v", got, tc.allowPrivate)
			}
		})
	}
}

// The policy rides on RequireOrgAdmin only for callers without instance
// authority.
func TestRequireOrgAdminAttachesTenantOutboundPolicyOnlyToTenantCallers(t *testing.T) {
	router := notificationAuthorityRouter(t)
	var seen []bool
	probe := RequireOrgAdmin(router.config, func(w http.ResponseWriter, r *http.Request) {
		_, ok := securityutil.TenantOutboundPolicyFromContext(r.Context())
		seen = append(seen, ok)
	})
	router.mux.HandleFunc("/api/test-only/tenant-outbound-policy", probe)

	for _, tc := range []struct {
		user, org string
		want      bool
	}{
		{"org-a-owner", "org-a", true},
		{"platform-admin", "default", false},
	} {
		seen = nil
		rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/test-only/tenant-outbound-policy", "{}", tc.user, tc.org)
		if rec.Code != http.StatusOK || len(seen) != 1 {
			t.Fatalf("%s in %s: status %d, handler calls %d: %s", tc.user, tc.org, rec.Code, len(seen), rec.Body.String())
		}
		if seen[0] != tc.want {
			t.Fatalf("%s in %s: policy attached = %v, want %v", tc.user, tc.org, seen[0], tc.want)
		}
	}
}
