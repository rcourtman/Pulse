package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// These are ordinary reads of a synthetic PBS API, not a provider refusal,
// retry/replay experiment, native backup or guest operation. A rate/timeout
// response must retain only already observed files, never refresh their proof.
func TestPBSRetryableReadLifecycle(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, stage := range []string{"namespace", "groups", "snapshots"} {
			for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests} {
				t.Run(fmt.Sprintf("%s/%s/%d", kind, stage, status), func(t *testing.T) {
					f := newPBSNamespaceContinuityFixture(t, kind)
					f.collect(t, true, nil)
					before, points := f.live(), f.points(t, "pbs-main")
					key := pbsBackupGroupKey{datastore: "archive", namespace: "tenant", backupType: kind, backupID: "101"}
					cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
					f.rootAt.Add(15 * 60)
					f.tenantAt.Add(15 * 60) // Require a snapshot read without changing any TTL.
					f.setRetryableFailure(stage, status)
					for range 2 {
						f.collectRetryable(t, stage, status)
						got := f.live()
						if len(got) != 2 || !reflect.DeepEqual(got["101"], before["101"]) {
							t.Error("temporary read loss erased or changed the last accepted file")
						}
						if got["100"].BackupTime.Unix() != before["100"].BackupTime.Add(15*time.Minute).Unix() {
							t.Error("readable sibling did not advance")
						}
						if !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
							t.Error("failed read renewed the snapshot cache")
						}
						for _, prior := range points {
							if prior.Details["namespace"] != "tenant" {
								continue
							}
							found := false
							for _, now := range f.points(t, "pbs-main") {
								found = found || (prior.ID == now.ID && reflect.DeepEqual(prior, now))
							}
							if !found {
								t.Error("unread historical point lost or its evidence renewed")
							}
						}
						for _, p := range f.postures(t) {
							if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 ||
								p.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsUnknown ||
								p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial {
								t.Errorf("temporary failure invented protection or denied access: %+v", p)
							}
						}
					}
					if f.failedReads.Load() != 2 {
						t.Fatal("failure triggered an extra request or hid a failed read")
					}
					f.setRetryableFailure(stage, 0)
					f.collect(t, true, nil)
					if f.live()["101"].BackupTime.Unix() != f.tenantAt.Load() {
						t.Fatal("ordinary recovery did not replace the retained file")
					}
					for _, p := range f.postures(t) {
						if p.State != recovery.ProtectionStateProtected {
							t.Error("complete read failed to restore protection")
						}
					}
					f.rootEmpty.Store(true)
					f.tenantEmpty.Store(true)
					f.collect(t, true, nil)
					if len(f.live()) != 0 || len(f.points(t, "pbs-main")) != 0 {
						t.Fatal("accepted empty read retained files that are now absent")
					}
				})
			}
		}
	}
}

func (f *pbsNamespaceContinuityFixture) setRetryableFailure(stage string, status int) {
	switch stage {
	case "namespace":
		f.namespaceStatus.Store(int32(status))
	case "groups":
		f.tenantStatus.Store(int32(status))
	case "snapshots":
		f.tenantSnapshotStatus.Store(int32(status))
	}
}

func (f *pbsNamespaceContinuityFixture) collectRetryable(t *testing.T, stage string, status int) {
	t.Helper()
	var discoveryErr error
	if stage == "namespace" {
		_, err := f.client.ListNamespaces(context.Background(), "archive", "", 0)
		if got, known := pbs.HTTPStatus(err); !known || got != status {
			t.Fatalf("namespace response lost actual status %d: %v", status, err)
		}
		discoveryErr = fmt.Errorf("discover namespaces: %w", err)
	}
	f.collect(t, stage != "namespace", discoveryErr)
}

func TestPBSRetryableReadWithoutCacheDoesNotInventFiles(t *testing.T) {
	for _, stage := range []string{"namespace", "groups", "snapshots"} {
		for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests} {
			t.Run(fmt.Sprintf("%s/%d", stage, status), func(t *testing.T) {
				f := newPBSNamespaceContinuityFixture(t, "vm")
				f.setRetryableFailure(stage, status)
				f.collectRetryable(t, stage, status)
				if len(f.live()) != 1 || f.live()["101"].ID != "" || len(f.points(t, "pbs-main")) != 1 {
					t.Fatal("unread cold scope invented files or lost the readable root")
				}
			})
		}
	}
}
