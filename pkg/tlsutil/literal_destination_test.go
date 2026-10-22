package tlsutil

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
)

// DNS is deliberately unavailable: numeric destinations are already resolved,
// and a scoped IPv6 literal must keep its interface identity outside DNS.
func TestDialContextWithCacheLiteralDestination(t *testing.T) {
	zone := literalLoopbackZone(t)
	for _, tc := range []struct{ name, network, address, host string }{
		{"ipv4", "tcp4", "127.0.0.1:0", "127.0.0.1"},
		{"scoped-ipv6", "tcp6", "[::1]:0", "::1%" + zone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen(tc.network, tc.address)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			dns := useFallbackDNS(t, nil)
			_, port, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			conn, err := DialContextWithCache(ctx, tc.network, net.JoinHostPort(tc.host, port))
			if err != nil {
				t.Errorf("literal socket depended on DNS: %v", err)
			}
			if conn != nil {
				conn.Close()
			}
			if got := dns.queries.Load(); got != 0 {
				t.Errorf("literal caused %d DNS lookups, want zero", got)
			}
		})
	}
}

func TestLookupDialAddressesPreservesLiteralScope(t *testing.T) {
	dns := useFallbackDNS(t, nil)
	for _, tc := range []struct{ input, want string }{
		{"127.0.0.1", "127.0.0.1"},
		{"::1", "::1"},
		{"2001:0DB8::1", "2001:db8::1"},
		{"fe80::1%InterfaceCase", "fe80::1%InterfaceCase"},
		{"::ffff:192.0.2.1", "::ffff:192.0.2.1"},
	} {
		got, err := lookupDialAddresses(context.Background(), tc.input)
		if err != nil || !reflect.DeepEqual(got, []string{tc.want}) {
			t.Errorf("literal %q addresses=%v err=%v, want %q", tc.input, got, err, tc.want)
		}
	}
	if got := dns.queries.Load(); got != 0 {
		t.Errorf("literals queried DNS %d times", got)
	}
}

func TestLookupDialAddressesKeepsHostnameCacheAndCancellation(t *testing.T) {
	const host = "literal-regression-hostname.invalid"
	want := []string{"127.0.0.2", "127.0.0.1"}
	dns := useFallbackDNS(t, map[string][]string{host: want})
	for i := 0; i < 2; i++ {
		got, err := lookupDialAddresses(context.Background(), host)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("named cache view changed: %v %v", got, err)
		}
	}
	if got := dns.queries.Load(); got != 1 {
		t.Errorf("two named lookups made %d DNS queries, want one", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, input := range []string{host, "fe80::1%InterfaceCase"} {
		got, err := lookupDialAddresses(ctx, input)
		if got != nil || !errors.Is(err, context.Canceled) {
			t.Errorf("cancelled lookup returned %v %v", got, err)
		}
	}
	if got := dns.queries.Load(); got != 1 {
		t.Errorf("cancelled lookup queried DNS: total %d", got)
	}
}

func literalLoopbackZone(t *testing.T) string {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			return iface.Name
		}
	}
	t.Fatal("no owned loopback interface for scoped IPv6 fixture")
	return ""
}
