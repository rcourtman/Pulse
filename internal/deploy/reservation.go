package deploy

import (
	"fmt"
	"sync"
	"time"
)

// ReservationManager tracks license slot reservations for in-flight deployments.
type ReservationManager struct {
	mu           sync.Mutex
	reservations map[string]*Reservation
}

// Reservation represents a set of license slots reserved for a deployment job.
type Reservation struct {
	JobID     string
	OrgID     string
	Slots     int
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewReservationManager creates a new reservation manager.
func NewReservationManager() *ReservationManager {
	return &ReservationManager{
		reservations: make(map[string]*Reservation),
	}
}

// Reserve allocates license slots for a deployment job. Returns an error if
// the job already has a reservation.
func (rm *ReservationManager) Reserve(jobID, orgID string, slots int, ttl time.Duration) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.reservations[jobID]; exists {
		return fmt.Errorf("reservation already exists for job %q", jobID)
	}
	if slots <= 0 {
		return fmt.Errorf("slots must be positive, got %d", slots)
	}
	if ttl <= 0 {
		return fmt.Errorf("ttl must be positive, got %v", ttl)
	}

	now := time.Now().UTC()
	rm.reservations[jobID] = &Reservation{
		JobID:     jobID,
		OrgID:     orgID,
		Slots:     slots,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	return nil
}

// Release removes the reservation for a job.
func (rm *ReservationManager) Release(jobID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.reservations, jobID)
}

// ReservedForOrg returns the total number of reserved slots for a given org,
// excluding expired reservations.
func (rm *ReservationManager) ReservedForOrg(orgID string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now().UTC()
	total := 0
	for _, r := range rm.reservations {
		if r.OrgID == orgID && now.Before(r.ExpiresAt) {
			total += r.Slots
		}
	}
	return total
}

// CleanExpired removes all expired reservations.
func (rm *ReservationManager) CleanExpired() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now().UTC()
	for id, r := range rm.reservations {
		if !now.Before(r.ExpiresAt) {
			delete(rm.reservations, id)
		}
	}
}
