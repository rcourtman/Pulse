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
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Only storage catalog/content reads go through the real HTTP client. No guest
// commands, backup operation, external service or database-internal change.
type storageInventoryHTTPClient struct {
	*pveReconciliationClient
	api *proxmox.Client
}

func (c *storageInventoryHTTPClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	return c.api.GetStorage(ctx, node)
}

func (c *storageInventoryHTTPClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	return c.api.GetStorageContent(ctx, node, storage)
}

type storageInventoryHTTPFixture struct {
	f      *pveReconciliationFixture
	client *storageInventoryHTTPClient
	mu     sync.Mutex
	bodies map[string]string
	paths  []string
}

func storageInventoryHTTPPath(kind, node string) string {
	path := "/api2/json/nodes/" + node + "/storage"
	if kind == "content" {
		path += "/archive-" + node + "/content"
	}
	return path
}

func newStorageInventoryHTTPFixture(t *testing.T) *storageInventoryHTTPFixture {
	t.Helper()
	h := &storageInventoryHTTPFixture{f: newPVEReconciliationFixture(t), bodies: make(map[string]string)}
	// Normal storage inventory supplies the carry-forward lookup when a node's
	// catalog cannot be read. Cold-state cases deliberately discard that cache.
	h.f.m.state.UpdateStorage([]models.Storage{
		{ID: "cluster-node-a-archive-node-a", Name: "archive-node-a", Node: "node-a", Instance: "cluster", Content: "backup", Type: "dir"},
		{ID: "cluster-node-b-archive-node-b", Name: "archive-node-b", Node: "node-b", Instance: "cluster", Content: "backup", Type: "dir"},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.paths = append(h.paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if body, ok := h.bodies[r.URL.Path]; ok {
			fmt.Fprint(w, body)
			return
		}
		for _, node := range []string{"node-a", "node-b"} {
			var data any
			if r.URL.Path == storageInventoryHTTPPath("catalog", node) {
				data, _ = h.f.client.GetStorage(r.Context(), node)
			} else if r.URL.Path == storageInventoryHTTPPath("content", node) {
				data, _ = h.f.client.GetStorageContent(r.Context(), node, "archive-"+node)
			} else {
				continue
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
				t.Error(err)
			}
			return
		}
		t.Errorf("unexpected fixture request: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	api, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	h.client = &storageInventoryHTTPClient{pveReconciliationClient: h.f.client, api: api}
	return h
}

func (h *storageInventoryHTTPFixture) set(kind, node, body string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if body == "" {
		delete(h.bodies, storageInventoryHTTPPath(kind, node))
	} else {
		h.bodies[storageInventoryHTTPPath(kind, node)] = body
	}
}

func (h *storageInventoryHTTPFixture) advance() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.f.client.at = h.f.client.at.Add(15 * time.Minute)
}

func (h *storageInventoryHTTPFixture) poll(t *testing.T, wantReads int) {
	t.Helper()
	h.mu.Lock()
	before := len(h.paths)
	h.mu.Unlock()
	h.f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", h.client, h.f.nodes, h.f.online)
	h.f.wait(t)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.paths)-before != wantReads {
		t.Fatalf("got HTTP reads %v, want %d; no retries or other operations", h.paths[before:], wantReads)
	}
}

func TestPVEStorageHTTPInvalidResponseRetainsHistory(t *testing.T) {
	for _, kind := range []string{"catalog", "content"} {
		row := `{"storage":"archive-node-b","type":"dir","content":"backup","enabled":1,"active":1}`
		missingDiscriminator := `{"storage":"archive-node-b","type":"dir"}`
		if kind == "content" {
			row = `{"volid":"archive-node-b:backup/vzdump-qemu-102.vma.zst","vmid":102,"content":"backup","ctime":1700000000}`
			missingDiscriminator = `{"volid":"archive-node-b:backup/vzdump-qemu-102.vma.zst"}`
		}
		cases := map[string]string{
			"missing-data":          `{}`,
			"null-data":             `{"data":null}`,
			"conflicting-error":     `{"data":[],"errors":{"message":"fixture-private-detail"}}`,
			"duplicate-data":        `{"data":[` + row + `],"data":[]}`,
			"trailing-value":        `{"data":[]} {"data":[]}`,
			"null-row":              `{"data":[null]}`,
			"bad-row-after-good":    `{"data":[` + row + `,null]}`,
			"missing-discriminator": `{"data":[` + missingDiscriminator + `]}`,
		}
		for _, cold := range []bool{false, true} {
			for name, body := range cases {
				t.Run(fmt.Sprintf("%s/cold=%t/%s", kind, cold, name), func(t *testing.T) {
					h := newStorageInventoryHTTPFixture(t)
					f := h.f
					h.poll(t, 4)
					other := recoveryIngestTestPoint("pve-backup:unrelated", "other-site", f.client.at)
					if err := f.store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{other}); err != nil {
						t.Fatal(err)
					}
					before := f.points(t)
					if len(before) != 3 {
						t.Fatalf("healthy control points=%d, want 3", len(before))
					}
					beforeRows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
					if cold {
						f.coldState()
					}
					h.advance()
					h.set(kind, "node-b", body)
					reads := 4
					if kind == "catalog" {
						reads = 3
					}
					for range 2 {
						h.poll(t, reads)
						after := f.points(t)
						if len(after) != 3 || !reflect.DeepEqual(before[other.ID], after[other.ID]) {
							t.Fatal("malformed success deleted history, admitted extra rows or changed another instance")
						}
						for id, point := range before {
							if point.SubjectResourceID == f.ids[1] && !reflect.DeepEqual(point, after[id]) {
								t.Fatal("unreadable backup was removed or its observation renewed")
							}
							if point.SubjectResourceID == f.ids[0] && (after[id].CompletedAt == nil || !after[id].CompletedAt.Equal(f.client.at)) {
								t.Fatal("independent accepted storage did not update")
							}
						}
						f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
						rows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
						want := 2
						if cold {
							want = 1
						}
						if len(rows) != want {
							t.Fatalf("live rows=%d want=%d; unread history cannot erase warm rows or invent cold ones", len(rows), want)
						}
						if !cold {
							for _, row := range rows {
								if row.VMID == 102 && !reflect.DeepEqual(row, beforeRows[1]) {
									t.Fatal("unread HTTP inventory changed its warm row")
								}
							}
						}
					}
					h.set(kind, "node-b", "")
					h.poll(t, 4)
					f.assertHistory(t, recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient)
					if len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 2 {
						t.Fatal("accepted recovery did not restore live inventory")
					}
					// Explicit scoped absence still clears only this connection.
					h.set(kind, "node-a", `{"data":[]}`)
					h.set(kind, "node-b", `{"data":[]}`)
					reads = 4
					if kind == "catalog" {
						reads = 2
					}
					h.poll(t, reads)
					if points := f.points(t); len(points) != 1 || !reflect.DeepEqual(before[other.ID], points[other.ID]) || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 {
						t.Fatal("explicit empty response failed to clear exactly its own backup scope")
					}
				})
			}
		}
	}
}

func TestPVEStorageHTTPAllUnreadCannotClaimEmpty(t *testing.T) {
	for _, kind := range []string{"catalog", "content"} {
		for _, cold := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cold=%t", kind, cold), func(t *testing.T) {
				h := newStorageInventoryHTTPFixture(t)
				h.poll(t, 4)
				before := h.f.points(t)
				if cold {
					h.f.coldState()
				}
				for _, node := range []string{"node-a", "node-b"} {
					h.set(kind, node, `{"data":null}`)
				}
				reads := 4
				if kind == "catalog" {
					reads = 2
				}
				for range 2 {
					h.poll(t, reads)
					if !reflect.DeepEqual(before, h.f.points(t)) {
						t.Fatal("all-unread response removed or renewed recovery evidence")
					}
					h.f.assertHistory(t, recovery.ProtectionHistoryUnavailable, operationaltrust.EvidencePermissionsUnknown)
				}
			})
		}
	}
}

// The browser consumes these actual HTTP -> poll -> queue -> store -> posture
// frames, not a separately invented provider state. Export stays opt-in.
func TestPVEStorageHTTPBrowserProducer(t *testing.T) {
	if os.Getenv("PULSE_STORAGE_HTTP_FRAMES") != "1" {
		t.Skip("producer export is opt-in")
	}
	h := newStorageInventoryHTTPFixture(t)
	frames := make(map[string]any)
	for _, phase := range []string{"observed", "catalog-unread", "content-unread", "cold-unread", "recovered", "confirmed-empty"} {
		switch phase {
		case "catalog-unread":
			h.set("catalog", "node-b", `{}`)
			h.advance()
		case "content-unread":
			h.set("catalog", "node-b", "")
			h.set("content", "node-b", `{"data":[{"volid":"unaccepted","content":"backup"},null]}`)
		case "cold-unread":
			h.f.coldState()
		case "recovered":
			h.set("content", "node-b", "")
		case "confirmed-empty":
			h.set("content", "node-a", `{"data":[]}`)
			h.set("content", "node-b", `{"data":[]}`)
		}
		reads := 4
		if phase == "catalog-unread" {
			reads = 3
		}
		h.poll(t, reads)
		points := h.f.points(t)
		want := 2
		if phase == "confirmed-empty" {
			want = 0
		}
		if len(points) != want {
			t.Fatalf("phase %s points=%d want=%d", phase, len(points), want)
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": h.f.m.state.GetSnapshot().PVEBackups},
			"postures": map[string]any{"data": h.f.postures(t), "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points}
	}
	data, err := json.Marshal(map[string]any{"observed_at": time.Now().UTC(), "resource_ids": h.f.ids, "frames": frames, "paths": h.paths})
	if err != nil {
		t.Fatal(err)
	}
	// One Go-JSON event may hold only part of this line; collectors must join
	// all Output events before extracting the producer frame.
	t.Log("STORAGE_HTTP_FRAMES=" + strings.TrimSpace(string(data)))
}
