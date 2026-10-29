package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// metricHistoryFixture serves one running VM through both the registry an
// agent addresses (canonical id vm:101) and the monitor the history read uses
// (metrics-store id pve1:node1:101). The VM's only history is its live point.
func metricHistoryFixture(t *testing.T, resources []unified.Resource) *Router {
	t.Helper()
	state := models.NewState()
	state.UpdateVMsForInstance("pve1", []models.VM{{
		ID: "pve1:node1:101", VMID: 101, Name: "db-01", Node: "node1", Instance: "pve1",
		Status: "running", Type: "qemu", CPU: 0.42,
		Memory: models.Memory{Usage: 55.0}, Disk: models.Disk{Usage: 33.0},
	}})
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init persistence: %v", err)
	}
	h, _, _ := fleetFixtureHandlers(t, resources)
	return &Router{monitor: monitor, licenseHandlers: NewLicenseHandlers(mtp, false), resourceHandlers: h.resources}
}

var metricHistoryVM = unified.Resource{
	ID: "vm:101", Type: unified.ResourceTypeVM, Name: "db-01",
	MetricsTarget: &unified.MetricsTarget{ResourceType: "vm", ResourceID: "pve1:node1:101"},
}

func getAgentMetricHistory(router *Router, values url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, agentcapabilities.MetricHistoryCapabilityPath+"?"+values.Encode(), nil)
	rec := httptest.NewRecorder()
	router.handleAgentMetricHistory(rec, req)
	return rec
}

func TestAgentMetricHistory_ResolvesCanonicalIDToTheChartSeries(t *testing.T) {
	router := metricHistoryFixture(t, []unified.Resource{metricHistoryVM})
	rec := getAgentMetricHistory(router, url.Values{"resourceId": {"vm:101"}, "metric": {"cpu"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ResourceId != "pve1:node1:101" || resp.Source != "live" || len(resp.Points) != 1 || math.Abs(resp.Points[0].Value-42.0) > 0.001 {
		t.Fatalf("resp = %+v; want the VM's live cpu point from its metrics target", resp)
	}
}

func TestAgentMetricHistory_PassesTheLicensedHistoryLimitThrough(t *testing.T) {
	router := metricHistoryFixture(t, []unified.Resource{metricHistoryVM})
	rec := getAgentMetricHistory(router, url.Values{"resourceId": {"vm:101"}, "metric": {"cpu"}, "range": {"30d"}})
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), "license_required") {
		t.Fatalf("30d on Community: status = %d, body=%s; want the chart read's license_required", rec.Code, rec.Body.String())
	}
}

// The registry names Kubernetes targets by kind (pod, k8s-node, ...), while
// the history store keeps them under one k8s type. A pod target must reach the
// history read rather than fail as an unsupported type.
func TestAgentMetricHistory_KubernetesTargetsReachTheHistoryRead(t *testing.T) {
	pod := unified.Resource{
		ID: "pod:1", Type: unified.ResourceTypePod, Name: "api-0",
		MetricsTarget: &unified.MetricsTarget{ResourceType: string(unified.ResourceTypePod), ResourceID: "k8s:prod:pod:default/api-0"},
	}
	// The fixture monitor has no persistent store and Kubernetes has no
	// in-memory fallback, so the read itself answers 503; before the store
	// type accepted target kinds it was a 400 for an unsupported type.
	router := metricHistoryFixture(t, []unified.Resource{pod})
	rec := getAgentMetricHistory(router, url.Values{"resourceId": {"pod:1"}, "metric": {"cpu"}})
	if rec.Code == http.StatusBadRequest || strings.Contains(rec.Body.String(), "unsupported resourceType") {
		t.Fatalf("pod target rejected before the history read: status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentMetricHistory_RejectsWhatItCannotServe(t *testing.T) {
	noHistory := unified.Resource{ID: "app:1", Type: unified.ResourceTypeAppContainer, Name: "no-history"}
	router := metricHistoryFixture(t, []unified.Resource{metricHistoryVM, noHistory})
	for _, tc := range []struct {
		values url.Values
		status int
	}{
		{url.Values{}, http.StatusBadRequest},
		{url.Values{"resourceId": {"vm:101"}, "range": {"2y"}}, http.StatusBadRequest},
		{url.Values{"resourceId": {"vm:101"}, "maxPoints": {"0"}}, http.StatusBadRequest},
		{url.Values{"resourceId": {"vm:101"}, "maxPoints": {"ten"}}, http.StatusBadRequest},
		{url.Values{"resourceId": {"vm:999"}}, http.StatusNotFound},
		{url.Values{"resourceId": {"app:1"}}, http.StatusNotFound},
	} {
		if rec := getAgentMetricHistory(router, tc.values); rec.Code != tc.status {
			t.Errorf("%s: status = %d, body=%s; want %d", tc.values.Encode(), rec.Code, rec.Body.String(), tc.status)
		}
	}
}

func TestAgentMetricHistoryQuery_DefaultsAndCaps(t *testing.T) {
	query, problem := parseAgentMetricHistoryQuery(url.Values{"resourceId": {" vm:101 "}})
	if problem != "" || query.resourceID != "vm:101" || query.timeRange != agentcapabilities.MetricHistoryDefaultRange || query.maxPoints != agentcapabilities.MetricHistoryDefaultMaxPoints {
		t.Fatalf("defaults: %+v %q", query, problem)
	}
	query, problem = parseAgentMetricHistoryQuery(url.Values{"resourceId": {"vm:101"}, "range": {"7D"}, "maxPoints": {"100000"}})
	if problem != "" || query.timeRange != "7d" || query.maxPoints != agentcapabilities.MetricHistoryMaxPoints {
		t.Fatalf("caps: %+v %q", query, problem)
	}
}
