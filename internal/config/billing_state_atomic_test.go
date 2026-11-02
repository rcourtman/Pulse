package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/require"
)

func privateBillingFixture(t *testing.T, orgID string) (*FileBillingStore, string, *pkglicensing.BillingState) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	writeTestEncryptionKey(t, dir)
	store := NewFileBillingStore(dir)
	path, err := store.billingStatePath(orgID)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	return store, path, &pkglicensing.BillingState{
		StripeCustomerID:        "synthetic-customer",
		EntitlementJWT:          "synthetic.lease.value",
		EntitlementRefreshToken: "synthetic-refresh-value",
	}
}

func requirePrivateBillingRoundTrip(t *testing.T, store *FileBillingStore, orgID, path string, want *pkglicensing.BillingState) {
	t.Helper()
	requirePrivateConfigFile(t, path)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(raw), want.EntitlementJWT)
	require.NotContains(t, string(raw), want.EntitlementRefreshToken)
	var persisted pkglicensing.BillingState
	require.NoError(t, json.Unmarshal(raw, &persisted))
	require.NotEmpty(t, persisted.EntitlementJWT)
	require.NotEmpty(t, persisted.EntitlementRefreshToken)
	require.NotEmpty(t, persisted.Integrity)

	// A fresh store must recover the exact secrets and verify the persisted HMAC.
	loaded, err := NewFileBillingStore(store.baseDataDir).GetBillingState(orgID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, want.StripeCustomerID, loaded.StripeCustomerID)
	require.Equal(t, want.EntitlementJWT, loaded.EntitlementJWT)
	require.Equal(t, want.EntitlementRefreshToken, loaded.EntitlementRefreshToken)
	key, err := store.loadHMACKey()
	require.NoError(t, err)
	require.True(t, verifyBillingIntegrity(loaded, key))
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".billing.json.*.tmp"))
	require.NoError(t, err)
	require.Empty(t, leftovers, "completed billing publications must leave no owned temporary file")
}

func TestBillingState_PrivatePublicationIgnoresStaleTemporaryPaths(t *testing.T) {
	for _, orgID := range []string{"default", "department-a"} {
		for _, kind := range []string{"regular", "symlink", "hardlink", "directory"} {
			t.Run(orgID+"/"+kind, func(t *testing.T) {
				store, path, state := privateBillingFixture(t, orgID)
				stale := path + ".tmp"
				unrelated := staleConfigPath(t, stale, kind)
				before, err := os.Lstat(stale)
				require.NoError(t, err)

				require.NoError(t, store.SaveBillingState(orgID, state))
				requireUntouchedConfigFixture(t, unrelated)
				requirePrivateBillingRoundTrip(t, store, orgID, path, state)
				after, err := os.Lstat(stale)
				require.NoError(t, err)
				require.True(t, os.SameFile(before, after), "a stale path is not owned by this write")
				require.Equal(t, before.Mode(), after.Mode())
				if kind == "regular" {
					raw, err := os.ReadFile(stale)
					require.NoError(t, err)
					require.Equal(t, "stale state", string(raw))
				}
			})
		}
	}
}

func TestBillingState_PrivatePublicationReplacesDestinationLinks(t *testing.T) {
	for _, orgID := range []string{"default", "department-a"} {
		for _, kind := range []string{"regular", "symlink", "hardlink"} {
			t.Run(orgID+"/"+kind, func(t *testing.T) {
				store, path, state := privateBillingFixture(t, orgID)
				unrelated := staleConfigPath(t, path, kind)
				require.NoError(t, store.SaveBillingState(orgID, state))
				requireUntouchedConfigFixture(t, unrelated)
				requirePrivateBillingRoundTrip(t, store, orgID, path, state)
			})
		}
	}
}

func TestBillingState_PrivateMigrationIgnoresStaleTemporaryPaths(t *testing.T) {
	for _, orgID := range []string{"default", "department-a"} {
		for _, kind := range []string{"regular", "symlink", "hardlink", "directory"} {
			t.Run(orgID+"/"+kind, func(t *testing.T) {
				store, path, state := privateBillingFixture(t, orgID)
				legacy, err := json.Marshal(state)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, legacy, 0o600))
				unrelated := staleConfigPath(t, path+".tmp", kind)

				loaded, err := store.GetBillingState(orgID)
				require.NoError(t, err)
				require.NotNil(t, loaded)
				require.Equal(t, state.EntitlementJWT, loaded.EntitlementJWT)
				requireUntouchedConfigFixture(t, unrelated)
				requirePrivateBillingRoundTrip(t, store, orgID, path, state)
			})
		}
	}
}

func TestBillingState_PrivatePublicationFailureCleansUpAndCanRetry(t *testing.T) {
	for _, orgID := range []string{"default", "department-a"} {
		t.Run(orgID, func(t *testing.T) {
			store, path, state := privateBillingFixture(t, orgID)
			require.NoError(t, os.Mkdir(path, 0o700))
			marker := filepath.Join(path, "unrelated")
			require.NoError(t, os.WriteFile(marker, []byte("unrelated state"), 0o600))
			err := store.SaveBillingState(orgID, state)
			require.Error(t, err)
			require.ErrorContains(t, err, "commit billing state")
			requireUntouchedConfigFixture(t, marker)
			entries, err := os.ReadDir(filepath.Dir(path))
			require.NoError(t, err)
			for _, entry := range entries {
				require.NotEqual(t, "billing.json.tmp", entry.Name())
			}
			leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".billing.json.*.tmp"))
			require.NoError(t, err)
			require.Empty(t, leftovers)

			// Move the obstruction aside without deleting any of its state.
			require.NoError(t, os.Rename(path, path+".preserved"))
			require.NoError(t, store.SaveBillingState(orgID, state))
			requireUntouchedConfigFixture(t, filepath.Join(path+".preserved", "unrelated"))
			requirePrivateBillingRoundTrip(t, store, orgID, path, state)
		})
	}
}

func TestBillingState_PrivateIndependentPublications(t *testing.T) {
	for _, orgID := range []string{"default", "department-a"} {
		t.Run(orgID, func(t *testing.T) {
			store, path, _ := privateBillingFixture(t, orgID)
			const writers = 12
			start, results := make(chan struct{}), make(chan error, writers)
			for i := range writers {
				go func() {
					<-start
					state := &pkglicensing.BillingState{
						StripeCustomerID:        fmt.Sprintf("synthetic-customer-%d", i),
						EntitlementJWT:          fmt.Sprintf("synthetic.lease.%d", i),
						EntitlementRefreshToken: fmt.Sprintf("synthetic-refresh-%d", i),
					}
					results <- NewFileBillingStore(store.baseDataDir).SaveBillingState(orgID, state)
				}()
			}
			close(start)
			// Drain every writer before assertions can dispose of the fixture directory.
			failures := make([]error, 0, writers)
			for range writers {
				failures = append(failures, <-results)
			}
			require.NoError(t, errors.Join(failures...))
			loaded, err := NewFileBillingStore(store.baseDataDir).GetBillingState(orgID)
			require.NoError(t, err)
			require.NotNil(t, loaded)
			complete := false
			for i := range writers {
				complete = complete || loaded.StripeCustomerID == fmt.Sprintf("synthetic-customer-%d", i) &&
					loaded.EntitlementJWT == fmt.Sprintf("synthetic.lease.%d", i) &&
					loaded.EntitlementRefreshToken == fmt.Sprintf("synthetic-refresh-%d", i)
			}
			require.True(t, complete, "recover one complete writer's state, never mixed fields")
			requirePrivateBillingRoundTrip(t, store, orgID, path, loaded)
		})
	}
}
