//go:build linux

package notifications

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	pkgsecurityutil "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
	"github.com/stretchr/testify/require"
)

// Fill an owned loopback listener's deliberately small accept queue. Further
// connects then stall in the kernel, without firewall changes, public traffic,
// privileged network configuration, or a replacement production dial function.
func notificationStalledAddress(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Close(fd) })
	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 2}}))
	require.NoError(t, syscall.Listen(fd, 1))
	address, err := syscall.Getsockname(fd)
	require.NoError(t, err)
	port := strconv.Itoa(address.(*syscall.SockaddrInet4).Port)

	healthy, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", port))
	require.NoError(t, err)
	var accepted atomic.Int32
	server := &httptest.Server{Listener: healthy, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accepted.Add(1)
		_, _ = io.WriteString(w, "accepted")
	})}}
	server.Start()
	t.Cleanup(server.Close)
	control := server.Client()
	control.Timeout = time.Second
	response, err := control.Get(server.URL)
	require.NoError(t, err, "the second address must independently accept HTTP before the fallback test")
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	control.CloseIdleConnections()
	require.Equal(t, "accepted", string(body))
	accepted.Store(0)

	var fillers []net.Conn
	t.Cleanup(func() {
		for _, connection := range fillers {
			_ = connection.Close()
		}
	})
	stalled := false
	for attempt := 0; attempt < 8; attempt++ {
		connection, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.2", port), 250*time.Millisecond)
		if err != nil {
			var networkError net.Error
			require.ErrorAs(t, err, &networkError)
			require.True(t, networkError.Timeout(), "the fixture must stall, rather than refuse a connection")
			stalled = true
			break
		}
		fillers = append(fillers, connection)
	}
	require.True(t, stalled, "failed to fill the owned listener's accept queue")
	require.NotEmpty(t, fillers, "the listener must first accept real TCP connections")
	return port, &accepted
}

func TestQueuedWebhookFallbackVerdictSurvivesRestart(t *testing.T) {
	port, accepted := notificationStalledAddress(t)
	dir := t.TempDir()
	manager := NewNotificationManagerWithDeferredQueue("", dir)
	t.Cleanup(manager.Stop)
	require.NoError(t, manager.UpdateAllowedPrivateCIDRs("127.0.0.0/8,::1/128"))
	manager.resolveWebhookIPs = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.1")}, nil
	}
	manager.AddWebhook(WebhookConfig{ID: "connectivity", Name: "synthetic receiver", Enabled: true,
		Service: "generic", Method: "POST", URL: "http://localhost:" + port + "/"})
	manager.SetGroupingConfig(false, 0, false, false)
	manager.SendAlert(&alerts.Alert{ID: "fallback-test", ResourceName: "synthetic-node", Type: "cpu",
		Level: alerts.AlertLevelWarning, Message: "synthetic alert", StartTime: time.Now()})
	pending, err := manager.queue.GetPending(10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	id := pending[0].ID
	manager.StartQueueProcessing()
	waitVerdictAudit(t, manager.queue, id)
	manager.Stop()

	reopened := NewNotificationManagerWithDeferredQueue("", dir)
	t.Cleanup(reopened.Stop)
	var status, failureClass string
	var attempts, success int
	require.NoError(t, reopened.queue.db.QueryRow(`SELECT status, attempts FROM notification_queue WHERE id = ?`, id).Scan(&status, &attempts))
	require.NoError(t, reopened.queue.db.QueryRow(`SELECT success, failure_class FROM notification_audit WHERE notification_id = ?`, id).Scan(&success, &failureClass))
	stats, err := reopened.GetTelemetryStats(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, string(QueueStatusSent), status)
	require.Equal(t, 1, attempts)
	require.Equal(t, 1, success)
	require.Empty(t, failureClass)
	require.Equal(t, 1, stats.Attempts)
	require.Equal(t, 1, stats.Deliveries)
	require.Zero(t, stats.Failures)
	require.Equal(t, int32(1), accepted.Load(), "fallback must not replay the HTTP request")
	require.True(t, reopened.DeliveryHealth().Healthy)
}

func TestPinnedNotificationConnectivityReachesHealthyAddressAfterStall(t *testing.T) {
	for _, transport := range []string{"webhook", "shared-restricted-client"} {
		t.Run(transport, func(t *testing.T) {
			port, accepted := notificationStalledAddress(t)
			ips := []net.IP{net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.1")}
			var client *http.Client
			if transport == "webhook" {
				manager := &NotificationManager{}
				require.NoError(t, manager.UpdateAllowedPrivateCIDRs("127.0.0.0/8"))
				manager.resolveWebhookIPs = func(context.Context, string) ([]net.IP, error) { return ips, nil }
				client = manager.createSecureWebhookClient(2 * time.Second)
			} else {
				client = pkgsecurityutil.NewRestrictedOutboundHTTPClient(2*time.Second, pkgsecurityutil.RestrictedOutboundHTTPOptions{
					AllowPrivateIPs: true,
					AllowLoopback:   true,
					ResolveIPAddrs: func(context.Context, string) ([]net.IPAddr, error) {
						return []net.IPAddr{{IP: ips[0]}, {IP: ips[1]}}, nil
					},
				})
			}
			t.Cleanup(client.CloseIdleConnections)
			response, err := client.Get("http://localhost:" + port + "/")
			if response != nil {
				defer response.Body.Close()
			}
			require.NoError(t, err, "a stalled approved address must leave time to reach the healthy approved address")
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, "accepted", string(body))
			require.Equal(t, int32(1), accepted.Load(), "only one HTTP request may reach the receiver")
		})
	}
}
