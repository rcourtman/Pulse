package alerts

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

func continuityManager(t *testing.T, explicit bool) (*Manager, *atomic.Int64) {
	t.Helper()
	m := newEventLogManager(t)
	cfg := characterizationBaseConfig()
	cfg.MetricTimeThresholds = map[string]map[string]int{"all": {"cpu": 60, "memory": 60, "disk": 60, "temperature": 60, "diskTemperature": 60, "diskRead": 60, "diskWrite": 60, "networkIn": 60, "networkOut": 60, "usage": 60}}
	cfg.MetricEvaluationWindows = nil
	cfg.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 99, Clear: 98}
	cfg.GuestDefaults.Memory = &HysteresisThreshold{Trigger: 80, Clear: 70}
	cfg.GuestDefaults.Disk = &HysteresisThreshold{Trigger: 80, Clear: 70}
	cfg.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 80, Clear: 70}
	cfg.DiskTempByType = nil
	cfg.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.UpdateConfig(cfg)
	elapsed := &atomic.Int64{}
	origin := time.Now().UTC()
	m.now = func() time.Time { return origin.Add(time.Duration(elapsed.Load())) }
	m.intentClock = func() time.Duration { return time.Duration(elapsed.Load()) }
	if explicit {
		doc := NewAlertIntentPolicyDocument()
		for _, metric := range []string{"cpu", "memory", "disk", "temperature", "diskTemperature", "diskWrite", "networkIn", "networkOut", "diskRead", "usage"} {
			doc.Defaults[MetricAlertIntentSignal(metric)] = AlertIntentRule{GraceSeconds: intPointer(60)}
		}
		if err := m.LoadIntentPolicies(doc); err != nil {
			t.Fatal(err)
		}
	}
	m.EnableShadowFeed()
	return m, elapsed
}

func continuityIncident(m *Manager, resourceID, metric string) (reducer.Incident, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.core.Incident(resourceID, canonicalMetricSpecID(resourceID, metric))
}

// Actual production entry points, not only a reducer interruption unit test.
func continuityObserver(t *testing.T, m *Manager, route, gap string) (string, string, func(float64, bool)) {
	t.Helper()
	threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	id, metric := "site:node:105", "memory"
	switch route {
	case "legacy", "canonical":
		spec, err := buildCanonicalMetricSpec(id, "guest", unifiedresources.ResourceTypeVM, metric, threshold)
		if err != nil {
			t.Fatal(err)
		}
		if gap == "history-error" || gap == "history-empty" || gap == "history-gap" || gap == "history-NaN" {
			m.mu.Lock()
			m.config.MetricEvaluationWindows = map[string]map[string]int{"all": {metric: 300}}
			m.mu.Unlock()
		}
		return id, metric, func(value float64, missing bool) {
			if missing {
				switch gap {
				case "NaN":
					value = math.NaN()
				case "+Inf":
					value = math.Inf(1)
				case "-Inf":
					value = math.Inf(-1)
				}
			}
			m.SetMetricWindowProvider(func(r MetricWindowRequest) ([]MetricWindowPoint, error) {
				if missing {
					switch gap {
					case "history-error":
						return nil, errors.New("history unavailable")
					case "history-empty":
						return nil, nil
					case "history-gap":
						return []MetricWindowPoint{{r.Start, value}}, nil
					case "history-NaN":
						return []MetricWindowPoint{{r.Start, value}, {r.Start.Add(2 * time.Minute), math.NaN()}, {r.Start.Add(4 * time.Minute), value}}, nil
					}
				}
				return []MetricWindowPoint{{r.Start, value}, {r.Start.Add(2 * time.Minute), value}, {r.Start.Add(4 * time.Minute), value}}, nil
			})
			if route == "legacy" {
				m.checkMetric(id, "guest", "node", "site", "vm", metric, value, threshold, nil)
			} else {
				m.checkMetricWithCanonicalSpec(spec, "guest", "node", "site", "vm", value, threshold, nil)
			}
		}
	case "unified":
		return id, metric, func(value float64, missing bool) {
			sample := &UnifiedResourceMetric{Percent: value}
			if missing {
				sample = nil
			}
			m.evaluateUnifiedMetrics(&UnifiedResourceInput{ID: id, Type: "vm", Name: "guest", Memory: sample}, ThresholdConfig{Memory: threshold}, nil)
		}
	case "host":
		id = "agent:host-105"
		return id, metric, func(value float64, missing bool) {
			host := models.Host{ID: "host-105", Hostname: "host", Status: "online", Memory: models.Memory{Total: 1000, Used: 850, Usage: value}}
			if missing {
				host.Memory = models.UnavailableMemory(1000)
			}
			m.CheckHost(host)
		}
	case "host-disk-temperature":
		id, metric = hostDiskTemperatureResourceID("temp-host", "/dev/sda"), "diskTemperature"
		return id, metric, func(value float64, missing bool) {
			host := models.Host{ID: "temp-host", Hostname: "host", Status: "online",
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "/dev/sda", Temperature: int(value)}}}}
			if missing {
				switch gap {
				case "empty":
					host.Sensors.SMART = nil
				case "omitted":
					host.Sensors.SMART = []models.HostDiskSMART{{Device: "/dev/sdb", Temperature: 10}}
				case "zero", "legacy":
					host.Sensors.SMART[0].Temperature = 0
				case "negative":
					host.Sensors.SMART[0].Temperature = -1
				case "standby":
					host.Sensors.SMART[0].Standby = true
				case "expired":
					m.HandleHostTelemetryExpired(host)
					return
				default:
					host.Sensors.SMART[0].Collection = &diskinventory.CollectionStatus{Temperature: diskinventory.FieldStatus{State: diskinventory.FieldState(gap)}}
				}
			} else if gap != "legacy" {
				host.Sensors.SMART[0].Collection = &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}
			}
			m.CheckHost(host)
		}
	case "guest-memory", "guest-aggregate", "guest-filesystem", "guest-filesystem-expired":
		if route != "guest-memory" {
			metric = "disk"
		}
		resourceID := id
		if route == "guest-filesystem" || route == "guest-filesystem-expired" {
			resourceID += "-disk-root"
		}
		return resourceID, metric, func(value float64, missing bool) {
			vm := models.VM{ID: id, VMID: 105, Name: "guest", Node: "node", Instance: "site", Type: "qemu", Status: "running",
				Memory: models.Memory{Total: 1000, Used: 100, Usage: 10, Observation: models.MemoryObservation{State: "current"}},
				Disk:   models.Disk{Total: 1000, Usage: 10}, IORateValidity: models.IORateValidity{Explicit: true}}
			if route == "guest-memory" {
				vm.Memory.Usage = value
				if missing {
					vm.Memory.Observation.State = "last-known"
				}
			} else {
				vm.Disk.Usage = value
				if route != "guest-aggregate" {
					vm.Disks = []models.Disk{{Total: 1000, Usage: value, Mountpoint: "/"}}
				}
				if missing {
					vm.DiskStatusReason, vm.GuestAgentStatus = "prev-vm-locked", "deferred"
					if route == "guest-filesystem-expired" {
						vm.Disks = nil
						vm.Disk.Usage = -1
					}
				}
			}
			m.CheckGuest(vm, "site")
		}
	}
	t.Fatalf("unknown observer %s", route)
	return "", "", nil
}

func TestMetricObservationGapRestartsActivation(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, route := range []string{"legacy", "canonical", "unified", "host", "guest-memory", "guest-aggregate", "guest-filesystem", "guest-filesystem-expired", "host-disk-temperature"} {
			gaps := []string{"missing"}
			if route == "legacy" || route == "canonical" {
				gaps = []string{"NaN", "+Inf", "-Inf", "history-error", "history-empty", "history-gap", "history-NaN"}
			}
			if route == "host-disk-temperature" {
				gaps = []string{"empty", "omitted", "zero", "negative", "standby", "unavailable", "unsupported", "missing", "invalid-state", "expired", "legacy"}
			}
			for _, gap := range gaps {
				t.Run(route+"/"+gap+"/explicit="+map[bool]string{false: "false", true: "true"}[explicit], func(t *testing.T) {
					m, elapsed := continuityManager(t, explicit)
					id, metric, observe := continuityObserver(t, m, route, gap)
					observe(85, false)
					incident, exists := continuityIncident(m, id, metric)
					if !exists || incident.State != reducer.StatePending {
						t.Fatalf("expected a real pending run at %s, got %+v %v", id, incident, exists)
					}
					m.checkMetric("unrelated", "other", "node", "site", "vm", "memory", 85, &HysteresisThreshold{Trigger: 80, Clear: 70}, nil)
					elapsed.Store(int64(2 * time.Minute))
					observe(85, true)
					if _, exists := continuityIncident(m, id, metric); exists {
						t.Error("unknown observation retained pending evidence")
					}
					if _, exists := continuityIncident(m, "unrelated", "memory"); !exists {
						t.Error("interruption leaked to another resource")
					}
					if len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeResolved}})) != 0 {
						t.Error("unknown observation emitted a lifecycle transition")
					}
					observe(85, false)
					if len(m.GetActiveAlerts()) != 0 {
						t.Fatal("fresh sample fired using time accrued before the unknown interval")
					}
					elapsed.Store(int64(2*time.Minute + 59*time.Second))
					observe(85, false)
					if len(m.GetActiveAlerts()) != 0 {
						t.Fatal("restarted delay fired early")
					}
					elapsed.Store(int64(3 * time.Minute))
					observe(85, false)
					if len(m.GetActiveAlerts()) != 1 {
						t.Fatal("continuous fresh evidence did not fire exactly once after the full delay")
					}
					if m.ShadowDivergences() != 0 {
						t.Fatal("timing interruption diverged from shadow")
					}
				})
			}
		}
	}
}

func TestMetricObservationGapRestartsRecovery(t *testing.T) {
	for _, route := range []string{"legacy", "canonical", "unified", "host", "guest-memory", "host-disk-temperature"} {
		gaps := []string{"missing"}
		if route == "legacy" || route == "canonical" {
			gaps = []string{"NaN", "+Inf", "-Inf", "history-error", "history-empty", "history-gap", "history-NaN"}
		}
		if route == "host-disk-temperature" {
			gaps = []string{"empty", "omitted", "zero", "negative", "standby", "unavailable", "unsupported", "missing", "invalid-state", "expired", "legacy"}
		}
		for _, gap := range gaps {
			t.Run(route+"/"+gap, func(t *testing.T) {
				m, elapsed := continuityManager(t, false)
				id, metric, observe := continuityObserver(t, m, route, gap)
				observe(95, false) // Critical evidence retains its legacy immediate activation.
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("critical control did not fire immediately")
				}
				if err := m.AcknowledgeAlert(m.GetActiveAlerts()[0].ID, "operator"); err != nil {
					t.Fatal(err)
				}
				elapsed.Store(int64(10 * time.Second))
				observe(10, false)
				incident, _ := continuityIncident(m, id, metric)
				if incident.RecoverySince.IsZero() {
					t.Fatal("known healthy evidence did not start a recovery run")
				}
				before, _ := json.Marshal(m.GetActiveAlerts())
				events := len(queryAlertEvents(t, m, eventlog.Filter{}))
				elapsed.Store(int64(2 * time.Minute))
				observe(10, true)
				incident, _ = continuityIncident(m, id, metric)
				if !incident.RecoverySince.IsZero() {
					t.Error("unknown observation retained recovery evidence")
				}
				after, _ := json.Marshal(m.GetActiveAlerts())
				if !bytes.Equal(before, after) || len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
					t.Fatal("unknown observation changed the acknowledged incident or transition ledger")
				}
				observe(10, false)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("fresh healthy sample resolved across an unknown interval")
				}
				elapsed.Store(int64(2*time.Minute + 59*time.Second))
				observe(10, false)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("restarted recovery completed early")
				}
				elapsed.Store(int64(3 * time.Minute))
				observe(10, false)
				if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
					t.Fatal("continuous healthy evidence did not resolve the original once")
				}
			})
		}
	}
}

func TestUnifiedMetricObservationGapCoversRejectedAndAbsentValues(t *testing.T) {
	cases := []struct{ resourceType, metric string }{
		{"vm", "cpu"}, {"vm", "memory"}, {"vm", "disk"}, {"vm", "diskRead"}, {"vm", "diskWrite"}, {"vm", "networkIn"}, {"vm", "networkOut"},
		{"node", "temperature"}, {"agent", "memory"}, {"pbs", "memory"}, {"storage", "usage"}, {"truenas-system", "memory"},
		{"truenas-pool", "usage"}, {"vmware-vm", "memory"}, {"k8s-node", "memory"}, {"pod", "networkIn"},
	}
	for _, tc := range cases {
		gaps := []string{"nil", "NaN", "+Inf"}
		if isPercentageMetric(tc.metric) {
			gaps = append(gaps, "negative", "over-100")
		}
		for _, gap := range gaps {
			t.Run(tc.resourceType+"/"+tc.metric+"/"+gap, func(t *testing.T) {
				m, elapsed := continuityManager(t, true)
				threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
				input := &UnifiedResourceInput{ID: "target", Type: tc.resourceType, Name: "target"}
				thresholds := ThresholdConfig{CPU: threshold, Memory: threshold, Disk: threshold, DiskRead: threshold, DiskWrite: threshold, NetworkIn: threshold, NetworkOut: threshold, Temperature: threshold, Usage: threshold}
				set := func(sample *UnifiedResourceMetric) {
					switch tc.metric {
					case "cpu":
						input.CPU = sample
					case "memory":
						input.Memory = sample
					case "disk", "usage":
						input.Disk = sample
					case "temperature":
						input.Temperature = sample
					case "diskRead":
						input.DiskRead = sample
					case "diskWrite":
						input.DiskWrite = sample
					case "networkIn":
						input.NetworkIn = sample
					case "networkOut":
						input.NetworkOut = sample
					}
				}
				set(&UnifiedResourceMetric{Value: 85, Percent: 85})
				m.evaluateUnifiedMetrics(input, thresholds, nil)
				if _, ok := continuityIncident(m, input.ID, tc.metric); !ok {
					t.Fatal("known observation did not start a run")
				}
				elapsed.Store(int64(2 * time.Minute))
				invalid := &UnifiedResourceMetric{}
				switch gap {
				case "nil":
					invalid = nil
				case "NaN":
					invalid.Value = math.NaN()
					invalid.Percent = math.NaN()
				case "+Inf":
					invalid.Value = math.Inf(1)
					invalid.Percent = math.Inf(1)
				case "negative":
					invalid.Value = -1
					invalid.Percent = -1
				case "over-100":
					invalid.Value = 101
					invalid.Percent = 101
				}
				set(invalid)
				m.evaluateUnifiedMetrics(input, thresholds, nil)
				if _, ok := continuityIncident(m, input.ID, tc.metric); ok {
					t.Fatal("absent or rejected observation retained a pending run")
				}
				set(&UnifiedResourceMetric{Value: 85, Percent: 85})
				m.evaluateUnifiedMetrics(input, thresholds, nil)
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("a fresh observation completed grace across missing telemetry")
				}
			})
		}
	}
}

func TestGuestFilesystemGapMatchesMigratedAndEmptyInventory(t *testing.T) {
	m, elapsed := continuityManager(t, true)
	threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	ids := []string{"site:oldnode:105-disk-root", "site:othernode:105-disk-data", "site:oldnode:1050-disk-root", "elsewhere:oldnode:105-disk-root"}
	for _, id := range ids {
		m.checkMetric(id, "filesystem", "oldnode", "site", "vm", "disk", 85, threshold, nil)
	}
	elapsed.Store(int64(2 * time.Minute))
	vm := models.VM{ID: "site:newnode:105", VMID: 105, Name: "guest", Node: "newnode", Instance: "site", Type: "qemu", Status: "running", Memory: models.Memory{Usage: 10}, Disk: models.Disk{Usage: -1}, DiskStatusReason: "prev-vm-locked"}
	m.CheckGuest(vm, "site")
	for i, id := range ids {
		_, exists := continuityIncident(m, id, "disk")
		if exists != (i >= 2) {
			t.Fatalf("migration/exact identity boundary at %s: exists=%v", id, exists)
		}
	}
	m.mu.RLock()
	for _, pending := range m.intentPending {
		if pending.ResourceID == ids[0] || pending.ResourceID == ids[1] {
			t.Error("migrated filesystem grace remained durable")
		}
	}
	m.mu.RUnlock()
	if len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
		t.Fatal("unknown empty inventory emitted lifecycle evidence")
	}
}
