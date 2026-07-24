package securityutil

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// This file raises branch coverage for the outbound SSRF helpers in
// outbound_http.go: resolveOutboundIPAddrs, resolvePermittedOutboundIPs, and
// cloneRestrictedTransport. No real DNS, daemon, or network is used: every
// hostname target is an IP literal (short-circuited before resolution) or is
// routed through an injected ResolveIPAddrs stub / the swapped
// resolveOutboundFetchIPs package variable. The package's tests do not use
// t.Parallel(), so temporarily swapping package-level variables and restoring
// them via t.Cleanup is safe.

func TestBranchcov0724pmResolveOutboundIPAddrs(t *testing.T) {
	t.Run("custom resolver invoked and args forwarded", func(t *testing.T) {
		ctx := context.Background()
		want := []net.IPAddr{{IP: net.ParseIP("203.0.113.9")}}
		var gotCtx context.Context
		var gotHost string
		got, err := resolveOutboundIPAddrs(ctx, "anything.test", RestrictedOutboundHTTPOptions{
			ResolveIPAddrs: func(c context.Context, h string) ([]net.IPAddr, error) {
				gotCtx, gotHost = c, h
				return want, nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotCtx != ctx {
			t.Fatalf("ctx not forwarded, got %v want %v", gotCtx, ctx)
		}
		if gotHost != "anything.test" {
			t.Fatalf("host = %q, want anything.test", gotHost)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("default resolver used when nil and args forwarded", func(t *testing.T) {
		orig := resolveOutboundFetchIPs
		var gotCtx context.Context
		var gotHost string
		resolveOutboundFetchIPs = func(c context.Context, h string) ([]net.IPAddr, error) {
			gotCtx, gotHost = c, h
			return []net.IPAddr{{IP: net.ParseIP("198.51.100.7")}}, nil
		}
		t.Cleanup(func() { resolveOutboundFetchIPs = orig })

		ctx := context.Background()
		got, err := resolveOutboundIPAddrs(ctx, "swap.example", RestrictedOutboundHTTPOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotCtx != ctx {
			t.Fatalf("ctx not forwarded to swapped default resolver, got %v want %v", gotCtx, ctx)
		}
		if gotHost != "swap.example" {
			t.Fatalf("host = %q, want swap.example", gotHost)
		}
		if len(got) != 1 || !got[0].IP.Equal(net.ParseIP("198.51.100.7")) {
			t.Fatalf("got %v, want [198.51.100.7]", got)
		}
	})

	t.Run("default resolver error propagated", func(t *testing.T) {
		orig := resolveOutboundFetchIPs
		resolveOutboundFetchIPs = func(context.Context, string) ([]net.IPAddr, error) {
			return nil, errors.New("no such host")
		}
		t.Cleanup(func() { resolveOutboundFetchIPs = orig })

		_, err := resolveOutboundIPAddrs(context.Background(), "missing.example", RestrictedOutboundHTTPOptions{})
		if err == nil || !strings.Contains(err.Error(), "no such host") {
			t.Fatalf("err = %v, want error containing no such host", err)
		}
	})
}

func TestBranchcov0724pmResolvePermittedOutboundIPs(t *testing.T) {
	strict := RestrictedOutboundHTTPOptions{} // no loopback/private
	permit := RestrictedOutboundHTTPOptions{AllowPrivateIPs: true, AllowLoopback: true}

	// Arms reached without any DNS: empty host, metadata hosts, and IP literals.
	tests := []struct {
		name    string
		host    string
		opts    RestrictedOutboundHTTPOptions
		wantErr string
		wantIPs []string
	}{
		{name: "empty host rejected", host: "  ", opts: permit, wantErr: "URL hostname is required"},
		{name: "metadata google internal rejected", host: "metadata.google.internal", opts: permit, wantErr: "metadata service host is not allowed"},
		{name: "metadata goog rejected", host: "metadata.goog", opts: permit, wantErr: "metadata service host is not allowed"},
		{name: "metadata host case insensitive", host: "Metadata.Goog", opts: permit, wantErr: "metadata service host is not allowed"},
		{name: "ip literal public permitted", host: "203.0.113.5", opts: strict, wantIPs: []string{"203.0.113.5"}},
		{name: "ip literal loopback blocked when disallowed", host: "127.0.0.1", opts: strict, wantErr: "loopback addresses are not allowed"},
		{name: "ip literal private blocked when disallowed", host: "10.0.0.1", opts: strict, wantErr: "private addresses are not allowed"},
		{name: "ip literal private permitted", host: "10.0.0.1", opts: permit, wantIPs: []string{"10.0.0.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePermittedOutboundIPs(context.Background(), tt.host, tt.opts)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("resolvePermittedOutboundIPs(%q) = %v, want error containing %q", tt.host, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolvePermittedOutboundIPs(%q) err = %q, want substring %q", tt.host, err.Error(), tt.wantErr)
				}
				if got != nil {
					t.Fatalf("resolvePermittedOutboundIPs(%q) returned non-nil %v with error", tt.host, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePermittedOutboundIPs(%q) unexpected error: %v", tt.host, err)
			}
			if len(got) != len(tt.wantIPs) {
				t.Fatalf("got %d IPs %v, want %d %v", len(got), got, len(tt.wantIPs), tt.wantIPs)
			}
			for i, want := range tt.wantIPs {
				if !got[i].Equal(net.ParseIP(want)) {
					t.Fatalf("got[%d] = %v, want %s", i, got[i], want)
				}
			}
		})
	}

	// DNS-dependent arms, driven entirely through an injected resolver so no
	// real network resolution is required.
	t.Run("resolve error wrapped", func(t *testing.T) {
		opts := permit
		opts.ResolveIPAddrs = func(context.Context, string) ([]net.IPAddr, error) {
			return nil, errors.New("lookup timeout")
		}
		_, err := resolvePermittedOutboundIPs(context.Background(), "host.example", opts)
		if err == nil || !strings.Contains(err.Error(), "failed to resolve hostname host.example") {
			t.Fatalf("err = %v, want failed to resolve hostname", err)
		}
		if !strings.Contains(err.Error(), "lookup timeout") {
			t.Fatalf("err = %v, want wrapped lookup timeout", err)
		}
	})

	t.Run("empty resolution rejected", func(t *testing.T) {
		opts := permit
		opts.ResolveIPAddrs = func(context.Context, string) ([]net.IPAddr, error) {
			return nil, nil
		}
		_, err := resolvePermittedOutboundIPs(context.Background(), "host.example", opts)
		if err == nil || !strings.Contains(err.Error(), "host.example did not resolve") {
			t.Fatalf("err = %v, want did not resolve", err)
		}
	})

	t.Run("mixed blocked and permitted returns permitted only", func(t *testing.T) {
		opts := strict // loopback blocked, public allowed
		opts.ResolveIPAddrs = func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.ParseIP("127.0.0.1")},   // blocked (loopback)
				{IP: net.ParseIP("203.0.113.5")}, // permitted (public)
				{IP: net.ParseIP("192.168.1.1")}, // blocked (private)
			}, nil
		}
		got, err := resolvePermittedOutboundIPs(context.Background(), "host.example", opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || !got[0].Equal(net.ParseIP("203.0.113.5")) {
			t.Fatalf("got %v, want only [203.0.113.5] (blocked ones filtered)", got)
		}
	})

	t.Run("only blocked addresses rejected", func(t *testing.T) {
		opts := strict
		opts.ResolveIPAddrs = func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.ParseIP("127.0.0.1")},   // blocked
				{IP: net.ParseIP("192.168.1.1")}, // blocked
			}, nil
		}
		_, err := resolvePermittedOutboundIPs(context.Background(), "host.example", opts)
		if err == nil || !strings.Contains(err.Error(), "resolves only to blocked addresses") {
			t.Fatalf("err = %v, want resolves only to blocked addresses", err)
		}
	})

	t.Run("all permitted returned in order", func(t *testing.T) {
		opts := strict
		opts.ResolveIPAddrs = func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.ParseIP("203.0.113.5")},
				{IP: net.ParseIP("198.51.100.7")},
			}, nil
		}
		got, err := resolvePermittedOutboundIPs(context.Background(), "host.example", opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || !got[0].Equal(net.ParseIP("203.0.113.5")) || !got[1].Equal(net.ParseIP("198.51.100.7")) {
			t.Fatalf("got %v, want both public addresses in order", got)
		}
	})

	t.Run("nil ctx falls back to background without panic", func(t *testing.T) {
		var seenCtx context.Context
		opts := permit
		opts.ResolveIPAddrs = func(c context.Context, _ string) ([]net.IPAddr, error) {
			seenCtx = c
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.5")}}, nil
		}
		got, err := resolvePermittedOutboundIPs(nil, "host.example", opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// context.WithTimeout panics on a nil parent, so the fact that the
		// resolver was invoked at all proves the nil-ctx -> Background fallback
		// arm ran.
		if seenCtx == nil {
			t.Fatal("resolver never invoked; nil ctx fallback arm did not run")
		}
		if len(got) != 1 || !got[0].Equal(net.ParseIP("10.0.0.5")) {
			t.Fatalf("got %v, want [10.0.0.5]", got)
		}
	})
}

func TestBranchcov0724pmCloneRestrictedTransport(t *testing.T) {
	t.Run("default transport cloned with tls12 floor and existing config re-cloned", func(t *testing.T) {
		opts := RestrictedOutboundHTTPOptions{}
		clone := cloneRestrictedTransport(opts)
		if clone == nil {
			t.Fatal("clone is nil")
		}
		if clone.TLSClientConfig == nil {
			t.Fatal("TLSClientConfig nil, want cloned config from DefaultTransport")
		}
		// Default transport clone already sets a non-nil TLSClientConfig, so the
		// "clone existing" arm should have run and MinVersion floored to TLS 1.2.
		if clone.TLSClientConfig.MinVersion != tls.VersionTLS12 {
			t.Fatalf("MinVersion = %d, want %d", clone.TLSClientConfig.MinVersion, tls.VersionTLS12)
		}
	})

	t.Run("opts tls config cloned onto clone", func(t *testing.T) {
		provided := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "provided.test"}
		clone := cloneRestrictedTransport(RestrictedOutboundHTTPOptions{TLSConfig: provided})
		if clone.TLSClientConfig == provided {
			t.Fatal("clone uses the same *tls.Config pointer as opts; expected a clone")
		}
		if clone.TLSClientConfig.ServerName != "provided.test" {
			t.Fatalf("ServerName = %q, want provided.test", clone.TLSClientConfig.ServerName)
		}
		if clone.TLSClientConfig.MinVersion != tls.VersionTLS12 {
			t.Fatalf("MinVersion = %d, want %d", clone.TLSClientConfig.MinVersion, tls.VersionTLS12)
		}
	})

	t.Run("response header timeout applied when positive", func(t *testing.T) {
		clone := cloneRestrictedTransport(RestrictedOutboundHTTPOptions{ResponseHeaderTimeout: 7 * time.Second})
		if clone.ResponseHeaderTimeout != 7*time.Second {
			t.Fatalf("ResponseHeaderTimeout = %v, want 7s", clone.ResponseHeaderTimeout)
		}
	})

	t.Run("response header timeout left untouched when zero", func(t *testing.T) {
		clone := cloneRestrictedTransport(RestrictedOutboundHTTPOptions{})
		if clone.ResponseHeaderTimeout != 0 {
			t.Fatalf("ResponseHeaderTimeout = %v, want 0 (not overridden)", clone.ResponseHeaderTimeout)
		}
	})

	t.Run("fallback transport and fresh tls config when default not http transport", func(t *testing.T) {
		// Swapping http.DefaultTransport lets the "else" arm run, which creates a
		// bare *http.Transport with a nil TLSClientConfig, in turn hitting the
		// switch default arm that allocates a fresh &tls.Config{}.
		orig := http.DefaultTransport
		http.DefaultTransport = roundTripperStub{}
		t.Cleanup(func() { http.DefaultTransport = orig })

		clone := cloneRestrictedTransport(RestrictedOutboundHTTPOptions{})
		if clone == nil {
			t.Fatal("clone is nil")
		}
		// The fallback transport is constructed with Proxy: http.ProxyFromEnvironment.
		if clone.Proxy == nil {
			t.Fatal("fallback transport Proxy nil, want ProxyFromEnvironment")
		}
		if clone.TLSClientConfig == nil {
			t.Fatal("TLSClientConfig nil, want freshly allocated config")
		}
		if clone.TLSClientConfig.MinVersion != tls.VersionTLS12 {
			t.Fatalf("MinVersion = %d, want %d", clone.TLSClientConfig.MinVersion, tls.VersionTLS12)
		}
	})

	t.Run("clone tls config is independent from opts source", func(t *testing.T) {
		provided := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: false}
		clone := cloneRestrictedTransport(RestrictedOutboundHTTPOptions{TLSConfig: provided})

		// Mutate the clone's nested config and assert the source is unchanged.
		clone.TLSClientConfig.InsecureSkipVerify = true
		clone.TLSClientConfig.MinVersion = tls.VersionTLS10
		if provided.InsecureSkipVerify {
			t.Fatal("mutating clone leaked into opts.TLSConfig (InsecureSkipVerify)")
		}
		if provided.MinVersion != tls.VersionTLS12 {
			t.Fatalf("opts.TLSConfig.MinVersion mutated to %d, want %d", provided.MinVersion, tls.VersionTLS12)
		}
	})

	t.Run("clone is independent from default transport", func(t *testing.T) {
		defaultBefore := http.DefaultTransport.(*http.Transport).Clone()
		clone := cloneRestrictedTransport(RestrictedOutboundHTTPOptions{ResponseHeaderTimeout: 9 * time.Second})

		// Mutate the clone's independent fields/config and assert the shared
		// default transport is untouched.
		clone.ResponseHeaderTimeout = 0
		clone.MaxIdleConns = 9999
		if clone.TLSClientConfig != nil {
			clone.TLSClientConfig.InsecureSkipVerify = true
		}
		defaultAfter := http.DefaultTransport.(*http.Transport).Clone()
		if defaultAfter.ResponseHeaderTimeout != defaultBefore.ResponseHeaderTimeout {
			t.Fatalf("DefaultTransport.ResponseHeaderTimeout changed: before %v after %v", defaultBefore.ResponseHeaderTimeout, defaultAfter.ResponseHeaderTimeout)
		}
		if defaultAfter.MaxIdleConns != defaultBefore.MaxIdleConns {
			t.Fatalf("DefaultTransport.MaxIdleConns changed: before %v after %v", defaultBefore.MaxIdleConns, defaultAfter.MaxIdleConns)
		}
		if defaultAfter.TLSClientConfig != nil && defaultAfter.TLSClientConfig.InsecureSkipVerify {
			t.Fatal("DefaultTransport TLSClientConfig.InsecureSkipVerify leaked to true")
		}
	})
}

// roundTripperStub is a non-*http.Transport used to drive the fallback arm of
// cloneRestrictedTransport when swapped in as http.DefaultTransport.
type roundTripperStub struct{}

func (roundTripperStub) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("roundTripperStub: not implemented")
}
