package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

func TestPollPBSInstancePausedDoesNotFabricateSuccess(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	instance := config.PBSInstance{
		Name:              "pbs-paused",
		Host:              server.URL,
		MonitorDatastores: true,
	}
	m := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	client := newPBSHealthTestClient(t, server.URL)

	// Record a real failure first so pausing must preserve that error ledger
	// rather than clearing it with a fabricated poll success.
	m.pollPBSInstance(context.Background(), instance.Name, client)
	statusBefore := *m.pollStatusMap["pbs::"+instance.Name]
	if statusBefore.ConsecutiveFailures != 1 || statusBefore.LastErrorMessage == "" || !statusBefore.LastSuccess.IsZero() {
		t.Fatalf("initial failure ledger = %+v, want 1 failure with no success", statusBefore)
	}
	snapBefore, hasSnapBefore := m.stalenessTracker.snapshot(InstanceTypePBS, instance.Name)
	if !hasSnapBefore || !snapBefore.LastSuccess.IsZero() {
		t.Fatalf("initial staleness snapshot = (%+v, %v), want error-only snapshot", snapBefore, hasSnapBefore)
	}

	hits.Store(0)
	config.Mu.Lock()
	m.config.PBSInstances[0].Disabled = true
	config.Mu.Unlock()

	m.pollPBSInstance(context.Background(), instance.Name, client)

	if hits.Load() != 0 {
		t.Fatalf("paused PBS instance made %d HTTP requests, want 0", hits.Load())
	}
	statusAfter := *m.pollStatusMap["pbs::"+instance.Name]
	if statusAfter.ConsecutiveFailures != statusBefore.ConsecutiveFailures ||
		statusAfter.LastErrorMessage != statusBefore.LastErrorMessage ||
		!statusAfter.LastSuccess.Equal(statusBefore.LastSuccess) ||
		!statusAfter.LastErrorAt.Equal(statusBefore.LastErrorAt) {
		t.Fatalf("paused PBS poll mutated scheduler ledger: before=%+v after=%+v", statusBefore, statusAfter)
	}
	snapAfter, _ := m.stalenessTracker.snapshot(InstanceTypePBS, instance.Name)
	if !snapAfter.LastSuccess.Equal(snapBefore.LastSuccess) {
		t.Fatalf("paused PBS poll fabricated staleness success: before=%+v after=%+v", snapBefore, snapAfter)
	}

	// A freshly configured paused PBS instance must stay unpolled.
	fresh := newPBSHealthAuthorityMonitor([]config.PBSInstance{{
		Name:     "pbs-never-polled",
		Host:     server.URL,
		Disabled: true,
	}})
	fresh.pollPBSInstance(context.Background(), "pbs-never-polled", client)
	if _, exists := fresh.pollStatusMap["pbs::pbs-never-polled"]; exists {
		t.Fatalf("fresh paused PBS instance created poll status entry: %+v", fresh.pollStatusMap["pbs::pbs-never-polled"])
	}
	if _, exists := fresh.stalenessTracker.snapshot(InstanceTypePBS, "pbs-never-polled"); exists {
		t.Fatal("fresh paused PBS instance created staleness snapshot")
	}
}

func TestPollPMGInstancePausedDoesNotFabricateSuccess(t *testing.T) {
	m := pmgScopeMonitor(allPMGScope())
	f := &pmgScopeFixture{calls: map[string]int{}, fail: "version"}

	// Record a real connection failure first.
	m.pollPMGInstance(context.Background(), "gateway", f)
	statusBefore := *m.pollStatusMap["pmg::gateway"]
	if statusBefore.ConsecutiveFailures != 1 || statusBefore.LastErrorMessage == "" || !statusBefore.LastSuccess.IsZero() {
		t.Fatalf("initial PMG failure ledger = %+v, want 1 failure with no success", statusBefore)
	}
	snapBefore, hasSnapBefore := m.stalenessTracker.snapshot(InstanceTypePMG, "gateway")
	if !hasSnapBefore || !snapBefore.LastSuccess.IsZero() {
		t.Fatalf("initial PMG staleness snapshot = (%+v, %v), want error-only snapshot", snapBefore, hasSnapBefore)
	}

	f.calls = map[string]int{}
	f.fail = ""
	config.Mu.Lock()
	m.config.PMGInstances[0].Disabled = true
	config.Mu.Unlock()

	m.pollPMGInstance(context.Background(), "gateway", f)

	if len(f.calls) != 0 {
		t.Fatalf("paused PMG instance made calls: %v", f.calls)
	}
	statusAfter := *m.pollStatusMap["pmg::gateway"]
	if statusAfter.ConsecutiveFailures != statusBefore.ConsecutiveFailures ||
		statusAfter.LastErrorMessage != statusBefore.LastErrorMessage ||
		!statusAfter.LastSuccess.Equal(statusBefore.LastSuccess) ||
		!statusAfter.LastErrorAt.Equal(statusBefore.LastErrorAt) {
		t.Fatalf("paused PMG poll mutated scheduler ledger: before=%+v after=%+v", statusBefore, statusAfter)
	}
	snapAfter, _ := m.stalenessTracker.snapshot(InstanceTypePMG, "gateway")
	if !snapAfter.LastSuccess.Equal(snapBefore.LastSuccess) {
		t.Fatalf("paused PMG poll fabricated staleness success: before=%+v after=%+v", snapBefore, snapAfter)
	}

	// Fresh paused PMG instance must not fabricate poll status.
	fresh := pmgScopeMonitor(config.PMGInstance{Name: "gateway", MonitoringConfigured: true, Disabled: true})
	fresh.pollPMGInstance(context.Background(), "gateway", f)
	if _, exists := fresh.pollStatusMap["pmg::gateway"]; exists {
		t.Fatalf("fresh paused PMG instance created poll status entry: %+v", fresh.pollStatusMap["pmg::gateway"])
	}
	if _, exists := fresh.stalenessTracker.snapshot(InstanceTypePMG, "gateway"); exists {
		t.Fatal("fresh paused PMG instance created staleness snapshot")
	}
}

func TestPollPMGInstancePanicRecordsFailure(t *testing.T) {
	m := pmgScopeMonitor(allPMGScope())
	panicker := &pmgPanicClient{}

	m.pollPMGInstance(context.Background(), "gateway", panicker)

	status := m.pollStatusMap["pmg::gateway"]
	if status == nil || status.ConsecutiveFailures != 1 || !strings.Contains(status.LastErrorMessage, "panic while polling PMG instance") || !status.LastSuccess.IsZero() {
		t.Fatalf("panicking PMG poll ledger = %+v, want recorded panic failure", status)
	}
}

type pmgPanicClient struct{}

func (*pmgPanicClient) GetVersion(context.Context) (*pmg.VersionInfo, error) {
	panic("simulated pmg transport panic")
}
func (*pmgPanicClient) GetClusterStatus(context.Context, bool) ([]pmg.ClusterStatusEntry, error) {
	return nil, nil
}
func (*pmgPanicClient) GetQueueStatus(context.Context, string) (*pmg.QueueStatusEntry, error) {
	return nil, nil
}
func (*pmgPanicClient) ListBackups(context.Context, string) ([]pmg.BackupEntry, error) {
	return nil, nil
}
func (*pmgPanicClient) GetMailStatistics(context.Context, string) (*pmg.MailStatistics, error) {
	return nil, nil
}
func (*pmgPanicClient) GetMailCount(context.Context, int) ([]pmg.MailCountEntry, error) {
	return nil, nil
}
func (*pmgPanicClient) GetSpamScores(context.Context) ([]pmg.SpamScore, error) {
	return nil, nil
}
func (*pmgPanicClient) GetQuarantineStatus(context.Context, string) (*pmg.QuarantineStatus, error) {
	return nil, nil
}
func (*pmgPanicClient) ListRelayDomains(context.Context) ([]pmg.RelayDomainEntry, error) {
	return nil, nil
}
func (*pmgPanicClient) GetDomainStatistics(context.Context, int64, int64) ([]pmg.DomainStatisticsEntry, error) {
	return nil, nil
}

func TestPollPBSInstanceMidPollContextExpiryRecordsFailureWithoutTruncatedPublish(t *testing.T) {
	for _, stage := range []string{"datastore", "namespace", "jobs"} {
		t.Run(stage, func(t *testing.T) {
			var blockStage atomic.Bool
			var cancelPoll context.CancelFunc
			var cancelMu sync.Mutex

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if blockStage.Load() {
					shouldCancel := (stage == "datastore" && r.URL.Path == "/api2/json/admin/datastore") ||
						(stage == "namespace" && r.URL.Path == "/api2/json/admin/datastore/secondary/namespace") ||
						(stage == "jobs" && strings.HasPrefix(r.URL.Path, "/api2/json/admin/sync"))
					if shouldCancel {
						cancelMu.Lock()
						fn := cancelPoll
						cancelMu.Unlock()
						if fn != nil {
							fn()
						}
						<-r.Context().Done()
						return
					}
				}
				switch r.URL.Path {
				case "/api2/json/version":
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "3.4.2"}})
				case "/api2/json/nodes/localhost/status":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": map[string]any{
							"cpu":    0.1,
							"memory": map[string]any{"used": 256, "total": 1024},
							"uptime": 100,
						},
					})
				case "/api2/json/admin/datastore":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": []map[string]any{
							{"store": "primary", "total": 1000, "used": 200, "avail": 800},
							{"store": "secondary", "total": 2000, "used": 400, "avail": 1600},
						},
					})
				case "/api2/json/admin/datastore/primary/status":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": map[string]any{"status": "available", "total": 1000, "used": 200, "avail": 800},
					})
				case "/api2/json/admin/datastore/secondary/status":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": map[string]any{"status": "available", "total": 2000, "used": 400, "avail": 1600},
					})
				default:
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
				}
			}))
			t.Cleanup(server.Close)

			instance := config.PBSInstance{
				Name:              "pbs-midpoll",
				Host:              server.URL,
				MonitorDatastores: true,
				MonitorSyncJobs:   true,
			}
			m := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
			client, err := pbs.NewClient(pbs.ClientConfig{
				Host:       server.URL,
				TokenName:  "root@pam!pulse",
				TokenValue: "secret",
				Timeout:    2 * time.Second,
			})
			if err != nil {
				t.Fatalf("new PBS client: %v", err)
			}

			// First poll succeeds and populates both unified storage entries.
			m.pollPBSInstance(context.Background(), instance.Name, client)
			assertPBSConnectionProjection(t, m, instance.Name, true, "online")
			prevSuccess := m.pollStatusMap["pbs::"+instance.Name].LastSuccess
			if got := len(m.state.GetSnapshot().Storage); got != 2 {
				t.Fatalf("initial unified PBS storage count = %d, want 2", got)
			}

			// Second poll cancels mid-flight after GetVersion succeeded.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cancelMu.Lock()
			cancelPoll = cancel
			cancelMu.Unlock()
			blockStage.Store(true)

			m.pollPBSInstance(ctx, instance.Name, client)

			status := m.pollStatusMap["pbs::"+instance.Name]
			if status == nil || status.ConsecutiveFailures != 1 || status.LastErrorMessage == "" {
				t.Fatalf("mid-poll expiry at %s did not record failure: %+v", stage, status)
			}
			if !status.LastSuccess.Equal(prevSuccess) {
				t.Fatalf("mid-poll expiry at %s advanced LastSuccess: got %s want %s", stage, status.LastSuccess, prevSuccess)
			}
			assertPBSConnectionProjection(t, m, instance.Name, false, "offline")
			snap := m.state.GetSnapshot()
			published := pbsInstanceByName(t, snap, instance.Name)
			if len(published.Datastores) != 0 {
				t.Fatalf("mid-poll expiry at %s published truncated datastores: %+v", stage, published.Datastores)
			}
			if len(snap.Storage) != 2 {
				t.Fatalf("mid-poll expiry at %s erased last-known unified storage: %+v", stage, snap.Storage)
			}
		})
	}
}

func TestPollPBSInstanceEmptyOrExcludedDatastoresClearUnifiedStorage(t *testing.T) {
	var mode atomic.Int32 // 0 = two datastores, 1 = empty, 2 = 503 failure
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "3.4.2"}})
		case "/api2/json/nodes/localhost/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"cpu": 0.1, "memory": map[string]any{"used": 100, "total": 1000}, "uptime": 60},
			})
		case "/api2/json/admin/datastore":
			switch mode.Load() {
			case 1:
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
			case 2:
				http.Error(w, "temporary datastore error", http.StatusServiceUnavailable)
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{
						{"store": "archive", "total": 1000, "used": 500, "avail": 500},
					},
				})
			}
		case "/api2/json/admin/datastore/archive/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"status": "available", "total": 1000, "used": 500, "avail": 500},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		}
	}))
	t.Cleanup(server.Close)

	m := newPBSHealthAuthorityMonitor([]config.PBSInstance{
		{Name: "pbs-a", Host: server.URL, MonitorDatastores: true},
		{Name: "pbs-b", Host: server.URL, MonitorDatastores: true},
	})
	client := newPBSHealthTestClient(t, server.URL)

	// Populate both PBS instances in unified Storage.
	m.pollPBSInstance(context.Background(), "pbs-a", client)
	m.pollPBSInstance(context.Background(), "pbs-b", client)
	storageFor := func(instanceID string) []models.Storage {
		var out []models.Storage
		for _, s := range m.state.GetSnapshot().Storage {
			if s.Instance == instanceID {
				out = append(out, s)
			}
		}
		return out
	}
	if len(storageFor("pbs-pbs-a")) != 1 || len(storageFor("pbs-pbs-b")) != 1 {
		t.Fatalf("initial storage entries missing: %+v", m.state.GetSnapshot().Storage)
	}

	// Excluding all datastores on pbs-a must clear pbs-a's unified storage without affecting pbs-b.
	config.Mu.Lock()
	m.config.PBSInstances[0].ExcludeDatastores = []string{"archive"}
	config.Mu.Unlock()
	m.pollPBSInstance(context.Background(), "pbs-a", client)
	if got := storageFor("pbs-pbs-a"); len(got) != 0 {
		t.Fatalf("excluded datastore remained in unified storage: %+v", got)
	}
	if got := storageFor("pbs-pbs-b"); len(got) != 1 {
		t.Fatalf("sibling PBS storage affected: %+v", got)
	}

	// Restore datastore, then verify a failed read (503) preserves last-known unified storage.
	config.Mu.Lock()
	m.config.PBSInstances[0].ExcludeDatastores = nil
	config.Mu.Unlock()
	m.pollPBSInstance(context.Background(), "pbs-a", client)
	if got := storageFor("pbs-pbs-a"); len(got) != 1 {
		t.Fatalf("restored datastore missing from unified storage: %+v", got)
	}

	mode.Store(2)
	m.pollPBSInstance(context.Background(), "pbs-a", client)
	if got := storageFor("pbs-pbs-a"); len(got) != 1 {
		t.Fatalf("failed datastore read erased last-known unified storage: %+v", got)
	}

	// Authoritative empty list from PBS must clear pbs-a's unified storage.
	mode.Store(1)
	m.pollPBSInstance(context.Background(), "pbs-a", client)
	if got := storageFor("pbs-pbs-a"); len(got) != 0 {
		t.Fatalf("authoritative empty datastore list left ghost unified storage: %+v", got)
	}
}

func TestPollPBSInstanceConcurrentConfigEditIsRaceFree(t *testing.T) {
	fixture := newPBSHealthTestServer(t)
	m := newPBSHealthAuthorityMonitor([]config.PBSInstance{
		{Name: "pbs-race", Host: fixture.server.URL, MonitorDatastores: true},
	})
	client := newPBSHealthTestClient(t, fixture.server.URL)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				config.Mu.Lock()
				m.config.PBSInstances[0].ExcludeDatastores = []string{"temp*"}
				m.config.PBSInstances[0].MonitorDatastores = true
				config.Mu.Unlock()
				config.Mu.Lock()
				m.config.PBSInstances[0].ExcludeDatastores = nil
				config.Mu.Unlock()
			}
		}
	}()

	for range 8 {
		m.pollPBSInstance(context.Background(), "pbs-race", client)
	}
	close(stop)
	wg.Wait()

	// Nil config must record a clean error instead of panicking.
	nilCfgMonitor := &Monitor{
		state:            models.NewState(),
		pollStatusMap:    make(map[string]*pollStatus),
		stalenessTracker: NewStalenessTracker(nil),
	}
	nilCfgMonitor.pollPBSInstance(context.Background(), "pbs-missing", client)
	status := nilCfgMonitor.pollStatusMap["pbs::pbs-missing"]
	if status == nil || status.ConsecutiveFailures != 1 || !strings.Contains(status.LastErrorMessage, "not found") {
		t.Fatalf("nil-config PBS poll ledger = %+v, want missing-config error", status)
	}
}

func TestPollPBSInstanceFallbackReusesDatastoresWithoutDuplicateFetch(t *testing.T) {
	var datastoreCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/version":
			http.Error(w, "forbidden", http.StatusForbidden)
		case "/api2/json/admin/datastore":
			datastoreCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"store": "store1", "total": 1000, "used": 100, "avail": 900}},
			})
		case "/api2/json/admin/datastore/store1/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"status": "available", "total": 1000, "used": 100, "avail": 900},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		}
	}))
	t.Cleanup(server.Close)

	m := newPBSHealthAuthorityMonitor([]config.PBSInstance{
		{Name: "pbs-fallback-once", Host: server.URL, MonitorDatastores: true},
	})
	client := newPBSHealthTestClient(t, server.URL)

	m.pollPBSInstance(context.Background(), "pbs-fallback-once", client)

	if got := datastoreCalls.Load(); got != 1 {
		t.Fatalf("fallback datastore list requests = %d, want 1", got)
	}
	published := pbsInstanceByName(t, m.state.GetSnapshot(), "pbs-fallback-once")
	if published.Version != "connected" || published.Status != "online" || len(published.Datastores) != 1 || published.Datastores[0].Name != "store1" {
		t.Fatalf("fallback PBS instance = %+v, want online with store1", published)
	}
}
