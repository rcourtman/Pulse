package monitoring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestBuildPVEProtectionProviderObservationMapsEvidenceQuality(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		stats           pveProtectionCollectionStats
		wantJob         recovery.Outcome
		wantHistory     recovery.ProtectionHistoryCompleteness
		wantPermissions operationaltrust.EvidencePermissions
		wantReason      string
	}{
		{
			name: "complete enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             2,
				SuccessfulNodeQueries: 2,
				BackupStorageCount:    2,
				ContentSuccesses:      2,
			},
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name: "complete empty enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             1,
				SuccessfulNodeQueries: 1,
			},
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name: "partial transient enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             2,
				SuccessfulNodeQueries: 1,
				BackupStorageCount:    1,
				ContentSuccesses:      1,
			},
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pve_partial_enumeration",
		},
		{
			name: "partial provider access",
			stats: pveProtectionCollectionStats{
				NodeCount:              1,
				SuccessfulNodeQueries:  1,
				BackupStorageCount:     2,
				ContentSuccesses:       1,
				ContentFailures:        1,
				PermissionFailureCount: 1,
			},
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsPartial,
			wantReason:      "pve_partial_provider_access",
		},
		{
			name: "provider timeout",
			stats: pveProtectionCollectionStats{
				NodeCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pve_collection_unavailable",
		},
		{
			name: "provider denies every node",
			stats: pveProtectionCollectionStats{
				NodeCount:              2,
				PermissionFailureCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsDenied,
			wantReason:      "pve_provider_access_denied",
		},
		{
			name: "provider denies every backup storage",
			stats: pveProtectionCollectionStats{
				NodeCount:              1,
				SuccessfulNodeQueries:  1,
				BackupStorageCount:     2,
				ContentFailures:        2,
				PermissionFailureCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsDenied,
			wantReason:      "pve_provider_access_denied",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildPVEProtectionProviderObservation("pve-main", test.stats, now)
			if err != nil {
				t.Fatalf("buildPVEProtectionProviderObservation() error = %v", err)
			}
			if got.JobState != test.wantJob ||
				got.HistoryCompleteness != test.wantHistory ||
				got.Permissions != test.wantPermissions {
				t.Fatalf(
					"observation = job %q, history %q, permissions %q; want %q, %q, %q",
					got.JobState,
					got.HistoryCompleteness,
					got.Permissions,
					test.wantJob,
					test.wantHistory,
					test.wantPermissions,
				)
			}
			if test.wantReason == "" {
				if got.Evidence.Reason != nil {
					t.Fatalf("reason = %#v, want nil", got.Evidence.Reason)
				}
			} else if got.Evidence.Reason == nil || got.Evidence.Reason.Code != test.wantReason {
				t.Fatalf("reason = %#v, want %q", got.Evidence.Reason, test.wantReason)
			}
		})
	}
}

func TestIsPVEBackupPermissionError(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		errors.New("403 Forbidden"),
		errors.New("permission check failed"),
		errors.New("HTTP 401"),
	} {
		if !isPVEBackupPermissionError(err) {
			t.Fatalf("expected %q to be recognized as a permission failure", err)
		}
	}
	if isPVEBackupPermissionError(errors.New("connection timed out")) {
		t.Fatal("transient timeout must not be classified as a permission failure")
	}
}

// persistGuestIdentity runs inside the protection/backup read-state walk and
// must hand its write to the store rather than detaching its own goroutine.
// A detached write cannot be drained by Monitor.Stop, so it can land after
// shutdown and after the data directory has started being removed.
func TestPersistGuestIdentityQueuesADrainableWrite(t *testing.T) {
	dataPath := t.TempDir()
	store := config.NewGuestMetadataStore(dataPath, nil)

	persistGuestIdentity(store, "pve1:node1:100", "web-01", "qemu")

	if !store.WaitForPendingWrites(10 * time.Second) {
		t.Fatal("persistGuestIdentity write was not drainable; the call site detached its own goroutine")
	}
	if _, err := os.Stat(filepath.Join(dataPath, "guest_metadata.json")); err != nil {
		t.Fatalf("guest identity was not persisted after the drain: %v", err)
	}
	if meta := store.Get("pve1:node1:100"); meta == nil || meta.LastKnownName != "web-01" || meta.LastKnownType != "qemu" {
		t.Fatalf("guest identity not recorded: %#v", meta)
	}
}

// An unchanged identity must not queue a write at all, so a steady-state poll
// cycle does no disk work.
func TestPersistGuestIdentitySkipsUnchangedIdentities(t *testing.T) {
	dataPath := t.TempDir()
	store := config.NewGuestMetadataStore(dataPath, nil)

	persistGuestIdentity(store, "pve1:node1:101", "db-01", "lxc")
	if !store.WaitForPendingWrites(10 * time.Second) {
		t.Fatal("initial write did not drain")
	}
	firstWrite, err := os.Stat(filepath.Join(dataPath, "guest_metadata.json"))
	if err != nil {
		t.Fatalf("stat after first write: %v", err)
	}

	persistGuestIdentity(store, "pve1:node1:101", "db-01", "lxc")
	if !store.WaitForPendingWrites(10 * time.Second) {
		t.Fatal("second call did not drain")
	}
	secondWrite, err := os.Stat(filepath.Join(dataPath, "guest_metadata.json"))
	if err != nil {
		t.Fatalf("stat after second call: %v", err)
	}
	if !firstWrite.ModTime().Equal(secondWrite.ModTime()) {
		t.Error("unchanged identity triggered a redundant write")
	}
}

// skippedBackupStorageClient exposes configured but disabled backup targets.
// It records attempted content reads so retaining their history cannot be
// achieved by bypassing the existing no-query boundary.
type skippedBackupStorageClient struct {
	*pveReconciliationClient
	disabled       map[string]bool
	contentQueries map[string]int
}

func (c *skippedBackupStorageClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	storages, err := c.pveReconciliationClient.GetStorage(ctx, node)
	for i := range storages {
		if c.disabled[node] {
			storages[i].Enabled = 0
		}
	}
	return storages, err
}

func (c *skippedBackupStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	c.contentQueries[node]++
	return c.pveReconciliationClient.GetStorageContent(ctx, node, storage)
}

func TestPVEProtectionDisabledStorageIsNotAnEmptyRead(t *testing.T) {
	for _, cold := range []bool{false, true} {
		for _, allDisabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("cold=%t/all-disabled=%t", cold, allDisabled), func(t *testing.T) {
				f := newPVEReconciliationFixture(t)
				f.poll(t)
				before := f.points(t)
				if len(before) != 2 {
					t.Fatal("healthy control did not establish both backup observations")
				}
				if cold {
					f.coldState()
				}
				client := &skippedBackupStorageClient{pveReconciliationClient: f.client,
					disabled: map[string]bool{"node-a": allDisabled, "node-b": true}, contentQueries: make(map[string]int)}
				f.client.at = f.client.at.Add(15 * time.Minute)
				for range 2 {
					f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
					f.wait(t)
					points := f.points(t)
					if len(points) != len(before) {
						t.Errorf("skipping disabled storage deleted durable history: points=%d want=%d", len(points), len(before))
					}
					for id, point := range before {
						if (allDisabled || point.SubjectResourceID == f.ids[1]) && !reflect.DeepEqual(points[id], point) {
							t.Error("unread disabled storage lost or renewed its point-level evidence")
						}
					}
					wantRaw := 2
					if cold {
						wantRaw = 1
						if allDisabled {
							wantRaw = 0
						}
					}
					if got := len(f.m.state.GetSnapshot().PVEBackups.StorageBackups); got != wantRaw {
						t.Errorf("live inventory=%d want=%d; no skipped read may delete known rows or invent cold rows", got, wantRaw)
					}
					history := recovery.ProtectionHistoryPartial
					if allDisabled {
						history = recovery.ProtectionHistoryUnavailable
					}
					f.assertHistory(t, history, operationaltrust.EvidencePermissionsUnknown)
				}
				if client.contentQueries["node-b"] != 0 || (allDisabled && client.contentQueries["node-a"] != 0) {
					t.Fatalf("disabled storage was queried: %+v", client.contentQueries)
				}
				if !allDisabled {
					if client.contentQueries["node-a"] != 2 {
						t.Fatal("independent readable storage did not keep polling")
					}
					for _, point := range f.points(t) {
						if point.SubjectResourceID == f.ids[0] && (point.CompletedAt == nil || !point.CompletedAt.Equal(f.client.at)) {
							t.Error("independent readable storage did not publish its accepted result")
						}
					}
				}

				// Normal re-enable permits a real read. A successful empty reply
				// still removes the absent backup, and a later accepted listing
				// restores both inventory and complete provider evidence.
				client.disabled = nil
				f.client.emptyB = true
				f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
				f.wait(t)
				if len(f.points(t)) != 1 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 1 {
					t.Fatal("a successful empty read after re-enable did not remove only the absent backup")
				}
				for _, posture := range f.postures(t) {
					if posture.SubjectResourceID == f.ids[0] {
						if len(posture.ProviderStates) != 1 || posture.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryComplete {
							t.Fatal("successful readable scope did not regain complete evidence")
						}
					} else if posture.LastSuccessfulPointAt != nil || posture.State == recovery.ProtectionStateProtected {
						t.Fatal("confirmed-absent backup still claimed a successful restore point")
					}
				}
				f.client.emptyB = false
				f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
				f.wait(t)
				if len(f.points(t)) != 2 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 2 {
					t.Fatal("accepted listing after re-enable did not recover both backups")
				}
				f.assertHistory(t, recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient)
			})
		}
	}
}
