package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestCheckPMGAnomalies_QuietSite(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	pmgID := "pmg1"
	pmgName := "PMG 1"

	// Helper to feed a sample
	feedSample := func(timestamp time.Time, spamIn int) {
		sample := models.PMGMailCountPoint{
			Timestamp: timestamp,
			SpamIn:    float64(spamIn),
			SpamOut:   5.0,
			VirusIn:   2.0,
			VirusOut:  0.0,
		}
		pmg := models.PMGInstance{
			ID:        pmgID,
			Name:      pmgName,
			MailCount: []models.PMGMailCountPoint{sample},
		}
		m.checkPMGAnomalies(pmg, PMGThresholdConfig{})
	}

	// 1. Warmup: Feed 24 samples of steady traffic (10 spam/hour)
	start := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 24; i++ {
		feedSample(start.Add(time.Duration(i)*time.Hour), 10)
	}

	// Verify no alerts
	if len(m.GetActiveAlerts()) != 0 {
		t.Errorf("Expected no alerts after steady traffic warmup")
	}

	// 2. Trigger Pending: Feed a spike (100 spam/hour)
	// Baseline should be ~10. 100 is > 10 * 2.5 (CritRatio) and > 10 + 300 (CritDelta)?
	// Wait, CritDelta = Baseline + 300. 10 + 300 = 310.
	// So 100 is NOT Critical if CritDelta is 310.

	// Let's check the logic:
	// Normal site (baseline >= 40): CritDelta = Baseline + 300
	// Quiet site (baseline < 40): CritDelta = Baseline + 120, WarnDelta = Baseline + 60

	// Our baseline is 10. So it's a "Quiet site".
	// WarnDelta = 10 + 60 = 70.
	// 100 > 70. So it should trigger Warning.

	spikeTime := time.Now()
	feedSample(spikeTime, 100)

	// Verify pending
	pendingKey := "pmg-anomaly-pmg1-spamIn"
	m.mu.Lock()
	_, isPending := m.pendingAlerts[pendingKey]
	m.mu.Unlock()
	if !isPending {
		t.Errorf("Expected pending alert for first spike")
	}
	if len(m.GetActiveAlerts()) != 0 {
		t.Errorf("Expected no active alerts for first spike")
	}

	// 3. Confirm Alert: Feed another spike
	feedSample(spikeTime.Add(1*time.Hour), 110)

	// Verify alert
	alerts := m.GetActiveAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert after second spike, got %d", len(alerts))
	} else {
		if alerts[0].Type != "anomaly-spamIn" {
			t.Errorf("Expected anomaly-spamIn alert, got %s", alerts[0].Type)
		}
		if alerts[0].Level != AlertLevelWarning {
			t.Errorf("Expected Warning level (quiet site < 120 delta), got %s", alerts[0].Level)
		}
	}

	// 4. Clear Alert: Return to normal (10 spam/hour)
	feedSample(spikeTime.Add(2*time.Hour), 10)

	if len(m.GetActiveAlerts()) != 0 {
		t.Errorf("Expected alert to clear after return to normal")
	}
}

func TestCheckPMGAnomalies_NormalSite(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	pmgID := "pmg2"
	pmgName := "PMG 2"

	// Helper to feed a sample
	feedSample := func(timestamp time.Time, spamIn int) {
		sample := models.PMGMailCountPoint{
			Timestamp: timestamp,
			SpamIn:    float64(spamIn),
			SpamOut:   5,
			VirusIn:   2,
			VirusOut:  0,
		}
		pmg := models.PMGInstance{
			ID:        pmgID,
			Name:      pmgName,
			MailCount: []models.PMGMailCountPoint{sample},
		}
		m.checkPMGAnomalies(pmg, PMGThresholdConfig{})
	}

	// 1. Warmup: Feed 24 samples of steady traffic (50 spam/hour) for "Normal" site (>= 40)
	baseline := 50
	start := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 24; i++ {
		feedSample(start.Add(time.Duration(i)*time.Hour), baseline)
	}

	// 2. Trigger Warning:
	// Normal site: WarnRatio = 1.8, WarnDelta = Baseline + 150
	// 50 * 1.8 = 90.
	// 50 + 150 = 200.
	// Must exceed BOTH. So need > 200.

	spikeTime := time.Now()
	val := 210
	feedSample(spikeTime, val) // Pending

	// Verify pending
	pendingKey := "pmg-anomaly-pmg2-spamIn"
	m.mu.Lock()
	_, isPending := m.pendingAlerts[pendingKey]
	m.mu.Unlock()
	if !isPending {
		t.Errorf("Expected pending alert for first spike")
	}

	// Confirm
	feedSample(spikeTime.Add(1*time.Hour), val) // Alert

	alerts := m.GetActiveAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	} else {
		if alerts[0].Level != AlertLevelWarning {
			t.Errorf("Expected Warning level, got %s", alerts[0].Level)
		}
	}
}

func TestCheckPMGAnomalies_NormalSite_Critical(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	pmgID := "pmg3"
	pmgName := "PMG 3"

	feedSample := func(timestamp time.Time, spamIn int) {
		sample := models.PMGMailCountPoint{
			Timestamp: timestamp,
			SpamIn:    float64(spamIn),
			SpamOut:   5,
			VirusIn:   2,
			VirusOut:  0,
		}
		pmg := models.PMGInstance{
			ID:        pmgID,
			Name:      pmgName,
			MailCount: []models.PMGMailCountPoint{sample},
		}
		m.checkPMGAnomalies(pmg, PMGThresholdConfig{})
	}

	baseline := 50
	start := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 24; i++ {
		feedSample(start.Add(time.Duration(i)*time.Hour), baseline)
	}

	// Normal site: CritRatio = 2.5, CritDelta = Baseline + 300
	// 50 * 2.5 = 125.
	// 50 + 300 = 350.
	// Must exceed BOTH. So need > 350.

	spikeTime := time.Now()
	val := 360
	feedSample(spikeTime, val)                  // Pending
	feedSample(spikeTime.Add(1*time.Hour), val) // Alert

	alerts := m.GetActiveAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	} else {
		if alerts[0].Level != AlertLevelCritical {
			t.Errorf("Expected Critical level, got %s", alerts[0].Level)
		}
	}
}

func TestCheckPMGAnomalies_QuietSite_Critical(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	pmgID := "pmg1-crit"
	pmgName := "PMG 1 Critical"

	feedSample := func(timestamp time.Time, spamIn int) {
		sample := models.PMGMailCountPoint{
			Timestamp: timestamp,
			SpamIn:    float64(spamIn),
			SpamOut:   5,
			VirusIn:   2,
			VirusOut:  0,
		}
		pmg := models.PMGInstance{
			ID:        pmgID,
			Name:      pmgName,
			MailCount: []models.PMGMailCountPoint{sample},
		}
		m.checkPMGAnomalies(pmg, PMGThresholdConfig{})
	}

	// Warmup: Steady 10 spam/hour (Baseline = 10)
	baseline := 10
	start := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 24; i++ {
		feedSample(start.Add(time.Duration(i)*time.Hour), baseline)
	}

	// Quiet site (Baseline < 40): CritDelta = Baseline + 120 = 130.
	// Feed 140.

	spikeTime := time.Now()
	val := 140
	feedSample(spikeTime, val)                  // Pending
	feedSample(spikeTime.Add(1*time.Hour), val) // Alert

	alerts := m.GetActiveAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	} else {
		if alerts[0].Level != AlertLevelCritical {
			t.Errorf("Expected Critical level, got %s", alerts[0].Level)
		}
	}
}
