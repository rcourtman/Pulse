package deploy

import (
	"testing"
	"time"
)

func TestReserveAndRelease(t *testing.T) {
	rm := NewReservationManager()

	if err := rm.Reserve("j1", "org-1", 3, 30*time.Minute); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if got := rm.ReservedForOrg("org-1"); got != 3 {
		t.Errorf("expected 3 reserved, got %d", got)
	}

	rm.Release("j1")
	if got := rm.ReservedForOrg("org-1"); got != 0 {
		t.Errorf("expected 0 after release, got %d", got)
	}
}

func TestReserveDuplicate(t *testing.T) {
	rm := NewReservationManager()
	_ = rm.Reserve("j1", "org-1", 2, 30*time.Minute)

	if err := rm.Reserve("j1", "org-1", 1, 30*time.Minute); err == nil {
		t.Fatal("expected error for duplicate reservation")
	}
}

func TestReserveInvalidSlots(t *testing.T) {
	rm := NewReservationManager()
	if err := rm.Reserve("j1", "org-1", 0, 30*time.Minute); err == nil {
		t.Fatal("expected error for zero slots")
	}
	if err := rm.Reserve("j2", "org-1", -1, 30*time.Minute); err == nil {
		t.Fatal("expected error for negative slots")
	}
}

func TestMultiOrgIsolation(t *testing.T) {
	rm := NewReservationManager()
	_ = rm.Reserve("j1", "org-1", 3, 30*time.Minute)
	_ = rm.Reserve("j2", "org-2", 5, 30*time.Minute)

	if got := rm.ReservedForOrg("org-1"); got != 3 {
		t.Errorf("expected 3 for org-1, got %d", got)
	}
	if got := rm.ReservedForOrg("org-2"); got != 5 {
		t.Errorf("expected 5 for org-2, got %d", got)
	}
	if got := rm.ReservedForOrg("org-3"); got != 0 {
		t.Errorf("expected 0 for unknown org, got %d", got)
	}
}

func TestMultipleReservationsSameOrg(t *testing.T) {
	rm := NewReservationManager()
	_ = rm.Reserve("j1", "org-1", 2, 30*time.Minute)
	_ = rm.Reserve("j2", "org-1", 3, 30*time.Minute)

	if got := rm.ReservedForOrg("org-1"); got != 5 {
		t.Errorf("expected 5 total, got %d", got)
	}

	rm.Release("j1")
	if got := rm.ReservedForOrg("org-1"); got != 3 {
		t.Errorf("expected 3 after partial release, got %d", got)
	}
}

func TestReserveInvalidTTL(t *testing.T) {
	rm := NewReservationManager()
	if err := rm.Reserve("j1", "org-1", 3, 0); err == nil {
		t.Fatal("expected error for zero TTL")
	}
	if err := rm.Reserve("j2", "org-1", 3, -1*time.Second); err == nil {
		t.Fatal("expected error for negative TTL")
	}
}

func TestTTLExpiry(t *testing.T) {
	rm := NewReservationManager()

	// Directly inject an already-expired reservation to test expiry counting.
	rm.mu.Lock()
	rm.reservations["j1"] = &Reservation{
		JobID: "j1", OrgID: "org-1", Slots: 3,
		CreatedAt: time.Now().UTC().Add(-2 * time.Minute),
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
	}
	rm.mu.Unlock()

	// Expired reservations should not be counted.
	if got := rm.ReservedForOrg("org-1"); got != 0 {
		t.Errorf("expected 0 for expired reservation, got %d", got)
	}
}

func TestCleanExpired(t *testing.T) {
	rm := NewReservationManager()

	// Directly inject an already-expired reservation.
	rm.mu.Lock()
	rm.reservations["j1"] = &Reservation{
		JobID: "j1", OrgID: "org-1", Slots: 2,
		CreatedAt: time.Now().UTC().Add(-2 * time.Minute),
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
	}
	rm.mu.Unlock()

	_ = rm.Reserve("j2", "org-1", 3, 30*time.Minute) // still valid

	rm.CleanExpired()

	// j1 should be cleaned up, only j2 remains.
	if got := rm.ReservedForOrg("org-1"); got != 3 {
		t.Errorf("expected 3 after cleanup, got %d", got)
	}

	// Verify j1 was actually removed (can re-reserve that ID).
	if err := rm.Reserve("j1", "org-1", 1, 30*time.Minute); err != nil {
		t.Errorf("expected to re-reserve cleaned-up job, got: %v", err)
	}
}

func TestReleaseNonexistent(t *testing.T) {
	rm := NewReservationManager()
	// Should not panic.
	rm.Release("nonexistent")
}
