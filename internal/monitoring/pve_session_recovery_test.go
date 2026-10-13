package monitoring

import (
	"context"
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
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Exercise the changed password client through the real parallel node poller,
// registry and frontend projection. No native QGA/SSH, backup or database is
// used. The emitted synthetic resources also feed the retained pixel proof.
func TestPVESessionRenewalProducesTrustedNodeReadback(t *testing.T) {
	const count = 8
	var authCalls, rejectedStatus, acceptedStatus, agentCalls atomic.Int32
	allOldReads := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/access/ticket"):
			n := authCalls.Add(1)
			fmt.Fprintf(w, `{"data":{"ticket":"synthetic-session-%d","CSRFPreventionToken":"synthetic-csrf-%d"}}`, n, n)
		case r.URL.Path == "/api2/json/nodes":
			nodes := make([]proxmox.Node, count)
			for i := range nodes {
				nodes[i] = proxmox.Node{Node: fmt.Sprintf("session-node-%02d", i+1), Status: "online", CPU: .05, MaxCPU: 4, MaxMem: 2 << 30, Mem: 1 << 30, MaxDisk: 100 << 30, Disk: 25 << 30, Uptime: 7200}
			}
			json.NewEncoder(w).Encode(map[string]any{"data": nodes})
		case strings.HasSuffix(r.URL.Path, "/status"):
			cookie, err := r.Cookie("PVEAuthCookie")
			if err != nil {
				t.Error("status missing synthetic session")
				w.WriteHeader(401)
				return
			}
			if cookie.Value == "synthetic-session-1" {
				if rejectedStatus.Add(1) == count {
					close(allOldReads)
				}
				select {
				case <-allOldReads:
				case <-ctx.Done():
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"message":"synthetic expired session"}`)
				return
			}
			acceptedStatus.Add(1)
			fmt.Fprint(w, `{"data":{"cpu":0.05,"memory":{"total":2147483648,"used":536870912,"free":1073741824,"available":1610612736},"rootfs":{"total":107374182400,"used":26843545600,"free":80530636800},"uptime":7200,"loadavg":[0.1,0.1,0.1],"kversion":"fixture-kernel","pveversion":"pve-manager/9.2.21/fixture","cpuinfo":{"model":"Synthetic CPU","cores":4,"sockets":1,"mhz":"2000"}}}`)
		case strings.Contains(r.URL.Path, "/agent/"):
			agentCalls.Add(1)
			t.Error("node recovery queued guest work")
			w.WriteHeader(500)
		default:
			fmt.Fprint(w, `{"data":[]}`) // ordinary network/apt inventory only
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	rawNodes, err := client.GetNodes(ctx)
	if err != nil || len(rawNodes) != count {
		t.Fatalf("ordinary node discovery: %v", err)
	}
	instance := config.PVEInstance{Name: "session-fixture", Host: server.URL, IsCluster: true, ClusterName: "Session recovery"}
	m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{instance}}, nodeLastOnline: make(map[string]time.Time)}
	nodes, statuses, _ := m.pollPVENodesParallel(ctx, instance.Name, &instance, client, rawNodes, "healthy", nil, false)
	if len(nodes) != count || len(statuses) != count || authCalls.Load() != 2 || rejectedStatus.Load() != count || acceptedStatus.Load() != count || agentCalls.Load() != 0 {
		t.Fatalf("node recovery counts nodes/statuses/auth/rejected/accepted/guest=%d/%d/%d/%d/%d/%d", len(nodes), len(statuses), authCalls.Load(), rejectedStatus.Load(), acceptedStatus.Load(), agentCalls.Load())
	}
	for _, node := range nodes {
		if node.Status != "online" || node.ConnectionHealth != "healthy" || !node.Memory.HasKnownUsage() || node.Memory.Usage != 25 || node.Disk.Usage != 25 {
			t.Fatalf("session recovery lost trusted ordinary fields: %+v", node)
		}
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{Nodes: nodes})
	resources, _ := convertPresentationResourcesForBroadcast(registry.ListForPresentation())
	if len(resources) != count {
		t.Fatal("node projection lost recovered nodes")
	}
	for _, resource := range resources {
		if resource.Memory == nil || resource.Memory.Current != 25 || resource.CPU == nil || resource.CPU.Current != 5 {
			t.Fatalf("rendered projection changed cache-aware memory/CPU: %+v", resource)
		}
	}
	payload, err := json.Marshal(map[string]any{"resources": resources, "authentications": authCalls.Load(), "rejectedStatusReads": rejectedStatus.Load(), "acceptedStatusReads": acceptedStatus.Load(), "guestCommands": agentCalls.Load()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("PULSE_SESSION_RECOVERY_JSON=%s", payload)
}
