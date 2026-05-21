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
				ID:          "pbs-1",
				Name:        "pbs-main",
				Host:        "https://pbs.example.com:8007",
				Status:      "online",
				Version:     "3.2.1",
				CPU:         22.5,
				Memory:      48.0,
				MemoryUsed:  8 * 1024 * 1024 * 1024,
				MemoryTotal: 16 * 1024 * 1024 * 1024,
				Uptime:      86400,
				Datastores: []models.PBSDatastore{{
					Name:   "fast",
					Status: "online",
					Total:  100,
					Used:   96,
				}},
				BackupJobs: []models.PBSBackupJob{{ID: "job-1"}},
				JobHealthEvidence: []models.PBSJobHealthEvidence{{
					ID:             "sync-remote-a",
					Family:         "sync",
					Store:          "fast",
					LastRunState:   "OK",
					LastRunUPID:    "UPID:sync:1",
					LastRunEndtime: now.Add(-time.Hour).Unix(),
					Confidence:     "direct-task-match",
					EvidenceSource: "pbs-job-config",
					EvidenceScope:  "configured-job",
					Freshness: models.PBSJobHealthFreshness{
						ObservedAt:     now,
						LastRunEndTime: now.Add(-time.Hour),
						State:          "observed",
					},
					Posture: "healthy",
				}},
				ConnectionHealth: "online",
				LastSeen:         now,
			},
		},
		PMGInstances: []models.PMGInstance{
			{
				ID:               "pmg-1",
				Name:             "pmg-main",
				Host:             "https://pmg.example.com:8006",
				GuestURL:         "https://pmg.example.com/quarantine",
				Status:           "online",
				Version:          "8.2",
				ConnectionHealth: "connected",
				LastSeen:         now,
				LastUpdated:      now,
				MailStats: &models.PMGMailStats{
					CountTotal:      1900,
					BytesIn:         5_000_000,
					BytesOut:        4_000_000,
					SpamIn:          125,
					VirusIn:         4,
					JunkIn:          32,
					PregreetRejects: 7,
					UpdatedAt:       now,
				},
				MailCount: []models.PMGMailCountPoint{{Timestamp: now, Count: 1900, CountIn: 1200, CountOut: 700, Timeframe: "hour", Index: 1}},
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
							OldestAge: 1800,
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
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	var pbsResource *Resource
	var datastoreResource *Resource
	var pmgResource *Resource
	for i := range resources {
		resource := resources[i]
		switch resource.Type {
		case ResourceTypePBS:
			pbsResource = &resource
		case ResourceTypeStorage:
			if resource.Storage != nil && resource.Storage.Platform == "pbs" {
				datastoreResource = &resource
			}
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
	if pbsResource.PBS.JobHealthEvidenceCount != 1 || len(pbsResource.PBS.JobHealthEvidence) != 1 {
		t.Fatalf("expected PBS job health evidence ledger, got %+v", pbsResource.PBS)
	}
	if got := pbsResource.PBS.JobHealthEvidence[0]; got.Confidence != "direct-task-match" || got.EvidenceSource != "pbs-job-config" || got.EvidenceScope != "configured-job" || got.LastRunState != "OK" {
		t.Fatalf("expected direct PBS job evidence with raw fields, got %+v", got)
	}
	if pbsResource.Status != StatusWarning {
		t.Fatalf("expected PBS instance warning status from rolled-up datastore risk, got %q", pbsResource.Status)
	}
	if pbsResource.PBS.StorageRisk == nil || len(pbsResource.PBS.StorageRisk.Reasons) == 0 {
		t.Fatalf("expected PBS instance storage risk payload, got %+v", pbsResource.PBS)
	}
	if len(pbsResource.Incidents) == 0 || pbsResource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("expected rolled-up PBS incidents, got %+v", pbsResource.Incidents)
	}
	if datastoreResource == nil {
		t.Fatal("expected PBS datastore storage resource")
	}
	if datastoreResource.Storage == nil {
		t.Fatalf("expected PBS datastore storage metadata, got %+v", datastoreResource)
	}
	if datastoreResource.Storage.Platform != "pbs" || datastoreResource.Storage.Topology != "datastore" {
		t.Fatalf("expected PBS datastore platform/topology, got %+v", datastoreResource.Storage)
	}
	if datastoreResource.Status != StatusWarning {
		t.Fatalf("expected PBS datastore warning status from derived risk, got %q", datastoreResource.Status)
	}
	if datastoreResource.Storage.Risk == nil || len(datastoreResource.Storage.Risk.Reasons) == 0 {
		t.Fatalf("expected PBS datastore risk payload, got %+v", datastoreResource.Storage)
	}
	if len(datastoreResource.Incidents) == 0 || datastoreResource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("expected PBS datastore incidents, got %+v", datastoreResource.Incidents)
	}
	if datastoreResource.ParentID == nil || *datastoreResource.ParentID != pbsResource.ID {
		t.Fatalf("expected PBS datastore to be parented under PBS instance, got %+v", datastoreResource.ParentID)
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
	if pmgResource.PMG.HostURL != "https://pmg.example.com:8006" || pmgResource.PMG.GuestURL != "https://pmg.example.com/quarantine" {
		t.Fatalf("expected PMG URL payloads, got %+v", pmgResource.PMG)
	}
	if len(pmgResource.PMG.Nodes) != 1 || pmgResource.PMG.Nodes[0].QueueStatus == nil || pmgResource.PMG.Nodes[0].QueueStatus.OldestAge != 1800 {
		t.Fatalf("expected PMG queue oldest age, got %+v", pmgResource.PMG.Nodes)
	}
	if pmgResource.PMG.MailStats == nil || pmgResource.PMG.MailStats.CountTotal != 1900 || pmgResource.PMG.MailStats.JunkIn != 32 || pmgResource.PMG.MailStats.PregreetRejects != 7 {
		t.Fatalf("expected full PMG mail stats payload, got %+v", pmgResource.PMG.MailStats)
	}
	if len(pmgResource.PMG.MailCount) != 1 || pmgResource.PMG.MailCount[0].Index != 1 {
		t.Fatalf("expected PMG mail count points, got %+v", pmgResource.PMG.MailCount)
	}
}
