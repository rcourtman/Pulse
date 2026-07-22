package config

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test functions use the TestBranchcov0722PM prefix so the scoped run
// `go test ./internal/config/ -run '^TestBranchcov0722PM'` selects only them.
//
// This file raises branch coverage for the five MultiTenantPersistence
// organization lifecycle methods in multi_tenant.go:
//   - LoadOrganization
//   - LoadOrganizationStrict
//   - SaveOrganization
//   - ListOrganizations
//   - DeleteOrganization
//
// Each MultiTenantPersistence is built over an isolated t.TempDir(), mirroring
// the pattern in multi_tenant_test.go. No writes escape the temp dir.

// newTestMultiTenantPersistence returns a MultiTenantPersistence rooted at a
// fresh temp directory along with the base directory path.
func newTestMultiTenantPersistence(t *testing.T) (*MultiTenantPersistence, string) {
	t.Helper()
	baseDir := t.TempDir()
	return NewMultiTenantPersistence(baseDir), baseDir
}

// TestBranchcov0722PM_LoadOrganization exercises every return path of
// (*MultiTenantPersistence).LoadOrganization.
func TestBranchcov0722PM_LoadOrganization(t *testing.T) {
	t.Run("missing non-default org yields default org", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		// No org.json exists: persistence.LoadOrganization returns os.ErrNotExist,
		// which LoadOrganization translates into a synthetic default org.
		org, err := mtp.LoadOrganization("ghost")
		require.NoError(t, err)
		require.NotNil(t, org)
		assert.Equal(t, "ghost", org.ID)
		assert.Equal(t, "ghost", org.DisplayName)
		assert.Empty(t, org.Members, "synthetic default org has no members")
	})

	t.Run("missing default org yields default org", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		org, err := mtp.LoadOrganization("default")
		require.NoError(t, err)
		require.NotNil(t, org)
		assert.Equal(t, "default", org.ID)
		assert.Equal(t, "default", org.DisplayName)
	})

	t.Run("invalid org id surfaces get-persistence error", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		org, err := mtp.LoadOrganization("../escape")
		require.Error(t, err)
		assert.Nil(t, org)
		assert.Contains(t, err.Error(), "invalid organization ID")
	})

	t.Run("malformed org.json surfaces non-notExist error", func(t *testing.T) {
		mtp, baseDir := newTestMultiTenantPersistence(t)
		require.NoError(t, mtp.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "Acme"}))
		// Corrupt the persisted org.json so persistence.LoadOrganization fails with
		// a parse error (distinct from os.ErrNotExist).
		orgFile := filepath.Join(baseDir, "orgs", "acme", "org.json")
		require.NoError(t, os.WriteFile(orgFile, []byte("{not valid json"), 0o600))
		org, err := mtp.LoadOrganization("acme")
		require.Error(t, err)
		assert.Nil(t, org)
		assert.False(t, errors.Is(err, os.ErrNotExist), "parse error must not be classified as not-exist")
		assert.Contains(t, err.Error(), "failed to parse org file")
	})
}

// TestBranchcov0722PM_LoadOrganizationStrict exercises every return path of
// (*MultiTenantPersistence).LoadOrganizationStrict, and asserts the contrast
// with LoadOrganization on a missing org.
func TestBranchcov0722PM_LoadOrganizationStrict(t *testing.T) {
	t.Run("missing non-default org returns os.ErrNotExist and does not create dir", func(t *testing.T) {
		mtp, baseDir := newTestMultiTenantPersistence(t)
		org, err := mtp.LoadOrganizationStrict("ghost")
		require.Error(t, err)
		assert.Nil(t, org)
		assert.True(t, errors.Is(err, os.ErrNotExist),
			"strict load of a missing org must return os.ErrNotExist, got %v", err)
		// Strict short-circuits at OrgExists before GetPersistence, so it must not
		// materialize the organization directory.
		_, statErr := os.Stat(filepath.Join(baseDir, "orgs", "ghost"))
		assert.True(t, os.IsNotExist(statErr),
			"strict load of a missing org must not create its directory")
	})

	t.Run("nil receiver returns no-persistence error", func(t *testing.T) {
		var mtp *MultiTenantPersistence
		org, err := mtp.LoadOrganizationStrict("default")
		require.Error(t, err)
		assert.Nil(t, org)
		assert.Contains(t, err.Error(), "no persistence configured")
	})

	t.Run("default with no org.json errors with wrapped not-exist", func(t *testing.T) {
		// "default" always passes OrgExists, so strict reaches persistence.LoadOrganization
		// which returns os.ErrNotExist for the absent org.json; strict wraps and returns it.
		mtp, _ := newTestMultiTenantPersistence(t)
		org, err := mtp.LoadOrganizationStrict("default")
		require.Error(t, err)
		assert.Nil(t, org)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})

	t.Run("existing org loads via strict", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		require.NoError(t, mtp.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "Acme"}))
		org, err := mtp.LoadOrganizationStrict("acme")
		require.NoError(t, err)
		require.NotNil(t, org)
		assert.Equal(t, "Acme", org.DisplayName)
	})
}

// TestBranchcov0722PM_LoadVsLoadStrictMissing is the explicit contrast: on the
// same missing-org condition the lenient loader returns a synthetic default org
// while the strict loader returns os.ErrNotExist.
func TestBranchcov0722PM_LoadVsLoadStrictMissing(t *testing.T) {
	// Lenient: returns synthetic default org, nil error.
	mtpLenient, _ := newTestMultiTenantPersistence(t)
	lenient, lenientErr := mtpLenient.LoadOrganization("ghost")
	require.NoError(t, lenientErr)
	require.NotNil(t, lenient)
	assert.Equal(t, &models.Organization{ID: "ghost", DisplayName: "ghost"}, lenient)

	// Strict: same missing org, different store so no dir was pre-created.
	mtpStrict, _ := newTestMultiTenantPersistence(t)
	strict, strictErr := mtpStrict.LoadOrganizationStrict("ghost")
	require.Error(t, strictErr)
	assert.Nil(t, strict)
	assert.True(t, errors.Is(strictErr, os.ErrNotExist))
}

// TestBranchcov0722PM_SaveOrganization exercises every return path of
// (*MultiTenantPersistence).SaveOrganization plus a save->load round trip.
func TestBranchcov0722PM_SaveOrganization(t *testing.T) {
	t.Run("nil org rejected", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		err := mtp.SaveOrganization(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "organization is required")
	})

	t.Run("invalid org id rejected", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		err := mtp.SaveOrganization(&models.Organization{ID: "../evil", DisplayName: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid organization ID")
	})

	t.Run("save then load round trip", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		saved := &models.Organization{
			ID:          "acme",
			DisplayName: "Acme Corporation",
			OwnerUserID: "owner-1",
			OwnerEmail:  "owner@acme.test",
		}
		require.NoError(t, mtp.SaveOrganization(saved))

		loaded, err := mtp.LoadOrganization("acme")
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, "acme", loaded.ID)
		assert.Equal(t, "Acme Corporation", loaded.DisplayName)
		assert.Equal(t, "owner-1", loaded.OwnerUserID)
		assert.Equal(t, "owner@acme.test", loaded.OwnerEmail)
	})
}

// TestBranchcov0722PM_ListOrganizations exercises the list path over zero, one,
// and several organizations, plus the skip and error branches.
func TestBranchcov0722PM_ListOrganizations(t *testing.T) {
	t.Run("only default present", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		orgs, err := mtp.ListOrganizations()
		require.NoError(t, err)
		require.Len(t, orgs, 1)
		assert.Equal(t, "default", orgs[0].ID)
		assert.Equal(t, "default", orgs[0].DisplayName)
	})

	t.Run("one extra org", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		require.NoError(t, mtp.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "Acme"}))
		orgs, err := mtp.ListOrganizations()
		require.NoError(t, err)
		require.Len(t, orgs, 2)
		byID := map[string]*models.Organization{}
		for _, o := range orgs {
			byID[o.ID] = o
		}
		assert.Contains(t, byID, "default")
		require.Contains(t, byID, "acme")
		assert.Equal(t, "Acme", byID["acme"].DisplayName)
	})

	t.Run("several extra orgs returned sorted by id", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		for _, id := range []string{"zeta", "alpha", "mid"} {
			require.NoError(t, mtp.SaveOrganization(&models.Organization{ID: id, DisplayName: id + "-display"}))
		}
		orgs, err := mtp.ListOrganizations()
		require.NoError(t, err)
		require.Len(t, orgs, 4, "default plus three extras")

		// The implementation sorts IDs before loading, so ordering is guaranteed.
		gotIDs := make([]string, len(orgs))
		for i, o := range orgs {
			gotIDs[i] = o.ID
		}
		assert.Equal(t, []string{"alpha", "default", "mid", "zeta"}, gotIDs,
			"ListOrganizations must return orgs sorted by ID")

		for _, o := range orgs {
			if o.ID == "default" {
				assert.Equal(t, "default", o.DisplayName)
			} else {
				assert.Equal(t, o.ID+"-display", o.DisplayName)
			}
		}
	})

	t.Run("skips non-dir entries and invalid directory names", func(t *testing.T) {
		mtp, baseDir := newTestMultiTenantPersistence(t)
		require.NoError(t, mtp.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "Acme"}))
		orgsDir := filepath.Join(baseDir, "orgs")

		// A stray regular file must be skipped by the IsDir check.
		require.NoError(t, os.WriteFile(filepath.Join(orgsDir, "stray.txt"), []byte("x"), 0o600))
		// A directory whose name fails the org-ID regex (space is invalid) must be
		// skipped by the isValidOrgID check.
		require.NoError(t, os.MkdirAll(filepath.Join(orgsDir, "bad name"), 0o700))

		orgs, err := mtp.ListOrganizations()
		require.NoError(t, err)
		gotIDs := make([]string, 0, len(orgs))
		for _, o := range orgs {
			gotIDs = append(gotIDs, o.ID)
		}
		sort.Strings(gotIDs)
		assert.Equal(t, []string{"acme", "default"}, gotIDs,
			"stray file and invalid-named directory must be excluded")
	})

	t.Run("unreadable orgs dir surfaces non-NotExist error", func(t *testing.T) {
		mtp, baseDir := newTestMultiTenantPersistence(t)
		// Make "orgs" a regular file so os.ReadDir fails with a non-IsNotExist error.
		require.NoError(t, os.WriteFile(filepath.Join(baseDir, "orgs"), []byte("x"), 0o600))
		orgs, err := mtp.ListOrganizations()
		require.Error(t, err)
		assert.Nil(t, orgs)
		assert.Contains(t, err.Error(), "failed to read organizations directory")
	})

	t.Run("load failure inside list is propagated", func(t *testing.T) {
		mtp, baseDir := newTestMultiTenantPersistence(t)
		// Manually create an org directory with a malformed org.json so that
		// LoadOrganization (called per org) returns a non-not-exist error, which
		// ListOrganizations must wrap and return.
		acmeDir := filepath.Join(baseDir, "orgs", "acme")
		require.NoError(t, os.MkdirAll(acmeDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(acmeDir, "org.json"), []byte("{broken"), 0o600))
		orgs, err := mtp.ListOrganizations()
		require.Error(t, err)
		assert.Nil(t, orgs)
		assert.Contains(t, err.Error(), "acme")
	})
}

// TestBranchcov0722PM_DeleteOrganization exercises every return path of
// (*MultiTenantPersistence).DeleteOrganization.
func TestBranchcov0722PM_DeleteOrganization(t *testing.T) {
	t.Run("existing org removed and subsequent load falls back to default", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		require.NoError(t, mtp.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "Acme Corp"}))
		loaded, err := mtp.LoadOrganization("acme")
		require.NoError(t, err)
		assert.Equal(t, "Acme Corp", loaded.DisplayName)

		require.NoError(t, mtp.DeleteOrganization("acme"))

		// After deletion org.json is gone, so Load returns the synthetic default org.
		after, err := mtp.LoadOrganization("acme")
		require.NoError(t, err)
		require.NotNil(t, after)
		assert.Equal(t, "acme", after.ID)
		assert.Equal(t, "acme", after.DisplayName,
			"deleted org's DisplayName must not survive")
		// Strict must report it as missing again.
		_, strictErr := mtp.LoadOrganizationStrict("acme")
		require.Error(t, strictErr)
		assert.True(t, errors.Is(strictErr, os.ErrNotExist))
	})

	t.Run("non-existent org returns os.ErrNotExist", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		err := mtp.DeleteOrganization("ghost")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})

	t.Run("default cannot be deleted", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		err := mtp.DeleteOrganization("default")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "default organization cannot be deleted")
	})

	t.Run("invalid id rejected", func(t *testing.T) {
		mtp, _ := newTestMultiTenantPersistence(t)
		err := mtp.DeleteOrganization("../evil")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid organization ID")
	})
}
