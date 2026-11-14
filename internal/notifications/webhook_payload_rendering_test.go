package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

// Keep an independent copy of the original parse-per-send renderer as a byte
// and error oracle. In particular, JSON validity alone is not equivalent to
// unmarshalling: valid JSON numbers can still overflow float64.
func freshWebhookPayload(source string, data WebhookPayloadData, service string) ([]byte, error) {
	tmpl, err := template.New("webhook").Funcs(templateFuncMap()).Parse(source)
	if err != nil {
		return nil, failNotificationConfiguration("invalid template", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, failNotificationConfiguration("template execution failed", err)
	}
	if service != "ntfy" {
		var decoded any
		if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
			return nil, failNotificationConfiguration("template produced invalid JSON", err)
		}
	}
	return buf.Bytes(), nil
}

func webhookRenderingFixture(marker string) WebhookPayloadData {
	return WebhookPayloadData{
		ID: marker + "-incident", MessageKey: "connection-degraded", Event: "alert",
		Level: "warning", Type: "cpu", ResourceName: marker + "-guest", ResourceID: marker + "-resource",
		Node: marker + "-node", NodeDisplayName: marker + "-node-display", ResourceType: "vm",
		Instance: "https://pulse.example.test", AlertURL: "https://pulse.example.test/alerts?node=a&kind=cpu",
		Message: marker + " poll_nodes failed: <unreachable> & \"quoted\"\n路径 \\ backup_*[]",
		Value:   91.25, Threshold: 80, ValueFormatted: "91.3%", ThresholdFormatted: "80%",
		StartTime: "2026-11-13T01:00:00Z", Timestamp: "2026-11-13T01:05:00Z", Duration: "5m",
		ResolvedAt: "2026-11-13T01:05:00Z", ResolvedAtISO: "2026-11-13T01:05:00Z",
		ChatID: "-1001234", Mention: "@ops <&> \"值\"", AlertCount: 3,
		TenantID: marker + "-org", TenantName: marker + " organisation",
		CustomFields: map[string]any{"routing_key": marker + "-synthetic-routing-key"},
	}
}

func TestWebhookPayloadRenderingMatchesFreshParse(t *testing.T) {
	manager := &NotificationManager{}
	for _, provider := range GetWebhookTemplates() {
		for _, event := range []struct{ name, source string }{
			{"firing", provider.PayloadTemplate}, {"resolved", provider.ResolvedPayloadTemplate},
		} {
			if event.source == "" {
				continue
			}
			t.Run(provider.Service+"/"+event.name, func(t *testing.T) {
				for i, kind := range []string{"cpu", "diskRead", "diskWrite"} {
					for _, minimal := range []bool{false, true} {
						data := webhookRenderingFixture(fmt.Sprintf("fixture-%d-%t", i, minimal))
						data.Type, data.Level = kind, []string{"critical", "warning", "info"}[i]
						data.NotRecovered = minimal
						if minimal {
							data.Mention, data.TenantID, data.TenantName, data.AlertURL = "", "", "", ""
						}
						if event.name == "resolved" {
							data.Event = "resolved"
						}
						want, err := freshWebhookPayload(event.source, data, provider.Service)
						if err != nil {
							t.Fatalf("fresh render: %v", err)
						}
						got, err := manager.generatePayloadFromTemplateWithService(event.source, data, provider.Service)
						if err != nil {
							t.Fatalf("production render: %v", err)
						}
						if !bytes.Equal(got, want) {
							t.Errorf("%s/minimal=%t changed rendered bytes", kind, minimal)
						}
					}
				}
			})
		}
	}
}

func TestWebhookPayloadRenderingCustomAndErrors(t *testing.T) {
	manager := &NotificationManager{}
	data := webhookRenderingFixture("custom")
	for _, tc := range []struct{ name, source, service string }{
		{"custom", `{"message":"{{.Message | jsonString}}","org":"{{.TenantID | jsonString}}"}`, "generic"},
		{"plain-text", `not JSON: {{.Message}}`, "ntfy"},
		{"template-definitions", `{{define "message"}}{{.Message | jsonString}}{{end}}{"text":"{{template "message" .}}"}`, "generic"},
		{"parse-error", `{{ if }}`, "generic"},
		{"unknown-function", `{{ notAFunction .Message }}`, "generic"},
		{"execution-error", `{"text":"{{.NotAField}}"}`, "generic"},
		{"invalid-json", `{"text":"{{.Message}}"}`, "generic"},
		{"number-overflow", `{"value": 1e1000}`, "generic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, wantErr := freshWebhookPayload(tc.source, data, tc.service)
			got, gotErr := manager.generatePayloadFromTemplateWithService(tc.source, data, tc.service)
			if !bytes.Equal(got, want) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
				t.Fatalf("render differs: error %v, want %v; equal bytes=%t", gotErr, wantErr, bytes.Equal(got, want))
			}
			if gotErr != nil && ClassifyNotificationFailureError(gotErr) != NotificationFailureConfiguration {
				t.Fatalf("error lost configuration classification: %v", gotErr)
			}
		})
	}
	// Validation is per delivery, not a property of the parsed template.
	for _, provider := range GetWebhookTemplates() {
		if provider.Service != "ntfy" {
			continue
		}
		if _, err := manager.generatePayloadFromTemplateWithService(provider.PayloadTemplate, data, "ntfy"); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.generatePayloadFromTemplateWithService(provider.PayloadTemplate, data, "generic"); err == nil {
			t.Fatal("plain-text source must still fail JSON validation for a JSON service")
		}
	}
}

func BenchmarkWebhookPayloadRendering(b *testing.B) {
	data := webhookRenderingFixture("benchmark")
	manager := &NotificationManager{}
	for _, provider := range GetWebhookTemplates() {
		for _, event := range []struct{ name, source string }{
			{"firing", provider.PayloadTemplate}, {"resolved", provider.ResolvedPayloadTemplate},
		} {
			if event.source == "" {
				continue
			}
			b.Run(provider.Service+"/"+event.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					payload, err := manager.generatePayloadFromTemplateWithService(event.source, data, provider.Service)
					if err != nil || len(payload) == 0 {
						b.Fatalf("render failed: %v", err)
					}
				}
			})
		}
	}
	b.Run("custom", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			payload, err := manager.generatePayloadFromTemplateWithService(`{"text":"{{.Message | jsonString}}"}`, data, "generic")
			if err != nil || !strings.Contains(string(payload), "benchmark") {
				b.Fatalf("custom render failed: %v", err)
			}
		}
	})
}

func BenchmarkWebhookDeliveryRendering(b *testing.B) {
	data := webhookRenderingFixture("benchmark")
	manager := &NotificationManager{}
	for _, service := range []string{"discord", "telegram", "slack", "teams-adaptive", "ntfy", "generic"} {
		b.Run(service, func(b *testing.B) {
			webhook := WebhookConfig{Service: service}
			fallback := func() ([]byte, error) { return json.Marshal(data) }
			b.ReportAllocs()
			for b.Loop() {
				payload, err := manager.renderWebhookPayloadJSON(webhook, data, webhookRenderModeGrouped, fallback)
				if err != nil || len(payload) == 0 {
					b.Fatalf("render failed: %v", err)
				}
			}
		})
	}
}
