package notifications

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Exercise the ordinary grouping timer, autonomous queue, real HTTP bodies and
// occurrence receipts. Changed tags/floors must not steal an owed recovery.
func TestNotificationBatchRoutingHTTPRecipients(t *testing.T) {
	type delivery struct {
		path    string
		Event   string          `json:"event"`
		Grouped bool            `json:"grouped"`
		Count   int             `json:"count"`
		Alerts  []*alerts.Alert `json:"alerts"`
	}
	received := make(chan delivery, 16)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var msg delivery
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("decode ordinary delivery: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		msg.path = r.URL.Path
		select {
		case received <- msg:
		default:
			t.Error("unexpected extra delivery")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	manager := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(manager.Stop)
	manager.SetCooldown(0)
	manager.SetGroupingConfig(true, 1, false, false)
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	hooks := []WebhookConfig{
		{ID: "alpha", URL: server.URL + "/alpha", Enabled: true, Service: "generic", TagFilter: []string{" CUSTOMER:ALPHA "}, MinimumSeverity: "warning"},
		{ID: "critical", URL: server.URL + "/critical", Enabled: true, Service: "generic", TagFilter: []string{"customer:alpha", "customer:beta"}, TagMode: "ANY", MinimumSeverity: "critical"},
		{ID: "global", URL: server.URL + "/global", Enabled: true, Service: "generic", MinimumSeverity: "critical"},
		{ID: "missing", URL: server.URL + "/missing", Enabled: true, Service: "generic", TagFilter: []string{"missing"}},
		{ID: "disabled", URL: server.URL + "/disabled", Service: "generic"},
	}
	for _, hook := range hooks {
		manager.AddWebhook(hook)
	}
	var batch []*alerts.Alert
	for _, tc := range []struct {
		id, tag string
		level   alerts.AlertLevel
	}{
		{"alpha-warning", "customer:alpha", alerts.AlertLevelWarning},
		{"alpha-critical", "customer:alpha", alerts.AlertLevelCritical},
		{"beta-critical", "customer:beta", alerts.AlertLevelCritical},
		{"alpha-info", "customer:alpha", alerts.AlertLevelInfo},
		{"untagged-critical", "", alerts.AlertLevelCritical},
	} {
		a := taggedRoutingAlert(tc.id, []string{tc.tag})
		a.Level, a.ResourceName, a.Type = tc.level, tc.id, "cpu"
		a.StartTime = time.Now().UTC().Add(-time.Minute)
		batch = append(batch, a)
		manager.SendAlert(a)
	}
	manager.StartQueueProcessing()
	want := map[string][]string{
		"/alpha":    {"alpha-warning", "alpha-critical"},
		"/critical": {"alpha-critical", "beta-critical"},
		"/global":   {"alpha-critical", "beta-critical", "untagged-critical"},
	}
	check := func(event string) {
		t.Helper()
		// The existing generic grouped firing payload has grouped/count but
		// no event field; the recovery producer adds event="resolved".
		wantEvent, wantGrouped := "", true
		if event == "resolved" {
			wantEvent, wantGrouped = event, false
		}
		seen := make(map[string]bool)
		deadline := time.NewTimer(15 * time.Second)
		defer deadline.Stop()
		for range len(want) {
			select {
			case msg := <-received:
				ids, known := want[msg.path]
				if !known || seen[msg.path] || msg.Event != wantEvent || msg.Grouped != wantGrouped || msg.Count != len(ids) || !reflect.DeepEqual(alertIDs(msg.Alerts), ids) {
					t.Fatalf("wrong %s recipients: path=%s event=%s grouped=%t count=%d ids=%v", event, msg.path, msg.Event, msg.Grouped, msg.Count, alertIDs(msg.Alerts))
				}
				seen[msg.path] = true
			case <-deadline.C:
				t.Fatalf("missing %s recipients; received destinations=%v", event, seen)
			}
		}
		// The HTTP response precedes local receipt settlement. Wait for actual
		// queue completion before asking it to route the corresponding clear.
		for {
			stats, err := manager.GetQueue().GetQueueStats()
			if err != nil {
				t.Fatal(err)
			}
			wantSent := len(want)
			if event == "resolved" {
				wantSent *= 2
			}
			if stats["sent"] == wantSent && stats["pending"] == 0 && stats["sending"] == 0 {
				break
			}
			select {
			case <-deadline.C:
				t.Fatalf("%s delivery did not settle: %v", event, stats)
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	check("alert")
	for i := range hooks {
		hooks[i].TagFilter, hooks[i].MinimumSeverity = []string{"destination-now-excludes"}, "critical"
		if err := manager.UpdateWebhook(hooks[i].ID, hooks[i]); err != nil {
			t.Fatal(err)
		}
	}
	for _, alert := range batch {
		alert.Metadata["tags"], alert.Level = []string{"changed-after-firing"}, alerts.AlertLevelInfo
		manager.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert, ResolvedTime: time.Now()})
	}
	check("resolved")
	manager.Stop()
	select {
	case msg := <-received:
		t.Fatalf("unexpected extra recipient: path=%s event=%s", msg.path, msg.Event)
	default:
	}
}
