package tools

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// ToolGovernanceSurface names the Pulse Intelligence surface requesting a
// registry-owned fallback manifest.
type ToolGovernanceSurface string

const (
	ToolGovernanceSurfacePulseIntelligence ToolGovernanceSurface = "pulse_intelligence"
	ToolGovernanceSurfacePulseAssistant    ToolGovernanceSurface = "pulse_assistant"
	ToolGovernanceSurfacePulsePatrol       ToolGovernanceSurface = "pulse_patrol"
)

// CanonicalToolGovernance returns the complete registry-owned governance
// descriptors available at a control level. Runtime turns should prefer
// PulseToolExecutor.ListToolGovernance so provider availability is still
// honored.
func CanonicalToolGovernance(controlLevel ControlLevel) []ToolGovernanceDescriptor {
	e := NewPulseToolExecutor(ExecutorConfig{})
	return e.registry.ListToolGovernance(controlLevel)
}

// CanonicalToolGovernanceForSurface returns the registry-owned fallback
// manifest projected for one Pulse Intelligence surface. The registry remains
// the source of truth; this helper only removes tools that belong to a
// different first-party runtime surface when no live executor is attached.
func CanonicalToolGovernanceForSurface(controlLevel ControlLevel, surface ToolGovernanceSurface) []ToolGovernanceDescriptor {
	governance := CanonicalToolGovernance(controlLevel)
	if len(governance) == 0 {
		return governance
	}

	projected := make([]ToolGovernanceDescriptor, 0, len(governance))
	for _, tool := range governance {
		if !toolGovernanceSurfaceIncludesTool(surface, tool.Name) {
			continue
		}
		projected = append(projected, tool)
	}
	return projected
}

// CanonicalToolGovernanceForManifestSurface returns registry-owned fallback
// governance for a manifest-declared Pulse Intelligence surface. The registry
// remains the source of tool governance; the shared manifest decides whether
// that operator surface may advertise tools at all.
func CanonicalToolGovernanceForManifestSurface(controlLevel ControlLevel, manifest agentcapabilities.Manifest, surfaceID string) []ToolGovernanceDescriptor {
	affordances, _ := agentcapabilities.ManifestSurfaceAffordances(manifest, surfaceID)
	if !affordances.Tools {
		return []ToolGovernanceDescriptor{}
	}
	return CanonicalToolGovernanceForSurface(controlLevel, toolGovernanceSurfaceForManifestSurfaceID(surfaceID))
}

func toolGovernanceSurfaceForManifestSurfaceID(surfaceID string) ToolGovernanceSurface {
	switch strings.TrimSpace(surfaceID) {
	case agentcapabilities.SurfaceIDPulseAssistant:
		return ToolGovernanceSurfacePulseAssistant
	default:
		return ToolGovernanceSurfacePulseIntelligence
	}
}

func toolGovernanceSurfaceIncludesTool(surface ToolGovernanceSurface, name string) bool {
	switch surface {
	case ToolGovernanceSurfacePulseAssistant:
		return !isPatrolRuntimeTool(name)
	case ToolGovernanceSurfacePulseIntelligence, ToolGovernanceSurfacePulsePatrol, "":
		return true
	default:
		return true
	}
}

func isPatrolRuntimeTool(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "patrol_")
}
