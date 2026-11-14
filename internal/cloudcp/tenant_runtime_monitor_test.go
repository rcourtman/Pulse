package cloudcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func runtimeMonitorFixture(t *testing.T, f *runtimeTransactionFixture, wrap func(http.Handler) http.Handler) (*Monitor, *fakeProviderDaemon) {
	t.Helper()
	reg, err := registry.NewTenantRegistry(strings.TrimSuffix(f.service.tenantsDir, "tenants") + "control-plane")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	if err := reg.Create(f.reg.tenant); err != nil {
		t.Fatal(err)
	}
	f.service.registry = reg
	d := &fakeProviderDaemon{health: "healthy", healthChecked: make(chan struct{}, 16)}
	var handler http.Handler = d
	if wrap != nil {
		handler = wrap(handler)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	mgr, err := cpDocker.NewManager(cpDocker.ManagerConfig{Image: "runtime:new", Network: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	return NewMonitor(reg, mgr, MonitorConfig{RestartOnFail: true, FailThreshold: 1}), d
}

func TestTenantRuntimeMonitorCannotRestartOrOverwriteDuringRollout(t *testing.T) {
	f := newRuntimeTransactionFixture(t, false)
	m, daemon := runtimeMonitorFixture(t, f, nil)
	m.failures["old"] = 2
	f.sync.afterSnapshot = func() {
		m.checkAll(context.Background())
		if len(daemon.healthChecked) != 0 || len(daemon.recorded()) != 0 {
			t.Fatal("monitor touched a runtime inside the stopped-writer snapshot transaction")
		}
		observed, err := m.registry.Get("t-SAFE")
		if err != nil || observed.ContainerID != "old" || observed.LastHealthCheck == nil || observed.HealthCheckOK {
			t.Fatalf("monitor persisted an in-flight result: %+v %v", observed, err)
		}
	}
	if _, err := f.rollout(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	m.checkAll(context.Background())
	observed, err := m.registry.Get("t-SAFE")
	if err != nil || observed.ContainerID != "new" || !observed.HealthCheckOK || len(daemon.healthChecked) != 1 {
		t.Fatalf("monitor did not re-read committed runtime: %+v %v", observed, err)
	}
	for _, call := range daemon.recorded() {
		if strings.Contains(call, "old/") {
			t.Fatalf("old runtime was restarted: %v", daemon.recorded())
		}
	}
}

func TestTenantRuntimeMonitorHonorsHoldAfterPartialRestoreAndOwnerExit(t *testing.T) {
	f := newRuntimeTransactionFixture(t, false)
	f.sync.restoreError = true
	f.docker.faults["health:new"] = errors.New("replacement unhealthy")
	m, daemon := runtimeMonitorFixture(t, f, nil)
	if _, err := f.rollout(context.Background(), false); err == nil {
		t.Fatal("expected retained rollback failure")
	}
	for range 4 {
		m.checkAll(context.Background())
	}
	if len(daemon.healthChecked) != 0 || len(daemon.recorded()) != 0 {
		t.Fatal("monitor ignored abandoned recovery hold")
	}
	f.assertHold(true)
	if f.docker.byID["old"].Running || len(f.docker.startCalls) != 0 {
		t.Fatal("partial restored data was reopened")
	}
}

func TestTenantRuntimeRolloutCannotOvertakeInFlightMonitor(t *testing.T) {
	f := newRuntimeTransactionFixture(t, false)
	entered := make(chan struct{})
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	m, _ := runtimeMonitorFixture(t, f, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/containers/old/json") {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	})
	done := make(chan struct{})
	go func() { m.checkTenant(context.Background(), "t-SAFE"); close(done) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor did not enter health check")
	}
	if _, err := f.rollout(context.Background(), false); !errors.Is(err, lifecycle.ErrBusy) || len(f.events) != 0 {
		t.Fatalf("rollout overtook a live monitor: %v events %v", err, f.events)
	}
	close(release)
	released = true
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor did not finish")
	}
}
