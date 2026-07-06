package securityutil

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func localNetworkHTTPOptions(resolver func(context.Context, string) ([]net.IPAddr, error)) PulseURLValidationOptions {
	return PulseURLValidationOptions{
		AllowLocalNetworkHTTP: true,
		ResolveIPAddrs:        resolver,
	}
}

func TestNormalizePulseHTTPBaseURLWithOptionsAllowsResolvedLocalDNS(t *testing.T) {
	opts := localNetworkHTTPOptions(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "myhost.fritz.box" {
			t.Fatalf("resolved host = %q, want myhost.fritz.box", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("192.168.178.20")}}, nil
	})

	got, err := NormalizePulseHTTPBaseURLWithOptions("http://myhost.fritz.box:7655/", opts)
	if err != nil {
		t.Fatalf("NormalizePulseHTTPBaseURLWithOptions() error = %v", err)
	}
	if got.String() != "http://myhost.fritz.box:7655" {
		t.Fatalf("NormalizePulseHTTPBaseURLWithOptions() = %q", got.String())
	}
}

func TestNormalizePulseHTTPBaseURLWithOptionsRejectsPublicDNS(t *testing.T) {
	opts := localNetworkHTTPOptions(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "pulse.example.test" {
			t.Fatalf("resolved host = %q, want pulse.example.test", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
	})

	_, err := NormalizePulseHTTPBaseURLWithOptions("http://pulse.example.test:7655/", opts)
	if err == nil || !strings.Contains(err.Error(), "must use https unless host is loopback or local/private") {
		t.Fatalf("NormalizePulseHTTPBaseURLWithOptions() error = %v, want public HTTP rejection", err)
	}
}

func TestNormalizePulseHTTPBaseURLWithOptionsRejectsMixedPublicAndLocalDNS(t *testing.T) {
	opts := localNetworkHTTPOptions(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "mixed.example.test" {
			t.Fatalf("resolved host = %q, want mixed.example.test", host)
		}
		return []net.IPAddr{
			{IP: net.ParseIP("192.168.1.25")},
			{IP: net.ParseIP("198.51.100.25")},
		}, nil
	})

	_, err := NormalizePulseHTTPBaseURLWithOptions("http://mixed.example.test:7655/", opts)
	if err == nil || !strings.Contains(err.Error(), "must use https unless host is loopback or local/private") {
		t.Fatalf("NormalizePulseHTTPBaseURLWithOptions() error = %v, want mixed DNS rejection", err)
	}
}

func TestNormalizePulseHTTPBaseURLWithOptionsRejectsUnresolvedDNS(t *testing.T) {
	opts := localNetworkHTTPOptions(func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("lookup failed")
	})

	_, err := NormalizePulseHTTPBaseURLWithOptions("http://unresolved.example.test:7655/", opts)
	if err == nil || !strings.Contains(err.Error(), "must use https unless host is loopback or local/private") {
		t.Fatalf("NormalizePulseHTTPBaseURLWithOptions() error = %v, want unresolved DNS rejection", err)
	}
}

func TestNormalizePulseWebSocketBaseURLWithOptionsAllowsResolvedCGNATDNS(t *testing.T) {
	opts := localNetworkHTTPOptions(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "pulse.tailnet.example" {
			t.Fatalf("resolved host = %q, want pulse.tailnet.example", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("100.100.100.5")}}, nil
	})

	got, err := NormalizePulseWebSocketBaseURLWithOptions("http://pulse.tailnet.example:7655/pulse", opts)
	if err != nil {
		t.Fatalf("NormalizePulseWebSocketBaseURLWithOptions() error = %v", err)
	}
	if got.String() != "ws://pulse.tailnet.example:7655/pulse" {
		t.Fatalf("NormalizePulseWebSocketBaseURLWithOptions() = %q", got.String())
	}
}
