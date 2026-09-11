package resourceapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This is the final leg of collector -> ApplyDockerReport -> /api/resources.
func TestResourcesRegistryResponseQualification(t *testing.T) {
	path := os.Getenv("PULSE_REGISTRY_SNAPSHOT_PROOF")
	if path == "" {
		t.Skip("set PULSE_REGISTRY_SNAPSHOT_PROOF to ingestion qualification output")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshots []models.StateSnapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatal("expected failure and recovery snapshots")
	}
	for phase, snapshot := range snapshots {
		registry := unified.NewRegistry(nil)
		registry.IngestSnapshot(snapshot)
		h := NewQueryService(&config.Config{DataPath: t.TempDir()})
		h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: snapshot, resources: registry.ListByType(unified.ResourceTypeAppContainer)})
		rec := httptest.NewRecorder()
		h.HandleListResources(rec, httptest.NewRequest(http.MethodGet, "/api/resources?type=app-container&limit=100", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
		}
		var response ResourcesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Data) != 4 {
			t.Fatalf("expected four resources: %s", rec.Body.String())
		}
		for _, r := range response.Data {
			var want *models.DockerContainerUpdateStatus
			for _, c := range snapshot.DockerHosts[0].Containers {
				if c.Name == r.Name {
					want = c.UpdateStatus
				}
			}
			if want == nil || r.Docker == nil || r.Docker.UpdateStatus == nil {
				t.Fatalf("lost identity/status: %+v", r)
			}
			got := r.Docker.UpdateStatus
			if got.CurrentDigest != want.CurrentDigest || got.LatestDigest != want.LatestDigest || got.Error != want.Error || got.UpdateAvailable != want.UpdateAvailable {
				t.Fatalf("projection changed status: %+v vs %+v", got, want)
			}
		}
		t.Logf("phase %d endpoint=%s", phase, rec.Body.String())
	}
}
