package updates

import (
	"errors"
	"time"
)

// Update check outcomes are a closed vocabulary shared with usage telemetry.
// A failed check never reaches the update history (nothing was applied), so
// without this record an install whose check is broken is indistinguishable
// from one that was offered an update and ignored it (#2285).
const (
	UpdateCheckOutcomeNotChecked    = "not_checked"
	UpdateCheckOutcomeUpToDate      = "up_to_date"
	UpdateCheckOutcomeAvailable     = "available"
	UpdateCheckOutcomeNoRelease     = "no_release"
	UpdateCheckOutcomeRateLimited   = "rate_limited"
	UpdateCheckOutcomeNetworkError  = "network_error"
	UpdateCheckOutcomeMetadataError = "metadata_error"
	UpdateCheckOutcomeSkipped       = "skipped"
	UpdateCheckOutcomeError         = "error"
)

// UpdateCheckObservation is the content-free result of the most recent update
// check on the install's effective channel. It carries no version strings,
// URLs, or error text.
type UpdateCheckObservation struct {
	Channel   string
	Outcome   string
	Available bool
	CheckedAt time.Time
}

// updateCheckError tags a release lookup failure with its outcome category
// while leaving the wrapped error message unchanged.
type updateCheckError struct {
	outcome string
	err     error
}

func (e *updateCheckError) Error() string { return e.err.Error() }
func (e *updateCheckError) Unwrap() error { return e.err }

func withUpdateCheckOutcome(outcome string, err error) error {
	if err == nil {
		return nil
	}
	return &updateCheckError{outcome: outcome, err: err}
}

func updateCheckErrorOutcome(err error) string {
	if errors.Is(err, errGitHubRateLimited) {
		return UpdateCheckOutcomeRateLimited
	}
	var tagged *updateCheckError
	if errors.As(err, &tagged) {
		return tagged.outcome
	}
	return UpdateCheckOutcomeError
}

// recordUpdateCheck stores the outcome of a check on the effective channel.
// Checks that preview another channel (an explicit UI override) are ignored so
// the observation always describes what the install itself would be offered.
func (m *Manager) recordUpdateCheck(channel string, effective bool, outcome string, available bool) {
	if !effective {
		return
	}
	m.statusMu.Lock()
	m.lastCheck = UpdateCheckObservation{
		Channel:   channel,
		Outcome:   outcome,
		Available: available,
		CheckedAt: time.Now().UTC(),
	}
	m.statusMu.Unlock()
}

// LastUpdateCheck returns the most recent effective-channel check result, with
// the currently effective channel and a not_checked outcome before any check.
func (m *Manager) LastUpdateCheck() UpdateCheckObservation {
	if m == nil {
		return UpdateCheckObservation{Outcome: UpdateCheckOutcomeNotChecked}
	}
	currentInfo, _ := GetCurrentVersion()
	channel := m.resolveChannel("", currentInfo)

	m.statusMu.RLock()
	observation := m.lastCheck
	m.statusMu.RUnlock()

	if observation.Outcome == "" || observation.Channel != channel {
		return UpdateCheckObservation{Channel: channel, Outcome: UpdateCheckOutcomeNotChecked}
	}
	return observation
}
