package alerts

func defaultAlertConfig() AlertConfig {
	alertOrphaned := true
	return AlertConfig{
		Enabled:                true,
		ActivationState:        ActivationPending,
		ObservationWindowHours: 24,
		GuestDefaults: ThresholdConfig{
			PoweredOffSeverity: AlertLevelWarning,
			CPU:                &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:             &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:               &HysteresisThreshold{Trigger: 90, Clear: 85},
			DiskRead:           &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
			DiskWrite:          &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
			NetworkIn:          &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
			NetworkOut:         &HysteresisThreshold{Trigger: 0, Clear: 0}, // Off by default
		},
		NodeDefaults: ThresholdConfig{
			CPU:         &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:      &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:        &HysteresisThreshold{Trigger: 90, Clear: 85},
			Temperature: &HysteresisThreshold{Trigger: 80, Clear: 75}, // Warning at 80°C, clear at 75°C
		},
		AgentDefaults: ThresholdConfig{
			CPU:             &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:          &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:            &HysteresisThreshold{Trigger: 90, Clear: 85},
			DiskTemperature: &HysteresisThreshold{Trigger: 55, Clear: 50},
		},
		DockerDefaults: DockerThresholdConfig{
			CPU:                     HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:                  HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:                    HysteresisThreshold{Trigger: 85, Clear: 80},
			RestartCount:            3,
			RestartWindow:           300, // 5 minutes
			MemoryWarnPct:           90,
			MemoryCriticalPct:       95,
			StatePoweredOffSeverity: AlertLevelWarning,
		},
		PMGDefaults: PMGThresholdConfig{
			QueueTotalWarning:       500,  // Warning at 500 total queued messages
			QueueTotalCritical:      1000, // Critical at 1000 total queued messages
			OldestMessageWarnMins:   30,   // Warning if oldest message is 30+ minutes old
			OldestMessageCritMins:   60,   // Critical if oldest message is 60+ minutes old
			DeferredQueueWarn:       200,  // Warning at 200 deferred messages
			DeferredQueueCritical:   500,  // Critical at 500 deferred messages
			HoldQueueWarn:           100,  // Warning at 100 held messages
			HoldQueueCritical:       300,  // Critical at 300 held messages
			QuarantineSpamWarn:      2000, // Warning at 2000 spam quarantined
			QuarantineSpamCritical:  5000, // Critical at 5000 spam quarantined
			QuarantineVirusWarn:     2000, // Warning at 2000 virus quarantined
			QuarantineVirusCritical: 5000, // Critical at 5000 virus quarantined
			QuarantineGrowthWarnPct: 25,   // Warning if growth >=25%
			QuarantineGrowthWarnMin: 250,  // AND >=250 messages
			QuarantineGrowthCritPct: 50,   // Critical if growth >=50%
			QuarantineGrowthCritMin: 500,  // AND >=500 messages
		},
		SnapshotDefaults: SnapshotAlertConfig{
			Enabled:         false,
			WarningDays:     30,
			CriticalDays:    45,
			WarningSizeGiB:  0,
			CriticalSizeGiB: 0,
		},
		BackupDefaults: BackupAlertConfig{
			Enabled:       false,
			WarningDays:   7,
			CriticalDays:  14,
			FreshHours:    24,
			StaleHours:    72,
			AlertOrphaned: &alertOrphaned,
			IgnoreVMIDs:   []string{},
		},
		PBSDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		StorageDefault:    HysteresisThreshold{Trigger: 85, Clear: 80},
		MinimumDelta:      2.0, // 2% minimum change
		SuppressionWindow: 5,   // 5 minutes
		HysteresisMargin:  5.0, // 5% default margin
		TimeThresholds: map[string]int{
			"guest":   5,
			"node":    5,
			"agent":   5,
			"storage": 5,
			"pbs":     5,
		},
		Overrides: make(map[string]ThresholdConfig),
		Schedule: ScheduleConfig{
			QuietHours: QuietHours{
				Enabled:  false, // OFF - users should opt-in to quiet hours
				Start:    "22:00",
				End:      "08:00",
				Timezone: "America/New_York",
				Days: map[string]bool{
					"monday":    true,
					"tuesday":   true,
					"wednesday": true,
					"thursday":  true,
					"friday":    true,
					"saturday":  false,
					"sunday":    false,
				},
				Suppress: QuietHoursSuppression{},
			},
			Cooldown:        5,  // ON - 5 minutes prevents spam
			MaxAlertsHour:   10, // ON - 10 alerts/hour prevents flooding
			NotifyOnResolve: true,
			Escalation: EscalationConfig{
				Enabled: false, // OFF - requires user configuration
				Levels: []EscalationLevel{
					{After: 15, Notify: "email"},
					{After: 30, Notify: "webhook"},
					{After: 60, Notify: "all"},
				},
			},
			Grouping: GroupingConfig{
				Enabled: true,  // ON - reduces notification noise
				Window:  30,    // 30 second window for grouping
				ByNode:  true,  // Group by node for mass node issues
				ByGuest: false, // Don't group by guest by default
			},
		},
		// Alert TTL defaults
		MaxAlertAgeDays:           7,  // Auto-cleanup alerts older than 7 days
		MaxAcknowledgedAgeDays:    1,  // Auto-cleanup acknowledged alerts older than 1 day
		AutoAcknowledgeAfterHours: 24, // Auto-acknowledge alerts after 24 hours
		// Flapping detection defaults
		FlappingEnabled:         true, // Enable flapping detection
		FlappingWindowSeconds:   300,  // 5 minute window
		FlappingThreshold:       5,    // 5 state changes triggers flapping
		FlappingCooldownMinutes: 15,   // 15 minute cooldown
	}
}
