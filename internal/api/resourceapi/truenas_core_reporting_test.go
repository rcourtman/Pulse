package resourceapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil/truenascore"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This bridge models the CORE REST schema, not an appliance capture. The
// rejection of query is the reported #2964 response; reporting_query is the
// schema name, distinct from the second positional WebSocket argument. Keep
// this boundary independent of the client's request struct so changing both
// sides to the same wrong spelling cannot make the regression pass.
func TestCORE13RESTReportingQueryResourceJourney(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	fixture := truenascore.NewRESTFixture(t)
	server := fixture.Server
	// Pin this fixture certificate through the production VerifyConnection
	// path; no public CA or trust bypass is needed or changed in the product.
	client, err := truenas.NewClient(truenas.ClientConfig{Host: server.URL, APIKey: "synthetic", InsecureSkipVerify: true, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	// Establish that the fixture really rejects the old envelope over HTTP.
	old, err := server.Client().Post(server.URL+"/api/v2.0/reporting/get_data", "application/json", strings.NewReader(`{"graphs":[{"name":"cpu"}],"query":{"start":1,"end":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = old.Body.Close()
	if old.StatusCode != 400 || fixture.RejectedOldKey.Load() != 1 {
		t.Fatal("fixture does not reproduce the reported query rejection")
	}
	provider := truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, "core-rest-fixture")
	for i, name := range []string{"complete", "rejected", "partial", "zero", "recovered"} {
		t.Run(name, func(t *testing.T) {
			fixture.Phase.Store(int64(i))
			beforePosts, beforeCatalogue := fixture.Posts.Load(), fixture.CatalogueCalls.Load()
			if err := provider.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			snapshot := provider.Snapshot()
			if len(snapshot.Disks) != 1 || len(snapshot.Pools) != 1 || snapshot.System.CPUCount != 8 || snapshot.System.MemoryTotalBytes != 16<<30 {
				t.Fatal("optional reporting failure discarded inventory or capacity")
			}
			registry := unified.NewRegistry(nil)
			registry.IngestRecords(unified.SourceTrueNAS, provider.Records())
			h := NewQueryService(&config.Config{DataPath: t.TempDir()})
			h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: time.Now()}, resources: registry.List()})
			rec := httptest.NewRecorder()
			h.HandleListResources(rec, httptest.NewRequest(http.MethodGet, "/api/resources?source=truenas&type=agent,storage,physical_disk&limit=100", nil))
			var response ResourcesResponse
			if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &response) != nil {
				t.Fatalf("resource HTTP response %d: %s", rec.Code, rec.Body.String())
			}
			var host *unified.Resource
			for index := range response.Data {
				if response.Data[index].Type == unified.ResourceTypeAgent {
					host = &response.Data[index]
				}
			}
			if host == nil || host.Metrics == nil || host.Metrics.Disk == nil || host.Metrics.Disk.Percent != 40 || host.Agent == nil || host.Agent.Memory == nil {
				t.Fatalf("host/storage capacity lost through HTTP: %s", rec.Body.String())
			}
			cpuKnown, memoryKnown := i != 1, i != 1 && i != 2
			if (host.Metrics.CPU != nil) != cpuKnown || (host.Metrics.Memory != nil) != memoryKnown || host.Agent.Memory.UsageUnavailable == memoryKnown {
				t.Fatalf("missing/partial readings became measured zero: %+v", host.Metrics)
			}
			if cpuKnown {
				want := 12.
				if i == 3 {
					want = 0
				}
				if host.Metrics.CPU.Percent != want {
					t.Fatalf("CPU = %v, want %v", host.Metrics.CPU.Percent, want)
				}
			}
			if memoryKnown {
				want := 37.5
				if i == 3 {
					want = 0
				}
				if host.Metrics.Memory.Percent != want {
					t.Fatalf("ARC-aware memory = %v, want %v", host.Metrics.Memory.Percent, want)
				}
			}
			id, history, historyErr := provider.SystemMetricHistory(context.Background(), time.Hour)
			wantCalls := int64(2) // one successful batch each for live and History
			if i == 1 {
				wantCalls = 14 // existing bounded optional-graph isolation
				if historyErr == nil || snapshot.System.Telemetry.ErrorCategory != "request_rejected" {
					t.Fatal("failed reporting disguised as successful History/telemetry")
				}
			} else if historyErr != nil || id != "core-rest-fixture" || len(history["cpu"]) != 2 {
				t.Fatalf("native CPU History lost: %s %+v %v", id, history, historyErr)
			}
			if fixture.Posts.Load()-beforePosts != wantCalls || fixture.CatalogueCalls.Load()-beforeCatalogue != 2 || fixture.RejectedOldKey.Load() != 1 {
				t.Fatal("unexpected request count, alias retry, or old query envelope")
			}
			for _, key := range []string{"memory", "diskread", "diskwrite", "netin", "netout"} {
				if (len(history[key]) > 0) != memoryKnown {
					t.Fatalf("History %s fabricated/lost a sibling: %+v", key, history[key])
				}
			}
			proof, err := json.Marshal(map[string]any{"phase": name, "resources": response, "history": history, "telemetry": snapshot.System.Telemetry})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("CORE_REST_BROWSER %s", proof)
		})
	}
}
