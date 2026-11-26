package monitoring

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

func TestStartPollerLoopSkipsPollAllWhenCancelledDuringSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var cancelSlot context.CancelFunc
	stopped := make(chan struct{})
	close(stopped)

	var syncCalls atomic.Int32
	var pollCalls atomic.Int32

	startPollerLoop(
		ctx,
		&mu,
		&cancelSlot,
		&stopped,
		func() {
			syncCalls.Add(1)
			cancel()
		},
		func(context.Context) {
			pollCalls.Add(1)
		},
		func(time.Time) time.Duration {
			return time.Hour
		},
	)

	mu.Lock()
	done := stopped
	mu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startPollerLoop did not stop after cancellation during syncConnections")
	}

	if got := syncCalls.Load(); got != 1 {
		t.Fatalf("expected 1 syncConnections call, got %d", got)
	}
	if got := pollCalls.Load(); got != 0 {
		t.Fatalf("expected 0 pollAll calls when canceled during sync, got %d", got)
	}
}

func TestTrueNASPollerPollAllDropsCancelledRefreshWithoutFabricatingFailure(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	var blockFirst atomic.Bool
	enteredFirst := make(chan struct{}, 1)
	var secondRequests atomic.Int64

	serveTrueNAS := func(hostname string, block *atomic.Bool, entered chan struct{}, counter *atomic.Int64) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if counter != nil {
				counter.Add(1)
			}
			if block != nil && block.Load() {
				select {
				case entered <- struct{}{}:
				default:
				}
				<-r.Context().Done()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v2.0/system/info":
				_, _ = w.Write([]byte(`{"hostname":"` + hostname + `","version":"TrueNAS-13.0-U6.8","uptime_seconds":3600,"system_serial":"SER-` + hostname + `"}`))
			case "/api/v2.0/pool", "/api/v2.0/pool/dataset", "/api/v2.0/disk", "/api/v2.0/alert/list", "/api/v2.0/service",
				"/api/v2.0/app", "/api/v2.0/vm", "/api/v2.0/zfs/snapshot", "/api/v2.0/sharing/smb", "/api/v2.0/sharing/nfs", "/api/v2.0/replication":
				_, _ = w.Write([]byte(`[]`))
			default:
				http.NotFound(w, r)
			}
		}))
	}

	firstServer := serveTrueNAS("nas-first", &blockFirst, enteredFirst, nil)
	t.Cleanup(firstServer.Close)
	secondServer := serveTrueNAS("nas-second", nil, nil, &secondRequests)
	t.Cleanup(secondServer.Close)

	mtp, persistence := newTestTenantPersistence(t)
	firstConn := trueNASInstanceForServer(t, "tn-01", firstServer.URL, true)
	secondConn := trueNASInstanceForServer(t, "tn-02", secondServer.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{firstConn, secondConn}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, time.Nanosecond, nil)
	t.Cleanup(func() {
		poller.mu.Lock()
		for _, byConn := range poller.providersByOrg {
			for _, provider := range byConn {
				provider.Close()
			}
		}
		poller.mu.Unlock()
	})

	poller.syncConnections()
	poller.pollAll(context.Background())

	before := poller.ConnectionSummaries("default", []config.TrueNASInstance{firstConn, secondConn})
	if before[firstConn.ID].Poll.LastSuccessAt == nil || before[secondConn.ID].Poll.LastSuccessAt == nil {
		t.Fatalf("expected initial polls to succeed, got %+v", before)
	}

	// A pre-canceled context skips all connections without dialing or recording failures.
	poller.mu.Lock()
	poller.statusByOrg["default"][firstConn.ID].nextPollAt = time.Unix(1, 0)
	poller.statusByOrg["default"][secondConn.ID].nextPollAt = time.Unix(1, 0)
	poller.mu.Unlock()

	secondBeforeCancel := secondRequests.Load()
	canceledCtx, cancelImmediately := context.WithCancel(context.Background())
	cancelImmediately()
	poller.pollAll(canceledCtx)
	if got := secondRequests.Load(); got != secondBeforeCancel {
		t.Fatalf("pre-canceled pollAll still dialed second TrueNAS connection: before=%d after=%d", secondBeforeCancel, got)
	}

	// Remove secondConn so the in-flight cancellation deterministically hits firstConn.
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{firstConn}); err != nil {
		t.Fatalf("SaveTrueNASConfig(single) error = %v", err)
	}
	poller.syncConnections()
	poller.mu.Lock()
	poller.statusByOrg["default"][firstConn.ID].nextPollAt = time.Unix(1, 0)
	poller.mu.Unlock()

	blockFirst.Store(true)
	runCtx, cancelRun := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		poller.pollAll(runCtx)
		close(done)
	}()

	select {
	case <-enteredFirst:
	case <-time.After(3 * time.Second):
		cancelRun()
		t.Fatal("timed out waiting for TrueNAS Refresh to reach server")
	}
	cancelRun()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("pollAll did not return promptly after cancellation")
	}

	after := poller.ConnectionSummaries("default", []config.TrueNASInstance{firstConn})[firstConn.ID]
	if after.Poll.ConsecutiveFailures != 0 || after.Poll.LastError != nil {
		t.Fatalf("cancelled TrueNAS poll fabricated failure: %+v", after.Poll)
	}
	if after.Poll.LastSuccessAt == nil || !after.Poll.LastSuccessAt.Equal(*before[firstConn.ID].Poll.LastSuccessAt) {
		t.Fatalf("cancelled TrueNAS poll altered LastSuccessAt: before=%v after=%v", before[firstConn.ID].Poll.LastSuccessAt, after.Poll.LastSuccessAt)
	}

	// Contrast with parent deadline exceeded: genuine timeouts must still record a failure.
	poller.mu.Lock()
	poller.statusByOrg["default"][firstConn.ID].nextPollAt = time.Unix(1, 0)
	poller.mu.Unlock()
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancelTimeout()
	poller.pollAll(timeoutCtx)

	timedOut := poller.ConnectionSummaries("default", []config.TrueNASInstance{firstConn})[firstConn.ID]
	if timedOut.Poll.ConsecutiveFailures != 1 || timedOut.Poll.LastError == nil || timedOut.Poll.LastError.Category != "timeout" {
		t.Fatalf("deadline-exceeded TrueNAS poll must still record timeout failure, got %+v", timedOut.Poll)
	}
}

type blockingVMwareProvider struct {
	vmwarePollerProvider
	entered    chan struct{}
	release    chan struct{}
	returnErr  error
	calls      atomic.Int32
	blockCalls atomic.Bool
}

func (p *blockingVMwareProvider) Refresh(ctx context.Context) error {
	p.calls.Add(1)
	if p.blockCalls.Load() {
		select {
		case p.entered <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.release:
		}
	}
	if p.returnErr != nil {
		return p.returnErr
	}
	if p.vmwarePollerProvider != nil {
		return p.vmwarePollerProvider.Refresh(ctx)
	}
	return nil
}

func TestVMwarePollerPollAllDropsCancelledRefreshWithoutFabricatingFailure(t *testing.T) {
	previous := vmware.IsFeatureEnabled()
	vmware.SetFeatureEnabled(true)
	t.Cleanup(func() { vmware.SetFeatureEnabled(previous) })

	poller := NewVMwarePoller(nil, time.Minute)
	conn1 := config.NewVMwareVCenterInstance()
	conn1.ID = "vc-01"
	conn1.Name = "vCenter 1"
	conn1.Host = "vc1.lab.local"
	conn2 := config.NewVMwareVCenterInstance()
	conn2.ID = "vc-02"
	conn2.Name = "vCenter 2"
	conn2.Host = "vc2.lab.local"

	p1 := &blockingVMwareProvider{
		vmwarePollerProvider: vmware.NewProvider(vmware.InventorySnapshot{
			ConnectionID: conn1.ID,
			CollectedAt:  time.Now().UTC(),
			Hosts:        []vmware.InventoryHost{{Host: "host-1", Name: "esxi-01"}},
		}),
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	p2 := &blockingVMwareProvider{
		vmwarePollerProvider: vmware.NewProvider(vmware.InventorySnapshot{
			ConnectionID: conn2.ID,
			CollectedAt:  time.Now().UTC(),
			Hosts:        []vmware.InventoryHost{{Host: "host-2", Name: "esxi-02"}},
		}),
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}

	poller.providersByOrg["default"] = map[string]vmwarePollerProvider{
		conn1.ID: p1,
		conn2.ID: p2,
	}

	// Initial healthy poll for both.
	poller.pollAll(context.Background())
	before := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{conn1, conn2})
	if before[conn1.ID].Poll.LastSuccessAt == nil || before[conn2.ID].Poll.LastSuccessAt == nil {
		t.Fatalf("expected healthy initial VMware polls, got %+v", before)
	}
	p2CallsBefore := p2.calls.Load()

	// Block vc-01 (sorted first), cancel context, and verify vc-01 does not
	// record a failure and vc-02 is never polled.
	p1.blockCalls.Store(true)
	runCtx, cancelRun := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		poller.pollAll(runCtx)
		close(done)
	}()

	select {
	case <-p1.entered:
	case <-time.After(2 * time.Second):
		cancelRun()
		t.Fatal("timed out waiting for VMware Refresh to start")
	}
	cancelRun()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("VMware pollAll did not return after cancellation")
	}

	after := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{conn1, conn2})
	if after[conn1.ID].Poll.ConsecutiveFailures != 0 || after[conn1.ID].Poll.LastError != nil {
		t.Fatalf("cancelled VMware poll fabricated failure on vc-01: %+v", after[conn1.ID].Poll)
	}
	if !after[conn1.ID].Poll.LastSuccessAt.Equal(*before[conn1.ID].Poll.LastSuccessAt) {
		t.Fatalf("cancelled VMware poll altered LastSuccessAt: before=%v after=%v", before[conn1.ID].Poll.LastSuccessAt, after[conn1.ID].Poll.LastSuccessAt)
	}
	if got := p2.calls.Load(); got != p2CallsBefore {
		t.Fatalf("cancelled VMware pollAll kept iterating to vc-02: before=%d after=%d", p2CallsBefore, got)
	}
}

func TestTrueNASPollerInFlightRefreshCannotResurrectRevokedOrReplacedConnection(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	for _, tc := range []struct {
		name       string
		transition string
		failPoll   bool
	}{
		{name: "deleted_on_success", transition: "deleted", failPoll: false},
		{name: "deleted_on_failure", transition: "deleted", failPoll: true},
		{name: "unreadable_config_on_success", transition: "unreadable_config", failPoll: false},
		{name: "unreadable_config_on_failure", transition: "unreadable_config", failPoll: true},
		{name: "disabled_on_success", transition: "disabled", failPoll: false},
		{name: "disabled_on_failure", transition: "disabled", failPoll: true},
		{name: "replaced_on_success", transition: "replaced", failPoll: false},
		{name: "replaced_on_failure", transition: "replaced", failPoll: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entered := make(chan struct{}, 1)
			release := make(chan struct{})
			var gate atomic.Bool
			gate.Store(true)

			oldServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if gate.CompareAndSwap(true, false) {
					entered <- struct{}{}
					<-release
				}
				if tc.failPoll {
					http.Error(w, "boom", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v2.0/system/info":
					_, _ = w.Write([]byte(`{"hostname":"old-stale-nas","version":"TrueNAS-13.0-U6.8","uptime_seconds":3600,"system_serial":"SER-OLD"}`))
				case "/api/v2.0/pool", "/api/v2.0/pool/dataset", "/api/v2.0/disk", "/api/v2.0/alert/list", "/api/v2.0/service",
					"/api/v2.0/app", "/api/v2.0/vm", "/api/v2.0/zfs/snapshot", "/api/v2.0/sharing/smb", "/api/v2.0/sharing/nfs", "/api/v2.0/replication":
					_, _ = w.Write([]byte(`[]`))
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(oldServer.Close)

			replacementServer := newTrueNASMockServer(t, "replacement-nas")
			t.Cleanup(replacementServer.Close)

			mtp, persistence := newTestTenantPersistence(t)
			conn := trueNASInstanceForServer(t, "tn-race", oldServer.URL, true)
			if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{conn}); err != nil {
				t.Fatalf("SaveTrueNASConfig() error = %v", err)
			}

			poller := NewTrueNASPoller(mtp, 0, nil)
			t.Cleanup(func() {
				poller.mu.Lock()
				for _, byConn := range poller.providersByOrg {
					for _, provider := range byConn {
						provider.Close()
					}
				}
				poller.mu.Unlock()
			})

			poller.syncConnections()

			done := make(chan struct{})
			go func() {
				poller.pollAll(context.Background())
				close(done)
			}()

			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				close(release)
				t.Fatal("timed out waiting for in-flight TrueNAS refresh")
			}

			switch tc.transition {
			case "deleted":
				if err := persistence.SaveTrueNASConfig(nil); err != nil {
					close(release)
					t.Fatalf("SaveTrueNASConfig(delete) error = %v", err)
				}
			case "unreadable_config":
				if err := os.WriteFile(filepath.Join(persistence.GetConfigDir(), "truenas.enc"), []byte("{broken"), 0o600); err != nil {
					close(release)
					t.Fatalf("WriteFile(truenas.enc) error = %v", err)
				}
			case "disabled":
				disabled := conn
				disabled.Enabled = false
				if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{disabled}); err != nil {
					close(release)
					t.Fatalf("SaveTrueNASConfig(disable) error = %v", err)
				}
			case "replaced":
				replaced := trueNASInstanceForServer(t, conn.ID, replacementServer.URL(), true)
				if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{replaced}); err != nil {
					close(release)
					t.Fatalf("SaveTrueNASConfig(replace) error = %v", err)
				}
			}
			poller.syncConnections()
			close(release)

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("in-flight TrueNAS pollAll did not complete")
			}

			switch tc.transition {
			case "deleted", "unreadable_config":
				poller.mu.Lock()
				_, statusPresent := poller.statusByOrg["default"][conn.ID]
				_, providerPresent := poller.providersByOrg["default"][conn.ID]
				poller.mu.Unlock()
				if statusPresent || providerPresent || len(poller.GetCurrentRecordsForOrg("default")) != 0 {
					t.Fatalf("in-flight refresh resurrected %s TrueNAS connection: status=%t provider=%t records=%d",
						tc.transition, statusPresent, providerPresent, len(poller.GetCurrentRecordsForOrg("default")))
				}
			case "disabled":
				summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{conn})[conn.ID]
				if summary.Poll.LastAttemptAt != nil || summary.Poll.LastSuccessAt != nil || summary.Poll.LastError != nil || summary.Observed != nil {
					t.Fatalf("in-flight refresh mutated disabled TrueNAS connection status: %+v", summary)
				}
				if len(poller.GetCurrentRecordsForOrg("default")) != 0 {
					t.Fatalf("in-flight refresh published records for disabled TrueNAS connection")
				}
			case "replaced":
				summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{conn})[conn.ID]
				if summary.Poll.LastAttemptAt != nil || summary.Poll.LastSuccessAt != nil || summary.Poll.LastError != nil || summary.Observed != nil {
					t.Fatalf("superseded provider overwrote replacement TrueNAS connection status: %+v", summary)
				}
				if hasTrueNASHostForOrg(poller, "default", "old-stale-nas") {
					t.Fatal("superseded provider published old-stale-nas records after replacement")
				}
			}
		})
	}
}

func TestVMwarePollerInFlightRefreshCannotResurrectRevokedOrReplacedConnection(t *testing.T) {
	previous := vmware.IsFeatureEnabled()
	vmware.SetFeatureEnabled(true)
	t.Cleanup(func() { vmware.SetFeatureEnabled(previous) })

	for _, tc := range []struct {
		name       string
		transition string
		failPoll   bool
	}{
		{name: "deleted_on_success", transition: "deleted", failPoll: false},
		{name: "deleted_on_failure", transition: "deleted", failPoll: true},
		{name: "unreadable_config_on_success", transition: "unreadable_config", failPoll: false},
		{name: "unreadable_config_on_failure", transition: "unreadable_config", failPoll: true},
		{name: "disabled_on_success", transition: "disabled", failPoll: false},
		{name: "disabled_on_failure", transition: "disabled", failPoll: true},
		{name: "replaced_on_success", transition: "replaced", failPoll: false},
		{name: "replaced_on_failure", transition: "replaced", failPoll: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mtp, persistence := newTestTenantPersistence(t)
			conn := config.NewVMwareVCenterInstance()
			conn.ID = "vc-race"
			conn.Name = "Old vCenter"
			conn.Host = "vc-old.lab.local"
			conn.Username = "administrator@vsphere.local"
			conn.Password = "secret"
			if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{conn}); err != nil {
				t.Fatalf("SaveVMwareConfig() error = %v", err)
			}

			oldProvider := &blockingVMwareProvider{
				vmwarePollerProvider: vmware.NewProvider(vmware.InventorySnapshot{
					ConnectionID:   conn.ID,
					ConnectionName: conn.Name,
					VCenterHost:    conn.Host,
					CollectedAt:    time.Now().UTC(),
					Hosts:          []vmware.InventoryHost{{Host: "host-old", Name: "esxi-old"}},
				}),
				entered: make(chan struct{}, 1),
				release: make(chan struct{}),
			}
			oldProvider.blockCalls.Store(true)
			if tc.failPoll {
				oldProvider.returnErr = errors.New("stale vcenter failure")
			}

			newProvider := vmware.NewProvider(vmware.InventorySnapshot{
				ConnectionID:   conn.ID,
				ConnectionName: "New vCenter",
				VCenterHost:    "vc-new.lab.local",
				CollectedAt:    time.Now().UTC(),
				Hosts:          []vmware.InventoryHost{{Host: "host-new", Name: "esxi-new"}},
			})

			poller := NewVMwarePoller(mtp, time.Minute)
			poller.newProvider = func(instance config.VMwareVCenterInstance) (vmwarePollerProvider, error) {
				if instance.Host == "vc-old.lab.local" {
					return oldProvider, nil
				}
				return newProvider, nil
			}

			poller.syncConnections()

			done := make(chan struct{})
			go func() {
				poller.pollAll(context.Background())
				close(done)
			}()

			select {
			case <-oldProvider.entered:
			case <-time.After(2 * time.Second):
				close(oldProvider.release)
				t.Fatal("timed out waiting for in-flight VMware refresh")
			}

			switch tc.transition {
			case "deleted":
				if err := persistence.SaveVMwareConfig(nil); err != nil {
					close(oldProvider.release)
					t.Fatalf("SaveVMwareConfig(delete) error = %v", err)
				}
			case "unreadable_config":
				if err := os.WriteFile(filepath.Join(persistence.GetConfigDir(), "vmware.enc"), []byte("{broken"), 0o600); err != nil {
					close(oldProvider.release)
					t.Fatalf("WriteFile(vmware.enc) error = %v", err)
				}
			case "disabled":
				disabled := conn
				disabled.Enabled = false
				if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{disabled}); err != nil {
					close(oldProvider.release)
					t.Fatalf("SaveVMwareConfig(disable) error = %v", err)
				}
			case "replaced":
				replaced := conn
				replaced.Name = "New vCenter"
				replaced.Host = "vc-new.lab.local"
				if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{replaced}); err != nil {
					close(oldProvider.release)
					t.Fatalf("SaveVMwareConfig(replace) error = %v", err)
				}
			}
			poller.syncConnections()
			close(oldProvider.release)

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("in-flight VMware pollAll did not complete")
			}

			switch tc.transition {
			case "deleted", "unreadable_config", "disabled":
				poller.mu.Lock()
				_, statusPresent := poller.statusByOrg["default"][conn.ID]
				_, providerPresent := poller.providersByOrg["default"][conn.ID]
				_, recordsPresent := poller.cachedRecordsByOrg["default"][conn.ID]
				_, changesPresent := poller.cachedChangesByOrg["default"][conn.ID]
				poller.mu.Unlock()
				if statusPresent || providerPresent || recordsPresent || changesPresent {
					t.Fatalf("in-flight refresh resurrected %s VMware connection: status=%t provider=%t records=%t changes=%t",
						tc.transition, statusPresent, providerPresent, recordsPresent, changesPresent)
				}
			case "replaced":
				summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{conn})[conn.ID]
				if summary.Poll.LastAttemptAt != nil || summary.Poll.LastSuccessAt != nil || summary.Poll.LastError != nil || summary.Observed != nil {
					t.Fatalf("superseded provider overwrote replacement VMware connection status: %+v", summary)
				}
				if got := len(poller.GetCurrentRecordsForOrg("default")); got != 0 {
					t.Fatalf("superseded provider wrote %d stale VMware records after replacement", got)
				}
			}
		})
	}
}

func TestTrueNASPollerSyncSchedulesImmediatePollAfterRepairOrReenable(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mockServer := newTrueNASMockServer(t, "repaired-nas")
	t.Cleanup(mockServer.Close)

	mtp, persistence := newTestTenantPersistence(t)

	// Start with an invalid config and a long poll interval (300s) so init fails
	// and schedules nextPollAt 5 minutes out.
	broken := trueNASInstanceForServer(t, "tn-recover", mockServer.URL(), true)
	broken.PollIntervalSecs = 300
	broken.UseHTTPS = true
	broken.Fingerprint = "not-a-valid-fingerprint"
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{broken}); err != nil {
		t.Fatalf("SaveTrueNASConfig(broken) error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 0, nil)
	t.Cleanup(func() {
		poller.mu.Lock()
		for _, byConn := range poller.providersByOrg {
			for _, provider := range byConn {
				provider.Close()
			}
		}
		poller.mu.Unlock()
	})

	poller.syncConnections()
	failed := poller.ConnectionSummaries("default", []config.TrueNASInstance{broken})[broken.ID]
	if failed.Poll == nil || failed.Poll.LastError == nil {
		t.Fatalf("expected initial broken config to record init error, got %+v", failed)
	}

	// Repair the config. syncConnections must reset nextPollAt so pollAll recovers
	// immediately without waiting 300s.
	repaired := trueNASInstanceForServer(t, broken.ID, mockServer.URL(), true)
	repaired.PollIntervalSecs = 300
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{repaired}); err != nil {
		t.Fatalf("SaveTrueNASConfig(repaired) error = %v", err)
	}
	poller.syncConnections()
	poller.pollAll(context.Background())

	recovered := poller.ConnectionSummaries("default", []config.TrueNASInstance{repaired})[repaired.ID]
	if recovered.Poll == nil || recovered.Poll.LastError != nil || recovered.Poll.ConsecutiveFailures != 0 || recovered.Observed == nil || recovered.Observed.Host != "repaired-nas" {
		t.Fatalf("repaired TrueNAS connection did not poll immediately: %+v", recovered)
	}

	// Disable and re-enable the connection while nextPollAt is still ~300s ahead.
	// Re-enabling must schedule an immediate poll.
	requestsBeforeDisable := mockServer.RequestCount()
	disabled := repaired
	disabled.Enabled = false
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{disabled}); err != nil {
		t.Fatalf("SaveTrueNASConfig(disabled) error = %v", err)
	}
	poller.syncConnections()

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{repaired}); err != nil {
		t.Fatalf("SaveTrueNASConfig(reenabled) error = %v", err)
	}
	poller.syncConnections()
	poller.pollAll(context.Background())

	if got := mockServer.RequestCount(); got <= requestsBeforeDisable {
		t.Fatalf("re-enabled TrueNAS connection did not poll immediately: requests before=%d after=%d", requestsBeforeDisable, got)
	}
}
