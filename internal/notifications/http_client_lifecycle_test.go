package notifications

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A one-request transport has no future owner to reuse its idle sockets.
// Observe the real receiver's connection state, not a mocked Close call.
func notificationHTTPConnectionFixture(t *testing.T, status int) (*httptest.Server, *atomic.Int32, *atomic.Int32, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var connections, requests atomic.Int32
	closed := make(chan struct{}, 16)
	server := &httptest.Server{Listener: listener, Config: &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, "receiver response")
		}),
		ConnState: func(_ net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				connections.Add(1)
			case http.StateClosed:
				closed <- struct{}{}
			}
		},
	}}
	server.Start()
	t.Cleanup(server.Close)
	return server, &connections, &requests, closed
}

func awaitNotificationHTTPConnectionClose(t *testing.T, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("request-owned HTTP client kept its completed connection open")
	}
}

func TestRequestOwnedNotificationHTTPClientsReleaseConnections(t *testing.T) {
	for _, path := range []string{"apprise", "webhook-no-shared-client", "webhook-timeout-override"} {
		for _, status := range []int{http.StatusAccepted, http.StatusServiceUnavailable} {
			t.Run(fmt.Sprintf("%s/%d", path, status), func(t *testing.T) {
				server, connections, requests, closed := notificationHTTPConnectionFixture(t, status)
				manager := createTestNotificationManager(t)
				if path == "webhook-timeout-override" {
					manager.webhookClient = manager.createSecureWebhookClient(WebhookTimeout)
					defer manager.webhookClient.CloseIdleConnections()
				}
				for attempt := range 3 {
					var err error
					if path == "apprise" {
						err = manager.sendAppriseViaHTTP(AppriseConfig{ServerURL: server.URL, TimeoutSeconds: 5}, "test", "test body", "warning")
					} else {
						timeout := WebhookTimeout
						if path == "webhook-timeout-override" {
							timeout = WebhookTestTimeout
						}
						result, sendErr := manager.executeWebhookRequest(WebhookConfig{URL: server.URL}, []byte(`{}`), webhookRequestOptions{
							timeout: timeout, validateURL: true, eventID: "client-lifecycle:alert",
						})
						err = sendErr
						if result == nil || result.statusCode != status || result.responseIncomplete || result.responseBytes != int64(len("receiver response")) {
							t.Fatalf("response did not finish unchanged: %+v (%v)", result, err)
						}
					}
					if (err == nil) != (status == http.StatusAccepted) {
						t.Fatalf("delivery verdict changed: HTTP %d: %v", status, err)
					}
					if err != nil && ClassifyNotificationFailureError(err) != NotificationFailureServerError {
						t.Fatalf("delivery failure class changed: %v", err)
					}
					awaitNotificationHTTPConnectionClose(t, closed)
					if got := requests.Load(); got != int32(attempt+1) {
						t.Fatalf("receiver requests = %d, want %d (no retries)", got, attempt+1)
					}
				}
				if got := connections.Load(); got != 3 {
					t.Fatalf("one-off requests used %d connections, want 3", got)
				}
			})
		}
	}
}

// A timeout override owns its own transport, not the manager's reusable pool.
// Releasing that override must leave the ordinary delivery socket reusable.
func TestRequestOwnedNotificationHTTPClientPreservesSharedPool(t *testing.T) {
	server, connections, requests, closed := notificationHTTPConnectionFixture(t, http.StatusAccepted)
	manager := createTestNotificationManager(t)
	manager.webhookClient = manager.createSecureWebhookClient(WebhookTimeout)
	defer manager.webhookClient.CloseIdleConnections()
	send := func(timeout time.Duration) {
		t.Helper()
		result, err := manager.executeWebhookRequest(WebhookConfig{URL: server.URL}, []byte(`{}`), webhookRequestOptions{
			timeout: timeout, validateURL: true, eventID: "client-lifecycle:alert",
		})
		if err != nil || result == nil || result.statusCode != http.StatusAccepted {
			t.Fatalf("send failed: %+v, %v", result, err)
		}
	}
	send(WebhookTimeout)
	send(WebhookTestTimeout)
	awaitNotificationHTTPConnectionClose(t, closed)
	send(WebhookTimeout)
	if connections.Load() != 2 || requests.Load() != 3 {
		t.Fatalf("shared pool was not reused: connections=%d requests=%d; want 2 and 3", connections.Load(), requests.Load())
	}
	select {
	case <-closed:
		t.Fatal("one-off cleanup closed the shared delivery connection")
	default:
	}
	manager.webhookClient.CloseIdleConnections()
	awaitNotificationHTTPConnectionClose(t, closed)
}
