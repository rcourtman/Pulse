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
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Disabled storage cannot be queried, but that does not establish that its
// archives were removed. These are synthetic, read-only HTTP inventories;
// no guest command, backup, appliance or installed service is involved.
func TestPVEDisabledBackupStorageRetainsKnownEvidence(t *testing.T) {
	for _, layout := range []string{"local", "shared", "sole"} {
		for _, omitted := range []bool{false, true} {
			for _, reversed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/omitted=%t/reversed=%t", layout, omitted, reversed), func(t *testing.T) {
					const instance = "fixture-pve"
					birth := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
					m, manager := recoveryIngestTestMonitor(t)
					m.state, m.config = models.NewState(), &config.Config{}
					m.state.UpdateGuestsForInstance(instance, []models.VM{{ID: "fixture-pve:pve-a:101", Instance: instance, Node: "pve-a", VMID: 101, Name: "recovery-vm", Status: "running"}}, nil)
					store, err := manager.StoreForOrg("default")
					if err != nil {
						t.Fatal(err)
					}
					wait := func() {
						t.Helper()
						deadline := time.Now().Add(10 * time.Second)
						for {
							m.recoveryIngestMu.Lock()
							running := m.recoveryIngestRunning
							m.recoveryIngestMu.Unlock()
							if !running {
								return
							}
							if time.Now().After(deadline) {
								t.Fatal("ordinary recovery ingestion did not settle")
							}
							time.Sleep(time.Millisecond)
						}
					}
					t.Cleanup(wait)
					peer := models.StorageBackup{ID: "independent-connection", Storage: "archive", StorageNode: "archive-b", Instance: "other", Node: "other-node", VMID: 101, Type: "qemu", Time: birth, CTime: birth.Unix(), Size: 8192}
					m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
					var phase, unexpected, skippedReads atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!read=fixture" {
							unexpected.Add(1)
							http.Error(w, "unexpected method or authentication", 405)
							return
						}
						p := phase.Load()
						isB := strings.Contains(r.URL.Path, "/archive-b/")
						disabled := (isB && p >= 1 && p <= 3) || p == 3
						w.Header().Set("Content-Type", "application/json")
						switch {
						case strings.HasSuffix(r.URL.Path, "/storage"):
							if p == 7 {
								fmt.Fprint(w, `{"data":[]}`)
								return
							}
							shared := 0
							if layout == "shared" {
								shared = 1
							}
							enabled := `,"enabled":1`
							if disabled {
								enabled = `,"enabled":0`
								if omitted {
									enabled = ""
								}
							}
							fmt.Fprintf(w, `{"data":[{"storage":"archive","type":"dir","content":"backup","active":1,"shared":%d%s}]}`, shared, enabled)
						case strings.HasSuffix(r.URL.Path, "/storage/archive/content"):
							if disabled {
								skippedReads.Add(1)
								http.Error(w, "disabled store must not be queried", 500)
								return
							}
							if p == 5 || (!isB && p >= 2 && p != 6) {
								fmt.Fprint(w, `{"data":[]}`)
								return
							}
							size, at := 1024, birth
							if isB {
								size = 2048
							}
							if p == 4 || p == 6 {
								size, at = 4096, birth.Add(30*time.Minute)
							}
							fmt.Fprintf(w, `{"data":[{"volid":"archive:backup/vzdump-qemu-101-copy.vma.zst","content":"backup","format":"vma.zst","vmid":101,"ctime":%d,"size":%d}]}`, at.Unix(), size)
						default:
							unexpected.Add(1)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture", Timeout: time.Second})
					if err != nil {
						t.Fatal(err)
					}
					nodes := []proxmox.Node{{Node: "archive-a"}, {Node: "archive-b"}}
					if layout == "sole" {
						nodes = nodes[1:]
					}
					if reversed && len(nodes) == 2 {
						nodes[0], nodes[1] = nodes[1], nodes[0]
					}
					subject := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, instance, 101)
					originals := map[string]models.StorageBackup{}
					for index, label := range []string{"initial", "disabled-b", "empty-a-disabled-b", "all-disabled", "reenabled-b", "empty-all", "restored", "removed-config"} {
						phase.Store(int32(index))
						m.pollStorageBackupsWithNodes(context.Background(), instance, client, nodes, map[string]string{"archive-a": "online", "archive-b": "online"})
						wait()
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
								originals[b.ID] = b
							}
							if index >= 1 && index <= 3 && (layout == "sole" || b.StorageNode == "archive-b") && !reflect.DeepEqual(b, originals[b.ID]) {
								t.Errorf("%s: disabled storage renewed/changed historical evidence: %+v", label, b)
							}
							if index == 4 && (b.Size != 4096 || !b.Time.Equal(birth.Add(30*time.Minute))) {
								t.Errorf("ordinary re-enable did not refresh known archive: %+v", b)
							}
						}
						want := []int{2, 2, 1, 1, 1, 0, 2, 0}[index]
						if layout == "shared" {
							want = []int{1, 1, 0, 0, 1, 0, 1, 0}[index]
						}
						if layout == "sole" {
							want = []int{1, 1, 1, 1, 1, 0, 1, 0}[index]
						}
						if len(copies) != want || !foundPeer {
							t.Errorf("%s: %d copies, want %d; independent connection retained=%t", label, len(copies), want, foundPeer)
						}
						points, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 100})
						if err != nil {
							t.Fatal(err)
						}
						// Incomplete provider reconciliation can retain older points, but must
						// never remove a held local/sole copy; complete empties may remove it.
						if (layout != "shared" && len(points) != want) || ((index == 5 || index == 7) && len(points) != 0) {
							t.Errorf("%s: persisted points=%d, expected %d", label, len(points), want)
						}
						postures, total, err := store.ListProtectionPostures(context.Background(), recovery.ProtectionPostureQuery{SubjectResourceIDs: []string{subject}})
						if err != nil || total != 1 {
							t.Fatalf("posture total=%d err=%v", total, err)
						}
						if index >= 1 && index <= 3 {
							for _, provider := range postures[0].ProviderStates {
								if provider.HistoryCompleteness == recovery.ProtectionHistoryComplete || provider.Permissions == "denied" {
									t.Errorf("%s: skipped content claimed complete history or rejected access: %+v", label, provider)
								}
							}
							if len(postures[0].ProviderStates) != 1 {
								t.Errorf("%s: missing unavailable/partial provider observation", label)
							}
						}
						if layout == "local" && !omitted && !reversed {
							wire, err := json.Marshal(map[string]any{"phase": label, "data": all, "points": points, "postures": postures, "now": time.Now().UTC(), "birth": birth, "subject": subject})
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("PVE_DISABLED_STORAGE_PROJECTION=%s", wire)
						}
					}
					if skippedReads.Load() != 0 || unexpected.Load() != 0 {
						t.Fatalf("queried disabled storage or sent an unexpected operation: disabled=%d unexpected=%d", skippedReads.Load(), unexpected.Load())
					}
				})
			}
		}
	}
}

// A healthy sibling must not withdraw PBS attribution merely because the
// source store was disabled. Re-enable and genuine empty reads still refresh
// and remove the association through the ordinary production reader.
func TestPVEDisabledPBSStorageRetainsAttribution(t *testing.T) {
	for _, omitted := range []bool{false, true} {
		t.Run(fmt.Sprint(omitted), func(t *testing.T) {
			birth := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
			var phase, unexpected atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p := phase.Load()
				if r.Method != http.MethodGet {
					unexpected.Add(1)
					http.Error(w, "unexpected method", 405)
					return
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/storage"):
					enabled := `,"enabled":1`
					if p == 1 {
						enabled = `,"enabled":0`
						if omitted {
							enabled = ""
						}
					}
					fmt.Fprintf(w, `{"data":[{"storage":"archive","type":"pbs","content":"backup","active":1%s},{"storage":"other","type":"dir","content":"backup","active":1,"enabled":1}]}`, enabled)
				case strings.HasSuffix(r.URL.Path, "/storage/archive/content"):
					if p == 1 {
						unexpected.Add(1)
						http.Error(w, "disabled store queried", 500)
						return
					}
					if p == 3 {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
					at := birth
					if p == 2 {
						at = birth.Add(30 * time.Minute)
					}
					fmt.Fprintf(w, `{"data":[{"volid":"archive:backup/vm/101/copy","content":"backup","format":"pbs-vm","vmid":101,"ctime":%d,"size":1024}]}`, at.Unix())
				case strings.HasSuffix(r.URL.Path, "/storage/other/content"):
					fmt.Fprint(w, `{"data":[]}`)
				default:
					unexpected.Add(1)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			m := &Monitor{state: models.NewState(), config: &config.Config{PBSInstances: []config.PBSInstance{{Name: "direct-pbs"}}}}
			var initial []models.PBSGuestConfirmation
			for index := 0; index < 4; index++ {
				phase.Store(int32(index))
				m.pollStorageBackupsWithNodes(context.Background(), "fixture-pve", client, []proxmox.Node{{Node: "pve-a"}}, map[string]string{"pve-a": "online"})
				got := m.state.PBSGuestConfirmationsForInstance("fixture-pve")
				switch index {
				case 0:
					if len(got) != 1 {
						t.Fatalf("missing initial attribution: %+v", got)
					}
					initial = got
				case 1:
					if !reflect.DeepEqual(got, initial) {
						t.Errorf("skipped store lost or renewed attribution: %+v -> %+v", initial, got)
					}
				case 2:
					if len(got) != 1 || got[0].Time != birth.Add(30*time.Minute).Unix() {
						t.Errorf("ordinary re-enable did not refresh attribution: %+v", got)
					}
				case 3:
					if len(got) != 0 {
						t.Errorf("complete empty content retained removed attribution: %+v", got)
					}
				}
			}
			if unexpected.Load() != 0 {
				t.Fatal("disabled content or unexpected operation was queried")
			}
		})
	}
}
