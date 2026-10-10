package ai

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func patrolMemoryFixture(percent float64, state, source string, at time.Time) models.Memory {
	const total = 20 * 1024 * 1024 * 1024
	used := int64(float64(total) * percent / 100)
	return models.Memory{Total: total, Used: used, Free: total - used, Usage: percent,
		Observation: models.MemoryObservation{State: state, Source: source, ObservedAt: at}}
}

// These connected controls use only existing APIs so the same tests can be
// applied to the exact parent. No guest command, provider or workload is used.
func TestPatrolGuestMemoryEvidence(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	at := now.Add(-2 * time.Minute)
	cases := []struct {
		name, state, source, label string
		percent                    float64
		at                         time.Time
		unavailable, pressure      bool
	}{
		{"healthy-cache-aware", "current", "guest-agent-meminfo", "cache-aware guest usage", 24, at, false, true},
		{"real-pressure", "current", "available-field", "cache-aware guest usage", 96, at, false, true},
		{"measured-zero", "current", "agent", "cache-aware guest usage", 0, at, false, true},
		{"zero-availability", "current", "guest-agent-meminfo", "cache-aware guest usage", 100, at, false, true},
		{"hypervisor", "current", "status-mem", "guest pressure unknown", 96, at, false, false},
		{"free-not-available", "current", "status-freemem", "guest pressure unknown", 96, at, false, false},
		{"cluster-cache", "current", "cluster-resources", "guest pressure unknown", 96, at, false, false},
		{"total-minus-used", "current", "derived-total-minus-used", "guest pressure unknown", 96, at, false, false},
		{"last-known-high", "last-known", "guest-agent-meminfo", "last-known", 96, at, false, false},
		{"last-known-low", "last-known", "agent", "last-known", 24, at, false, false},
		{"missing-time", "current", "available-field", "time unknown", 96, time.Time{}, false, false},
		{"future-time", "current", "agent", "time unknown", 96, now.Add(time.Hour), false, false},
		{"legacy", "", "", "guest pressure unknown", 96, time.Time{}, false, false},
		{"future-source", "current", "future-source", "source unknown", 96, at, false, false},
		{"unavailable", "unavailable", "unavailable", "N/A", 0, time.Time{}, true, false},
	}
	for _, tc := range cases {
		for _, kind := range []string{"vm", "system-container"} {
			for _, path := range []string{"legacy", "canonical"} {
				t.Run(tc.name+"/"+kind+"/"+path, func(t *testing.T) {
					mem := patrolMemoryFixture(tc.percent, tc.state, tc.source, tc.at)
					mem.UsageUnavailable = tc.unavailable
					snapshot := models.StateSnapshot{}
					if kind == "vm" {
						snapshot.VMs = []models.VM{{ID: "guest", Name: "guest", VMID: 100, Type: "qemu", Instance: "pve", Node: "node", Status: "running", LastSeen: now, LastBackup: now, Memory: mem}}
					} else {
						snapshot.Containers = []models.Container{{ID: "guest", Name: "guest", VMID: 100, Type: "lxc", Instance: "pve", Node: "node", Status: "running", LastSeen: now, LastBackup: now, Memory: mem}}
					}
					snap := newPatrolRuntimeState(snapshot)
					if path == "canonical" {
						registry := unifiedresources.NewRegistry(nil)
						registry.IngestSnapshot(snapshot)
						snap = patrolRuntimeState{readState: registry}
					}
					ps := NewPatrolService(nil, nil)
					cfg := PatrolConfig{AnalyzeGuests: true}
					full := ps.seedResourceInventoryState(snap, nil, cfg, now, false, nil)
					if !strings.Contains(full, tc.label) {
						t.Errorf("memory qualification %q missing from actual inventory:\n%s", tc.label, full)
					}
					if tc.unavailable && strings.Contains(full, "| 0% | 0% |") {
						t.Error("unavailable memory was shown as measured zero")
					}
					if !tc.unavailable && !tc.at.IsZero() && !tc.at.After(now) && !strings.Contains(full, tc.at.Format(time.RFC3339)) {
						t.Error("original memory observation time was lost or renewed")
					}
					flags := triageThresholdChecksState(snap, nil, DefaultPatrolThresholds())
					memoryFlag := triageFindFlag(flags, func(f TriageFlag) bool { return f.Metric == "memory" })
					if (memoryFlag != nil) != (tc.pressure && tc.percent > DefaultPatrolThresholds().GuestMemWatch) {
						t.Errorf("pressure flag mismatch: known=%t percent=%v flag=%+v", tc.pressure, tc.percent, memoryFlag)
					}
					metrics, found := patrolLookupResourceMetricsForType(snap, "guest", kind)
					if !found {
						t.Fatal("guest identity disappeared from metric verification")
					}
					_, hasMemory := metrics["memory"]
					if hasMemory != tc.pressure {
						t.Errorf("unqualified memory entered verification: %+v", metrics)
					}
					ok, err := verifyMetricRecoveredState(snap, DefaultPatrolThresholds(), "memory-high", "guest", kind)
					if !tc.pressure && (ok || !errors.Is(err, aicontracts.ErrVerificationUnknown)) {
						t.Errorf("unknown memory decided recovery: ok=%t err=%v", ok, err)
					}
					if tc.pressure && (err != nil || ok != (tc.percent < DefaultPatrolThresholds().GuestMemWarning*0.95)) {
						t.Errorf("measured recovery control failed: ok=%t err=%v", ok, err)
					}
					if !tc.pressure {
						compact := ps.seedResourceInventorySummaryState(snap, nil, cfg, now, nil)
						quiet := ps.seedResourceInventoryState(snap, nil, cfg, now, true, nil)
						for _, output := range []string{compact, quiet} {
							if !strings.Contains(output, "Guest memory pressure unknown") || strings.Contains(output, "guest (Mem 96%)") {
								t.Errorf("condensed seed lost uncertainty or declared pressure:\n%s", output)
							}
						}
						ps.SetConfig(cfg)
						triage := ps.runDeterministicTriageState(context.Background(), snap, nil, nil)
						seed, _ := ps.buildTriageSeedContextState(triage, snap, nil, nil)
						if !strings.Contains(seed, "Guest memory pressure unknown") || strings.Contains(seed, "## Healthy Resources") {
							t.Errorf("actual triage seed lost unflagged memory uncertainty:\n%s", seed)
						}
					}
				})
			}
		}
	}
}

func TestPatrolGuestMemoryForecastsAndAnomalies(t *testing.T) {
	now := time.Now().UTC()
	for _, source := range []string{"status-mem", "guest-agent-meminfo"} {
		t.Run(source, func(t *testing.T) {
			ps := NewPatrolService(nil, nil)
			bs := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
			var baselinePoints []baseline.MetricPoint
			var points []models.MetricPoint
			for i := 0; i < 10; i++ {
				at := now.Add(-time.Duration(10-i) * time.Hour)
				baselinePoints = append(baselinePoints, baseline.MetricPoint{Value: 24, Timestamp: at})
				points = append(points, models.MetricPoint{Value: 96, Timestamp: at})
			}
			if err := bs.Learn("guest", "vm", "memory", baselinePoints); err != nil {
				t.Fatal(err)
			}
			ps.SetBaselineStore(bs)
			ps.SetMetricsHistoryProvider(&precomputeMetricsHistoryProvider{metrics: map[string][]models.MetricPoint{"guest:memory": points, "guest:disk": points}})
			snap := newPatrolRuntimeState(models.StateSnapshot{VMs: []models.VM{{ID: "guest", VMID: 100, Type: "qemu", Status: "running", Memory: patrolMemoryFixture(96, "current", source, now.Add(-time.Minute)), Disk: models.Disk{Usage: 96}}}})
			intel := ps.seedPrecomputeIntelligenceState(snap, nil, now)
			memoryForecast, diskForecast, memoryAnomaly := false, false, false
			for _, f := range intel.forecasts {
				memoryForecast = memoryForecast || f.metric == "memory"
				diskForecast = diskForecast || f.metric == "disk"
			}
			for _, a := range intel.anomalies {
				memoryAnomaly = memoryAnomaly || a.Metric == "memory"
			}
			known := source == "guest-agent-meminfo"
			if memoryForecast != known || memoryAnomaly != known || !diskForecast {
				t.Errorf("memory knowledge did not gate derived signals independently of disk: forecast=%t anomaly=%t disk=%t known=%t", memoryForecast, memoryAnomaly, diskForecast, known)
			}
		})
	}
}

func TestPatrolGuestMemoryAlertRecovery(t *testing.T) {
	now := time.Now().UTC()
	for _, state := range []string{"unavailable", "last-known", "current"} {
		t.Run(state, func(t *testing.T) {
			ps := NewPatrolService(nil, nil)
			resolver := &stubAlertResolver{alerts: []AlertInfo{{ID: "a", Type: "memory", ResourceID: "guest", ResourceType: "vm", Threshold: 80, StartTime: now.Add(-time.Hour)}}}
			ps.alertResolver = resolver
			provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: memory is now low"}
			ps.aiService = &Service{provider: provider, cfg: &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}}
			mem := patrolMemoryFixture(0, state, "agent", now.Add(-time.Minute))
			mem.UsageUnavailable = state == "unavailable"
			snap := newPatrolRuntimeState(models.StateSnapshot{VMs: []models.VM{{ID: "guest", Name: "guest", VMID: 100, Type: "qemu", Status: "running", Memory: mem}}})
			resolved := ps.reviewAndResolveAlertsState(context.Background(), snap, true, "")
			if provider.calls != 1 || (resolved == 1) != (state == "current") || len(resolver.clears) != resolved {
				t.Errorf("unknown memory falsely resolved, or measured zero could not recover: calls=%d resolved=%d clears=%v", provider.calls, resolved, resolver.clears)
			}
			prompt := ps.buildAlertReviewPromptState(resolver.alerts, snap)
			if state != "current" && !strings.Contains(prompt, map[string]string{"unavailable": "N/A", "last-known": "guest pressure unknown"}[state]) {
				t.Errorf("alert review prompt claims current guest memory: %s", prompt)
			}
		})
	}
}

func TestPatrolGuestMemorySelectedMetricAndOtherPlatforms(t *testing.T) {
	now := time.Now().UTC().Add(-time.Minute)
	r := &unifiedresources.Resource{ID: "guest", Name: "guest", Type: unifiedresources.ResourceTypeVM, Status: unifiedresources.StatusOnline,
		Proxmox: &unifiedresources.ProxmoxData{VMID: 100, RuntimeStatus: "running", Memory: func() *models.Memory {
			m := patrolMemoryFixture(96, "last-known", "status-mem", now.Add(-time.Hour))
			return &m
		}()},
		Metrics: &unifiedresources.ResourceMetrics{Memory: &unifiedresources.MetricValue{Percent: 24, Source: unifiedresources.SourceAgent, Observation: models.MemoryObservation{State: "current", Source: "agent", ObservedAt: now}}}}
	vm := unifiedresources.NewVMView(r)
	snap := patrolRuntimeState{readState: &mockReadState{vms: []*unifiedresources.VMView{&vm}}}
	ps := NewPatrolService(nil, nil)
	output := ps.seedResourceInventoryState(snap, nil, PatrolConfig{AnalyzeGuests: true}, time.Now(), false, nil)
	if !strings.Contains(output, "24% (current; source agent") || strings.Contains(output, "status-mem") || strings.Contains(output, "96%") {
		t.Errorf("raw platform memory replaced selected agent evidence: %s", output)
	}
	// A non-Proxmox VM keeps its existing selected reading without being
	// required to invent the Proxmox observation metadata.
	r.Proxmox = nil
	r.Metrics.Memory = &unifiedresources.MetricValue{Percent: 96, Source: unifiedresources.SourceVMware}
	flags := triageThresholdChecksState(snap, nil, DefaultPatrolThresholds())
	if triageFindFlag(flags, func(f TriageFlag) bool { return f.Metric == "memory" }) == nil {
		t.Error("unrelated platform's real pressure was silenced")
	}
}

func TestIssue2762NamedMemoryFields(t *testing.T) {
	// A synthetic 20 GiB guest with 15.2 GiB available and little truly free
	// memory is not at 96% pressure. Field order and reclaimable cache do not
	// override MemAvailable, and missing/truncated evidence stays unknown.
	fields := []string{"MemTotal: 20971520 kB", "MemFree: 838860 kB", "MemAvailable: 15938355 kB", "Buffers: 1048576 kB", "Cached: 14942208 kB"}
	for _, reverse := range []bool{false, true} {
		if reverse {
			for i, j := 0, len(fields)-1; i < j; i, j = i+1, j-1 {
				fields[i], fields[j] = fields[j], fields[i]
			}
		}
		got, err := proxmox.ParseLinuxMemoryAvailability(strings.Join(fields, "\n"), false)
		if err != nil {
			t.Fatal(err)
		}
		pressure := float64(got.Total-got.EffectiveAvailable) / float64(got.Total) * 100
		if math.Abs(pressure-24) > 0.01 || got.Source != "meminfo-available" {
			t.Errorf("named available field was mistaken for cache or free: pressure=%v evidence=%+v", pressure, got)
		}
	}
	if _, err := proxmox.ParseLinuxMemoryAvailability("MemTotal: 20971520 kB\nMemFree: 838860 kB", true); err == nil {
		t.Error("partial free-only payload manufactured available memory")
	}
}
