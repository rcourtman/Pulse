package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type testSupplementalUsageProvider struct {
	source  unifiedresources.DataSource
	settled bool
	readyAt time.Time
	records []unifiedresources.IngestRecord
}

func newTestSupplementalUsageProvider(source unifiedresources.DataSource) *testSupplementalUsageProvider {
	return &testSupplementalUsageProvider{source: source}
}

func (p *testSupplementalUsageProvider) SupplementalRecords(*monitoring.Monitor, string) []unifiedresources.IngestRecord {
	out := make([]unifiedresources.IngestRecord, len(p.records))
	copy(out, p.records)
	return out
}

func (p *testSupplementalUsageProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	if p == nil || p.source == "" {
		return nil
	}
	return []unifiedresources.DataSource{p.source}
}

func (p *testSupplementalUsageProvider) SupplementalInventoryReadyAt(*monitoring.Monitor, string) (time.Time, bool) {
	if p == nil {
		return time.Time{}, false
	}
	return p.readyAt, p.settled
}

func (p *testSupplementalUsageProvider) settleWithRecords(records []unifiedresources.IngestRecord) time.Time {
	now := time.Now().UTC()
	return p.settleAtWithRecords(now, records)
}

func (p *testSupplementalUsageProvider) settleAtWithRecords(at time.Time, records []unifiedresources.IngestRecord) time.Time {
	p.readyAt = at
	p.settled = true
	p.records = make([]unifiedresources.IngestRecord, len(records))
	copy(p.records, records)
	return at
}

func bindTestSupplementalUsageProvider(
	monitor *monitoring.Monitor,
	source unifiedresources.DataSource,
	provider *testSupplementalUsageProvider,
) {
	if monitor == nil || provider == nil {
		return
	}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(nil))
	monitor.SetSupplementalRecordsProvider(source, provider)
}
