package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

type trueNASAppReadProvider struct {
	poller *monitoring.TrueNASPoller
}

func newTrueNASAppReadProvider(poller *monitoring.TrueNASPoller) tools.AppContainerReadProvider {
	if poller == nil {
		return nil
	}
	return &trueNASAppReadProvider{poller: poller}
}

func (p *trueNASAppReadProvider) ReadLogs(ctx context.Context, req tools.AppContainerReadRequest) (*tools.AppContainerReadResult, error) {
	if p == nil || p.poller == nil {
		return nil, fmt.Errorf("truenas app read provider is unavailable")
	}

	appID := strings.TrimSpace(req.ProviderUID)
	if appID == "" {
		appID = strings.TrimSpace(req.Name)
	}
	result, err := p.poller.ReadAppLogs(ctx, req.OrgID, req.Host, appID, req.Container, req.Lines)
	if err != nil {
		return nil, err
	}

	readResult := &tools.AppContainerReadResult{
		ResourceID:  strings.TrimSpace(req.ResourceID),
		ProviderUID: strings.TrimSpace(req.ProviderUID),
		Name:        strings.TrimSpace(req.Name),
		Host:        strings.TrimSpace(req.Host),
		Platform:    strings.TrimSpace(req.Platform),
		Container:   strings.TrimSpace(req.Container),
		Lines:       req.Lines,
	}
	if result != nil {
		if id := strings.TrimSpace(result.App.ID); id != "" {
			readResult.ProviderUID = id
		}
		if name := strings.TrimSpace(result.App.Name); name != "" {
			readResult.Name = name
		}
		if host := strings.TrimSpace(result.Host); host != "" {
			readResult.Host = host
		}
		if containerName := strings.TrimSpace(result.Container.ServiceName); containerName != "" {
			readResult.Container = containerName
		} else if containerID := strings.TrimSpace(result.Container.ID); containerID != "" {
			readResult.Container = containerID
		}
		if result.TailLines > 0 {
			readResult.Lines = result.TailLines
		}
		readResult.Output = formatTrueNASAppLogOutput(result.Lines)
	}
	if readResult.Platform == "" {
		readResult.Platform = "truenas"
	}
	return readResult, nil
}

func formatTrueNASAppLogOutput(lines []truenas.AppLogLine) string {
	if len(lines) == 0 {
		return ""
	}
	formatted := make([]string, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(line.Data)
		if text == "" {
			continue
		}
		if timestamp := strings.TrimSpace(line.Timestamp); timestamp != "" {
			formatted = append(formatted, fmt.Sprintf("%s %s", timestamp, text))
			continue
		}
		formatted = append(formatted, text)
	}
	return strings.Join(formatted, "\n")
}
