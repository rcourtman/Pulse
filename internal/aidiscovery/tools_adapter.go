package aidiscovery

import (
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

func (a *ToolsAdapter) convertToSourceData(d *ResourceDiscovery) tools.DiscoverySourceData {
	facts := make([]tools.DiscoverySourceFact, 0, len(d.Facts))
	for _, f := range d.Facts {
		facts = append(facts, tools.DiscoverySourceFact{
			Category: string(f.Category),
			Key:      f.Key,
			Value:    f.Value,
			Source:   f.Source,
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
			Category: FactCategory(f.Category),
			Key:      f.Key,
			Value:    f.Value,
			Source:   f.Source,
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
