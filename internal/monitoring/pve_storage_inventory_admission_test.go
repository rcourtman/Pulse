package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A complete empty sibling cannot erase an unreadable local archive. Exercise
// ordinary HTTP inventory reads, production replacement and recovery mapping;
// never send a guest command, create a backup or change an installed service.
func TestPVEStorageInventoryAdmissionPreservesUnreadableCopies(t *testing.T) {
	for _, endpoint := range []string{"storage-list", "content"} {
		for _, failure := range []string{"missing", "null", "competing-errors", "duplicate-data", "case-data", "trailing-json", "short-body", "invalid-row"} {
			for _, reversed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/reversed=%t", endpoint, failure, reversed), func(t *testing.T) {
					const instance = "fixture-pve"
					birth := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
					var phase, unexpected atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!read=fixture" {
							unexpected.Add(1)
							http.Error(w, "unexpected method or authentication", http.StatusMethodNotAllowed)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						p := phase.Load()
						isB := strings.Contains(r.URL.Path, "/archive-b/")
						isList := strings.HasSuffix(r.URL.Path, "/storage")
						isContent := strings.HasSuffix(r.URL.Path, "/storage/archive/content")
						if !isList && !isContent {
							unexpected.Add(1)
							http.NotFound(w, r)
							return
						}
						if isB && (p == 1 || p == 2) && ((endpoint == "storage-list" && isList) || (endpoint == "content" && isContent)) {
							body := map[string]string{
								"missing": `{}`, "null": `{"data":null}`,
								"competing-errors": `{"data":[],"errors":{"detail":"synthetic-unavailable"}}`,
								"duplicate-data":   `{"data":[{}],"data":[]}`,
								"case-data":        `{"data":[{}],"DATA":[]}`,
								"trailing-json":    `{"data":[]}{"ignored":true}`,
								"short-body":       `{"data":[]}`, "invalid-row": `{"data":[{}]}`,
							}[failure]
							if failure == "short-body" {
								w.Header().Set("Content-Length", "1000")
							}
							fmt.Fprint(w, body)
							return
						}
						if isList {
							fmt.Fprint(w, `{"data":[{"storage":"archive","type":"dir","content":"backup","enabled":1,"active":1,"shared":0}]}`)
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
						fmt.Fprintf(w, `{"data":[{"volid":"archive:backup/vzdump-qemu-101-copy.vma.zst","content":"backup","format":"vma.zst","vmid":101,"ctime":%d,"size":%d}]}`, at.Unix(), size)
					}))
					defer server.Close()
					client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture", Timeout: 5 * time.Second})
					if err != nil {
						t.Fatal(err)
					}
					m := &Monitor{state: models.NewState(), config: &config.Config{}}
					m.state.UpdateGuestsForInstance(instance, []models.VM{{ID: "fixture-pve:pve-a:101", Name: "recovery-vm", Instance: instance, Node: "pve-a", VMID: 101, Status: "running"}}, nil)
					peer := models.StorageBackup{ID: "independent-connection", Storage: "archive", StorageNode: "archive-b", Instance: "other", Node: "other-node", VMID: 101, Type: "qemu", Time: birth, CTime: birth.Unix(), Size: 8192}
					m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
					nodes := []proxmox.Node{{Node: "archive-a"}, {Node: "archive-b"}}
					if reversed {
						nodes[0], nodes[1] = nodes[1], nodes[0]
					}
					originals := map[string]models.StorageBackup{}
					for index, label := range []string{"initial", "unreadable-b", "empty-a-unreadable-b", "recovered-b", "empty-all"} {
						phase.Store(int32(index))
						m.pollStorageBackupsWithNodes(context.Background(), instance, client, nodes, map[string]string{"archive-a": "online", "archive-b": "online"})
						all := m.PVEBackupsSnapshot()
						var copies []models.StorageBackup
						foundPeer := false
						for _, b := range all.StorageBackups {
							if b.Instance == "other" {
								foundPeer = reflect.DeepEqual(b, peer)
								continue
							}
							copies = append(copies, b)
							if index == 0 {
								originals[b.StorageNode] = b
							} else if index <= 2 && b.StorageNode == "archive-b" && !reflect.DeepEqual(b, originals[b.StorageNode]) {
								t.Errorf("%s renewed or changed held archive: before=%+v after=%+v", label, originals[b.StorageNode], b)
							}
							if index >= 2 && (b.StorageNode != "archive-b" || b.Node != "pve-a") {
								t.Errorf("%s crossed storage/guest scope: %+v", label, b)
							}
							if index == 3 && (b.Size != 4096 || !b.Time.Equal(birth.Add(time.Hour)) || b.ID != originals["archive-b"].ID) {
								t.Errorf("ordinary complete recovery did not refresh same archive: %+v", b)
							}
						}
						want := []int{2, 2, 1, 1, 0}[index]
						points := proxmoxmapper.FromPVEStorageBackups(copies, nil)
						if len(copies) != want || len(points) != want || !foundPeer {
							t.Errorf("%s: got %d archive copies/%d recovery projections, want %d; independent connection retained=%t", label, len(copies), len(points), want, foundPeer)
						}
						if failure == "null" && !reversed {
							wire, err := json.Marshal(map[string]any{"endpoint": endpoint, "phase": label, "data": all, "points": points})
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("PVE_STORAGE_ADMISSION_PROJECTION=%s", wire)
						}
					}
					if unexpected.Load() != 0 {
						t.Fatal("inventory sent a guest, backup, mutation or unexpected request")
					}
				})
			}
		}
	}
}
