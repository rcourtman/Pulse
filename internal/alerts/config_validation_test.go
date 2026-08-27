package alerts

import "testing"

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
