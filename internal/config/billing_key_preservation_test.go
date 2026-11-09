package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBillingCryptoFailurePreservesSavedLease(t *testing.T) {
	for _, orgID := range []string{"default", "department-a"} {
		for _, operation := range []string{"read_encrypted", "read_legacy", "save"} {
			t.Run(orgID+"/"+operation, func(t *testing.T) {
				store, path, state := privateBillingFixture(t, orgID)
				if err := store.SaveBillingState(orgID, state); err != nil {
					t.Fatal(err)
				}
				// Before corruption, record the canonical result rather than assuming
				// normalization leaves the input untouched.
				original, err := store.GetBillingState(orgID)
				if err != nil || original == nil {
					t.Fatalf("seed read failed: %v", err)
				}
				if operation == "read_legacy" {
					raw, err := json.Marshal(state)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
				saved, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				keyPath := filepath.Join(store.baseDataDir, ".encryption.key")
				originalKey, err := os.ReadFile(keyPath)
				if err != nil {
					t.Fatal(err)
				}
				invalid := []byte("invalid-synthetic-key-never-replace-or-disclose")
				if err := os.WriteFile(keyPath, invalid, 0600); err != nil {
					t.Fatal(err)
				}
				if operation == "save" {
					// Error must not normalize the caller, erase either secret, or publish
					// the requested replacement over the previously signed state.
					requested := *state
					requested.StripeCustomerID = "  replacement-synthetic-customer  "
					before := requested
					err = store.SaveBillingState(orgID, &requested)
					if err == nil || !strings.Contains(err.Error(), "refusing to replace") {
						t.Errorf("failed crypto must refuse save: %v", err)
					}
					if !reflect.DeepEqual(before, requested) {
						t.Error("failed billing encryption changed the caller's state")
					}
				} else {
					loaded, err := store.GetBillingState(orgID)
					if err == nil || loaded != nil || !strings.Contains(err.Error(), "refusing to replace") {
						t.Errorf("failed crypto must be unknown, not empty/dropped/migrated authority: state=%t error=%v", loaded != nil, err)
					}
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(after, saved) {
					t.Errorf("failed crypto rewrote the original billing file: %v", err)
				}
				afterInfo, err := os.Stat(path)
				if err != nil || !os.SameFile(info, afterInfo) || info.Mode() != afterInfo.Mode() {
					t.Errorf("failed crypto replaced billing inode/mode: %v", err)
				}
				if got, err := os.ReadFile(keyPath); err != nil || !bytes.Equal(got, invalid) {
					t.Errorf("billing operation replaced invalid key: %v", err)
				}
				leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".billing.json.*.tmp"))
				if err != nil || len(leftovers) != 0 {
					t.Errorf("failed crypto left private staging files: %v", err)
				}

				if err := os.WriteFile(keyPath, originalKey, 0600); err != nil {
					t.Fatal(err)
				}
				recovered, err := store.GetBillingState(orgID)
				if err != nil || !reflect.DeepEqual(original, recovered) {
					t.Errorf("original key did not recover the same saved lease/identity: %v", err)
				}
				if operation != "read_legacy" {
					if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, saved) {
						t.Errorf("recovered read rewrote original encrypted billing bytes: %v", err)
					}
				} else {
					raw, err := os.ReadFile(path)
					if err != nil || bytes.Contains(raw, []byte(state.EntitlementJWT)) || bytes.Contains(raw, []byte(state.EntitlementRefreshToken)) {
						t.Errorf("restored legacy migration did not keep secrets encrypted: %v", err)
					}
				}
			})
		}
	}
}
