// Package chat provides direct AI chat integration without external sidecar processes
package chat

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// Session represents a chat session
type Session struct {
	ID           string    `json:"id"`
	Title        string    `json:"title,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count,omitempty"`
}

// Message represents a chat message
type Message struct {
	ID               string      `json:"id"`
	Role             string      `json:"role"` // "user", "assistant", "system"
	Content          string      `json:"content"`
	ReasoningContent string      `json:"reasoning_content,omitempty"` // For extended thinking
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`
	ToolResult       *ToolResult `json:"tool_result,omitempty"`
	Timestamp        time.Time   `json:"timestamp"`
}

// ToolCall represents a tool invocation
type ToolCall struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Input            map[string]interface{} `json:"input"`
	ThoughtSignature json.RawMessage        `json:"thought_signature,omitempty"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// StreamEvent represents a streaming event sent to the frontend
type StreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// StreamCallback is called for each streaming event
type StreamCallback func(event StreamEvent)

// StructuredMention represents a resource explicitly tagged by the user via @ mention.
// The frontend resolves these from the autocomplete and sends them alongside the prompt,
// so the backend doesn't need to re-derive resource identity from text.
type StructuredMention struct {
	ID   string `json:"id"`             // e.g. "lxc:delly:123", "docker:host:container"
	Name string `json:"name"`           // Display name, e.g. "ntfy"
	Type string `json:"type"`           // "vm", "lxc", "container", "docker", "node", "host"
	Node string `json:"node,omitempty"` // Proxmox node or parent host
}

// ExecuteRequest represents a chat execution request
type ExecuteRequest struct {
	Prompt         string              `json:"prompt"`
	SessionID      string              `json:"session_id,omitempty"`
	Model          string              `json:"model,omitempty"`
	Mentions       []StructuredMention `json:"mentions,omitempty"`
	FindingID      string              `json:"finding_id,omitempty"`      // Pre-populate finding context for "Discuss" flow
	MaxTurns       int                 `json:"max_turns,omitempty"`       // Override max agentic turns (0 = use default)
	AutonomousMode *bool               `json:"autonomous_mode,omitempty"` // Per-request autonomous override (nil = use service default)
}

// QuestionAnswer represents a user's answer to a question
type QuestionAnswer struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// ContentData is the data for "content" events
type ContentData struct {
	Text string `json:"text"`
}

// ThinkingData is the data for "thinking" events (extended thinking/reasoning)
type ThinkingData struct {
	Text string `json:"text"`
}

// ExploreStatusData is the data for "explore_status" events
type ExploreStatusData struct {
	Phase   string `json:"phase"`             // started | completed | failed | skipped
	Message string `json:"message"`           // Human-readable status text for the UI
	Model   string `json:"model,omitempty"`   // provider:model used for explore
	Outcome string `json:"outcome,omitempty"` // success | failed | skipped_no_model | skipped_no_tools
}

// ToolStartData is the data for "tool_start" events
type ToolStartData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"`               // JSON string of input parameters
	RawInput string `json:"raw_input,omitempty"` // Unmodified JSON input
}

// ToolEndData is the data for "tool_end" events
type ToolEndData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input,omitempty"`
	RawInput string `json:"raw_input,omitempty"`
	Output   string `json:"output,omitempty"`
	Success  bool   `json:"success"`
}

// ApprovalNeededData is the data for "approval_needed" events
type ApprovalNeededData struct {
	ApprovalID  string `json:"approval_id"`
	ToolID      string `json:"tool_id"`
	ToolName    string `json:"tool_name"`
	Command     string `json:"command"`
	RunOnHost   bool   `json:"run_on_host"`
	TargetHost  string `json:"target_host,omitempty"`
	Risk        string `json:"risk,omitempty"`
	Description string `json:"description,omitempty"`
}

// QuestionData is the data for "question" events
type QuestionData struct {
	SessionID  string     `json:"session_id,omitempty"`
	QuestionID string     `json:"question_id"`
	Questions  []Question `json:"questions"`
}

// Question represents a question from the AI to the user
type Question struct {
	ID       string           `json:"id"`
	Type     string           `json:"type,omitempty"` // "text" | "select"
	Question string           `json:"question"`
	Header   string           `json:"header,omitempty"`
	Options  []QuestionOption `json:"options,omitempty"`
}

// QuestionOption represents an option for a question
type QuestionOption struct {
	Label       string `json:"label"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
}

// ErrorData is the data for "error" events
type ErrorData struct {
	Message string `json:"message"`
}

// DoneData is the data for "done" events
type DoneData struct {
	SessionID    string `json:"session_id,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// Control level constants
const (
	ControlLevelReadOnly   = "read_only"
	ControlLevelControlled = "controlled"
	ControlLevelAutonomous = "autonomous"
)

// ResolvedResource represents a resource that has been authoritatively resolved
// through pulse_query or pulse_discovery. This is the source of truth that
// the model cannot override - action tools must reference these IDs.
//
// This type implements tools.ResolvedResourceInfo interface.
type ResolvedResource struct {
	// === Structured Identity (primary) ===

	// Kind is the resource type: "node", "vm", "lxc", "docker_container", "docker_host", "k8s_pod", etc.
	Kind string `json:"kind"`

	// ProviderUID is the stable identifier from the provider (container ID, VMID, pod UID).
	// This is the primary identifier - names are aliases.
	ProviderUID string `json:"provider_uid"`

	// Scope defines the context/location of this resource
	Scope ResourceScope `json:"scope"`

	// Aliases are user-friendly names that resolve to this resource.
	// e.g., ["jellyfin", "@jellyfin", "media-server/jellyfin"]
	Aliases []string `json:"aliases"`

	// === Derived/Display Fields ===

	// ResourceID is the canonical string identifier, derived from Kind:ProviderUID
	// Format: "{kind}:{provider_uid}" e.g., "docker_container:abc123def456"
	// For backwards compatibility, falls back to "{kind}:{name}" if no ProviderUID
	ResourceID string `json:"resource_id"`

	// Name is the primary display name (first alias, typically)
	Name string `json:"name"`

	// ResourceType is kept for backwards compatibility (same as Kind)
	ResourceType string `json:"resource_type"`

	// DisplayPath is the human-readable location string
	// e.g., "docker:jellyfin @ lxc:media-server @ node:delly"
	DisplayPath string `json:"display_path,omitempty"`

	// LocationChain is the hierarchical path (for backwards compatibility)
	// Example: ["node:delly", "lxc:media-server", "docker:jellyfin"]
	LocationChain []string `json:"location_chain"`

	// === Executor Paths ===

	// ReachableVia lists executors that can reach this resource and what actions each supports.
	// This replaces the single TargetHost/AgentID/Adapter fields for multi-path routing.
	ReachableVia []ExecutorPath `json:"reachable_via,omitempty"`

	// Legacy routing fields (for backwards compatibility, derived from first ReachableVia)
	TargetHost string `json:"target_host"` // Where commands should be sent
	AgentID    string `json:"agent_id"`    // Agent that handles this resource
	Adapter    string `json:"adapter"`     // "direct", "pct", "qm", "docker"

	// === Resource-specific fields ===

	// Proxmox-specific
	VMID int    `json:"vmid,omitempty"`
	Node string `json:"node,omitempty"`

	// AllowedActions is the union of all actions from ReachableVia (for quick checks)
	AllowedActions []string `json:"allowed_actions"`

	// Timestamps
	ResolvedAt time.Time `json:"resolved_at"`
}

// ResourceScope defines where a resource exists in the infrastructure hierarchy.
type ResourceScope struct {
	// HostUID is the stable ID of the host/node this resource runs on
	HostUID string `json:"host_uid,omitempty"`

	// HostName is the display name of the host
	HostName string `json:"host_name,omitempty"`

	// ParentUID is the stable ID of the parent resource (e.g., LXC container for nested Docker)
	ParentUID string `json:"parent_uid,omitempty"`

	// ParentKind is the type of parent resource
	ParentKind string `json:"parent_kind,omitempty"`

	// ClusterUID for Kubernetes resources
	ClusterUID string `json:"cluster_uid,omitempty"`

	// Namespace for Kubernetes resources
	Namespace string `json:"namespace,omitempty"`
}

// ExecutorPath describes how to reach a resource through a specific executor.
type ExecutorPath struct {
	// ExecutorID identifies the agent/executor that can reach this resource
	ExecutorID string `json:"executor_id"`

	// Adapter is the execution method: "direct", "pct", "qm", "docker", "kubectl"
	Adapter string `json:"adapter"`

	// Actions this executor can perform on the resource
	Actions []string `json:"actions"`

	// Priority for executor selection (higher = preferred)
	Priority int `json:"priority"`
}

// Implement tools.ResolvedResourceInfo interface
func (r *ResolvedResource) GetResourceID() string       { return r.ResourceID }
func (r *ResolvedResource) GetResourceType() string     { return r.ResourceType }
func (r *ResolvedResource) GetTargetHost() string       { return r.TargetHost }
func (r *ResolvedResource) GetAgentID() string          { return r.AgentID }
func (r *ResolvedResource) GetAdapter() string          { return r.Adapter }
func (r *ResolvedResource) GetVMID() int                { return r.VMID }
func (r *ResolvedResource) GetNode() string             { return r.Node }
func (r *ResolvedResource) GetAllowedActions() []string { return r.AllowedActions }
func (r *ResolvedResource) GetProviderUID() string      { return r.ProviderUID }
func (r *ResolvedResource) GetKind() string             { return r.Kind }
func (r *ResolvedResource) GetAliases() []string        { return r.Aliases }

// HasAlias checks if the resource has a specific alias (case-insensitive)
func (r *ResolvedResource) HasAlias(name string) bool {
	nameLower := strings.ToLower(name)
	for _, alias := range r.Aliases {
		if strings.ToLower(alias) == nameLower {
			return true
		}
	}
	return strings.ToLower(r.Name) == nameLower
}

// GetBestExecutor returns the highest-priority executor path that supports the given action
func (r *ResolvedResource) GetBestExecutor(action string) *ExecutorPath {
	var best *ExecutorPath
	for i := range r.ReachableVia {
		path := &r.ReachableVia[i]
		for _, a := range path.Actions {
			if a == action || a == "*" {
				if best == nil || path.Priority > best.Priority {
					best = path
				}
				break
			}
		}
	}
	return best
}

// Default values for TTL and size caps
const (
	DefaultResolvedContextTTL        = 45 * time.Minute // Sliding window TTL
	DefaultResolvedContextMaxEntries = 500              // Max resources per session
)

// ResolvedContext holds all resolved resources for a session.
// This is the authoritative context that action tools validate against.
// The model cannot fabricate or mutate these - they only come from
// successful query/discovery tool executions.
//
// Features:
//   - TTL: Resources expire after DefaultResolvedContextTTL of inactivity (sliding window)
//   - LRU eviction: When MaxEntries is reached, least recently used resources are evicted
//   - Pinning: Resources can be pinned to prevent eviction (e.g., primary targets)
//
// This type implements tools.ResolvedContextProvider interface.
type ResolvedContext struct {
	// mu protects all map fields from concurrent access during parallel tool execution.
	mu sync.RWMutex `json:"-"`

	SessionID string `json:"session_id"`

	// Resources maps resource names to their resolved information
	// Multiple names can point to the same resource (aliases)
	Resources map[string]*ResolvedResource `json:"resources"`

	// ResourcesByID provides lookup by canonical ResourceID (kind:provider_uid)
	ResourcesByID map[string]*ResolvedResource `json:"-"` // Not persisted, rebuilt on load

	// ResourcesByAlias provides lookup by any alias (case-insensitive)
	ResourcesByAlias map[string]*ResolvedResource `json:"-"` // Not persisted, rebuilt on load

	// LRU tracking: maps ResourceID to last access time (used for TTL and LRU eviction)
	lastAccessed map[string]time.Time `json:"-"`

	// Explicit access tracking: maps ResourceID to time of explicit user access
	// This is SEPARATE from lastAccessed and is ONLY set via MarkExplicitAccess.
	// Used for routing validation to detect "user referenced this resource this turn".
	// Key distinction: lastAccessed tracks all access (add, get, lookup);
	// explicitlyAccessed tracks only explicit user intent (get, @mention).
	explicitlyAccessed map[string]time.Time `json:"-"`

	// Pinned resources that won't be evicted (by ResourceID)
	pinned map[string]bool `json:"-"`

	// Configuration
	ttl        time.Duration `json:"-"`
	maxEntries int           `json:"-"`

	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
}

// NewResolvedContext creates a new empty resolved context for a session
func NewResolvedContext(sessionID string) *ResolvedContext {
	return &ResolvedContext{
		SessionID:          sessionID,
		Resources:          make(map[string]*ResolvedResource),
		ResourcesByID:      make(map[string]*ResolvedResource),
		ResourcesByAlias:   make(map[string]*ResolvedResource),
		lastAccessed:       make(map[string]time.Time),
		explicitlyAccessed: make(map[string]time.Time),
		pinned:             make(map[string]bool),
		ttl:                DefaultResolvedContextTTL,
		maxEntries:         DefaultResolvedContextMaxEntries,
		CreatedAt:          time.Now(),
		LastUpdated:        time.Now(),
	}
}

// NewResolvedContextWithConfig creates a new resolved context with custom TTL and size limits
func NewResolvedContextWithConfig(sessionID string, ttl time.Duration, maxEntries int) *ResolvedContext {
	ctx := NewResolvedContext(sessionID)
	if ttl > 0 {
		ctx.ttl = ttl
	}
	if maxEntries > 0 {
		ctx.maxEntries = maxEntries
	}
	return ctx
}

// touch updates the last access time for a resource (LRU tracking)
func (rc *ResolvedContext) touch(resourceID string) {
	if rc.lastAccessed == nil {
		rc.lastAccessed = make(map[string]time.Time)
	}
	rc.lastAccessed[resourceID] = time.Now()
}

// evictExpired removes resources that haven't been accessed within the TTL
func (rc *ResolvedContext) evictExpired() {
	if rc.ttl == 0 {
		return
	}

	cutoff := time.Now().Add(-rc.ttl)
	var toEvict []string

	for resourceID, lastAccess := range rc.lastAccessed {
		if lastAccess.Before(cutoff) && !rc.pinned[resourceID] {
			toEvict = append(toEvict, resourceID)
		}
	}

	for _, resourceID := range toEvict {
		rc.removeByID(resourceID)
	}

	// Also prune expired explicit access entries to prevent memory creep
	rc.pruneExpiredExplicitAccess()
}

// pruneExpiredExplicitAccess removes explicit access entries older than RecentAccessWindow.
// This prevents the explicitlyAccessed map from growing indefinitely in long sessions.
// Note: Uses tools.RecentAccessWindow (30s) as the cutoff, not the general TTL.
func (rc *ResolvedContext) pruneExpiredExplicitAccess() {
	if rc.explicitlyAccessed == nil {
		return
	}

	cutoff := time.Now().Add(-tools.RecentAccessWindow)
	for resourceID, accessTime := range rc.explicitlyAccessed {
		if accessTime.Before(cutoff) {
			delete(rc.explicitlyAccessed, resourceID)
		}
	}
}

// evictOverflow removes least recently used resources when over capacity
func (rc *ResolvedContext) evictOverflow() {
	if rc.maxEntries == 0 {
		return
	}

	// Count unique resources (by ID)
	uniqueCount := len(rc.ResourcesByID)
	if uniqueCount <= rc.maxEntries {
		return
	}

	// Find LRU candidates (not pinned)
	type lruEntry struct {
		resourceID string
		lastAccess time.Time
	}
	var candidates []lruEntry

	for resourceID, lastAccess := range rc.lastAccessed {
		if !rc.pinned[resourceID] {
			candidates = append(candidates, lruEntry{resourceID, lastAccess})
		}
	}

	// Sort by last access time (oldest first)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].lastAccess.Before(candidates[i].lastAccess) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Evict oldest until under capacity
	toEvict := uniqueCount - rc.maxEntries
	for i := 0; i < toEvict && i < len(candidates); i++ {
		rc.removeByID(candidates[i].resourceID)
	}
}

// removeByID removes a resource and all its aliases from the context
func (rc *ResolvedContext) removeByID(resourceID string) {
	res, ok := rc.ResourcesByID[resourceID]
	if !ok {
		return
	}

	// Remove from primary maps
	delete(rc.ResourcesByID, resourceID)
	delete(rc.Resources, res.Name)
	delete(rc.lastAccessed, resourceID)

	// Remove all aliases
	for _, alias := range res.Aliases {
		delete(rc.ResourcesByAlias, strings.ToLower(alias))
	}
	delete(rc.ResourcesByAlias, strings.ToLower(res.Name))
}

// PinResource marks a resource as pinned (won't be evicted)
func (rc *ResolvedContext) PinResource(resourceID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.pinned == nil {
		rc.pinned = make(map[string]bool)
	}
	rc.pinned[resourceID] = true
}

// UnpinResource removes the pinned status from a resource
func (rc *ResolvedContext) UnpinResource(resourceID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.pinned, resourceID)
}

// IsPinned returns whether a resource is pinned
func (rc *ResolvedContext) IsPinned(resourceID string) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.pinned[resourceID]
}

// Clear removes all resources from the context (respects pinned)
func (rc *ResolvedContext) Clear(keepPinned bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !keepPinned {
		rc.Resources = make(map[string]*ResolvedResource)
		rc.ResourcesByID = make(map[string]*ResolvedResource)
		rc.ResourcesByAlias = make(map[string]*ResolvedResource)
		rc.lastAccessed = make(map[string]time.Time)
		rc.pinned = make(map[string]bool)
		return
	}

	// Keep only pinned resources
	var toKeep []string
	for resourceID := range rc.pinned {
		if rc.pinned[resourceID] {
			toKeep = append(toKeep, resourceID)
		}
	}

	// Rebuild with only pinned resources
	oldByID := rc.ResourcesByID
	rc.Resources = make(map[string]*ResolvedResource)
	rc.ResourcesByID = make(map[string]*ResolvedResource)
	rc.ResourcesByAlias = make(map[string]*ResolvedResource)
	newLastAccessed := make(map[string]time.Time)

	for _, resourceID := range toKeep {
		if res, ok := oldByID[resourceID]; ok {
			rc.Resources[res.Name] = res
			rc.ResourcesByID[resourceID] = res
			for _, alias := range res.Aliases {
				rc.ResourcesByAlias[strings.ToLower(alias)] = res
			}
			rc.ResourcesByAlias[strings.ToLower(res.Name)] = res
			if lastAccess, ok := rc.lastAccessed[resourceID]; ok {
				newLastAccessed[resourceID] = lastAccess
			}
		}
	}
	rc.lastAccessed = newLastAccessed
}

// Stats returns statistics about the context
func (rc *ResolvedContext) Stats() ResolvedContextStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	uniqueCount := len(rc.ResourcesByID)
	pinnedCount := 0
	for _, isPinned := range rc.pinned {
		if isPinned {
			pinnedCount++
		}
	}

	return ResolvedContextStats{
		UniqueResources: uniqueCount,
		TotalAliases:    len(rc.ResourcesByAlias),
		PinnedResources: pinnedCount,
		MaxEntries:      rc.maxEntries,
		TTL:             rc.ttl,
	}
}

// ResolvedContextStats provides statistics about a resolved context
type ResolvedContextStats struct {
	UniqueResources int           `json:"unique_resources"`
	TotalAliases    int           `json:"total_aliases"`
	PinnedResources int           `json:"pinned_resources"`
	MaxEntries      int           `json:"max_entries"`
	TTL             time.Duration `json:"ttl"`
}

// HasAnyResources implements tools.ResolvedContextProvider interface.
// Returns true if at least one resource has been discovered in this session.
func (rc *ResolvedContext) HasAnyResources() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.ResourcesByID) > 0
}

// WasRecentlyAccessed checks if a resource was EXPLICITLY accessed within the given time window.
// This uses the separate explicitlyAccessed tracking (set only via MarkExplicitAccess),
// NOT the general lastAccessed (which is set on every add/get for LRU purposes).
//
// This distinction is critical for routing validation:
// - Bulk discovery adds resources (sets lastAccessed) but should NOT trigger routing blocks
// - Explicit get/select sets explicitlyAccessed, indicating user intent to target that resource
func (rc *ResolvedContext) WasRecentlyAccessed(resourceID string, window time.Duration) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.explicitlyAccessed == nil {
		return false
	}
	explicitAccess, ok := rc.explicitlyAccessed[resourceID]
	if !ok {
		return false
	}
	return time.Since(explicitAccess) <= window
}

// GetRecentlyAccessedResources returns resources that were EXPLICITLY accessed within the given time window.
// Uses explicitlyAccessed (not lastAccessed) to avoid false positives from bulk discovery.
// Returns a slice of resource IDs.
func (rc *ResolvedContext) GetRecentlyAccessedResources(window time.Duration) []string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.explicitlyAccessed == nil {
		return nil
	}
	var recent []string
	cutoff := time.Now().Add(-window)
	for resourceID, explicitAccess := range rc.explicitlyAccessed {
		if explicitAccess.After(cutoff) {
			recent = append(recent, resourceID)
		}
	}
	return recent
}

// GetRecentlyAccessedResourcesSorted returns recently accessed resources ordered by most recent access.
// If max <= 0, all recent resources are returned.
func (rc *ResolvedContext) GetRecentlyAccessedResourcesSorted(window time.Duration, max int) []string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.explicitlyAccessed == nil {
		return nil
	}

	cutoff := time.Now().Add(-window)
	type recentResource struct {
		id string
		ts time.Time
	}
	var recent []recentResource

	for resourceID, explicitAccess := range rc.explicitlyAccessed {
		if explicitAccess.After(cutoff) {
			if _, ok := rc.ResourcesByID[resourceID]; ok {
				recent = append(recent, recentResource{id: resourceID, ts: explicitAccess})
			}
		}
	}

	sort.Slice(recent, func(i, j int) bool {
		return recent[i].ts.After(recent[j].ts)
	})

	if max > 0 && len(recent) > max {
		recent = recent[:max]
	}

	ids := make([]string, len(recent))
	for i, res := range recent {
		ids[i] = res.id
	}
	return ids
}

// AddResource adds a resolved resource to the context (internal use).
// NOTE: This does NOT mark the resource as "recently accessed" for routing validation.
// Use MarkExplicitAccess() when the user explicitly selects/queries a single resource.
// This separation prevents bulk discovery operations from poisoning routing validation.
func (rc *ResolvedContext) AddResource(name string, res *ResolvedResource) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.addResourceInternal(name, res, false)
}

// AddResourceWithExplicitAccess adds a resource AND marks it as recently accessed.
// Use this for single-resource operations where user intent is clear (e.g., pulse_query get).
func (rc *ResolvedContext) AddResourceWithExplicitAccess(name string, res *ResolvedResource) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.addResourceInternal(name, res, true)
}

// addResourceInternal is the shared implementation for adding resources.
func (rc *ResolvedContext) addResourceInternal(name string, res *ResolvedResource, markExplicitAccess bool) {
	// Initialize maps if needed (e.g., after JSON deserialization)
	if rc.Resources == nil {
		rc.Resources = make(map[string]*ResolvedResource)
	}
	if rc.ResourcesByID == nil {
		rc.ResourcesByID = make(map[string]*ResolvedResource)
	}
	if rc.ResourcesByAlias == nil {
		rc.ResourcesByAlias = make(map[string]*ResolvedResource)
	}
	if rc.lastAccessed == nil {
		rc.lastAccessed = make(map[string]time.Time)
	}
	if rc.explicitlyAccessed == nil {
		rc.explicitlyAccessed = make(map[string]time.Time)
	}
	if rc.pinned == nil {
		rc.pinned = make(map[string]bool)
	}
	if rc.maxEntries == 0 {
		rc.maxEntries = DefaultResolvedContextMaxEntries
	}
	if rc.ttl == 0 {
		rc.ttl = DefaultResolvedContextTTL
	}

	// Evict expired and overflow before adding
	rc.evictExpired()

	rc.Resources[name] = res
	rc.ResourcesByID[res.ResourceID] = res
	// Index all aliases
	for _, alias := range res.Aliases {
		rc.ResourcesByAlias[strings.ToLower(alias)] = res
	}
	// Also index the primary name
	rc.ResourcesByAlias[strings.ToLower(res.Name)] = res

	// Always update lastAccessed for LRU/TTL tracking
	rc.touch(res.ResourceID)

	// Only mark explicit access if requested (single-resource operations)
	// This prevents bulk discovery from poisoning routing validation
	if markExplicitAccess {
		rc.explicitlyAccessed[res.ResourceID] = time.Now()
	}

	// Evict overflow after adding
	rc.evictOverflow()

	rc.LastUpdated = time.Now()
}

// MarkExplicitAccess marks a resource as explicitly accessed for routing validation.
//
// IMPORTANT: This method should ONLY be called from user-intent codepaths:
//
//	✅ User @mention resolution (service.go - prefetcher mentions)
//	✅ Single-resource pulse_query get (tools_query.go - registerResolvedResourceWithExplicitAccess)
//	✅ Explicit user selection of a specific resource
//
// DO NOT call from:
//
//	❌ Bulk discovery/search operations (pulse_query search, list)
//	❌ Background topology refresh
//	❌ Periodic prefetch scans
//	❌ Low-level discovery registration helpers
//
// This sets the explicitlyAccessed timestamp (separate from lastAccessed used for LRU).
// The routing validation (WasRecentlyAccessed) checks explicitlyAccessed, not lastAccessed.
//
// If this is called incorrectly from bulk operations, routing validation will produce
// false positives, blocking legitimate host operations.
func (rc *ResolvedContext) MarkExplicitAccess(resourceID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.explicitlyAccessed == nil {
		rc.explicitlyAccessed = make(map[string]time.Time)
	}
	rc.explicitlyAccessed[resourceID] = time.Now()
}

// AddResolvedResource implements tools.ResolvedContextProvider interface.
// Called by query/discovery tools to register discovered resources.
func (rc *ResolvedContext) AddResolvedResource(reg tools.ResourceRegistration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	// Build the canonical resource ID with scope for global uniqueness.
	// Format: {kind}:{host}:{provider_uid} for scoped resources
	//         {kind}:{provider_uid} for global resources (nodes, clusters)
	//
	// Examples:
	//   lxc:delly:141         (LXC container 141 on node delly)
	//   vm:minipc:203         (VM 203 on node minipc)
	//   docker_container:media-server:abc123  (Docker container on host media-server)
	//   node:delly            (Proxmox node - no parent scope)
	var resourceID string
	if reg.ProviderUID != "" {
		// Include host scope for resources that have a parent host
		if reg.HostUID != "" || reg.HostName != "" {
			hostScope := reg.HostUID
			if hostScope == "" {
				hostScope = reg.HostName
			}
			resourceID = reg.Kind + ":" + hostScope + ":" + reg.ProviderUID
		} else {
			// Global resources (nodes, clusters) don't need host scope
			resourceID = reg.Kind + ":" + reg.ProviderUID
		}
	} else {
		// Fallback to name-based ID for backwards compatibility
		if reg.HostUID != "" || reg.HostName != "" {
			hostScope := reg.HostUID
			if hostScope == "" {
				hostScope = reg.HostName
			}
			resourceID = reg.Kind + ":" + hostScope + ":" + reg.Name
		} else {
			resourceID = reg.Kind + ":" + reg.Name
		}
	}

	// Build display path from location chain
	var displayPath string
	if len(reg.LocationChain) > 0 {
		displayPath = strings.Join(reg.LocationChain, " @ ")
	}

	// Collect all allowed actions from executors
	actionSet := make(map[string]bool)
	var reachableVia []ExecutorPath
	for _, exec := range reg.Executors {
		for _, action := range exec.Actions {
			actionSet[action] = true
		}
		reachableVia = append(reachableVia, ExecutorPath{
			ExecutorID: exec.ExecutorID,
			Adapter:    exec.Adapter,
			Actions:    exec.Actions,
			Priority:   exec.Priority,
		})
	}
	var allowedActions []string
	for action := range actionSet {
		allowedActions = append(allowedActions, action)
	}

	// Get legacy routing from first/best executor
	var targetHost, agentID, adapter string
	if len(reg.Executors) > 0 {
		best := reg.Executors[0]
		for _, exec := range reg.Executors[1:] {
			if exec.Priority > best.Priority {
				best = exec
			}
		}
		targetHost = best.ExecutorID
		agentID = best.ExecutorID
		adapter = best.Adapter
	}

	// Build aliases list, ensuring name is included
	aliases := reg.Aliases
	if len(aliases) == 0 {
		aliases = []string{reg.Name}
	} else {
		// Ensure name is in aliases
		hasName := false
		for _, a := range aliases {
			if strings.EqualFold(a, reg.Name) {
				hasName = true
				break
			}
		}
		if !hasName {
			aliases = append([]string{reg.Name}, aliases...)
		}
	}

	res := &ResolvedResource{
		// Structured identity
		Kind:        reg.Kind,
		ProviderUID: reg.ProviderUID,
		Scope: ResourceScope{
			HostUID:    reg.HostUID,
			HostName:   reg.HostName,
			ParentUID:  reg.ParentUID,
			ParentKind: reg.ParentKind,
			ClusterUID: reg.ClusterUID,
			Namespace:  reg.Namespace,
		},
		Aliases: aliases,

		// Derived/display
		ResourceID:    resourceID,
		Name:          reg.Name,
		ResourceType:  reg.Kind, // backwards compat
		DisplayPath:   displayPath,
		LocationChain: reg.LocationChain,

		// Executor paths
		ReachableVia: reachableVia,

		// Legacy routing
		TargetHost: targetHost,
		AgentID:    agentID,
		Adapter:    adapter,

		// Resource-specific
		VMID: reg.VMID,
		Node: reg.Node,

		// Actions
		AllowedActions: allowedActions,

		// Timestamps
		ResolvedAt: time.Now(),
	}
	rc.addResourceInternal(reg.Name, res, false)
}

// GetResource retrieves a resource by name (case-insensitive)
// Updates LRU tracking on access.
func (rc *ResolvedContext) GetResource(name string) (*ResolvedResource, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.getResourceInternal(name)
}

// getResourceInternal is the lock-free implementation of GetResource.
func (rc *ResolvedContext) getResourceInternal(name string) (*ResolvedResource, bool) {
	// Evict expired on access
	rc.evictExpired()

	// Try exact match first
	if res, ok := rc.Resources[name]; ok {
		rc.touch(res.ResourceID)
		return res, true
	}
	// Try alias index
	if res, ok := rc.ResourcesByAlias[strings.ToLower(name)]; ok {
		rc.touch(res.ResourceID)
		return res, true
	}
	return nil, false
}

// GetResourceByID retrieves a resource by its canonical ResourceID (internal use)
// Updates LRU tracking on access.
func (rc *ResolvedContext) GetResourceByID(resourceID string) (*ResolvedResource, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.getResourceByIDInternal(resourceID)
}

// getResourceByIDInternal is the lock-free implementation of GetResourceByID.
func (rc *ResolvedContext) getResourceByIDInternal(resourceID string) (*ResolvedResource, bool) {
	// Evict expired on access
	rc.evictExpired()

	res, ok := rc.ResourcesByID[resourceID]
	if ok {
		rc.touch(resourceID)
	}
	return res, ok
}

// GetResolvedResourceByID implements tools.ResolvedContextProvider interface.
// Updates LRU tracking on access.
func (rc *ResolvedContext) GetResolvedResourceByID(resourceID string) (tools.ResolvedResourceInfo, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Evict expired on access
	rc.evictExpired()

	res, ok := rc.ResourcesByID[resourceID]
	if !ok {
		return nil, false
	}
	rc.touch(resourceID)
	return res, true
}

// GetResolvedResourceByAlias implements tools.ResolvedContextProvider interface.
// Retrieves a resource by any of its aliases (case-insensitive).
// Updates LRU tracking on access.
func (rc *ResolvedContext) GetResolvedResourceByAlias(alias string) (tools.ResolvedResourceInfo, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Evict expired on access
	rc.evictExpired()

	res, ok := rc.ResourcesByAlias[strings.ToLower(alias)]
	if !ok {
		return nil, false
	}
	rc.touch(res.ResourceID)
	return res, true
}

// ValidateResourceID checks if a resource ID exists in this context
// and returns the resource if valid
func (rc *ResolvedContext) ValidateResourceID(resourceID string) (*ResolvedResource, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.validateResourceIDInternal(resourceID)
}

// validateResourceIDInternal is the lock-free implementation of ValidateResourceID.
func (rc *ResolvedContext) validateResourceIDInternal(resourceID string) (*ResolvedResource, error) {
	res, ok := rc.getResourceByIDInternal(resourceID)
	if !ok {
		return nil, &ResourceNotResolvedError{ResourceID: resourceID}
	}
	return res, nil
}

// ValidateAction checks if an action is allowed for a resource (internal use)
func (rc *ResolvedContext) ValidateAction(resourceID, action string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.validateActionInternal(resourceID, action)
}

// validateActionInternal is the lock-free implementation of ValidateAction.
func (rc *ResolvedContext) validateActionInternal(resourceID, action string) error {
	res, err := rc.validateResourceIDInternal(resourceID)
	if err != nil {
		return err
	}

	// If no actions specified, allow all (backwards compatibility)
	if len(res.AllowedActions) == 0 {
		return nil
	}

	for _, allowed := range res.AllowedActions {
		if allowed == action || allowed == "*" {
			return nil
		}
	}

	return &ActionNotAllowedError{
		ResourceID: resourceID,
		Action:     action,
		Allowed:    res.AllowedActions,
	}
}

// ValidateResourceForAction implements tools.ResolvedContextProvider interface.
// Returns the resource if valid, error if not found or action not allowed.
func (rc *ResolvedContext) ValidateResourceForAction(resourceID, action string) (tools.ResolvedResourceInfo, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if err := rc.validateActionInternal(resourceID, action); err != nil {
		return nil, err
	}
	res, _ := rc.getResourceByIDInternal(resourceID)
	return res, nil
}

// ResourceNotResolvedError indicates a resource ID wasn't found in the context
type ResourceNotResolvedError struct {
	ResourceID string
}

func (e *ResourceNotResolvedError) Error() string {
	return "resource not resolved: " + e.ResourceID + " - use pulse_query first to discover this resource"
}

// ActionNotAllowedError indicates an action isn't permitted for a resource
type ActionNotAllowedError struct {
	ResourceID string
	Action     string
	Allowed    []string
}

func (e *ActionNotAllowedError) Error() string {
	return "action '" + e.Action + "' not allowed for resource " + e.ResourceID
}

// Max turns for the agentic loop to prevent infinite loops
const (
	MaxAgenticTurns         = 20
	DefaultStatelessContext = false
	MaxContextMessagesLimit = 40
	// MaxToolResultCharsLimit caps tool results sent to the LLM.
	// With context compaction in place, this safety net rarely fires.
	// 16K chars (~4K tokens) is sufficient for any single tool result.
	// The UI shows full results regardless of this limit.
	MaxToolResultCharsLimit = 16000
)

var (
	StatelessContext = DefaultStatelessContext
)
