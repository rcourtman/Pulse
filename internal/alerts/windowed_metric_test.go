package alerts

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestCalculateMetricWindowUsesTimeWeightedAverage(t *testing.T) {
	end := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	start := end.Add(-5 * time.Minute)
	points := []MetricWindowPoint{
		{Timestamp: start, Value: 40},
		{Timestamp: start.Add(2 * time.Minute), Value: 40},
		{Timestamp: start.Add(4 * time.Minute), Value: 40},
		{Timestamp: end, Value: 100},
	}

	got := calculateMetricWindow(points, start, end, 100, 300)
	if !got.Ready {
		t.Fatal("expected complete window to be ready")
	}
	if math.Abs(got.Value-46) > 0.001 {
		t.Fatalf("expected a time-weighted average of 46, got %.3f", got.Value)
	}
	if got.SampleCount != 4 || got.CoverageSeconds != 300 {
		t.Fatalf("unexpected evidence: %+v", got)
	}
}

func TestCalculateMetricWindowRejectsIncompleteAndGappedEvidence(t *testing.T) {
	end := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	start := end.Add(-5 * time.Minute)

	for name, points := range map[string][]MetricWindowPoint{
		"too few samples": {
			{Timestamp: start, Value: 90},
			{Timestamp: end, Value: 90},
		},
		"insufficient coverage": {
			{Timestamp: end.Add(-2 * time.Minute), Value: 90},
			{Timestamp: end.Add(-time.Minute), Value: 90},
			{Timestamp: end, Value: 90},
		},
		"large internal gap": {
			{Timestamp: start, Value: 90},
			{Timestamp: start.Add(30 * time.Second), Value: 90},
			{Timestamp: end, Value: 90},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := calculateMetricWindow(points, start, end, 90, 300); got.Ready {
				t.Fatalf("expected weak evidence to remain unknown, got %+v", got)
			}
		})
	}
}

func TestWindowedMetricSpikeDoesNotFireAndMetadataExplainsAverage(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	defer m.Stop()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	cfg := m.GetConfig()
	cfg.ActivationState = ActivationActive
	cfg.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": 300}}
	m.UpdateConfig(cfg)
	m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
		return []MetricWindowPoint{
			{Timestamp: request.Start, Value: 40},
			{Timestamp: request.Start.Add(2 * time.Minute), Value: 40},
			{Timestamp: request.Start.Add(4 * time.Minute), Value: 40},
		}, nil
	})

	threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 100, threshold, nil)
	if alerts := m.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("expected a short CPU spike to be filtered, got %+v", alerts)
	}

	m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
		return []MetricWindowPoint{
			{Timestamp: request.Start, Value: 90},
			{Timestamp: request.Start.Add(2 * time.Minute), Value: 90},
			{Timestamp: request.Start.Add(4 * time.Minute), Value: 90},
		}, nil
	})
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	now = now.Add(6 * time.Second)
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected sustained CPU to fire, got %+v", active)
	}
	alert := active[0]
	if alert.Metadata["evaluationMode"] != "rolling_average" || alert.Metadata["evaluationWindowSeconds"] != 300 {
		t.Fatalf("missing rolling-window evidence: %+v", alert.Metadata)
	}
	if alert.Message == "" || alert.Value != 90 {
		t.Fatalf("unexpected rolling alert presentation: %+v", alert)
	}
}

func TestWindowedMetricUnknownHistoryPreservesActiveIncident(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	defer m.Stop()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	cfg := m.GetConfig()
	cfg.ActivationState = ActivationActive
	cfg.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": 300}}
	m.UpdateConfig(cfg)
	complete := true
	m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
		if !complete {
			return nil, nil
		}
		return []MetricWindowPoint{
			{Timestamp: request.Start, Value: 90},
			{Timestamp: request.Start.Add(2 * time.Minute), Value: 90},
			{Timestamp: request.Start.Add(4 * time.Minute), Value: 90},
		}, nil
	})
	threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	now = now.Add(6 * time.Second)
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 90, threshold, nil)
	complete = false
	now = now.Add(time.Minute)
	m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 10, threshold, nil)
	if active := m.GetActiveAlerts(); len(active) != 1 {
		t.Fatalf("expected unknown history to preserve the incident, got %+v", active)
	}
}

func nonFiniteMetricSamples() map[string]float64 {
	return map[string]float64{"NaN": math.NaN(), "positive infinity": math.Inf(1), "negative infinity": math.Inf(-1)}
}

func TestMetricEvaluationRejectsNonFiniteCurrent(t *testing.T) {
	for _, mode := range []string{"instant", "window without provider", "window with provider"} {
		for name, value := range nonFiniteMetricSamples() {
			t.Run(mode+"/"+name, func(t *testing.T) {
				m := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
				t.Cleanup(m.Stop)
				window := 0
				if mode != "instant" {
					window = 300
				}
				m.mu.Lock()
				m.config.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": window}}
				m.mu.Unlock()
				calls := 0
				if mode == "window with provider" {
					m.SetMetricWindowProvider(func(MetricWindowRequest) ([]MetricWindowPoint, error) {
						calls++
						return nil, nil
					})
				}
				got := m.evaluateMetricWindow("vm-1", "vm", "cpu", value, time.Now())
				if got.Ready || calls != 0 {
					t.Fatalf("invalid current is not trustworthy or a reason to query history: ready=%v calls=%d", got.Ready, calls)
				}
			})
		}
	}
}

func TestMetricWindowRejectsNonFiniteEvidence(t *testing.T) {
	end := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	start := end.Add(-4 * time.Minute)
	for name, value := range nonFiniteMetricSamples() {
		for _, position := range []string{"first", "middle", "last", "current"} {
			t.Run(name+"/"+position, func(t *testing.T) {
				points := []MetricWindowPoint{{start, 90}, {start.Add(2 * time.Minute), 90}, {end, 90}}
				current := 90.0
				switch position {
				case "first":
					points[0].Value = value
				case "middle":
					points[1].Value = value
				case "last":
					points[2].Value = value
				case "current":
					current = value
				}
				if got := calculateMetricWindow(points, start, end, current, 240); got.Ready {
					t.Fatalf("non-finite %s was marked ready: %+v", position, got)
				}
			})
		}
	}
	for name, value := range map[string]float64{"positive overflow": math.MaxFloat64, "negative overflow": -math.MaxFloat64} {
		t.Run(name, func(t *testing.T) {
			points := []MetricWindowPoint{{start, value}, {start.Add(2 * time.Minute), value}, {end, value}}
			if got := calculateMetricWindow(points, start, end, value, 240); got.Ready {
				t.Fatalf("non-finite weighted result was marked ready: %+v", got)
			}
		})
	}
}

func TestMetricWindowFiniteAuthorityAndInputIsolation(t *testing.T) {
	end := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	start := end.Add(-4 * time.Minute)
	for name, invalid := range nonFiniteMetricSamples() {
		t.Run(name, func(t *testing.T) {
			// Unordered input and obsolete invalid duplicates are not evidence.
			points := []MetricWindowPoint{{end, invalid}, {start, 90}, {start.Add(2 * time.Minute), 90}, {end, 90}, {start.Add(-time.Second), invalid}, {end.Add(time.Second), invalid}}
			before := append([]MetricWindowPoint(nil), points...)
			got := calculateMetricWindow(points, start, end, 90, 240)
			if !got.Ready || got.Value != 90 || got.SampleCount != 3 || got.CoverageSeconds != 240 {
				t.Fatalf("last in-range duplicate must remain authoritative: %+v", got)
			}
			for i := range points {
				if points[i].Timestamp != before[i].Timestamp || math.Float64bits(points[i].Value) != math.Float64bits(before[i].Value) {
					t.Fatal("window evaluation mutated provider evidence")
				}
			}
			points = append(points, MetricWindowPoint{end, invalid})
			if got := calculateMetricWindow(points, start, end, 90, 240); got.Ready {
				t.Fatal("invalid latest duplicate must not resurrect the older finite value")
			}
		})
	}
	for _, value := range []float64{-20, 0, 90, 1e300} {
		points := []MetricWindowPoint{{start, value}, {start.Add(2 * time.Minute), value}, {end, value}}
		got := calculateMetricWindow(points, start, end, value, 240)
		if !got.Ready || math.Abs(got.Value-value) > math.Abs(value)*1e-14 {
			t.Fatalf("finite values must not be clipped or made unknown: value=%v result=%+v", value, got)
		}
	}
}

func TestNonFiniteMetricPreservesIncidentAndDeliveryEvidence(t *testing.T) {
	for _, mode := range []string{"legacy instant", "canonical instant", "legacy history", "canonical history"} {
		for name, invalid := range nonFiniteMetricSamples() {
			t.Run(mode+"/"+name, func(t *testing.T) {
				m := newEventLogManager(t)
				m.mu.Lock()
				m.config.TimeThresholds = map[string]int{}
				m.config.MinimumDelta = 0
				m.config.SuppressionWindow = 0
				m.config.FlappingEnabled = false
				window := 0
				withHistory := mode == "legacy history" || mode == "canonical history"
				if withHistory {
					window = 300
				}
				m.config.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": window}}
				m.mu.Unlock()
				now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
				m.now = func() time.Time { return now }
				historyValue := 85.0
				m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
					return []MetricWindowPoint{{request.Start, historyValue}, {request.Start.Add(2 * time.Minute), historyValue}, {request.Start.Add(4 * time.Minute), historyValue}}, nil
				})
				threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
				spec, err := buildCanonicalMetricSpec("vm-1", "web-1", unifiedresources.ResourceTypeVM, "cpu", threshold)
				if err != nil {
					t.Fatal(err)
				}
				var deliveries atomic.Int64
				fired := make(chan struct{}, 8)
				m.SetAlertCallback(func(*Alert) { deliveries.Add(1); fired <- struct{}{} })
				evaluate := func(value float64) {
					if mode == "canonical instant" || mode == "canonical history" {
						m.evaluateCanonicalMetricAlert(spec, "web-1", "node-1", "pve-1", "vm", value, threshold, nil)
					} else {
						m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", value, threshold, nil)
					}
				}
				evaluate(85)
				select {
				case <-fired:
				case <-time.After(5 * time.Second):
					t.Fatal("finite trigger did not reach the asynchronous dispatch callback")
				}
				active := m.GetActiveAlerts()
				if len(active) != 1 || deliveries.Load() != 1 {
					t.Fatalf("finite trigger must fire and dispatch exactly once: active=%d deliveries=%d", len(active), deliveries.Load())
				}
				if err := m.AcknowledgeAlert(active[0].ID, "operator"); err != nil {
					t.Fatal(err)
				}
				before, err := json.Marshal(m.GetActiveAlerts())
				if err != nil {
					t.Fatal(err)
				}
				eventsBefore := len(queryAlertEvents(t, m, eventlog.Filter{}))
				now = now.Add(time.Minute)
				if withHistory {
					historyValue = invalid
					evaluate(10)
				} else {
					evaluate(invalid)
				}
				after, err := json.Marshal(m.GetActiveAlerts())
				if err != nil || !bytes.Equal(before, after) || deliveries.Load() != 1 {
					t.Fatalf("unknown evidence changed acknowledged incident/delivery or broke JSON: error=%v before=%s after=%s deliveries=%d", err, before, after, deliveries.Load())
				}
				if got := len(queryAlertEvents(t, m, eventlog.Filter{})); got != eventsBefore {
					t.Fatalf("unknown evidence changed durable transition ledger: before=%d after=%d", eventsBefore, got)
				}
				historyValue = 10
				now = now.Add(time.Minute)
				evaluate(10)
				evaluate(10)
				if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
					t.Fatal("fresh finite evidence must recover the same incident exactly once")
				}
			})
		}
	}
}

func TestNonFiniteMetricCannotActivateIncident(t *testing.T) {
	for _, window := range []int{0, 300} {
		for name, invalid := range nonFiniteMetricSamples() {
			t.Run((time.Duration(window)*time.Second).String()+"/"+name, func(t *testing.T) {
				m := newEventLogManager(t)
				m.mu.Lock()
				m.config.TimeThresholds = map[string]int{}
				m.config.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": window}}
				m.mu.Unlock()
				m.SetMetricWindowProvider(func(request MetricWindowRequest) ([]MetricWindowPoint, error) {
					return []MetricWindowPoint{{request.Start, invalid}, {request.Start.Add(2 * time.Minute), invalid}, {request.Start.Add(4 * time.Minute), invalid}}, nil
				})
				var deliveries atomic.Int64
				m.SetAlertCallback(func(*Alert) { deliveries.Add(1) })
				value := invalid
				if window > 0 {
					value = 85
				}
				m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", value, &HysteresisThreshold{Trigger: 80, Clear: 75}, nil)
				if len(m.GetActiveAlerts()) != 0 || deliveries.Load() != 0 || len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
					t.Fatal("invalid evidence opened an incident or dispatched a notification")
				}
			})
		}
	}
}

func TestNonFiniteMetricDoesNotPreventExplicitDisable(t *testing.T) {
	for _, canonical := range []bool{false, true} {
		m := newEventLogManager(t)
		m.mu.Lock()
		m.config.TimeThresholds = map[string]int{}
		m.config.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": 0}}
		m.mu.Unlock()
		threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
		spec, err := buildCanonicalMetricSpec("vm-1", "web-1", unifiedresources.ResourceTypeVM, "cpu", threshold)
		if err != nil {
			t.Fatal(err)
		}
		if canonical {
			m.evaluateCanonicalMetricAlert(spec, "web-1", "node-1", "pve-1", "vm", 85, threshold, nil)
			spec.Disabled = true
			m.evaluateCanonicalMetricAlert(spec, "web-1", "node-1", "pve-1", "vm", math.NaN(), threshold, nil)
		} else {
			m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", 85, threshold, nil)
			m.checkMetric("vm-1", "web-1", "node-1", "pve-1", "vm", "cpu", math.NaN(), nil, nil)
		}
		if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired}})) != 1 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
			t.Fatal("explicit disable must still resolve the active rule independently of telemetry")
		}
	}
}

// A skipped window must not leave grace on disk for a restart to resume.
func TestUnknownMetricObservationDropsDurableIntentCheckpoint(t *testing.T) {
	for _, route := range []string{"legacy", "canonical", "unified", "host", "guest-memory", "guest-filesystem-expired"} {
		t.Run(route, func(t *testing.T) {
			m, elapsed := continuityManager(t, true)
			gap := "missing"
			if route == "legacy" || route == "canonical" {
				gap = "history-empty"
			}
			_, _, observe := continuityObserver(t, m, route, gap)
			observe(85, false)
			elapsed.Store(int64(40 * time.Second))
			observe(85, false)
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(m.getAlertsDir(), intentPendingFileName)
			read := func() []IntentPendingState {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var saved []IntentPendingState
				if err := json.Unmarshal(data, &saved); err != nil {
					t.Fatal(err)
				}
				return saved
			}
			if len(read()) != 1 {
				t.Fatal("control did not retain a real in-progress grace checkpoint")
			}
			elapsed.Store(int64(2 * time.Minute))
			observe(85, true)
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			if len(read()) != 0 {
				t.Fatal("unknown observation left grace for a restart to reuse")
			}
			dir := filepath.Dir(m.getAlertsDir())
			m.Stop()
			restarted := NewManagerWithDataDir(dir)
			t.Cleanup(restarted.Stop)
			restarted.mu.RLock()
			pending := len(restarted.intentPending)
			restarted.mu.RUnlock()
			if pending != 0 {
				t.Fatal("restart restored interrupted grace")
			}
		})
	}
}
