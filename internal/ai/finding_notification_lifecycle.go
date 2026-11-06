package ai

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// NotificationOccurrenceLifecycle projects existing finding facts into the
// notification owner's lifecycle seam. It creates no history or expiry policy.
// In particular, dismissal, snooze and suppression are not resolution.
func (s *FindingsStore) NotificationOccurrenceLifecycle(id string, start time.Time) alerts.NotificationOccurrenceLifecycle {
	unknown := alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
	if s == nil || id == "" || start.IsZero() {
		return unknown
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.persistence != nil && !s.persistenceLoaded {
		return unknown
	}
	f := s.findings[id]
	if f == nil {
		// Only a successfully loaded current inventory can confirm absence.
		// Even that is not resolution: cleanup still needs the receipt's own
		// previously observed positive terminal fact and its existing grace.
		if s.persistenceLoaded {
			return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceNoRecord, CurrentInactive: true}
		}
		return unknown
	}
	if !f.DetectedAt.Equal(start) {
		return unknown
	}
	activation := f.DetectedAt
	if f.LastRegressionAt != nil && f.LastRegressionAt.After(activation) {
		activation = *f.LastRegressionAt
	}
	if f.ResolvedAt == nil {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive, LatestActivationAt: activation}
	}
	if f.ResolvedAt.IsZero() || f.ResolvedAt.Before(activation) {
		return unknown
	}
	return alerts.NotificationOccurrenceLifecycle{
		State: alerts.NotificationOccurrenceResolved, CurrentInactive: true,
		ResolvedAt: *f.ResolvedAt, LatestActivationAt: activation,
	}
}
