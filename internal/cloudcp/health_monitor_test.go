package cloudcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// fakeProviderDaemon answers the few Docker Engine API calls the health
// monitor makes for one provider-hosted client workspace, and records every
// mutating call so a test can assert what the monitor did to the host.
type fakeProviderDaemon struct {
	mu            sync.Mutex
	tenantNetwork string
	attached      map[string]bool // container IDs on the tenant network
	support       map[string]string
	health        string
	calls         []string
}

var dockerAPIVersionPrefix = regexp.MustCompile(`^/v[0-9.]+`)

func (d *fakeProviderDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := dockerAPIVersionPrefix.ReplaceAllString(r.URL.Path, "")
	w.Header().Set("Api-Version", "1.47")
	w.Header().Set("Content-Type", "application/json")
	switch {
	case path == "/_ping":
		_, _ = w.Write([]byte("OK"))
	case r.Method == http.MethodGet && path == "/networks/"+d.tenantNetwork:
		containers := map[string]any{}
		for id := range d.attached {
			containers[id] = map[string]any{"Name": id}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Name": d.tenantNetwork, "Id": "net-1", "Containers": containers})
	case r.Method == http.MethodPost && path == "/networks/"+d.tenantNetwork+"/connect":
		var body struct{ Container string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		d.attached[body.Container] = true
		d.calls = append(d.calls, "connect "+body.Container)
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && path == "/containers/json":
		var items []map[string]any
		for label, id := range d.support {
			if strings.Contains(r.URL.Query().Get("filters"), label) {
				items = append(items, map[string]any{"Id": id, "State": "running"})
			}
		}
		_ = json.NewEncoder(w).Encode(items)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json"):
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Id":     strings.TrimSuffix(strings.TrimPrefix(path, "/containers/"), "/json"),
			"State":  map[string]any{"Status": "running", "Running": true, "Health": map[string]any{"Status": d.health}},
			"Config": map[string]any{"Labels": map[string]string{}},
		})
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/containers/"):
		d.calls = append(d.calls, strings.TrimPrefix(path, "/containers/"))
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "no such object: " + path})
	}
}

func (d *fakeProviderDaemon) recorded() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

// An upgrade recreates the control plane (and often Traefik), and the new
// containers are not on any client's isolated network. The monitor used to
// read that as a failing client and, three minutes later, Stop it; Docker's
// unless-stopped policy never restarts a container stopped through the API,
// so every client workspace stayed down. The monitor must put the support
// containers back on the network, and when a client really is unhealthy it
// must restart it rather than stop it.
func TestHealthMonitorReattachesSupportContainersAndRestartsInsteadOfStopping(t *testing.T) {
	daemon := &fakeProviderDaemon{
		tenantNetwork: "pulse-provider-msp-tenant-t-acme",
		attached:      map[string]bool{"tenant-container": true},
		support: map[string]string{
			"pulse.provider-msp.role=traefik":       "traefik-recreated",
			"pulse.provider-msp.role=control-plane": "control-plane-recreated",
		},
		health: "healthy",
	}
	srv := httptest.NewServer(daemon)
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	mgr, err := docker.NewManager(docker.ManagerConfig{
		Image:                 "pulse:test",
		Network:               "pulse-provider-msp",
		IsolateTenantNetworks: true,
		TenantNetworkPrefix:   "pulse-provider-msp-tenant",
		BaseDomain:            "msp.example.com",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	reg, err := registry.NewTenantRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	if err := reg.Create(&registry.Tenant{ID: "t-acme", AccountID: "a-1", State: registry.TenantStateActive, ContainerID: "tenant-container"}); err != nil {
		t.Fatalf("Create tenant: %v", err)
	}

	monitor := NewMonitor(reg, mgr, MonitorConfig{RestartOnFail: true, FailThreshold: 3})
	monitor.checkAll(context.Background())

	got := strings.Join(daemon.recorded(), ", ")
	for _, want := range []string{"connect traefik-recreated", "connect control-plane-recreated"} {
		if !strings.Contains(got, want) {
			t.Fatalf("after one pass the daemon saw %q, want %q", got, want)
		}
	}
	tenant, err := reg.Get("t-acme")
	if err != nil || tenant == nil || !tenant.HealthCheckOK {
		t.Fatalf("tenant after healthy pass = %+v, err %v; want HealthCheckOK", tenant, err)
	}

	// Already attached: later passes change nothing on the network.
	monitor.checkAll(context.Background())
	if again := daemon.recorded(); len(again) != 2 {
		t.Fatalf("second pass reconnected attached containers: %v", again)
	}

	daemon.mu.Lock()
	daemon.health = "unhealthy"
	daemon.mu.Unlock()
	for range 3 {
		monitor.checkAll(context.Background())
	}
	got = strings.Join(daemon.recorded(), ", ")
	if !strings.Contains(got, "tenant-container/restart") {
		t.Fatalf("unhealthy client was not restarted; daemon saw %q", got)
	}
	if strings.Contains(got, "tenant-container/stop") {
		t.Fatalf("unhealthy client was stopped, which unless-stopped never undoes; daemon saw %q", got)
	}
}
