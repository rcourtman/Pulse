package alerts

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestCheckHostAlertsWhenSMARTCRCCountIncreases(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	crcErrors := int64(7)
	host := models.Host{
		ID:       "crc-host",
		Hostname: "crc-host",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device: "/dev/sda",
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					UDMACRCErrors: &crcErrors,
				},
			}},
		},
	}

	alertKey := buildCanonicalStateID("agent:crc-host/disk:sda", "agent:crc-host/disk:sda-disk-health")
	m.CheckHost(host)

	m.mu.RLock()
	_, exists := testLookupActiveAlert(t, m, alertKey)
	m.mu.RUnlock()
	if exists {
		t.Fatal("historical CRC errors must not alert on the first observation")
	}

	crcErrors = 8
	m.CheckHost(host)

	m.mu.RLock()
	alert := testRequireActiveAlert(t, m, alertKey)
	m.mu.RUnlock()
	if alert.Level != AlertLevelWarning {
		t.Fatalf("Level = %q, want %q", alert.Level, AlertLevelWarning)
	}
	codes, ok := alert.Metadata["riskCodes"].([]string)
	if !ok || !slices.Contains(codes, "crc_errors_increased") {
		t.Fatalf("riskCodes = %#v, want crc_errors_increased", alert.Metadata["riskCodes"])
	}
	summaries, ok := alert.Metadata["riskSummaries"].([]string)
	if !ok || len(summaries) != 1 || !strings.Contains(summaries[0], "increased from 7 to 8") {
		t.Fatalf("riskSummaries = %#v, want counter delta", alert.Metadata["riskSummaries"])
	}

	// Growth is an event, not a warning on the historical non-zero value. Once
	// the next sample is stable, the active alert resolves while history keeps it.
	m.CheckHost(host)
	m.mu.RLock()
	_, exists = testLookupActiveAlert(t, m, alertKey)
	m.mu.RUnlock()
	if exists {
		t.Fatal("stable CRC counter should resolve the growth alert")
	}
}

func TestCheckHostSMARTCRCCounterResetEstablishesNewBaseline(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	crcErrors := int64(10)
	host := models.Host{
		ID: "reset-host",
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
			Device:     "/dev/sdb",
			Attributes: &models.SMARTAttributes{UDMACRCErrors: &crcErrors},
		}}},
	}

	alertKey := buildCanonicalStateID("agent:reset-host/disk:sdb", "agent:reset-host/disk:sdb-disk-health")
	m.CheckHost(host)
	crcErrors = 1
	m.CheckHost(host)

	m.mu.RLock()
	_, exists := testLookupActiveAlert(t, m, alertKey)
	m.mu.RUnlock()
	if exists {
		t.Fatal("counter reset must not be treated as growth")
	}

	crcErrors = 2
	m.CheckHost(host)
	m.mu.RLock()
	alert := testRequireActiveAlert(t, m, alertKey)
	m.mu.RUnlock()
	if alert.Level != AlertLevelWarning {
		t.Fatalf("Level = %q, want %q", alert.Level, AlertLevelWarning)
	}
}

func TestCleanupRemovesOnlyStaleSMARTCounterSnapshots(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()

	m.mu.Lock()
	m.smartCounterSnapshots["agent:old/disk:sda"] = smartCounterSnapshot{
		UDMACRCErrors: 3,
		LastObserved:  now.Add(-25 * time.Hour),
	}
	m.smartCounterSnapshots["agent:current/disk:sdb"] = smartCounterSnapshot{
		UDMACRCErrors: 4,
		LastObserved:  now.Add(-time.Hour),
	}
	m.mu.Unlock()

	m.Cleanup(time.Hour)

	m.mu.RLock()
	_, oldExists := m.smartCounterSnapshots["agent:old/disk:sda"]
	_, currentExists := m.smartCounterSnapshots["agent:current/disk:sdb"]
	m.mu.RUnlock()
	if oldExists {
		t.Fatal("stale SMART counter snapshot was not removed")
	}
	if !currentExists {
		t.Fatal("current SMART counter snapshot was removed")
	}
}
