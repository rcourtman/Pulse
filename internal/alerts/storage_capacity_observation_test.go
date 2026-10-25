package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func storageObservationManager(t *testing.T, intent bool) (*Manager, *atomic.Int64) {
	t.Helper()
	m, clock := continuityManager(t, intent)
	m.mu.Lock()
	m.config.StorageDefault = HysteresisThreshold{Trigger: 80, Clear: 70}
	if !intent {
		m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"usage": 0}}
	}
	m.mu.Unlock()
	return m, clock
}

func storageObservationSample(id string, usage float64) models.Storage {
	return models.Storage{ID: id, Name: "backups", Node: "pve", Instance: "site", Type: "dir", Status: "available", Total: 1000, Used: int64(usage * 10), Free: 1000 - int64(usage*10), Usage: usage}
}

func storageObservationGap(s models.Storage, gap string) models.Storage {
	switch gap {
	case "missing":
		s.Total, s.Used, s.Free, s.Usage = 0, 0, 0, 0
	case "no-capacity":
		s.Total = 0
	case "negative-capacity":
		s.Total = -1
	case "negative-used":
		s.Used = -1
	case "over-capacity":
		s.Used = s.Total + 1
	case "negative-percent":
		s.Usage = -1
	case "over-100":
		s.Usage = 101
	case "NaN":
		s.Usage = math.NaN()
	case "+Inf":
		s.Usage = math.Inf(1)
	case "unconfirmed-zero":
		s.Usage = 0
	}
	return s
}

func storageObservationJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStorageCapacityUnknownPreservesOccurrence(t *testing.T) {
	for _, gap := range []string{"missing", "no-capacity", "negative-capacity", "negative-used", "over-capacity", "negative-percent", "over-100", "NaN", "+Inf", "unconfirmed-zero"} {
		t.Run(gap, func(t *testing.T) {
			m, clock := storageObservationManager(t, false)
			s := storageObservationSample("site-pve-backups", 95)
			m.CheckStorage(s)
			alert := testRequireActiveAlert(t, m, canonicalMetricStateID(s.ID, "usage"))
			if err := m.AcknowledgeAlert(alert.ID, "operator"); err != nil {
				t.Fatal(err)
			}
			before := storageObservationJSON(t, m.GetActiveAlerts())
			events := len(queryAlertEvents(t, m, eventlog.Filter{}))
			clock.Store(int64(2 * time.Minute))
			// A contradictory low percentage must not recover; a high one must not
			// refresh the occurrence's trusted evidence or severity either.
			for _, value := range []float64{10, 98} {
				for range 3 {
					m.CheckStorage(storageObservationGap(storageObservationSample(s.ID, value), gap))
				}
				if !bytes.Equal(before, storageObservationJSON(t, m.GetActiveAlerts())) || len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
					t.Fatalf("%s at %.0f%% changed occurrence, acknowledgement, evidence or lifecycle", gap, value)
				}
			}
			m.CheckStorage(storageObservationSample(s.ID, 0))
			if testHasActiveAlert(t, m, canonicalMetricStateID(s.ID, "usage")) || len(queryAlertEvents(t, m, eventlog.Filter{AlertID: alert.ID, Types: []string{eventlog.TypeResolved}})) != 1 {
				t.Fatal("confirmed empty storage must resolve the original exactly once")
			}
		})
	}
}

func TestStorageCapacityUnknownCannotActivateStaticOrForecast(t *testing.T) {
	for _, forecast := range []bool{false, true} {
		for _, gap := range []string{"missing", "no-capacity", "negative-capacity", "negative-used", "over-capacity", "negative-percent", "over-100", "NaN", "+Inf", "unconfirmed-zero"} {
			t.Run(fmt.Sprintf("forecast=%v/%s", forecast, gap), func(t *testing.T) {
				m, clock := storageObservationManager(t, false)
				value := 95.0
				if forecast {
					value = 72
				}
				s := storageObservationGap(storageObservationSample("site-pve-backups", value), gap)
				trend := CapacityTrendObservation{}
				if forecast {
					trend = storageObservationTrend()
				}
				for range 3 {
					m.CheckStorageWithCapacityTrend(s, trend)
					clock.Add(int64(time.Minute))
				}
				if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
					t.Fatal("unobserved capacity invented an incident")
				}
			})
		}
	}
}

func storageObservationTrend() CapacityTrendObservation {
	return CapacityTrendObservation{Ready: true, Reason: "increasing", DailyChange: 5, Confidence: .98, ObservedAt: time.Now(), SampleCount: 400, BucketCount: 72, CoverageSpan: 72 * time.Hour}
}

func TestStorageCapacityGapRestartsActivation(t *testing.T) {
	for _, intent := range []bool{false, true} {
		for _, forecast := range []bool{false, true} {
			for _, gap := range []string{"missing", "no-capacity", "negative-used"} {
				t.Run(fmt.Sprintf("intent=%v/forecast=%v/%s", intent, forecast, gap), func(t *testing.T) {
					m, clock := storageObservationManager(t, intent)
					m.mu.Lock()
					m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"usage": 60}}
					m.mu.Unlock()
					value := 85.0
					trend := CapacityTrendObservation{}
					if forecast {
						value = 72
						trend = storageObservationTrend()
					}
					s := storageObservationSample("site-pve-backups", value)
					other := storageObservationSample("other-pve-backups", value)
					observe := func(sample models.Storage) { m.CheckStorageWithCapacityTrend(sample, trend) }
					observe(s)
					observe(other)
					if i, ok := continuityIncident(m, s.ID, "usage"); !ok || i.State != reducer.StatePending {
						t.Fatal("control must start pending activation")
					}
					clock.Store(int64(2 * time.Minute))
					observe(storageObservationGap(s, gap))
					if _, ok := continuityIncident(m, s.ID, "usage"); ok {
						t.Fatal("gap retained confirmation or timed activation")
					}
					if _, ok := continuityIncident(m, other.ID, "usage"); !ok {
						t.Fatal("gap crossed storage identity")
					}
					observe(s)
					if len(m.GetActiveAlerts()) != 0 {
						t.Fatal("first fresh observation fired across the gap")
					}
					if !forecast || intent {
						clock.Store(int64(2*time.Minute + 59*time.Second))
						observe(s)
						if len(m.GetActiveAlerts()) != 0 {
							t.Fatal("activation completed before fresh grace")
						}
					}
					clock.Store(int64(3 * time.Minute))
					observe(s)
					if len(m.GetActiveAlerts()) != 1 {
						t.Fatal("full fresh activation did not fire once")
					}
					if m.ShadowDivergences() != 0 {
						t.Fatal("shadow diverged")
					}
				})
			}
		}
	}
}

func TestStorageCapacityUnknownHonoursPolicyAndConnectivity(t *testing.T) {
	for _, policy := range []string{"usage-off", "alias-off", "resource-off", "all-storage-off", "pause"} {
		for _, gap := range []string{"missing", "NaN"} {
			t.Run(policy+"/"+gap, func(t *testing.T) {
				m, _ := storageObservationManager(t, false)
				s := storageObservationSample("site-pve-backups", 95)
				s.AliasIDs = []string{"site/pve/backups"}
				m.CheckStorage(s)
				id := canonicalMetricStateID(s.ID, "usage")
				testRequireActiveAlert(t, m, id)
				m.mu.Lock()
				switch policy {
				case "usage-off":
					m.config.StorageDefault = HysteresisThreshold{}
				case "alias-off":
					m.config.Overrides[s.AliasIDs[0]] = ThresholdConfig{Usage: &HysteresisThreshold{}}
				case "resource-off":
					m.config.Overrides[s.ID] = ThresholdConfig{Disabled: true}
				case "all-storage-off":
					m.config.DisableAllStorage = true
				case "pause":
					m.config.Enabled = false
				}
				m.mu.Unlock()
				missing := storageObservationGap(s, gap)
				missing.Status = "unavailable"
				m.CheckStorage(missing)
				if testHasActiveAlert(t, m, id) != (policy == "pause") {
					t.Fatal("observation gap overrode explicit policy")
				}
				if policy == "usage-off" || policy == "alias-off" {
					for range 3 {
						m.CheckStorage(missing)
					}
					testRequireActiveAlert(t, m, canonicalConnectivityStateID(s.ID))
				}
			})
		}
	}
}

func TestStorageCapacityUnknownHoldsForecastAndIndependentHealth(t *testing.T) {
	m, clock := storageObservationManager(t, false)
	s := storageObservationSample("site-pve-backups", 72)
	trend := storageObservationTrend()
	for range 2 {
		m.CheckStorageWithCapacityTrend(s, trend)
	}
	alert := testRequireActiveAlert(t, m, canonicalMetricStateID(s.ID, "usage"))
	if alert.Metadata[capacityAlertOriginKey] != capacityAlertOriginForecast {
		t.Fatal("control must be a forecast")
	}
	before := storageObservationJSON(t, m.GetActiveAlerts())
	clock.Store(int64(2 * time.Minute))
	gap := storageObservationGap(storageObservationSample(s.ID, 10), "negative-used")
	for range 3 {
		m.CheckStorageWithCapacityTrend(gap, CapacityTrendObservation{Ready: true, Reason: "not-increasing"})
	}
	if !bytes.Equal(before, storageObservationJSON(t, m.GetActiveAlerts())) {
		t.Fatal("unobserved low capacity resolved the forecast")
	}
	// Missing capacity does not suppress independent connectivity evaluation.
	gap.Status = "unavailable"
	for range 3 {
		m.CheckStorage(gap)
	}
	testRequireActiveAlert(t, m, canonicalConnectivityStateID(s.ID))
	testRequireActiveAlert(t, m, canonicalMetricStateID(s.ID, "usage"))
	// A real low reading still clears capacity while preserving the outage.
	gap = storageObservationSample(s.ID, 0)
	gap.Status = "unknown"
	m.CheckStorage(gap)
	if testHasActiveAlert(t, m, canonicalMetricStateID(s.ID, "usage")) {
		t.Fatal("confirmed empty store did not recover capacity")
	}
	testRequireActiveAlert(t, m, canonicalConnectivityStateID(s.ID))
}

// Only synthetic manager payloads; no transport or installed recovery claim.
func TestStorageCapacityBrowserProducer(t *testing.T) {
	m, clock := storageObservationManager(t, false)
	s := storageObservationSample("site-browser-backups", 95)
	frames := map[string]json.RawMessage{}
	for _, phase := range []string{"breach", "missing", "over-capacity", "recovered"} {
		sample := s
		switch phase {
		case "missing":
			sample = storageObservationGap(storageObservationSample(s.ID, 10), "no-capacity")
		case "over-capacity":
			sample = storageObservationGap(storageObservationSample(s.ID, 10), phase)
		case "recovered":
			sample = storageObservationSample(s.ID, 0)
		}
		clock.Add(int64(time.Second))
		m.CheckStorage(sample)
		active := m.GetActiveAlerts()
		want := 1
		if phase == "recovered" {
			want = 0
		}
		if len(active) != want {
			t.Fatalf("phase %s: %d active, want %d", phase, len(active), want)
		}
		frames[phase] = storageObservationJSON(t, active)
	}
	if !bytes.Equal(frames["breach"], frames["missing"]) || !bytes.Equal(frames["breach"], frames["over-capacity"]) {
		t.Fatal("gap changed exported occurrence")
	}
	if os.Getenv("PULSE_STORAGE_CAPACITY_FRAMES") == "1" {
		fmt.Printf("STORAGE_CAPACITY_FRAMES=%s\n", storageObservationJSON(t, frames))
	}
}
