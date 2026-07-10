package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
	InvocationID   string
	Identity       ProposalIdentity
	ResourceID     string
	CapabilityName string
	Params         map[string]interface{}
	Reason         string
}

// ProposalCatalog resolves a resource's advertised capabilities for
// proposal validation. Injected by the core investigation entrypoint
// (ultimately the tenant-bound action lifecycle Capabilities path).
type ProposalCatalog func(ctx context.Context, resourceID string) ([]unified.ResourceCapability, error)

// Typed terminal proposal errors surfaced on the structured run result.
var (
	// ErrProposalAmbiguous: two distinct valid proposal calls were made.
	// The run's proposal is invalidated - concurrency makes "first"
	// nondeterministic, so ambiguity latches terminally.
	ErrProposalAmbiguous = errors.New("ambiguous investigation result: multiple distinct action proposals were submitted")
	// ErrProposalIntegrity: the same tool-use ID re-submitted a different
	// payload. Latches terminally and invalidates the capture.
	ErrProposalIntegrity = errors.New("proposal integrity violation: one tool-use id submitted conflicting payloads")
	// ErrProposalAttemptsFailed: no proposal was captured but proposal
	// attempts failed validation - this is an error outcome, never the
	// valid zero-proposal conclusion.
	ErrProposalAttemptsFailed = errors.New("investigation made proposal attempts but none validated")
)

type proposalCaptureState int

const (
	proposalCaptureEmpty proposalCaptureState = iota
	proposalCaptureHeld
	proposalCaptureAmbiguous
	proposalCaptureIntegrityViolated
)

// ProposalCapture is the request-local sink for typed action proposals.
// One capture serves one investigation run; executor clones share it
// deliberately so every provider attempt lands in the same sink. State
// latches terminally: a second distinct valid proposal (or a conflicting
// replay) invalidates the captured proposal for the whole run.
type ProposalCapture struct {
	mu             sync.Mutex
	identity       ProposalIdentity
	catalog        ProposalCatalog
	state          proposalCaptureState
	proposal       *CapturedProposal
	fingerprint    string
	failedAttempts int
}

// NewProposalCapture builds the sink with trusted identity and the
// capability catalog used for validation.
func NewProposalCapture(identity ProposalIdentity, catalog ProposalCatalog) *ProposalCapture {
	return &ProposalCapture{identity: identity, catalog: catalog}
}

func proposalFingerprint(resourceID, capabilityName, reason string, params map[string]interface{}) string {
	payload := struct {
		ResourceID     string                 `json:"resourceId"`
		CapabilityName string                 `json:"capabilityName"`
		Reason         string                 `json:"reason"`
		Params         map[string]interface{} `json:"params"`
	}{resourceID, capabilityName, reason, params}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "unfingerprintable"
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// RecordFailedAttempt tallies a proposal call that failed validation.
// Failed attempts never count as proposals, but their presence turns a
// zero-proposal run into a typed error rather than a valid conclusion.
func (c *ProposalCapture) RecordFailedAttempt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failedAttempts++
}

// Submit records one validated proposal call. Semantics by call identity
// and payload fingerprint:
//   - first valid call: captured;
//   - same ID, same payload: idempotent replay (still captured);
//   - same ID, different payload: terminal integrity error, capture
//     invalidated;
//   - distinct ID, valid payload: terminal ambiguity, capture invalidated
//     (concurrent execution makes "first" nondeterministic, so neither
//     call wins).
func (c *ProposalCapture) Submit(invocationID, resourceID, capabilityName, reason string, params map[string]interface{}) error {
	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		c.mu.Lock()
		c.failedAttempts++
		c.mu.Unlock()
		return fmt.Errorf("proposal call carries no tool-use id; cannot establish call identity")
	}
	fingerprint := proposalFingerprint(resourceID, capabilityName, reason, params)

	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.state {
	case proposalCaptureAmbiguous:
		return ErrProposalAmbiguous
	case proposalCaptureIntegrityViolated:
		return ErrProposalIntegrity
	case proposalCaptureEmpty:
		c.state = proposalCaptureHeld
		c.fingerprint = fingerprint
		c.proposal = &CapturedProposal{
			InvocationID:   invocationID,
			Identity:       c.identity,
			ResourceID:     resourceID,
			CapabilityName: capabilityName,
			Params:         params,
			Reason:         reason,
		}
		return nil
	default: // proposalCaptureHeld
		if c.proposal != nil && c.proposal.InvocationID == invocationID {
			if c.fingerprint == fingerprint {
				// Idempotent replay of the same call.
				return nil
			}
			c.state = proposalCaptureIntegrityViolated
			c.proposal = nil
			return ErrProposalIntegrity
		}
		c.state = proposalCaptureAmbiguous
		c.proposal = nil
		return ErrProposalAmbiguous
	}
}

// Outcome reports the run's terminal proposal state: the captured
// proposal (nil for a valid zero-proposal run) or the typed error that
// invalidated the run. Zero proposals with failed attempts is an error,
// never a valid conclusion.
func (c *ProposalCapture) Outcome() (*CapturedProposal, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.state {
	case proposalCaptureAmbiguous:
		return nil, c.failedAttempts, ErrProposalAmbiguous
	case proposalCaptureIntegrityViolated:
		return nil, c.failedAttempts, ErrProposalIntegrity
	case proposalCaptureHeld:
		proposal := *c.proposal
		return &proposal, c.failedAttempts, nil
	default:
		if c.failedAttempts > 0 {
			return nil, c.failedAttempts, ErrProposalAttemptsFailed
		}
		return nil, 0, nil
	}
}

// validateProposalAgainstCatalog checks the proposal against the
// resource's advertised capability contract. Error messages never echo
// parameter values: proposal params exist only transiently for provider
// continuation and validation.
func validateProposalAgainstCatalog(ctx context.Context, catalog ProposalCatalog, resourceID, capabilityName string, params map[string]interface{}) error {
	if catalog == nil {
		return errors.New("no capability catalog is wired for proposal validation")
	}
	capabilities, err := catalog(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("capability catalog lookup failed for resource %q", resourceID)
	}
	var capability *unified.ResourceCapability
	for i := range capabilities {
		if strings.EqualFold(strings.TrimSpace(capabilities[i].Name), capabilityName) {
			capability = &capabilities[i]
			break
		}
	}
	if capability == nil {
		return fmt.Errorf("resource %q does not advertise capability %q", resourceID, capabilityName)
	}

	declared := map[string]unified.CapabilityParam{}
	for _, param := range capability.Params {
		declared[param.Name] = param
	}
	var missing []string
	for _, param := range capability.Params {
		if !param.Required {
			continue
		}
		if _, ok := params[param.Name]; !ok {
			missing = append(missing, param.Name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("proposal is missing required parameter(s) %v for capability %q", missing, capabilityName)
	}
	for name, value := range params {
		param, ok := declared[name]
		if !ok {
			return fmt.Errorf("parameter %q is not declared by capability %q", name, capabilityName)
		}
		if param.IsSensitive && value != nil {
			return fmt.Errorf("parameter %q is sensitive and must be supplied by an operator on the canonical approval surface, never by an investigation", name)
		}
		if len(param.Enum) > 0 {
			text, isString := value.(string)
			if !isString {
				return fmt.Errorf("parameter %q must be one of the declared enum values", name)
			}
			allowed := false
			for _, candidate := range param.Enum {
				if candidate == text {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("parameter %q must be one of the declared enum values", name)
			}
		}
	}
	return nil
}
