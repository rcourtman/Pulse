package securityutil

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func pinnedTestConnection(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	connection, peer := net.Pipe()
	t.Cleanup(func() {
		_ = connection.Close()
		_ = peer.Close()
	})
	return connection, peer
}

func TestDialPinnedIPsFallbackPrefersOtherFamily(t *testing.T) {
	winner, _ := pinnedTestConnection(t)
	started := make(chan string, 3)
	stopped := make(chan struct{})
	ips := []net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2"), net.ParseIP("192.0.2.1")}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		started <- address
		if address == "192.0.2.1:443" {
			return winner, nil
		}
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}
	connection, err := dialPinnedIPs(context.Background(), "tcp", "443", ips, time.Second, dial)
	require.NoError(t, err)
	require.Same(t, winner, connection)
	require.Equal(t, "[2001:db8::1]:443", <-started)
	require.Equal(t, "192.0.2.1:443", <-started)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("winning connection did not cancel the stalled attempt")
	}
}

func TestDialPinnedIPsSharesBudgetAndBoundsConcurrency(t *testing.T) {
	winner, _ := pinnedTestConnection(t)
	var active, peak atomic.Int32
	var attemptsMu sync.Mutex
	var attempts []string
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); current > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, current) {
				break
			}
		}
		attemptsMu.Lock()
		attempts = append(attempts, address)
		attemptsMu.Unlock()
		if address == "192.0.2.3:443" {
			return winner, nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ips := []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2"), net.ParseIP("192.0.2.3"), net.ParseIP("192.0.2.4")}
	connection, err := dialPinnedIPs(context.Background(), "tcp", "443", ips, 1500*time.Millisecond, dial)
	require.NoError(t, err, "two stalled addresses must leave budget for the third")
	require.Same(t, winner, connection)
	require.LessOrEqual(t, peak.Load(), int32(2), "only two TCP attempts may overlap")
	attemptsMu.Lock()
	defer attemptsMu.Unlock()
	require.Contains(t, attempts, "192.0.2.3:443")
}

func TestDialPinnedIPsUnevenFamiliesKeepResolverOrder(t *testing.T) {
	winner, _ := pinnedTestConnection(t)
	var mu sync.Mutex
	var alternateAttempts []string
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "[2001:db8::1]:443" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		mu.Lock()
		alternateAttempts = append(alternateAttempts, address)
		mu.Unlock()
		if address == "192.0.2.3:443" {
			return winner, nil
		}
		return nil, errors.New("connection refused")
	}
	ips := []net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2"), net.ParseIP("192.0.2.3")}
	connection, err := dialPinnedIPs(context.Background(), "tcp", "443", ips, time.Second, dial)
	require.NoError(t, err)
	require.Same(t, winner, connection)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"192.0.2.1:443", "192.0.2.2:443", "192.0.2.3:443"}, alternateAttempts)
}

func TestDialPinnedIPsImmediateFailureStartsFallback(t *testing.T) {
	winner, _ := pinnedTestConnection(t)
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "192.0.2.1:443" {
			return nil, errors.New("connection refused")
		}
		return winner, nil
	}
	// A shorter budget than the stagger proves an exhausted primary does not
	// wait for the fallback timer. No elapsed-time threshold is needed.
	connection, err := dialPinnedIPs(context.Background(), "tcp", "443", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}, 100*time.Millisecond, dial)
	require.NoError(t, err)
	require.Same(t, winner, connection)
}

func TestDialPinnedIPsShortCallerDeadlineStillAllowsFallback(t *testing.T) {
	winner, _ := pinnedTestConnection(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "192.0.2.2:443" {
			return winner, nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	connection, err := dialPinnedIPs(ctx, "tcp", "443", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}, time.Second, dial)
	require.NoError(t, err)
	require.Same(t, winner, connection)
}

func TestDialPinnedIPsCancellationStopsBothWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 2)
	stopped := make(chan struct{}, 2)
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
		return nil, ctx.Err()
	}
	completed := make(chan error, 1)
	go func() {
		_, err := dialPinnedIPs(ctx, "tcp", "443", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}, 3*time.Second, dial)
		completed <- err
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("both workers must start before cancellation")
		}
	}
	cancel()
	require.ErrorIs(t, <-completed, context.Canceled)
	for i := 0; i < 2; i++ {
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("cancellation left a dial running")
		}
	}
}

type pinnedCloseObservedConn struct {
	net.Conn
	closed chan struct{}
}

func (c *pinnedCloseObservedConn) Close() error {
	err := c.Conn.Close()
	close(c.closed)
	return err
}

func TestDialPinnedIPsClosesLateSuccessfulLoser(t *testing.T) {
	for _, alreadyClosed := range []bool{false, true} {
		name := "read-immediately"
		if alreadyClosed {
			name = "already-closed"
		}
		t.Run(name, func(t *testing.T) {
			winner, winnerPeer := pinnedTestConnection(t)
			loser, peer := pinnedTestConnection(t)
			observed := &pinnedCloseObservedConn{Conn: loser, closed: make(chan struct{})}
			// net.Pipe refuses deadline changes once either endpoint closes.
			// Arm the bound before cancellation can correctly close the loser.
			require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
			started := make(chan struct{})
			dial := func(ctx context.Context, network, address string) (net.Conn, error) {
				if address == "192.0.2.2:443" {
					<-started
					return winner, nil
				}
				close(started)
				// Model a connect that completes just as its cancellation arrives.
				<-ctx.Done()
				return observed, nil
			}
			connection, err := dialPinnedIPs(context.Background(), "tcp", "443", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}, time.Second, dial)
			require.NoError(t, err)
			require.Same(t, winner, connection)
			if alreadyClosed {
				// Make the formerly failing ordering deterministic; a closed
				// loser is the desired result, not a failed observation setup.
				select {
				case <-observed.closed:
				case <-time.After(time.Second):
					t.Fatal("the unused successful connection did not close")
				}
			}
			_, err = peer.Read(make([]byte, 1))
			require.ErrorIs(t, err, io.EOF, "the unused successful connection must close")

			// Cancellation owns only the unused connection, not the winner.
			require.NoError(t, winner.SetDeadline(time.Now().Add(time.Second)))
			written := make(chan error, 1)
			go func() { _, err := winnerPeer.Write([]byte{42}); written <- err }()
			buf := make([]byte, 1)
			_, err = connection.Read(buf)
			require.NoError(t, err)
			require.Equal(t, byte(42), buf[0])
			require.NoError(t, <-written)
		})
	}
}

func TestDialPinnedIPsValidatesAndNormalizesCandidates(t *testing.T) {
	refused := errors.New("connection refused")
	for _, test := range []struct {
		name    string
		network string
		port    string
		ips     []net.IP
		want    []string
	}{
		{name: "empty", network: "tcp", port: "443"},
		{name: "nil", network: "tcp", port: "443", ips: []net.IP{nil}},
		{name: "malformed", network: "tcp", port: "443", ips: []net.IP{{1, 2, 3}}},
		{name: "invalid after valid", network: "tcp", port: "443", ips: []net.IP{net.ParseIP("192.0.2.1"), nil}},
		{name: "network", network: "udp", port: "443", ips: []net.IP{net.ParseIP("192.0.2.1")}},
		{name: "named port", network: "tcp", port: "https", ips: []net.IP{net.ParseIP("192.0.2.1")}},
		{name: "wrong family", network: "tcp6", port: "443", ips: []net.IP{net.ParseIP("192.0.2.1")}},
		{name: "deduplicate mapped IPv4", network: "tcp4", port: "443", ips: []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("::ffff:192.0.2.1"), net.ParseIP("2001:db8::1")}, want: []string{"192.0.2.1:443"}},
		{name: "IPv6 filter", network: "tcp6", port: "443", ips: []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("2001:db8::1")}, want: []string{"[2001:db8::1]:443"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var attempted []string
			dial := func(ctx context.Context, network, address string) (net.Conn, error) {
				attempted = append(attempted, address)
				return nil, refused
			}
			connection, err := dialPinnedIPs(context.Background(), test.network, test.port, test.ips, time.Second, dial)
			require.Error(t, err)
			require.Nil(t, connection)
			require.Equal(t, test.want, attempted)
			if len(test.want) > 0 {
				require.ErrorIs(t, err, refused, "retain the preferred address's network error")
			}
		})
	}
}
