package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestWebhookPayloadStockTransportConsumers(t *testing.T) {
	// Exercise the actual delivery producers and loopback HTTP consumer without
	// starting a notification queue, opening storage or running a reload path.
	for _, provider := range GetWebhookTemplates() {
		for _, path := range []string{"single", "grouped", "resolved", "enhanced"} {
			t.Run(provider.Service+"/"+path, func(t *testing.T) {
				type request struct {
					body    []byte
					headers http.Header
					query   string
				}
				captured := make(chan request, 2)
				server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					captured <- request{body, r.Header.Clone(), r.URL.RawQuery}
					w.WriteHeader(http.StatusNoContent)
				}))
				defer server.Close()
				manager := &NotificationManager{
					publicURL: "https://pulse.example.test", webhookClient: server.Client(),
					webhookRateLimits: make(map[string]*webhookRateLimit),
				}
				manager.SetTenantIdentityResolver(func() (string, string) { return "synthetic-org", "Synthetic org" })
				if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				webhook := WebhookConfig{ID: "synthetic", Name: "Synthetic receiver", Enabled: true,
					Service: provider.Service, URL: server.URL + "/hook", SigningSecret: "synthetic-signing-key",
					Headers:      map[string]string{"routing_key": "synthetic-routing-key"},
					CustomFields: map[string]string{"token": "synthetic-token", "user": "synthetic-user"}}
				if provider.Service == "telegram" {
					webhook.URL += "?chat_id=-1001234"
				}
				batch := []*alerts.Alert{
					{ID: "synthetic-incident-1", ResourceName: "synthetic-guest-1", Node: "synthetic-node-1", Type: "cpu",
						Level: alerts.AlertLevelWarning, Value: 91, Threshold: 80, StartTime: time.Now().Add(-5 * time.Minute),
						Message: "synthetic-guest-1 poll_nodes failed <not markup> & \"quoted\""},
					{ID: "synthetic-incident-2", ResourceName: "synthetic-guest-2", Node: "synthetic-node-2", Type: "cpu",
						Level: alerts.AlertLevelCritical, Value: 95, Threshold: 80, StartTime: time.Now().Add(-4 * time.Minute),
						Message: "synthetic-guest-2 CPU above threshold"},
				}
				var err error
				switch path {
				case "single":
					err = manager.sendGroupedWebhook(webhook, batch[:1])
				case "grouped":
					err = manager.sendGroupedWebhook(webhook, batch)
				case "resolved":
					err = manager.sendResolvedWebhook(webhook, batch, time.Now())
				case "enhanced":
					err = manager.SendEnhancedWebhook(BuildEnhancedWebhookTestConfig(webhook, provider.Service), batch[0])
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(captured) != 1 {
					t.Fatalf("got %d HTTP sends, want exactly one", len(captured))
				}
				got := <-captured
				if !strings.Contains(string(got.body), "synthetic-") {
					t.Fatal("delivery lost its incident data")
				}
				wantType := "application/json"
				if provider.Service == "ntfy" {
					wantType = "text/plain"
				} else {
					var payload any
					if err := json.Unmarshal(got.body, &payload); err != nil {
						t.Fatalf("wire payload is not valid JSON: %v", err)
					}
				}
				if got.headers.Get("Content-Type") != wantType {
					t.Fatalf("Content-Type = %q, want %q", got.headers.Get("Content-Type"), wantType)
				}
				if provider.Service == "telegram" && got.query != "" {
					t.Fatalf("chat_id leaked from payload into query: %q", got.query)
				}
				// The separate legacy ntfy recovery producer does not pass through
				// the shared template/signed sender and is unchanged by this repair.
				if provider.Service != "ntfy" || path != "resolved" {
					event := "alert"
					if path == "resolved" {
						event = "resolved"
					}
					if got.headers.Get("X-Pulse-Event-ID") != "synthetic-incident-1:"+event {
						t.Fatal("delivery changed its event identity")
					}
					timestamp := got.headers.Get("X-Pulse-Timestamp")
					if timestamp == "" || got.headers.Get("X-Pulse-Signature") != "v1="+signWebhookPayload(webhook.SigningSecret, timestamp, got.body) {
						t.Fatal("delivery signature does not bind the actual rendered bytes")
					}
				}
			})
		}
	}
}
