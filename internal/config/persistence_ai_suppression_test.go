package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSaveAIFindings_PreservesSuppressionRulesWhenNotProvided(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	findings := map[string]*AIFindingRecord{
		"f1": {
			ID:          "f1",
			Severity:    "warning",
			Category:    "performance",
			ResourceID:  "res-1",
			Title:       "High CPU",
			Description: "CPU high",
			DetectedAt:  time.Now(),
			LastSeenAt:  time.Now(),
		},
	}
	rules := map[string]*AISuppressionRuleRecord{
		"rule1": {
			ID:          "rule1",
			ResourceID:  "res-1",
			Category:    "performance",
			Description: "Ignore for now",
			CreatedAt:   time.Now(),
			CreatedFrom: "manual",
		},
	}

	require.NoError(t, cp.SaveAIFindingsWithSuppression(findings, rules))

	// Now save findings via the legacy method; suppression rules should be preserved.
	require.NoError(t, cp.SaveAIFindings(findings))

	loaded, err := cp.LoadAIFindings()
	require.NoError(t, err)
	require.NotNil(t, loaded.SuppressionRules)
	require.Contains(t, loaded.SuppressionRules, "rule1")
	require.Equal(t, "Ignore for now", loaded.SuppressionRules["rule1"].Description)
}
