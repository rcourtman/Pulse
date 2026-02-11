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

type ReapResult struct {
	OrgID         string
	RequestedAt   time.Time
	RetentionDays int
	ExpiredAt     time.Time
	Action        string
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
	if r == nil || r.lister == nil || r.now == nil {
		return nil
	}

	orgs, err := r.lister.ListOrganizations()
	if err != nil {
		log.Error().Err(err).Msg("Hosted reaper failed to list organizations")
		return nil
	}

	now := r.now()
	results := make([]ReapResult, 0)

	for _, org := range orgs {
		if org == nil {
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
		log.Info().
			Str("org_id", org.ID).
			Time("deletion_requested_at", *org.DeletionRequestedAt).
			Int("retention_days", retentionDays).
			Dur("expired_for", expiredFor).
			Msg("Hosted reaper found expired pending-deletion organization")

		result := ReapResult{
			OrgID:         org.ID,
			RequestedAt:   *org.DeletionRequestedAt,
			RetentionDays: retentionDays,
			ExpiredAt:     expiry,
		}

		if r.liveMode {
			result.Action = "deleted"

			if r.deleter == nil {
				result.Error = errors.New("org deleter is nil")
				log.Error().
					Str("org_id", org.ID).
					Msg("Hosted reaper is in live mode but deleter is nil")
			} else {
				if r.OnBeforeDelete != nil {
					if err := r.OnBeforeDelete(org.ID); err != nil {
						result.Error = err
						log.Error().
							Err(err).
							Str("org_id", org.ID).
							Msg("Hosted reaper OnBeforeDelete hook failed")
						results = append(results, result)
						continue
					}
				}
				result.Error = r.deleter.DeleteOrganization(org.ID)
				if result.Error != nil {
					log.Error().
						Err(result.Error).
						Str("org_id", org.ID).
						Msg("Hosted reaper failed to delete expired organization")
				} else {
					log.Info().
						Str("org_id", org.ID).
						Msg("Hosted reaper deleted expired organization")
				}
			}
		} else {
			result.Action = "dry_run"
			log.Info().Str("org_id", org.ID).Msg("DRY RUN: would delete org")
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
