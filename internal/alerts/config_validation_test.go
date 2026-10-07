package alerts

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestUpdateConfigNormalizesBackupIndicatorHours(t *testing.T) {
	t.Run("non-positive values default", func(t *testing.T) {
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled: true,
			BackupDefaults: BackupAlertConfig{
				WarningDays:  2,
				CriticalDays: 4,
				FreshHours:   -1,
				StaleHours:   0,
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.BackupDefaults.FreshHours != 24 {
			t.Fatalf("expected FreshHours to default to 24, got %d", m.config.BackupDefaults.FreshHours)
		}
		if m.config.BackupDefaults.StaleHours != 72 {
			t.Fatalf("expected StaleHours to default to 72 when invalid, got %d", m.config.BackupDefaults.StaleHours)
		}
	})

	t.Run("stale is clamped to fresh when lower", func(t *testing.T) {
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled: true,
			BackupDefaults: BackupAlertConfig{
				WarningDays:  2,
				CriticalDays: 4,
				FreshHours:   48,
				StaleHours:   24,
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.BackupDefaults.FreshHours != 48 {
			t.Fatalf("expected FreshHours to remain 48, got %d", m.config.BackupDefaults.FreshHours)
		}
		if m.config.BackupDefaults.StaleHours != 48 {
			t.Fatalf("expected StaleHours to clamp to FreshHours (48), got %d", m.config.BackupDefaults.StaleHours)
		}
	})
}

func TestUpdateConfigNormalizesFlappingSettings(t *testing.T) {
	m := newTestManager(t)

	cfg := AlertConfig{
		Enabled:                 true,
		FlappingEnabled:         true,
		FlappingWindowSeconds:   0,
		FlappingThreshold:       -3,
		FlappingCooldownMinutes: 0,
	}

	m.UpdateConfig(cfg)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.FlappingWindowSeconds != 300 {
		t.Fatalf("expected FlappingWindowSeconds to default to 300, got %d", m.config.FlappingWindowSeconds)
	}
	if m.config.FlappingThreshold != 5 {
		t.Fatalf("expected FlappingThreshold to default to 5, got %d", m.config.FlappingThreshold)
	}
	if m.config.FlappingCooldownMinutes != 15 {
		t.Fatalf("expected FlappingCooldownMinutes to default to 15, got %d", m.config.FlappingCooldownMinutes)
	}
}

// The thresholds page sends a positive trigger with clear max(0, trigger-5),
// so a 1-5% default arrives with clear 0. Agent defaults used to fall back to
// the factory clear, storing {trigger: 1, clear: 75}; the evaluator ignored
// that clear and cleared at the trigger, but firing alerts reported 75 as
// their clear level.
func TestUpdateConfigKeepsLowTriggerClearBelowTrigger(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(AlertConfig{
		Enabled: true,
		AgentDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 1, Clear: 0},
			Memory: &HysteresisThreshold{Trigger: 3, Clear: 0},
			Disk:   &HysteresisThreshold{Trigger: 5, Clear: 0},
		},
		NodeDefaults: ThresholdConfig{
			Temperature: &HysteresisThreshold{Trigger: 2, Clear: 0},
		},
		PBSDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 4, Clear: 0},
			Memory: &HysteresisThreshold{Trigger: 5, Clear: 0},
		},
	})

	cfg := m.GetConfig()
	for name, tc := range map[string]struct {
		got  *HysteresisThreshold
		want HysteresisThreshold
	}{
		"agent cpu":        {cfg.AgentDefaults.CPU, HysteresisThreshold{Trigger: 1, Clear: 0}},
		"agent memory":     {cfg.AgentDefaults.Memory, HysteresisThreshold{Trigger: 3, Clear: 0}},
		"agent disk":       {cfg.AgentDefaults.Disk, HysteresisThreshold{Trigger: 5, Clear: 0}},
		"node temperature": {cfg.NodeDefaults.Temperature, HysteresisThreshold{Trigger: 2, Clear: 0}},
		"pbs cpu":          {cfg.PBSDefaults.CPU, HysteresisThreshold{Trigger: 4, Clear: 0}},
		"pbs memory":       {cfg.PBSDefaults.Memory, HysteresisThreshold{Trigger: 5, Clear: 0}},
	} {
		if tc.got == nil || *tc.got != tc.want {
			t.Errorf("%s = %+v, want %+v", name, tc.got, tc.want)
		}
	}

	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()
	host := models.Host{
		ID:              "host-low-trigger",
		DisplayName:     "Low Trigger Host",
		Hostname:        "low-trigger.example",
		Memory:          models.Memory{Usage: 68.9, Total: 16384, Used: 11288, Free: 5096},
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	}
	m.CheckHost(host)
	memAlert, exists := testLookupActiveAlert(t, m, canonicalMetricStateID(hostResourceID(host.ID), "memory"))
	if !exists {
		t.Fatalf("expected memory alert at a 3%% trigger")
	}
	// No band below a 3% trigger: the alert clears at the trigger itself.
	if got := memAlert.Metadata["clearThreshold"]; got != 3.0 {
		t.Fatalf("memory alert clearThreshold = %v, want 3", got)
	}

	// A config saved before the fix is repaired on its next load.
	m.UpdateConfig(AlertConfig{
		Enabled: true,
		AgentDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 1, Clear: 75},
		},
	})
	if got := m.GetConfig().AgentDefaults.CPU; got == nil || *got != (HysteresisThreshold{Trigger: 1, Clear: 0}) {
		t.Fatalf("stored agent cpu {1 75} normalized to %+v, want {1 0}", got)
	}
}

func TestUpdateConfigNormalizesExactEscalationRoutingAndRepeatPolicy(t *testing.T) {
	m := newTestManager(t)
	cfg := m.GetConfig()
	cfg.Schedule.Escalation = EscalationConfig{
		Enabled:        true,
		RepeatCritical: true,
		RepeatEvery:    1,
		Levels: []EscalationLevel{
			{
				After:          -1,
				Notify:         "WEBHOOKS",
				DestinationIDs: []string{" webhook:pager ", "email", "webhook:pager", "https://secret.example"},
			},
			{After: 181, Notify: "email"},
		},
	}

	m.UpdateConfig(cfg)

	got := m.GetConfig().Schedule.Escalation
	if got.RepeatEvery != DefaultEscalationRepeatMinutes {
		t.Fatalf("repeat interval = %d, want %d", got.RepeatEvery, DefaultEscalationRepeatMinutes)
	}
	if got.Levels[0].Notify != "webhook" {
		t.Fatalf("legacy target = %q, want webhook", got.Levels[0].Notify)
	}
	if got.Levels[0].After != MinEscalationDelayMinutes {
		t.Fatalf("minimum escalation delay = %d, want %d", got.Levels[0].After, MinEscalationDelayMinutes)
	}
	if got.Levels[1].After != MaxEscalationDelayMinutes {
		t.Fatalf("maximum escalation delay = %d, want %d", got.Levels[1].After, MaxEscalationDelayMinutes)
	}
	wantIDs := []string{"webhook:pager", "email"}
	if len(got.Levels[0].DestinationIDs) != len(wantIDs) {
		t.Fatalf("destination IDs = %v, want %v", got.Levels[0].DestinationIDs, wantIDs)
	}
	for index, want := range wantIDs {
		if got.Levels[0].DestinationIDs[index] != want {
			t.Fatalf("destination IDs = %v, want %v", got.Levels[0].DestinationIDs, wantIDs)
		}
	}
}

// The thresholds page sends no flapping, alert TTL or custom-rule keys. A save
// through ApplyConfigUpdate must leave those settings as stored, also after
// the merged config is normalized in place.
func TestUpdateConfigKeepsSettingsAThresholdsSaveDidNotSend(t *testing.T) {
	m := newTestManager(t)
	stored := m.GetConfig()
	stored.FlappingEnabled = true
	stored.FlappingWindowSeconds = 600
	stored.MaxAlertAgeDays = 7
	stored.MaxAcknowledgedAgeDays = 1
	stored.AutoAcknowledgeAfterHours = 24
	stored.CustomRules = []CustomAlertRule{{ID: "rule-1", Name: "Busy web", Enabled: true}}
	m.UpdateConfig(stored)

	snapshot, err := m.ApplyConfigUpdate([]byte(`{
		"enabled": true,
		"nodeDefaults": {"cpu": {"trigger": 82, "clear": 77}},
		"overrides": {}
	}`))
	if err != nil {
		t.Fatalf("ApplyConfigUpdate: %v", err)
	}
	snapshot.TimeThresholds["guest"] = 999
	if m.GetConfig().TimeThresholds["guest"] == 999 {
		t.Fatal("the returned snapshot shares its maps with the live config")
	}

	got := m.GetConfig()
	if got.NodeDefaults.CPU == nil || got.NodeDefaults.CPU.Trigger != 82 {
		t.Fatalf("node CPU default = %+v, want the saved trigger 82", got.NodeDefaults.CPU)
	}
	if !got.FlappingEnabled || got.FlappingWindowSeconds != 600 {
		t.Fatalf("flapping = %v/%d after a thresholds save, want true/600", got.FlappingEnabled, got.FlappingWindowSeconds)
	}
	if got.MaxAlertAgeDays != 7 || got.MaxAcknowledgedAgeDays != 1 || got.AutoAcknowledgeAfterHours != 24 {
		t.Fatalf("TTL cleanup = %d/%d/%d after a thresholds save, want 7/1/24",
			got.MaxAlertAgeDays, got.MaxAcknowledgedAgeDays, got.AutoAcknowledgeAfterHours)
	}
	if len(got.CustomRules) != 1 || got.CustomRules[0].ID != "rule-1" {
		t.Fatalf("custom rules = %+v after a thresholds save, want the stored rule", got.CustomRules)
	}
}

// Each partial update reads, merges and applies under the manager lock, so
// two saves that touch different settings cannot revert each other.
func TestApplyConfigUpdateKeepsConcurrentPartialUpdates(t *testing.T) {
	m := newTestManager(t)

	for round := 0; round < 50; round++ {
		base := m.GetConfig()
		base.FlappingEnabled = false
		base.MaxAlertAgeDays = 0
		m.UpdateConfig(base)

		bodies := []string{`{"flappingEnabled": true}`, `{"maxAlertAgeDays": 9}`}
		done := make(chan struct{}, len(bodies))
		for _, body := range bodies {
			go func(body string) {
				defer func() { done <- struct{}{} }()
				if _, err := m.ApplyConfigUpdate([]byte(body)); err != nil {
					t.Errorf("ApplyConfigUpdate(%s): %v", body, err)
				}
			}(body)
		}
		for range bodies {
			<-done
		}

		got := m.GetConfig()
		if !got.FlappingEnabled || got.MaxAlertAgeDays != 9 {
			t.Fatalf("round %d: flapping=%v maxAlertAgeDays=%d, want both concurrent updates kept",
				round, got.FlappingEnabled, got.MaxAlertAgeDays)
		}
	}
}

func inputOwnershipConfig(m *Manager) AlertConfig {
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
	cfg.TimeThresholds = map[string]int{"guest": 0}
	cfg.Overrides = map[string]ThresholdConfig{"ownership-guest": {CPU: &HysteresisThreshold{Trigger: 80, Clear: 75}}}
	cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"cpu": 0}}
	cfg.MetricEvaluationWindows = map[string]map[string]int{"guest": {"cpu": 0}}
	cfg.IgnoredGuestPrefixes = []string{"ignored-"}
	cfg.Schedule.QuietHours.Days = map[string]bool{"monday": true}
	cfg.Schedule.Escalation.Levels = []EscalationLevel{{After: 10, Notify: "all", DestinationIDs: []string{"webhook:original"}}}
	return cfg
}

func TestUpdateConfigDoesNotNormalizeCallerInput(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	cfg := inputOwnershipConfig(m)
	// Invalid hysteresis and delays must still be normalised in the manager,
	// without rewriting the caller's pointers or maps.
	cfg.GuestDefaults.CPU.Clear = 90
	cfg.TimeThresholds["guest"] = -1
	before := cfg.Clone()
	m.UpdateConfig(cfg)
	if !reflect.DeepEqual(cfg, before) {
		t.Error("applying a config rewrote caller-owned configuration")
	}
	live := m.GetConfig()
	if live.GuestDefaults.CPU.Clear != 75 || live.Overrides["ownership-guest"].CPU.Clear != 75 || live.TimeThresholds["guest"] < 0 {
		t.Fatalf("owned config did not retain normalisation: guest=%+v override=%+v delay=%d", live.GuestDefaults.CPU, live.Overrides["ownership-guest"].CPU, live.TimeThresholds["guest"])
	}
}

func TestUpdateConfigCallerEditsDoNotChangeAppliedPolicy(t *testing.T) {
	cases := map[string]func(*AlertConfig){
		"threshold-pointer": func(c *AlertConfig) { c.GuestDefaults.CPU.Trigger = 0 },
		"override-map":      func(c *AlertConfig) { c.Overrides["ownership-guest"] = ThresholdConfig{Disabled: true} },
		"override-pointer":  func(c *AlertConfig) { c.Overrides["ownership-guest"].CPU.Trigger = 0 },
		"delay-map":         func(c *AlertConfig) { c.TimeThresholds["guest"] = 3600 },
		"nested-delay-map":  func(c *AlertConfig) { c.MetricTimeThresholds["guest"]["cpu"] = 3600 },
		"window-map":        func(c *AlertConfig) { c.MetricEvaluationWindows["guest"]["cpu"] = 3600 },
		"prefix-slice":      func(c *AlertConfig) { c.IgnoredGuestPrefixes[0] = "ownership-" },
		"schedule-map":      func(c *AlertConfig) { c.Schedule.QuietHours.Days["monday"] = false },
		"destination-slice": func(c *AlertConfig) { c.Schedule.Escalation.Levels[0].DestinationIDs[0] = "webhook:changed" },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewManagerWithDataDir(t.TempDir())
			t.Cleanup(m.Stop)
			cfg := inputOwnershipConfig(m)
			m.UpdateConfig(cfg)
			applied := m.GetConfig()
			edit(&cfg)
			if got := m.GetConfig(); !reflect.DeepEqual(got, applied) {
				t.Error("editing the submitted config changed live policy without an update")
			}
			guest := models.VM{ID: "ownership-guest", Name: "ownership-guest", Status: "running", CPU: .95}
			m.CheckGuest(guest, "ownership")
			found := false
			for _, alert := range m.GetActiveAlerts() {
				if alert.ResourceID == guest.ID && alert.Type == "cpu" {
					found = true
				}
			}
			if !found {
				t.Error("submitted-config edit prevented the ordinary CPU alert")
			}
			// An intentional resubmission still applies the changed setting.
			m.UpdateConfig(cfg)
			expected := m.GetConfig()
			if reflect.DeepEqual(expected, applied) {
				t.Error("explicit resubmission did not apply the edit")
			}
		})
	}
}

func TestUpdateConfigInputCanBeEncodedDuringLaterUpdates(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	cfg := inputOwnershipConfig(m)
	m.UpdateConfig(cfg)
	before := cfg.Clone()
	// A caller may retain an applied snapshot for a response or persistence.
	// Later policy reconciliation must not write its override map while it is
	// being encoded. Exercise this under -race, using actual manager updates.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if _, err := json.Marshal(cfg); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			// An alias of an already applied config is legal sequential input. The
			// manager owns normalisation; neither call transfers caller ownership.
			m.UpdateConfig(cfg)
			m.CheckGuest(models.VM{ID: "ownership-guest", Status: "running", CPU: .2, LastSeen: time.Now()}, "ownership")
		}
	}()
	wg.Wait()
	if !reflect.DeepEqual(cfg, before) {
		t.Error("later updates modified retained input")
	}
}
