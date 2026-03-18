package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

const (
	alertRollupsPageLimit = 500
	alertRollupsMaxPages  = 50
)

func (m *Monitor) listRecoveryRollupsForAlerts(ctx context.Context, kind recovery.Kind) ([]recovery.ProtectionRollup, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if mock.IsMockEnabled() {
		points := mock.GetMockRecoveryPoints()
		filtered := make([]recovery.RecoveryPoint, 0, len(points))
		for _, p := range points {
			if kind != "" && p.Kind != kind {
				continue
			}
			filtered = append(filtered, p)
		}
		return recovery.BuildRollupsFromPoints(filtered), nil
	}

	if m == nil || m.recoveryManager == nil {
		return nil, nil
	}
	orgID := m.GetOrgID()
	if orgID == "" {
		orgID = "default"
	}

	store, err := m.recoveryManager.StoreForOrg(orgID)
	if err != nil {
		return nil, err
	}

	opts := recovery.ListPointsOptions{
		Kind:  kind,
		Page:  1,
		Limit: alertRollupsPageLimit,
	}

	out := make([]recovery.ProtectionRollup, 0, 256)
	page := 1
	for page <= alertRollupsMaxPages {
		opts.Page = page
		rollups, total, err := store.ListRollups(ctx, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, rollups...)

		if len(rollups) < opts.Limit {
			break
		}
		totalPages := 1
		if opts.Limit > 0 {
			totalPages = (total + opts.Limit - 1) / opts.Limit
		}
		if page >= totalPages {
			break
		}
		if page == alertRollupsMaxPages && page < totalPages {
			return out, fmt.Errorf("recovery rollups pagination exceeded max pages (%d)", alertRollupsMaxPages)
		}
		page++
	}

	return out, nil
}

func (m *Monitor) listBackupRollupsForAlerts(ctx context.Context) ([]recovery.ProtectionRollup, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return m.listRecoveryRollupsForAlerts(ctx, recovery.KindBackup)
}
