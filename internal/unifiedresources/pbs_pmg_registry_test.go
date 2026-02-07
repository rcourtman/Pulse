package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIngestSnapshotIncludesPBSAndPMGInstances(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{
				ID:               "pbs-1",
				Name:             "pbs-main",
				Host:             "https://pbs.example.com:8007",
				Status:           "online",
				Version:          "3.2.1",
				CPU:              22.5,
				Memory:           48.0,
				MemoryUsed:       8 * 1024 * 1024 * 1024,
				MemoryTotal:      16 * 1024 * 1024 * 1024,
				Uptime:           86400,
				Datastores:       []models.PBSDatastore{{Name: "fast"}},
				BackupJobs:       []models.PBSBackupJob{{ID: "job-1"}},
				ConnectionHealth: "online",
				LastSeen:         now,
			},
		},
		PMGInstances: []models.PMGInstance{
			{
				ID:               "pmg-1",
				Name:             "pmg-main",
				Host:             "https://pmg.example.com:8006",
				Status:           "online",
				Version:          "8.2",
				ConnectionHealth: "connected",
				LastSeen:         now,
				LastUpdated:      now,
				MailStats: &models.PMGMailStats{
					CountTotal: 1900,
					BytesIn:    5_000_000,
					BytesOut:   4_000_000,
					SpamIn:     125,
					VirusIn:    4,
				},
				Nodes: []models.PMGNodeStatus{
					{
						Name:   "pmg-node-1",
						Status: "online",
						Uptime: 43200,
						QueueStatus: &models.PMGQueueStatus{
							Active:    12,
							Deferred:  5,
							Hold:      2,
							Incoming:  3,
							Total:     22,
							UpdatedAt: now,
						},
					},
				},
			},
		},
	}

	registry := NewRegistry(NewMemoryStore())
	registry.IngestSnapshot(snapshot)

	resources := registry.List()
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	var pbsResource *Resource
	var pmgResource *Resource
	for i := range resources {
		resource := resources[i]
		switch resource.Type {
		case ResourceTypePBS:
			pbsResource = &resource
		case ResourceTypePMG:
			pmgResource = &resource
		}
	}

	if pbsResource == nil {
		t.Fatal("expected PBS resource")
	}
	if !containsDataSource(pbsResource.Sources, SourcePBS) {
		t.Fatalf("expected PBS source, got %+v", pbsResource.Sources)
	}
	if pbsResource.Metrics == nil || pbsResource.Metrics.CPU == nil {
		t.Fatalf("expected PBS CPU metrics, got %+v", pbsResource.Metrics)
	}
	if pbsResource.PBS == nil || pbsResource.PBS.DatastoreCount != 1 {
		t.Fatalf("expected PBS payload with datastore count, got %+v", pbsResource.PBS)
	}

	if pmgResource == nil {
		t.Fatal("expected PMG resource")
	}
	if !containsDataSource(pmgResource.Sources, SourcePMG) {
		t.Fatalf("expected PMG source, got %+v", pmgResource.Sources)
	}
	if pmgResource.Metrics == nil {
		t.Fatalf("expected PMG metrics payload, got nil")
	}
	if pmgResource.PMG == nil || pmgResource.PMG.QueueTotal != 22 {
		t.Fatalf("expected PMG payload with queue totals, got %+v", pmgResource.PMG)
	}
}
