// Package memory provides persistent context and memory capabilities for AI patrol.
// This file implements cross-run memory that helps AI learn and remember insights.
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// MemoryType categorizes different types of memories
type MemoryType string

const (
	// MemoryTypeResource stores per-resource notes
	MemoryTypeResource MemoryType = "resource"
	// MemoryTypeIncident stores past incident learnings
	MemoryTypeIncident MemoryType = "incident"
	// MemoryTypePattern stores learned patterns
	MemoryTypePattern MemoryType = "pattern"
	// MemoryTypePreference stores user preferences
	MemoryTypePreference MemoryType = "preference"
)

// Memory represents a single memory entry
type Memory struct {
	ID         string     `json:"id"`
	Type       MemoryType `json:"type"`
	ResourceID string     `json:"resource_id,omitempty"`
	Content    string     `json:"content"`
	Source     string     `json:"source,omitempty"` // "ai", "user", "system"
	CreatedAt  time.Time  `json:"created_at"`
	LastUsed   time.Time  `json:"last_used"`
	UseCount   int        `json:"use_count"`
	Relevance  float64    `json:"relevance"` // 0-1, decays over time
	Tags       []string   `json:"tags,omitempty"`
	RelatedIDs []string   `json:"related_ids,omitempty"` // Related memory IDs
}

// ResourceMemory stores notes about a specific resource
type ResourceMemory struct {
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name,omitempty"`
	ResourceType string    `json:"resource_type,omitempty"`
	Notes        []string  `json:"notes"`
	Patterns     []string  `json:"patterns,omitempty"` // Observed patterns
	LastUpdated  time.Time `json:"last_updated"`
}

// IncidentMemory stores learnings from past incidents
type IncidentMemory struct {
	ID             string    `json:"id"`
	ResourceID     string    `json:"resource_id"`
	Timestamp      time.Time `json:"timestamp"`
	Summary        string    `json:"summary"`
	RootCause      string    `json:"root_cause,omitempty"`
	Resolution     string    `json:"resolution,omitempty"`
	LessonsLearned []string  `json:"lessons_learned,omitempty"`
}

// PatternMemory stores learned operational patterns
type PatternMemory struct {
	ID          string    `json:"id"`
	Pattern     string    `json:"pattern"`
	Description string    `json:"description"`
	Occurrences int       `json:"occurrences"`
	LastSeen    time.Time `json:"last_seen"`
	Confidence  float64   `json:"confidence"`
	Example     string    `json:"example,omitempty"`
}

// ContextStoreConfig configures the context store
type ContextStoreConfig struct {
	DataDir            string
	MaxMemoriesPerType int
	MaxResourceNotes   int
	RetentionDays      int
	RelevanceDecayDays int // Days after which relevance starts decaying
}

// DefaultContextStoreConfig returns sensible defaults
func DefaultContextStoreConfig() ContextStoreConfig {
	return ContextStoreConfig{
		MaxMemoriesPerType: 1000,
		MaxResourceNotes:   20,
		RetentionDays:      90,
		RelevanceDecayDays: 7,
	}
}

// ContextStore stores and manages persistent AI context
type ContextStore struct {
	mu     sync.RWMutex
	saveMu sync.Mutex // serializes disk writes to prevent .tmp file races

	config ContextStoreConfig

	// Memories by type
	memories map[MemoryType]map[string]*Memory

	// Resource-specific memories
	resourceMemories map[string]*ResourceMemory

	// Incident memories
	incidentMemories map[string]*IncidentMemory

	// Pattern memories
	patternMemories map[string]*PatternMemory

	// Persistence
	dataDir string
	dirty   bool
}

// NewContextStore creates a new context store
func NewContextStore(cfg ContextStoreConfig) *ContextStore {
	if cfg.MaxMemoriesPerType <= 0 {
		cfg.MaxMemoriesPerType = 1000
	}
	if cfg.MaxResourceNotes <= 0 {
		cfg.MaxResourceNotes = 20
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 90
	}
	if cfg.RelevanceDecayDays <= 0 {
		cfg.RelevanceDecayDays = 7
	}

	store := &ContextStore{
		config:           cfg,
		memories:         make(map[MemoryType]map[string]*Memory),
		resourceMemories: make(map[string]*ResourceMemory),
		incidentMemories: make(map[string]*IncidentMemory),
		patternMemories:  make(map[string]*PatternMemory),
		dataDir:          cfg.DataDir,
	}

	// Initialize memory type maps
	store.memories[MemoryTypeResource] = make(map[string]*Memory)
	store.memories[MemoryTypeIncident] = make(map[string]*Memory)
	store.memories[MemoryTypePattern] = make(map[string]*Memory)
	store.memories[MemoryTypePreference] = make(map[string]*Memory)

	// Load from disk
	if cfg.DataDir != "" {
		if err := store.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load context store from disk")
		} else {
			total := len(store.resourceMemories) + len(store.incidentMemories) + len(store.patternMemories)
			if total > 0 {
				log.Info().Int("total_memories", total).Msg("Loaded context store from disk")
			}
		}
	}

	return store
}

// Remember stores a new memory
func (s *ContextStore) Remember(resourceID, content, source string, memType MemoryType, tags ...string) string {
	s.mu.Lock()

	memory := &Memory{
		ID:         generateMemoryID(),
		Type:       memType,
		ResourceID: resourceID,
		Content:    content,
		Source:     source,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		UseCount:   1,
		Relevance:  1.0,
		Tags:       tags,
	}

	if _, ok := s.memories[memType]; !ok {
		s.memories[memType] = make(map[string]*Memory)
	}
	s.memories[memType][memory.ID] = memory

	// Also add to resource-specific memory
	if resourceID != "" && (memType == MemoryTypeResource || memType == MemoryTypePattern) {
		s.addResourceNoteLocked(resourceID, content)
	}

	s.dirty = true
	memoryID := memory.ID
	s.mu.Unlock()

	go s.saveIfDirty()

	log.Debug().
		Str("memory_id", memoryID).
		Str("type", string(memType)).
		Str("resource", resourceID).
		Msg("Stored new memory")

	return memoryID
}

// addResourceNoteLocked adds a note to a resource's memory (must hold lock)
func (s *ContextStore) addResourceNoteLocked(resourceID, note string) {
	if resourceID == "" || note == "" {
		return
	}

	mem, ok := s.resourceMemories[resourceID]
	if !ok {
		mem = &ResourceMemory{
			ResourceID: resourceID,
			Notes:      make([]string, 0),
		}
		s.resourceMemories[resourceID] = mem
	}

	// Check for duplicate
	for _, existing := range mem.Notes {
		if existing == note {
			return
		}
	}

	mem.Notes = append(mem.Notes, note)
	mem.LastUpdated = time.Now()

	// Trim to max notes
	if len(mem.Notes) > s.config.MaxResourceNotes {
		mem.Notes = mem.Notes[len(mem.Notes)-s.config.MaxResourceNotes:]
	}
}

// AddResourceNote adds a note about a specific resource
func (s *ContextStore) AddResourceNote(resourceID, resourceName, resourceType, note string) {
	s.mu.Lock()

	s.addResourceNoteLocked(resourceID, note)

	// Update name and type if provided
	if mem, ok := s.resourceMemories[resourceID]; ok {
		if resourceName != "" {
			mem.ResourceName = resourceName
		}
		if resourceType != "" {
			mem.ResourceType = resourceType
		}
	}

	s.dirty = true
	s.mu.Unlock()

	go s.saveIfDirty()
}

// AddIncidentMemory stores a learning from an incident
func (s *ContextStore) AddIncidentMemory(incident *IncidentMemory) {
	s.mu.Lock()

	if incident.ID == "" {
		incident.ID = generateIncidentMemoryID()
	}
	if incident.Timestamp.IsZero() {
		incident.Timestamp = time.Now()
	}

	s.incidentMemories[incident.ID] = incident

	// Also create a general memory
	content := incident.Summary
	if incident.RootCause != "" {
		content += " Root cause: " + incident.RootCause
	}
	if incident.Resolution != "" {
		content += " Resolution: " + incident.Resolution
	}

	memory := &Memory{
		ID:         generateMemoryID(),
		Type:       MemoryTypeIncident,
		ResourceID: incident.ResourceID,
		Content:    content,
		Source:     "system",
		CreatedAt:  incident.Timestamp,
		LastUsed:   time.Now(),
		UseCount:   1,
		Relevance:  1.0,
	}
	s.memories[MemoryTypeIncident][memory.ID] = memory

	s.dirty = true
	s.mu.Unlock()

	go s.saveIfDirty()
}

// AddPatternMemory stores a learned pattern
func (s *ContextStore) AddPatternMemory(pattern *PatternMemory) {
	s.mu.Lock()

	if pattern.ID == "" {
		pattern.ID = generatePatternMemoryID()
	}

	// Check if similar pattern exists
	for _, existing := range s.patternMemories {
		if existing.Pattern == pattern.Pattern {
			existing.Occurrences++
			existing.LastSeen = time.Now()
			existing.Confidence = calculatePatternConfidence(existing.Occurrences)
			s.dirty = true
			s.mu.Unlock()
			go s.saveIfDirty()
			return
		}
	}

	pattern.LastSeen = time.Now()
	pattern.Confidence = calculatePatternConfidence(pattern.Occurrences)
	s.patternMemories[pattern.ID] = pattern

	s.dirty = true
	s.mu.Unlock()

	go s.saveIfDirty()
}

// Recall retrieves memories relevant to a resource
func (s *ContextStore) Recall(resourceID string) []Memory {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []Memory

	// Get resource-specific memories
	for _, mem := range s.memories[MemoryTypeResource] {
		if mem.ResourceID == resourceID {
			s.markUsedLocked(mem)
			result = append(result, *mem)
		}
	}

	// Get incident memories
	for _, mem := range s.memories[MemoryTypeIncident] {
		if mem.ResourceID == resourceID {
			s.markUsedLocked(mem)
			result = append(result, *mem)
		}
	}

	// Get pattern memories related to resource
	for _, mem := range s.memories[MemoryTypePattern] {
		if mem.ResourceID == resourceID {
			s.markUsedLocked(mem)
			result = append(result, *mem)
		}
	}

	// Sort by relevance
	sort.Slice(result, func(i, j int) bool {
		return result[i].Relevance > result[j].Relevance
	})

	return result
}

// RecallByType retrieves memories of a specific type
func (s *ContextStore) RecallByType(memType MemoryType, limit int) []Memory {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []Memory
	if memories, ok := s.memories[memType]; ok {
		for _, mem := range memories {
			s.markUsedLocked(mem)
			result = append(result, *mem)
		}
	}

	// Sort by relevance
	sort.Slice(result, func(i, j int) bool {
		return result[i].Relevance > result[j].Relevance
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetResourceMemory returns the memory for a specific resource
func (s *ContextStore) GetResourceMemory(resourceID string) *ResourceMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if mem, ok := s.resourceMemories[resourceID]; ok {
		copy := *mem
		return &copy
	}
	return nil
}

// GetRecentIncidents returns recent incident memories
func (s *ContextStore) GetRecentIncidents(limit int) []*IncidentMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getRecentIncidentsLocked(limit)
}

// getRecentIncidentsLocked retrieves recent incidents without acquiring the lock (caller must hold it).
func (s *ContextStore) getRecentIncidentsLocked(limit int) []*IncidentMemory {
	var result []*IncidentMemory
	for _, incident := range s.incidentMemories {
		copy := *incident
		result = append(result, &copy)
	}

	// Sort by timestamp (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetPatterns returns learned patterns above confidence threshold
func (s *ContextStore) GetPatterns(minConfidence float64) []*PatternMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getPatternsLocked(minConfidence)
}

// getPatternsLocked retrieves patterns without acquiring the lock (caller must hold it).
func (s *ContextStore) getPatternsLocked(minConfidence float64) []*PatternMemory {
	var result []*PatternMemory
	for _, pattern := range s.patternMemories {
		if pattern.Confidence >= minConfidence {
			copy := *pattern
			result = append(result, &copy)
		}
	}

	// Sort by confidence
	sort.Slice(result, func(i, j int) bool {
		return result[i].Confidence > result[j].Confidence
	})

	return result
}

// markUsedLocked updates usage stats for a memory (must hold lock)
func (s *ContextStore) markUsedLocked(mem *Memory) {
	mem.LastUsed = time.Now()
	mem.UseCount++
	// Boost relevance slightly on use
	mem.Relevance = minF(1.0, mem.Relevance+0.1)
	s.dirty = true
}

// DecayRelevance applies time-based decay to memory relevance
func (s *ContextStore) DecayRelevance() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	decayStart := time.Duration(s.config.RelevanceDecayDays) * 24 * time.Hour

	for _, memories := range s.memories {
		for _, mem := range memories {
			age := now.Sub(mem.LastUsed)
			if age > decayStart {
				// Decay by 10% per week after decay starts
				weeks := (age - decayStart).Hours() / (24 * 7)
				decay := 0.1 * weeks
				mem.Relevance = maxF(0.1, mem.Relevance-decay)
			}
		}
	}

	s.dirty = true
}

// Cleanup removes old and low-relevance memories
func (s *ContextStore) Cleanup() int {
	s.mu.Lock()

	removed := 0
	cutoff := time.Now().AddDate(0, 0, -s.config.RetentionDays)
	minRelevance := 0.1

	for memType, memories := range s.memories {
		for id, mem := range memories {
			if mem.CreatedAt.Before(cutoff) || mem.Relevance < minRelevance {
				delete(s.memories[memType], id)
				removed++
			}
		}

		// Trim to max per type
		if len(memories) > s.config.MaxMemoriesPerType {
			// Convert to slice and sort by relevance
			var memList []*Memory
			for _, m := range memories {
				memList = append(memList, m)
			}
			sort.Slice(memList, func(i, j int) bool {
				return memList[i].Relevance > memList[j].Relevance
			})

			// Keep only top memories
			s.memories[memType] = make(map[string]*Memory)
			for i := 0; i < s.config.MaxMemoriesPerType && i < len(memList); i++ {
				s.memories[memType][memList[i].ID] = memList[i]
			}
			removed += len(memList) - s.config.MaxMemoriesPerType
		}
	}

	needsSave := removed > 0
	if needsSave {
		s.dirty = true
	}
	s.mu.Unlock()

	if needsSave {
		go s.saveIfDirty()
	}

	return removed
}

// FormatForPatrol formats context for patrol prompt injection
func (s *ContextStore) FormatForPatrol() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result string

	// Add resource notes
	if len(s.resourceMemories) > 0 {
		result += "\n## Resource Notes\n"
		result += "Notes about specific resources from previous observations:\n\n"

		count := 0
		for _, mem := range s.resourceMemories {
			if count >= 10 { // Limit for context size
				break
			}
			if len(mem.Notes) > 0 {
				name := mem.ResourceName
				if name == "" {
					name = mem.ResourceID
				}
				result += fmt.Sprintf("### %s\n", name)
				for _, note := range mem.Notes {
					result += "- " + note + "\n"
				}
				count++
			}
		}
	}

	// Add recent incidents
	incidents := s.getRecentIncidentsLocked(5)
	if len(incidents) > 0 {
		result += "\n## Recent Incidents\n"
		result += "Past incidents that may be relevant:\n\n"

		for _, incident := range incidents {
			result += fmt.Sprintf("- %s: %s", incident.Timestamp.Format("2006-01-02"), incident.Summary)
			if incident.RootCause != "" {
				result += fmt.Sprintf(" (Root cause: %s)", incident.RootCause)
			}
			result += "\n"
		}
	}

	// Add learned patterns
	patterns := s.getPatternsLocked(0.5)
	if len(patterns) > 0 {
		result += "\n## Learned Patterns\n"
		result += "Operational patterns observed over time:\n\n"

		for _, pattern := range patterns {
			if len(patterns) > 5 {
				break
			}
			result += fmt.Sprintf("- %s (%.0f%% confidence)\n", pattern.Description, pattern.Confidence*100)
		}
	}

	return result
}

// FormatForResource formats context for a specific resource
func (s *ContextStore) FormatForResource(resourceID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mem, ok := s.resourceMemories[resourceID]
	if !ok || len(mem.Notes) == 0 {
		return ""
	}

	result := "\n## Resource Memory\n"
	result += fmt.Sprintf("Notes about %s:\n", mem.ResourceName)

	for _, note := range mem.Notes {
		result += "- " + note + "\n"
	}

	// Add patterns if any
	if len(mem.Patterns) > 0 {
		result += "\nObserved patterns:\n"
		for _, pattern := range mem.Patterns {
			result += "- " + pattern + "\n"
		}
	}

	return result
}

// saveIfDirty saves to disk if there are changes
func (s *ContextStore) saveIfDirty() {
	s.mu.Lock()
	if !s.dirty || s.dataDir == "" {
		s.mu.Unlock()
		return
	}
	s.dirty = false
	s.mu.Unlock()

	if err := s.saveToDisk(); err != nil {
		log.Warn().Err(err).Msg("Failed to save context store")
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
	}
}

// saveToDisk persists data
func (s *ContextStore) saveToDisk() error {
	if s.dataDir == "" {
		return nil
	}

	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	s.mu.RLock()
	data := struct {
		Memories         map[MemoryType]map[string]*Memory `json:"memories"`
		ResourceMemories map[string]*ResourceMemory        `json:"resource_memories"`
		IncidentMemories map[string]*IncidentMemory        `json:"incident_memories"`
		PatternMemories  map[string]*PatternMemory         `json:"pattern_memories"`
	}{
		Memories:         s.memories,
		ResourceMemories: s.resourceMemories,
		IncidentMemories: s.incidentMemories,
		PatternMemories:  s.patternMemories,
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(s.dataDir, "ai_context.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// loadFromDisk loads data
func (s *ContextStore) loadFromDisk() error {
	if s.dataDir == "" {
		return nil
	}

	path := filepath.Join(s.dataDir, "ai_context.json")
	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data struct {
		Memories         map[MemoryType]map[string]*Memory `json:"memories"`
		ResourceMemories map[string]*ResourceMemory        `json:"resource_memories"`
		IncidentMemories map[string]*IncidentMemory        `json:"incident_memories"`
		PatternMemories  map[string]*PatternMemory         `json:"pattern_memories"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	if data.Memories != nil {
		s.memories = data.Memories
	}
	if data.ResourceMemories != nil {
		s.resourceMemories = data.ResourceMemories
	}
	if data.IncidentMemories != nil {
		s.incidentMemories = data.IncidentMemories
	}
	if data.PatternMemories != nil {
		s.patternMemories = data.PatternMemories
	}

	return nil
}

// ForceSave immediately saves to disk
func (s *ContextStore) ForceSave() error {
	s.mu.Lock()
	s.dirty = false
	s.mu.Unlock()
	return s.saveToDisk()
}

// Helper functions

var memoryCounter, incidentMemCounter, patternMemCounter atomic.Int64

func generateMemoryID() string {
	n := memoryCounter.Add(1)
	return fmt.Sprintf("mem-%s-%d", time.Now().Format("20060102150405"), n%1000)
}

func generateIncidentMemoryID() string {
	n := incidentMemCounter.Add(1)
	return fmt.Sprintf("inc-mem-%s-%d", time.Now().Format("20060102150405"), n%1000)
}

func generatePatternMemoryID() string {
	n := patternMemCounter.Add(1)
	return fmt.Sprintf("pat-mem-%s-%d", time.Now().Format("20060102150405"), n%1000)
}

func calculatePatternConfidence(occurrences int) float64 {
	// Logarithmic confidence growth
	if occurrences < 3 {
		return float64(occurrences) * 0.15
	}
	confidence := 0.45 + 0.1*float64(occurrences-3)
	if confidence > 0.95 {
		confidence = 0.95
	}
	return confidence
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
