package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// The snapshot calls use the production HTTP client; unrelated storage calls
// stay empty. No QGA, backup job or external service is used by this fixture.
type snapshotInventoryHTTPClient struct {
	mockPVEClientExtra
	api *proxmox.Client
}

func (c *snapshotInventoryHTTPClient) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return c.api.GetVMSnapshots(ctx, node, vmid)
}

func (c *snapshotInventoryHTTPClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return c.api.GetContainerSnapshots(ctx, node, vmid)
}

func (*snapshotInventoryHTTPClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return nil, nil
}

type snapshotInventoryHTTPFixture struct {
	f        *snapshotRecoveryFixture
	client   *snapshotInventoryHTTPClient
	mu       sync.Mutex
	bodies   map[string]string
	requests int
}

func snapshotInventoryBody(name string, at time.Time) string {
	return fmt.Sprintf(`{"data":[{"name":"current"},{"name":%q,"snaptime":%d,"description":"accepted listing"}]}`, name, at.Unix())
}

func newSnapshotInventoryHTTPFixture(t *testing.T, kind string) *snapshotInventoryHTTPFixture {
	t.Helper()
	m := snapshotCollectionMonitor(t, kind, true)
	_, manager := recoveryIngestTestMonitor(t)
	m.recoveryManager = manager
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	f := &snapshotRecoveryFixture{m: m, store: store, kind: kind}
	t.Cleanup(func() { f.wait(t); _ = store.Close() })
	at := time.Now().Add(-16 * 24 * time.Hour)
	h := &snapshotInventoryHTTPFixture{f: f, bodies: map[string]string{
		"node-a": snapshotInventoryBody("weekly", at),
		"node-b": snapshotInventoryBody("weekly", at),
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.requests++
		for node, body := range h.bodies {
			vmid := 101
			if node == "node-b" {
				vmid = 202
			}
			if r.URL.Path == fmt.Sprintf("/api2/json/nodes/%s/%s/%d/snapshot", node, kind, vmid) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, body)
				return
			}
		}
		t.Errorf("unexpected fixture request: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	api, err := proxmox.NewClient(proxmox.ClientConfig{
		Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.client = &snapshotInventoryHTTPClient{api: api}
	return h
}

func (h *snapshotInventoryHTTPFixture) set(node, body string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.bodies[node] = body
}

func (h *snapshotInventoryHTTPFixture) poll(t *testing.T, instance string) {
	t.Helper()
	h.mu.Lock()
	before := h.requests
	h.mu.Unlock()
	h.f.m.pollGuestSnapshots(context.Background(), instance, h.client)
	h.f.wait(t)
	want := 2
	if instance == "other-site" {
		want = 1
	}
	h.mu.Lock()
	requests := h.requests - before
	h.mu.Unlock()
	if requests != want {
		t.Fatalf("poll made %d HTTP reads, want %d (no retries or other operations)", requests, want)
	}
}

func TestPVESnapshotHTTPInvalidResponseRetainsHistory(t *testing.T) {
	cases := map[string]string{
		"missing-data":       `{}`,
		"null-data":          `{"data":null}`,
		"error-envelope":     `{"data":[],"errors":{"message":"fixture-private-detail"}}`,
		"duplicate-data":     `{"data":[{"name":"unaccepted","snaptime":1700000000}],"data":[]}`,
		"trailing-value":     `{"data":[]} {"data":[]}`,
		"null-row":           `{"data":[null]}`,
		"invalid-time":       `{"data":[{"name":"unaccepted"}]}`,
		"bad-row-after-good": `{"data":[{"name":"unaccepted","snaptime":1700000000},null]}`,
	}
	for _, kind := range []string{"qemu", "lxc"} {
		for _, cold := range []bool{false, true} {
			for name, body := range cases {
				t.Run(fmt.Sprintf("%s/cold=%t/%s", kind, cold, name), func(t *testing.T) {
					h := newSnapshotInventoryHTTPFixture(t, kind)
					h.poll(t, "site")
					h.poll(t, "other-site")
					before := h.f.points(t)
					if len(before) != 3 {
						t.Fatalf("control has %d points, want 3", len(before))
					}
					beforeRows := snapshotCollectionByID(h.f.m)
					warning := snapshotCollectionAlert(t, h.f.m, "site:node-a:101")
					if err := h.f.m.alertManager.AcknowledgeAlert(warning.ID, "operator"); err != nil {
						t.Fatal(err)
					}
					warning = snapshotCollectionAlert(t, h.f.m, "site:node-a:101")
					if cold {
						h.f.coldState(true)
					}
					h.set("node-a", body)
					h.set("node-b", snapshotInventoryBody("fresh-peer", time.Now().Add(-time.Hour)))
					for range 2 {
						h.poll(t, "site")
					}
					after := h.f.points(t)
					failedID, otherID := "pve-snapshot:site-node-a-101-weekly", "pve-snapshot:other-site-node-a-101-weekly"
					if !reflect.DeepEqual(before[failedID], after[failedID]) || !reflect.DeepEqual(before[otherID], after[otherID]) {
						t.Fatal("invalid HTTP success deleted or renewed failed/unrelated durable history")
					}
					if len(after) != 4 || after["pve-snapshot:site-node-b-202-fresh-peer"].ID == "" {
						t.Fatalf("unaccepted rows ingested or accepted peer lost: %+v", after)
					}
					if !cold {
						rows := snapshotCollectionByID(h.f.m)
						if rows["site-node-a-101-weekly"] != beforeRows["site-node-a-101-weekly"] || rows["other-site-node-a-101-weekly"] != beforeRows["other-site-node-a-101-weekly"] {
							t.Fatal("invalid success changed retained live membership")
						}
						if got := snapshotCollectionAlert(t, h.f.m, "site:node-a:101"); got.ID != warning.ID || !got.StartTime.Equal(warning.StartTime) || !got.Acknowledged || !reflect.DeepEqual(got.Metadata["snapshotCreatedAt"], warning.Metadata["snapshotCreatedAt"]) {
							t.Fatalf("invalid success changed warning occurrence/acknowledgement: before=%+v after=%+v", warning, got)
						}
					}
					h.set("node-a", snapshotInventoryBody("recovered", time.Now().Add(-2*time.Hour)))
					h.poll(t, "site")
					after = h.f.points(t)
					if len(after) != 3 || after["pve-snapshot:site-node-a-101-recovered"].ID == "" || !reflect.DeepEqual(before[otherID], after[otherID]) {
						t.Fatal("complete recovery failed to replace only its own snapshots")
					}
					h.set("node-a", `{"data":[]}`)
					h.set("node-b", `{"data":[{"name":"current"}]}`)
					h.poll(t, "site")
					after = h.f.points(t)
					if len(after) != 1 || !reflect.DeepEqual(before[otherID], after[otherID]) {
						t.Fatal("explicit empty/current-only listing failed to clear only its scope")
					}
					for _, alert := range h.f.m.GetState().ActiveAlerts {
						if strings.HasPrefix(alert.ResourceID, "site:") && alert.Type == "snapshot-age" {
							t.Fatal("confirmed empty inventory retained snapshot warning")
						}
					}
				})
			}
		}
	}
}

// Opt-in JSON uses the same HTTP reader, poller, queue and public store API as
// the regression. It is a producer for the production consuming/browser proof.
func TestPVESnapshotHTTPBrowserProducer(t *testing.T) {
	h := newSnapshotInventoryHTTPFixture(t, "qemu")
	h.poll(t, "other-site")
	type frame struct {
		State  models.StateSnapshot     `json:"state"`
		Points []recovery.RecoveryPoint `json:"points"`
	}
	frames := map[string]frame{}
	for _, phase := range []string{"observed", "missing-data", "invalid-row", "recovered", "confirmed-empty"} {
		switch phase {
		case "missing-data":
			h.set("node-a", `{}`)
		case "invalid-row":
			h.set("node-a", `{"data":[{"name":"unaccepted","snaptime":1700000000},null]}`)
		case "recovered":
			h.set("node-a", snapshotInventoryBody("recovered", time.Now().Add(-time.Hour)))
		case "confirmed-empty":
			h.set("node-a", `{"data":[]}`)
			h.set("node-b", `{"data":[{"name":"current"}]}`)
		}
		if phase == "missing-data" {
			h.set("node-b", snapshotInventoryBody("fresh-peer", time.Now().Add(-2*time.Hour)))
		}
		h.poll(t, "site")
		points, _, err := h.f.store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 1000})
		if err != nil {
			t.Fatal(err)
		}
		state := h.f.m.GetState()
		wantPoints := map[string]int{"observed": 3, "missing-data": 4, "invalid-row": 4, "recovered": 3, "confirmed-empty": 1}[phase]
		if len(points) != wantPoints {
			t.Fatalf("phase %s: got %d points, want %d", phase, len(points), wantPoints)
		}
		frames[phase] = frame{State: state, Points: points}
	}
	if os.Getenv("PULSE_SNAPSHOT_HTTP_FRAMES") == "1" {
		data, err := json.Marshal(frames)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("SNAPSHOT_HTTP_FRAMES=%s\n", data)
	}
}
