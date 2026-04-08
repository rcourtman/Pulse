package monitoring

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// MonitoredSystemUsageSnapshot describes whether the monitor can currently
// supply a canonical monitored-system count that is safe for billing and
// admission enforcement to consume.
type MonitoredSystemUsageSnapshot struct {
	Count     int
	ReadState unifiedresources.ReadState
	Available bool
}

// MonitoredSystemUsage returns the canonical monitored-system count only when
// the current unified view is settled enough for billing boundaries. When
// supplemental provider-owned sources are still settling, the result fails
// closed with Available=false.
func (m *Monitor) MonitoredSystemUsage() MonitoredSystemUsageSnapshot {
	if m == nil {
		return MonitoredSystemUsageSnapshot{}
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return MonitoredSystemUsageSnapshot{}
	}

	orgID := normalizedMonitorUsageOrgID(m)
	readyAt, settled := m.supplementalInventoryReadyAt(orgID)
	if !settled {
		return MonitoredSystemUsageSnapshot{ReadState: readState}
	}
	if !readyAt.IsZero() {
		freshness := m.currentUnifiedResourceFreshness()
		if freshness.IsZero() || freshness.Before(readyAt) {
			return MonitoredSystemUsageSnapshot{ReadState: readState}
		}
	}

	return MonitoredSystemUsageSnapshot{
		Count:     unifiedresources.MonitoredSystemCount(readState),
		ReadState: readState,
		Available: true,
	}
}

func normalizedMonitorUsageOrgID(m *Monitor) string {
	if m == nil {
		return "default"
	}
	orgID := strings.TrimSpace(m.GetOrgID())
	if orgID == "" {
		return "default"
	}
	return orgID
}

func (m *Monitor) supplementalInventoryReadyAt(orgID string) (time.Time, bool) {
	providers := m.supplementalProviderSnapshot()
	if len(providers) == 0 {
		return time.Time{}, true
	}

	var watermark time.Time
	for _, provider := range providers {
		if provider == nil {
			continue
		}

		readinessProvider, hasReadiness := provider.(MonitorSupplementalInventoryReadinessProvider)
		if !hasReadiness {
			if len(supplementalProviderOwnedSourcesForOrg(provider, orgID)) > 0 {
				return time.Time{}, false
			}
			continue
		}

		readyAt, settled := readinessProvider.SupplementalInventoryReadyAt(m, orgID)
		if !settled {
			return time.Time{}, false
		}
		if readyAt.After(watermark) {
			watermark = readyAt
		}
	}

	return watermark, true
}

func supplementalProviderOwnedSourcesForOrg(
	provider MonitorSupplementalRecordsProvider,
	orgID string,
) []unifiedresources.DataSource {
	if provider == nil {
		return nil
	}

	var sources []unifiedresources.DataSource
	if tenantOwner, ok := provider.(interface {
		SnapshotOwnedSourcesForOrg(string) []unifiedresources.DataSource
	}); ok {
		sources = tenantOwner.SnapshotOwnedSourcesForOrg(orgID)
	} else if owner, ok := provider.(interface {
		SnapshotOwnedSources() []unifiedresources.DataSource
	}); ok {
		sources = owner.SnapshotOwnedSources()
	}

	if len(sources) == 0 {
		return nil
	}

	out := make([]unifiedresources.DataSource, 0, len(sources))
	for _, source := range sources {
		normalized := unifiedresources.DataSource(strings.ToLower(strings.TrimSpace(string(source))))
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}
