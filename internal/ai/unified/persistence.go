// Package unified provides a unified alert/finding system.
package unified

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
)

// FilePersistence implements UnifiedPersistence using JSON files
type FilePersistence struct {
	mu       sync.Mutex
	dataDir  string
	filename string
}

// NewFilePersistence creates a new file-based persistence
func NewFilePersistence(dataDir string) *FilePersistence {
	return &FilePersistence{
		dataDir:  dataDir,
		filename: "unified_findings.json",
	}
}

// SaveFindings saves findings to disk
func (p *FilePersistence) SaveFindings(findings map[string]*UnifiedFinding) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(p.dataDir, 0755); err != nil {
		return fmt.Errorf("unified.FilePersistence.SaveFindings: create data directory %q: %w", p.dataDir, err)
	}

	// Convert to list for cleaner JSON
	var findingsList []*UnifiedFinding
	for _, f := range findings {
		findingsList = append(findingsList, f)
	}

	data, err := json.MarshalIndent(findingsList, "", "  ")
	if err != nil {
		return fmt.Errorf("unified.FilePersistence.SaveFindings: marshal findings: %w", err)
	}

	filePath := filepath.Join(p.dataDir, p.filename)
	tempPath := filePath + ".tmp"

	// Write to temp file first
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("unified.FilePersistence.SaveFindings: write temp file %q: %w", tempPath, err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		removeErr := os.Remove(tempPath)
		renameErr := fmt.Errorf("unified.FilePersistence.SaveFindings: rename %q to %q: %w", tempPath, filePath, err)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return errors.Join(renameErr, fmt.Errorf("unified.FilePersistence.SaveFindings: remove temp file %q: %w", tempPath, removeErr))
		}
		return renameErr
	}

	log.Debug().
		Int("findings_count", len(findings)).
		Str("file", filePath).
		Msg("Saved unified findings")

	return nil
}

// LoadFindings loads findings from disk
func (p *FilePersistence) LoadFindings() (map[string]*UnifiedFinding, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	filePath := filepath.Join(p.dataDir, p.filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*UnifiedFinding), nil
		}
		return nil, fmt.Errorf("unified.FilePersistence.LoadFindings: read file %q: %w", filePath, err)
	}

	var findingsList []*UnifiedFinding
	if err := json.Unmarshal(data, &findingsList); err != nil {
		log.Error().Err(err).Msg("failed to parse unified findings, starting fresh")
		return make(map[string]*UnifiedFinding), nil
	}

	// Convert to map
	findings := make(map[string]*UnifiedFinding, len(findingsList))
	for _, f := range findingsList {
		findings[f.ID] = f
	}

	log.Debug().
		Int("findings_count", len(findings)).
		Str("file", filePath).
		Msg("Loaded unified findings")

	return findings, nil
}

// persistedState is the internal format for persistence
type persistedState struct {
	Version  int               `json:"version"`
	Findings []*UnifiedFinding `json:"findings"`
}

// VersionedPersistence adds versioning to persistence
type VersionedPersistence struct {
	*FilePersistence
	currentVersion int
}

// NewVersionedPersistence creates a versioned persistence layer
func NewVersionedPersistence(dataDir string) *VersionedPersistence {
	return &VersionedPersistence{
		FilePersistence: NewFilePersistence(dataDir),
		currentVersion:  1,
	}
}

// SaveFindings saves findings with version info
func (p *VersionedPersistence) SaveFindings(findings map[string]*UnifiedFinding) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(p.dataDir, 0755); err != nil {
		return fmt.Errorf("unified.VersionedPersistence.SaveFindings: create data directory %q: %w", p.dataDir, err)
	}

	state := persistedState{
		Version:  p.currentVersion,
		Findings: make([]*UnifiedFinding, 0, len(findings)),
	}

	for _, f := range findings {
		state.Findings = append(state.Findings, f)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("unified.VersionedPersistence.SaveFindings: marshal findings state: %w", err)
	}

	filePath := filepath.Join(p.dataDir, p.filename)
	tempPath := filePath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("unified.VersionedPersistence.SaveFindings: write temp file %q: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		removeErr := os.Remove(tempPath)
		renameErr := fmt.Errorf("unified.VersionedPersistence.SaveFindings: rename %q to %q: %w", tempPath, filePath, err)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return errors.Join(renameErr, fmt.Errorf("unified.VersionedPersistence.SaveFindings: remove temp file %q: %w", tempPath, removeErr))
		}
		return renameErr
	}

	return nil
}

// LoadFindings loads findings with version handling
func (p *VersionedPersistence) LoadFindings() (map[string]*UnifiedFinding, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	filePath := filepath.Join(p.dataDir, p.filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*UnifiedFinding), nil
		}
		return nil, fmt.Errorf("unified.VersionedPersistence.LoadFindings: read file %q: %w", filePath, err)
	}

	// Try versioned format first
	var state persistedState
	if err := json.Unmarshal(data, &state); err == nil && state.Version > 0 {
		findings := make(map[string]*UnifiedFinding, len(state.Findings))
		for _, f := range state.Findings {
			findings[f.ID] = f
		}
		return findings, nil
	}

	// Fall back to legacy format (list only)
	var findingsList []*UnifiedFinding
	if err := json.Unmarshal(data, &findingsList); err != nil {
		log.Error().Err(err).Msg("failed to parse unified findings")
		return make(map[string]*UnifiedFinding), nil
	}

	findings := make(map[string]*UnifiedFinding, len(findingsList))
	for _, f := range findingsList {
		findings[f.ID] = f
	}

	return findings, nil
}
