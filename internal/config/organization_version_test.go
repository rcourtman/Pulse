package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/require"
)

func TestOrganizationSnapshotConflictsPreserveCurrentGrants(t *testing.T) {
	for _, id := range []string{"default", "acme"} {
		t.Run(id, func(t *testing.T) {
			dir := t.TempDir()
			first := NewMultiTenantPersistence(dir)
			second := NewMultiTenantPersistence(dir)
			org := &models.Organization{ID: id, DisplayName: id, OwnerUserID: "owner", Members: []models.OrganizationMember{{UserID: "owner", Role: models.OrgRoleOwner}, {UserID: "withdrawn", Role: models.OrgRoleViewer}}, PendingInvitations: []models.OrganizationInvitation{{UserID: "invitee", Role: models.OrgRoleViewer}}, SharedResources: []models.OrganizationShare{{ID: "share-1", TargetOrgID: "peer", ResourceType: "vm", ResourceID: "vm-1", AccessRole: models.OrgRoleViewer}}}
			require.NoError(t, first.SaveOrganization(org))
			require.NotEmpty(t, org.PersistenceVersion)
			stale, err := first.LoadOrganizationStrict(id)
			require.NoError(t, err)
			current, err := second.LoadOrganizationStrict(id)
			require.NoError(t, err)
			current.Members = current.Members[:1]
			current.PendingInvitations = nil
			current.SharedResources = nil
			current.Status = models.OrgStatusSuspended
			require.NoError(t, second.SaveOrganization(current))
			stale.DisplayName = "Obsolete rename"
			require.ErrorIs(t, first.SaveOrganization(stale), ErrOrganizationChanged)
			saved, err := second.LoadOrganizationStrict(id)
			require.NoError(t, err)
			require.Equal(t, current, saved, "conflict must preserve every current field")
			// No API/backup round trip can manufacture the internal write snapshot.
			wire, err := json.Marshal(saved)
			require.NoError(t, err)
			require.NotContains(t, string(wire), saved.PersistenceVersion)
			require.NotContains(t, string(wire), "PersistenceVersion")
			var unbound models.Organization
			require.NoError(t, json.Unmarshal(wire, &unbound))
			require.Empty(t, unbound.PersistenceVersion)
			require.ErrorIs(t, first.SaveOrganization(&unbound), ErrOrganizationChanged)
			// An explicit fresh edit succeeds and advances this object's snapshot, so
			// sequential saves/owned rollback do not require a special overwrite path.
			saved.DisplayName = "Reviewed rename"
			require.NoError(t, first.SaveOrganization(saved))
			saved.DisplayName = "Second reviewed rename"
			require.NoError(t, second.SaveOrganization(saved))
			fresh, err := first.LoadOrganizationStrict(id)
			require.NoError(t, err)
			require.Equal(t, saved, fresh)
		})
	}
}

func TestOrganizationSnapshotConcurrentWritersHaveOneWinner(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run(map[bool]string{false: "loaded", true: "creation"}[create], func(t *testing.T) {
			dir := t.TempDir()
			seed := NewMultiTenantPersistence(dir)
			if !create {
				require.NoError(t, seed.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "original"}))
			}
			const count = 12
			writers := make([]*MultiTenantPersistence, count)
			snapshots := make([]*models.Organization, count)
			for i := range count {
				writers[i] = NewMultiTenantPersistence(dir)
				if create {
					snapshots[i] = &models.Organization{ID: "acme"}
				} else {
					var err error
					snapshots[i], err = writers[i].LoadOrganizationStrict("acme")
					require.NoError(t, err)
				}
				snapshots[i].DisplayName = string(rune('A' + i))
			}
			start := make(chan struct{})
			results := make(chan int, count)
			var wg sync.WaitGroup
			for i := range count {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					err := writers[i].SaveOrganization(snapshots[i])
					if err == nil {
						results <- i
						return
					}
					if !errors.Is(err, ErrOrganizationChanged) {
						t.Errorf("writer %d: %v", i, err)
					}
					results <- -1
				}()
			}
			close(start)
			wg.Wait()
			close(results)
			winner, successes := -1, 0
			for result := range results {
				if result >= 0 {
					winner = result
					successes++
				}
			}
			require.Equal(t, 1, successes)
			saved, err := seed.LoadOrganizationStrict("acme")
			require.NoError(t, err)
			require.Equal(t, snapshots[winner], saved)
		})
	}
}

func TestOrganizationSnapshotDeletionDoesNotRecreateAuthority(t *testing.T) {
	dir := t.TempDir()
	first := NewMultiTenantPersistence(dir)
	second := NewMultiTenantPersistence(dir)
	org := &models.Organization{ID: "acme", DisplayName: "Acme", Members: []models.OrganizationMember{{UserID: "withdrawn", Role: models.OrgRoleViewer}}}
	require.NoError(t, first.SaveOrganization(org))
	old, err := second.LoadOrganizationStrict("acme")
	require.NoError(t, err)
	cp, err := second.GetPersistence("acme")
	require.NoError(t, err)
	require.NoError(t, first.DeleteOrganization("acme"))
	require.ErrorIs(t, second.SaveOrganization(old), ErrOrganizationChanged)
	require.ErrorIs(t, cp.SaveOrganization(old), ErrOrganizationChanged)
	require.False(t, first.OrgExists("acme"))
	_, err = os.Stat(filepath.Join(dir, "orgs", "acme"))
	require.True(t, os.IsNotExist(err))
	// A new explicit organization is independent of the old object's path-bound
	// snapshot. Moving a loaded snapshot to a different path is not creation.
	old.ID = "beta"
	require.ErrorIs(t, second.SaveOrganization(old), ErrOrganizationChanged)
	require.False(t, first.OrgExists("beta"))
	require.NoError(t, first.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "New owner", OwnerUserID: "new-owner"}))
	old.ID = "acme"
	require.ErrorIs(t, second.SaveOrganization(old), ErrOrganizationChanged)
}

func TestOrganizationSnapshotMissingOrUnknownStateIsNotReplacementAuthority(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			p := NewConfigPersistence(t.TempDir())
			require.NoError(t, p.EnsureConfigDir())
			org := &models.Organization{ID: "default", DisplayName: "Original"}
			require.NoError(t, p.SaveOrganization(org))
			switch mode {
			case "missing":
				require.NoError(t, os.Remove(p.orgFile))
			case "corrupt":
				require.NoError(t, os.WriteFile(p.orgFile, []byte("{unknown"), 0600))
			case "unreadable":
				require.NoError(t, os.Remove(p.orgFile))
				require.NoError(t, os.Mkdir(p.orgFile, 0700))
			}
			previous := org.PersistenceVersion
			org.DisplayName = "must not publish"
			require.Error(t, p.SaveOrganization(org))
			require.Equal(t, previous, org.PersistenceVersion)
			if mode == "corrupt" {
				raw, err := os.ReadFile(p.orgFile)
				require.NoError(t, err)
				require.Equal(t, "{unknown", string(raw))
				require.ErrorIs(t, p.SaveOrganization(&models.Organization{ID: "default"}), ErrOrganizationChanged)
			}
		})
	}
}

func TestOrganizationSnapshotAtomicFailureRetainsSourceAndVersion(t *testing.T) {
	for _, fault := range []string{"create", "chmod", "write", "short write", "close", "rename"} {
		t.Run(fault, func(t *testing.T) {
			p := NewConfigPersistence(t.TempDir())
			require.NoError(t, p.EnsureConfigDir())
			org := &models.Organization{ID: "default", DisplayName: "Original"}
			require.NoError(t, p.SaveOrganization(org))
			old, err := os.ReadFile(p.orgFile)
			require.NoError(t, err)
			version := org.PersistenceVersion
			p.fs = &atomicConfigFaultFS{FileSystem: defaultFileSystem{}, fault: fault, failure: errors.New("synthetic organization write failure")}
			org.DisplayName = "must not publish"
			require.Error(t, p.SaveOrganization(org))
			require.Equal(t, version, org.PersistenceVersion)
			current, err := os.ReadFile(p.orgFile)
			require.NoError(t, err)
			require.True(t, bytes.Equal(old, current))
			matches, err := filepath.Glob(filepath.Join(p.configDir, ".org.json.*.tmp"))
			require.NoError(t, err)
			require.Empty(t, matches)
			p.fs = defaultFileSystem{}
			require.NoError(t, p.SaveOrganization(org))
			stat, err := os.Stat(p.orgFile)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), stat.Mode().Perm())
		})
	}
}

func TestOrganizationSnapshotLockEntriesRetire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "org.json")
	unlock := lockOrganizationFile(path)
	organizationFileLocks.Lock()
	refs := organizationFileLocks.files[path].refs
	organizationFileLocks.Unlock()
	require.Equal(t, 1, refs)
	unlock()
	organizationFileLocks.Lock()
	_, present := organizationFileLocks.files[path]
	organizationFileLocks.Unlock()
	require.False(t, present)
}
