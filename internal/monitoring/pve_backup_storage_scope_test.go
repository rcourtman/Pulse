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
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Two local archives can have the same volume ID after a copy or migration.
// They are independent recovery locations, unlike two views of shared storage.
// Exercise only ordinary inventory GETs; no guest or backup command is sent.
func TestPVELocalBackupCopiesKeepIndependentIdentity(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			var reads, unexpected atomic.Int32
			birth := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					unexpected.Add(1)
					http.Error(w, "unexpected mutation", http.StatusMethodNotAllowed)
					return
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/storage"):
					flag := 0
					if shared {
						flag = 1
					}
					fmt.Fprintf(w, `{"data":[{"storage":"archive","type":"dir","content":"backup","enabled":1,"active":1,"shared":%d}]}`, flag)
				case strings.HasSuffix(r.URL.Path, "/storage/archive/content"):
					reads.Add(1)
					size := 1024
					if strings.Contains(r.URL.Path, "/node-b/") {
						size = 2048
					}
					fmt.Fprintf(w, `{"data":[{"volid":"archive:backup/vzdump-qemu-100-copy.vma.zst","content":"backup","format":"vma.zst","vmid":100,"ctime":%d,"size":%d}]}`, birth.Unix(), size)
				default:
					unexpected.Add(1)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture"})
			if err != nil {
				t.Fatal(err)
			}
			m := &Monitor{state: models.NewState(), config: &config.Config{}}
			// Guest placement is deliberately different from the archive hosts.
			m.state.UpdateGuestsForInstance("estate", []models.VM{{ID: "estate:guest-node:100", Instance: "estate", Node: "guest-node", VMID: 100, Name: "database", Status: "running"}}, nil)
			m.pollStorageBackupsWithNodes(context.Background(), "estate", client,
				[]proxmox.Node{{Node: "node-a"}, {Node: "node-b"}}, map[string]string{"node-a": "online", "node-b": "online"})
			backups := m.state.GetSnapshot().PVEBackups.StorageBackups
			want := 2
			if shared {
				want = 1
			}
			if len(backups) != want {
				t.Fatalf("independent storage locations collapsed: got %d backups, want %d: %+v", len(backups), want, backups)
			}
			ids := make(map[string]bool)
			for _, b := range backups {
				if ids[b.ID] || b.Node != "guest-node" || !b.Time.Equal(birth) {
					t.Fatalf("archive identity changed guest placement or observation: %+v", backups)
				}
				ids[b.ID] = true
			}
			points := proxmoxmapper.FromPVEStorageBackups(backups, nil)
			if len(points) != want || (want == 2 && points[0].ID == points[1].ID) {
				t.Fatalf("recovery point identities collapsed: %+v", points)
			}
			if reads.Load() != 2 || unexpected.Load() != 0 {
				t.Fatalf("unexpected inventory or command: reads=%d unexpected=%d", reads.Load(), unexpected.Load())
			}
			wire, err := json.Marshal(map[string]any{"shared": shared, "backups": backups, "points": points})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("PVE_STORAGE_SCOPE_PROJECTION=%s", wire)
		})
	}
}

// A successful empty read from A can delete only A's local archives, even
// when B's same-named storage is unreadable. Shared stores keep the opposite
// rule: a complete view from either node is authoritative for that store.
func TestPVEBackupStorageScopeFailureAndRecovery(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, failure := range []string{"content", "storage-list", "offline"} {
			for _, reversed := range []bool{false, true} {
				t.Run(fmt.Sprintf("shared=%t/%s/reversed=%t", shared, failure, reversed), func(t *testing.T) {
					m, manager := recoveryIngestTestMonitor(t)
					m.state, m.config = models.NewState(), &config.Config{}
					birth := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
					var phase, unexpected atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						if r.Method != http.MethodGet {
							unexpected.Add(1)
							http.Error(w, "unexpected mutation", http.StatusMethodNotAllowed)
							return
						}
						p := phase.Load()
						isB := strings.Contains(r.URL.Path, "/node-b/")
						failed := isB && (p == 1 || p == 2)
						switch {
						case strings.HasSuffix(r.URL.Path, "/storage"):
							if failed && failure == "storage-list" {
								http.Error(w, "synthetic denied inventory", http.StatusForbidden)
								return
							}
							flag := 0
							if shared {
								flag = 1
							}
							fmt.Fprintf(w, `{"data":[{"storage":"archive","type":"dir","content":"backup","enabled":1,"active":1,"shared":%d}]}`, flag)
						case strings.HasSuffix(r.URL.Path, "/storage/archive/content"):
							if failed && failure == "content" {
								http.Error(w, "synthetic denied inventory", http.StatusForbidden)
								return
							}
							if p == 4 || (!isB && p >= 2) {
								fmt.Fprint(w, `{"data":[]}`)
								return
							}
							size, at := 1024, birth
							if isB {
								size = 2048
							}
							if p == 3 {
								size, at = 4096, birth.Add(time.Hour)
							}
							fmt.Fprintf(w, `{"data":[{"volid":"archive:backup/vzdump-qemu-100-copy.vma.zst","content":"backup","format":"vma.zst","vmid":100,"ctime":%d,"size":%d}]}`, at.Unix(), size)
						default:
							unexpected.Add(1)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture"})
					if err != nil {
						t.Fatal(err)
					}
					waitForIngest := func() {
						t.Helper()
						deadline := time.Now().Add(10 * time.Second)
						for {
							m.recoveryIngestMu.Lock()
							pending := m.recoveryIngestRunning
							m.recoveryIngestMu.Unlock()
							if !pending {
								return
							}
							if time.Now().After(deadline) {
								t.Fatal("inventory ingestion did not settle")
							}
							time.Sleep(time.Millisecond)
						}
					}
					t.Cleanup(waitForIngest)
					// Another connection's same-named storage is outside this poll.
					peer := models.StorageBackup{ID: "peer-point", Storage: "archive", StorageNode: "node-b", Node: "peer-guest", Instance: "other", VMID: 100, Type: "qemu", Time: birth, CTime: birth.Unix(), Size: 8192}
					m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
					store, err := manager.StoreForOrg("default")
					if err != nil {
						t.Fatal(err)
					}
					if err := store.UpsertPoints(context.Background(), proxmoxmapper.FromPVEStorageBackups([]models.StorageBackup{peer}, nil)); err != nil {
						t.Fatal(err)
					}
					nodes := []proxmox.Node{{Node: "node-a"}, {Node: "node-b"}}
					if reversed {
						nodes[0], nodes[1] = nodes[1], nodes[0]
					}
					originalIDs := map[string]bool{}
					for index, label := range []string{"initial", "failed-b", "empty-a-failed-b", "recovered-b", "empty-all"} {
						phase.Store(int32(index))
						guestNode := "guest-node"
						if index > 0 {
							guestNode = "moved-guest"
						}
						m.state.UpdateGuestsForInstance("estate", []models.VM{{ID: "estate:" + guestNode + ":100", Instance: "estate", Node: guestNode, VMID: 100, Name: "database", Status: "running"}}, nil)
						status := map[string]string{"node-a": "online", "node-b": "online"}
						if failure == "offline" && (index == 1 || index == 2) {
							status["node-b"] = "offline"
						}
						m.pollStorageBackupsWithNodes(context.Background(), "estate", client, nodes, status)
						waitForIngest()
						state := m.GetState()
						want := 2
						if shared || index >= 2 {
							want = 1
						}
						if index == 4 || (shared && index == 2) {
							want = 0
						}
						found := 0
						for _, b := range state.PVEBackups.StorageBackups {
							if b.Instance == "other" {
								if b != peer {
									t.Fatal("collection replaced independent connection evidence")
								}
								continue
							}
							found++
							if b.Node != guestNode || (shared && b.StorageNode != "") || (!shared && b.StorageNode == "") {
								t.Fatalf("%s: storage location moved with the guest: %+v", label, b)
							}
							if index == 0 {
								originalIDs[b.ID] = true
							} else if !originalIDs[b.ID] {
								t.Fatalf("%s: guest movement or failed read replaced archive identity: %+v", label, b)
							}
							if index == 2 && (b.StorageNode != "node-b" || b.Size != 2048 || !b.Time.Equal(birth)) {
								t.Fatalf("failed B was cleared or refreshed by empty A: %+v", b)
							}
							if index == 3 && (b.Size != 4096 || !b.Time.Equal(birth.Add(time.Hour))) {
								t.Fatalf("ordinary recovery did not replace held B: %+v", b)
							}
						}
						points, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{})
						if err != nil || found != want || len(points) != want+1 || len(state.PVEBackups.StorageBackups) != want+1 {
							t.Fatalf("%s: state/store diverged: want %d estate archives; state=%+v points=%+v err=%v", label, want, state.PVEBackups.StorageBackups, points, err)
						}
						for _, p := range points {
							if p.Details["instance"] == "estate" && (p.SubjectRef == nil || p.SubjectRef.Class != guestNode || (!shared && p.RepositoryRef.Class != p.Details["storageNode"])) {
								t.Fatalf("%s: recovery confused guest and storage location: %+v", label, p)
							}
						}
						if !shared && failure == "content" && !reversed {
							wire, err := json.Marshal(map[string]any{"phase": label, "data": m.PVEBackupsSnapshot(), "points": points})
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("PVE_STORAGE_LIFECYCLE_PROJECTION=%s", wire)
						}
					}
					if unexpected.Load() != 0 {
						t.Fatal("inventory issued a guest, backup or unexpected request")
					}
				})
			}
		}
	}
}
