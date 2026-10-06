package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// Exercise every abort boundary after a successful refresh, without flushing
// cache on recovery. A timestamp for an unpublished row must not hide a failed
// verification behind the previously published verified snapshot.
func TestPBSOrdinaryPollCancellationDoesNotRenewUnpublishedCache(t *testing.T) {
	for _, boundary := range []string{"before-datastore", "mid-datastore", "after-enumeration", "during-task-correlation"} {
		t.Run(boundary, func(t *testing.T) {
			f := newPBSPosturePollFixture(t, 1)
			var phase, snapshotReads atomic.Int32
			blocked := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				aborting := phase.Load() == 1
				isOther := strings.Contains(r.URL.Path, "/other/") || r.URL.Query().Get("ns") == "other"
				isTask := strings.HasSuffix(r.URL.Path, "/tasks")
				if aborting && ((boundary != "during-task-correlation" && isOther) || (boundary == "during-task-correlation" && isTask)) {
					blocked <- struct{}{}
					<-r.Context().Done()
					return
				}
				if strings.HasSuffix(r.URL.Path, "/groups") {
					groups := []map[string]any{}
					if !isOther {
						groups = append(groups, map[string]any{"backup-type": "vm", "backup-id": "100", "last-backup": f.backupAt, "backup-count": 1})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": groups})
					return
				}
				if strings.HasSuffix(r.URL.Path, "/snapshots") {
					snapshotReads.Add(1)
					verification := "ok"
					if phase.Load() != 0 {
						verification = "failed"
					}
					snapshot := map[string]any{"backup-type": "vm", "backup-id": "100", "backup-time": f.backupAt, "size": 1024, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": verification}}
					if aborting && boundary == "during-task-correlation" {
						// This forces the final asynchronous HTTP read after enumeration.
						snapshot["size"] = 0
						snapshot["files"] = []string{"qemu-server.conf.blob"}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{snapshot}})
					return
				}
				http.NotFound(w, r)
			}))
			t.Cleanup(server.Close)
			var err error
			f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			datastores := []models.PBSDatastore{{Name: "archive"}, {Name: "other"}}
			switch boundary {
			case "before-datastore":
				datastores = append(datastores, models.PBSDatastore{Name: "never"})
			case "mid-datastore":
				datastores = []models.PBSDatastore{{Name: "archive", Namespaces: []models.PBSNamespace{{Path: ""}, {Path: "other"}, {Path: "never"}}}}
			case "during-task-correlation":
				datastores = []models.PBSDatastore{{Name: "archive"}}
			}
			f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, datastores)
			f.wait(t)
			f.assertProtected(t, "pbs-main")
			key := pbsBackupGroupKey{datastore: "archive", backupType: "vm", backupID: "100"}
			expired := time.Now().Add(-2 * pbsBackupCacheTTL)
			f.m.setPBSBackupCacheTime("pbs-main", key, expired)
			phase.Store(1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				f.m.pollPBSBackups(ctx, "pbs-main", f.client, datastores)
			}()
			select {
			case <-blocked:
				cancel()
			case <-time.After(10 * time.Second):
				cancel()
				<-done
				t.Fatal("poll did not reach the intended cancellation boundary")
			}
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("cancelled poll did not stop")
			}
			f.wait(t)
			if got := f.m.pbsBackupCacheTimeFor("pbs-main", key); !got.Equal(expired) {
				t.Errorf("cancelled poll renewed unpublished cache: got %v, want %v", got, expired)
			}
			raw := f.m.state.GetSnapshot().PBSBackups
			if len(raw) != 1 || !raw[0].Verified || raw[0].InProgress {
				t.Errorf("cancelled poll replaced published raw rows: %+v", raw)
			}
			points := f.points(t, "pbs-main")
			if len(points) != 1 || points[0].Verified == nil || !*points[0].Verified {
				t.Errorf("cancelled poll replaced persisted evidence: %+v", points)
			}
			// Do not clear or expire cache again. The next ordinary poll must
			// independently fetch the verification change the aborted poll lost.
			phase.Store(2)
			f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, datastores)
			f.wait(t)
			raw = f.m.state.GetSnapshot().PBSBackups
			mapped, err := proxmoxmapper.FromPBSBackupsWithEvidence(raw, buildPBSGuestCandidates(f.m.GetUnifiedReadStateOrSnapshot()), time.Now())
			points = f.points(t, "pbs-main")
			postures := f.postures(t)
			if snapshotReads.Load() != 3 || len(raw) != 1 || raw[0].Verified || raw[0].InProgress || err != nil || len(mapped) != 1 || mapped[0].Verified == nil || *mapped[0].Verified || len(points) != 1 || points[0].Verified == nil || *points[0].Verified || postures[0].State != recovery.ProtectionStateAttention || postures[0].Verification != recovery.ProtectionVerificationUnverified {
				t.Errorf("recovery reads=%d raw=%+v mapped=%+v err=%v persisted=%+v posture=%+v", snapshotReads.Load(), raw, mapped, err, points, postures)
			}
			if mapped[0].SubjectResourceID != f.ids[0] || points[0].SubjectResourceID != f.ids[0] {
				t.Errorf("recovery changed subject identity: mapped=%s persisted=%s want=%s", mapped[0].SubjectResourceID, points[0].SubjectResourceID, f.ids[0])
			}
			publishedAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
			if !publishedAt.After(expired) {
				t.Error("successful recovery did not renew its published cache")
			}
			// Once the matching failed row has been published, normal TTL reuse
			// must avoid an unnecessary fourth read without restoring verified.
			f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, datastores)
			f.wait(t)
			if snapshotReads.Load() != 3 || !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(publishedAt) || f.postures(t)[0].Verification != recovery.ProtectionVerificationUnverified {
				t.Error("cache-hit poll reread or renewed snapshots, or restored stale verification")
			}
			t.Logf("boundary=%s snapshots=%d raw/mapped/persisted=%d/%d/%d subject=%s posture=%s/%s", boundary, snapshotReads.Load(), len(raw), len(mapped), len(points), f.ids[0], postures[0].State, postures[0].Verification)
		})
	}
}
