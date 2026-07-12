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

type TransportRole string

const (
	TransportRoleMutationRequest       TransportRole = "mutation_request"
	TransportRoleAdministrativeRequest TransportRole = "administrative_request"
	TransportRoleOperationQuery        TransportRole = "operation_query"
	TransportRoleOperationResult       TransportRole = "operation_result"
	TransportRoleOperationReceipt      TransportRole = "operation_receipt"
	TransportRoleProtocol              TransportRole = "protocol"
)

// TransportSurface classifies one actual wire-catalog member. Only mutation
// requests may bind lifecycle authority. Queries, results, receipts, and
// protocol messages are explicitly non-admitting and cannot carry a mutation
// registry identity or durable-authority reference.
type TransportSurface struct {
	Name               string
	WireValue          string
	Role               TransportRole
	MutationID         string
	DurableAuthorityID string
}

func AuditTransportSurfaces(surfaces []TransportSurface) error {
	seenNames := make(map[string]struct{}, len(surfaces))
	seenWireValues := make(map[string]struct{}, len(surfaces))
	for _, surface := range surfaces {
		name := strings.TrimSpace(surface.Name)
		wireValue := strings.TrimSpace(surface.WireValue)
		if name == "" || wireValue == "" {
			return fmt.Errorf("transport surface has empty name or wire value: %+v", surface)
		}
		if _, duplicate := seenNames[name]; duplicate {
			return fmt.Errorf("transport surface name %q is classified more than once", name)
		}
		if _, duplicate := seenWireValues[wireValue]; duplicate {
			return fmt.Errorf("transport wire value %q is classified more than once", wireValue)
		}
		seenNames[name] = struct{}{}
		seenWireValues[wireValue] = struct{}{}

		switch surface.Role {
		case TransportRoleMutationRequest:
			if err := AuditRuntimeCandidates([]RuntimeCandidate{{
				Surface:            "transport:" + wireValue,
				MutationID:         surface.MutationID,
				Transport:          true,
				DurableAuthorityID: surface.DurableAuthorityID,
			}}); err != nil {
				return err
			}
		case TransportRoleAdministrativeRequest:
			entry, ok := Lookup(surface.MutationID)
			if !ok || entry.Disposition != DispositionAdministrativeException || surface.DurableAuthorityID != "" {
				return fmt.Errorf("administrative transport %q has invalid registry authority", wireValue)
			}
		case TransportRoleOperationQuery, TransportRoleOperationResult, TransportRoleOperationReceipt, TransportRoleProtocol:
			if strings.TrimSpace(surface.MutationID) != "" || strings.TrimSpace(surface.DurableAuthorityID) != "" {
				return fmt.Errorf("non-admitting transport %q cannot carry mutation authority", wireValue)
			}
		default:
			return fmt.Errorf("transport %q has unknown role %q", wireValue, surface.Role)
		}
	}
	return nil
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
