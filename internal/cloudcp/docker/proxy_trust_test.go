package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTenantProxyTrustKeepsIsolatedMSPDiscoverySeparate(t *testing.T) {
	var inspections atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Api-Version", "1.47")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			_, _ = w.Write([]byte("OK"))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/networks/provider-t-acme"):
			inspections.Add(1)
			_, _ = w.Write([]byte(`{"IPAM":{"Config":[{"Subnet":"172.31.8.0/24"},{"Subnet":"2001:db8:31::/64"}]}}`))
		default:
			t.Errorf("unexpected synthetic Docker request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	for _, isolated := range []bool{false, true} {
		mgr, err := NewManager(ManagerConfig{
			Network: "provider", IsolateTenantNetworks: isolated,
			TrustedProxyCIDRs: []string{"192.0.2.2", "192.0.2.2/32", "2001:db8::2", "bad-value"},
		})
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		t.Cleanup(func() { _ = mgr.Close() })
		want := []string{"192.0.2.2/32", "2001:db8::2/128"}
		if isolated {
			want = []string{"172.31.8.0/24", "192.0.2.2/32", "2001:db8:31::/64", "2001:db8::2/128"}
		}
		got := mgr.tenantTrustedProxyCIDRs(context.Background(), "provider-t-acme")
		if !slices.Equal(got, want) {
			t.Errorf("isolated=%v trust = %v, want %v", isolated, got, want)
		}
	}
	if got := inspections.Load(); got != 1 {
		t.Errorf("network inspections = %d, want only the isolated MSP lookup", got)
	}
}
