package notifications

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestProviderClientWebhookRoutingStaysRuntimeLocal(t *testing.T) {
	t.Parallel()

	clientAReceiver := newWebhookCapture()
	clientAServer := newIPv4HTTPServer(t, clientAReceiver)
	defer clientAServer.Close()

	clientBReceiver := newWebhookCapture()
	clientBServer := newIPv4HTTPServer(t, clientBReceiver)
	defer clientBServer.Close()

	clientA := NewNotificationManagerWithDataDir("https://client-a.example.test", t.TempDir())
	t.Cleanup(clientA.Stop)
	require.NoError(t, clientA.UpdateAllowedPrivateCIDRs("127.0.0.1"))

	clientB := NewNotificationManagerWithDataDir("https://client-b.example.test", t.TempDir())
	t.Cleanup(clientB.Stop)
	require.NoError(t, clientB.UpdateAllowedPrivateCIDRs("127.0.0.1"))

	alertA := providerClientAlert("alert-client-a", "client-a-node")
	alertB := providerClientAlert("alert-client-b", "client-b-node")

	err := clientA.SendEnhancedWebhook(EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name:    "client-a-gotify",
			URL:     clientAServer.URL + "/message?token=client-a-token",
			Enabled: true,
			Service: "gotify",
		},
		Service: "gotify",
	}, alertA)
	require.NoError(t, err)

	genericTemplate := webhookPayloadTemplateForTest(t, "generic")
	err = clientB.SendEnhancedWebhook(EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name:    "client-b-generic",
			URL:     clientBServer.URL + "/hook",
			Enabled: true,
			Service: "generic",
		},
		Service:         "generic",
		PayloadTemplate: genericTemplate,
	}, alertB)
	require.NoError(t, err)

	clientABodies := clientAReceiver.bodies()
	clientBBodies := clientBReceiver.bodies()
	require.Len(t, clientABodies, 1)
	require.Len(t, clientBBodies, 1)

	if !strings.Contains(clientABodies[0], "alert-client-a") || !strings.Contains(clientABodies[0], "client-a-node") {
		t.Fatalf("client A Gotify payload missing client A event: %s", clientABodies[0])
	}
	if strings.Contains(clientABodies[0], "alert-client-b") || strings.Contains(clientABodies[0], "client-b-node") {
		t.Fatalf("client A Gotify payload included client B event: %s", clientABodies[0])
	}

	if !strings.Contains(clientBBodies[0], "alert-client-b") || !strings.Contains(clientBBodies[0], "client-b-node") {
		t.Fatalf("client B generic payload missing client B event: %s", clientBBodies[0])
	}
	if strings.Contains(clientBBodies[0], "alert-client-a") || strings.Contains(clientBBodies[0], "client-a-node") {
		t.Fatalf("client B generic payload included client A event: %s", clientBBodies[0])
	}
}

func providerClientAlert(id, resource string) *alerts.Alert {
	return &alerts.Alert{
		ID:           id,
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceName: resource,
		Node:         resource,
		Message:      "CPU threshold exceeded for " + resource,
		Value:        96,
		Threshold:    90,
		StartTime:    time.Now().UTC().Add(-5 * time.Minute),
		Instance:     "https://" + resource + ".example.test",
	}
}

func webhookPayloadTemplateForTest(t *testing.T, service string) string {
	t.Helper()
	for _, tmpl := range GetWebhookTemplates() {
		if tmpl.Service == service {
			return tmpl.PayloadTemplate
		}
	}
	t.Fatalf("missing webhook template %q", service)
	return ""
}

type webhookCapture struct {
	mu     sync.Mutex
	values []string
}

func newWebhookCapture() *webhookCapture {
	return &webhookCapture{}
}

func (c *webhookCapture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.values = append(c.values, string(body))
	c.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (c *webhookCapture) bodies() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.values))
	copy(out, c.values)
	return out
}
