package mutationregistry

import (
	"fmt"
	"strings"
)

// RuntimeCandidate is emitted by executable surface audits from the real model
// registry, HTTP router, job registration, or transport command catalog.
type RuntimeCandidate struct {
	Surface            string
	MutationID         string
	Transport          bool
	DurableAuthorityID string
}

// AuditRuntimeCandidates requires every mechanically discovered mutation
// candidate to resolve to exactly one closed registry disposition. Transport
// candidates additionally name the lifecycle authority that must exist before
// delivery; retired transports must name no authority.
func AuditRuntimeCandidates(candidates []RuntimeCandidate) error {
	seen := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		surface := strings.TrimSpace(candidate.Surface)
		if surface == "" || strings.TrimSpace(candidate.MutationID) == "" {
			return fmt.Errorf("mutation candidate has empty surface or id: %+v", candidate)
		}
		if prior, duplicate := seen[surface]; duplicate {
			return fmt.Errorf("mutation surface %q is classified more than once (%s, %s)", surface, prior, candidate.MutationID)
		}
		seen[surface] = candidate.MutationID
		entry, ok := Lookup(candidate.MutationID)
		if !ok {
			return fmt.Errorf("mutation surface %q has unregistered id %q", surface, candidate.MutationID)
		}
		if !candidate.Transport {
			continue
		}
		if entry.Disposition == DispositionLifecycle {
			authority, ok := Lookup(candidate.DurableAuthorityID)
			if !ok || authority.Disposition != DispositionLifecycle || authority.Origin == OriginTransport {
				return fmt.Errorf("transport %q has no non-transport lifecycle authority", surface)
			}
			if entry.Delivery != DeliveryCommittedLifecycle {
				return fmt.Errorf("transport %q can deliver before committed authority", surface)
			}
		} else if candidate.DurableAuthorityID != "" {
			return fmt.Errorf("denied transport %q must not name executable authority", surface)
		}
	}
	return nil
}
