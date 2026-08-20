package config

import (
	"errors"
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

// A failed read of the existing findings file must abort the save: proceeding
// would rewrite the file without the user-authored suppression rules it still
// holds. (A genuinely missing file loads as empty data with no error, so it
// never reaches this path.)
func TestSaveAIFindings_ReadErrorAbortsInsteadOfDroppingSuppressionRules(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	findings := map[string]*AIFindingRecord{
		"f1": {ID: "f1", Title: "High CPU", DetectedAt: time.Now(), LastSeenAt: time.Now()},
	}
	rules := map[string]*AISuppressionRuleRecord{
		"rule1": {ID: "rule1", ResourceID: "res-1", Description: "Ignore for now", CreatedAt: time.Now()},
	}
	require.NoError(t, cp.SaveAIFindingsWithSuppression(findings, rules))

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("transient read failure")}
	cp.SetFileSystem(mfs)

	err := cp.SaveAIFindings(findings)
	require.Error(t, err)
	require.ErrorContains(t, err, "transient read failure")

	// The aborted save must have left the file untouched: once reads work
	// again, the rules are still there.
	mfs.readError = nil
	loaded, err := cp.LoadAIFindings()
	require.NoError(t, err)
	require.Contains(t, loaded.SuppressionRules, "rule1")
}

// Explicit suppression rules need no read of the existing file, so a failing
// read must not block that save.
func TestSaveAIFindings_ExplicitRulesSaveDespiteReadError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("transient read failure")}
	cp.SetFileSystem(mfs)

	rules := map[string]*AISuppressionRuleRecord{
		"rule1": {ID: "rule1", ResourceID: "res-1", Description: "Ignore for now", CreatedAt: time.Now()},
	}
	require.NoError(t, cp.SaveAIFindingsWithSuppression(nil, rules))

	mfs.readError = nil
	loaded, err := cp.LoadAIFindings()
	require.NoError(t, err)
	require.Contains(t, loaded.SuppressionRules, "rule1")
}
