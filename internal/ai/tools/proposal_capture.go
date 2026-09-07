package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// ToolInvocation is the explicit envelope for one tool call: the
// provider-assigned tool-use ID plus name and arguments. The ID travels
// with the call through the registry so stateful capture (proposal
// cardinality, idempotent replay) can key on call identity instead of
// guessing from payloads.
type ToolInvocation struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

type invocationIDContextKey struct{}

// withInvocationID stores the tool-use ID for the current call; handlers
// that need call identity (the proposal capture) read it back with
// InvocationIDFromContext. Context-carried because tool calls from one
// provider turn execute concurrently on a shared executor clone.
func withInvocationID(ctx context.Context, id string) context.Context {
	if strings.TrimSpace(id) == "" {
		return ctx
	}
	return context.WithValue(ctx, invocationIDContextKey{}, id)
}

// InvocationIDFromContext returns the tool-use ID for the current call.
func InvocationIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(invocationIDContextKey{}).(string)
	return id
}

// ProposalIdentity is the trusted correlation identity for one
// investigation run, injected by core orchestration context - never by
// the model, whose tool schema carries only resource, capability, params,
// and reason.
type ProposalIdentity struct {
	ProposalID      string
	FindingID       string
	InvestigationID string
	EvidenceIDs     []string
}

// CapturedProposal is one validated typed action proposal.
type CapturedProposal struct {
	Action       *unified.ActionAuditRecord
	InvocationID string
	Identity     ProposalIdentity
	ResourceID   string
	// CausalResourceID is the model's optional causal attribution. Empty means
	// the cause is not established. Capture validates the action contract, not
	// this diagnosis or its rationale.
	CausalResourceID string
	CapabilityName   string
	Params           map[string]interface{}
	Reason           string
}

// ProposalCatalog resolves a resource's advertised capabilities for
// proposal validation. Injected by the core investigation entrypoint
// (ultimately the tenant-bound action lifecycle Capabilities path).
type ProposalCatalog func(ctx context.Context, resourceID string) ([]unified.ResourceCapability, error)

// ProposalPlanner is the core-owned plan-only boundary. It persists a canonical
// action and returns its audit record. It grants no approval or execution.
type ProposalPlanner func(context.Context, CapturedProposal) (unified.ActionAuditRecord, error)

// ProposalCapture retains the accepted action for one explicitly budgeted
// investigation. Canonical request identity owns replay and conflicts. A later
// refused call or provider failure cannot erase an already persisted action.
type ProposalCapture struct {
	mu       sync.Mutex
	identity ProposalIdentity
	catalog  ProposalCatalog
	planner  ProposalPlanner
	proposal *CapturedProposal
}

func (i ProposalIdentity) clone() ProposalIdentity {
	i.EvidenceIDs = append([]string(nil), i.EvidenceIDs...)
	return i
}

func NewProposalCapture(identity ProposalIdentity, catalog ProposalCatalog) *ProposalCapture {
	return &ProposalCapture{identity: identity.clone(), catalog: catalog}
}

func (c *ProposalCapture) SetPlanner(planner ProposalPlanner) { c.planner = planner }

// RecordEvidence binds completed observations, including explicit access
// refusals, to the action origin. An ID links to a result, not a diagnosis proof.
func (c *ProposalCapture) RecordEvidence(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(id) == "" {
		return
	}
	for _, existing := range c.identity.EvidenceIDs {
		if existing == id {
			return
		}
	}
	c.identity.EvidenceIDs = append(c.identity.EvidenceIDs, id)
}

func (c *ProposalCapture) Capabilities(ctx context.Context, resourceID string) ([]unified.ResourceCapability, error) {
	if c == nil || c.catalog == nil {
		return nil, errors.New("no capability catalog is wired for this investigation")
	}
	resourceID = unified.CanonicalResourceID(resourceID)
	if resourceID == "" {
		return nil, errors.New("resource_id is required")
	}
	return c.catalog(ctx, resourceID)
}

// cloneParams deep-clones a parameter map via JSON round-trip (the values
// arrived as decoded JSON, so the round-trip is lossless). The sink never
// retains or returns the caller's map: mutation after validation must not
// be able to change the actionable proposal.
func cloneParams(params map[string]interface{}) (map[string]interface{}, error) {
	if params == nil {
		return map[string]interface{}{}, nil
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("proposal parameters are not serializable")
	}
	clone := map[string]interface{}{}
	if err := json.Unmarshal(encoded, &clone); err != nil {
		return nil, fmt.Errorf("proposal parameters are not serializable")
	}
	return clone, nil
}

// Submit consults the canonical planner during the tool call. Serialization
// enforces the one-action budget while the durable store owns idempotency.
func (c *ProposalCapture) Submit(ctx context.Context, invocationID, resourceID, causalResourceID, capabilityName, reason string, params map[string]interface{}) (*unified.ActionAuditRecord, error) {
	if strings.TrimSpace(invocationID) == "" {
		return nil, errors.New("action planning requires a tool invocation identity")
	}
	params, err := cloneParams(params)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.planner == nil {
		return nil, errors.New("canonical action planning is unavailable for this investigation")
	}
	identity := c.identity.clone()
	// Replay keeps the evidence attached at first acceptance. Later observations
	// remain in the investigation history and cannot rewrite accepted intent.
	if c.proposal != nil {
		identity = c.proposal.Identity.clone()
	}
	proposal := CapturedProposal{InvocationID: invocationID, Identity: identity, ResourceID: resourceID, CausalResourceID: causalResourceID, CapabilityName: capabilityName, Reason: reason, Params: params}
	record, err := c.planner(ctx, proposal)
	if record.ID != "" {
		proposal.Action = &record
		if c.proposal == nil {
			c.proposal = &proposal
		} else {
			c.proposal.Action = &record
		}
	}
	if err != nil {
		if c.proposal != nil {
			return c.proposal.Action, err
		}
		return proposal.Action, err
	}
	if record.ID == "" {
		return nil, errors.New("canonical planner returned no persisted action identity")
	}
	return &record, nil
}

func (c *ProposalCapture) Outcome() (*CapturedProposal, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proposal == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(c.proposal)
	if err != nil {
		return nil, err
	}
	var proposal CapturedProposal
	if err = json.Unmarshal(encoded, &proposal); err != nil {
		return nil, err
	}
	return &proposal, nil
}
