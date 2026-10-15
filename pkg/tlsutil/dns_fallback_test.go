package tlsutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fallbackDNSResolver struct {
	answers map[string][]string
	queries atomic.Int32
}

func (r *fallbackDNSResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	r.queries.Add(1)
	if answer, ok := r.answers[host]; ok {
		return answer, nil
	}
	return nil, &net.DNSError{Err: "no fixture answer", Name: host, IsNotFound: true}
}

func (r *fallbackDNSResolver) LookupAddr(_ context.Context, addr string) ([]string, error) {
	return nil, &net.DNSError{Err: "no fixture reverse answer", Name: addr, IsNotFound: true}
}

func useFallbackDNS(t *testing.T, answers map[string][]string) *fallbackDNSResolver {
	t.Helper()
	resolver := GetDNSResolver()
	previous := resolver.Resolver
	fixture := &fallbackDNSResolver{answers: answers}
	resolver.Resolver = fixture
	t.Cleanup(func() { resolver.Resolver = previous })
	return fixture
}

func TestDialContextWithCacheFallback(t *testing.T) {
	for _, tc := range []struct {
		name, network string
		answers       []string
	}{
		{"refused-ipv4", "tcp", []string{"127.0.0.2", "127.0.0.1"}},
		{"refused-ipv6", "tcp", []string{"::1", "127.0.0.1"}},
		{"tcp4-family", "tcp4", []string{"::1", "127.0.0.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			accepted := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err == nil {
					err = conn.Close()
				}
				accepted <- err
			}()
			host := "cached-fallback-" + tc.name + ".invalid"
			dns := useFallbackDNS(t, map[string][]string{host: tc.answers})
			_, port, _ := net.SplitHostPort(listener.Addr().String())
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			for read := 0; read < 2; read++ {
				conn, err := DialContextWithCache(ctx, tc.network, net.JoinHostPort(host, port))
				if err != nil {
					listener.Close()
					<-accepted
					t.Fatalf("reachable cached alternative was not tried: %v", err)
				}
				if !strings.HasPrefix(conn.RemoteAddr().String(), "127.0.0.1:") {
					t.Errorf("dial reached unexpected address: %s", conn.RemoteAddr())
				}
				conn.Close()
				if read == 0 {
					if err := <-accepted; err != nil {
						t.Fatal(err)
					}
					go func() {
						conn, err := listener.Accept()
						if err == nil {
							err = conn.Close()
						}
						accepted <- err
					}()
				}
			}
			if err := <-accepted; err != nil {
				t.Fatal(err)
			}
			if dns.queries.Load() != 1 {
				t.Fatalf("two dials repeated DNS: %d queries", dns.queries.Load())
			}
			t.Log("two connections reached the second cached answer, one resolver query")
		})
	}
}

func TestCachedFallbackKeepsTLSTrust(t *testing.T) {
	for _, pinMatches := range []bool{true, false} {
		name := "rejected-pin"
		if pinMatches {
			name = "accepted-pin"
		}
		t.Run(name, func(t *testing.T) {
			host := "cached-tls-" + name + ".invalid"
			var authenticated atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.TLS.ServerName != host {
					t.Errorf("SNI changed from hostname to dialled IP: %q", r.TLS.ServerName)
				}
				if r.Header.Get("Authorization") != "fixture-token" {
					t.Error("ordinary fixture header lost")
				}
				authenticated.Add(1)
				io.WriteString(w, `{"data":[{"node":"cached-node","status":"online"}]}`)
			}))
			defer server.Close()
			_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
			dns := useFallbackDNS(t, map[string][]string{host: {"127.0.0.2", "127.0.0.1"}})
			digest := sha256.Sum256(server.Certificate().Raw)
			pin := hex.EncodeToString(digest[:])
			if !pinMatches {
				pin = strings.Repeat("0", 64)
			}
			client := CreateHTTPClientWithTimeout(true, pin, 2*time.Second)
			// The fixture hostname is private evidence, not a proxy request.
			client.Transport.(*http.Transport).Proxy = nil
			defer client.CloseIdleConnections()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+net.JoinHostPort(host, port)+"/api2/json/nodes", nil)
			req.Header.Set("Authorization", "fixture-token")
			resp, err := client.Do(req)
			if !pinMatches {
				if resp != nil {
					resp.Body.Close()
				}
				var mismatch *FingerprintMismatchError
				if !errors.As(err, &mismatch) || authenticated.Load() != 0 {
					t.Fatalf("alternative bypassed pin or never checked it: requests=%d err=%v", authenticated.Load(), err)
				}
			} else {
				if err != nil {
					t.Fatalf("ordinary pinned read could not reach healthy alternative: %v", err)
				}
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil || !strings.Contains(string(body), `"node":"cached-node"`) || authenticated.Load() != 1 {
					t.Fatalf("ordinary fixture read changed: requests=%d err=%v body=%s", authenticated.Load(), err, body)
				}
			}
			if dns.queries.Load() != 1 {
				t.Errorf("fallback re-resolved hostname: queries=%d", dns.queries.Load())
			}
			t.Logf("matching pin=%t; authenticated requests=%d; one cached hostname resolution", pinMatches, authenticated.Load())
		})
	}
}

func TestDialCachedAddressesBudgetAndStop(t *testing.T) {
	t.Run("stalled-first-reserves-caller-budget", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		accepted := make(chan error, 1)
		go func() {
			conn, err := listener.Accept()
			if err == nil {
				err = conn.Close()
			}
			accepted <- err
		}()
		_, port, _ := net.SplitHostPort(listener.Addr().String())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		calls := 0
		dial := func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
			calls++
			if calls == 1 {
				if timeout <= 0 || timeout > time.Second {
					t.Errorf("first address consumed caller budget: %s", timeout)
				}
				timer := time.NewTimer(timeout)
				defer timer.Stop()
				select {
				case <-timer.C:
					return nil, context.DeadlineExceeded
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, address)
		}
		conn, err := dialCachedAddresses(ctx, "tcp4", "budget.invalid", port, []string{"127.0.0.2", "127.0.0.1"}, dial)
		if err != nil {
			listener.Close()
			<-accepted
			t.Fatal(err)
		}
		conn.Close()
		if err := <-accepted; err != nil {
			t.Fatal(err)
		}
		if calls != 2 || ctx.Err() != nil {
			t.Fatalf("later address lost its share: calls=%d err=%v", calls, ctx.Err())
		}
		t.Log("timed-out first attempt left enough of the original two-second budget for an actual second socket")
	})
	t.Run("ten-second-cap-and-first-error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		firstErr := errors.New("first socket failed")
		calls := 0
		conn, err := dialCachedAddresses(ctx, "tcp", "cap.invalid", "443", []string{"127.0.0.2", "127.0.0.1"}, func(_ context.Context, _, _ string, timeout time.Duration) (net.Conn, error) {
			calls++
			limit := 10 * time.Second
			if calls == 1 {
				limit = 5 * time.Second
			}
			if timeout <= 0 || timeout > limit {
				t.Errorf("attempt %d timeout %s exceeds %s", calls, timeout, limit)
			}
			if calls == 1 {
				return nil, firstErr
			}
			return nil, errors.New("second socket failed")
		})
		if conn != nil || !errors.Is(err, firstErr) || calls != 2 {
			t.Fatalf("all-failed result: calls=%d conn=%v err=%v", calls, conn, err)
		}
	})
	t.Run("cancelled-first-stops-alternatives", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		conn, err := dialCachedAddresses(ctx, "tcp", "cancel.invalid", "443", []string{"127.0.0.2", "127.0.0.1"}, func(_ context.Context, _, _ string, _ time.Duration) (net.Conn, error) {
			calls++
			cancel()
			return nil, errors.New("socket stopped")
		})
		if conn != nil || !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("cancelled caller started another socket: calls=%d conn=%v err=%v", calls, conn, err)
		}
	})
	t.Run("late-success-after-cancellation-is-closed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		local, peer := net.Pipe()
		defer peer.Close()
		conn, err := dialCachedAddresses(ctx, "tcp", "late.invalid", "443", []string{"127.0.0.1"}, func(_ context.Context, _, _ string, _ time.Duration) (net.Conn, error) {
			cancel()
			return local, nil
		})
		if conn != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled caller received a connection: %v, %v", conn, err)
		}
		peer.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := peer.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
			t.Fatalf("cancelled late-success socket stayed open: %v", err)
		}
	})
}

func TestDialCachedAddressesFamilyAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, network string
		ips, want     []string
	}{
		{"tcp4", "tcp4", []string{"bad.invalid", "::1", "::ffff:127.0.0.1", "127.0.0.1", "127.0.0.2"}, []string{"127.0.0.1:8006", "127.0.0.2:8006"}},
		{"tcp6", "tcp6", []string{"127.0.0.1", "::ffff:127.0.0.1", "::1", "0:0:0:0:0:0:0:1", "fe80::1%FixtureZone"}, []string{"[::1]:8006", "[fe80::1%FixtureZone]:8006"}},
		{"udp4", "udp4", []string{"::1", "127.0.0.1"}, []string{"127.0.0.1:8006"}},
		{"no-usable-address", "tcp6", []string{"bad.invalid", "127.0.0.1"}, nil},
		{"empty-answer", "tcp", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attempted []string
			firstErr := errors.New("fixture socket failure")
			conn, err := dialCachedAddresses(context.Background(), tc.network, "family.invalid", "8006", tc.ips, func(_ context.Context, network, address string, _ time.Duration) (net.Conn, error) {
				if network != tc.network {
					t.Errorf("network changed: %s", network)
				}
				attempted = append(attempted, address)
				return nil, firstErr
			})
			if conn != nil || strings.Join(attempted, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("addresses tried=%v, want=%v", attempted, tc.want)
			}
			if len(tc.want) == 0 {
				var dnsErr *net.DNSError
				if !errors.As(err, &dnsErr) {
					t.Fatalf("no suitable answer was not a DNS failure: %v", err)
				}
			} else if !errors.Is(err, firstErr) {
				t.Fatalf("first socket error lost: %v", err)
			}
		})
	}
}

func TestDialContextWithCacheCancelledBeforeDNS(t *testing.T) {
	const host = "cached-pre-cancelled.invalid"
	dns := useFallbackDNS(t, map[string][]string{host: {"127.0.0.1"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := DialContextWithCache(ctx, "tcp", host+":443")
	if conn != nil || !errors.Is(err, context.Canceled) || dns.queries.Load() != 0 {
		t.Fatalf("cancelled caller queried DNS: conn=%v err=%v queries=%d", conn, err, dns.queries.Load())
	}
}
