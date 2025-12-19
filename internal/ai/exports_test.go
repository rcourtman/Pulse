package ai

import (
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	// Verify some default values
	if cfg.Provider != "" && cfg.Provider != ProviderAnthropic && cfg.Provider != ProviderOpenAI && cfg.Provider != ProviderOllama && cfg.Provider != ProviderDeepSeek {
		t.Errorf("Unexpected provider: %s", cfg.Provider)
	}
}

func TestDefaultBaselineConfig(t *testing.T) {
	cfg := DefaultBaselineConfig()

	// Verify config has reasonable defaults
	if cfg.LearningWindow <= 0 {
		t.Error("Expected positive LearningWindow")
	}
	if cfg.MinSamples <= 0 {
		t.Error("Expected positive MinSamples")
	}
}

func TestNewBaselineStore(t *testing.T) {
	cfg := DefaultBaselineConfig()
	store := NewBaselineStore(cfg)

	if store == nil {
		t.Fatal("Expected non-nil baseline store")
	}
}

func TestDefaultPatternConfig(t *testing.T) {
	cfg := DefaultPatternConfig()

	// Verify config exists (the actual struct may vary)
	// Just check it doesn't panic
	_ = cfg
}

func TestNewPatternDetector(t *testing.T) {
	cfg := DefaultPatternConfig()
	detector := NewPatternDetector(cfg)

	if detector == nil {
		t.Fatal("Expected non-nil pattern detector")
	}
}

func TestDefaultCorrelationConfig(t *testing.T) {
	cfg := DefaultCorrelationConfig()

	// Verify config exists
	_ = cfg
}

func TestNewCorrelationDetector(t *testing.T) {
	cfg := DefaultCorrelationConfig()
	detector := NewCorrelationDetector(cfg)

	if detector == nil {
		t.Fatal("Expected non-nil correlation detector")
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
		{10, 10, 10},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestNewChangeDetector(t *testing.T) {
	cfg := ChangeDetectorConfig{
		MaxChanges: 100,
	}
	detector := NewChangeDetector(cfg)

	if detector == nil {
		t.Fatal("Expected non-nil change detector")
	}
}

func TestNewRemediationLog(t *testing.T) {
	cfg := RemediationLogConfig{
		MaxRecords: 50,
	}
	log := NewRemediationLog(cfg)

	if log == nil {
		t.Fatal("Expected non-nil remediation log")
	}
}
