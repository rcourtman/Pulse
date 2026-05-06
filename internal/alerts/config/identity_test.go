package config_test

import (
	"reflect"
	"testing"

	alerts "github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

// TestTypeAliasIdentity verifies that alerts.AlertConfig and config.AlertConfig
// are the same underlying type via the type alias in config_facade.go.
func TestTypeAliasIdentity(t *testing.T) {
	if reflect.TypeOf(alertconfig.AlertConfig{}) != reflect.TypeOf(alerts.AlertConfig{}) {
		t.Error("config.AlertConfig and alerts.AlertConfig must be the same type (type alias)")
	}

	if reflect.TypeOf(alertconfig.ThresholdConfig{}) != reflect.TypeOf(alerts.ThresholdConfig{}) {
		t.Error("config.ThresholdConfig and alerts.ThresholdConfig must be the same type")
	}

	if reflect.TypeOf(alertconfig.AlertLevelWarning) != reflect.TypeOf(alerts.AlertLevelWarning) {
		t.Error("config.AlertLevelWarning and alerts.AlertLevelWarning must be the same type")
	}
}
