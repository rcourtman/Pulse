package licensing

import (
	"errors"
	"testing"
)

func TestClassifyPersistedLicenseLoadError(t *testing.T) {
	t.Run("nil error yields no status", func(t *testing.T) {
		if status := ClassifyPersistedLicenseLoadError(nil); status != nil {
			t.Fatalf("expected nil status, got %+v", status)
		}
	})

	t.Run("load failure is terminal with re-enter-key action", func(t *testing.T) {
		status := ClassifyPersistedLicenseLoadError(errors.New("failed to decrypt license: cipher: message authentication failed"))
		if status == nil {
			t.Fatal("expected status, got nil")
		}
		if status.Source != CommercialMigrationSourceV5License {
			t.Errorf("Source = %q, want %q", status.Source, CommercialMigrationSourceV5License)
		}
		if status.State != CommercialMigrationStateFailed {
			t.Errorf("State = %q, want %q", status.State, CommercialMigrationStateFailed)
		}
		if status.Reason != CommercialMigrationReasonPersistedUnreadable {
			t.Errorf("Reason = %q, want %q", status.Reason, CommercialMigrationReasonPersistedUnreadable)
		}
		if status.RecommendedAction != CommercialMigrationActionEnterSupportedV5 {
			t.Errorf("RecommendedAction = %q, want %q", status.RecommendedAction, CommercialMigrationActionEnterSupportedV5)
		}
	})

	t.Run("survives contract normalization", func(t *testing.T) {
		status := NormalizeCommercialMigrationStatus(ClassifyPersistedLicenseLoadError(errors.New("unreadable")))
		if status == nil {
			t.Fatal("normalization dropped the load-failure status")
		}
		if status.Reason != CommercialMigrationReasonPersistedUnreadable {
			t.Errorf("Reason = %q, want %q", status.Reason, CommercialMigrationReasonPersistedUnreadable)
		}
	})
}
