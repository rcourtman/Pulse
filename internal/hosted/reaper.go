package hosted

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const (
	defaultReaperScanInterval = time.Minute
	maxRetentionDays          = int(math.MaxInt64 / int64(24*time.Hour))
	minRetentionDays          = -maxRetentionDays
)

type OrgLister interface {
	ListOrganizations() ([]*models.Organization, error)
}

type OrgDeleter interface {
	DeleteOrganization(orgID string) error
}

type ReapAction string

const (
	ReapActionDeleted ReapAction = "deleted"
	ReapActionDryRun  ReapAction = "dry_run"
)

type ReapResult struct {
	OrgID         string
	RequestedAt   time.Time
	RetentionDays int
	ExpiredAt     time.Time
	Action        ReapAction
	Error         error
}

type Reaper struct {
	lister         OrgLister
	deleter        OrgDeleter
	scanInterval   time.Duration
	liveMode       bool
	now            func() time.Time
	OnBeforeDelete func(orgID string) error
}

func NewReaper(lister OrgLister, deleter OrgDeleter, scanInterval time.Duration, liveMode bool) *Reaper {
	normalizedScanInterval := normalizeReaperScanInterval(scanInterval)
	if normalizedScanInterval != scanInterval {
		log.Warn().
			Dur("scan_interval", scanInterval).
			Dur("default_scan_interval", defaultReaperScanInterval).
			Msg("Hosted reaper scan interval must be positive; defaulting to safe value")
	}

	return &Reaper{
		lister:       lister,
		deleter:      deleter,
		scanInterval: normalizedScanInterval,
		liveMode:     liveMode,
		now:          time.Now,
	}
}

// ScanOnce runs a single scan cycle and returns the results.
// This is used by integration tests and operational tooling to avoid ticker-based timing.
func (r *Reaper) ScanOnce() []ReapResult {
	return r.scan()
}

func (r *Reaper) Run(ctx context.Context) error {
	if r == nil {
		return nil
	}

	scanInterval := normalizeReaperScanInterval(r.scanInterval)
	if scanInterval != r.scanInterval {
		log.Warn().
			Dur("scan_interval", r.scanInterval).
			Dur("default_scan_interval", defaultReaperScanInterval).
			Msg("Hosted reaper runtime scan interval was invalid; defaulting to safe value")
		r.scanInterval = scanInterval
	}

	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.scan()
		}
	}
}

func (r *Reaper) scan() []ReapResult {
	if r == nil {
		return nil
	}
	if r.lister == nil {
		log.Error().
			Bool("live_mode", r.liveMode).
			Dur("scan_interval", r.scanInterval).
			Msg("Hosted reaper scan skipped because lister is nil")
		return nil
	}
	if r.now == nil {
		log.Error().
			Bool("live_mode", r.liveMode).
			Dur("scan_interval", r.scanInterval).
			Msg("Hosted reaper scan skipped because clock is nil")
		return nil
	}

	orgs, err := r.lister.ListOrganizations()
	if err != nil {
		log.Error().
			Err(err).
			Bool("live_mode", r.liveMode).
			Dur("scan_interval", r.scanInterval).
			Msg("Hosted reaper failed to list organizations")
		return nil
	}

	now := r.now()
	results := make([]ReapResult, 0)

	for _, org := range orgs {
		if org == nil {
			continue
		}
		if !isValidOrganizationID(org.ID) {
			log.Warn().
				Str("org_id", org.ID).
				Msg("Hosted reaper skipping organization with invalid ID")
			continue
		}
		if org.ID == "default" {
			continue
		}
		if models.NormalizeOrgStatus(org.Status) != models.OrgStatusPendingDeletion {
			continue
		}
		if org.DeletionRequestedAt == nil {
			continue
		}

		retentionDays, clamped := clampRetentionDays(org.RetentionDays)
		if clamped {
			log.Warn().
				Str("org_id", org.ID).
				Int("retention_days", org.RetentionDays).
				Int("normalized_retention_days", retentionDays).
				Int("min_retention_days", minRetentionDays).
				Int("max_retention_days", maxRetentionDays).
				Msg("Hosted reaper retention days exceeded safe duration bounds; clamped value")
		}

		expiry := org.DeletionRequestedAt.Add(time.Duration(retentionDays) * 24 * time.Hour)
		if now.Before(expiry) {
			continue
		}

		expiredFor := now.Sub(expiry)
		orgLog := log.With().
			Str("org_id", org.ID).
			Time("deletion_requested_at", *org.DeletionRequestedAt).
			Time("deletion_expires_at", expiry).
			Int("retention_days", org.RetentionDays).
			Dur("expired_for", expiredFor).
			Bool("live_mode", r.liveMode).
			Logger()
		orgLog.Info().
			Str("action", "scan_match").
			Msg("Hosted reaper found expired pending-deletion organization")

		result := ReapResult{
			OrgID:         org.ID,
			RequestedAt:   *org.DeletionRequestedAt,
			RetentionDays: retentionDays,
			ExpiredAt:     expiry,
		}

		if r.liveMode {
			result.Action = ReapActionDeleted

			if r.deleter == nil {
				result.Error = errors.New("org deleter is nil")
				orgLog.Error().
					Str("action", result.Action).
					Msg("Hosted reaper is in live mode but deleter is nil")
			} else {
				if r.OnBeforeDelete != nil {
					if err := r.OnBeforeDelete(org.ID); err != nil {
						result.Error = err
						orgLog.Error().
							Err(err).
							Str("action", result.Action).
							Msg("Hosted reaper OnBeforeDelete hook failed")
						results = append(results, result)
						continue
					}
				}
				result.Error = r.deleter.DeleteOrganization(org.ID)
				if result.Error != nil {
					orgLog.Error().
						Err(result.Error).
						Str("action", result.Action).
						Msg("Hosted reaper failed to delete expired organization")
				} else {
					orgLog.Info().
						Str("action", result.Action).
						Msg("Hosted reaper deleted expired organization")
				}
			}
		} else {
			result.Action = "dry_run"
			orgLog.Info().
				Str("action", result.Action).
				Msg("Hosted reaper dry-run would delete expired organization")
		}

		results = append(results, result)
	}

	return results
}

func normalizeReaperScanInterval(scanInterval time.Duration) time.Duration {
	if scanInterval <= 0 {
		return defaultReaperScanInterval
	}
	return scanInterval
}

func clampRetentionDays(retentionDays int) (int, bool) {
	if retentionDays > maxRetentionDays {
		return maxRetentionDays, true
	}
	if retentionDays < minRetentionDays {
		return minRetentionDays, true
	}
	return retentionDays, false
}
