package config

import "testing"

// TestNormalizeTrueNASDiskTemperatureKeepsOnlyChosenValues pins the saved
// config migration that lets TrueNAS disks follow the disk temperature policy.
// In a config written before it, the old flat factory default was written
// back on every save, so it is not a choice; anything else the user saved is
// kept.
func TestNormalizeTrueNASDiskTemperatureKeepsOnlyChosenValues(t *testing.T) {
	tests := []struct {
		name   string
		stored *HysteresisThreshold
		want   *HysteresisThreshold
	}{
		{name: "unset stays unset", stored: nil, want: nil},
		{name: "old factory default is migrated to unset", stored: &HysteresisThreshold{Trigger: 55, Clear: 50}, want: nil},
		{name: "old factory trigger without a clear is migrated to unset", stored: &HysteresisThreshold{Trigger: 55}, want: nil},
		{name: "negative trigger is unset", stored: &HysteresisThreshold{Trigger: -1}, want: nil},
		{name: "off is kept", stored: &HysteresisThreshold{Trigger: 0, Clear: 10}, want: &HysteresisThreshold{Trigger: 0, Clear: 0}},
		{name: "custom value is kept", stored: &HysteresisThreshold{Trigger: 62, Clear: 58}, want: &HysteresisThreshold{Trigger: 62, Clear: 58}},
		{name: "custom value gets a clear", stored: &HysteresisThreshold{Trigger: 62}, want: &HysteresisThreshold{Trigger: 62, Clear: 57}},
		{name: "55 with a custom clear is a choice", stored: &HysteresisThreshold{Trigger: 55, Clear: 45}, want: &HysteresisThreshold{Trigger: 55, Clear: 45}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AlertConfig{TrueNASDiskDefaults: ThresholdConfig{Temperature: tt.stored}}
			NormalizeTrueNASDefaults(cfg)
			if !cfg.TrueNASDiskTemperatureByType {
				t.Fatalf("normalized config is not marked as following disk type")
			}
			got := cfg.TrueNASDiskDefaults.Temperature
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("Temperature = %+v, want unset", *got)
			case tt.want != nil && got == nil:
				t.Fatalf("Temperature unset, want %+v", *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Fatalf("Temperature = %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

// Once a config follows disk type, 55 is a value the user typed, and a save
// or restart must keep it.
func TestNormalizeTrueNASDiskTemperatureKeepsChosen55(t *testing.T) {
	cfg := &AlertConfig{
		TrueNASDiskDefaults:          ThresholdConfig{Temperature: &HysteresisThreshold{Trigger: 55, Clear: 50}},
		TrueNASDiskTemperatureByType: true,
	}
	NormalizeTrueNASDefaults(cfg)
	NormalizeTrueNASDefaults(cfg)
	got := cfg.TrueNASDiskDefaults.Temperature
	if got == nil || *got != (HysteresisThreshold{Trigger: 55, Clear: 50}) {
		t.Fatalf("Temperature = %+v, want the chosen 55/50 kept", got)
	}
}
