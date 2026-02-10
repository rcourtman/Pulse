package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestEncryptionKey writes a deterministic 32-byte base64-encoded key to .encryption.key.
func writeTestEncryptionKey(t *testing.T, dir string) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".encryption.key"), []byte(encoded), 0o600))
}

func TestBillingState_IntegrityOnSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	writeTestEncryptionKey(t, dir)

	store := NewFileBillingStore(dir)

	now := int64(1700000000)
	endsAt := int64(1701209600)
	state := &entitlements.BillingState{
		Capabilities:      []string{"relay", "ai_autofix"},
		SubscriptionState: entitlements.SubStateTrial,
		PlanVersion:       "trial",
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}

	require.NoError(t, store.SaveBillingState("default", state))

	// Verify the integrity field was set on the struct.
	assert.NotEmpty(t, state.Integrity, "integrity should be set on state after save")

	// Verify the integrity field is persisted in the JSON file.
	data, err := os.ReadFile(filepath.Join(dir, "billing.json"))
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.NotEmpty(t, raw["integrity"], "integrity field should be present in billing.json")
}

func TestBillingState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	writeTestEncryptionKey(t, dir)

	store := NewFileBillingStore(dir)

	now := int64(1700000000)
	endsAt := int64(1701209600)
	state := &entitlements.BillingState{
		Capabilities:      []string{"relay", "ai_autofix"},
		SubscriptionState: entitlements.SubStateTrial,
		PlanVersion:       "trial",
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}

	require.NoError(t, store.SaveBillingState("default", state))

	loaded, err := store.GetBillingState("default")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, entitlements.SubStateTrial, loaded.SubscriptionState)
	assert.ElementsMatch(t, []string{"relay", "ai_autofix"}, loaded.Capabilities)
	assert.Equal(t, now, *loaded.TrialStartedAt)
	assert.Equal(t, endsAt, *loaded.TrialEndsAt)
}

func TestBillingState_TamperDetection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	writeTestEncryptionKey(t, dir)

	store := NewFileBillingStore(dir)

	now := int64(1700000000)
	endsAt := int64(1701209600)
	state := &entitlements.BillingState{
		Capabilities:      []string{"relay"},
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}

	require.NoError(t, store.SaveBillingState("default", state))

	// Confirm valid state loads.
	loaded, err := store.GetBillingState("default")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Tamper: change trial_ends_at in the JSON file.
	billingPath := filepath.Join(dir, "billing.json")
	data, err := os.ReadFile(billingPath)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
	raw["trial_ends_at"] = float64(1800000000) // tampered value
	tampered, err := json.Marshal(raw)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(billingPath, tampered, 0o600))

	// Tampered state should be treated as nonexistent.
	loaded, err = store.GetBillingState("default")
	require.NoError(t, err)
	assert.Nil(t, loaded, "tampered billing state should be treated as nonexistent")
}

func TestBillingState_TamperDetection_CapabilitiesModified(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	writeTestEncryptionKey(t, dir)

	store := NewFileBillingStore(dir)

	now := int64(1700000000)
	endsAt := int64(1701209600)
	state := &entitlements.BillingState{
		Capabilities:      []string{"relay"},
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}

	require.NoError(t, store.SaveBillingState("default", state))

	// Tamper: add a capability.
	billingPath := filepath.Join(dir, "billing.json")
	data, err := os.ReadFile(billingPath)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
	raw["capabilities"] = []interface{}{"relay", "ai_autofix", "multi_tenant"}
	tampered, err := json.Marshal(raw)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(billingPath, tampered, 0o600))

	loaded, err := store.GetBillingState("default")
	require.NoError(t, err)
	assert.Nil(t, loaded, "state with injected capabilities should be treated as nonexistent")
}

func TestBillingState_MigrationFromUnsignedState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	writeTestEncryptionKey(t, dir)

	// Write a billing.json without an integrity field (simulates pre-upgrade state).
	now := int64(1700000000)
	endsAt := int64(1701209600)
	state := entitlements.BillingState{
		Capabilities:      []string{"relay"},
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}
	data, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "billing.json"), data, 0o600))

	store := NewFileBillingStore(dir)

	// First read should trigger migration and return valid state.
	loaded, err := store.GetBillingState("default")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, entitlements.SubStateTrial, loaded.SubscriptionState)
	assert.NotEmpty(t, loaded.Integrity, "integrity should be computed during migration")

	// Verify integrity was persisted to file.
	fileData, err := os.ReadFile(filepath.Join(dir, "billing.json"))
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(fileData, &raw))
	assert.NotEmpty(t, raw["integrity"], "integrity should be persisted to billing.json after migration")

	// Subsequent reads should pass verification without re-migration.
	loaded2, err := store.GetBillingState("default")
	require.NoError(t, err)
	require.NotNil(t, loaded2)
	assert.Equal(t, loaded.Integrity, loaded2.Integrity)
}

func TestBillingState_NoKeyGracefulDegradation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	// No .encryption.key written — key is missing.

	store := NewFileBillingStore(dir)

	now := int64(1700000000)
	endsAt := int64(1701209600)
	state := &entitlements.BillingState{
		Capabilities:      []string{"relay"},
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}

	// Save should succeed without a key (no integrity computed).
	require.NoError(t, store.SaveBillingState("default", state))
	assert.Empty(t, state.Integrity, "integrity should not be set when no key is available")

	// Load should succeed and return the state without integrity checks.
	loaded, err := store.GetBillingState("default")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, entitlements.SubStateTrial, loaded.SubscriptionState)
	assert.Empty(t, loaded.Integrity)
}

func TestBillingState_CapabilityOrderIndependent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	writeTestEncryptionKey(t, dir)

	store := NewFileBillingStore(dir)

	now := int64(1700000000)
	endsAt := int64(1701209600)

	// Save with capabilities in one order.
	state := &entitlements.BillingState{
		Capabilities:      []string{"relay", "ai_autofix", "multi_tenant"},
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}
	require.NoError(t, store.SaveBillingState("default", state))
	hmac1 := state.Integrity

	// Save with capabilities in reverse order — HMAC should be identical.
	state2 := &entitlements.BillingState{
		Capabilities:      []string{"multi_tenant", "ai_autofix", "relay"},
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &now,
		TrialEndsAt:       &endsAt,
	}
	require.NoError(t, store.SaveBillingState("default", state2))

	assert.Equal(t, hmac1, state2.Integrity, "HMAC should be independent of capability ordering")
}
