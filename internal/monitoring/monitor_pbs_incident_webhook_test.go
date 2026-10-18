package monitoring

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

// #1686: metric fixtures and direct sender calls do not establish that a
// provider incident reaches ordinary recovery delivery. Use the real PBS
// adapter, alert lifecycle, Monitor callbacks and autonomous persistent queue.
// The report-shaped template is synthetic, not the unknown HA automation;
// local HTTP acceptance does not establish creation/removal on an HA device.
func TestPBSProviderIncidentWebhookLifecycle(t *testing.T) {
	for _, shape := range []string{"datastore", "full-pbs"} {
		for _, grouped := range []bool{false, true} {
			name := shape + "/individual"
			if grouped {
				name = shape + "/grouped"
			}
			t.Run(name, func(t *testing.T) { testPBSProviderIncidentWebhookLifecycle(t, shape, grouped) })
		}
	}
}

type pbsWebhookMember struct {
	ID        string          `json:"id"`
	Start     string          `json:"start"`
	Type      string          `json:"type"`
	Resource  string          `json:"resource"`
	Value     json.RawMessage `json:"value"`
	Threshold json.RawMessage `json:"threshold"`
}

type pbsWebhookReceipt struct {
	EventID string
	Body    json.RawMessage
}

func testPBSProviderIncidentWebhookLifecycle(t *testing.T, shape string, grouped bool) {
	requests := make(chan pbsWebhookReceipt, 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		if err != nil || !json.Valid(body) || r.Method != http.MethodPost {
			t.Error("invalid ordinary webhook request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- pbsWebhookReceipt{EventID: r.Header.Get("X-Pulse-Event-ID"), Body: body}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	n := notifications.NewNotificationManagerWithDeferredQueue("https://pulse.example.invalid", t.TempDir())
	t.Cleanup(n.Stop)
	require.NoError(t, n.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"))
	n.SetNotifyOnResolve(true)
	window := 0
	if grouped {
		window = 1
	}
	n.SetGroupingConfig(grouped, window, true, false)
	// Keep the primary fields visible too: a group primary is not a list of
	// all its members, and Value=0 on a discrete incident is not boolean false.
	n.AddWebhook(notifications.WebhookConfig{ID: "synthetic-pbs", Name: "synthetic-pbs", Enabled: true,
		URL: server.URL, Service: "generic", Template: `{
	"event":"{{.Event}}", "p_id":"{{.ID | jsonString}}", "p_type":"{{.Type | jsonString}}",
	"p_value":{{.Value}}, "p_threshold":{{.Threshold}}, "resolved":"{{.ResolvedAtISO}}", "count":{{.AlertCount}},
	"alerts":[{{range $i, $a := .Alerts}}{{if $i}},{{end}}{
	"id":"{{$a.ID | jsonString}}", "start":"{{$a.StartTime.UTC.Format "2006-01-02T15:04:05.999999999Z07:00"}}",
	"type":"{{$a.Type | jsonString}}", "resource":"{{$a.ResourceID | jsonString}}",
	"value":{{$a.Value}}, "threshold":{{$a.Threshold}}}{{end}}]}`})
	a := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(a.Stop)
	cfg := a.GetConfig()
	cfg.Enabled, cfg.ActivationState, cfg.FlappingEnabled = true, alerts.ActivationActive, false
	cfg.Schedule.QuietHours.Enabled = false
	a.UpdateConfig(cfg)
	m := &Monitor{alertManager: a, notificationMgr: n}
	m.wireExternalAlertCallbacks(nil)

	produce := func(unavailable bool) []unifiedresources.Resource {
		status, detail := "available", ""
		if unavailable {
			status, detail = "unavailable", "synthetic datastore observation"
		}
		snapshot := models.StateSnapshot{PBSInstances: []models.PBSInstance{{
			ID: "synthetic-pbs", Name: "synthetic-backup", Host: "https://pbs.example.invalid:8007",
			Status: "online", ConnectionHealth: "healthy", LastSeen: time.Now(),
			Datastores: []models.PBSDatastore{{Name: "synthetic-store", Status: status, Error: detail}},
		}}}
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(snapshot)
		resources := registry.List()
		if shape == "datastore" {
			// Isolate the original datastore producer shape as well as testing
			// the complete PBS snapshot. Do not hand-build an Alert or change
			// live parent/child suppression policy.
			filtered := make([]unifiedresources.Resource, 0, len(resources))
			for _, resource := range resources {
				if resource.Type == unifiedresources.ResourceTypeStorage {
					filtered = append(filtered, resource)
				}
			}
			resources = filtered
		}
		return resources
	}
	wantMembers := 4
	if shape == "datastore" {
		wantMembers = 2
	}
	activeMembers := func() map[string]pbsWebhookMember {
		t.Helper()
		active := a.GetActiveAlerts()
		require.Len(t, active, wantMembers, "status and error are distinct provider/datastore incidents")
		typeCounts := make(map[string]int)
		members := make(map[string]pbsWebhookMember, len(active))
		for _, alert := range active {
			require.Contains(t, alert.ID, "::alertspec:provider-incident:")
			require.Contains(t, []string{"backup-posture-incident", "backup-storage-incident"}, alert.Type)
			typeCounts[alert.Type]++
			require.Equal(t, alerts.AlertLevelCritical, alert.Level)
			require.False(t, alert.StartTime.IsZero())
			if shape == "full-pbs" && alert.Type == "backup-storage-incident" {
				diagnosis, exists := a.DiagnoseAlertDelivery(alert.ID)
				require.True(t, exists)
				require.Equal(t, alerts.AlertDeliveryReasonCorrelatedPrimary, diagnosis.Reason,
					"the complete PBS roll-up owns delivery for its supported datastore symptoms")
				continue
			}
			members[alert.ID] = pbsWebhookMember{ID: alert.ID, Start: alert.StartTime.UTC().Format(time.RFC3339Nano),
				Type: alert.Type, Resource: alert.ResourceID, Value: json.RawMessage("0"), Threshold: json.RawMessage("0")}
		}
		require.Equal(t, 2, typeCounts["backup-storage-incident"])
		if shape == "full-pbs" {
			require.Equal(t, 2, typeCounts["backup-posture-incident"])
		}
		require.Len(t, members, 2, "two independent status/error occurrences own delivery")
		return members
	}

	// Do not install a fake processor, fabricate receipts, call Test, force the
	// worker, or send directly. Inspect real pending rows before starting it.
	a.SyncUnifiedResourceIncidents(produce(true))
	first := activeMembers()
	wantRows := len(first)
	if grouped {
		wantRows = 1
	}
	rows := awaitOccurrenceRows(t, n, wantRows)
	queued := make(map[string]pbsWebhookMember)
	for _, row := range rows {
		require.Equal(t, "webhook", row.Type)
		for _, alert := range row.Alerts {
			want, ok := first[alert.ID]
			require.True(t, ok)
			require.Equal(t, want.Start, alert.StartTime.UTC().Format(time.RFC3339Nano))
			queued[alert.ID] = want
		}
	}
	require.Equal(t, first, queued, "every delivery-owning occurrence must enter the ordinary queue")
	select {
	case <-requests:
		t.Fatal("HTTP delivery bypassed the deferred ordinary queue")
	default:
	}
	n.StartQueueProcessing()

	// Each receipt is the actual rendered HTTP body, not a second renderer.
	// Compare member identity even if grouping changes the chosen primary.
	sent := 0
	receive := func(event string, want map[string]pbsWebhookMember) map[string]string {
		t.Helper()
		seen, correlations := make(map[string]pbsWebhookMember), make(map[string]string)
		for len(seen) < len(want) {
			select {
			case receipt := <-requests:
				var payload struct {
					Event     string             `json:"event"`
					ID        string             `json:"p_id"`
					Type      string             `json:"p_type"`
					Value     json.RawMessage    `json:"p_value"`
					Threshold json.RawMessage    `json:"p_threshold"`
					Resolved  string             `json:"resolved"`
					Count     int                `json:"count"`
					Members   []pbsWebhookMember `json:"alerts"`
				}
				require.NoError(t, json.Unmarshal(receipt.Body, &payload))
				require.Equal(t, event, payload.Event)
				require.NotEmpty(t, payload.Members)
				require.Equal(t, payload.Members[0].Type, payload.Type)
				require.Equal(t, json.RawMessage("0"), payload.Value)
				require.Equal(t, json.RawMessage("0"), payload.Threshold)
				require.NotEmpty(t, payload.Members)
				require.Equal(t, len(payload.Members), payload.Count)
				require.Equal(t, payload.Members[0].ID, payload.ID)
				require.Equal(t, payload.ID+":"+event, receipt.EventID)
				if event == "resolved" {
					_, err := time.Parse(time.RFC3339, payload.Resolved)
					require.NoError(t, err)
				} else {
					require.Empty(t, payload.Resolved)
				}
				for _, member := range payload.Members {
					expected, exists := want[member.ID]
					require.True(t, exists, "wrong incident delivered")
					require.Equal(t, expected, member)
					_, duplicate := seen[member.ID]
					require.False(t, duplicate, "duplicate occurrence delivery")
					seen[member.ID], correlations[member.ID] = member, receipt.EventID
				}
				sent++
				t.Logf("PBS_WEBHOOK_RECEIPT event_id=%s body=%s", receipt.EventID, receipt.Body)
			case <-time.After(12 * time.Second):
				t.Fatalf("ordinary %s HTTP missing: received %d/%d members", event, len(seen), len(want))
			}
		}
		require.Eventually(t, func() bool {
			stats, err := n.GetQueueStats()
			return err == nil && stats["sent"] == sent && stats["pending"] == 0 && stats["sending"] == 0 &&
				stats["failed"] == 0 && stats["dlq"] == 0 && stats["cancelled"] == 0
		}, 3*time.Second, time.Millisecond, "HTTP must complete through the persistent queue")
		return correlations
	}
	firstCorrelations := receive("alert", first)
	a.SyncUnifiedResourceIncidents(produce(false))
	require.Empty(t, a.GetActiveAlerts())
	resolvedCorrelations := receive("resolved", first)
	for id, member := range first {
		resolved := a.GetResolvedAlert(id)
		require.NotNil(t, resolved)
		require.Equal(t, member.Start, resolved.Alert.StartTime.UTC().Format(time.RFC3339Nano))
		require.NotEqual(t, firstCorrelations[id], resolvedCorrelations[id], "event correlation distinguishes firing from recovery")
	}
	require.Len(t, a.GetAlertHistory(10), wantMembers)

	// The existing short-refire continuity policy reopens the same occurrence.
	// A fresh firing must still deliver after its recovery consumed the receipt;
	// neither an old cooldown nor the reused correlation header may discard it.
	a.SyncUnifiedResourceIncidents(produce(true))
	current := activeMembers()
	for id, member := range current {
		require.Equal(t, first[id].ID, id)
		require.Equal(t, first[id].Start, member.Start, "short refire retains occurrence continuity")
	}
	receive("alert", current)
	a.SyncUnifiedResourceIncidents(produce(false))
	require.Empty(t, a.GetActiveAlerts())
	receive("resolved", current)
	require.Len(t, a.GetAlertHistory(10), wantMembers, "short refires retain the existing history row")
	select {
	case extra := <-requests:
		t.Fatalf("unexpected delivery after completed lifecycles: %s", extra.Body)
	case <-time.After(100 * time.Millisecond):
	}
	t.Logf("PBS_LIFECYCLE_COMPLETE shape=%s grouped=%t phases=4 active_incidents=%d delivered_members_per_phase=%d requests=%d", shape, grouped, wantMembers, len(first), sent)
}
