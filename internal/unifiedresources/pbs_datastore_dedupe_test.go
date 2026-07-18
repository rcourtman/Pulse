package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A PBS datastore reaches the registry twice: once via the PBS instance
// snapshot (canonical "<instance-id>/<name>" source ID) and once via the PBS
// poller's models.Storage conversion (legacy "<instance-id>-<name>" ID). Both
// must resolve to ONE resource, or the thresholds page renders duplicate cards
// and overrides split across two key formats (#1591).
func TestIngestSnapshot_PBSDatastoreStorageConversionDoesNotDuplicate(t *testing.T) {
	now := time.Now()
	registry := NewRegistry(nil)

	registry.IngestSnapshot(models.StateSnapshot{
		PBSInstances: []models.PBSInstance{{
			ID:       "pbs-pbs-docker",
			Name:     "pbs-docker",
			Host:     "https://pbs-docker.lan:8007",
			Status:   "online",
			LastSeen: now,
			Datastores: []models.PBSDatastore{{
				Name:   "main",
				Total:  1000,
				Used:   400,
				Free:   600,
				Usage:  40,
				Status: "available",
			}},
		}},
		Storage: []models.Storage{{
			ID:       "pbs-pbs-docker-main",
			AliasIDs: []string{"pbs-pbs-docker/main"},
			Name:     "main",
			Node:     "pbs-docker",
			Instance: "pbs-pbs-docker",
			Type:     "pbs",
			Status:   "available",
			Total:    1000,
			Used:     400,
			Free:     600,
			Usage:    40,
			Content:  "backup",
			Enabled:  true,
			Active:   true,
			LastSeen: now,
		}},
	})

	var datastores []Resource
	for _, resource := range registry.List() {
		if resource.Type == ResourceTypeStorage && resource.Name == "main" {
			datastores = append(datastores, resource)
		}
	}

	if len(datastores) != 1 {
		t.Fatalf("expected one unified resource for the PBS datastore, got %d: %+v", len(datastores), datastores)
	}
	if datastores[0].Storage == nil || datastores[0].Storage.Type != "pbs-datastore" {
		t.Fatalf("expected the canonical pbs-datastore resource to survive, got %+v", datastores[0].Storage)
	}
}
