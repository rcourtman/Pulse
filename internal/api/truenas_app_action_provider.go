package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

type trueNASAppActionProvider struct {
	poller *monitoring.TrueNASPoller
}

func newTrueNASAppActionProvider(poller *monitoring.TrueNASPoller) tools.AppContainerActionProvider {
	if poller == nil {
		return nil
	}
	return &trueNASAppActionProvider{poller: poller}
}

func (p *trueNASAppActionProvider) ExecuteAction(ctx context.Context, req tools.AppContainerActionRequest) (*tools.AppContainerActionResult, error) {
	if p == nil || p.poller == nil {
		return nil, fmt.Errorf("truenas app action provider is unavailable")
	}

	appID := strings.TrimSpace(req.ProviderUID)
	if appID == "" {
		appID = strings.TrimSpace(req.Name)
	}
	app, err := p.poller.ControlApp(ctx, req.OrgID, req.Host, appID, req.Action)
	if err != nil {
		return nil, err
	}

	result := &tools.AppContainerActionResult{
		ResourceID:  strings.TrimSpace(req.ResourceID),
		ProviderUID: strings.TrimSpace(req.ProviderUID),
		Name:        strings.TrimSpace(req.Name),
		Host:        strings.TrimSpace(req.Host),
		Platform:    strings.TrimSpace(req.Platform),
		Action:      strings.TrimSpace(req.Action),
	}
	if app != nil {
		if id := strings.TrimSpace(app.ID); id != "" {
			result.ProviderUID = id
		}
		if name := strings.TrimSpace(app.Name); name != "" {
			result.Name = name
		}
		if state := strings.ToLower(strings.TrimSpace(app.State)); state != "" {
			result.Status = state
		}
	}
	if result.Platform == "" {
		result.Platform = "truenas"
	}
	if result.Output == "" {
		output := fmt.Sprintf("%s app %s", strings.TrimSpace(result.Action), strings.TrimSpace(result.Name))
		if result.Host != "" {
			output = fmt.Sprintf("%s on %s", output, result.Host)
		}
		if result.Status != "" {
			output = fmt.Sprintf("%s; current state=%s", output, result.Status)
		}
		result.Output = output
	}
	return result, nil
}
