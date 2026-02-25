package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

// Ensure FileBillingStore satisfies the hosted entitlement BillingStore interface.
var _ pkglicensing.BillingStore = (*FileBillingStore)(nil)

// FileBillingStore persists billing state in per-org files under the data directory.
type FileBillingStore struct {
	baseDataDir string
	mu          sync.RWMutex
}

// NewFileBillingStore creates a file-backed billing store rooted at baseDataDir.
func NewFileBillingStore(baseDataDir string) *FileBillingStore {
	return &FileBillingStore{baseDataDir: baseDataDir}
}

// GetBillingState returns the current billing state for an org.
// Missing billing files are treated as "no state yet" and return (nil, nil).
// If the state has been tampered with (invalid HMAC), it is treated as nonexistent.
func (s *FileBillingStore) GetBillingState(orgID string) (*pkglicensing.BillingState, error) {
	billingPath, err := s.billingStatePath(orgID)
	if err != nil {
		return nil, err
	}

	// Read file under read lock, then release before potential migration write.
	s.mu.RLock()
	data, err := os.ReadFile(billingPath)
	s.mu.RUnlock()

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read billing state for org %q: %w", orgID, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var state pkglicensing.BillingState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode billing state for org %q: %w", orgID, err)
	}

	// Integrity verification: derive HMAC key from .encryption.key.
	// If the key is unavailable (new install, key not yet created), skip checks.
	hmacKey, keyErr := s.loadHMACKey()
	if keyErr == nil {
		if state.Integrity == "" {
			// Migration: pre-upgrade state without integrity. Compute and persist.
			state.Integrity = billingIntegrity(&state, hmacKey)
			if saveErr := s.SaveBillingState(orgID, &state); saveErr != nil {
				log.Warn().
					Err(saveErr).
					Str("org_id", orgID).
					Msg("Failed to persist billing integrity migration")
			}
		} else if !verifyBillingIntegrity(&state, hmacKey) {
			// Check legacy HMAC format (without Limits) for format migration.
			if verifyBillingIntegrityLegacy(&state, hmacKey) {
				// Valid legacy signature — re-sign with current format and persist.
				state.Integrity = billingIntegrity(&state, hmacKey)
				if saveErr := s.SaveBillingState(orgID, &state); saveErr != nil {
					log.Warn().
						Err(saveErr).
						Str("org_id", orgID).
						Msg("Failed to persist billing integrity format migration")
				}
			} else {
				// Tampered state — treat as nonexistent (free tier).
				return nil, nil
			}
		}
	}

	return &state, nil
}

// SaveBillingState persists billing state for an org to billing.json.
func (s *FileBillingStore) SaveBillingState(orgID string, state *pkglicensing.BillingState) error {
	if state == nil {
		return errors.New("billing state is required")
	}

	// Compute integrity HMAC if encryption key is available.
	if hmacKey, err := s.loadHMACKey(); err == nil {
		state.Integrity = billingIntegrity(state, hmacKey)
	}

	billingPath, err := s.billingStatePath(orgID)
	if err != nil {
		return fmt.Errorf("resolve billing state path for org %q: %w", orgID, err)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode billing state for org %q: %w", orgID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(billingPath), 0o700); err != nil {
		return fmt.Errorf("create billing directory for org %q: %w", orgID, err)
	}

	tmpPath := billingPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp billing state for org %q: %w", orgID, err)
	}
	if err := os.Rename(tmpPath, billingPath); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Warn().
				Err(removeErr).
				Str("tmp_path", tmpPath).
				Str("org_id", orgID).
				Msg("Failed to remove temporary billing state file after failed rename")
		}
		return fmt.Errorf("commit billing state for org %q: %w", orgID, err)
	}

	return nil
}

func (s *FileBillingStore) billingStatePath(orgID string) (string, error) {
	orgID = strings.TrimSpace(orgID)
	if !isValidOrgID(orgID) {
		return "", fmt.Errorf("invalid organization ID: %s", orgID)
	}
	// Default org stores config at the root data dir for backward compatibility,
	// so billing state for the default org must live alongside other root configs.
	if orgID == "default" {
		return filepath.Join(s.resolveDataDir(), "billing.json"), nil
	}
	return filepath.Join(s.resolveDataDir(), "orgs", orgID, "billing.json"), nil
}

func (s *FileBillingStore) resolveDataDir() string {
	if dir := strings.TrimSpace(s.baseDataDir); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("PULSE_DATA_DIR")); dir != "" {
		return dir
	}
	return "/etc/pulse"
}

// loadHMACKey derives a purpose-specific HMAC key from the .encryption.key file.
// Returns an error if the key file is missing or invalid (graceful degradation).
func (s *FileBillingStore) loadHMACKey() ([]byte, error) {
	keyPath := filepath.Join(s.resolveDataDir(), ".encryption.key")
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(raw))
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(trimmed)))
	n, err := base64.StdEncoding.Decode(decoded, []byte(trimmed))
	if err != nil || n != 32 {
		return nil, fmt.Errorf("invalid encryption key")
	}

	// Domain-separated key: SHA256("pulse-billing-integrity-" || raw_key)
	h := sha256.New()
	h.Write([]byte("pulse-billing-integrity-"))
	h.Write(decoded[:n])
	return h.Sum(nil), nil
}

// billingIntegrityPayload contains only the entitlement-critical fields used
// for HMAC computation. Non-critical metadata (e.g. Stripe IDs) is excluded so
// adding informational fields to BillingState won't break existing signatures.
//
// IMPORTANT: When adding a new field to BillingState that gates entitlements or
// affects billing logic, add it here too. Existing on-disk signatures will
// auto-migrate on next read (see GetBillingState migration path).
type billingIntegrityPayload struct {
	Capabilities      []string                       `json:"capabilities"`
	Limits            map[string]int64               `json:"limits"`
	PlanVersion       string                         `json:"plan_version"`
	SubscriptionState pkglicensing.SubscriptionState `json:"subscription_state"`
	TrialStartedAt    *int64                         `json:"trial_started_at"`
	TrialEndsAt       *int64                         `json:"trial_ends_at"`
	TrialExtendedAt   *int64                         `json:"trial_extended_at"`
}

// billingIntegrity computes the HMAC-SHA256 over the critical billing fields.
func billingIntegrity(state *pkglicensing.BillingState, key []byte) string {
	caps := make([]string, len(state.Capabilities))
	copy(caps, state.Capabilities)
	sort.Strings(caps)

	// Clone and canonicalize limits: nil → empty map for deterministic JSON.
	// Snapshot avoids aliasing; callers must not mutate state concurrently.
	limits := make(map[string]int64, len(state.Limits))
	for k, v := range state.Limits {
		limits[k] = v
	}

	payload := billingIntegrityPayload{
		Capabilities:      caps,
		Limits:            limits,
		PlanVersion:       state.PlanVersion,
		SubscriptionState: state.SubscriptionState,
		TrialStartedAt:    state.TrialStartedAt,
		TrialEndsAt:       state.TrialEndsAt,
		TrialExtendedAt:   state.TrialExtendedAt,
	}

	data, _ := json.Marshal(payload) // struct marshal cannot fail
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyBillingIntegrity checks whether the stored HMAC matches the computed one.
func verifyBillingIntegrity(state *pkglicensing.BillingState, key []byte) bool {
	expected := billingIntegrity(state, key)
	return hmac.Equal([]byte(expected), []byte(state.Integrity))
}

// billingIntegrityPayloadLegacy is the pre-v6 HMAC payload format (without Limits).
// Kept only for migration verification — new signatures always use billingIntegrityPayload.
type billingIntegrityPayloadLegacy struct {
	Capabilities      []string                       `json:"capabilities"`
	PlanVersion       string                         `json:"plan_version"`
	SubscriptionState pkglicensing.SubscriptionState `json:"subscription_state"`
	TrialStartedAt    *int64                         `json:"trial_started_at"`
	TrialEndsAt       *int64                         `json:"trial_ends_at"`
	TrialExtendedAt   *int64                         `json:"trial_extended_at"`
}

// billingIntegrityLegacy computes the legacy HMAC (without Limits) for migration checks.
func billingIntegrityLegacy(state *pkglicensing.BillingState, key []byte) string {
	caps := make([]string, len(state.Capabilities))
	copy(caps, state.Capabilities)
	sort.Strings(caps)

	payload := billingIntegrityPayloadLegacy{
		Capabilities:      caps,
		PlanVersion:       state.PlanVersion,
		SubscriptionState: state.SubscriptionState,
		TrialStartedAt:    state.TrialStartedAt,
		TrialEndsAt:       state.TrialEndsAt,
		TrialExtendedAt:   state.TrialExtendedAt,
	}

	data, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyBillingIntegrityLegacy checks if a stored HMAC was signed with the legacy format.
func verifyBillingIntegrityLegacy(state *pkglicensing.BillingState, key []byte) bool {
	expected := billingIntegrityLegacy(state, key)
	return hmac.Equal([]byte(expected), []byte(state.Integrity))
}
