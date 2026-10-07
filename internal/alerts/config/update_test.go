package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func storedAlertConfigForUpdate() AlertConfig {
	activated := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	note := "keep"
	return AlertConfig{
		IdentitySchemaVersion:     2,
		Enabled:                   true,
		ActivationState:           ActivationActive,
		ActivationTime:            &activated,
		ObservationWindowHours:    24,
		GuestDefaults:             ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 75}},
		StorageDefault:            HysteresisThreshold{Trigger: 85, Clear: 80},
		DiskTempByType:            map[string]HysteresisThreshold{"nvme": {Trigger: 70, Clear: 65}},
		IgnoredGuestPrefixes:      []string{"test-"},
		Overrides:                 map[string]ThresholdConfig{"node:pve1": {Disabled: true, Note: &note}},
		CustomRules:               []CustomAlertRule{{ID: "rule-1", Name: "Busy web", Enabled: true}},
		Schedule:                  ScheduleConfig{Cooldown: 5, MaxAlertsHour: 10, InitialNotify: "all"},
		MinimumDelta:              3,
		SuppressionWindow:         10,
		HysteresisMargin:          4,
		TimeThresholds:            map[string]int{"guest": 5},
		MaxAlertAgeDays:           7,
		MaxAcknowledgedAgeDays:    1,
		AutoAcknowledgeAfterHours: 24,
		FlappingEnabled:           true,
		FlappingWindowSeconds:     600,
		FlappingThreshold:         4,
		FlappingCooldownMinutes:   20,
	}
}

func TestApplyAlertConfigUpdateKeepsSettingsTheClientDidNotSend(t *testing.T) {
	stored := storedAlertConfigForUpdate()

	// The settings page sends thresholds and schedule but has no control for
	// flapping detection, alert TTL cleanup or custom rules.
	got, err := ApplyAlertConfigUpdate(stored, []byte(`{
		"enabled": true,
		"guestDefaults": {"cpu": {"trigger": 90, "clear": 85}},
		"overrides": {},
		"flapping": {"enabled": true, "threshold": 5}
	}`))
	if err != nil {
		t.Fatalf("ApplyAlertConfigUpdate: %v", err)
	}

	if got.GuestDefaults.CPU == nil || got.GuestDefaults.CPU.Trigger != 90 {
		t.Fatalf("guest CPU default = %+v, want the sent trigger 90", got.GuestDefaults.CPU)
	}
	if len(got.Overrides) != 0 {
		t.Fatalf("overrides = %v, want the sent empty map to replace the stored overrides", got.Overrides)
	}
	if !got.FlappingEnabled || got.FlappingWindowSeconds != 600 || got.FlappingThreshold != 4 || got.FlappingCooldownMinutes != 20 {
		t.Fatalf("flapping = %v/%d/%d/%d, want the stored true/600/4/20",
			got.FlappingEnabled, got.FlappingWindowSeconds, got.FlappingThreshold, got.FlappingCooldownMinutes)
	}
	if got.MaxAlertAgeDays != 7 || got.MaxAcknowledgedAgeDays != 1 || got.AutoAcknowledgeAfterHours != 24 {
		t.Fatalf("TTL cleanup = %d/%d/%d, want the stored 7/1/24",
			got.MaxAlertAgeDays, got.MaxAcknowledgedAgeDays, got.AutoAcknowledgeAfterHours)
	}
	if !reflect.DeepEqual(got.CustomRules, stored.CustomRules) {
		t.Fatalf("custom rules = %+v, want the stored rules", got.CustomRules)
	}
	if got.MinimumDelta != 3 || got.SuppressionWindow != 10 || got.HysteresisMargin != 4 {
		t.Fatalf("general settings = %v/%d/%v, want the stored 3/10/4",
			got.MinimumDelta, got.SuppressionWindow, got.HysteresisMargin)
	}
}

func TestApplyAlertConfigUpdateEmptyObjectChangesNothing(t *testing.T) {
	stored := storedAlertConfigForUpdate()

	got, err := ApplyAlertConfigUpdate(stored, []byte(`{}`))
	if err != nil {
		t.Fatalf("ApplyAlertConfigUpdate({}): %v", err)
	}
	if !reflect.DeepEqual(got, stored) {
		t.Fatalf("ApplyAlertConfigUpdate({}) changed the config:\n got %+v\nwant %+v", got, stored)
	}
}

func TestApplyAlertConfigUpdateHonorsExplicitOffAndClear(t *testing.T) {
	stored := storedAlertConfigForUpdate()

	got, err := ApplyAlertConfigUpdate(stored, []byte(`{
		"flappingEnabled": false,
		"maxAlertAgeDays": 0,
		"maxAcknowledgedAgeDays": 0,
		"autoAcknowledgeAfterHours": 0,
		"customRules": null,
		"ignoredGuestPrefixes": []
	}`))
	if err != nil {
		t.Fatalf("ApplyAlertConfigUpdate: %v", err)
	}

	if got.FlappingEnabled || got.MaxAlertAgeDays != 0 || got.MaxAcknowledgedAgeDays != 0 || got.AutoAcknowledgeAfterHours != 0 {
		t.Fatalf("explicit off was not applied: flapping=%v ttl=%d/%d/%d",
			got.FlappingEnabled, got.MaxAlertAgeDays, got.MaxAcknowledgedAgeDays, got.AutoAcknowledgeAfterHours)
	}
	if got.CustomRules != nil {
		t.Fatalf("custom rules = %+v, want null to clear them", got.CustomRules)
	}
	if len(got.IgnoredGuestPrefixes) != 0 {
		t.Fatalf("ignored guest prefixes = %v, want the sent empty list", got.IgnoredGuestPrefixes)
	}
	if !got.Enabled || got.ActivationState != ActivationActive {
		t.Fatalf("unsent enabled/activation = %v/%q, want the stored true/active", got.Enabled, got.ActivationState)
	}
}

// The merge is top-level only: a sent object replaces the stored one whole,
// so a sent truenasDiskDefaults without a temperature must not inherit the
// stored temperature.
func TestApplyAlertConfigUpdateReplacesSentObjectsWhole(t *testing.T) {
	stored := storedAlertConfigForUpdate()
	stored.TrueNASDiskDefaults = ThresholdConfig{Temperature: &HysteresisThreshold{Trigger: 62, Clear: 57}}

	got, err := ApplyAlertConfigUpdate(stored, []byte(`{"truenasDiskDefaults": {}}`))
	if err != nil {
		t.Fatalf("ApplyAlertConfigUpdate: %v", err)
	}
	if got.TrueNASDiskDefaults.Temperature != nil {
		t.Fatalf("TrueNAS disk temperature = %+v, want the sent empty object to clear it", got.TrueNASDiskDefaults.Temperature)
	}
}

// encoding/json matches keys to fields case-insensitively, so a key in another
// casing still names the field and must not leave the stored value in place.
func TestApplyAlertConfigUpdateMatchesKeysLikeTheDecoder(t *testing.T) {
	stored := storedAlertConfigForUpdate()

	got, err := ApplyAlertConfigUpdate(stored, []byte(`{"FlappingEnabled": false}`))
	if err != nil {
		t.Fatalf("ApplyAlertConfigUpdate: %v", err)
	}
	if got.FlappingEnabled {
		t.Fatal("FlappingEnabled sent as false stayed true")
	}
}

// Sent keys mean what decoding the body alone makes of them, duplicates and
// all.
func TestApplyAlertConfigUpdateDecodesSentKeysLikeTheBodyAlone(t *testing.T) {
	stored := storedAlertConfigForUpdate()

	for _, test := range []struct {
		body  string
		field func(AlertConfig) any
	}{
		{`{"enabled": false, "enabled": null}`, func(c AlertConfig) any { return c.Enabled }},
		{`{"flappingEnabled": false, "FlappingEnabled": true}`, func(c AlertConfig) any { return c.FlappingEnabled }},
		{
			`{"guestDefaults": {"cpu": {"trigger": 90, "clear": 85}}, "guestDefaults": {"memory": {"trigger": 70, "clear": 65}}}`,
			func(c AlertConfig) any { return c.GuestDefaults },
		},
	} {
		var alone AlertConfig
		if err := json.Unmarshal([]byte(test.body), &alone); err != nil {
			t.Fatalf("decode %s alone: %v", test.body, err)
		}
		got, err := ApplyAlertConfigUpdate(stored, []byte(test.body))
		if err != nil {
			t.Fatalf("ApplyAlertConfigUpdate(%s): %v", test.body, err)
		}
		if !reflect.DeepEqual(test.field(got), test.field(alone)) {
			t.Fatalf("ApplyAlertConfigUpdate(%s) = %+v, decoding the body alone gives %+v",
				test.body, test.field(got), test.field(alone))
		}
	}

	if _, err := ApplyAlertConfigUpdate(stored, []byte(`{"enabled": "yes", "enabled": true}`)); err == nil {
		t.Fatal("a type error in a duplicated key succeeded, want the decoder's error")
	}
}

// The merge copies unsent settings field by field under their JSON keys, so
// an embedded, unexported or untagged field would be skipped or matched
// under the wrong key.
func TestAlertConfigFieldsHaveExplicitJSONKeys(t *testing.T) {
	fields := reflect.TypeOf(AlertConfig{})
	keys := make([]string, fields.NumField())
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		keys[i], _, _ = strings.Cut(field.Tag.Get("json"), ",")
		if field.Anonymous || !field.IsExported() || keys[i] == "" || keys[i] == "-" {
			t.Errorf("AlertConfig.%s needs to be an exported, named field with an explicit json key", field.Name)
		}
		// Keys that fold equal would both count as sent for either key.
		for j := 0; j < i; j++ {
			if strings.EqualFold(keys[i], keys[j]) {
				t.Errorf("AlertConfig.%s and AlertConfig.%s have json keys that fold equal",
					fields.Field(j).Name, field.Name)
			}
		}
	}
}

func TestApplyAlertConfigUpdateSharesNoStateWithStored(t *testing.T) {
	stored := storedAlertConfigForUpdate()

	got, err := ApplyAlertConfigUpdate(stored, []byte(`{"enabled": true}`))
	if err != nil {
		t.Fatalf("ApplyAlertConfigUpdate: %v", err)
	}
	got.TimeThresholds["guest"] = 99
	got.DiskTempByType["nvme"] = HysteresisThreshold{Trigger: 1, Clear: 0}
	got.IgnoredGuestPrefixes[0] = "changed-"
	*got.ActivationTime = time.Time{}

	if stored.TimeThresholds["guest"] != 5 || stored.DiskTempByType["nvme"].Trigger != 70 ||
		stored.IgnoredGuestPrefixes[0] != "test-" || stored.ActivationTime.IsZero() {
		t.Fatal("editing the merged config changed the stored config it was merged from")
	}
}

func TestApplyAlertConfigUpdateRejectsMalformedBodies(t *testing.T) {
	stored := storedAlertConfigForUpdate()

	for _, body := range []string{`null`, `[]`, `"config"`, `{"flappingEnabled": "yes"}`, `{`} {
		if _, err := ApplyAlertConfigUpdate(stored, []byte(body)); err == nil {
			t.Fatalf("ApplyAlertConfigUpdate(%s) succeeded, want a decode error", body)
		}
	}
}

// A config marshaled by a Go client still replaces every field it writes.
// Empty omitempty fields are left out of that body, so they keep their
// stored values like any other unsent key.
func TestApplyAlertConfigUpdateFullBodyReplacesWrittenFields(t *testing.T) {
	stored := storedAlertConfigForUpdate()
	replacement := AlertConfig{Enabled: true, Overrides: map[string]ThresholdConfig{}, TimeThresholds: map[string]int{}}

	body, err := json.Marshal(replacement)
	if err != nil {
		t.Fatalf("marshal replacement: %v", err)
	}
	got, err := ApplyAlertConfigUpdate(stored, body)
	if err != nil {
		t.Fatalf("ApplyAlertConfigUpdate: %v", err)
	}
	if got.FlappingEnabled || got.MaxAlertAgeDays != 0 || got.MinimumDelta != 0 || len(got.Overrides) != 0 {
		t.Fatalf("full body left written fields at their stored values: %+v", got)
	}
	if !reflect.DeepEqual(got.CustomRules, stored.CustomRules) {
		t.Fatalf("custom rules = %+v, want the stored rules for the omitted key", got.CustomRules)
	}
}
