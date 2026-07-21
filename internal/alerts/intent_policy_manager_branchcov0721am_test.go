package alerts

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestBranchcov0721amIntentPolicyManager raises branch coverage on three
// *Manager methods: UpdateIntentPolicies, GetIntentPolicies, and
// SetBackupIntentContextResolver. Each arm uses t.Run so the test name
// prefix matches the -run filter requested by the task brief.
func TestBranchcov0721amIntentPolicyManager(t *testing.T) {
	const wantSchema = CurrentAlertIntentPolicySchemaVersion

	// validBranchcov0721amDoc builds a document that is guaranteed to pass
	// ValidateAlertIntentPolicyDocument with the requested revision. The
	// shape mirrors the construction used in intent_policy_test.go.
	validBranchcov0721amDoc := func(revision int64) AlertIntentPolicyDocument {
		doc := NewAlertIntentPolicyDocument()
		doc.Revision = revision
		doc.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{
			GraceSeconds:       intPointer(45),
			HonorOperatorState: boolPointer(true),
		}
		return doc
	}

	// --- UpdateIntentPolicies: arm (a) invalid document ---
	t.Run("UpdateIntentPolicies_invalid_document_returns_validation_error", func(t *testing.T) {
		m := NewManagerWithDataDir(t.TempDir())
		t.Cleanup(m.Stop)

		invalid := NewAlertIntentPolicyDocument()
		invalid.SchemaVersion = wantSchema + 7 // unsupported schema triggers the validator
		if err := ValidateAlertIntentPolicyDocument(invalid); err == nil {
			t.Fatalf("precondition: expected doc to be invalid")
		}

		returned, err := m.UpdateIntentPolicies(invalid)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported alert intent policy schema version") {
			t.Fatalf("expected schema-version validation error, got %v", err)
		}
		if returned.SchemaVersion != 0 || returned.Revision != 0 || returned.Defaults != nil || returned.ResourceTypes != nil || returned.Resources != nil {
			t.Fatalf("expected zero-value document on validation failure, got %+v", returned)
		}
	})

	// --- UpdateIntentPolicies: arm (b) nil manager ---
	t.Run("UpdateIntentPolicies_nil_manager_returns_nil_manager_error", func(t *testing.T) {
		var m *Manager
		returned, err := m.UpdateIntentPolicies(validBranchcov0721amDoc(0))
		if err == nil {
			t.Fatalf("expected error from nil manager, got nil")
		}
		if !strings.Contains(err.Error(), "alert manager is nil") {
			t.Fatalf("expected 'alert manager is nil' error, got %v", err)
		}
		if returned.SchemaVersion != 0 || returned.Revision != 0 {
			t.Fatalf("expected zero-value document from nil manager, got %+v", returned)
		}
	})

	// --- UpdateIntentPolicies: arm (c) revision conflict ---
	t.Run("UpdateIntentPolicies_revision_conflict_returns_sentinel_and_leaves_state", func(t *testing.T) {
		m := NewManagerWithDataDir(t.TempDir())
		t.Cleanup(m.Stop)

		seed := validBranchcov0721amDoc(7)
		if err := m.LoadIntentPolicies(seed); err != nil {
			t.Fatalf("LoadIntentPolicies error: %v", err)
		}
		currentRevision := m.GetIntentPolicies().Revision
		if currentRevision != 7 {
			t.Fatalf("expected loaded revision 7, got %d", currentRevision)
		}

		conflict := validBranchcov0721amDoc(currentRevision + 1) // wrong revision
		returned, err := m.UpdateIntentPolicies(conflict)
		if !errors.Is(err, ErrAlertIntentPolicyRevisionConflict) {
			t.Fatalf("expected errors.Is(err, ErrAlertIntentPolicyRevisionConflict), got %v", err)
		}
		if returned.SchemaVersion != 0 || returned.Revision != 0 {
			t.Fatalf("expected zero-value document on conflict, got %+v", returned)
		}
		// Stored policy state must be unchanged.
		if got := m.GetIntentPolicies().Revision; got != currentRevision {
			t.Fatalf("expected revision unchanged after conflict, got %d", got)
		}
	})

	// --- UpdateIntentPolicies: arm (d) success ---
	t.Run("UpdateIntentPolicies_success_bumps_revision_and_round_trips", func(t *testing.T) {
		m := NewManagerWithDataDir(t.TempDir())
		t.Cleanup(m.Stop)

		// Freeze the clock so the UpdatedAt stamp is observable & reproducible.
		frozen := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
		m.now = func() time.Time { return frozen }

		seed := validBranchcov0721amDoc(3)
		if err := m.LoadIntentPolicies(seed); err != nil {
			t.Fatalf("LoadIntentPolicies error: %v", err)
		}
		currentRevision := m.GetIntentPolicies().Revision
		if currentRevision != 3 {
			t.Fatalf("expected loaded revision 3, got %d", currentRevision)
		}

		// Build the update payload with the matching revision but mutate the
		// policy body so the round-trip reflects the new content.
		update := validBranchcov0721amDoc(currentRevision)
		update.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{
			GraceSeconds:       intPointer(120),
			HonorOperatorState: boolPointer(false),
		}
		returned, err := m.UpdateIntentPolicies(update)
		if err != nil {
			t.Fatalf("UpdateIntentPolicies unexpected error: %v", err)
		}
		if returned.Revision != currentRevision+1 {
			t.Fatalf("expected returned revision %d, got %d", currentRevision+1, returned.Revision)
		}
		if returned.UpdatedAt == nil {
			t.Fatalf("expected UpdatedAt to be set on returned document")
		}
		if !returned.UpdatedAt.Equal(frozen) {
			t.Fatalf("expected UpdatedAt %v, got %v", frozen, *returned.UpdatedAt)
		}
		if returned.SchemaVersion != wantSchema {
			t.Fatalf("expected schema version %d, got %d", wantSchema, returned.SchemaVersion)
		}

		// Returned doc must equal the normalized shape of the update body
		// (with the bumped revision + UpdatedAt applied).
		expectedNormalized := NormalizeAlertIntentPolicyDocument(update)
		expectedNormalized.Revision = currentRevision + 1
		expectedUpdatedAt := frozen.UTC()
		expectedNormalized.UpdatedAt = &expectedUpdatedAt
		if !branchcov0721amDocsEqual(returned, expectedNormalized) {
			t.Fatalf("returned doc shape mismatch\n got=%+v\nwant=%+v", returned, expectedNormalized)
		}

		// A subsequent GetIntentPolicies must reflect the new revision and content.
		got := m.GetIntentPolicies()
		if got.Revision != currentRevision+1 {
			t.Fatalf("expected GetIntentPolicies revision %d, got %d", currentRevision+1, got.Revision)
		}
		rule, ok := got.Defaults[string(AlertIntentSignalOffline)]
		if !ok {
			t.Fatalf("expected defaults entry for %q after update", AlertIntentSignalOffline)
		}
		if rule.GraceSeconds == nil || *rule.GraceSeconds != 120 {
			t.Fatalf("expected grace seconds 120 after update, got %+v", rule)
		}
		if rule.HonorOperatorState == nil || *rule.HonorOperatorState != false {
			t.Fatalf("expected honorOperatorState false after update, got %+v", rule)
		}
	})

	// --- GetIntentPolicies: nil manager ---
	t.Run("GetIntentPolicies_nil_manager_returns_factory_default", func(t *testing.T) {
		var m *Manager
		got := m.GetIntentPolicies()
		expected := NewAlertIntentPolicyDocument()
		if !branchcov0721amDocsEqual(got, expected) {
			t.Fatalf("nil-manager GetIntentPolicies mismatch\n got=%+v\nwant=%+v", got, expected)
		}
	})

	// --- GetIntentPolicies: populated manager (round-trip after Load) ---
	t.Run("GetIntentPolicies_populated_manager_round_trips_stored_doc", func(t *testing.T) {
		m := NewManagerWithDataDir(t.TempDir())
		t.Cleanup(m.Stop)

		seed := validBranchcov0721amDoc(11)
		if err := m.LoadIntentPolicies(seed); err != nil {
			t.Fatalf("LoadIntentPolicies error: %v", err)
		}
		got := m.GetIntentPolicies()
		expected := NormalizeAlertIntentPolicyDocument(seed)
		if !branchcov0721amDocsEqual(got, expected) {
			t.Fatalf("populated GetIntentPolicies mismatch\n got=%+v\nwant=%+v", got, expected)
		}
	})

	// --- SetBackupIntentContextResolver: nil manager (no panic) ---
	t.Run("SetBackupIntentContextResolver_nil_manager_no_panic", func(t *testing.T) {
		var m *Manager
		m.SetBackupIntentContextResolver(func(resourceID, instance, node string, vmid int, now time.Time) (BackupIntentContext, bool) {
			return BackupIntentContext{Active: true, Evidence: "from-nil-arm"}, true
		})
		// Reaching here without panic is the assertion. Also verify the nil
		// manager still reports the factory-default policy.
		if got := m.GetIntentPolicies(); got.Revision != 0 || got.SchemaVersion != wantSchema {
			t.Fatalf("nil manager policy unexpectedly mutated: %+v", got)
		}
	})

	// --- SetBackupIntentContextResolver: non-nil manager (stores resolver) ---
	t.Run("SetBackupIntentContextResolver_non_nil_manager_stores_resolver", func(t *testing.T) {
		m := NewManagerWithDataDir(t.TempDir())
		t.Cleanup(m.Stop)

		var capturedResource, capturedInstance, capturedNode string
		var capturedVMID int
		var capturedNow time.Time
		resolver := func(resourceID, instance, node string, vmid int, now time.Time) (BackupIntentContext, bool) {
			capturedResource = resourceID
			capturedInstance = instance
			capturedNode = node
			capturedVMID = vmid
			capturedNow = now
			return BackupIntentContext{Active: true, Evidence: "nightly"}, true
		}
		m.SetBackupIntentContextResolver(resolver)

		// The setter must have written the resolver field.
		m.mu.RLock()
		stored := m.backupIntentResolver
		m.mu.RUnlock()
		if stored == nil {
			t.Fatalf("expected backup resolver to be stored on manager")
		}
		// Invoke the stored resolver to confirm it is the function we
		// registered (i.e. SetBackupIntentContextResolver wrote the field).
		frozen := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
		ctx, ok := stored("vm:101", "qemu", "node-a", 101, frozen)
		if !ok || !ctx.Active || ctx.Evidence != "nightly" {
			t.Fatalf("stored resolver returned unexpected value: %+v ok=%v", ctx, ok)
		}
		if capturedResource != "vm:101" || capturedInstance != "qemu" || capturedNode != "node-a" || capturedVMID != 101 || !capturedNow.Equal(frozen) {
			t.Fatalf("stored resolver captured unexpected args: resource=%q instance=%q node=%q vmid=%d now=%v",
				capturedResource, capturedInstance, capturedNode, capturedVMID, capturedNow)
		}
	})
}

// branchcov0721amDocsEqual compares two documents field-by-field for the
// assertions above. Avoids reflect to keep dependencies minimal.
func branchcov0721amDocsEqual(a, b AlertIntentPolicyDocument) bool {
	if a.SchemaVersion != b.SchemaVersion || a.Revision != b.Revision {
		return false
	}
	if (a.UpdatedAt == nil) != (b.UpdatedAt == nil) {
		return false
	}
	if a.UpdatedAt != nil && !a.UpdatedAt.Equal(*b.UpdatedAt) {
		return false
	}
	if !branchcov0721amSignalRulesEqual(a.Defaults, b.Defaults) {
		return false
	}
	if len(a.ResourceTypes) != len(b.ResourceTypes) {
		return false
	}
	for k, av := range a.ResourceTypes {
		bv, ok := b.ResourceTypes[k]
		if !ok || !branchcov0721amSignalRulesEqual(av, bv) {
			return false
		}
	}
	if len(a.Resources) != len(b.Resources) {
		return false
	}
	for k, av := range a.Resources {
		bv, ok := b.Resources[k]
		if !ok || !branchcov0721amSignalRulesEqual(av, bv) {
			return false
		}
	}
	return true
}

func branchcov0721amSignalRulesEqual(a, b map[string]AlertIntentRule) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if (av.GraceSeconds == nil) != (bv.GraceSeconds == nil) {
			return false
		}
		if av.GraceSeconds != nil && *av.GraceSeconds != *bv.GraceSeconds {
			return false
		}
		if (av.HonorOperatorState == nil) != (bv.HonorOperatorState == nil) {
			return false
		}
		if av.HonorOperatorState != nil && *av.HonorOperatorState != *bv.HonorOperatorState {
			return false
		}
		if (av.BackupOffline == nil) != (bv.BackupOffline == nil) {
			return false
		}
		if av.BackupOffline != nil && *av.BackupOffline != *bv.BackupOffline {
			return false
		}
	}
	return true
}
