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
	FactCategoryDiscovery FactCategory = "discovery"
	FactCategoryExec      FactCategory = "exec"
	FactCategoryMetrics   FactCategory = "metrics"
	FactCategoryFinding   FactCategory = "finding"
)

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
	facts       map[string]*Fact // key -> fact (upsert: same key updates value)
	order       []string         // insertion order for LRU eviction
	totalChars  int
	maxEntries  int
	maxChars    int
	currentTurn int
}

// NewKnowledgeAccumulator creates a new bounded accumulator.
func NewKnowledgeAccumulator() *KnowledgeAccumulator {
	return &KnowledgeAccumulator{
		facts:      make(map[string]*Fact),
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

	// Truncate value
	if len(value) > maxValueLen {
		value = value[:maxValueLen]
	}

	now := time.Now()

	if existing, ok := ka.facts[key]; ok {
		// Upsert: update existing fact
		ka.totalChars -= len(existing.Value)
		existing.Value = value
		existing.Category = category
		existing.ObservedAt = now
		existing.Turn = ka.currentTurn
		ka.totalChars += len(value)
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
		ka.totalChars += len(value)
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
			ka.totalChars -= len(fact.Value)
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
		FactCategoryDiscovery,
		FactCategoryExec,
		FactCategoryMetrics,
		FactCategoryFinding,
	}

	categoryLabels := map[FactCategory]string{
		FactCategoryResource:  "Resources",
		FactCategoryStorage:   "Storage",
		FactCategoryDiscovery: "Discovery",
		FactCategoryExec:      "Exec",
		FactCategoryMetrics:   "Metrics",
		FactCategoryFinding:   "Findings",
	}

	grouped := make(map[FactCategory][]*Fact)
	for _, key := range ka.order {
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
			sb.WriteString(fmt.Sprintf("\n- %s", fact.Value))
		}
	}

	return sb.String()
}
