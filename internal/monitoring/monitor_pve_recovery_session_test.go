package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Public cluster reads drive the actual background recovery and the production
// monitor's per-endpoint projection. The independent sibling stays usable,
// while Proxmox's online state remains distinct from Pulse's usable session.
func TestClusterRecoverySessionEndpointProjection(t *testing.T) {
	var phase atomic.Int32
	var recoveryLogins atomic.Int32
	newEndpoint := func(primary bool) *httptest.Server {
		return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api2/json/access/ticket":
				if primary && phase.Load() == 1 && recoveryLogins.Add(1) > 1 {
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprint(w, `{"message":"fixture polling session refused"}`)
					return
				}
				fmt.Fprint(w, `{"data":{"ticket":"fixture-session","CSRFPreventionToken":"fixture-csrf"}}`)
			case "/api2/json/nodes":
				if !strings.Contains(r.Header.Get("Cookie"), "fixture-session") {
					t.Error("ordinary polling lost fixture authentication")
				}
				if primary && phase.Load() == 0 {
					w.WriteHeader(http.StatusServiceUnavailable)
					fmt.Fprint(w, `{"message":"fixture warming"}`)
					return
				}
				fmt.Fprint(w, `{"data":[{"node":"primary","status":"online"},{"node":"sibling","status":"online"}]}`)
			default:
				t.Errorf("recovery dispatched outside ordinary nodes/session reads: %s", r.URL.Path)
				http.NotFound(w, r)
			}
		}))
	}
	primary, sibling := newEndpoint(true), newEndpoint(false)
	defer primary.Close()
	defer sibling.Close()
	pin := func(s *httptest.Server) string {
		value := sha256.Sum256(s.Certificate().Raw)
		return hex.EncodeToString(value[:])
	}
	pins := map[string]string{primary.URL: pin(primary), sibling.URL: pin(sibling)}
	cc := proxmox.NewClusterClient("recovery-projection", proxmox.ClientConfig{
		Host: primary.URL, User: "fixture@pam", Password: "fixture-password",
		Fingerprint: pins[primary.URL], VerifySSL: true, Timeout: time.Second,
	}, []string{primary.URL, sibling.URL}, pins)
	if cc.GetHealthStatus()[primary.URL] || !cc.GetHealthStatus()[sibling.URL] {
		t.Fatal("fixture did not establish an unreachable primary with a usable sibling")
	}
	instance := config.PVEInstance{
		Name: "recovery-projection", Host: primary.URL, IsCluster: true, VerifySSL: true,
		Fingerprint: pins[primary.URL], ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "primary", NodeIdentity: "primary-identity", Host: primary.URL, Fingerprint: pins[primary.URL]},
			{NodeName: "sibling", NodeIdentity: "sibling-identity", Host: sibling.URL, Fingerprint: pins[sibling.URL]},
		},
	}
	monitor := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{instance}}}
	project := func(stage string) {
		monitor.updateClusterEndpointStatus(instance.Name, &instance, cc, []models.Node{
			{Name: "primary", Status: "online"}, {Name: "sibling", Status: "online"},
		})
		if !instance.ClusterEndpoints[0].Online || !instance.ClusterEndpoints[1].Online {
			t.Fatal("Pulse session evidence changed native cluster online state")
		}
		for i, endpoint := range instance.ClusterEndpoints {
			if endpoint.NodeIdentity != []string{"primary-identity", "sibling-identity"}[i] || endpoint.Fingerprint != pins[endpoint.Host] {
				t.Fatal("projection changed identity or saved TLS trust")
			}
		}
		wire, err := json.Marshal(struct {
			Phase     string                   `json:"phase"`
			Endpoints []config.ClusterEndpoint `json:"endpoints"`
		}{stage, instance.ClusterEndpoints})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("CLUSTER_SESSION_PROJECTION_JSON=%s", wire)
	}
	project("initial")

	// Honour the existing ten-second endpoint recovery interval. No process
	// clock, monitor, credentials or guest-command fence is reset for this test.
	phase.Store(1)
	time.Sleep(11 * time.Second)
	if nodes, err := cc.GetNodes(context.Background()); err != nil || len(nodes) != 2 {
		t.Fatalf("healthy sibling was blocked by primary recovery: %+v %v", nodes, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		health := cc.GetHealthStatusWithErrors()[primary.URL]
		if health.Healthy || health.LastError == "Authentication failed - check API token or credentials" {
			if health.Healthy || recoveryLogins.Load() != 2 {
				t.Fatalf("failed actual polling session became healthy or replayed: %+v logins=%d", health, recoveryLogins.Load())
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background recovery did not publish its actual session failure: %+v", health)
		}
		time.Sleep(5 * time.Millisecond)
	}
	project("refused")
	if endpoint := instance.ClusterEndpoints[0]; endpoint.PulseReachable == nil || *endpoint.PulseReachable || endpoint.PulseError == "" {
		t.Fatalf("monitor lost refused primary's actual session evidence: %+v", endpoint)
	}
	if nodes, err := cc.GetNodes(context.Background()); err != nil || len(nodes) != 2 || recoveryLogins.Load() != 2 {
		t.Fatalf("ordinary sibling read replayed refused authentication: %+v %v logins=%d", nodes, err, recoveryLogins.Load())
	}

	phase.Store(2)
	time.Sleep(11 * time.Second)
	if _, err := cc.GetNodes(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for !cc.GetHealthStatus()[primary.URL] {
		if time.Now().After(deadline) {
			t.Fatal("ordinary later recovery did not accept the usable polling session")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if nodes, err := cc.GetNodes(context.Background()); err != nil || len(nodes) != 2 {
		t.Fatalf("recovered primary cannot serve ordinary polling: %+v %v", nodes, err)
	}
	project("recovered")
	if endpoint := instance.ClusterEndpoints[0]; endpoint.PulseReachable == nil || !*endpoint.PulseReachable || endpoint.PulseError != "" {
		t.Fatalf("accepted recovery left stale primary failure: %+v", endpoint)
	}
}
