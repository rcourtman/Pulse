package config

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/require"
)

// Use the real billing encryption/HMAC implementation and the real Python
// profile writer. This is a synthetic, private filesystem test, not a native
// service, production entitlement or deployment acceptance.
func TestBillingState_DemoProfilePreservesCanonicalState(t *testing.T) {
	for _, mode := range []string{"encrypted", "legacy-plaintext", "tampered-signature"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
			writeTestEncryptionKey(t, dir)
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("DEMO_MODE=true\nPRIVATE_SYNTHETIC=unchanged\n"), 0o600))
			store := NewFileBillingStore(dir)
			state := &pkglicensing.BillingState{
				Capabilities:            []string{pkglicensing.FeatureDemoFixtures},
				SubscriptionState:       pkglicensing.SubStateActive,
				EntitlementJWT:          "synthetic-lease.jwt.value",
				EntitlementRefreshToken: "synthetic-refresh-token",
			}
			require.NoError(t, store.SaveBillingState("default", state))
			billingPath := filepath.Join(dir, "billing.json")
			if mode != "encrypted" {
				data, err := os.ReadFile(billingPath)
				require.NoError(t, err)
				var raw pkglicensing.BillingState
				require.NoError(t, json.Unmarshal(data, &raw))
				if mode == "legacy-plaintext" {
					// The same valid signature covers the unencrypted historical values.
					raw.EntitlementJWT = state.EntitlementJWT
					raw.EntitlementRefreshToken = state.EntitlementRefreshToken
				} else {
					raw.Integrity = "invalid-synthetic-signature"
				}
				data, err = json.Marshal(&raw)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(billingPath, data, 0o600))
			}
			beforeBilling, err := os.ReadFile(billingPath)
			require.NoError(t, err)
			keyPath := filepath.Join(dir, ".encryption.key")
			beforeKey, err := os.ReadFile(keyPath)
			require.NoError(t, err)
			info, err := os.Stat(billingPath)
			require.NoError(t, err)
			helper, err := filepath.Abs(filepath.Join("..", "..", ".github", "scripts", "demo-runtime-transaction.py"))
			require.NoError(t, err)
			script := `import importlib.util,pathlib,sys
s=importlib.util.spec_from_file_location("demo",sys.argv[1]); m=importlib.util.module_from_spec(s); s.loader.exec_module(m)
m.PATHS={**m.PATHS,"data":pathlib.Path(sys.argv[2])}
request={"profile":{**{name:"2" for name in m.COUNT_KEYS},"seed_duration":"2h","sample_interval":"5m","update_interval":"15s"}}
m.Host().profile(request,False)
assert m.Host().profile_matches(request)
`
			cmd := exec.Command("python3", "-c", script, helper, dir)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "synthetic demo profile failed: %s", output)
			require.Empty(t, output, "helper must not disclose any billing or key material")
			afterBilling, err := os.ReadFile(billingPath)
			require.NoError(t, err)
			require.Equal(t, beforeBilling, afterBilling, "billing bytes, including all fields and integrity, must be untouched")
			afterKey, err := os.ReadFile(keyPath)
			require.NoError(t, err)
			require.Equal(t, beforeKey, afterKey, "profile must not rotate or rewrite the key")
			afterInfo, err := os.Stat(billingPath)
			require.NoError(t, err)
			require.Equal(t, info.Mode(), afterInfo.Mode())
			require.Equal(t, info.ModTime(), afterInfo.ModTime())
			// The canonical loader still owns both migration and tamper detection.
			loaded, err := store.GetBillingState("default")
			require.NoError(t, err)
			if mode == "tampered-signature" {
				require.Nil(t, loaded, "profile must not launder invalid signed billing into accepted state")
				return
			}
			require.NotNil(t, loaded)
			require.Equal(t, state.EntitlementJWT, loaded.EntitlementJWT)
			require.Equal(t, state.EntitlementRefreshToken, loaded.EntitlementRefreshToken)
			afterLoad, err := os.ReadFile(billingPath)
			require.NoError(t, err)
			require.False(t, bytes.Contains(afterLoad, []byte(state.EntitlementJWT)))
			require.False(t, bytes.Contains(afterLoad, []byte(state.EntitlementRefreshToken)))
			var persisted pkglicensing.BillingState
			require.NoError(t, json.Unmarshal(afterLoad, &persisted))
			manager, err := store.billingCryptoManager()
			require.NoError(t, err)
			persisted.EntitlementJWT, err = manager.DecryptString(persisted.EntitlementJWT)
			require.NoError(t, err)
			persisted.EntitlementRefreshToken, err = manager.DecryptString(persisted.EntitlementRefreshToken)
			require.NoError(t, err)
			hmacKey, err := store.loadHMACKey()
			require.NoError(t, err)
			require.True(t, verifyBillingIntegrity(&persisted, hmacKey), "preserved state/migration must retain a valid canonical HMAC")
		})
	}
}
