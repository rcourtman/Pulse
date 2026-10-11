package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Docker and PMG count thresholds are plain ints where 0 is Off. An
// alerts.json written before these sections existed carries neither key, and
// must load with factory values rather than everything Off.
func TestLoadAlertConfigOlderFileTakesFactoryDockerAndPMGCounts(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "alerts.json"), []byte(`{"enabled": true}`), 0600))

	cfg, err := cp.LoadAlertConfig()
	require.NoError(t, err)

	assert.Equal(t, 3, cfg.DockerDefaults.RestartCount)
	assert.Equal(t, 300, cfg.DockerDefaults.RestartWindow)
	assert.Equal(t, 90, cfg.DockerDefaults.MemoryWarnPct)
	assert.Equal(t, 95, cfg.DockerDefaults.MemoryCriticalPct)
	assert.Equal(t, 500, cfg.PMGDefaults.QueueTotalWarning)
	assert.Equal(t, 500, cfg.PMGDefaults.QuarantineGrowthCritMin)
}

// An import hands SaveAlertConfig a config decoded from the export file
// without the manager's normalization. The unset marker for keys that file
// omits must not reach disk, where it would read back as Off.
func TestSaveAlertConfigResolvesUnsetDockerAndPMGCounts(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	var imported alerts.AlertConfig
	require.NoError(t, json.Unmarshal([]byte(`{"enabled": true, "pmgDefaults": {"queueTotalWarning": 0}}`), &imported))
	require.NoError(t, cp.SaveAlertConfig(imported))

	data, err := os.ReadFile(filepath.Join(tempDir, "alerts.json"))
	require.NoError(t, err)
	var saved struct {
		DockerDefaults map[string]any `json:"dockerDefaults"`
		PMGDefaults    map[string]any `json:"pmgDefaults"`
	}
	require.NoError(t, json.Unmarshal(data, &saved))

	assert.EqualValues(t, 3, saved.DockerDefaults["restartCount"])
	assert.EqualValues(t, 0, saved.PMGDefaults["queueTotalWarning"], "the import's explicit Off")
	assert.EqualValues(t, 1000, saved.PMGDefaults["queueTotalCritical"], "a key the import omits")

	reloaded, err := cp.LoadAlertConfig()
	require.NoError(t, err)
	assert.Equal(t, 0, reloaded.PMGDefaults.QueueTotalWarning)
	assert.Equal(t, 1000, reloaded.PMGDefaults.QueueTotalCritical)
}
