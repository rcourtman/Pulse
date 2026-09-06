// Package metrics preserves access to legacy incident recording archives.
package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IncidentWindow represents a saved legacy recording window. Status and sample timestamps are historical
type IncidentWindow struct {
	ID           string               `json:"id"`
	ResourceID   string               `json:"resource_id"`
	ResourceName string               `json:"resource_name,omitempty"`
	ResourceType string               `json:"resource_type,omitempty"`
	TriggerType  string               `json:"trigger_type"` // "alert", "anomaly", "focus", "manual"
	TriggerID    string               `json:"trigger_id,omitempty"`
	StartTime    time.Time            `json:"start_time"`
	EndTime      *time.Time           `json:"end_time,omitempty"`
	Status       IncidentWindowStatus `json:"status"`
	DataPoints   []IncidentDataPoint  `json:"data_points"`
	Summary      *IncidentSummary     `json:"summary,omitempty"`
}

// IncidentWindowStatus represents the status of an incident window
type IncidentWindowStatus string

const (
	IncidentWindowStatusRecording IncidentWindowStatus = "recording"
	IncidentWindowStatusComplete  IncidentWindowStatus = "complete"
	IncidentWindowStatusTruncated IncidentWindowStatus = "truncated" // Stopped due to limits

	maxIncidentWindowsFileSize = 16 << 20 // 16 MiB
)

var errUnsafeIncidentArchivePath = errors.New("unsafe incident archive path")

// IncidentDataPoint represents a single data point in an incident window
type IncidentDataPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]float64     `json:"metrics"` // cpu, memory, disk, etc.
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// IncidentSummary provides computed statistics about an incident window
type IncidentSummary struct {
	Duration   time.Duration      `json:"duration_ms"`
	DataPoints int                `json:"data_points"`
	Peaks      map[string]float64 `json:"peaks"`               // Maximum values
	Lows       map[string]float64 `json:"lows"`                // Minimum values
	Averages   map[string]float64 `json:"averages"`            // Average values
	Changes    map[string]float64 `json:"changes"`             // Change from start to end
	Anomalies  []string           `json:"anomalies,omitempty"` // Detected anomalies
}

// IncidentArchive reads saved recordings on explicit request. It never starts
// collectors or rewrites, expires, creates or changes permissions on archives.
type IncidentArchive struct{ filePath string }

var ErrIncidentArchiveUnavailable = errors.New("legacy incident recording archive is unavailable")

func NewIncidentArchive(dataDir string) *IncidentArchive {
	if strings.TrimSpace(dataDir) == "" {
		return &IncidentArchive{}
	}
	return &IncidentArchive{filePath: filepath.Join(dataDir, "incident_windows.json")}
}

// GetWindow requires the exact resource and window identifiers in this org's
// archive. Historical names and aliases cannot authorize an archive lookup.
func (a *IncidentArchive) GetWindow(resourceID, windowID string) (*IncidentWindow, error) {
	if a == nil || a.filePath == "" {
		return nil, ErrIncidentArchiveUnavailable
	}
	if strings.TrimSpace(resourceID) == "" || strings.TrimSpace(windowID) == "" {
		return nil, errors.New("resource_id and window_id are required")
	}
	data, err := readBoundedRegularFile(a.filePath, maxIncidentWindowsFileSize)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrIncidentArchiveUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("read legacy incident archive: %w", err)
	}
	var saved struct {
		CompletedWindows json.RawMessage `json:"completed_windows"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("decode legacy incident archive: %w", err)
	}
	if len(saved.CompletedWindows) == 0 {
		return nil, errors.New("legacy incident archive has no completed_windows field")
	}
	var windows []*IncidentWindow
	if err := json.Unmarshal(saved.CompletedWindows, &windows); err != nil {
		return nil, fmt.Errorf("decode legacy incident windows: %w", err)
	}
	var match *IncidentWindow
	for _, window := range windows {
		if window == nil || window.ID != windowID || window.ResourceID != resourceID {
			continue
		}
		if match != nil {
			return nil, errors.New("legacy incident archive contains duplicate resource/window identifiers")
		}
		match = window
	}
	return match, nil
}

func validateRegularFilePath(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing symlink path %q", errUnsafeIncidentArchivePath, path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: non-regular path %q", errUnsafeIncidentArchivePath, path)
	}
	return nil
}

func readBoundedRegularFile(path string, maxSize int64) ([]byte, error) {
	initialInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := validateRegularFilePath(path, initialInfo); err != nil {
		return nil, err
	}
	if maxSize > 0 && initialInfo.Size() > maxSize {
		return nil, fmt.Errorf("%w: file %q exceeds size limit (%d bytes)", errUnsafeIncidentArchivePath, path, initialInfo.Size())
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	openInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := validateRegularFilePath(path, openInfo); err != nil {
		return nil, err
	}
	if !os.SameFile(initialInfo, openInfo) {
		return nil, fmt.Errorf("%w: file %q changed during read", errUnsafeIncidentArchivePath, path)
	}

	reader := io.Reader(file)
	if maxSize > 0 {
		reader = io.LimitReader(file, maxSize+1)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if maxSize > 0 && int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%w: file %q exceeded size limit while reading", errUnsafeIncidentArchivePath, path)
	}
	return data, nil
}
