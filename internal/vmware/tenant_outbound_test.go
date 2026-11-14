package vmware

import (
	"context"
	"errors"
	"fmt"
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
	var hits, sockets atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(503) }))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			sockets.Add(1)
		}
	}
	server.StartTLS()
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
	var connectionErr *ConnectionError
	if !errors.As(err, &connectionErr) || connectionErr.Category != "invalid_config" || !strings.Contains(err.Error(), securityutil.ErrTenantOutboundDestination.Error()) {
		t.Fatalf("rebound refusal=%v", err)
	}
	if hits.Load() != 0 || sockets.Load() != 0 || calls != 2 {
		t.Fatalf("requests=%d sockets=%d resolutions=%d", hits.Load(), sockets.Load(), calls)
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

func TestTenantVMwarePolicyMessageKeepsEndpointAndSecretsOut(t *testing.T) {
	inner := fmt.Errorf("%w: https://fixture-user:fixture-password@private.example.test/private?token=synthetic", securityutil.ErrTenantOutboundDestination)
	err := classifyTransportError("automation session", &url.Error{Op: "Post", URL: "https://private.example.test", Err: inner})
	var failure *ConnectionError
	if !errors.As(err, &failure) || failure.Category != "invalid_config" || !strings.Contains(failure.Message, "instance administrator") {
		t.Fatalf("policy guidance=%v", err)
	}
	for _, secret := range []string{"fixture-user", "fixture-password", "private.example.test", "private?", "synthetic"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("refusal exposed %s", secret)
		}
	}
	ordinary := classifyTransportError("automation session", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("fixture refused")})
	if !errors.As(ordinary, &failure) || failure.Category != "network" {
		t.Fatal("ordinary network failure changed category")
	}
}
