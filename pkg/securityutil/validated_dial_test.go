package securityutil

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestValidatedDialPinsOneCompleteDNSAnswer(t *testing.T) {
	calls := 0
	opts := RestrictedOutboundHTTPOptions{AllowPrivateIPs: true, ResolveIPAddrs: func(ctx context.Context, host string) ([]net.IPAddr, error) {
		calls++
		if host != "nas.example.test" {
			t.Errorf("resolved host %q", host)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("resolution has no bound")
		}
		if calls > 1 {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("10.1.2.3")}, {IP: net.ParseIP("fd00::4")}}, nil
	}}
	dials := 0
	dial := validatedOutboundDialer(opts, time.Second, func(ctx context.Context, network, port string, ips []net.IP, timeout time.Duration) (net.Conn, error) {
		dials++
		if network != "tcp" || port != "443" || timeout != time.Second {
			t.Errorf("dial changed parameters %s %s %s", network, port, timeout)
		}
		got := []string{}
		for _, ip := range ips {
			got = append(got, ip.String())
		}
		if !reflect.DeepEqual(got, []string{"10.1.2.3", "fd00::4"}) {
			t.Errorf("dialed unapproved answer: %v", got)
		}
		a, b := net.Pipe()
		b.Close()
		return a, nil
	})
	conn, err := dial(context.Background(), "tcp", "nas.example.test:443")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if calls != 1 || dials != 1 {
		t.Fatalf("resolve=%d,dial=%d", calls, dials)
	}
	if _, err := dial(context.Background(), "tcp", "nas.example.test:443"); err == nil {
		t.Fatal("rebound loopback answer allowed")
	}
	if calls != 2 || dials != 1 {
		t.Fatalf("rebound answer reached dialer: resolve=%d,dial=%d", calls, dials)
	}
}

func TestValidatedDialRefusesWholeMixedAnswer(t *testing.T) {
	for _, blocked := range []string{"127.0.0.1", "::1", "169.254.169.254", "fe80::1", "0.0.0.0", "224.0.0.1", "64:ff9b::7f00:1"} {
		t.Run(blocked, func(t *testing.T) {
			opts := RestrictedOutboundHTTPOptions{AllowPrivateIPs: true, ResolveIPAddrs: resolveTo("93.184.216.34", blocked)}
			dial := validatedOutboundDialer(opts, time.Second, func(context.Context, string, string, []net.IP, time.Duration) (net.Conn, error) {
				t.Fatal("mixed answer reached socket dial")
				return nil, nil
			})
			if _, err := dial(context.Background(), "tcp", "mixed.example.test:443"); err == nil {
				t.Fatal("mixed answer allowed")
			}
		})
	}
}

func TestValidatedDialKeepsCloudAndLANRules(t *testing.T) {
	for _, host := range []string{"10.1.2.3", "fd00::1", "93.184.216.34"} {
		for _, allowPrivate := range []bool{false, true} {
			t.Run(host+map[bool]string{true: "/LAN", false: "/public"}[allowPrivate], func(t *testing.T) {
				called := false
				dial := validatedOutboundDialer(RestrictedOutboundHTTPOptions{AllowPrivateIPs: allowPrivate}, time.Second, func(context.Context, string, string, []net.IP, time.Duration) (net.Conn, error) {
					called = true
					return nil, nil
				})
				_, err := dial(context.Background(), "tcp", net.JoinHostPort(host, "443"))
				want := allowPrivate || host == "93.184.216.34"
				if called != want || (err == nil) != want {
					t.Fatalf("allowed=%v err=%v,want %v", called, err, want)
				}
			})
		}
	}
}

func TestValidatedDialResolutionFailuresNeverReachSocket(t *testing.T) {
	for _, tc := range []struct {
		name    string
		resolve func(context.Context, string) ([]net.IPAddr, error)
	}{
		{"empty", resolveTo()},
		{"invalid", func(context.Context, string) ([]net.IPAddr, error) { return []net.IPAddr{{IP: nil}}, nil }},
		{"failure", func(context.Context, string) ([]net.IPAddr, error) { return nil, errors.New("fixture resolver failed") }},
		{"canceled", func(ctx context.Context, _ string) ([]net.IPAddr, error) { return nil, ctx.Err() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			dial := validatedOutboundDialer(RestrictedOutboundHTTPOptions{ResolveIPAddrs: tc.resolve}, time.Second, func(context.Context, string, string, []net.IP, time.Duration) (net.Conn, error) {
				t.Fatal("failed resolution dialed")
				return nil, nil
			})
			if _, err := dial(ctx, "tcp", "nas.example.test:443"); err == nil {
				t.Fatal("failed resolution allowed")
			}
		})
	}
}

func TestValidatedDialDoesNotMislabelSocketFailureOrCancellation(t *testing.T) {
	socketErr := errors.New("approved peer refused the connection")
	dial := validatedOutboundDialer(RestrictedOutboundHTTPOptions{ResolveIPAddrs: resolveTo("93.184.216.34")}, time.Second, func(context.Context, string, string, []net.IP, time.Duration) (net.Conn, error) {
		return nil, socketErr
	})
	_, err := dial(context.Background(), "tcp", "nas.example.test:443")
	if err != socketErr || errors.Is(err, ErrOutboundDestinationValidation) {
		t.Fatalf("socket failure mislabeled: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dial = validatedOutboundDialer(RestrictedOutboundHTTPOptions{ResolveIPAddrs: func(ctx context.Context, _ string) ([]net.IPAddr, error) { return nil, ctx.Err() }}, time.Second, func(context.Context, string, string, []net.IP, time.Duration) (net.Conn, error) {
		t.Fatal("canceled resolver dialed")
		return nil, nil
	})
	_, err = dial(ctx, "tcp", "nas.example.test:443")
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrOutboundDestinationValidation) {
		t.Fatalf("cancellation mislabeled: %v", err)
	}
}
