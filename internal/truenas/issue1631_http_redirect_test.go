package truenas

// Pins the issue #1631 regression: the v6.1.2 JSON-RPC migration broke
// http-configured appliances behind TrueNAS's HTTP -> HTTPS redirect. The
// REST transport used before followed the redirect transparently; the
// websocket handshake fails with status=302 instead, so the client must
// upgrade a same-host https redirect to wss itself, and refuse anything
// that would move the connection to another host.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newHTTPRedirectServer(t *testing.T, location func() string, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, location(), status)
	}))
	t.Cleanup(server.Close)
	return server
}

func redirectFixtureClient(t *testing.T, redirectURL string) *Client {
	t.Helper()
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("parse redirect server URL %q: %v", redirectURL, err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("parse redirect server port from %q: %v", redirectURL, err)
	}
	client, err := NewClient(ClientConfig{
		Host:               "http://" + parsed.Hostname(),
		Port:               port,
		Username:           "pulse-readonly",
		APIKey:             "readonly-key",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestIssue1631HTTPRedirectUpgradesHandshakeToTLS(t *testing.T) {
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		if request.Method == "auth.login_ex" {
			return protocolFixtureReply{result: map[string]any{"response_type": "SUCCESS", "user_info": nil}}
		}
		t.Errorf("unexpected rpc method %q", request.Method)
		return protocolFixtureReply{close: true}
	}, nil)

	redirect := newHTTPRedirectServer(t, func() string {
		return fixture.server.URL + "/api/current"
	}, http.StatusFound)
	client := redirectFixtureClient(t, redirect.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mode, err := client.ensureTransport(ctx)
	if err != nil {
		t.Fatalf("ensureTransport() error = %v", err)
	}
	if mode != TransportJSONRPC {
		t.Fatalf("ensureTransport() mode = %s, want %s", mode, TransportJSONRPC)
	}

	status := client.TransportStatus()
	if !status.TLS {
		t.Errorf("TransportStatus().TLS = false, want true after https upgrade")
	}
	if !strings.HasPrefix(status.Endpoint, "wss://") {
		t.Errorf("TransportStatus().Endpoint = %q, want wss:// endpoint", status.Endpoint)
	}
	if !client.config.UseHTTPS {
		t.Errorf("config.UseHTTPS = false, want true after https upgrade")
	}
	if fixture.sessions.Load() == 0 {
		t.Errorf("TLS fixture saw no websocket sessions; upgrade retry never landed")
	}
}

func TestIssue1631CrossHostRedirectIsRefused(t *testing.T) {
	redirect := newHTTPRedirectServer(t, func() string {
		return "https://other-appliance.example/api/current"
	}, http.StatusFound)
	client := redirectFixtureClient(t, redirect.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.ensureTransport(ctx)
	if err == nil {
		t.Fatal("ensureTransport() error = nil, want cross-host redirect refusal")
	}
	if !strings.Contains(err.Error(), "redirected to") || !strings.Contains(err.Error(), "other-appliance.example") {
		t.Errorf("ensureTransport() error = %q, want actionable message naming the redirect target", err)
	}
	if status := client.TransportStatus(); status.TLS {
		t.Errorf("TransportStatus().TLS = true, want false when redirect is refused")
	}
}

func TestIssue1631HTTPSUpgradeTargetRules(t *testing.T) {
	cases := []struct {
		name     string
		current  string
		location string
		want     string
		ok       bool
	}{
		{"same host default port", "ws://nas.local:80/api/current", "https://nas.local/ui/", "wss://nas.local:443/api/current", true},
		{"same host explicit port", "ws://nas.local:80/api/current", "https://nas.local:8443/", "wss://nas.local:8443/api/current", true},
		{"cross host", "ws://nas.local:80/api/current", "https://evil.example/", "", false},
		{"relative location", "ws://nas.local:80/api/current", "/ui/", "", false},
		{"downgrade from wss", "wss://nas.local:443/api/current", "https://nas.local/", "", false},
		{"http location", "ws://nas.local:80/api/current", "http://nas.local:8080/", "", false},
		{"empty location", "ws://nas.local:80/api/current", "", "", false},
	}
	for _, tc := range cases {
		got, ok := httpsUpgradeTarget(tc.current, tc.location)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: httpsUpgradeTarget(%q, %q) = (%q, %v), want (%q, %v)", tc.name, tc.current, tc.location, got, ok, tc.want, tc.ok)
		}
	}
}
