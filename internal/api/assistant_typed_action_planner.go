package api

import (
	"context"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type assistantTypedActionPlanner struct {
	resources *ResourceHandlers
}

func (p assistantTypedActionPlanner) PlanTypedAction(ctx context.Context, orgID string, req unified.ActionRequest) (*unified.ActionPlan, error) {
	plan, err := p.resources.ActionLifecycle().Plan(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}
