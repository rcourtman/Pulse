package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelboundary"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rs/zerolog/log"
)

// InvestigationRunRequest is the core-owned request for one Patrol
// investigation run. It deliberately carries no autonomy field and no
// profile selector: the run always executes under the Patrol
// investigation profile, and correlation identity comes from trusted
// orchestration context, never from the model or a transport payload.
type InvestigationRunRequest struct {
	SessionID        string
	Prompt           string
	SystemPrompt     string
	MaxTurns         int
	MaxEvidenceCalls int
	ExecutionID      string
	// ResourceType is trusted finding metadata used only for least-manifest
	// projection. Unknown types retain the normal governed profile.
	ResourceType string
	// Identity is the trusted correlation identity injected into any
	// captured proposal.
	Identity tools.ProposalIdentity
	// Catalog resolves advertised resource capabilities for proposal
	// validation (ultimately the tenant-bound action lifecycle path).
	Catalog tools.ProposalCatalog
	Planner tools.ProposalPlanner
}

// InvestigationRunResult is the structured outcome of one investigation
// run. Proposal cardinality is a first-class field: consumers never
// reconstruct proposals by parsing session messages.
type InvestigationRunResult struct {
	Content string
	// Proposal is the single validated typed action proposal, nil for a
	// valid zero-proposal conclusion.
	Proposal      *tools.CapturedProposal
	InputTokens   int
	OutputTokens  int
	ModelTurns    int
	EvidenceCalls int
	ToolCalls     int
}

// ExecuteInvestigationStream runs one Patrol investigation under the
// investigation execution profile and returns the structured result.
// Planning refusals remain tool results. Provider/runtime errors return the
// partial result so any already persisted action remains discoverable.
func (s *Service) ExecuteInvestigationStream(ctx context.Context, req InvestigationRunRequest, callback StreamCallback) (*InvestigationRunResult, error) {
	// Correlation identity is a precondition: without it a captured
	// proposal could never be reconciled, so the run refuses before any
	// provider call or session exists.
	if strings.TrimSpace(req.Identity.ProposalID) == "" || strings.TrimSpace(req.Identity.FindingID) == "" || strings.TrimSpace(req.Identity.InvestigationID) == "" {
		return nil, fmt.Errorf("investigation run requires proposal, finding and investigation identity before it can start")
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
	capture.SetPlanner(req.Planner)
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
	if req.MaxEvidenceCalls > 0 {
		loop.SetMaxEvidenceCalls(req.MaxEvidenceCalls)
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
	filteredTools, err := restrictInvestigationProviderToolsForResourceType(
		s.toolsForExecutor(executor, false), req.ResourceType,
	)
	if err != nil {
		return nil, err
	}

	resultMessages, runErr := loop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, func(event StreamEvent) {
		if event.Type == "tool_end" {
			var result ToolEndData
			if json.Unmarshal(event.Data, &result) == nil && isInvestigationEvidenceTool(result.Name) {
				capture.RecordEvidence(result.ID)
			}
		}
		if callback != nil {
			callback(event)
		}
	})
	for _, msg := range resultMessages {
		if msg.Role == "user" && msg.ToolResult == nil {
			continue
		}
		if err := sessions.AddMessage(session.ID, msg); err != nil {
			log.Warn().Err(err).Msg("failed to save investigation message")
		}
	}

	proposal, proposalErr := capture.Outcome()
	var contentBuilder strings.Builder
	for _, msg := range resultMessages {
		if msg.Role == "assistant" && msg.Content != "" {
			contentBuilder.WriteString(msg.Content)
		}
	}
	content := contentBuilder.String()

	result := &InvestigationRunResult{
		Content:       content,
		Proposal:      proposal,
		InputTokens:   loop.GetTotalInputTokens(),
		OutputTokens:  loop.GetTotalOutputTokens(),
		ModelTurns:    loop.GetTotalModelTurns(),
		EvidenceCalls: loop.GetTotalEvidenceCalls(),
		ToolCalls:     loop.GetTotalToolCalls(),
	}
	if runErr != nil || proposalErr != nil {
		// A provider failure cannot erase a persisted action. The caller must
		// retain its reference and must not request automatic progression.
		return result, errors.Join(runErr, proposalErr)
	}
	return result, nil
}

func restrictInvestigationProviderToolsForResourceType(providerTools []providers.Tool, resourceType string) ([]providers.Tool, error) {
	scopedNames, ok := agentcapabilities.PatrolInvestigationToolNamesForResourceTypes([]string{resourceType})
	if !ok {
		// Unknown and additive resource kinds retain the full already-governed
		// profile so they do not silently lose evidence access. They must still
		// satisfy the same investigation boundary as a typed projection before
		// inference begins.
		filtered := append([]providers.Tool(nil), providerTools...)
		if err := validateInvestigationProviderTools(filtered, resourceType); err != nil {
			return nil, err
		}
		return filtered, nil
	}

	// The typed map is a least-manifest ceiling, not a claim that every
	// deployment has every optional evidence adapter. In particular,
	// pulse_read exists only when an agent-routed or native log adapter is
	// available. Intersect evidence with the already-governed runtime profile,
	// while keeping the investigation-owned proposal boundary mandatory.
	scoped := make(map[string]bool, len(scopedNames))
	for _, name := range scopedNames {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("failed to project scoped investigation tools: scoped tool name must not be empty")
		}
		scoped[name] = true
	}

	filtered := make([]providers.Tool, 0, len(scoped))
	for _, tool := range providerTools {
		name := strings.TrimSpace(tool.Name)
		if !scoped[name] {
			continue
		}
		filtered = append(filtered, tool)
	}
	if err := validateInvestigationProviderTools(filtered, resourceType); err != nil {
		return nil, err
	}
	return filtered, nil
}

func validateInvestigationProviderTools(providerTools []providers.Tool, resourceType string) error {
	present := make(map[string]bool, len(providerTools))
	evidenceCount := 0
	for _, tool := range providerTools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		present[name] = true
		if isInvestigationEvidenceTool(name) {
			evidenceCount++
		}
	}
	for _, required := range []string{
		agentcapabilities.PatrolActionCapabilitiesToolName,
		agentcapabilities.PatrolProposeActionToolName,
	} {
		if !present[required] {
			return fmt.Errorf("failed to project investigation tools: required tool unavailable after profile projection: %s", required)
		}
	}
	if evidenceCount == 0 {
		return fmt.Errorf("failed to project investigation tools: no evidence tool is available for resource type %q", strings.TrimSpace(resourceType))
	}
	return nil
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
