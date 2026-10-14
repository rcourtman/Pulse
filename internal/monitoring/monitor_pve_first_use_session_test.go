package monitoring

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
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

// Drive the password-construction TLS mismatch through the real cluster and
// monitor projection. The short pinned probe is accepted, but the first full
// polling session is refused; ordinary later recovery must precede removing
// the consumer's Unreachable state. No guest, backup or native host is used.
func TestClusterFirstUsePasswordEndpointProjection(t *testing.T) {
	var acceptMember atomic.Bool
	var memberLogins atomic.Int32
	endpoint := func(member bool) *httptest.Server {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
		der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
		if err != nil {
			t.Fatal(err)
		}
		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api2/json/access/ticket":
				if member && memberLogins.Add(1) > 1 && !acceptMember.Load() {
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprint(w, `{"message":"fixture full polling session refused"}`)
					return
				}
				fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			case "/api2/json/nodes":
				if !strings.Contains(r.Header.Get("Cookie"), "PVEAuthCookie=fixture-ticket") {
					t.Error("node read lost password session")
				}
				fmt.Fprint(w, `{"data":[{"node":"primary","status":"online","cpu":0.125},{"node":"member","status":"online","cpu":0.25}]}`)
			default:
				t.Errorf("outside ordinary cluster reads: %s", r.URL.Path)
				http.NotFound(w, r)
			}
		}))
		s.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
		s.StartTLS()
		t.Cleanup(s.Close)
		return s
	}
	primary, member := endpoint(false), endpoint(true)
	sum := sha256.Sum256(primary.Certificate().Raw)
	primaryPin := hex.EncodeToString(sum[:])
	cc := proxmox.NewClusterClient("first-use-cluster", proxmox.ClientConfig{Host: primary.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: primaryPin, Timeout: 2 * time.Second}, []string{primary.URL, member.URL}, nil)
	if !cc.GetHealthStatus()[primary.URL] || cc.GetHealthStatus()[member.URL] || memberLogins.Load() != 2 {
		t.Fatalf("first-use probe/full-session admission was not distinguished: member logins=%d", memberLogins.Load())
	}
	inst := config.PVEInstance{Name: "first-use-cluster", Host: primary.URL, IsCluster: true, ClusterName: "Fixture cluster", VerifySSL: true, Fingerprint: primaryPin, ClusterEndpoints: []config.ClusterEndpoint{
		{NodeName: "primary", NodeIdentity: "primary-identity", Host: primary.URL, Fingerprint: primaryPin, DisplayName: "Trusted primary"},
		{NodeName: "member", NodeIdentity: "member-identity", Host: member.URL, DisplayName: "New member"},
	}}
	m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{inst}}}
	project := func(phase string) {
		nodes, err := cc.GetNodes(context.Background())
		if err != nil || len(nodes) != 2 {
			t.Fatalf("trusted ordinary read failed: %+v %v", nodes, err)
		}
		model := make([]models.Node, len(nodes))
		for i, n := range nodes {
			model[i] = models.Node{Name: n.Node, Status: n.Status}
		}
		m.updateClusterEndpointStatus(inst.Name, &inst, cc, model)
		for i, ep := range inst.ClusterEndpoints {
			if !ep.Online || ep.NodeIdentity != []string{"primary-identity", "member-identity"}[i] {
				t.Fatal("Pulse reachability changed native status or identity")
			}
		}
		if inst.Fingerprint != primaryPin || inst.ClusterEndpoints[0].Fingerprint != primaryPin || inst.ClusterEndpoints[1].Fingerprint != "" {
			t.Fatal("runtime first-use trust changed saved configuration")
		}
		b, err := json.Marshal(struct {
			Phase     string                   `json:"phase"`
			Endpoints []config.ClusterEndpoint `json:"endpoints"`
		}{phase, inst.ClusterEndpoints})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("FIRST_USE_ENDPOINT_PROJECTION_JSON=%s", b)
	}
	project("full-session-refused")
	if ep := inst.ClusterEndpoints[1]; ep.PulseReachable == nil || *ep.PulseReachable || ep.PulseError != "Authentication failed - check API token or credentials" {
		t.Fatalf("consumer lost failed polling-session reason: %+v", ep)
	}
	acceptMember.Store(true)
	// Preserve the real ten-second recovery interval and use public reads to
	// initiate the ordinary background recovery, rather than calling internals.
	time.Sleep(11 * time.Second)
	if _, err := cc.GetNodes(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !cc.GetHealthStatus()[member.URL] {
		if time.Now().After(deadline) {
			t.Fatal("later normal recovery did not accept member")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if memberLogins.Load() != 4 {
		t.Fatalf("separate accepted probe/full session logins=%d", memberLogins.Load())
	}
	project("member-recovered")
	if ep := inst.ClusterEndpoints[1]; ep.PulseReachable == nil || !*ep.PulseReachable || ep.PulseError != "" {
		t.Fatalf("accepted polling session did not clear consumer error: %+v", ep)
	}
}
