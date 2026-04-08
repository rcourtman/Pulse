package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

func assertMonitoredSystemUsageUnavailableReason(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	want string,
) APIError {
	t.Helper()

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}

	var apiErr APIError
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode API error: %v", err)
	}
	apiErr = apiErr.NormalizeCollections()

	if apiErr.Code != "monitored_system_usage_unavailable" {
		t.Fatalf("API error code = %q, want monitored_system_usage_unavailable", apiErr.Code)
	}
	if got := apiErr.Details["reason"]; got != want {
		t.Fatalf("API error details.reason = %q, want %q", got, want)
	}

	return apiErr
}
