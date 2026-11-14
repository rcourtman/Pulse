package vmware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	pubsec "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

func TestTenantVMwareRebindingNeverReachesAuthentication(t *testing.T) {
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
	if err := pubsec.ValidateOutboundHost(context.Background(), "vc.example.test", pubsec.RestrictedOutboundHTTPOptions{ResolveIPAddrs: resolve}); err != nil {
		t.Fatal(err)
	}
	client, err := newClientWithResolver(ClientConfig{Host: "vc.example.test", Port: port, Username: "fixture", Password: "synthetic", InsecureSkipVerify: true, OutboundPolicy: securityutil.ConnectionOutboundTenantLAN, Timeout: time.Second}, resolve)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if client.httpClient.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("tenant kept proxy")
	}
	_, err = client.TestConnection(context.Background())
	if err == nil || !strings.Contains(err.Error(), securityutil.ErrTenantOutboundDestination.Error()) {
		t.Fatalf("rebound refusal=%v", err)
	}
	if hits.Load() != 0 || calls != 2 {
		t.Fatalf("requests=%d resolutions=%d", hits.Load(), calls)
	}
}

func TestTenantVMwareTransportKeepsTLSAndRedirectPolicy(t *testing.T) {
	for _, skip := range []bool{false, true} {
		instance, err := NewClient(ClientConfig{Host: "https://vc.example.test", InsecureSkipVerify: skip})
		if err != nil {
			t.Fatal(err)
		}
		defer instance.Close()
		tenant, err := NewClient(ClientConfig{Host: "https://vc.example.test", InsecureSkipVerify: skip, OutboundPolicy: securityutil.ConnectionOutboundTenantPublic})
		if err != nil {
			t.Fatal(err)
		}
		defer tenant.Close()
		a := instance.httpClient.Transport.(*http.Transport)
		b := tenant.httpClient.Transport.(*http.Transport)
		if a.TLSClientConfig.InsecureSkipVerify != b.TLSClientConfig.InsecureSkipVerify || a.TLSClientConfig.MinVersion != b.TLSClientConfig.MinVersion || b.DialContext == nil {
			t.Fatal("TLS setting changed or policy dial missing")
		}
		req, _ := http.NewRequest(http.MethodGet, "https://other.example.test", nil)
		origin, _ := http.NewRequest(http.MethodGet, "https://vc.example.test", nil)
		if err := tenant.httpClient.CheckRedirect(req, []*http.Request{origin}); err == nil {
			t.Fatal("cross-host redirect allowed")
		}
	}
}
