// Package metrics provides metrics collection and incident recording functionality.
package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// IncidentWindow represents a high-frequency recording window during an incident
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

	incidentRecorderDirPerm      = 0o700
	incidentRecorderFilePerm     = 0o600
	maxIncidentWindowsFileSize   = 16 << 20 // 16 MiB
	maxWindowIDResourceSegment   = 64
	unknownWindowResourceSegment = "unknown"
)

var errUnsafeIncidentPersistencePath = errors.New("unsafe incident recorder persistence path")

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

// IncidentRecorderConfig configures the incident recorder
type IncidentRecorderConfig struct {
	// Recording settings
	SampleInterval         time.Duration // How often to record data points (default: 5s)
	PreIncidentWindow      time.Duration // How much data to capture before incident (default: 5min)
	PostIncidentWindow     time.Duration // How much data to capture after incident (default: 10min)
	MaxDataPointsPerWindow int           // Maximum data points per window (default: 500)

	// Storage settings
	DataDir           string
	MaxWindows        int           // Maximum number of windows to keep (default: 100)
	RetentionDuration time.Duration // How long to keep windows (default: 24h)
}

// DefaultIncidentRecorderConfig returns sensible defaults
func DefaultIncidentRecorderConfig() IncidentRecorderConfig {
	return IncidentRecorderConfig{
		SampleInterval:         5 * time.Second,
		PreIncidentWindow:      5 * time.Minute,
		PostIncidentWindow:     10 * time.Minute,
		MaxDataPointsPerWindow: 500,
		MaxWindows:             100,
		RetentionDuration:      24 * time.Hour,
	}
}

// MetricsProvider provides current metrics for a resource
type MetricsProvider interface {
	GetCurrentMetrics(resourceID string) (map[string]float64, error)
	GetMonitoredResourceIDs() []string // Returns all resource IDs being monitored
}

// IncidentRecorder captures high-frequency metrics during incidents
type IncidentRecorder struct {
	mu sync.RWMutex

	config   IncidentRecorderConfig
	provider MetricsProvider

	// Active recordings
	activeWindows map[string]*IncidentWindow // keyed by window ID

	// Completed recordings (ring buffer)
	completedWindows []*IncidentWindow

	// Background recording for pre-incident buffer
	preIncidentBuffer map[string][]IncidentDataPoint // keyed by resource ID

	// Persistence
	dataDir  string
	filePath string

	// Control
	stopCh   chan struct{}
	loopDone chan struct{}
	running  bool

	// Async save coordination
	saveMu         sync.Mutex
	saveCond       *sync.Cond
	saveInProgress bool
	saveRequested  bool
}

// NewIncidentRecorder creates a new incident recorder
func NewIncidentRecorder(cfg IncidentRecorderConfig) *IncidentRecorder {
	if cfg.SampleInterval <= 0 {
		cfg.SampleInterval = 5 * time.Second
	}
	if cfg.PreIncidentWindow <= 0 {
		cfg.PreIncidentWindow = 5 * time.Minute
	}
	if cfg.PostIncidentWindow <= 0 {
		cfg.PostIncidentWindow = 10 * time.Minute
	}
	if cfg.MaxDataPointsPerWindow <= 0 {
		cfg.MaxDataPointsPerWindow = 500
	}
	if cfg.MaxWindows <= 0 {
		cfg.MaxWindows = 100
	}
	if cfg.RetentionDuration <= 0 {
		cfg.RetentionDuration = 24 * time.Hour
	}
	if cfg.DataDir != "" {
		trimmed := strings.TrimSpace(cfg.DataDir)
		if trimmed == "" {
			log.Warn().Msg("Ignoring incident recorder data dir: blank after trimming whitespace")
			cfg.DataDir = ""
		} else {
			cfg.DataDir = filepath.Clean(trimmed)
		}
	}

	dataDir := strings.TrimSpace(cfg.DataDir)
	if dataDir != "" {
		dataDir = filepath.Clean(dataDir)
	}

	recorder := &IncidentRecorder{
		config:            cfg,
		activeWindows:     make(map[string]*IncidentWindow),
		completedWindows:  make([]*IncidentWindow, 0),
		preIncidentBuffer: make(map[string][]IncidentDataPoint),
		dataDir:           dataDir,
		stopCh:            make(chan struct{}),
		loopDone:          make(chan struct{}),
	}
	recorder.saveCond = sync.NewCond(&recorder.saveMu)

	if dataDir != "" {
		recorder.filePath = filepath.Join(dataDir, "incident_windows.json")
		if err := recorder.loadFromDisk(); err != nil {
			log.Warn().
				Str("file_path", recorder.filePath).
				Err(err).
				Msg("Failed to load incident windows from disk")
		}
	}

	return recorder
}

// SetMetricsProvider sets the metrics provider for recording
func (r *IncidentRecorder) SetMetricsProvider(provider MetricsProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.provider = provider
}

// Start begins background recording for pre-incident buffer
func (r *IncidentRecorder) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	stopCh := make(chan struct{})
	loopDone := make(chan struct{})
	r.running = true
	stopCh := make(chan struct{})
	loopDone := make(chan struct{})
	r.stopCh = stopCh
	r.loopDone = loopDone
	r.mu.Unlock()

	go r.recordingLoop()
	log.Info().
		Dur("sample_interval", r.config.SampleInterval).
		Dur("pre_incident_window", r.config.PreIncidentWindow).
		Dur("post_incident_window", r.config.PostIncidentWindow).
		Int("max_data_points_per_window", r.config.MaxDataPointsPerWindow).
		Msg("Incident recorder started")
}

// Stop stops the incident recorder
func (r *IncidentRecorder) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	stopCh := r.stopCh
	loopDone := r.loopDone
	r.stopCh = nil
	r.loopDone = nil
	r.mu.Unlock()

	if loopDone != nil {
		<-loopDone
	}

	// Save to disk
	if err := r.saveToDisk(); err != nil {
		log.Warn().
			Str("file_path", r.filePath).
			Err(err).
			Msg("Failed to save incident windows on stop")
	}
	log.Info().Msg("incident recorder stopped")
}

// recordingLoop runs in the background to maintain pre-incident buffers and active windows
func (r *IncidentRecorder) recordingLoop(stopCh <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(r.config.SampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			r.recordSample()
		}
	}
}

// recordSample captures a data point for all active windows and buffers
func (r *IncidentRecorder) recordSample() {
	r.mu.Lock()
	if r.provider == nil {
		r.mu.Unlock()
		return
	}

	now := time.Now()
	shouldSave := false

	// Record for active windows
	for _, window := range r.activeWindows {
		if window.Status != IncidentWindowStatusRecording {
			continue
		}

		// Check if we've exceeded the post-incident window
		if window.EndTime != nil && now.After(*window.EndTime) {
			r.completeWindowLocked(window)
			shouldSave = true
			continue
		}

		// Check if we've exceeded max data points
		if len(window.DataPoints) >= r.config.MaxDataPointsPerWindow {
			window.Status = IncidentWindowStatusTruncated
			log.Warn().
				Str("window_id", window.ID).
				Str("resource_id", window.ResourceID).
				Int("max_data_points_per_window", r.config.MaxDataPointsPerWindow).
				Msg("Truncating incident window after reaching max data points")
			r.completeWindow(window)
			continue
		}

		// Get metrics
		metrics, err := r.provider.GetCurrentMetrics(window.ResourceID)
		if err != nil {
			log.Debug().
				Str("window_id", window.ID).
				Str("resource_id", window.ResourceID).
				Str("window_id", window.ID).
				Err(err).
				Msg("failed to get metrics for incident window")
			continue
		}

		window.DataPoints = append(window.DataPoints, IncidentDataPoint{
			Timestamp: now,
			Metrics:   copyMetrics(metrics),
		})
	}

	// Continuously buffer ALL monitored resources for pre-incident data
	// This ensures we have history when an alert fires on any resource
	monitoredResources := r.provider.GetMonitoredResourceIDs()
	bufferCutoff := now.Add(-r.config.PreIncidentWindow)

	for _, resourceID := range monitoredResources {
		metrics, err := r.provider.GetCurrentMetrics(resourceID)
		if err != nil {
			log.Debug().
				Str("resource_id", resourceID).
				Err(err).
				Msg("Failed to get metrics for pre-incident buffer")
			continue
		}

		// Add to pre-incident buffer
		buffer := r.preIncidentBuffer[resourceID]
		buffer = append(buffer, IncidentDataPoint{
			Timestamp: now,
			Metrics:   copyMetrics(metrics),
		})

		// Keep only last PreIncidentWindow duration
		kept := make([]IncidentDataPoint, 0, len(buffer))
		for _, dp := range buffer {
			if dp.Timestamp.After(bufferCutoff) {
				kept = append(kept, dp)
			}
		}
		r.preIncidentBuffer[resourceID] = kept
	}

	// Clean up buffers for resources no longer monitored
	monitoredSet := make(map[string]bool, len(monitoredResources))
	for _, resourceID := range monitoredResources {
		monitoredSet[resourceID] = true
	}
	for resourceID := range r.preIncidentBuffer {
		if !monitoredSet[resourceID] {
			delete(r.preIncidentBuffer, resourceID)
		}
	}
	r.mu.Unlock()

	if shouldSave {
		if err := r.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to save incident windows")
		}
	}
}

// StartRecording begins recording an incident window
func (r *IncidentRecorder) StartRecording(resourceID, resourceName, resourceType, triggerType, triggerID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we already have an active window for this resource
	for _, window := range r.activeWindows {
		if window.ResourceID == resourceID && window.Status == IncidentWindowStatusRecording {
			// Extend existing window
			endTime := time.Now().Add(r.config.PostIncidentWindow)
			window.EndTime = &endTime
			return window.ID
		}
	}

	// Create new window
	windowID := generateWindowID(resourceID)
	now := time.Now()
	endTime := now.Add(r.config.PostIncidentWindow)

	window := &IncidentWindow{
		ID:           windowID,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ResourceType: resourceType,
		TriggerType:  triggerType,
		TriggerID:    triggerID,
		StartTime:    now.Add(-r.config.PreIncidentWindow), // Include pre-incident data
		EndTime:      &endTime,
		Status:       IncidentWindowStatusRecording,
		DataPoints:   make([]IncidentDataPoint, 0),
	}

	// Copy pre-incident buffer if available
	if preBuffer, ok := r.preIncidentBuffer[resourceID]; ok {
		window.DataPoints = append(window.DataPoints, copyDataPoints(preBuffer)...)
	}

	r.activeWindows[windowID] = window

	log.Info().
		Str("window_id", windowID).
		Str("resource_id", resourceID).
		Str("trigger_type", triggerType).
		Msg("started incident recording")

	return windowID
}

// StopRecording stops recording for a specific window
func (r *IncidentRecorder) StopRecording(windowID string) {
	r.mu.Lock()
	shouldSave := false
	if window, ok := r.activeWindows[windowID]; ok {
		r.completeWindowLocked(window)
		shouldSave = true
	}
	r.mu.Unlock()

	if shouldSave {
		if err := r.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to save incident windows")
		}
	}
}

// completeWindowLocked finalizes a recording window.
// Caller must hold r.mu.
func (r *IncidentRecorder) completeWindowLocked(window *IncidentWindow) {
	if window.Status != IncidentWindowStatusRecording && window.Status != IncidentWindowStatusTruncated {
		return
	}

	now := time.Now()
	if window.Status == IncidentWindowStatusRecording {
		window.Status = IncidentWindowStatusComplete
	}
	window.EndTime = &now

	// Compute summary
	window.Summary = r.computeSummary(window)

	// Move to completed
	r.completedWindows = append(r.completedWindows, window)
	delete(r.activeWindows, window.ID)

	// Trim completed windows
	r.trimCompletedWindows()

	log.Info().
		Str("window_id", window.ID).
		Str("resource_id", window.ResourceID).
		Str("status", string(window.Status)).
		Int("data_points", len(window.DataPoints)).
		Str("status", string(window.Status)).
		Msg("completed incident recording")

	// Save asynchronously
	go func() {
		if err := r.saveToDisk(); err != nil {
			log.Warn().
				Str("window_id", window.ID).
				Str("resource_id", window.ResourceID).
				Str("file_path", r.filePath).
				Err(err).
				Msg("Failed to save incident windows")
		}
	}()
}

// computeSummary computes statistics for a window
func (r *IncidentRecorder) computeSummary(window *IncidentWindow) *IncidentSummary {
	if len(window.DataPoints) == 0 {
		return nil
	}

	summary := &IncidentSummary{
		DataPoints: len(window.DataPoints),
		Peaks:      make(map[string]float64),
		Lows:       make(map[string]float64),
		Averages:   make(map[string]float64),
		Changes:    make(map[string]float64),
	}

	// Calculate duration
	if len(window.DataPoints) > 1 {
		first := window.DataPoints[0].Timestamp
		last := window.DataPoints[len(window.DataPoints)-1].Timestamp
		summary.Duration = last.Sub(first)
	}

	// Track sums for averages
	sums := make(map[string]float64)
	counts := make(map[string]int)

	// First and last values for change calculation
	firstValues := make(map[string]float64)
	lastValues := make(map[string]float64)

	for i, dp := range window.DataPoints {
		for metric, value := range dp.Metrics {
			// Track first value
			if i == 0 {
				firstValues[metric] = value
				summary.Peaks[metric] = value
				summary.Lows[metric] = value
			}

			// Track last value
			lastValues[metric] = value

			// Track peaks and lows
			if value > summary.Peaks[metric] {
				summary.Peaks[metric] = value
			}
			if value < summary.Lows[metric] {
				summary.Lows[metric] = value
			}

			// Track sums for average
			sums[metric] += value
			counts[metric]++
		}
	}

	// Calculate averages and changes
	for metric, sum := range sums {
		if counts[metric] > 0 {
			summary.Averages[metric] = sum / float64(counts[metric])
		}
		if first, ok := firstValues[metric]; ok {
			if last, ok := lastValues[metric]; ok {
				summary.Changes[metric] = last - first
			}
		}
	}

	return summary
}

// trimCompletedWindows removes old windows
func (r *IncidentRecorder) trimCompletedWindows() {
	// Remove by retention duration
	cutoff := time.Now().Add(-r.config.RetentionDuration)
	kept := make([]*IncidentWindow, 0, len(r.completedWindows))
	for _, w := range r.completedWindows {
		if w.EndTime != nil && w.EndTime.After(cutoff) {
			kept = append(kept, w)
		}
	}
	r.completedWindows = kept

	// Remove by max windows
	if len(r.completedWindows) > r.config.MaxWindows {
		r.completedWindows = r.completedWindows[len(r.completedWindows)-r.config.MaxWindows:]
	}
}

// GetWindow returns a specific incident window
func (r *IncidentRecorder) GetWindow(windowID string) *IncidentWindow {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check active windows
	if window, ok := r.activeWindows[windowID]; ok {
		return copyWindow(window)
	}

	// Check completed windows
	for _, window := range r.completedWindows {
		if window.ID == windowID {
			return copyWindow(window)
		}
	}

	return nil
}

// GetWindowsForResource returns all incident windows for a resource
func (r *IncidentRecorder) GetWindowsForResource(resourceID string, limit int) []*IncidentWindow {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*IncidentWindow

	// Check active windows
	for _, window := range r.activeWindows {
		if window.ResourceID == resourceID {
			result = append(result, copyWindow(window))
		}
	}

	// Check completed windows (in reverse order for most recent first)
	for i := len(r.completedWindows) - 1; i >= 0; i-- {
		if r.completedWindows[i].ResourceID == resourceID {
			result = append(result, copyWindow(r.completedWindows[i]))
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// saveToDisk persists completed windows
func (r *IncidentRecorder) saveToDisk() error {
	if r.filePath == "" {
		return nil
	}

	data := struct {
		CompletedWindows []*IncidentWindow `json:"completed_windows"`
	}{
		CompletedWindows: r.snapshotCompletedWindows(),
	}

	data := struct {
		CompletedWindows []*IncidentWindow `json:"completed_windows"`
	}{
		CompletedWindows: completed,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("incident recorder save: marshal completed windows: %w", err)
	}

	if err := ensureOwnerOnlyDir(r.dataDir); err != nil {
		return err
	}

	if info, err := os.Lstat(r.filePath); err == nil {
		if err := validateRegularFilePath(r.filePath, info); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tmpFile, err := os.CreateTemp(r.dataDir, filepath.Base(r.filePath)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(incidentRecorderFilePerm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err := tmpFile.Write(jsonData); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, r.filePath); err != nil {
		return err
	}
	cleanup = false
	return os.Chmod(r.filePath, incidentRecorderFilePerm)
}

func (r *IncidentRecorder) snapshotCompletedWindows() []*IncidentWindow {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make([]*IncidentWindow, len(r.completedWindows))
	for i, window := range r.completedWindows {
		snapshot[i] = copyWindow(window)
	}
	return snapshot
}

func (r *IncidentRecorder) requestAsyncSave() {
	if r.filePath == "" {
		return
	}

	r.saveMu.Lock()
	r.saveRequested = true
	if r.saveInProgress {
		r.saveMu.Unlock()
		return
	}
	r.saveInProgress = true
	r.saveMu.Unlock()

	go r.saveLoop()
}

func (r *IncidentRecorder) saveLoop() {
	for {
		r.saveMu.Lock()
		if !r.saveRequested {
			r.saveInProgress = false
			r.saveCond.Broadcast()
			r.saveMu.Unlock()
			return
		}
		r.saveRequested = false
		r.saveMu.Unlock()

		if err := r.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to save incident windows")
		}
	}
}

func (r *IncidentRecorder) waitForPendingSaves() {
	if r.filePath == "" {
		return
	}

	r.saveMu.Lock()
	for r.saveInProgress || r.saveRequested {
		r.saveCond.Wait()
	}
	r.saveMu.Unlock()
}

// loadFromDisk loads completed windows
func (r *IncidentRecorder) loadFromDisk() error {
	if r.filePath == "" {
		return nil
	}

	jsonData, err := readBoundedRegularFile(r.filePath, maxIncidentWindowsFileSize)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("incident recorder load: read file %q: %w", r.filePath, err)
	}

	var data struct {
		CompletedWindows []*IncidentWindow `json:"completed_windows"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("incident recorder load: parse file %q: %w", r.filePath, err)
	}

	r.completedWindows = make([]*IncidentWindow, 0, len(data.CompletedWindows))
	for _, window := range data.CompletedWindows {
		if window == nil {
			continue
		}
		r.completedWindows = append(r.completedWindows, window)
	}
	r.trimCompletedWindows()

	return os.Chmod(r.filePath, incidentRecorderFilePerm)
}

// Helper functions

func ensureOwnerOnlyDir(dir string) error {
	if err := os.MkdirAll(dir, incidentRecorderDirPerm); err != nil {
		return err
	}
	return os.Chmod(dir, incidentRecorderDirPerm)
}

func validateRegularFilePath(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing symlink path %q", errUnsafeIncidentPersistencePath, path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: non-regular path %q", errUnsafeIncidentPersistencePath, path)
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
		return nil, fmt.Errorf("%w: file %q exceeds size limit (%d bytes)", errUnsafeIncidentPersistencePath, path, initialInfo.Size())
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
		return nil, fmt.Errorf("%w: file %q changed during read", errUnsafeIncidentPersistencePath, path)
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
		return nil, fmt.Errorf("%w: file %q exceeded size limit while reading", errUnsafeIncidentPersistencePath, path)
	}
	return data, nil
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	if in == nil {
		return nil
	}
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneAnyMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyDataPoints(dataPoints []IncidentDataPoint) []IncidentDataPoint {
	if dataPoints == nil {
		return nil
	}
	out := make([]IncidentDataPoint, len(dataPoints))
	for i, dp := range dataPoints {
		out[i] = IncidentDataPoint{
			Timestamp: dp.Timestamp,
			Metrics:   cloneFloatMap(dp.Metrics),
			Metadata:  cloneAnyMap(dp.Metadata),
		}
	}
	return out
}

func copyWindow(w *IncidentWindow) *IncidentWindow {
	if w == nil {
		return nil
	}
	windowCopy := *w
	if w.EndTime != nil {
		t := *w.EndTime
		copy.EndTime = &t
	}
	if w.DataPoints != nil {
		copy.DataPoints = copyDataPoints(w.DataPoints)
	}
	copy.DataPoints = copyDataPoints(w.DataPoints)
	if w.Summary != nil {
		s := *w.Summary
		s.Peaks = copyMetrics(w.Summary.Peaks)
		s.Lows = copyMetrics(w.Summary.Lows)
		s.Averages = copyMetrics(w.Summary.Averages)
		s.Changes = copyMetrics(w.Summary.Changes)
		if w.Summary.Anomalies != nil {
			s.Anomalies = append([]string(nil), w.Summary.Anomalies...)
		}
		copy.Summary = &s
	}
	return &windowCopy
}

var windowCounter int64

func generateWindowID(resourceID string) string {
	counter := atomic.AddInt64(&windowCounter, 1)
	return "iw-" + resourceID + "-" + time.Now().Format("20060102150405") + "-" + intToString(int(counter))
}

func copyDataPoints(points []IncidentDataPoint) []IncidentDataPoint {
	copied := make([]IncidentDataPoint, len(points))
	for i, dp := range points {
		copied[i] = dp
		copied[i].Metrics = copyMetrics(dp.Metrics)
		if dp.Metadata != nil {
			copied[i].Metadata = make(map[string]interface{}, len(dp.Metadata))
			for k, v := range dp.Metadata {
				copied[i].Metadata[k] = v
			}
		}
	}
	return copied
}

func copyMetrics(metrics map[string]float64) map[string]float64 {
	if metrics == nil {
		return nil
	}

	copied := make(map[string]float64, len(metrics))
	for k, v := range metrics {
		copied[k] = v
	}
	return copied
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var result string
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}
