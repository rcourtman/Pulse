package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

type trueNASAppConfigProvider struct {
	poller *monitoring.TrueNASPoller
}

func newTrueNASAppConfigProvider(poller *monitoring.TrueNASPoller) tools.AppContainerConfigProvider {
	if poller == nil {
		return nil
	}
	return &trueNASAppConfigProvider{poller: poller}
}

func (p *trueNASAppConfigProvider) GetConfig(ctx context.Context, req tools.AppContainerConfigRequest) (*tools.AppContainerConfigResult, error) {
	if p == nil || p.poller == nil {
		return nil, fmt.Errorf("truenas app config provider is unavailable")
	}

	appID := strings.TrimSpace(req.ProviderUID)
	if appID == "" {
		appID = strings.TrimSpace(req.Name)
	}
	result, err := p.poller.GetAppConfig(ctx, req.OrgID, req.Host, appID)
	if err != nil {
		return nil, err
	}

	configResult := &tools.AppContainerConfigResult{
		ResourceID:  strings.TrimSpace(req.ResourceID),
		ProviderUID: strings.TrimSpace(req.ProviderUID),
		Name:        strings.TrimSpace(req.Name),
		Host:        strings.TrimSpace(req.Host),
		Platform:    strings.TrimSpace(req.Platform),
	}
	if result != nil {
		if id := strings.TrimSpace(result.App.ID); id != "" {
			configResult.ProviderUID = id
		}
		if name := strings.TrimSpace(result.App.Name); name != "" {
			configResult.Name = name
		}
		if host := strings.TrimSpace(result.Host); host != "" {
			configResult.Host = host
		}
		configResult.Status = strings.ToLower(strings.TrimSpace(result.App.State))
		configResult.Version = strings.TrimSpace(result.App.Version)
		configResult.HumanVersion = strings.TrimSpace(result.App.HumanVersion)
		configResult.Notes = strings.TrimSpace(result.App.Notes)
		configResult.CustomApp = result.App.CustomApp
		configResult.UpgradeAvailable = result.App.UpgradeAvailable
		configResult.ImageUpdatesAvailable = result.App.ImageUpdatesAvailable
		configResult.ContainerCount = result.App.ContainerCount
		if configResult.ContainerCount <= 0 {
			configResult.ContainerCount = len(result.App.Containers)
		}
		configResult.UsedHostIPs = append([]string{}, result.App.UsedHostIPs...)
		configResult.Images = append([]string{}, result.App.Images...)
		configResult.Ports = mapTrueNASAppPortsToToolPorts(result.App.UsedPorts)
		configResult.Networks = mapTrueNASAppNetworksToToolNetworks(result.App.Networks)
		configResult.Mounts = mapTrueNASAppVolumesToToolMounts(result.App.Volumes)
		configResult.Containers = mapTrueNASAppContainersToToolContainers(result.App.Containers)
	}
	if configResult.Platform == "" {
		configResult.Platform = "truenas"
	}
	return configResult, nil
}

func mapTrueNASAppPortsToToolPorts(ports []truenas.AppPort) []tools.PortInfo {
	if len(ports) == 0 {
		return nil
	}
	out := make([]tools.PortInfo, 0, len(ports))
	for _, port := range ports {
		protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
		if len(port.HostPorts) == 0 {
			out = append(out, tools.PortInfo{
				Private:  port.ContainerPort,
				Protocol: protocol,
			})
			continue
		}
		for _, hostPort := range port.HostPorts {
			out = append(out, tools.PortInfo{
				Private:  port.ContainerPort,
				Public:   hostPort.HostPort,
				Protocol: protocol,
				IP:       strings.TrimSpace(hostPort.HostIP),
			})
		}
	}
	return out
}

func mapTrueNASAppNetworksToToolNetworks(networks []truenas.AppNetwork) []tools.NetworkInfo {
	if len(networks) == 0 {
		return nil
	}
	out := make([]tools.NetworkInfo, 0, len(networks))
	for _, network := range networks {
		name := strings.TrimSpace(network.Name)
		if name == "" {
			name = strings.TrimSpace(network.ID)
		}
		out = append(out, tools.NetworkInfo{Name: name})
	}
	return out
}

func mapTrueNASAppVolumesToToolMounts(volumes []truenas.AppVolume) []tools.MountInfo {
	if len(volumes) == 0 {
		return nil
	}
	out := make([]tools.MountInfo, 0, len(volumes))
	for _, volume := range volumes {
		out = append(out, tools.MountInfo{
			Source:      strings.TrimSpace(volume.Source),
			Destination: strings.TrimSpace(volume.Destination),
			ReadWrite:   !strings.EqualFold(strings.TrimSpace(volume.Mode), "ro"),
		})
	}
	return out
}

func mapTrueNASAppContainersToToolContainers(containers []truenas.AppContainer) []tools.AppContainerConfigContainer {
	if len(containers) == 0 {
		return nil
	}
	out := make([]tools.AppContainerConfigContainer, 0, len(containers))
	for _, container := range containers {
		out = append(out, tools.AppContainerConfigContainer{
			ID:      strings.TrimSpace(container.ID),
			Service: strings.TrimSpace(container.ServiceName),
			Image:   strings.TrimSpace(container.Image),
			State:   strings.ToLower(strings.TrimSpace(container.State)),
			Ports:   mapTrueNASAppPortsToToolPorts(container.PortConfig),
			Mounts:  mapTrueNASAppVolumesToToolMounts(container.VolumeMounts),
		})
	}
	return out
}
