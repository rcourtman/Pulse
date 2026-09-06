package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIncidentArchivePreservesHistoricalFileAndValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incident_windows.json")
	// Older than the former retention window, with data that the old adapter dropped.
	original := []byte(`{"completed_windows":[null,{"id":"old-window","resource_id":"docker:old-host/full-id","resource_type":"host","status":"recording","start_time":"2020-01-02T03:04:05Z","data_points":[{"timestamp":"2020-01-02T03:04:06Z","metrics":{"cpu":12.5},"metadata":{"source":"cached","nested":{"retained":true}}}],"summary":{"duration_ms":60000000000,"data_points":1,"anomalies":["stored observation"]}}]}`)
	require.NoError(t, os.WriteFile(path, original, 0640))
	require.NoError(t, os.Chmod(path, 0640))
	oldTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, os.Chtimes(path, oldTime, oldTime))
	before, err := os.Stat(path)
	require.NoError(t, err)
	archive := NewIncidentArchive(dir)
	window, err := archive.GetWindow("docker:old-host/full-id", "old-window")
	require.NoError(t, err)
	require.NotNil(t, window)
	require.Equal(t, IncidentWindowStatusRecording, window.Status)
	require.Equal(t, "host", window.ResourceType)
	require.Equal(t, oldTime, window.StartTime)
	require.Equal(t, time.Minute, window.Summary.Duration)
	require.Equal(t, []string{"stored observation"}, window.Summary.Anomalies)
	require.Equal(t, map[string]interface{}{"retained": true}, window.DataPoints[0].Metadata["nested"])
	// Each read decodes independently, so a caller cannot alter later evidence.
	window.DataPoints[0].Metrics["cpu"] = 99
	again, err := archive.GetWindow("docker:old-host/full-id", "old-window")
	require.NoError(t, err)
	require.Equal(t, 12.5, again.DataPoints[0].Metrics["cpu"])
	for _, key := range [][2]string{{"other-resource", "old-window"}, {"docker:old-host/full-id", "absent"}} {
		got, err := archive.GetWindow(key[0], key[1])
		require.NoError(t, err)
		require.Nil(t, got)
	}
	after, err := os.Stat(path)
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, got)
	require.Equal(t, before.Mode(), after.Mode())
	require.Equal(t, before.ModTime(), after.ModTime())
}

func TestIncidentArchiveReadsOnlyOnRequestAndReportsFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incident_windows.json")
	archive := NewIncidentArchive(dir)
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err))
	_, err = archive.GetWindow("r", "w")
	require.ErrorIs(t, err, ErrIncidentArchiveUnavailable)
	// A repaired or newly restored file is visible on the next explicit read.
	for _, raw := range []string{`{"completed_windows":[]}`, `{"completed_windows":null}`} {
		require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
		got, err := archive.GetWindow("r", "w")
		require.NoError(t, err)
		require.Nil(t, got)
	}
	for _, raw := range []string{`{`, `{}`, `null`, `{"completed_windows":{}}`, `{"completed_windows":[{"id":"w","resource_id":"r"},{"id":"w","resource_id":"r"}]}`} {
		require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
		got, err := archive.GetWindow("r", "w")
		require.Error(t, err)
		require.Nil(t, got)
	}
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0700))
	_, err = archive.GetWindow("r", "w")
	require.ErrorIs(t, err, errUnsafeIncidentArchivePath)
	require.NoError(t, os.Remove(path))
	target := filepath.Join(dir, "other.json")
	require.NoError(t, os.WriteFile(target, []byte(`{"completed_windows":[]}`), 0600))
	require.NoError(t, os.Symlink(target, path))
	_, err = archive.GetWindow("r", "w")
	require.ErrorIs(t, err, errUnsafeIncidentArchivePath)
	require.NoError(t, os.Remove(path))
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(maxIncidentWindowsFileSize+1))
	require.NoError(t, f.Close())
	_, err = archive.GetWindow("r", "w")
	require.ErrorIs(t, err, errUnsafeIncidentArchivePath)
	for _, a := range []*IncidentArchive{nil, NewIncidentArchive("")} {
		_, err = a.GetWindow("r", "w")
		require.ErrorIs(t, err, ErrIncidentArchiveUnavailable)
	}
}

func TestIncidentArchiveRequiresExactResourceAndOrg(t *testing.T) {
	aDir, bDir := t.TempDir(), t.TempDir()
	for _, org := range []struct{ dir, name string }{{aDir, "tenant-a"}, {bDir, "tenant-b"}} {
		raw, err := json.Marshal(map[string]interface{}{"completed_windows": []*IncidentWindow{{ID: "same-window", ResourceID: "same-resource", ResourceName: org.name}}})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(org.dir, "incident_windows.json"), raw, 0600))
	}
	for _, org := range []struct{ dir, name string }{{aDir, "tenant-a"}, {bDir, "tenant-b"}} {
		a := NewIncidentArchive(org.dir)
		got, err := a.GetWindow("same-resource", "same-window")
		require.NoError(t, err)
		require.Equal(t, org.name, got.ResourceName)
		for _, keys := range [][2]string{{"same-resource ", "same-window"}, {"same-resource", "same-window "}} {
			got, err = a.GetWindow(keys[0], keys[1])
			require.NoError(t, err)
			require.Nil(t, got)
		}
		_, err = a.GetWindow("", "same-window")
		require.Error(t, err)
	}
}
