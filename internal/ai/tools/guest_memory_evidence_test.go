package tools

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These actual tool controls use APIs present in the parent. The parent gets
// the tests, not the repair, for a discriminating comparison.
func TestGuestMemoryQueryEvidence(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	for _, kind := range []string{"vm", "system-container"} {
		for _, tc := range []struct {
			name, state, source string
			percent             float64
			at                  time.Time
			missing, known      bool
		}{
			{"measured-zero", "current", "agent", 0, at, false, true},
			{"healthy-available", "current", "guest-agent-meminfo", 24, at, false, true},
			{"real-pressure", "current", "available-field", 96, at, false, true},
			{"status-cache", "current", "status-mem", 96, at, false, false},
			{"retained-low", "last-known", "agent", 24, at.Add(-time.Hour), false, false},
			{"retained-high", "last-known", "guest-agent-meminfo", 96, at.Add(-time.Hour), false, false},
			{"undated", "current", "available-field", 24, time.Time{}, false, false},
			{"future", "current", "agent", 24, time.Now().Add(time.Hour), false, false},
			{"unknown-source", "current", "unrecognised-source", 24, at, false, false},
			{"unavailable", "unavailable", "unavailable", 0, time.Time{}, false, false},
			{"missing", "", "", 0, time.Time{}, true, false},
			{"invalid", "current", "agent", math.NaN(), at, false, false},
		} {
			for _, path := range []string{"canonical", "typed-view"} {
				t.Run(kind+"/"+tc.name+"/"+path, func(t *testing.T) {
					total, used := int64(20<<30), int64(5<<30)
					r := unifiedresources.Resource{ID: "selected-guest", Name: "selected-guest", Type: unifiedresources.ResourceType(kind), Status: unifiedresources.StatusOnline, LastSeen: time.Now(),
						Proxmox: &unifiedresources.ProxmoxData{VMID: 100, NodeName: "node", Instance: "fixture", SourceID: "source-guest", RuntimeStatus: "running",
							Memory: &models.Memory{Usage: 99, Observation: models.MemoryObservation{State: "current", Source: "available-field", ObservedAt: time.Now()}}},
						Metrics: &unifiedresources.ResourceMetrics{Memory: &unifiedresources.MetricValue{Percent: tc.percent, Used: &used, Total: &total, Observation: models.MemoryObservation{State: tc.state, Source: tc.source, ObservedAt: tc.at}}},
					}
					if tc.missing {
						r.Metrics.Memory = nil
					}
					vm, ct := unifiedresources.NewVMView(&r), unifiedresources.NewContainerView(&r)
					rs := &fakeReadState{}
					if kind == "vm" {
						rs.vms = []*unifiedresources.VMView{&vm}
					} else {
						rs.containers = []*unifiedresources.ContainerView{&ct}
					}
					cfg := ExecutorConfig{ReadState: rs}
					if path == "canonical" {
						cfg.UnifiedResourceProvider = &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{r}}
					}
					exec := NewPulseToolExecutor(cfg)
					for _, action := range []string{"get", "list", "topology"} {
						result, err := exec.ExecuteTool(context.Background(), agentcapabilities.PulseQueryToolName, map[string]interface{}{"action": action, "resource_type": kind, "resource_id": r.ID})
						if err != nil || result.IsError || len(result.Content) == 0 {
							t.Fatalf("%s read failed: err=%v result=%+v", action, err, result)
						}
						var decoded map[string]interface{}
						if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
							t.Fatal(err)
						}
						var evidence interface{}
						var percent interface{}
						if action == "get" {
							memory := decoded["memory"].(map[string]interface{})
							evidence, percent = memory["evidence"], memory["percent"]
							if (tc.missing || tc.state == "unavailable" || math.IsNaN(tc.percent)) && memory["used_gb"] != nil {
								t.Error("missing usage became zero used bytes")
							}
						} else {
							rows := decoded
							if action == "topology" {
								rows = decoded["proxmox"].(map[string]interface{})["nodes"].([]interface{})[0].(map[string]interface{})
							}
							family := "vms"
							if kind != "vm" {
								family = "containers"
							}
							row := rows[family].([]interface{})[0].(map[string]interface{})
							evidence, percent = row["memory_evidence"], row["memory_percent"]
						}
						e, ok := evidence.(map[string]interface{})
						if !ok {
							t.Errorf("%s lost guest memory qualification: %s", action, result.Content[0].Text)
							continue
						}
						available := !tc.missing && tc.state != "unavailable" && !math.IsNaN(tc.percent)
						if e["available"] != available || e["pressure_known"] != tc.known {
							t.Errorf("%s qualification=%+v want available=%t known=%t", action, e, available, tc.known)
						}
						if available && percent != tc.percent || !available && percent != nil {
							t.Errorf("%s percent=%v want available=%t percent=%g", action, percent, available, tc.percent)
						}
						if !tc.at.IsZero() && !tc.at.After(time.Now()) && available && e["observed_at"] != tc.at.Format(time.RFC3339) {
							t.Errorf("%s renewed/lost original time: %+v", action, e)
						}
						if tc.source == "status-mem" && e["may_include_reclaimable_cache"] != true {
							t.Errorf("%s erased cache distinction", action)
						}
					}
				})
			}
		}
	}
}

func TestGuestMemoryHistoryDoesNotBorrowLiveOrigin(t *testing.T) {
	for _, family := range []string{"vm", "system-container", "agent"} {
		t.Run(family, func(t *testing.T) {
			history := &mockMetricsHistoryProvider{}
			history.On("GetResourceMetrics", "source-guest", 24*time.Hour).Return([]MetricPoint{{Memory: 96}}, nil)
			history.On("GetAllMetricsSummary", 24*time.Hour).Return(map[string]ResourceMetricsSummary{"selected-guest": {ResourceType: family, AvgMemory: 96, MaxMemory: 99}}, nil)
			r := unifiedresources.Resource{ID: "selected-guest", Type: unifiedresources.ResourceType(family), MetricsTarget: &unifiedresources.MetricsTarget{ResourceType: family, ResourceID: "source-guest"},
				Metrics: &unifiedresources.ResourceMetrics{Memory: &unifiedresources.MetricValue{Percent: 24, Observation: models.MemoryObservation{State: "current", Source: "agent", ObservedAt: time.Now().Add(-time.Minute)}}}}
			exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}, MetricsHistory: history, UnifiedResourceProvider: &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{r}}})
			for _, id := range []string{r.ID, ""} {
				result, err := exec.ExecuteTool(context.Background(), agentcapabilities.PulseMetricsToolName, map[string]interface{}{"type": "performance", "resource_id": id})
				if err != nil || result.IsError {
					t.Fatalf("history read failed: %v %+v", err, result)
				}
				var decoded map[string]interface{}
				if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
					t.Fatal(err)
				}
				note, _ := decoded["memory_interpretation"].(string)
				if family != "agent" && (!strings.Contains(note, "per-sample") || !strings.Contains(note, "cannot reattribute")) {
					t.Errorf("history borrowed live memory trust: %s", result.Content[0].Text)
				}
				if family == "agent" && note != "" {
					t.Error("non-guest history changed")
				}
			}
			history.AssertExpectations(t)
		})
	}
}
