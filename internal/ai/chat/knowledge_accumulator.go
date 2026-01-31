package chat

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// FactCategory classifies a knowledge fact for grouping in the rendered output.
type FactCategory string

const (
	FactCategoryResource  FactCategory = "resource"
	FactCategoryStorage   FactCategory = "storage"
	FactCategoryAlert     FactCategory = "alert"
	FactCategoryDiscovery FactCategory = "discovery"
	FactCategoryExec      FactCategory = "exec"
	FactCategoryMetrics   FactCategory = "metrics"
	FactCategoryFinding   FactCategory = "finding"
)

// isMarkerKey returns true if the fact key is a marker (ends with ":queried").
// Markers exist only for gate matching and should not appear in rendered output
// or count toward the character budget.
func isMarkerKey(key string) bool {
	return strings.HasSuffix(key, ":queried")
}

// Fact is a single extracted knowledge entry.
type Fact struct {
	Category   FactCategory
	Key        string // Dedup key, e.g. "lxc:delly:106:status"
	Value      string // Compact value, e.g. "running, Postfix, hostname=patrol-signal-test"
	ObservedAt time.Time
	Turn       int
}

const (
	defaultMaxEntries = 60
	defaultMaxChars   = 2000
	maxValueLen       = 200
)

// KnowledgeAccumulator stores extracted facts from tool results.
// Per-session, in-memory, bounded. Facts are keyed for upsert semantics.
// Thread-safe: all methods are protected by a mutex.
type KnowledgeAccumulator struct {
	mu          sync.Mutex
	facts       map[string]*Fact    // key -> fact (upsert: same key updates value)
	order       []string            // insertion order for LRU eviction
	toolFacts   map[string][]string // tool_use_id -> fact keys extracted from that tool call
	totalChars  int
	maxEntries  int
	maxChars    int
	currentTurn int
}

// NewKnowledgeAccumulator creates a new bounded accumulator.
func NewKnowledgeAccumulator() *KnowledgeAccumulator {
	return &KnowledgeAccumulator{
		facts:      make(map[string]*Fact),
		toolFacts:  make(map[string][]string),
		maxEntries: defaultMaxEntries,
		maxChars:   defaultMaxChars,
	}
}

// SetTurn updates the current turn number for new facts.
func (ka *KnowledgeAccumulator) SetTurn(turn int) {
	ka.mu.Lock()
	ka.currentTurn = turn
	ka.mu.Unlock()
}

// AddFact upserts a fact. If the key already exists, the value is updated.
// If over budget after insertion, the oldest non-pinned fact is evicted.
func (ka *KnowledgeAccumulator) AddFact(category FactCategory, key, value string) {
	if key == "" || value == "" {
		return
	}

	ka.mu.Lock()
	defer ka.mu.Unlock()

	ka.addFactLocked(category, key, value)
}

// AddFactForTool upserts a fact and records the association with a specific tool_use_id.
// This allows FactSummaryForTool to later retrieve what facts were extracted from a given tool call.
func (ka *KnowledgeAccumulator) AddFactForTool(toolUseID string, category FactCategory, key, value string) {
	if key == "" || value == "" {
		return
	}

	ka.mu.Lock()
	defer ka.mu.Unlock()

	ka.addFactLocked(category, key, value)
	if toolUseID != "" {
		ka.toolFacts[toolUseID] = append(ka.toolFacts[toolUseID], key)
	}
}

// FactSummaryForTool returns a joined summary of fact values extracted from the given tool call.
// Returns empty string if no facts were recorded for this tool_use_id.
func (ka *KnowledgeAccumulator) FactSummaryForTool(toolUseID string) string {
	ka.mu.Lock()
	defer ka.mu.Unlock()

	keys, ok := ka.toolFacts[toolUseID]
	if !ok || len(keys) == 0 {
		return ""
	}

	var parts []string
	for _, key := range keys {
		if fact, ok := ka.facts[key]; ok {
			parts = append(parts, fmt.Sprintf("%s = %s", key, fact.Value))
		}
	}
	return strings.Join(parts, "; ")
}

// addFactLocked is the internal implementation of AddFact. Caller must hold ka.mu.
func (ka *KnowledgeAccumulator) addFactLocked(category FactCategory, key, value string) {
	// Truncate value
	if len(value) > maxValueLen {
		value = value[:maxValueLen]
	}

	now := time.Now()

	marker := isMarkerKey(key)

	if existing, ok := ka.facts[key]; ok {
		// Upsert: update existing fact
		if !marker {
			ka.totalChars -= len(existing.Value)
		}
		existing.Value = value
		existing.Category = category
		existing.ObservedAt = now
		existing.Turn = ka.currentTurn
		if !marker {
			ka.totalChars += len(value)
		}
	} else {
		// New fact
		fact := &Fact{
			Category:   category,
			Key:        key,
			Value:      value,
			ObservedAt: now,
			Turn:       ka.currentTurn,
		}
		ka.facts[key] = fact
		ka.order = append(ka.order, key)
		if !marker {
			ka.totalChars += len(value)
		}
	}

	// Evict until within budget
	ka.evict()
}

// evict removes the oldest non-pinned facts until within budget.
// Facts from the current or previous turn are soft-pinned (not evicted).
// Caller must hold ka.mu.
func (ka *KnowledgeAccumulator) evict() {
	for (len(ka.facts) > ka.maxEntries || ka.totalChars > ka.maxChars) && len(ka.order) > 0 {
		evicted := false
		for i, key := range ka.order {
			fact, ok := ka.facts[key]
			if !ok {
				// Stale key in order slice — remove it
				ka.order = append(ka.order[:i], ka.order[i+1:]...)
				evicted = true
				break
			}
			// Soft-pin: don't evict facts from current or previous turn
			if fact.Turn >= ka.currentTurn-1 {
				continue
			}
			// Evict this fact
			if !isMarkerKey(key) {
				ka.totalChars -= len(fact.Value)
			}
			delete(ka.facts, key)
			ka.order = append(ka.order[:i], ka.order[i+1:]...)
			evicted = true
			break
		}
		if !evicted {
			// All remaining facts are pinned — can't evict more
			break
		}
	}
}

// Len returns the number of facts stored.
func (ka *KnowledgeAccumulator) Len() int {
	ka.mu.Lock()
	defer ka.mu.Unlock()
	return len(ka.facts)
}

// TotalChars returns the total character count of all fact values.
func (ka *KnowledgeAccumulator) TotalChars() int {
	ka.mu.Lock()
	defer ka.mu.Unlock()
	return ka.totalChars
}

// Lookup returns the fact value for a given key, or empty string if not found.
func (ka *KnowledgeAccumulator) Lookup(key string) (string, bool) {
	ka.mu.Lock()
	defer ka.mu.Unlock()
	if fact, ok := ka.facts[key]; ok {
		return fact.Value, true
	}
	return "", false
}

// RelatedFacts returns a semicolon-joined summary of non-marker facts
// whose keys start with the given prefix. Used by the gate to enrich
// marker-based cache hits with actual per-resource data.
func (ka *KnowledgeAccumulator) RelatedFacts(prefix string) string {
	ka.mu.Lock()
	defer ka.mu.Unlock()

	var parts []string
	for _, key := range ka.order {
		if isMarkerKey(key) || !strings.HasPrefix(key, prefix) {
			continue
		}
		if fact, ok := ka.facts[key]; ok {
			parts = append(parts, fmt.Sprintf("%s = %s", key, fact.Value))
		}
	}
	return strings.Join(parts, "; ")
}

// Render produces the system prompt section with grouped facts.
func (ka *KnowledgeAccumulator) Render() string {
	ka.mu.Lock()
	defer ka.mu.Unlock()

	if len(ka.facts) == 0 {
		return ""
	}

	// Group facts by category
	categoryOrder := []FactCategory{
		FactCategoryResource,
		FactCategoryStorage,
		FactCategoryAlert,
		FactCategoryDiscovery,
		FactCategoryExec,
		FactCategoryMetrics,
		FactCategoryFinding,
	}

	categoryLabels := map[FactCategory]string{
		FactCategoryResource:  "Resources",
		FactCategoryStorage:   "Storage",
		FactCategoryAlert:     "Alerts",
		FactCategoryDiscovery: "Discovery",
		FactCategoryExec:      "Exec",
		FactCategoryMetrics:   "Metrics",
		FactCategoryFinding:   "Findings",
	}

	grouped := make(map[FactCategory][]*Fact)
	for _, key := range ka.order {
		if isMarkerKey(key) {
			continue
		}
		if fact, ok := ka.facts[key]; ok {
			grouped[fact.Category] = append(grouped[fact.Category], fact)
		}
	}

	var sb strings.Builder
	sb.WriteString("## Known Facts (auto-collected — do NOT re-query unless user asks for fresh data)")

	for _, cat := range categoryOrder {
		facts, ok := grouped[cat]
		if !ok || len(facts) == 0 {
			continue
		}
		label := categoryLabels[cat]
		sb.WriteString(fmt.Sprintf("\n%s:", label))
		for _, fact := range facts {
			sb.WriteString(fmt.Sprintf("\n- [%s] %s", fact.Key, fact.Value))
		}
	}

	return sb.String()
}
