package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ApprovalStatus represents the state of an approval request.
type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "pending"
	StatusApproved ApprovalStatus = "approved"
	StatusDenied   ApprovalStatus = "denied"
	StatusExpired  ApprovalStatus = "expired"
)

// RiskLevel indicates the potential impact of a command.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// ApprovalRequest represents a pending command awaiting user approval.
type ApprovalRequest struct {
	ID          string         `json:"id"`
	ExecutionID string         `json:"executionId"` // Groups related approvals
	ToolID      string         `json:"toolId"`      // From LLM tool call
	Command     string         `json:"command"`
	TargetType  string         `json:"targetType"` // host, container, vm, node
	TargetID    string         `json:"targetId"`
	TargetName  string         `json:"targetName"`
	Context     string         `json:"context"`   // Why AI wants to run this
	RiskLevel   RiskLevel      `json:"riskLevel"` // low, medium, high
	Status      ApprovalStatus `json:"status"`
	RequestedAt time.Time      `json:"requestedAt"`
	ExpiresAt   time.Time      `json:"expiresAt"`
	DecidedAt   *time.Time     `json:"decidedAt,omitempty"`
	DecidedBy   string         `json:"decidedBy,omitempty"`
	DenyReason  string         `json:"denyReason,omitempty"`
}

// ExecutionState stores the AI conversation state for resumption after approval.
type ExecutionState struct {
	ID              string                   `json:"id"`
	OriginalRequest map[string]interface{}   `json:"originalRequest"` // Serialized ExecuteRequest
	Messages        []map[string]interface{} `json:"messages"`        // Conversation history
	PendingToolCall map[string]interface{}   `json:"pendingToolCall"` // Tool call awaiting approval
	CreatedAt       time.Time                `json:"createdAt"`
	ExpiresAt       time.Time                `json:"expiresAt"`
}

// Store manages approval requests and execution states.
type Store struct {
	mu             sync.RWMutex
	approvals      map[string]*ApprovalRequest
	executions     map[string]*ExecutionState
	dataDir        string
	defaultTimeout time.Duration
	maxApprovals   int
}

// StoreConfig configures the approval store.
type StoreConfig struct {
	DataDir        string
	DefaultTimeout time.Duration // Default 5 minutes
	MaxApprovals   int           // Maximum pending approvals (default 100)
}

// NewStore creates a new approval store.
func NewStore(cfg StoreConfig) (*Store, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}

	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = 5 * time.Minute
	}

	if cfg.MaxApprovals == 0 {
		cfg.MaxApprovals = 100
	}

	s := &Store{
		approvals:      make(map[string]*ApprovalRequest),
		executions:     make(map[string]*ExecutionState),
		dataDir:        cfg.DataDir,
		defaultTimeout: cfg.DefaultTimeout,
		maxApprovals:   cfg.MaxApprovals,
	}

	// Load existing data
	if err := s.load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load approval data, starting fresh")
	}

	// Note: Call StartCleanup(ctx) after creating the store to begin cleanup goroutine

	return s, nil
}

// CreateApproval creates a new approval request.
func (s *Store) CreateApproval(req *ApprovalRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check capacity
	pendingCount := 0
	for _, a := range s.approvals {
		if a.Status == StatusPending {
			pendingCount++
		}
	}
	if pendingCount >= s.maxApprovals {
		return fmt.Errorf("maximum pending approvals (%d) reached", s.maxApprovals)
	}

	// Generate ID if not set
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	// Set defaults
	req.Status = StatusPending
	req.RequestedAt = time.Now()
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = req.RequestedAt.Add(s.defaultTimeout)
	}

	// Assess risk if not set
	if req.RiskLevel == "" {
		req.RiskLevel = AssessRiskLevel(req.Command, req.TargetType)
	}

	s.approvals[req.ID] = req

	// Persist asynchronously
	go s.save()

	log.Info().
		Str("id", req.ID).
		Str("command", truncateCommand(req.Command, 50)).
		Str("risk", string(req.RiskLevel)).
		Msg("Created approval request")

	return nil
}

// GetApproval returns an approval request by ID.
func (s *Store) GetApproval(id string) (*ApprovalRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.approvals[id]
	if !ok {
		return nil, false
	}

	// Check expiration
	if req.Status == StatusPending && time.Now().After(req.ExpiresAt) {
		// Don't modify here, let cleanup handle it
		reqCopy := *req
		reqCopy.Status = StatusExpired
		return &reqCopy, true
	}

	return req, true
}

// GetPendingApprovals returns all pending approval requests.
func (s *Store) GetPendingApprovals() []*ApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var pending []*ApprovalRequest

	for _, req := range s.approvals {
		if req.Status == StatusPending && now.Before(req.ExpiresAt) {
			pending = append(pending, req)
		}
	}

	return pending
}

// GetApprovalsByExecution returns all approvals for an execution ID.
func (s *Store) GetApprovalsByExecution(executionID string) []*ApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*ApprovalRequest
	for _, req := range s.approvals {
		if req.ExecutionID == executionID {
			results = append(results, req)
		}
	}

	return results
}

// Approve marks an approval request as approved.
func (s *Store) Approve(id, username string) (*ApprovalRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.approvals[id]
	if !ok {
		return nil, fmt.Errorf("approval request not found: %s", id)
	}

	// Idempotent: if already approved, return success (handles double-clicks, race conditions)
	if req.Status == StatusApproved {
		return req, nil
	}

	if req.Status != StatusPending {
		return nil, fmt.Errorf("approval request is not pending (status: %s)", req.Status)
	}

	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		go s.save()
		return nil, fmt.Errorf("approval request has expired")
	}

	now := time.Now()
	req.Status = StatusApproved
	req.DecidedAt = &now
	req.DecidedBy = username

	go s.save()

	log.Info().
		Str("id", id).
		Str("by", username).
		Str("command", truncateCommand(req.Command, 50)).
		Msg("Approval request approved")

	return req, nil
}

// Deny marks an approval request as denied.
func (s *Store) Deny(id, username, reason string) (*ApprovalRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.approvals[id]
	if !ok {
		return nil, fmt.Errorf("approval request not found: %s", id)
	}

	if req.Status != StatusPending {
		return nil, fmt.Errorf("approval request is not pending (status: %s)", req.Status)
	}

	now := time.Now()
	req.Status = StatusDenied
	req.DecidedAt = &now
	req.DecidedBy = username
	req.DenyReason = reason

	go s.save()

	log.Info().
		Str("id", id).
		Str("by", username).
		Str("reason", reason).
		Msg("Approval request denied")

	return req, nil
}

// StoreExecution saves an execution state for later resumption.
func (s *Store) StoreExecution(state *ExecutionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state.ID == "" {
		return fmt.Errorf("execution ID is required")
	}

	state.CreatedAt = time.Now()
	if state.ExpiresAt.IsZero() {
		state.ExpiresAt = state.CreatedAt.Add(s.defaultTimeout)
	}

	s.executions[state.ID] = state

	go s.save()

	return nil
}

// GetExecution returns an execution state by ID.
func (s *Store) GetExecution(id string) (*ExecutionState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.executions[id]
	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(state.ExpiresAt) {
		return nil, false
	}

	return state, true
}

// DeleteExecution removes an execution state.
func (s *Store) DeleteExecution(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.executions, id)
	go s.save()
}

// CleanupExpired removes expired approvals and executions.
func (s *Store) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cleaned := 0

	// Expire pending approvals
	for _, req := range s.approvals {
		if req.Status == StatusPending && now.After(req.ExpiresAt) {
			req.Status = StatusExpired
			cleaned++
		}
	}

	// Remove old completed approvals (keep for 24 hours)
	cutoff := now.Add(-24 * time.Hour)
	for id, req := range s.approvals {
		if req.Status != StatusPending && req.DecidedAt != nil && req.DecidedAt.Before(cutoff) {
			delete(s.approvals, id)
			cleaned++
		}
	}

	// Remove expired executions
	for id, state := range s.executions {
		if now.After(state.ExpiresAt) {
			delete(s.executions, id)
			cleaned++
		}
	}

	if cleaned > 0 {
		go s.save()
	}

	return cleaned
}

// GetStats returns statistics about the approval store.
func (s *Store) GetStats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]int{
		"pending":    0,
		"approved":   0,
		"denied":     0,
		"expired":    0,
		"executions": len(s.executions),
	}

	for _, req := range s.approvals {
		switch req.Status {
		case StatusPending:
			stats["pending"]++
		case StatusApproved:
			stats["approved"]++
		case StatusDenied:
			stats["denied"]++
		case StatusExpired:
			stats["expired"]++
		}
	}

	return stats
}

// Persistence

func (s *Store) approvalsFile() string {
	return filepath.Join(s.dataDir, "ai_approvals.json")
}

func (s *Store) executionsFile() string {
	return filepath.Join(s.dataDir, "ai_executions.json")
}

func (s *Store) load() error {
	// Load approvals
	if data, err := os.ReadFile(s.approvalsFile()); err == nil {
		var approvals []*ApprovalRequest
		if err := json.Unmarshal(data, &approvals); err == nil {
			for _, a := range approvals {
				s.approvals[a.ID] = a
			}
		}
	}

	// Load executions
	if data, err := os.ReadFile(s.executionsFile()); err == nil {
		var executions []*ExecutionState
		if err := json.Unmarshal(data, &executions); err == nil {
			for _, e := range executions {
				s.executions[e.ID] = e
			}
		}
	}

	return nil
}

func (s *Store) save() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Save approvals
	approvals := make([]*ApprovalRequest, 0, len(s.approvals))
	for _, a := range s.approvals {
		approvals = append(approvals, a)
	}
	if data, err := json.MarshalIndent(approvals, "", "  "); err == nil {
		os.WriteFile(s.approvalsFile(), data, 0600)
	}

	// Save executions
	executions := make([]*ExecutionState, 0, len(s.executions))
	for _, e := range s.executions {
		executions = append(executions, e)
	}
	if data, err := json.MarshalIndent(executions, "", "  "); err == nil {
		os.WriteFile(s.executionsFile(), data, 0600)
	}
}

// StartCleanup begins periodic cleanup of expired approvals and executions.
// Call this with a context that cancels on shutdown.
func (s *Store) StartCleanup(ctx context.Context) {
	go s.cleanupLoop(ctx)
}

func (s *Store) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Msg("Approval store cleanup loop stopped")
			return
		case <-ticker.C:
			cleaned := s.CleanupExpired()
			if cleaned > 0 {
				log.Debug().Int("count", cleaned).Msg("Cleaned up expired approval items")
			}
		}
	}
}

// Risk Assessment

// High risk patterns - destructive or system-wide impact
var highRiskPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+(-rf?|--recursive)\s`),
	regexp.MustCompile(`(?i)\bdd\s+.*of=/dev/`),
	regexp.MustCompile(`(?i)\bmkfs\b`),
	regexp.MustCompile(`(?i)\bchmod\s+(-R\s+)?777\b`),
	regexp.MustCompile(`(?i)\bapt\s+(remove|purge)\b`),
	regexp.MustCompile(`(?i)\byum\s+(remove|erase)\b`),
	regexp.MustCompile(`(?i)\bdnf\s+remove\b`),
	regexp.MustCompile(`(?i)\bpacman\s+-R`),
	regexp.MustCompile(`(?i)\biptables\s+-F\b`),
	regexp.MustCompile(`(?i)\bsystemctl\s+(disable|mask)\b`),
	regexp.MustCompile(`(?i)\bkill\s+-9\s`),
	regexp.MustCompile(`(?i)\bpkill\s+-9\b`),
	regexp.MustCompile(`(?i)\bdocker\s+rm\s+-f`),
	regexp.MustCompile(`(?i)\bdocker\s+system\s+prune`),
	regexp.MustCompile(`(?i)\bpct\s+destroy\b`),
	regexp.MustCompile(`(?i)\bqm\s+destroy\b`),
}

// Medium risk patterns - service impact but recoverable
var mediumRiskPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsystemctl\s+(restart|stop|start)\b`),
	regexp.MustCompile(`(?i)\bservice\s+\S+\s+(restart|stop|start)\b`),
	regexp.MustCompile(`(?i)\bdocker\s+(restart|stop|start|kill)\b`),
	regexp.MustCompile(`(?i)\bapt\s+(update|upgrade|install)\b`),
	regexp.MustCompile(`(?i)\byum\s+(update|install)\b`),
	regexp.MustCompile(`(?i)\bdnf\s+(update|install)\b`),
	regexp.MustCompile(`(?i)\bpct\s+(start|stop|reboot|resize)\b`),
	regexp.MustCompile(`(?i)\bqm\s+(start|stop|reboot|resize)\b`),
	regexp.MustCompile(`(?i)\bkill\b`),
	regexp.MustCompile(`(?i)\bpkill\b`),
	regexp.MustCompile(`(?i)\bchmod\b`),
	regexp.MustCompile(`(?i)\bchown\b`),
	regexp.MustCompile(`(?i)\bmv\s`),
	regexp.MustCompile(`(?i)\bcp\s+-r`),
}

// AssessRiskLevel determines the risk level of a command.
func AssessRiskLevel(command, targetType string) RiskLevel {
	// Check high risk patterns first
	for _, pattern := range highRiskPatterns {
		if pattern.MatchString(command) {
			return RiskHigh
		}
	}

	// Check medium risk patterns
	for _, pattern := range mediumRiskPatterns {
		if pattern.MatchString(command) {
			return RiskMedium
		}
	}

	// Production targets are higher risk
	if targetType == "node" {
		// Commands on nodes are generally higher risk
		for _, pattern := range mediumRiskPatterns {
			if pattern.MatchString(command) {
				return RiskHigh
			}
		}
	}

	return RiskLow
}

// Helper functions

func truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen] + "..."
}

// Global store instance
var (
	globalStore *Store
	storeMu     sync.RWMutex
)

// SetStore sets the global approval store.
func SetStore(s *Store) {
	storeMu.Lock()
	defer storeMu.Unlock()
	globalStore = s
}

// GetStore returns the global approval store.
func GetStore() *Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return globalStore
}
