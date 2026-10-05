package monitoring

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// Only the polling clock is deterministic. Inventory/capacity is fetched by
// the real TLS client. This is not a capture from a native appliance.
type poolCapacityClockFetcher struct {
	truenas.APIFetcher
	at time.Time
}

func (f *poolCapacityClockFetcher) Fetch(ctx context.Context) (*truenas.FixtureSnapshot, error) {
	snapshot, err := f.APIFetcher.Fetch(ctx)
	if err == nil {
		snapshot.CollectedAt = f.at
	}
	return snapshot, err
}

func TestTrueNASPoolCapacityCollectionHistory(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	previousMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previousMock) })

	var cycle atomic.Int32
	var poolReads atomic.Int32
	fields := []string{
		`,"size":1000,"allocated":400,"free":600`,
		`,"size":1000`, // Must not refresh the last reading as zero.
		`,"size":1000,"allocated":"unknown","free":600`,
		`,"size":1000,"allocated":1000,"free":0`,
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/current" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-a" && r.Header.Get("Authorization") != "Bearer synthetic-b" {
			t.Error("collection lost its connection authentication")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			body = `{"hostname":"same-host","version":"TrueNAS-13.0-U6.8","cores":4,"physmem":4096}`
		case "/api/v2.0/pool":
			poolReads.Add(1)
			current := fields[cycle.Load()]
			if r.Header.Get("Authorization") == "Bearer synthetic-b" {
				current = `,"size":2000,"allocated":500,"free":1500`
			}
			body = `[{"id":1,"name":"tank","status":"DEGRADED","scan":{"function":"SCRUB","state":"FINISHED"},"topology":{"data":[{"type":"DISK","disk":"da0","status":"DEGRADED"}]}` + current + `}]`
		case "/api/v2.0/boot/get_state":
			body = `{"name":"boot-pool","status":"ONLINE","properties":{"size":{"parsed":200},"allocated":{"parsed":50},"free":{"parsed":150}}}`
		case "/api/v2.0/pool/dataset":
			body = `[{"id":"tank/quota","name":"tank/quota","pool":"tank","used":{"parsed":30},"available":{"parsed":70}}]`
		case "/api/v2.0/disk":
			body = `[{"identifier":"disk-1","name":"da0","size":10000,"rotationrate":"unknown","type":"UNKNOWN"}]`
		case "/api/v2.0/alert/list", "/api/v2.0/service", "/api/v2.0/app", "/api/v2.0/vm", "/api/v2.0/sharing/smb", "/api/v2.0/sharing/nfs", "/api/v2.0/zfs/snapshot", "/api/v2.0/replication", "/api/v2.0/reporting/graphs", "/api/v2.0/reporting/get_data":
			body = `[]`
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
	start := time.Now().UTC().Truncate(time.Second).Add(-5 * time.Minute)
	providers := make([]*truenas.Provider, 2)
	fetchers := make([]*poolCapacityClockFetcher, 2)
	for i, name := range []string{"a", "b"} {
		// Trust only this synthetic certificate's pin. The self-signed lab
		// certificate is not a host trust-store or production TLS change.
		client, err := truenas.NewClient(truenas.ClientConfig{Host: server.URL, UseHTTPS: true, InsecureSkipVerify: true, Fingerprint: fingerprint, APIKey: "synthetic-" + name})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(client.Close)
		fetchers[i] = &poolCapacityClockFetcher{APIFetcher: truenas.APIFetcher{Client: client}, at: start}
		providers[i] = truenas.NewLiveProviderForConnection(fetchers[i], "connection-"+name)
	}
	persistent, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = persistent.Close() })
	adapter := unifiedresources.NewMonitorAdapter(nil)
	monitor := &Monitor{resourceStore: adapter, metricsHistory: NewMetricsHistory(100, time.Hour), metricsStore: persistent}
	hostA := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAgent, unifiedresources.SourceTrueNAS, "system:connection-a")
	hostB := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAgent, unifiedresources.SourceTrueNAS, "system:connection-b")
	targets := map[string]string{}
	for step := range fields {
		cycle.Store(int32(step))
		var records []unifiedresources.IngestRecord
		for i, provider := range providers {
			fetchers[i].at = start.Add(time.Duration(step) * time.Minute)
			if err := provider.Refresh(context.Background()); err != nil {
				t.Fatalf("capacity metadata aborted collection: %v", err)
			}
			records = append(records, provider.Records()...)
		}
		adapter.PopulateSnapshotAndSupplemental(models.StateSnapshot{}, map[unifiedresources.DataSource][]unifiedresources.IngestRecord{unifiedresources.SourceTrueNAS: records})
		hostAChecked := false
		for _, resource := range adapter.GetAll() {
			if resource.Storage != nil && resource.Name == "tank" {
				if resource.ParentID == nil {
					t.Fatal("pool lost its owning connection")
				}
				targets[*resource.ParentID] = resource.ID
				if resource.Status != unifiedresources.StatusWarning || resource.Storage.ZFSPool == nil || resource.Storage.ZFSPool.ScanDetails == nil {
					t.Fatal("capacity availability lost pool health/scan")
				}
			}
			if resource.Type == unifiedresources.ResourceTypeAgent && resource.ID == hostA {
				hostAChecked = true
				if (resource.Metrics.Disk != nil) != (step == 0 || step == 3) {
					t.Fatal("known boot pool was presented as whole-host capacity")
				}
			}
		}
		if !hostAChecked || targets[hostA] == "" || targets[hostB] == "" || targets[hostA] == targets[hostB] {
			t.Fatal("host availability and two independent pool identities were not exercised")
		}
		monitor.syncUnifiedStorageMetrics(adapter)
	}
	if poolReads.Load() != 8 || len(targets) != 2 {
		t.Fatalf("reads=%d independent pool identities=%d", poolReads.Load(), len(targets))
	}
	persistent.Flush()
	for _, resource := range adapter.GetAll() {
		if resource.Storage == nil {
			continue
		}
		wantValues := []float64{40, 100}
		wantTimes := []time.Time{start, start.Add(3 * time.Minute)}
		if resource.Name == "tank/quota" {
			wantValues = []float64{30, 30, 30, 30}
			wantTimes = []time.Time{start, start.Add(time.Minute), start.Add(2 * time.Minute), start.Add(3 * time.Minute)}
		} else if resource.Name == "boot-pool" {
			continue
		} else if resource.ParentID != nil && *resource.ParentID == hostB {
			wantValues = []float64{25, 25, 25, 25}
			wantTimes = []time.Time{start, start.Add(time.Minute), start.Add(2 * time.Minute), start.Add(3 * time.Minute)}
		}
		// Metrics target resolution, not a guessed cross-host pool name, owns
		// both the memory series and persistent History identity.
		target := adapter.MetricsTargetForResource(resource.ID)
		if target == nil || target.ResourceType != "storage" {
			t.Fatal("pool/dataset lost its storage target")
		}
		memory := monitor.GetStorageMetrics(target.ResourceID, time.Hour)["usage"]
		stored, err := persistent.Query("storage", target.ResourceID, "usage", start.Add(-time.Second), time.Now(), 0)
		if err != nil || len(memory) != len(wantValues) || len(stored) != len(wantValues) {
			t.Fatalf("%s History points memory=%d persisted=%d want=%d err=%v", resource.ID, len(memory), len(stored), len(wantValues), err)
		}
		for i, value := range wantValues {
			if memory[i].Value != value || stored[i].Value != value || !memory[i].Timestamp.Equal(wantTimes[i]) || !stored[i].Timestamp.Equal(wantTimes[i]) {
				t.Fatal("unknown capacity became zero, stale refresh, wrong units, timestamp or connection")
			}
		}
	}
}
