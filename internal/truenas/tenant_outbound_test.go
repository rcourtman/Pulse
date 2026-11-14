package truenas

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	pubsec "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

func TestTenantTrueNASRebindingNeverReachesRESTOrWebSocket(t *testing.T) {
	for _, rpc := range []bool{false, true} {
		t.Run(map[bool]string{true: "WebSocket", false: "REST"}[rpc], func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(503) }))
			defer server.Close()
			endpoint, _ := url.Parse(server.URL)
			port, _ := strconv.Atoi(endpoint.Port())
			calls := 0
			resolve := func(context.Context, string) ([]net.IPAddr, error) {
				calls++
				if calls == 1 {
					return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
				}
				return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
			}
			// This is the successful earlier validation. The consumer must not trust
			// that answer, nor re-resolve a name after its own validation.
			if err := pubsec.ValidateOutboundHost(context.Background(), "nas.example.test", pubsec.RestrictedOutboundHTTPOptions{ResolveIPAddrs: resolve}); err != nil {
				t.Fatal(err)
			}
			client, err := newClientWithResolver(ClientConfig{Host: "nas.example.test", Port: port, UseHTTPS: true, InsecureSkipVerify: true, APIKey: "synthetic", Username: "fixture", OutboundPolicy: securityutil.ConnectionOutboundTenantLAN, Timeout: time.Second}, resolve)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			transport := client.httpClient.Transport.(*http.Transport)
			if transport.Proxy != nil {
				t.Fatal("tenant REST kept proxy DNS path")
			}
			if rpc {
				conn, e := client.dialRPCEndpoint(context.Background(), client.rpcURL, true)
				err = e
				if conn != nil {
					conn.Close()
				}
			} else {
				var result any
				err = client.getJSON(context.Background(), http.MethodGet, "/system/info", &result)
			}
			if !errors.Is(err, securityutil.ErrTenantOutboundDestination) {
				t.Fatalf("rebound refusal=%v", err)
			}
			if hits.Load() != 0 || calls != 2 {
				t.Fatalf("requests=%d resolutions=%d", hits.Load(), calls)
			}
		})
	}
}

func TestTenantTrueNASProxyCannotResolveAroundPolicy(t *testing.T) {
	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { proxyHits.Add(1); w.WriteHeader(503) }))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	client, err := newClientWithResolver(ClientConfig{Host: "https://nas.example.test", OutboundPolicy: securityutil.ConnectionOutboundTenantLAN, APIKey: "synthetic", Timeout: time.Second}, func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.GetSystemInfo(context.Background()); !errors.Is(err, securityutil.ErrTenantOutboundDestination) {
		t.Fatalf("proxy refusal=%v", err)
	}
	if proxyHits.Load() != 0 {
		t.Fatal("tenant request reached proxy")
	}
	legacy, err := NewClient(ClientConfig{Host: "https://nas.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	if legacy.httpClient.Transport.(*http.Transport).Proxy == nil || legacy.outboundDialContext != nil {
		t.Fatal("instance proxy/dial changed")
	}
}
