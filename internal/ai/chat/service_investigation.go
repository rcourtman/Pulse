package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelboundary"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rs/zerolog/log"
)

// InvestigationRunRequest is the core-owned request for one Patrol
// investigation run. It deliberately carries no autonomy field and no
// profile selector: the run always executes under the Patrol
// investigation profile, and correlation identity comes from trusted
// orchestration context, never from the model or a transport payload.
type InvestigationRunRequest struct {
	SessionID    string
	Prompt       string
	SystemPrompt string
	MaxTurns     int
	ExecutionID  string
	// Identity is the trusted correlation identity injected into any
	// captured proposal.
	Identity tools.ProposalIdentity
	// Catalog resolves advertised resource capabilities for proposal
	// validation (ultimately the tenant-bound action lifecycle path).
	Catalog tools.ProposalCatalog
}

// InvestigationRunResult is the structured outcome of one investigation
// run. Proposal cardinality is a first-class field: consumers never
// reconstruct proposals by parsing session messages.
type InvestigationRunResult struct {
	Content string
	// Proposal is the single validated typed action proposal, nil for a
	// valid zero-proposal conclusion.
	Proposal *tools.CapturedProposal
	// FailedProposalAttempts counts proposal calls that failed
	// validation. When >0 with no captured proposal, the run error is
	// tools.ErrProposalAttemptsFailed.
	FailedProposalAttempts int
	InputTokens            int
	OutputTokens           int
}

// InvestigationRunError preserves the two independent failure channels from
// an investigation run. Proposal-only failures are valid completed runs that
// require operator attention; RunErr means the provider/runtime itself failed
// and must never be collapsed into that completed outcome.
type InvestigationRunError struct {
	runErr      error
	proposalErr error
}

// NewInvestigationRunError constructs a two-channel investigation failure.
// The package is internal; the exported constructor lets the API adapter's
// boundary tests exercise the same concrete error it receives at runtime.
func NewInvestigationRunError(runErr, proposalErr error) *InvestigationRunError {
	if runErr == nil && proposalErr == nil {
		return nil
	}
	return &InvestigationRunError{runErr: runErr, proposalErr: proposalErr}
}

func (e *InvestigationRunError) Error() string {
	if e == nil {
		return ""
	}
	return errors.Join(e.runErr, e.proposalErr).Error()
}

// Unwrap preserves errors.Is/errors.As behavior for both failure channels.
func (e *InvestigationRunError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return []error{e.runErr, e.proposalErr}
}

// RunFailure returns the provider/runtime failure, if any.
func (e *InvestigationRunError) RunFailure() error {
	if e == nil {
		return nil
	}
	return e.runErr
}

// ProposalFailure returns the proposal-channel failure, if any.
func (e *InvestigationRunError) ProposalFailure() error {
	if e == nil {
		return nil
	}
	return e.proposalErr
}

// ExecuteInvestigationStream runs one Patrol investigation under the
// investigation execution profile and returns the structured result.
// Proposal-channel violations (ambiguity, integrity, failed-only
// attempts) return the result alongside the typed proposal error.
func (s *Service) ExecuteInvestigationStream(ctx context.Context, req InvestigationRunRequest, callback StreamCallback) (*InvestigationRunResult, error) {
	// Correlation identity is a precondition: without it a captured
	// proposal could never be reconciled, so the run refuses before any
	// provider call or session exists.
	if strings.TrimSpace(req.Identity.FindingID) == "" || strings.TrimSpace(req.Identity.InvestigationID) == "" {
		return nil, fmt.Errorf("investigation run requires finding and investigation identity before it can start")
	}

	s.mu.RLock()
	if !s.started {
		s.mu.RUnlock()
		return nil, fmt.Errorf("service not started")
	}
	sessions := s.sessions
	baseExecutor := s.executor
	unifiedResourceProvider := s.unifiedResourceProvider
	cfg := s.cfg
	effectiveControlLevel := s.effectiveControlLevelLocked()
	s.mu.RUnlock()

	if baseExecutor == nil {
		return nil, fmt.Errorf("no tool executor available")
	}

	// One effective request executor, built before projection; the
	// proposal capture sink is shared by design (one run, one capture).
	capture := tools.NewProposalCapture(req.Identity, req.Catalog)
	executor := baseExecutor.Clone()
	executor.SetControlLevel(effectiveControlLevel)
	executor.ApplyExecutionProfile(tools.ProfilePatrolInvestigation)
	executor.SetProposalCapture(capture)

	// Investigation uses the Patrol model.
	investigationModel := ""
	if cfg != nil {
		investigationModel = cfg.GetPatrolModel()
		if investigationModel == "" {
			investigationModel = cfg.GetChatModel()
		}
	}
	if investigationModel == "" {
		return nil, fmt.Errorf("no patrol model configured")
	}
	provider, err := s.createPatrolProviderForModel(investigationModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create investigation provider: %w", err)
	}

	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.buildSystemPrompt()
	}
	loop := NewAgenticLoop(provider, executor, systemPrompt)
	loop.SetOrgID(s.orgID)
	loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
	loop.SetExecutionID(req.ExecutionID)
	loop.SetRequestSanitizer(modelboundary.RequestSanitizerForModel(investigationModel, unifiedResourceProvider))
	if req.MaxTurns > 0 {
		loop.SetMaxTurns(req.MaxTurns)
	}
	if parts := strings.SplitN(investigationModel, ":", 2); len(parts) == 2 {
		loop.SetProviderInfo(parts[0], parts[1])
	}
	if s.budgetChecker != nil {
		loop.SetBudgetChecker(s.budgetChecker)
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "investigation-" + strings.TrimSpace(req.Identity.InvestigationID)
	}
	session, err := sessions.EnsureSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure investigation session: %w", err)
	}
	executor.SetResolvedContext(sessions.GetResolvedContext(session.ID))
	loop.SetSessionFSM(sessions.GetSessionFSM(session.ID))
	loop.SetKnowledgeAccumulator(sessions.NewKnowledgeAccumulatorForRun(session.ID))

	userMsg := Message{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Prompt,
		Timestamp: time.Now(),
	}
	if err := sessions.AddMessage(session.ID, userMsg); err != nil {
		log.Warn().Err(err).Msg("failed to save investigation user message")
	}

	// Investigation runs are stateless like Patrol detection: only this
	// run's prompt is loaded; the session is a forensic log.
	messages := []Message{userMsg}
	filteredTools := s.toolsForExecutor(executor, false)

	resultMessages, runErr := loop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, callback)
	for _, msg := range resultMessages {
		if msg.Role == "user" && msg.ToolResult == nil {
			continue
		}
		if err := sessions.AddMessage(session.ID, msg); err != nil {
			log.Warn().Err(err).Msg("failed to save investigation message")
		}
	}

	var contentBuilder strings.Builder
	for _, msg := range resultMessages {
		if msg.Role == "assistant" && msg.Content != "" {
			contentBuilder.WriteString(msg.Content)
		}
	}

	proposal, failedAttempts, proposalErr := capture.Outcome()
	result := &InvestigationRunResult{
		Content:                contentBuilder.String(),
		Proposal:               proposal,
		FailedProposalAttempts: failedAttempts,
		InputTokens:            loop.GetTotalInputTokens(),
		OutputTokens:           loop.GetTotalOutputTokens(),
	}
	if runErr != nil || proposalErr != nil {
		// A proposal is actionable only from a completely successful
		// run: any error nils it, and simultaneous run/proposal errors
		// are both preserved.
		result.Proposal = nil
		return result, NewInvestigationRunError(runErr, proposalErr)
	}
	return result, nil
}

// ListInvestigationTools names the tools an investigation run offers,
// projected through the same investigation-profile path the run uses.
func (s *Service) ListInvestigationTools() []string {
	s.mu.RLock()
	executor := s.executor
	effectiveControlLevel := s.effectiveControlLevelLocked()
	s.mu.RUnlock()
	if executor == nil {
		return nil
	}
	effective := executor.Clone()
	effective.SetControlLevel(effectiveControlLevel)
	effective.ApplyExecutionProfile(tools.ProfilePatrolInvestigation)
	// The proposal tool is offered only when a capture could exist; for
	// listing purposes project with a placeholder sink so the manifest
	// reflects a real run.
	effective.SetProposalCapture(tools.NewProposalCapture(tools.ProposalIdentity{}, nil))
	offered := s.toolsForExecutor(effective, false)
	names := make([]string, 0, len(offered))
	for _, tool := range offered {
		if tool.Name != "" {
			names = append(names, tool.Name)
		}
	}
	return names
}
