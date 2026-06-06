package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog/log"
)

// SessionStore manages chat sessions persisted as JSON files
type SessionStore struct {
	mu      sync.RWMutex
	dataDir string

	// resolvedContexts holds per-session resolved resource contexts (in-memory only)
	// These are NOT persisted - resources should be re-resolved after restart
	// because infrastructure state may have changed
	resolvedContexts map[string]*ResolvedContext

	// sessionFSMs holds per-session workflow state machines (in-memory only)
	// These track the RESOLVING -> READING -> WRITING -> VERIFYING workflow
	// to ensure structural guarantees (must discover before write, verify after write)
	sessionFSMs map[string]*SessionFSM

	// sessionToolSets holds per-session tool allowlists (in-memory only).
	// These keep tool availability stable across turns while allowing additive expansion.
	sessionToolSets map[string]map[string]bool

	// knowledgeAccumulators holds per-session knowledge accumulators (in-memory only).
	// These extract and preserve key facts from tool results to prevent amnesia
	// when old tool results are compacted from the conversation context.
	knowledgeAccumulators map[string]*KnowledgeAccumulator
}

// sessionData is the on-disk format for a session
type sessionData struct {
	ID           string               `json:"id"`
	Title        string               `json:"title"`
	Messages     []Message            `json:"messages"`
	ModelContext *sessionModelContext `json:"model_context,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

type sessionModelContext struct {
	HandoffFindingID string            `json:"handoff_finding_id,omitempty"`
	HandoffContext   string            `json:"handoff_context,omitempty"`
	HandoffResources []HandoffResource `json:"handoff_resources,omitempty"`
	HandoffActions   []HandoffAction   `json:"handoff_actions,omitempty"`
	HandoffMetadata  HandoffMetadata   `json:"handoff_metadata,omitempty"`
	UpdatedAt        time.Time         `json:"updated_at,omitempty"`
}

func normalizeHandoffResources(resources []HandoffResource) []HandoffResource {
	if len(resources) == 0 {
		return nil
	}

	normalized := make([]HandoffResource, 0, len(resources))
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		resource.ID = strings.TrimSpace(resource.ID)
		resource.Name = strings.TrimSpace(resource.Name)
		resource.Type = strings.TrimSpace(resource.Type)
		resource.Node = strings.TrimSpace(resource.Node)
		if resource.ID == "" && resource.Name == "" {
			continue
		}
		key := strings.ToLower(resource.Type + "\x00" + resource.ID + "\x00" + resource.Name + "\x00" + resource.Node)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, resource)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeHandoffActions(actions []HandoffAction) []HandoffAction {
	if len(actions) == 0 {
		return nil
	}

	normalized := make([]HandoffAction, 0, len(actions))
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		action.FindingID = strings.TrimSpace(action.FindingID)
		action.RecordID = strings.TrimSpace(action.RecordID)
		action.ApprovalID = strings.TrimSpace(action.ApprovalID)
		action.ApprovalStatus = strings.TrimSpace(action.ApprovalStatus)
		action.ApprovalRequestedAt = strings.TrimSpace(action.ApprovalRequestedAt)
		action.ApprovalExpiresAt = strings.TrimSpace(action.ApprovalExpiresAt)
		action.ApprovalDecidedAt = strings.TrimSpace(action.ApprovalDecidedAt)
		action.ActionID = strings.TrimSpace(action.ActionID)
		action.ActionState = strings.TrimSpace(action.ActionState)
		action.ActionUpdatedAt = strings.TrimSpace(action.ActionUpdatedAt)
		action.ActionRequestedBy = strings.TrimSpace(action.ActionRequestedBy)
		action.ActionCapability = strings.TrimSpace(action.ActionCapability)
		action.ActionApprovalPolicy = strings.TrimSpace(action.ActionApprovalPolicy)
		action.ActionPlanExpiresAt = strings.TrimSpace(action.ActionPlanExpiresAt)
		action.ActionPlanMessage = strings.TrimSpace(action.ActionPlanMessage)
		action.ActionPreflight = strings.TrimSpace(action.ActionPreflight)
		action.ActionDryRunSummary = strings.TrimSpace(action.ActionDryRunSummary)
		action.ActionResult = strings.TrimSpace(action.ActionResult)
		action.FixID = strings.TrimSpace(action.FixID)
		action.Description = strings.TrimSpace(action.Description)
		action.RiskLevel = strings.TrimSpace(action.RiskLevel)
		action.TargetHost = strings.TrimSpace(action.TargetHost)
		action.TargetResourceID = strings.TrimSpace(action.TargetResourceID)
		action.TargetResourceName = strings.TrimSpace(action.TargetResourceName)
		action.TargetResourceType = strings.TrimSpace(action.TargetResourceType)
		action.TargetNode = strings.TrimSpace(action.TargetNode)
		if action.ApprovalID == "" && action.ActionID == "" && action.FixID == "" && action.Description == "" && action.FindingID == "" {
			continue
		}
		key := strings.ToLower(action.FindingID + "\x00" + action.RecordID + "\x00" + action.ApprovalID + "\x00" + action.ActionID + "\x00" + action.FixID + "\x00" + action.Description + "\x00" + action.TargetResourceID + "\x00" + action.TargetHost)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, action)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func trimHandoffMetadataField(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxRunes]))
}

// NormalizeHandoffMetadata returns the browser-safe subset of product-originated
// handoff identity that can be persisted and exposed in session summaries.
func NormalizeHandoffMetadata(metadata HandoffMetadata) HandoffMetadata {
	kind := strings.ToLower(trimHandoffMetadataField(metadata.Kind, 64))
	switch kind {
	case sessionHandoffKindPatrolAssessment,
		sessionHandoffKindPatrolConfigurationFailure,
		sessionHandoffKindPatrolFinding,
		sessionHandoffKindPatrolRun,
		sessionHandoffKindResourceContext:
	default:
		return HandoffMetadata{}
	}

	normalized := HandoffMetadata{
		Kind:           kind,
		RunID:          trimHandoffMetadataField(metadata.RunID, 256),
		RunType:        trimHandoffMetadataField(metadata.RunType, 128),
		RunStatus:      trimHandoffMetadataField(metadata.RunStatus, 128),
		RuntimeFailure: metadata.RuntimeFailure,
	}
	if normalized.Kind == sessionHandoffKindPatrolRun && normalized.RunID == "" {
		return HandoffMetadata{}
	}
	if normalized.Kind != sessionHandoffKindPatrolRun {
		normalized.RunID = ""
		normalized.RunType = ""
		normalized.RunStatus = ""
	}
	if normalized.Kind != sessionHandoffKindPatrolRun && normalized.Kind != sessionHandoffKindPatrolConfigurationFailure {
		normalized.RuntimeFailure = false
	}
	return normalized
}

func handoffMetadataEmpty(metadata HandoffMetadata) bool {
	return NormalizeHandoffMetadata(metadata) == (HandoffMetadata{})
}

func inferPatrolRunHandoffMetadata(handoffContext string) HandoffMetadata {
	lines := strings.Split(strings.TrimSpace(handoffContext), "\n")
	if len(lines) == 0 {
		return HandoffMetadata{}
	}

	var metadata HandoffMetadata
	sawRunContext := false
	sawRunHistorySource := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "[Patrol Run Context]" {
			sawRunContext = true
			continue
		}
		if !sawRunContext {
			continue
		}

		label, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		label = strings.ToLower(strings.TrimSpace(label))
		value = strings.TrimSpace(value)
		switch label {
		case "source":
			if strings.EqualFold(value, "Pulse Patrol run history") {
				sawRunHistorySource = true
			}
		case "run id":
			metadata.RunID = value
		case "run type":
			metadata.RunType = value
		case "status":
			metadata.RunStatus = value
		case "runtime failure":
			metadata.RuntimeFailure = value != ""
		}
	}

	if !sawRunContext || !sawRunHistorySource || strings.TrimSpace(metadata.RunID) == "" {
		return HandoffMetadata{}
	}
	metadata.Kind = sessionHandoffKindPatrolRun
	return NormalizeHandoffMetadata(metadata)
}

func modelContextEmpty(modelContext *sessionModelContext) bool {
	if modelContext == nil {
		return true
	}
	return strings.TrimSpace(modelContext.HandoffFindingID) == "" &&
		strings.TrimSpace(modelContext.HandoffContext) == "" &&
		len(normalizeHandoffResources(modelContext.HandoffResources)) == 0 &&
		len(normalizeHandoffActions(modelContext.HandoffActions)) == 0 &&
		handoffMetadataEmpty(modelContext.HandoffMetadata)
}

func cloneSessionModelContext(modelContext *sessionModelContext) *sessionModelContext {
	if modelContext == nil {
		return nil
	}
	clone := *modelContext
	if len(modelContext.HandoffResources) > 0 {
		clone.HandoffResources = append([]HandoffResource(nil), modelContext.HandoffResources...)
	}
	if len(modelContext.HandoffActions) > 0 {
		clone.HandoffActions = append([]HandoffAction(nil), modelContext.HandoffActions...)
	}
	return &clone
}

const (
	sessionHandoffKindPatrolAssessment           = "patrol_assessment"
	sessionHandoffKindPatrolConfigurationFailure = "patrol_configuration_failure"
	sessionHandoffKindPatrolFinding              = "patrol_finding"
	sessionHandoffKindPatrolRun                  = "patrol_run"
	sessionHandoffKindResourceContext            = "resource_context"
	sessionHandoffKindScopedContext              = "scoped_context"
)

func handoffActionCurrentlyRequiresApproval(action HandoffAction) bool {
	approvalStatus := strings.ToLower(strings.TrimSpace(action.ApprovalStatus))
	actionState := strings.ToLower(strings.TrimSpace(action.ActionState))

	if approvalStatus == "pending" && !action.ApprovalConsumed {
		return true
	}
	switch actionState {
	case "pending_approval", "awaiting_approval":
		return approvalStatus == "" || approvalStatus == "pending"
	case "approved", "rejected", "executing", "completed", "failed", "planned":
		return false
	}
	if approvalStatus != "" || strings.TrimSpace(action.ApprovalID) != "" {
		return false
	}
	return action.ActionRequiresApproval
}

func modelContextHandoffSummary(modelContext *sessionModelContext) *SessionHandoffSummary {
	if modelContextEmpty(modelContext) {
		return nil
	}

	resources := normalizeHandoffResources(modelContext.HandoffResources)
	actions := normalizeHandoffActions(modelContext.HandoffActions)
	metadata := NormalizeHandoffMetadata(modelContext.HandoffMetadata)
	if handoffMetadataEmpty(metadata) {
		metadata = inferPatrolRunHandoffMetadata(modelContext.HandoffContext)
	}
	findingID := strings.TrimSpace(modelContext.HandoffFindingID)
	if findingID == "" && metadata.Kind == "" {
		for _, action := range actions {
			if strings.TrimSpace(action.FindingID) != "" {
				findingID = strings.TrimSpace(action.FindingID)
				break
			}
		}
	}

	kind := sessionHandoffKindScopedContext
	if metadata.Kind != "" {
		kind = metadata.Kind
	} else if findingID != "" {
		kind = sessionHandoffKindPatrolFinding
	}

	summary := &SessionHandoffSummary{
		Kind:            kind,
		FindingID:       findingID,
		RunID:           metadata.RunID,
		RunType:         metadata.RunType,
		RunStatus:       metadata.RunStatus,
		RuntimeFailure:  metadata.RuntimeFailure,
		HasModelContext: strings.TrimSpace(modelContext.HandoffContext) != "",
		ResourceCount:   len(resources),
		ActionCount:     len(actions),
	}
	if kind != sessionHandoffKindPatrolRun {
		summary.RunID = ""
		summary.RunType = ""
		summary.RunStatus = ""
	}
	if kind != sessionHandoffKindPatrolRun && kind != sessionHandoffKindPatrolConfigurationFailure {
		summary.RuntimeFailure = false
	}
	if !modelContext.UpdatedAt.IsZero() {
		updatedAt := modelContext.UpdatedAt
		summary.UpdatedAt = &updatedAt
	}
	if len(resources) > 0 {
		primaryResource := resources[0]
		summary.PrimaryResource = &primaryResource
	}
	for _, action := range actions {
		if !summary.RequiresApproval && handoffActionCurrentlyRequiresApproval(action) {
			summary.RequiresApproval = true
		}
		if summary.LastKnownApprovalStatus == "" {
			summary.LastKnownApprovalStatus = strings.TrimSpace(action.ApprovalStatus)
		}
		if summary.LastKnownActionState == "" {
			summary.LastKnownActionState = strings.TrimSpace(action.ActionState)
		}
		if summary.LastKnownActionRisk == "" {
			summary.LastKnownActionRisk = strings.TrimSpace(action.RiskLevel)
		}
	}

	return summary
}

const (
	maxSessionIDLength   = 128
	maxSessionTitleRunes = 120
)

var errSessionNotFound = errors.New("session not found")

// NewSessionStore creates a new session store
func NewSessionStore(dataDir string) (*SessionStore, error) {
	sessionsDir := filepath.Join(dataDir, "ai_sessions")
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &SessionStore{
		dataDir:               sessionsDir,
		resolvedContexts:      make(map[string]*ResolvedContext),
		sessionFSMs:           make(map[string]*SessionFSM),
		sessionToolSets:       make(map[string]map[string]bool),
		knowledgeAccumulators: make(map[string]*KnowledgeAccumulator),
	}, nil
}

// sessionPath returns the file path for a session
func (s *SessionStore) sessionPath(id string) (string, error) {
	if err := validateSessionID(id); err != nil {
		return "", err
	}
	return securityutil.JoinStorageLeaf(s.dataDir, securityutil.HashedStorageName(id)+".json")
}

func (s *SessionStore) directLegacySessionPath(id string) (string, error) {
	if err := validateSessionID(id); err != nil {
		return "", err
	}
	return securityutil.JoinStorageLeaf(s.dataDir, id+".json")
}

func (s *SessionStore) findLegacySessionPath(id string) (string, error) {
	if err := validateSessionID(id); err != nil {
		return "", err
	}
	canonicalPath, err := s.sessionPath(id)
	if err != nil {
		return "", err
	}
	directPath, err := s.directLegacySessionPath(id)
	if err != nil {
		return "", err
	}
	if directPath != canonicalPath {
		if _, err := os.Stat(directPath); err == nil {
			return directPath, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to stat legacy session path: %w", err)
		}
	}
	canonicalName := securityutil.HashedStorageName(id) + ".json"
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to scan session directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if entry.Name() == canonicalName {
			continue
		}
		path, err := securityutil.JoinStorageLeaf(s.dataDir, entry.Name())
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to resolve legacy session candidate path")
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to read legacy session candidate")
			continue
		}
		var session sessionData
		if err := json.Unmarshal(data, &session); err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to parse legacy session candidate")
			continue
		}
		if session.ID == id {
			return path, nil
		}
	}
	return "", nil
}

// List returns all sessions, sorted by updated_at descending
func (s *SessionStore) List() ([]Session, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path, err := securityutil.JoinStorageLeaf(s.dataDir, entry.Name())
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to resolve session file path")
			continue
		}

		file, err := os.ReadFile(path)
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to read session file")
			continue
		}
		var data sessionData
		if err := json.Unmarshal(file, &data); err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to parse session file")
			continue
		}
		for i := range data.Messages {
			data.Messages[i] = data.Messages[i].NormalizeCollections()
		}

		sessions = append(sessions, Session{
			ID:             data.ID,
			Title:          data.Title,
			CreatedAt:      data.CreatedAt,
			UpdatedAt:      data.UpdatedAt,
			MessageCount:   len(data.Messages),
			HandoffSummary: modelContextHandoffSummary(data.ModelContext),
		})
	}

	// Sort by updated_at descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Create creates a new session
func (s *SessionStore) Create() (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	data := sessionData{
		ID:        uuid.New().String(),
		Title:     "",
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.writeSession(data); err != nil {
		return nil, err
	}

	return &Session{
		ID:        data.ID,
		Title:     data.Title,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

// Get retrieves a session by ID
func (s *SessionStore) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:             data.ID,
		Title:          data.Title,
		CreatedAt:      data.CreatedAt,
		UpdatedAt:      data.UpdatedAt,
		MessageCount:   len(data.Messages),
		HandoffSummary: modelContextHandoffSummary(data.ModelContext),
	}, nil
}

// Rename updates a session title without touching messages or handoff context.
func (s *SessionStore) Rename(id, title string) (*Session, error) {
	normalizedTitle := normalizeSessionTitle(title)
	if normalizedTitle == "" {
		return nil, fmt.Errorf("session title required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}

	data.Title = normalizedTitle
	data.UpdatedAt = time.Now()
	if err := s.writeSession(*data); err != nil {
		return nil, err
	}

	return &Session{
		ID:             data.ID,
		Title:          data.Title,
		CreatedAt:      data.CreatedAt,
		UpdatedAt:      data.UpdatedAt,
		MessageCount:   len(data.Messages),
		HandoffSummary: modelContextHandoffSummary(data.ModelContext),
	}, nil
}

// Fork clones a persisted session into a new durable session. The copied
// messages intentionally preserve their per-session IDs so tool-call/result
// relationships remain intact inside the forked transcript.
func (s *SessionStore) Fork(id string) (*Session, error) {
	if err := validateSessionID(id); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	source, err := s.readSession(id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	messages := make([]Message, len(source.Messages))
	for i, msg := range source.Messages {
		messages[i] = msg.NormalizeCollections()
	}

	title := strings.TrimSpace(source.Title)
	if title == "" {
		for _, msg := range messages {
			if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
				title = generateTitle(msg.Content)
				break
			}
		}
	}
	if title == "" {
		title = "Forked session"
	} else if !strings.HasPrefix(strings.ToLower(title), "fork of ") {
		title = "Fork of " + title
	}

	fork := sessionData{
		ID:           uuid.New().String(),
		Title:        title,
		Messages:     messages,
		ModelContext: cloneSessionModelContext(source.ModelContext),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.writeSession(fork); err != nil {
		return nil, err
	}

	return &Session{
		ID:             fork.ID,
		Title:          fork.Title,
		CreatedAt:      fork.CreatedAt,
		UpdatedAt:      fork.UpdatedAt,
		MessageCount:   len(fork.Messages),
		HandoffSummary: modelContextHandoffSummary(fork.ModelContext),
	}, nil
}

// Delete deletes a session
func (s *SessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.sessionPath(id)
	if err != nil {
		return err
	}
	var removed bool
	candidates := []string{path}
	legacyPath, err := s.findLegacySessionPath(id)
	if err != nil {
		return err
	}
	if legacyPath != "" && legacyPath != path {
		candidates = append(candidates, legacyPath)
	}
	for _, candidate := range candidates {
		if err := os.Remove(candidate); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to delete session: %w", err)
		}
		removed = true
	}
	if !removed {
		return fmt.Errorf("session not found: %s", id)
	}

	// Also clean up resolved context, FSM, and knowledge accumulator
	delete(s.resolvedContexts, id)
	delete(s.sessionFSMs, id)
	delete(s.sessionToolSets, id)
	delete(s.knowledgeAccumulators, id)

	return nil
}

// GetMessages retrieves all messages for a session
func (s *SessionStore) GetMessages(id string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}

	return data.Messages, nil
}

// TrimMessages keeps at most keepMostRecent messages in the session,
// dropping older ones. Used for sessions like patrol-main that are
// reused indefinitely across scheduled runs and would otherwise grow
// unbounded — at the default 3-hour Patrol cadence with ~20 messages
// per run, the file grew to 16 MB / 3,593 messages before this bound
// existed, and every AddMessage was rewriting the whole file to disk.
//
// keepMostRecent <= 0 is treated as a no-op so callers can disable the
// bound by passing 0 when they want full retention (e.g. user-driven
// chat sessions where conversation history is the product).
func (s *SessionStore) TrimMessages(id string, keepMostRecent int) error {
	if keepMostRecent <= 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}
	if len(data.Messages) <= keepMostRecent {
		return nil
	}
	start := len(data.Messages) - keepMostRecent
	trimmed := make([]Message, keepMostRecent)
	copy(trimmed, data.Messages[start:])
	data.Messages = trimmed
	data.UpdatedAt = time.Now()
	return s.writeSession(*data)
}

// AddMessage adds a message to a session
func (s *SessionStore) AddMessage(id string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	// Generate message ID if not set
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	msg = msg.NormalizeCollections()

	data.Messages = append(data.Messages, msg)
	data.UpdatedAt = time.Now()

	// Auto-generate title from first user message if not set
	if data.Title == "" && msg.Role == "user" && msg.Content != "" {
		data.Title = generateTitle(msg.Content)
	}

	return s.writeSession(*data)
}

// SetModelHandoffFindingID stores the product-originated finding reference for
// follow-up turns. The reference lets API handlers refresh the current Patrol
// context without treating the finding as user-authored chat text.
func (s *SessionStore) SetModelHandoffFindingID(id, findingID string) error {
	findingID = strings.TrimSpace(findingID)
	if findingID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	now := time.Now()
	if data.ModelContext == nil {
		data.ModelContext = &sessionModelContext{}
	}
	data.ModelContext.HandoffFindingID = findingID
	data.ModelContext.UpdatedAt = now
	data.UpdatedAt = now

	return s.writeSession(*data)
}

// SetModelHandoffEnvelope replaces the session's product-originated handoff
// as one coherent scope. This avoids stale finding, run, resource, or action
// identity leaking between separate handoffs within the same chat session.
func (s *SessionStore) SetModelHandoffEnvelope(id string, findingID string, handoffContext string, handoffResources []HandoffResource, handoffActions []HandoffAction, handoffMetadata HandoffMetadata) error {
	findingID = strings.TrimSpace(findingID)
	handoffContext = strings.TrimSpace(handoffContext)
	resources := normalizeHandoffResources(handoffResources)
	actions := normalizeHandoffActions(handoffActions)
	metadata := NormalizeHandoffMetadata(handoffMetadata)

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	now := time.Now()
	modelContext := &sessionModelContext{
		HandoffFindingID: findingID,
		HandoffContext:   handoffContext,
		HandoffResources: resources,
		HandoffActions:   actions,
		HandoffMetadata:  metadata,
		UpdatedAt:        now,
	}
	if modelContextEmpty(modelContext) {
		data.ModelContext = nil
	} else {
		data.ModelContext = modelContext
	}
	data.UpdatedAt = now

	return s.writeSession(*data)
}

// SetModelHandoffContext stores model-only handoff context for future turns.
// It is intentionally session metadata, not a user-authored chat message.
func (s *SessionStore) SetModelHandoffContext(id, handoffContext string) error {
	handoffContext = strings.TrimSpace(handoffContext)
	if handoffContext == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	now := time.Now()
	if data.ModelContext == nil {
		data.ModelContext = &sessionModelContext{}
	}
	data.ModelContext.HandoffContext = handoffContext
	data.ModelContext.UpdatedAt = now
	data.UpdatedAt = now

	return s.writeSession(*data)
}

// SetModelHandoffResources stores product-originated resource references for
// future turns. These references are not authority by themselves; chat execution
// re-resolves them through the canonical unified-resource model before use.
func (s *SessionStore) SetModelHandoffResources(id string, handoffResources []HandoffResource) error {
	resources := normalizeHandoffResources(handoffResources)

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	now := time.Now()
	if data.ModelContext == nil {
		data.ModelContext = &sessionModelContext{}
	}
	data.ModelContext.HandoffResources = resources
	data.ModelContext.UpdatedAt = now
	if modelContextEmpty(data.ModelContext) {
		data.ModelContext = nil
	}
	data.UpdatedAt = now

	return s.writeSession(*data)
}

// SetModelHandoffActions stores product-originated pending action references
// for future turns. These references are not executable authority and must not
// contain raw command text.
func (s *SessionStore) SetModelHandoffActions(id string, handoffActions []HandoffAction) error {
	actions := normalizeHandoffActions(handoffActions)

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	now := time.Now()
	if data.ModelContext == nil {
		data.ModelContext = &sessionModelContext{}
	}
	data.ModelContext.HandoffActions = actions
	data.ModelContext.UpdatedAt = now
	if modelContextEmpty(data.ModelContext) {
		data.ModelContext = nil
	}
	data.UpdatedAt = now

	return s.writeSession(*data)
}

// SetModelHandoffMetadata stores browser-safe handoff identity for future
// session summaries without exposing private model context details.
func (s *SessionStore) SetModelHandoffMetadata(id string, handoffMetadata HandoffMetadata) error {
	metadata := NormalizeHandoffMetadata(handoffMetadata)
	if handoffMetadataEmpty(metadata) {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	now := time.Now()
	if data.ModelContext == nil {
		data.ModelContext = &sessionModelContext{}
	}
	data.ModelContext.HandoffMetadata = metadata
	data.ModelContext.UpdatedAt = now
	data.UpdatedAt = now

	return s.writeSession(*data)
}

// GetModelHandoffFindingID returns the stored product-originated finding
// reference for a session.
func (s *SessionStore) GetModelHandoffFindingID(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return "", err
	}
	if data.ModelContext == nil {
		return "", nil
	}
	return strings.TrimSpace(data.ModelContext.HandoffFindingID), nil
}

// GetModelHandoffMetadata returns the browser-safe handoff identity stored for
// a session.
func (s *SessionStore) GetModelHandoffMetadata(id string) (HandoffMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return HandoffMetadata{}, err
	}
	if data.ModelContext == nil {
		return HandoffMetadata{}, nil
	}
	return NormalizeHandoffMetadata(data.ModelContext.HandoffMetadata), nil
}

// GetModelHandoffContext returns model-only handoff context for a session.
func (s *SessionStore) GetModelHandoffContext(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return "", err
	}
	if data.ModelContext == nil {
		return "", nil
	}
	return strings.TrimSpace(data.ModelContext.HandoffContext), nil
}

// GetModelHandoffResources returns stored handoff resource references for a
// session. Callers must rehydrate them through canonical resource registration
// before using them for action validation.
func (s *SessionStore) GetModelHandoffResources(id string) ([]HandoffResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}
	if data.ModelContext == nil {
		return nil, nil
	}
	return normalizeHandoffResources(data.ModelContext.HandoffResources), nil
}

// GetModelHandoffActions returns stored product-originated pending action
// references for a session. Callers must treat them as review context only.
func (s *SessionStore) GetModelHandoffActions(id string) ([]HandoffAction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}
	if data.ModelContext == nil {
		return nil, nil
	}
	return normalizeHandoffActions(data.ModelContext.HandoffActions), nil
}

// GetModelHandoffEnvelope returns the persisted model-only handoff fields in
// one read. Send paths use this instead of several independent metadata reads
// so large stores do not pay repeated session-file I/O before the model starts.
func (s *SessionStore) GetModelHandoffEnvelope(id string) (string, []HandoffResource, []HandoffAction, HandoffMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSessionFast(id)
	if err != nil {
		return "", nil, nil, HandoffMetadata{}, err
	}
	if data.ModelContext == nil {
		return "", nil, nil, HandoffMetadata{}, nil
	}
	return strings.TrimSpace(data.ModelContext.HandoffContext),
		normalizeHandoffResources(data.ModelContext.HandoffResources),
		normalizeHandoffActions(data.ModelContext.HandoffActions),
		NormalizeHandoffMetadata(data.ModelContext.HandoffMetadata),
		nil
}

func (s *SessionStore) clearModelHandoffContextLocked(id string) error {
	data, err := s.readSession(id)
	if err != nil {
		return err
	}
	if modelContextEmpty(data.ModelContext) {
		return nil
	}

	data.ModelContext = nil
	data.UpdatedAt = time.Now()
	return s.writeSession(*data)
}

// ClearModelHandoffContext removes product-originated model-only handoff
// metadata while leaving the user-authored message history intact.
func (s *SessionStore) ClearModelHandoffContext(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.clearModelHandoffContextLocked(id)
}

// UpdateLastMessage updates the last message in a session (for streaming updates)
func (s *SessionStore) UpdateLastMessage(id string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	if len(data.Messages) == 0 {
		return fmt.Errorf("no messages to update")
	}

	data.Messages[len(data.Messages)-1] = msg.NormalizeCollections()
	data.UpdatedAt = time.Now()

	return s.writeSession(*data)
}

// readSession reads a session from disk (caller must hold lock).
func (s *SessionStore) readSession(id string) (*sessionData, error) {
	return s.readSessionWithLegacyScan(id, true)
}

// readSessionFast reads canonical sessions and direct <id>.json legacy sessions
// without scanning the full session directory. Use it on create/send hot paths
// where a missing session usually means "create a new one".
func (s *SessionStore) readSessionFast(id string) (*sessionData, error) {
	return s.readSessionWithLegacyScan(id, false)
}

func (s *SessionStore) readSessionWithLegacyScan(id string, allowLegacyScan bool) (*sessionData, error) {
	path, err := s.sessionPath(id)
	if err != nil {
		return nil, err
	}
	file, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		directLegacyPath, directLegacyErr := s.directLegacySessionPath(id)
		if directLegacyErr != nil {
			return nil, directLegacyErr
		}
		if directLegacyPath != path {
			file, err = os.ReadFile(directLegacyPath)
		}
		if os.IsNotExist(err) && allowLegacyScan {
			legacyPath, legacyErr := s.findLegacySessionPath(id)
			if legacyErr != nil {
				return nil, legacyErr
			}
			if legacyPath != "" && legacyPath != directLegacyPath {
				file, err = os.ReadFile(legacyPath)
			}
		}
		if os.IsNotExist(err) {
			return nil, sessionNotFoundError(id)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	var data sessionData
	if err := json.Unmarshal(file, &data); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}
	for i := range data.Messages {
		data.Messages[i] = data.Messages[i].NormalizeCollections()
	}

	return &data, nil
}

// writeSession writes a session to disk (caller must hold lock)
func (s *SessionStore) writeSession(data sessionData) error {
	if err := validateSessionID(data.ID); err != nil {
		return err
	}
	for i := range data.Messages {
		data.Messages[i] = data.Messages[i].NormalizeCollections()
	}

	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	path, err := s.sessionPath(data.ID)
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-")
	if err != nil {
		return fmt.Errorf("failed to create session temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to secure session temp file: %w", err)
	}
	if _, err := tmpFile.Write(file); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write session temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close session temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}
	cleanupTemp = false
	if legacyPath, err := s.directLegacySessionPath(data.ID); err == nil && legacyPath != "" && legacyPath != path {
		_ = os.Remove(legacyPath)
	}

	return nil
}

// generateTitle creates a session title from the first user message
func generateTitle(content string) string {
	// Clean up the content
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", " ")

	// Collapse multiple spaces
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	const maxLen = 50

	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}

	// Find a good break point
	truncated := string(runes[:maxLen])
	lastSpace := strings.LastIndex(truncated, " ")

	if lastSpace > 20 {
		return truncated[:lastSpace] + "..."
	}

	return truncated + "..."
}

func normalizeSessionTitle(title string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	runes := []rune(normalized)
	if len(runes) <= maxSessionTitleRunes {
		return normalized
	}
	return string(runes[:maxSessionTitleRunes])
}

// EnsureSession ensures a session exists, creating one if needed
func (s *SessionStore) EnsureSession(id string) (*Session, error) {
	if id == "" {
		return s.Create()
	}
	if err := validateSessionID(id); err != nil {
		return nil, err
	}

	s.mu.RLock()
	data, err := s.readSessionFast(id)
	s.mu.RUnlock()
	if err == nil {
		return &Session{
			ID:        data.ID,
			Title:     data.Title,
			CreatedAt: data.CreatedAt,
			UpdatedAt: data.UpdatedAt,
		}, nil
	}
	if !errors.Is(err, errSessionNotFound) {
		return nil, err
	}

	// Session doesn't exist, create it with the specified ID. Re-check under the
	// write lock so concurrent callers do not race into duplicate writes.
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err = s.readSessionFast(id)
	if err == nil {
		return &Session{
			ID:        data.ID,
			Title:     data.Title,
			CreatedAt: data.CreatedAt,
			UpdatedAt: data.UpdatedAt,
		}, nil
	}
	if !errors.Is(err, errSessionNotFound) {
		return nil, err
	}

	now := time.Now()
	data = &sessionData{
		ID:        id,
		Title:     "",
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.writeSession(*data); err != nil {
		return nil, err
	}

	return &Session{
		ID:        data.ID,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

func validateSessionID(id string) error {
	if id == "" {
		return fmt.Errorf("invalid session id: cannot be empty")
	}
	if len(id) > maxSessionIDLength {
		return fmt.Errorf("invalid session id: too long")
	}
	for _, r := range id {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if isLower || isUpper || isDigit || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("invalid session id: only letters, numbers, '-' and '_' are allowed")
	}
	return nil
}

func sessionNotFoundError(id string) error {
	return fmt.Errorf("%w: %s", errSessionNotFound, id)
}

// GetResolvedContext returns the resolved context for a session, creating one if needed
func (s *SessionStore) GetResolvedContext(sessionID string) *ResolvedContext {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.resolvedContexts[sessionID]
	if !ok {
		ctx = NewResolvedContext(sessionID)
		s.resolvedContexts[sessionID] = ctx
	}
	return ctx
}

// GetSessionFSM returns the workflow FSM for a session, creating one if needed
func (s *SessionStore) GetSessionFSM(sessionID string) *SessionFSM {
	s.mu.Lock()
	defer s.mu.Unlock()

	fsm, ok := s.sessionFSMs[sessionID]
	if !ok {
		fsm = NewSessionFSM()
		s.sessionFSMs[sessionID] = fsm
	}
	return fsm
}

// GetKnowledgeAccumulator returns the knowledge accumulator for a session, creating one if needed.
// For user chat sessions, this persists across messages (facts accumulate during a conversation).
func (s *SessionStore) GetKnowledgeAccumulator(sessionID string) *KnowledgeAccumulator {
	s.mu.Lock()
	defer s.mu.Unlock()

	ka, ok := s.knowledgeAccumulators[sessionID]
	if !ok {
		ka = NewKnowledgeAccumulator()
		s.knowledgeAccumulators[sessionID] = ka
	}
	return ka
}

// NewKnowledgeAccumulatorForRun creates a fresh KA for a patrol run.
// Unlike GetKnowledgeAccumulator (which reuses a session-scoped KA),
// this always returns a new instance to avoid stale facts from prior runs.
func (s *SessionStore) NewKnowledgeAccumulatorForRun(sessionID string) *KnowledgeAccumulator {
	s.mu.Lock()
	defer s.mu.Unlock()

	ka := NewKnowledgeAccumulator()
	s.knowledgeAccumulators[sessionID] = ka
	return ka
}

// ResetSessionFSM resets the FSM for a session (e.g., after context clear)
func (s *SessionStore) ResetSessionFSM(sessionID string, keepProgress bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fsm, ok := s.sessionFSMs[sessionID]
	if ok {
		if keepProgress {
			fsm.ResetKeepProgress()
		} else {
			fsm.Reset()
		}
	}
}

// AddResolvedResource adds a resolved resource to a session's context
func (s *SessionStore) AddResolvedResource(sessionID, name string, res *ResolvedResource) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.resolvedContexts[sessionID]
	if !ok {
		ctx = NewResolvedContext(sessionID)
		s.resolvedContexts[sessionID] = ctx
	}
	ctx.AddResource(name, res)

	log.Debug().
		Str("session_id", sessionID).
		Str("name", name).
		Str("resource_id", res.ResourceID).
		Str("resource_type", res.ResourceType).
		Str("target_host", res.TargetHost).
		Msg("[SessionStore] Added resolved resource to context")
}

// ValidateResourceForAction validates that a resource can perform an action
// Returns the resolved resource if valid, error if not
func (s *SessionStore) ValidateResourceForAction(sessionID, resourceID, action string) (*ResolvedResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ctx, ok := s.resolvedContexts[sessionID]
	if !ok {
		return nil, &ResourceNotResolvedError{ResourceID: resourceID}
	}

	if err := ctx.ValidateAction(resourceID, action); err != nil {
		return nil, err
	}

	res, _ := ctx.GetResourceByID(resourceID)
	return res, nil
}

// ClearResolvedContext removes the resolved context for a session
func (s *SessionStore) ClearResolvedContext(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.resolvedContexts, sessionID)
}

// ClearSessionState clears both resolved context and FSM coherently.
// This is the preferred method when clearing session state.
// - keepPinned=false: Full reset (RESOLVING state, no resources)
// - keepPinned=true: Keep pinned resources, FSM stays in READING if resources exist
func (s *SessionStore) ClearSessionState(sessionID string, keepPinned bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear resolved context
	ctx, hasCtx := s.resolvedContexts[sessionID]
	if hasCtx {
		ctx.Clear(keepPinned)
	}

	if !keepPinned {
		delete(s.sessionToolSets, sessionID)
		delete(s.knowledgeAccumulators, sessionID)
		if err := s.clearModelHandoffContextLocked(sessionID); err != nil {
			log.Warn().Err(err).Str("session_id", sessionID).Msg("[SessionStore] Failed to clear model handoff context")
		}
	}

	// Reset FSM coherently with context state
	fsm, hasFSM := s.sessionFSMs[sessionID]
	if hasFSM {
		if !keepPinned {
			// Full reset: back to RESOLVING (must discover again)
			fsm.Reset()
		} else if hasCtx && ctx.HasAnyResources() {
			// Pinned resources remain: keep progress (stay in READING if possible)
			fsm.ResetKeepProgress()
		} else {
			// keepPinned=true but no resources left: must rediscover
			fsm.Reset()
		}
	}

	log.Debug().
		Str("session_id", sessionID).
		Bool("keep_pinned", keepPinned).
		Bool("has_resources", hasCtx && ctx.HasAnyResources()).
		Str("fsm_state", func() string {
			if hasFSM {
				return string(fsm.State)
			}
			return "none"
		}()).
		Msg("[SessionStore] Cleared session state")
}

// cleanupResolvedContext is called when a session is deleted to also remove its context
func (s *SessionStore) cleanupResolvedContext(sessionID string) {
	// Note: caller must NOT hold the lock (or use a separate lock for contexts)
	delete(s.resolvedContexts, sessionID)
}

// GetToolSet returns a copy of the tool allowlist for a session, or nil if none set.
func (s *SessionStore) GetToolSet(sessionID string) map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	toolSet, ok := s.sessionToolSets[sessionID]
	if !ok {
		return nil
	}
	return copyToolSet(toolSet)
}

// SetToolSet stores a tool allowlist for a session.
func (s *SessionStore) SetToolSet(sessionID string, toolSet map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessionToolSets[sessionID] = copyToolSet(toolSet)
}

// AddToolSet merges tool allowlist entries into the session's tool set.
// Returns a copy of the updated tool set.
func (s *SessionStore) AddToolSet(sessionID string, additions map[string]bool) map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	toolSet, ok := s.sessionToolSets[sessionID]
	if !ok {
		toolSet = make(map[string]bool)
	}
	for name := range additions {
		toolSet[name] = true
	}
	s.sessionToolSets[sessionID] = toolSet
	return copyToolSet(toolSet)
}

func copyToolSet(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	out := make(map[string]bool, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}
