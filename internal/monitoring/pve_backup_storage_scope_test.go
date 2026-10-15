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
