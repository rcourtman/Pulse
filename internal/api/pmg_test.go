package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestHandleListPMGInstancesUsesUnifiedReadStateWithFullPayload(t *testing.T) {
	now := time.Now().UTC()
	queueUpdated := now.Add(-2 * time.Minute)
	statsUpdated := now.Add(-1 * time.Minute)
	domainStatsAsOf := now.Add(-30 * time.Second)

	source := models.PMGInstance{
		ID:       "pmg-1",
		Name:     "gateway-main",
		Host:     "https://pmg.example.com:8006",
		GuestURL: "https://guest.example.com/pmg",
		Status:   "online",
		Version:  "8.2",
		Nodes: []models.PMGNodeStatus{{
			Name:    "pmg-node-1",
			Status:  "online",
			Role:    "master",
			Uptime:  3600,
			LoadAvg: "0.10",
			QueueStatus: &models.PMGQueueStatus{
				Active:    1,
				Deferred:  2,
				Hold:      3,
				Incoming:  4,
				Total:     10,
				OldestAge: 600,
				UpdatedAt: queueUpdated,
			},
		}},
		MailStats: &models.PMGMailStats{
			Timeframe:            "day",
			CountTotal:           1000,
			CountIn:              700,
			CountOut:             300,
			SpamIn:               11,
			SpamOut:              2,
			VirusIn:              3,
			VirusOut:             4,
			BouncesIn:            5,
			BouncesOut:           6,
			BytesIn:              7000,
			BytesOut:             8000,
			GreylistCount:        9,
			JunkIn:               10,
			AverageProcessTimeMs: 42,
			RBLRejects:           12,
			PregreetRejects:      13,
			UpdatedAt:            statsUpdated,
		},
		MailCount: []models.PMGMailCountPoint{{
			Timestamp: statsUpdated,
			Count:     1000,
			CountIn:   700,
			CountOut:  300,
			Timeframe: "hour",
			Index:     1,
		}},
		SpamDistribution: []models.PMGSpamBucket{{Score: "5", Count: 6}},
		Quarantine:       &models.PMGQuarantineTotals{Spam: 1, Virus: 2, Attachment: 3, Blacklisted: 4},
		RelayDomains:     []models.PMGRelayDomain{{Domain: "example.com", Comment: "primary relay"}},
		DomainStats:      []models.PMGDomainStat{{Domain: "example.com", MailCount: 100, SpamCount: 5, VirusCount: 1, Bytes: 2048}},
		DomainStatsAsOf:  domainStatsAsOf,
		ConnectionHealth: "connected",
		LastSeen:         now,
		LastUpdated:      now,
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{PMGInstances: []models.PMGInstance{source}})
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "resourceStore", unifiedresources.NewMonitorAdapter(registry))

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/pmg/instances?id=pmg-1", nil)
	rec := httptest.NewRecorder()

	router.handleListPMGInstances(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp PMGInstancesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Meta.Total != 1 || len(resp.Data) != 1 {
		t.Fatalf("expected one PMG instance, got total=%d len=%d", resp.Meta.Total, len(resp.Data))
	}

	got := resp.Data[0]
	if got.ID != source.ID || got.Name != source.Name || got.GuestURL != source.GuestURL || got.Status != source.Status || got.Version != source.Version {
		t.Fatalf("basic PMG identity/status fields not preserved: %+v", got)
	}
	if got.Host != source.Host {
		t.Fatalf("expected canonical host %q, got %q", source.Host, got.Host)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].QueueStatus == nil {
		t.Fatalf("expected node queue payload, got %+v", got.Nodes)
	}
	queue := got.Nodes[0].QueueStatus
	if queue.OldestAge != 600 || !queue.UpdatedAt.Equal(queueUpdated) || queue.Incoming != 4 || queue.Total != 10 {
		t.Fatalf("queue payload not preserved: %+v", queue)
	}
	if got.MailStats == nil {
		t.Fatal("expected mail stats payload")
	}
	if got.MailStats.CountTotal != 1000 || got.MailStats.JunkIn != 10 || got.MailStats.PregreetRejects != 13 || !got.MailStats.UpdatedAt.Equal(statsUpdated) {
		t.Fatalf("mail stats payload not preserved: %+v", got.MailStats)
	}
	if len(got.MailCount) != 1 || got.MailCount[0].Count != 1000 || got.MailCount[0].Index != 1 {
		t.Fatalf("mail count payload not preserved: %+v", got.MailCount)
	}
	if len(got.SpamDistribution) != 1 || got.SpamDistribution[0].Score != "5" {
		t.Fatalf("spam distribution not preserved: %+v", got.SpamDistribution)
	}
	if got.Quarantine == nil || got.Quarantine.Blacklisted != 4 {
		t.Fatalf("quarantine payload not preserved: %+v", got.Quarantine)
	}
	if len(got.RelayDomains) != 1 || got.RelayDomains[0].Domain != "example.com" {
		t.Fatalf("relay domains not preserved: %+v", got.RelayDomains)
	}
	if len(got.DomainStats) != 1 || got.DomainStats[0].Bytes != 2048 || !got.DomainStatsAsOf.Equal(domainStatsAsOf) {
		t.Fatalf("domain stats not preserved: stats=%+v asOf=%v", got.DomainStats, got.DomainStatsAsOf)
	}
	if got.ConnectionHealth != "connected" || !got.LastSeen.Equal(now) || !got.LastUpdated.Equal(now) {
		t.Fatalf("health/timestamps not preserved: %+v", got)
	}
}
