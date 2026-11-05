package actionlifecycle

import (
	"context"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// AvailabilityBatcher is implemented by a checker whose per-capability checks
// repeat expensive lookups across resources. The returned context scopes those
// lookups to one projection pass, so a pass sees one answer for each lookup.
// It must be used by one goroutine and dropped when the pass ends.
type AvailabilityBatcher interface {
	BeginAvailabilityBatch(ctx context.Context) context.Context
}

// ProjectActionAvailability is the REST projection: refused capabilities move
// out of Capabilities and into ActionReadiness. It writes only top-level fields
// of the given elements, so callers pass a flat copy of shared resources.
func ProjectActionAvailability(ctx context.Context, checker AvailabilityChecker, resources []unified.Resource) {
	if checker == nil {
		return
	}
	ctx = beginAvailabilityBatch(ctx, checker)
	for i := range resources {
		resource := &resources[i]
		if len(resource.Capabilities) == 0 {
			continue
		}
		filtered := make([]unified.ResourceCapability, 0, len(resource.Capabilities))
		var refusals []unified.ResourceActionReadiness
		for _, capability := range resource.Capabilities {
			refusal, refused := checkCapability(ctx, checker, resource, capability.Name)
			if refused {
				refusals = append(refusals, refusal)
				continue
			}
			filtered = append(filtered, capability)
		}
		if len(filtered) == 0 {
			resource.Capabilities = nil
		} else {
			resource.Capabilities = filtered
		}
		if len(refusals) > 0 {
			resource.ActionReadiness = refusals
		}
	}
}

// AttachActionRefusals is the broadcast projection: ActionReadiness gains each
// refusal and Capabilities stays whole. A broadcast keeps a stable capability
// set for a row while its readiness moves with the agent's connection, and the
// resource drawer's operator policy needs the capabilities a host offers even
// while the host is disconnected. It writes only top-level fields of the given
// elements and does not allocate for a resource with nothing refused.
func AttachActionRefusals(ctx context.Context, checker AvailabilityChecker, resources []unified.Resource) {
	if checker == nil {
		return
	}
	ctx = beginAvailabilityBatch(ctx, checker)
	for i := range resources {
		resource := &resources[i]
		var refusals []unified.ResourceActionReadiness
		for _, capability := range resource.Capabilities {
			if refusal, refused := checkCapability(ctx, checker, resource, capability.Name); refused {
				refusals = append(refusals, refusal)
			}
		}
		if len(refusals) > 0 {
			resource.ActionReadiness = refusals
		}
	}
}

func beginAvailabilityBatch(ctx context.Context, checker AvailabilityChecker) context.Context {
	if batcher, ok := checker.(AvailabilityBatcher); ok {
		return batcher.BeginAvailabilityBatch(ctx)
	}
	return ctx
}

func checkCapability(ctx context.Context, checker AvailabilityChecker, resource *unified.Resource, capabilityName string) (unified.ResourceActionReadiness, bool) {
	readiness := checker.CheckActionAvailable(ctx, unified.ActionRequest{
		RequestID:      "resource-capability-availability",
		ResourceID:     resource.ID,
		CapabilityName: capabilityName,
		Reason:         "Projecting currently executable resource capabilities.",
		RequestedBy:    "system:resource-api",
	}, *resource)
	return readiness, readiness.Name != "" && !readiness.Available
}
