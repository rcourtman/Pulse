package api

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

type assistantTypedActionPlanner struct {
	resources *ResourceHandlers
}

func (p assistantTypedActionPlanner) PlanTypedAction(ctx context.Context, orgID string, req unified.ActionRequest) (*unified.ActionPlan, error) {
	actor := unified.ActionActor{SubjectID: "pulse_assistant", Kind: unified.ActionActorService, CredentialID: "service:assistant", OrgID: orgID}
	lifecycle := p.resources.ActionLifecycle()

	// Operator state that blocks remediation (Never auto-remediate, or a retired
	// lifecycle) already refuses every dispatch for the resource, approved or not.
	// Refuse the Assistant's plan too, so it reports the block instead of leaving
	// a plan in Actions that can never run. The state is read through the same
	// tenant-scoped lifecycle store dispatch uses. A state that cannot be read is
	// not a refusal here: planning executes nothing and dispatch re-checks.
	if state, found, err := lifecycle.ResourceOperatorState(orgID, req.ResourceID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Str("resource_id", req.ResourceID).
			Msg("Assistant planning could not read resource operator state; dispatch will re-check")
	} else if found && state.BlocksRemediation() {
		return nil, fmt.Errorf("%w: %s", unified.ErrResourceRemediationLocked, unified.CanonicalResourceID(req.ResourceID))
	}

	// The Assistant plans under its own service identity for the chat
	// caller. Only a caller allowed to run host commands (action-execute)
	// may have the agent check the plan; dispatch checks it either way.
	ctx = actionlifecycle.WithLiveCheckAuthority(ctx, chat.HostCommandAuthorityFromContext(ctx))
	plan, err := lifecycle.Plan(ctx, orgID, req, actor)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}
