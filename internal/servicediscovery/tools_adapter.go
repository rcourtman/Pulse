package servicediscovery

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// ToolsAdapter wraps Service to implement tools.DiscoverySource
type ToolsAdapter struct {
	service *Service
}

// NewToolsAdapter creates a new adapter for the discovery service
func NewToolsAdapter(service *Service) *ToolsAdapter {
	if service == nil {
		return nil
	}
	return &ToolsAdapter{service: service}
}

// GetDiscovery implements tools.DiscoverySource
func (a *ToolsAdapter) GetDiscovery(id string) (tools.DiscoverySourceData, error) {
	discovery, err := a.service.GetDiscovery(id)
	if err != nil {
		return tools.DiscoverySourceData{}, err
	}
	if discovery == nil {
		return tools.DiscoverySourceData{}, nil
	}
	return a.convertToSourceData(discovery), nil
}

// GetDiscoveryByResource implements tools.DiscoverySource
func (a *ToolsAdapter) GetDiscoveryByResource(resourceType, hostID, resourceID string) (tools.DiscoverySourceData, error) {
	discovery, err := a.service.GetDiscoveryByResource(ResourceType(resourceType), hostID, resourceID)
	if err != nil {
		return tools.DiscoverySourceData{}, err
	}
	if discovery == nil {
		return tools.DiscoverySourceData{}, nil
	}
	return a.convertToSourceData(discovery), nil
}

// ListDiscoveries implements tools.DiscoverySource
func (a *ToolsAdapter) ListDiscoveries() ([]tools.DiscoverySourceData, error) {
	discoveries, err := a.service.ListDiscoveries()
	if err != nil {
		return nil, err
	}
	return a.convertList(discoveries), nil
}

// ListDiscoveriesByType implements tools.DiscoverySource
func (a *ToolsAdapter) ListDiscoveriesByType(resourceType string) ([]tools.DiscoverySourceData, error) {
	discoveries, err := a.service.ListDiscoveriesByType(ResourceType(resourceType))
	if err != nil {
		return nil, err
	}
	return a.convertList(discoveries), nil
}

// ListDiscoveriesByHost implements tools.DiscoverySource
func (a *ToolsAdapter) ListDiscoveriesByHost(hostID string) ([]tools.DiscoverySourceData, error) {
	discoveries, err := a.service.ListDiscoveriesByHost(hostID)
	if err != nil {
		return nil, err
	}
	return a.convertList(discoveries), nil
}

// FormatForAIContext implements tools.DiscoverySource
func (a *ToolsAdapter) FormatForAIContext(sourceData []tools.DiscoverySourceData) string {
	// Convert back to ResourceDiscovery for formatting
	discoveries := make([]*ResourceDiscovery, 0, len(sourceData))
	for _, sd := range sourceData {
		discoveries = append(discoveries, a.convertFromSourceData(sd))
	}
	return FormatForAIContext(discoveries)
}

// TriggerDiscovery implements tools.DiscoverySource - initiates discovery for a resource
func (a *ToolsAdapter) TriggerDiscovery(ctx context.Context, resourceType, hostID, resourceID string) (tools.DiscoverySourceData, error) {
	req := DiscoveryRequest{
		ResourceType: ResourceType(resourceType),
		HostID:       hostID,
		ResourceID:   resourceID,
		Force:        false, // Don't force if recently discovered
	}

	discovery, err := a.service.DiscoverResource(ctx, req)
	if err != nil {
		return tools.DiscoverySourceData{}, err
	}
	if discovery == nil {
		return tools.DiscoverySourceData{}, nil
	}
	return a.convertToSourceData(discovery), nil
}

func (a *ToolsAdapter) convertToSourceData(d *ResourceDiscovery) tools.DiscoverySourceData {
	facts := make([]tools.DiscoverySourceFact, 0, len(d.Facts))
	for _, f := range d.Facts {
		facts = append(facts, tools.DiscoverySourceFact{
			Category:   string(f.Category),
			Key:        f.Key,
			Value:      f.Value,
			Source:     f.Source,
			Confidence: f.Confidence,
		})
	}

	ports := make([]tools.DiscoverySourcePort, 0, len(d.Ports))
	for _, p := range d.Ports {
		ports = append(ports, tools.DiscoverySourcePort{
			Port:     p.Port,
			Protocol: p.Protocol,
			Process:  p.Process,
			Address:  p.Address,
		})
	}

	dockerMounts := make([]tools.DiscoverySourceDockerMount, 0, len(d.DockerMounts))
	for _, m := range d.DockerMounts {
		dockerMounts = append(dockerMounts, tools.DiscoverySourceDockerMount{
			ContainerName: m.ContainerName,
			Source:        m.Source,
			Destination:   m.Destination,
			Type:          m.Type,
			ReadOnly:      m.ReadOnly,
		})
	}

	return tools.DiscoverySourceData{
		ID:             d.ID,
		ResourceType:   string(d.ResourceType),
		ResourceID:     d.ResourceID,
		HostID:         d.HostID,
		Hostname:       d.Hostname,
		ServiceType:    d.ServiceType,
		ServiceName:    d.ServiceName,
		ServiceVersion: d.ServiceVersion,
		Category:       string(d.Category),
		CLIAccess:      d.CLIAccess,
		Facts:          facts,
		ConfigPaths:    d.ConfigPaths,
		DataPaths:      d.DataPaths,
		LogPaths:       d.LogPaths,
		Ports:          ports,
		DockerMounts:   dockerMounts,
		UserNotes:      d.UserNotes,
		Confidence:     d.Confidence,
		AIReasoning:    d.AIReasoning,
		DiscoveredAt:   d.DiscoveredAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func (a *ToolsAdapter) convertFromSourceData(sd tools.DiscoverySourceData) *ResourceDiscovery {
	facts := make([]DiscoveryFact, 0, len(sd.Facts))
	for _, f := range sd.Facts {
		facts = append(facts, DiscoveryFact{
			Category:   FactCategory(f.Category),
			Key:        f.Key,
			Value:      f.Value,
			Source:     f.Source,
			Confidence: f.Confidence,
		})
	}

	ports := make([]PortInfo, 0, len(sd.Ports))
	for _, p := range sd.Ports {
		ports = append(ports, PortInfo{
			Port:     p.Port,
			Protocol: p.Protocol,
			Process:  p.Process,
			Address:  p.Address,
		})
	}

	dockerMounts := make([]DockerBindMount, 0, len(sd.DockerMounts))
	for _, m := range sd.DockerMounts {
		dockerMounts = append(dockerMounts, DockerBindMount{
			ContainerName: m.ContainerName,
			Source:        m.Source,
			Destination:   m.Destination,
			Type:          m.Type,
			ReadOnly:      m.ReadOnly,
		})
	}

	return &ResourceDiscovery{
		ID:             sd.ID,
		ResourceType:   ResourceType(sd.ResourceType),
		ResourceID:     sd.ResourceID,
		HostID:         sd.HostID,
		Hostname:       sd.Hostname,
		ServiceType:    sd.ServiceType,
		ServiceName:    sd.ServiceName,
		ServiceVersion: sd.ServiceVersion,
		Category:       ServiceCategory(sd.Category),
		CLIAccess:      sd.CLIAccess,
		Facts:          facts,
		ConfigPaths:    sd.ConfigPaths,
		DataPaths:      sd.DataPaths,
		LogPaths:       sd.LogPaths,
		Ports:          ports,
		DockerMounts:   dockerMounts,
		UserNotes:      sd.UserNotes,
		Confidence:     sd.Confidence,
		AIReasoning:    sd.AIReasoning,
		DiscoveredAt:   sd.DiscoveredAt,
		UpdatedAt:      sd.UpdatedAt,
	}
}

func (a *ToolsAdapter) convertList(discoveries []*ResourceDiscovery) []tools.DiscoverySourceData {
	result := make([]tools.DiscoverySourceData, 0, len(discoveries))
	for _, d := range discoveries {
		if d != nil {
			result = append(result, a.convertToSourceData(d))
		}
	}
	return result
}
