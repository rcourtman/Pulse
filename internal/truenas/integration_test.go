package truenas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestIntegrationFullLifecycle(t *testing.T) {
	server := newMockServer(t, defaultAPIResponses(), nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	fetcher := &APIFetcher{Client: client}
	provider := NewLiveProvider(fetcher)

	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	enableTrueNASFeatureFlag(t)
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected non-empty records after successful refresh")
	}

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	resources := registry.List()
	if len(resources) == 0 {
		t.Fatal("expected resources in registry after ingest")
	}

	foundHost := false
	foundPoolStorage := false
	foundDatasetStorage := false
	foundPoolHealthTag := false

	for _, resource := range resources {
		if resource.Type == unifiedresources.ResourceTypeHost && resource.Name == "truenas-main" {
			foundHost = true
		}

		if resource.Type != unifiedresources.ResourceTypeStorage {
			continue
		}

		if resource.Storage == nil {
			t.Fatalf("expected StorageMeta for storage resource %q", resource.Name)
		}
		if !resource.Storage.IsZFS {
			t.Fatalf("expected IsZFS=true for storage resource %q", resource.Name)
		}

		if hasTag(resource.Tags, "pool") {
			foundPoolStorage = true
			if hasTagPrefix(resource.Tags, "health:") {
				foundPoolHealthTag = true
			}
		}
		if hasTag(resource.Tags, "dataset") {
			foundDatasetStorage = true
		}
	}

	if !foundHost {
		t.Fatal("expected host resource named truenas-main")
	}
	if !foundPoolStorage {
		t.Fatal("expected at least one storage resource with pool tag")
	}
	if !foundDatasetStorage {
		t.Fatal("expected at least one storage resource with dataset tag")
	}
	if !foundPoolHealthTag {
		t.Fatal("expected at least one pool resource with health tag")
	}
}

func TestIntegrationConnectionRefused(t *testing.T) {
	client, err := NewClient(ClientConfig{
		Host:     "127.0.0.1",
		Port:     65534,
		UseHTTPS: false,
		APIKey:   "api-key",
		Timeout:  250 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	provider := NewLiveProvider(&APIFetcher{Client: client})
	if err := provider.Refresh(context.Background()); err == nil {
		t.Fatal("expected Refresh() error for unreachable endpoint")
	}

	enableTrueNASFeatureFlag(t)
	if records := provider.Records(); records != nil {
		t.Fatalf("expected nil records with no cached snapshot, got %d", len(records))
	}
}

func TestIntegrationAuthFailure(t *testing.T) {
	responses := make(map[string]apiResponse)
	for path := range defaultAPIResponses() {
		responses[path] = apiResponse{
			status: http.StatusUnauthorized,
			body:   `{"error":"unauthorized"}`,
		}
	}

	server := newMockServer(t, responses, nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "bad-key"})
	provider := NewLiveProvider(&APIFetcher{Client: client})

	err := provider.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected Refresh() auth failure error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected auth error to contain 401, got %v", err)
	}
}

func TestIntegrationMalformedResponse(t *testing.T) {
	responses := copyAPIResponses(defaultAPIResponses())
	responses["/api/v2.0/system/info"] = apiResponse{
		status: http.StatusOK,
		body:   `{not valid json`,
	}

	server := newMockServer(t, responses, nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	provider := NewLiveProvider(&APIFetcher{Client: client})

	if err := provider.Refresh(context.Background()); err == nil {
		t.Fatal("expected Refresh() error for malformed JSON")
	}
}

func TestIntegrationHealthStateTransition(t *testing.T) {
	dynamic := &dynamicResponses{
		responses: copyAPIResponses(defaultAPIResponses()),
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		response, ok := dynamic.get(request.URL.Path)
		if !ok {
			http.NotFound(writer, request)
			return
		}

		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		contentType := response.contentType
		if contentType == "" {
			contentType = "application/json"
		}

		writer.Header().Set("Content-Type", contentType)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(response.body))
	}))
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	provider := NewLiveProvider(&APIFetcher{Client: client})
	enableTrueNASFeatureFlag(t)

	transitions := []struct {
		poolStatus string
		want       unifiedresources.ResourceStatus
	}{
		{poolStatus: "ONLINE", want: unifiedresources.StatusOnline},
		{poolStatus: "DEGRADED", want: unifiedresources.StatusWarning},
		{poolStatus: "FAULTED", want: unifiedresources.StatusOffline},
		{poolStatus: "ONLINE", want: unifiedresources.StatusOnline},
	}

	for _, transition := range transitions {
		dynamic.set("/api/v2.0/pool", apiResponse{
			status: http.StatusOK,
			body:   poolResponseBody(transition.poolStatus),
		})

		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatalf("Refresh() error for pool status %q: %v", transition.poolStatus, err)
		}

		records := provider.Records()
		if len(records) == 0 {
			t.Fatalf("expected records for pool status %q", transition.poolStatus)
		}

		poolRecord := requirePoolRecord(t, records, "tank")
		if poolRecord.Resource.Status != transition.want {
			t.Fatalf("pool status for %q = %q, want %q", transition.poolStatus, poolRecord.Resource.Status, transition.want)
		}
	}
}

func TestIntegrationStaleRecovery(t *testing.T) {
	t.Skip("redundant with TestProviderRefreshPreservesLastSnapshotOnError in provider_test.go")
}

type dynamicResponses struct {
	mu        sync.Mutex
	responses map[string]apiResponse
}

func (d *dynamicResponses) set(path string, resp apiResponse) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.responses[path] = resp
}

func (d *dynamicResponses) get(path string) (apiResponse, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	resp, ok := d.responses[path]
	return resp, ok
}

func enableTrueNASFeatureFlag(t *testing.T) {
	t.Helper()
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})
}

func requirePoolRecord(t *testing.T, records []unifiedresources.IngestRecord, poolName string) unifiedresources.IngestRecord {
	t.Helper()
	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypeStorage {
			continue
		}
		if record.Resource.Name != poolName {
			continue
		}
		if !hasTag(record.Resource.Tags, "pool") {
			continue
		}
		return record
	}
	t.Fatalf("missing pool record for %q", poolName)
	return unifiedresources.IngestRecord{}
}

func poolResponseBody(status string) string {
	return `[{"id":1,"name":"tank","status":"` + status + `","size":1000,"allocated":400,"free":600}]`
}

func copyAPIResponses(in map[string]apiResponse) map[string]apiResponse {
	out := make(map[string]apiResponse, len(in))
	for path, response := range in {
		out[path] = response
	}
	return out
}

func hasTagPrefix(tags []string, prefix string) bool {
	for _, tag := range tags {
		if strings.HasPrefix(tag, prefix) {
			return true
		}
	}
	return false
}
