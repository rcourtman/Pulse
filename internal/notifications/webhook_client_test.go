package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// createTestNotificationManager creates a NotificationManager with localhost allowlisted
// for testing purposes (since httptest servers run on localhost).
func createTestNotificationManager(t *testing.T) *NotificationManager {
	t.Helper()
	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	// Allowlist localhost for test servers
	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.0/8"); err != nil {
		t.Fatalf("failed to allowlist localhost: %v", err)
	}
	return nm
}

// TestSecureWebhookClientRedirectLimit verifies that the client stops following
// redirects after exceeding WebhookMaxRedirects.
func TestSecureWebhookClientRedirectLimit(t *testing.T) {
	redirectCount := 0
	// Create a server that always redirects to itself
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		http.Redirect(w, r, r.URL.String()+"x", http.StatusFound)
	}))
	defer server.Close()

	nm := createTestNotificationManager(t)

	client := nm.createSecureWebhookClient(WebhookTimeout)
	_, err := client.Get(server.URL)

	if err == nil {
		t.Fatal("expected error from too many redirects, got nil")
	}
	if !strings.Contains(err.Error(), "stopped after") {
		t.Errorf("expected 'stopped after' error, got: %v", err)
	}
	// CheckRedirect is called when len(via) >= WebhookMaxRedirects.
	// via includes previous requests, so with WebhookMaxRedirects=3:
	// - Initial request (not in via yet)
	// - Redirect 1: via has 1 entry (initial) - allowed
	// - Redirect 2: via has 2 entries - allowed
	// - Redirect 3: via has 3 entries - blocked (len(via) >= 3)
	// Total server hits: initial + 2 successful redirects = 3
	expectedRequests := WebhookMaxRedirects
	if redirectCount != expectedRequests {
		t.Errorf("expected %d requests, got %d", expectedRequests, redirectCount)
	}
}

// TestSecureWebhookClientBlocksUnsafeRedirect verifies that redirects to
// localhost/private networks are blocked when not in allowlist.
func TestSecureWebhookClientBlocksUnsafeRedirect(t *testing.T) {
	// Create a server that redirects to localhost (allowlist 127.0.0.1 only, not 127.0.0.2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect to a different localhost IP that's not in our allowlist
		http.Redirect(w, r, "http://127.0.0.2:8080/evil", http.StatusFound)
	}))
	defer server.Close()

	// Create manager with narrow allowlist (only the test server's IP)
	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	// Allowlist only 127.0.0.1/32, so 127.0.0.2 will be blocked
	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("failed to set allowlist: %v", err)
	}

	client := nm.createSecureWebhookClient(WebhookTimeout)
	_, err := client.Get(server.URL)

	if err == nil {
		t.Fatal("expected error from unsafe redirect, got nil")
	}
	if !strings.Contains(err.Error(), "private IP") {
		t.Errorf("expected private IP validation error, got: %v", err)
	}
}

// TestSecureWebhookClientBlocksPrivateNetworkRedirect verifies that redirects to
// private network IPs (10.x, 192.168.x, 172.16-31.x) are blocked.
func TestSecureWebhookClientBlocksPrivateNetworkRedirect(t *testing.T) {
	privateIPs := []string{
		"http://10.0.0.1/webhook",
		"http://192.168.1.1/webhook",
		"http://172.16.0.1/webhook",
	}

	for _, privateURL := range privateIPs {
		t.Run(privateURL, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, privateURL, http.StatusFound)
			}))
			defer server.Close()

			// Allowlist localhost for test server, but not private network ranges
			nm := &NotificationManager{
				lastNotified:      make(map[string]notificationRecord),
				webhookRateLimits: make(map[string]*webhookRateLimit),
			}
			if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.0/8"); err != nil {
				t.Fatalf("failed to set allowlist: %v", err)
			}

			client := nm.createSecureWebhookClient(WebhookTimeout)
			_, err := client.Get(server.URL)

			if err == nil {
				t.Fatalf("expected error from redirect to %s, got nil", privateURL)
			}
			if !strings.Contains(err.Error(), "private IP") {
				t.Errorf("expected private IP validation error for %s, got: %v", privateURL, err)
			}
		})
	}
}

// TestSecureWebhookClientAllowsValidRedirects verifies that valid redirects
// to safe URLs are followed successfully.
func TestSecureWebhookClientAllowsValidRedirects(t *testing.T) {
	// Create a chain of servers for redirect testing
	// Final server returns 200 OK
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer finalServer.Close()

	// Middle server redirects to final
	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusFound)
	}))
	defer middleServer.Close()

	// First server redirects to middle
	firstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, middleServer.URL, http.StatusFound)
	}))
	defer firstServer.Close()

	nm := createTestNotificationManager(t)

	client := nm.createSecureWebhookClient(WebhookTimeout)
	resp, err := client.Get(firstServer.URL)

	if err != nil {
		t.Fatalf("expected successful redirect chain, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestSecureWebhookClientBlocksLinkLocalRedirect verifies that redirects to
// link-local addresses (169.254.x.x) are blocked.
func TestSecureWebhookClientBlocksLinkLocalRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer server.Close()

	nm := createTestNotificationManager(t)

	client := nm.createSecureWebhookClient(WebhookTimeout)
	_, err := client.Get(server.URL)

	if err == nil {
		t.Fatal("expected error from redirect to link-local/metadata address, got nil")
	}
	if !strings.Contains(err.Error(), "link-local") {
		t.Errorf("expected link-local validation error, got: %v", err)
	}
}

func TestSecureWebhookClientDialContextBlocksPrivateIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	client := nm.createSecureWebhookClient(WebhookTimeout)

	_, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("expected error from blocked private IP, got nil")
	}
	if !strings.Contains(err.Error(), "blocked private IP") {
		t.Fatalf("expected blocked private IP error, got: %v", err)
	}
}

func TestSecureWebhookClientDialContextBlocksHostnameWithoutAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Rewrite URL to use hostname to exercise DNS resolution path.
	hostedURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)

	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	client := nm.createSecureWebhookClient(WebhookTimeout)

	_, err := client.Get(hostedURL)
	if err == nil {
		t.Fatal("expected error from blocked hostname, got nil")
	}
	if !strings.Contains(err.Error(), "resolves to blocked private IPs") {
		t.Fatalf("expected blocked hostname error, got: %v", err)
	}
}

func TestSecureWebhookClientDialContextAllowsHostnameWithAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Rewrite URL to use hostname to exercise DNS resolution path.
	hostedURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)

	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("failed to set allowlist: %v", err)
	}

	client := nm.createSecureWebhookClient(WebhookTimeout)
	resp, err := client.Get(hostedURL)
	if err != nil {
		t.Fatalf("expected request to succeed with allowlist, got: %v", err)
	}
	resp.Body.Close()
}

func TestSecureWebhookClientDialContextAllowsPublicIP(t *testing.T) {
	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	client := nm.createSecureWebhookClient(WebhookTimeout)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected transport to be *http.Transport")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use TEST-NET-3 address to exercise non-private IP branch.
	_, err := transport.DialContext(ctx, "tcp", "203.0.113.1:80")
	if err == nil {
		t.Fatalf("expected dial to fail due to canceled context")
	}
}

func TestSecureWebhookClientDialContextRejectsBadAddress(t *testing.T) {
	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	client := nm.createSecureWebhookClient(WebhookTimeout)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected transport to be *http.Transport")
	}

	_, err := transport.DialContext(context.Background(), "tcp", "badaddress")
	if err == nil {
		t.Fatalf("expected dial to fail for invalid address")
	}
}

func TestSecureWebhookClientDialContextLookupFailure(t *testing.T) {
	nm := &NotificationManager{
		lastNotified:      make(map[string]notificationRecord),
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}
	client := nm.createSecureWebhookClient(WebhookTimeout)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected transport to be *http.Transport")
	}

	_, err := transport.DialContext(context.Background(), "tcp", "bad host:80")
	if err == nil {
		t.Fatalf("expected lookup failure for invalid hostname")
	}
}
