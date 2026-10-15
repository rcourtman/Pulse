package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Notifications link into Pulse with the address the frontend opens one alert
// at (buildAlertHistoryAlertPath in frontend-modern/src/routing/resourceLinks.ts).
func TestAlertHistoryURLMatchesTheFrontendAddress(t *testing.T) {
	got := alertHistoryURL("https://pulse.example/ ", "docker:host/web app::health")
	want := "https://pulse.example/alerts/history?alert=docker%3Ahost%2Fweb+app%3A%3Ahealth"
	if got != want {
		t.Fatalf("alertHistoryURL = %q, want %q", got, want)
	}
	if alertHistoryURL("", "a-1") != "" || alertHistoryURL("https://pulse.example", " ") != "" {
		t.Fatal("alertHistoryURL must be empty without a public URL or an alert id")
	}
}

func TestAlertsPageURLLinksOneAlertOrTheListingPage(t *testing.T) {
	one := []*alerts.Alert{{ID: "a-1"}}
	two := []*alerts.Alert{{ID: "a-1"}, nil, {ID: "a-2"}}
	base := "https://pulse.example"
	if got := alertsPageURL(base, one, false); got != base+"/alerts/history?alert=a-1" {
		t.Fatalf("single alert link = %q", got)
	}
	if got := alertsPageURL(base, two, false); got != base+"/alerts/overview" {
		t.Fatalf("grouped firing link = %q", got)
	}
	if got := alertsPageURL(base, two, true); got != base+"/alerts/history" {
		t.Fatalf("grouped resolved link = %q", got)
	}
	if got := alertsPageURL("", one, false); got != "" {
		t.Fatalf("link without a public URL = %q", got)
	}
}

func linkTestAlert(id, resource string) *alerts.Alert {
	return &alerts.Alert{
		ID:           id,
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   resource,
		ResourceName: resource,
		Node:         "pve1",
		Message:      "CPU above threshold",
		Value:        91,
		Threshold:    80,
		StartTime:    time.Now().Add(-5 * time.Minute),
	}
}

func TestAlertEmailsLinkEachAlertIntoPulse(t *testing.T) {
	base := "https://pulse.example"
	alert := linkTestAlert("vm-101-cpu", "web-01")
	link := base + "/alerts/history?alert=vm-101-cpu"

	_, htmlBody, textBody := EmailTemplate([]*alerts.Alert{alert}, true, base)
	if !strings.Contains(htmlBody, `<a href="`+link+`">View this alert in Pulse</a>`) {
		t.Fatalf("single email HTML lacks the alert link:\n%s", htmlBody)
	}
	if !strings.Contains(textBody, "View this alert in Pulse: "+link) {
		t.Fatalf("single email text lacks the alert link:\n%s", textBody)
	}

	second := linkTestAlert("vm-102-cpu", "web-02")
	_, htmlBody, textBody = EmailTemplate([]*alerts.Alert{alert, second}, false, base)
	for _, want := range []string{
		`<a href="` + link + `" style="color: #1a1a1a;">web-01</a>`,
		`<a href="` + base + `/alerts/history?alert=vm-102-cpu" style="color: #1a1a1a;">web-02</a>`,
		`<a href="` + base + `/alerts/overview">Open these alerts in Pulse</a>`,
	} {
		if !strings.Contains(htmlBody, want) {
			t.Fatalf("grouped email HTML lacks %q", want)
		}
	}
	if !strings.Contains(textBody, "   View in Pulse: "+link) ||
		!strings.Contains(textBody, "Open these alerts in Pulse: "+base+"/alerts/overview") {
		t.Fatalf("grouped email text lacks its links:\n%s", textBody)
	}

	// Without a public URL the emails keep their old pointer and no links.
	_, htmlBody, textBody = EmailTemplate([]*alerts.Alert{alert}, true, "")
	if strings.Contains(htmlBody, "/alerts/history") ||
		!strings.Contains(textBody, "View alerts and configure settings in your Pulse dashboard.") {
		t.Fatal("email without a public URL changed its footer")
	}
}

func TestWebhookPayloadsCarryTheAlertLink(t *testing.T) {
	n := NewNotificationManager("https://pulse.example")
	defer n.Stop()
	alert := linkTestAlert("vm-101-cpu", "web-01")
	data := n.prepareWebhookData(alert, nil)
	if data.AlertURL != "https://pulse.example/alerts/history?alert=vm-101-cpu" {
		t.Fatalf("AlertURL = %q", data.AlertURL)
	}

	bare := NewNotificationManager("")
	defer bare.Stop()
	alert.Instance = "https://pve.example:8006"
	if got := bare.prepareWebhookData(alert, nil).AlertURL; got != alert.Instance {
		t.Fatalf("AlertURL without a public URL = %q, want the Instance fallback", got)
	}

	// Every built-in template that links back to Pulse links to the alert.
	linked := 0
	data.AlertURL = "https://pulse.example/alerts/history?alert=LINK"
	for _, tmpl := range GetWebhookTemplates() {
		for _, source := range []string{tmpl.PayloadTemplate, tmpl.ResolvedPayloadTemplate} {
			if strings.Contains(source, "View in Proxmox") {
				t.Fatalf("template %q still labels its Pulse link as Proxmox", tmpl.Service)
			}
			if !strings.Contains(source, ".AlertURL") {
				continue
			}
			payload, err := n.generatePayloadFromTemplateWithService(source, data, tmpl.Service)
			if err != nil {
				t.Fatalf("template %q: %v", tmpl.Service, err)
			}
			linked++
			if !strings.Contains(string(payload), "alerts/history?alert=LINK") {
				t.Fatalf("template %q does not link to the alert:\n%s", tmpl.Service, payload)
			}
		}
	}
	if linked < 8 {
		t.Fatalf("only %d built-in templates link to the alert, want at least 8", linked)
	}
}

func TestTextNotificationsLinkEachAlert(t *testing.T) {
	base := "https://pulse.example"
	alert := linkTestAlert("vm-101-cpu", "web-01")
	link := "View in Pulse: " + base + "/alerts/history?alert=vm-101-cpu"

	_, body, _ := buildApprisePayload([]*alerts.Alert{alert}, base)
	if !strings.Contains(body, link) {
		t.Fatalf("Apprise body lacks the alert link:\n%s", body)
	}
	_, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert}, time.Now(), base)
	if !strings.Contains(textBody, link) {
		t.Fatalf("resolved body lacks the alert link:\n%s", textBody)
	}
}

// Patrol findings reach notifications as synthetic alerts Alert History never
// stores; a link to History would open its "no longer in history" state.
func TestPatrolFindingNotificationsLinkToPatrol(t *testing.T) {
	base := "https://pulse.example"
	finding := linkTestAlert("patrol-finding-f-1", "db-01")
	finding.Type = "patrol_finding"

	if got := alertLinkURL(base, finding); got != base+"/patrol" {
		t.Fatalf("finding link = %q, want the Patrol page", got)
	}
	if got := alertsPageURL(base, []*alerts.Alert{finding}, false); got != base+"/patrol" {
		t.Fatalf("single finding page link = %q", got)
	}

	n := NewNotificationManager(base)
	defer n.Stop()
	if got := n.prepareWebhookData(finding, nil).AlertURL; got != base+"/patrol" {
		t.Fatalf("finding webhook AlertURL = %q", got)
	}
	_, htmlBody, _ := EmailTemplate([]*alerts.Alert{finding}, true, base)
	if strings.Contains(htmlBody, "/alerts/history") || !strings.Contains(htmlBody, `href="`+base+`/patrol"`) {
		t.Fatalf("finding email does not link to Patrol:\n%s", htmlBody)
	}
	_, body, _ := buildApprisePayload([]*alerts.Alert{finding}, base)
	if strings.Contains(body, "/alerts/history") {
		t.Fatalf("finding Apprise body links to Alert History:\n%s", body)
	}

	// Two findings grouped together still belong to Patrol; a batch that
	// also holds a real alert links to the Alerts page that lists it.
	other := linkTestAlert("patrol-finding-f-2", "db-02")
	other.Type = "patrol_finding"
	pair := []*alerts.Alert{finding, other}
	for _, resolved := range []bool{false, true} {
		if got := alertsPageURL(base, pair, resolved); got != base+"/patrol" {
			t.Fatalf("grouped findings page link (resolved=%v) = %q", resolved, got)
		}
	}
	_, htmlBody, _ = EmailTemplate(pair, false, base)
	if !strings.Contains(htmlBody, `<a href="`+base+`/patrol">Open these alerts in Pulse</a>`) {
		t.Fatalf("grouped findings email footer does not link to Patrol:\n%s", htmlBody)
	}
	mixed := []*alerts.Alert{finding, linkTestAlert("vm-101-cpu", "web-01")}
	if got := alertsPageURL(base, mixed, false); got != base+"/alerts/overview" {
		t.Fatalf("mixed batch page link = %q", got)
	}
}
