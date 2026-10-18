package notifications

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSecureWebhookDNSResolutionHonorsCancellation(t *testing.T) {
	// Keep the production lookup path, with only its DNS connection replaced.
	// This test is deliberately nonparallel while replacing DefaultResolver.
	original := net.DefaultResolver
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	net.DefaultResolver = &net.Resolver{PreferGo: true, StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-release:
				return nil, fmt.Errorf("synthetic DNS connection stopped")
			}
		}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := &NotificationManager{}
	client := manager.createSecureWebhookClient(time.Second)
	defer client.CloseIdleConnections()
	completed := make(chan error, 1)
	finished := false
	defer func() {
		close(release)
		cancel()
		if !finished {
			select {
			case <-completed:
			case <-time.After(2 * time.Second):
				t.Error("controlled lookup did not stop during cleanup")
			}
		}
		net.DefaultResolver = original
	}()
	go func() {
		connection, err := client.Transport.(*http.Transport).DialContext(ctx, "tcp", "notification-dns-control.invalid:443")
		if connection != nil {
			_ = connection.Close()
		}
		completed <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("the production lookup did not enter the controlled resolver")
	}
	cancel()
	select {
	case err := <-completed:
		finished = true
		require.Error(t, err)
		require.Equal(t, NotificationFailureConnectivity, ClassifyNotificationFailureError(err))
	case <-time.After(500 * time.Millisecond):
		t.Fatal("webhook DNS resolution continued after dial cancellation")
	}
}

func TestWebhookDNSAdmissionBudgetAndCancellation(t *testing.T) {
	t.Run("public validation has a bounded lookup", func(t *testing.T) {
		manager := &NotificationManager{}
		calls := 0
		manager.resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IP, error) {
			calls++
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			require.Positive(t, time.Until(deadline))
			require.LessOrEqual(t, time.Until(deadline), webhookDNSLookupTimeout)
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		}
		require.NoError(t, manager.ValidateWebhookURL("https://notification-dns-control.invalid/"))
		require.Equal(t, 1, calls)
	})
	t.Run("redirect validation keeps caller cancellation", func(t *testing.T) {
		manager := &NotificationManager{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		manager.resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IP, error) {
			require.ErrorIs(t, ctx.Err(), context.Canceled)
			return nil, ctx.Err()
		}
		err := manager.validateWebhookURLWithContext(ctx, "https://notification-dns-control.invalid/", RedactWebhookURLSecrets)
		require.ErrorIs(t, err, context.Canceled)
	})
	t.Run("empty DNS result fails admission", func(t *testing.T) {
		manager := &NotificationManager{resolveWebhookIPs: func(context.Context, string) ([]net.IP, error) { return nil, nil }}
		err := manager.ValidateWebhookURL("https://notification-dns-control.invalid/")
		require.Error(t, err)
		require.Equal(t, NotificationFailureConnectivity, ClassifyNotificationFailureError(err))
	})
	t.Run("validation still rejects private DNS destinations", func(t *testing.T) {
		manager := &NotificationManager{resolveWebhookIPs: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("127.0.0.1")}, nil
		}}
		require.ErrorContains(t, manager.ValidateWebhookURL("https://notification-dns-control.invalid/"), "private IP")
	})
}
