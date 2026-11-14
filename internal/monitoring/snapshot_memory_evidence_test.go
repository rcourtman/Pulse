package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Exercise the real HTTP readers and collector, then the existing agent read.
// The only provider operations are snapshot inventories. Sizing is an empty
// unrelated fixture; no QGA, backup execution or recovery database is used.
type snapshotMemoryPoints []recovery.RecoveryPoint

func (p snapshotMemoryPoints) ListPoints(_ context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	if opts.Provider != recovery.ProviderProxmoxPVE || opts.Kind != recovery.KindSnapshot || opts.Page != 1 {
		return nil, len(p), nil
	}
	return p, len(p), nil
}

func testSnapshotMemoryEvidenceReachesExistingConsumers(t *testing.T) {
	m, _ := snapshotPoolMonitor(2)
	var mu sync.Mutex
	body := ""
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests++
		if r.URL.Path != "/api2/json/nodes/node/qemu/1000/snapshot" && r.URL.Path != "/api2/json/nodes/node/lxc/1001/snapshot" {
			t.Errorf("unexpected provider operation: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	api, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	client := &snapshotInventoryHTTPClient{api: api}
	poll := func(response string) []models.GuestSnapshot {
		t.Helper()
		mu.Lock()
		body, requests = response, 0
		mu.Unlock()
		backupPollingCompleteRead(t, m, func(m *Monitor) { m.pollGuestSnapshots(context.Background(), "site", client) })
		mu.Lock()
		count := requests
		mu.Unlock()
		if count != 2 {
			t.Errorf("provider reads = %d, want one per guest with no command/retry", count)
		}
		return m.PVEBackupsSnapshot().GuestSnapshots
	}

	for _, tc := range []struct {
		name, flag string
		memory     bool
	}{
		{"explicit-memory", `,"vmstate":1`, true},
		{"disk-only", `,"vmstate":0`, false},
		{"omitted-legacy", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := poll(`{"data":[{"name":"weekly","snaptime":1700000000,"description":"accepted","parent":"base"` + tc.flag + `}]}`)
			if len(rows) != 3 {
				t.Fatalf("target snapshots plus unchanged peer = %d, want 3", len(rows))
			}
			want := make(map[string]bool)
			for _, row := range rows {
				if row.Instance == "peer" {
					if row.Name != "independent" || row.VMState {
						t.Errorf("peer evidence changed: %+v", row)
					}
					continue
				}
				memory := tc.memory && row.Type == "qemu"
				want[row.ID] = memory
				if row.VMState != memory || row.Name != "weekly" || row.Time.Unix() != 1700000000 || row.Description != "accepted" || row.Parent != "base" {
					t.Errorf("%s memory=%v, want %v with original metadata: %+v", tc.name, row.VMState, memory, row)
				}
			}

			// PVEBackupsSnapshot supplies the public backup endpoint and state
			// stream. False must remain an explicit boolean, not an omitted key.
			wire, err := json.Marshal(m.PVEBackupsSnapshot())
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				GuestSnapshots []map[string]json.RawMessage `json:"guestSnapshots"`
			}
			if err := json.Unmarshal(wire, &payload); err != nil {
				t.Fatal(err)
			}
			for _, row := range payload.GuestSnapshots {
				var id string
				if err := json.Unmarshal(row["id"], &id); err != nil {
					t.Fatal(err)
				}
				if expected, present := want[id]; present && string(row["vmstate"]) != fmt.Sprint(expected) {
					t.Errorf("public snapshot %s vmstate = %s, want %v", id, row["vmstate"], expected)
				}
			}

			// Round-trip mapper output through JSON like the persisted reader,
			// without opening a store, then invoke the existing public tool.
			points := proxmoxmapper.FromPVEGuestSnapshots(rows, nil)
			pointJSON, err := json.Marshal(points)
			if err != nil {
				t.Fatal(err)
			}
			var readback snapshotMemoryPoints
			if err := json.Unmarshal(pointJSON, &readback); err != nil {
				t.Fatal(err)
			}
			executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{StateProvider: m, RecoveryPointsProvider: readback})
			result, err := executor.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{"type": "snapshots", "instance": "site"})
			if err != nil || result.IsError || len(result.Content) != 1 {
				t.Fatalf("existing snapshots read: result=%+v err=%v", result, err)
			}
			var response tools.SnapshotsResponse
			if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Snapshots) != len(want) {
				t.Fatalf("agent returned %d snapshots, want %d", len(response.Snapshots), len(want))
			}
			for _, row := range response.Snapshots {
				expected, present := want[row.ID]
				if !present || row.VMState != expected || row.SnapshotName != "weekly" || row.Description != "accepted" || row.Instance != "site" || row.Time.Unix() != 1700000000 {
					t.Errorf("agent snapshot lost evidence or scope: %+v; want memory %v", row, expected)
				}
				delete(want, row.ID)
			}
			if len(want) != 0 {
				t.Errorf("agent omitted target snapshots: %v", want)
			}
			if os.Getenv("PULSE_SNAPSHOT_MEMORY_EXPORT") == "1" {
				t.Logf("SNAPSHOT_MEMORY=%s", wire)
				t.Logf("SNAPSHOT_MEMORY_AGENT=%s", result.Content[0].Text)
			}
		})
	}

	// A malformed success is unread, not a replacement memory verdict or
	// removal. Recovery resumes on the next ordinary accepted inventory.
	previous := poll(`{"data":[{"name":"weekly","snaptime":1700000000,"vmstate":1}]}`)
	byID := func(rows []models.GuestSnapshot) map[string]models.GuestSnapshot {
		indexed := make(map[string]models.GuestSnapshot, len(rows))
		for _, row := range rows {
			indexed[row.ID] = row
		}
		return indexed
	}
	// State sorts snapshots by creation time; equal-time guests can come
	// back in either map iteration order. Compare every field by exact ID,
	// not their incidental order, without hiding duplicates or missing rows.
	wantPrevious := byID(previous)
	for _, flag := range []string{`null`, `2`, `true`, `"1"`, `1,"VMState":0`} {
		rows := poll(`{"data":[{"name":"would-replace","snaptime":1700000100,"vmstate":` + flag + `}]}`)
		indexed := byID(rows)
		if len(rows) != len(previous) || len(indexed) != len(rows) || !reflect.DeepEqual(indexed, wantPrevious) {
			t.Errorf("ambiguous vmstate %s replaced accepted evidence: %+v, want %+v", flag, rows, previous)
		}
	}
	rows := poll(`{"data":[{"name":"recovered","snaptime":1700000200,"vmstate":0}]}`)
	for _, row := range rows {
		if row.Instance == "site" && (row.Name != "recovered" || row.VMState) {
			t.Errorf("ordinary disk-only read did not recover: %+v", row)
		}
	}
	rows = poll(`{"data":[]}`)
	if len(rows) != 1 || rows[0].Instance != "peer" {
		t.Errorf("accepted scoped removal changed the peer or retained targets: %+v", rows)
	}
}
