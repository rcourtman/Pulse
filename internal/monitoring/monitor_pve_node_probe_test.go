package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A failed member /nodes response cannot establish a healthy polling endpoint
// merely by quoting a guest error. The primary's accepted inventory still
// reports both members online. Public reads initiate later ordinary recovery;
// no guest command, backup, credential change or guest fence is exercised.
func TestClusterNodeProbeEndpointProjection(t *testing.T) {
	testClusterNodeProbeEndpointProjection(t, http.StatusInternalServerError, "QEMU guest agent is not running", true)
}

func TestClusterNodeInventoryEndpointProjection(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		complete   bool
	}{
		{"missing", `{}`, true},
		{"duplicate", `{"data":{},"data":[]}`, true},
		{"truncated", `{"data":[{"node":"member","status":"online"}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) { testClusterNodeProbeEndpointProjection(t, http.StatusOK, tc.body, tc.complete) })
	}
}

func testClusterNodeProbeEndpointProjection(t *testing.T, status int, failureBody string, complete bool) {
	var acceptMember atomic.Bool
	var memberLogins, memberNodes atomic.Int32
	endpoint := func(member bool) *httptest.Server {
		s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api2/json/access/ticket":
				if member {
					memberLogins.Add(1)
				}
				fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			case "/api2/json/nodes":
				if r.Header.Get("Cookie") != "PVEAuthCookie=fixture-ticket" {
					t.Error("ordinary nodes read lost password authentication")
				}
				if member {
					memberNodes.Add(1)
					if !acceptMember.Load() {
						if !complete {
							w.Header().Set("Content-Length", fmt.Sprint(len(failureBody)+100))
						}
						w.WriteHeader(status)
						fmt.Fprint(w, failureBody)
						return
					}
				}
				fmt.Fprint(w, `{"data":[{"node":"primary","status":"online","cpu":0.125},{"node":"member","status":"online","cpu":0.25}]}`)
			default:
				t.Errorf("left ordinary session/nodes reads: %s", r.URL.Path)
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(s.Close)
		return s
	}
	primary, member := endpoint(false), endpoint(true)
	pin := func(s *httptest.Server) string {
		v := sha256.Sum256(s.Certificate().Raw)
		return hex.EncodeToString(v[:])
	}
	pins := map[string]string{primary.URL: pin(primary), member.URL: pin(member)}
	cc := proxmox.NewClusterClient("node-probe-cluster", proxmox.ClientConfig{Host: primary.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: pins[primary.URL], Timeout: 2 * time.Second}, []string{primary.URL, member.URL}, pins)
	if !cc.GetHealthStatus()[primary.URL] || cc.GetHealthStatus()[member.URL] || memberLogins.Load() != 1 || memberNodes.Load() != 1 {
		t.Fatalf("failed nodes probe became healthy or advanced to a full session: logins=%d nodes=%d", memberLogins.Load(), memberNodes.Load())
	}
	inst := config.PVEInstance{Name: "node-probe-cluster", Host: primary.URL, IsCluster: true, ClusterName: "Fixture cluster", VerifySSL: true, Fingerprint: pins[primary.URL], ClusterEndpoints: []config.ClusterEndpoint{
		{NodeName: "primary", NodeIdentity: "primary-identity", Host: primary.URL, Fingerprint: pins[primary.URL]},
		{NodeName: "member", NodeIdentity: "member-identity", Host: member.URL, Fingerprint: pins[member.URL]},
	}}
	m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{inst}}}
	project := func(phase string) {
		nodes, err := cc.GetNodes(context.Background())
		if err != nil || len(nodes) != 2 {
			t.Fatalf("accepted primary read failed: %+v %v", nodes, err)
		}
		model := make([]models.Node, len(nodes))
		for i, n := range nodes {
			model[i] = models.Node{Name: n.Node, Status: n.Status}
		}
		m.updateClusterEndpointStatus(inst.Name, &inst, cc, model)
		for i, ep := range inst.ClusterEndpoints {
			if !ep.Online || ep.NodeIdentity != []string{"primary-identity", "member-identity"}[i] || ep.Fingerprint != pins[ep.Host] {
				t.Fatal("projection changed native online state, identity or saved trust")
			}
		}
		wire, err := json.Marshal(struct {
			Phase     string                   `json:"phase"`
			Case      string                   `json:"case"`
			Endpoints []config.ClusterEndpoint `json:"endpoints"`
		}{phase, t.Name(), inst.ClusterEndpoints})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("NODE_PROBE_ENDPOINT_PROJECTION_JSON=%s", wire)
	}
	project("nodes-probe-refused")
	expectedError := "API error 500: QEMU guest agent is not running"
	if status == http.StatusOK {
		expectedError = "invalid Proxmox node inventory response"
		if !complete {
			expectedError = "read Proxmox node inventory: unexpected EOF"
		}
	}
	if ep := inst.ClusterEndpoints[1]; ep.PulseReachable == nil || *ep.PulseReachable || ep.PulseError != expectedError {
		t.Fatalf("consumer lost failed nodes observation: %+v", ep)
	}
	acceptMember.Store(true)
	// Honour the real recovery interval. Do not clear a production admission,
	// replace clients or change clocks to manufacture the consuming result.
	time.Sleep(11 * time.Second)
	if _, err := cc.GetNodes(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !cc.GetHealthStatus()[member.URL] {
		if time.Now().After(deadline) {
			t.Fatal("later ordinary recovery did not accept member")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if memberLogins.Load() != 3 || memberNodes.Load() != 2 {
		t.Fatalf("later recovery did not use one distinct probe/full session: logins=%d nodes=%d", memberLogins.Load(), memberNodes.Load())
	}
	project("member-recovered")
	if ep := inst.ClusterEndpoints[1]; ep.PulseReachable == nil || !*ep.PulseReachable || ep.PulseError != "" {
		t.Fatalf("accepted recovery retained stale failure: %+v", ep)
	}
}
