package tools

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// resolvedResourceLister is the optional session-context extension that lets
// the executor enumerate every resource the session has resolved so far. The
// chat ResolvedContext implements it; narrower test doubles may omit it.
type resolvedResourceLister interface {
	ListResolvedResources() []ResolvedResourceInfo
}

// controlTarget is the canonical binding pulse_control plans against. The
// session entry is the alias index the model saw in earlier query output; the
// canonical resource is the unified-inventory record whose ID the shared
// action lifecycle keys on and whose capability list is the only source of
// truth for "advertised".
type controlTarget struct {
	session   ResolvedResourceInfo
	canonical *unifiedresources.Resource
}

// canonicalID returns the identifier the action lifecycle registry keys on.
// A session-only binding (no unified provider wired) falls back to the
// session ID so narrow deployments keep working.
func (t controlTarget) canonicalID() string {
	if t.canonical != nil {
		if id := unifiedresources.CanonicalResourceID(t.canonical.ID); id != "" {
			return id
		}
	}
	if t.session != nil {
		return unifiedresources.CanonicalResourceID(t.session.GetResourceID())
	}
	return ""
}

func (t controlTarget) displayName() string {
	if t.canonical != nil {
		if name := strings.TrimSpace(resourceDisplayName(*t.canonical)); name != "" {
			return name
		}
	}
	if t.session != nil {
		for _, alias := range t.session.GetAliases() {
			if alias = strings.TrimSpace(alias); alias != "" {
				return alias
			}
		}
		return strings.TrimSpace(t.session.GetResourceID())
	}
	return ""
}

// AdvertisedActionTarget names a session-resolved canonical resource that
// currently advertises a lifecycle capability. The agentic loop uses it to
// refuse a final answer that narrates an advertised action instead of
// submitting it through pulse_control.
type AdvertisedActionTarget struct {
	CanonicalID string
	Name        string
	Kind        string
	Capability  string
}

// lifecycleActionSynonym mirrors the action lifecycle's capability synonym
// table: Proxmox guests advertise "reboot" while container platforms
// advertise "restart", and operators use the words interchangeably.
func lifecycleActionSynonym(action string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "restart":
		return "reboot", true
	case "reboot":
		return "restart", true
	}
	return "", false
}

// advertisedCapabilityNames lists the resource's current capability names in a
// stable order for tool evidence.
func advertisedCapabilityNames(resource unifiedresources.Resource) []string {
	names := canonicalCapabilityActions(resource)
	sort.Strings(names)
	return names
}

// advertisedActionName reports the capability name the resource advertises
// for a requested lifecycle action, following the same synonym rule the
// action lifecycle applies when it plans.
func advertisedActionName(resource unifiedresources.Resource, action string) (string, bool) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return "", false
	}
	if _, found := actionplanner.FindCapability(resource.Capabilities, action); found {
		return action, true
	}
	if synonym, ok := lifecycleActionSynonym(action); ok {
		if _, found := actionplanner.FindCapability(resource.Capabilities, synonym); found {
			return synonym, true
		}
	}
	return "", false
}

var controlCandidateResourceTypes = []unifiedresources.ResourceType{
	unifiedresources.ResourceTypeVM,
	unifiedresources.ResourceTypeSystemContainer,
	unifiedresources.ResourceTypeAppContainer,
	unifiedresources.ResourceTypeAgent,
}

func controlResourceTypeForKind(kind string) (unifiedresources.ResourceType, bool) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "vm":
		return unifiedresources.ResourceTypeVM, true
	case "system-container", "lxc":
		return unifiedresources.ResourceTypeSystemContainer, true
	case "app-container":
		return unifiedresources.ResourceTypeAppContainer, true
	case "agent", "node", "docker-host":
		return unifiedresources.ResourceTypeAgent, true
	}
	return "", false
}

// canonicalResourceForResolved maps a session-resolved resource back to its
// unified-inventory record. It matches on the canonical ID that query tools
// register as an alias, then on provider identity (Proxmox VMID on the same
// node, app-container provider ID on the same host, agent name). It never
// matches on host names, IP addresses, or tags, which the alias list also
// carries: those are lookup conveniences, not identity, and a write must not
// bind to a neighbour by accident.
func canonicalResourceForResolved(provider UnifiedResourceProvider, resolved ResolvedResourceInfo) (unifiedresources.Resource, bool) {
	if provider == nil || resolved == nil {
		return unifiedresources.Resource{}, false
	}
	resourceType, ok := controlResourceTypeForKind(firstNonEmptyString(resolved.GetKind(), resolved.GetResourceType()))
	if !ok {
		return unifiedresources.Resource{}, false
	}
	candidates := provider.GetByType(resourceType)
	for _, alias := range resolved.GetAliases() {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		for _, resource := range candidates {
			if strings.EqualFold(strings.TrimSpace(resource.ID), alias) {
				return resource, true
			}
		}
	}
	switch resourceType {
	case unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeSystemContainer:
		vmid := resolved.GetVMID()
		node := strings.TrimSpace(resolved.GetNode())
		if vmid <= 0 {
			return unifiedresources.Resource{}, false
		}
		for _, resource := range candidates {
			if resource.Proxmox == nil || resource.Proxmox.VMID != vmid {
				continue
			}
			if node == "" || strings.EqualFold(strings.TrimSpace(resource.Proxmox.NodeName), node) {
				return resource, true
			}
		}
	case unifiedresources.ResourceTypeAppContainer:
		providerUID := strings.TrimSpace(resolved.GetProviderUID())
		host := strings.TrimSpace(resolved.GetTargetHost())
		if providerUID == "" {
			return unifiedresources.Resource{}, false
		}
		for _, resource := range candidates {
			if !strings.EqualFold(strings.TrimSpace(appContainerProviderID(resource)), providerUID) {
				continue
			}
			if host == "" || strings.EqualFold(strings.TrimSpace(canonicalAppContainerHost(resource)), host) {
				return resource, true
			}
		}
	case unifiedresources.ResourceTypeAgent:
		for _, resource := range candidates {
			name := strings.TrimSpace(resourceDisplayName(resource))
			for _, alias := range resolved.GetAliases() {
				if name != "" && strings.EqualFold(name, strings.TrimSpace(alias)) {
					return resource, true
				}
			}
		}
	}
	return unifiedresources.Resource{}, false
}

// canonicalControlCandidates resolves a model-supplied reference against the
// unified inventory. An exact canonical-ID match wins outright; otherwise every
// control-capable resource whose display name or name equals the reference is
// returned so the caller can refuse an ambiguous write instead of guessing.
func canonicalControlCandidates(provider UnifiedResourceProvider, ref string) []unifiedresources.Resource {
	ref = strings.TrimSpace(ref)
	if provider == nil || ref == "" {
		return nil
	}
	var byName []unifiedresources.Resource
	for _, resourceType := range controlCandidateResourceTypes {
		for _, resource := range provider.GetByType(resourceType) {
			if strings.EqualFold(strings.TrimSpace(resource.ID), ref) {
				return []unifiedresources.Resource{resource}
			}
			if strings.EqualFold(strings.TrimSpace(resourceDisplayName(resource)), ref) ||
				strings.EqualFold(strings.TrimSpace(resource.Name), ref) {
				byName = append(byName, resource)
			}
		}
	}
	return byName
}

// resolveControlTarget binds a pulse_control reference to a canonical
// resource. Session context is consulted first because it is what the model
// just saw, but a reference that is absent from the session and resolves
// uniquely in the unified inventory is registered and accepted: the inventory
// is Pulse's own discovery, so demanding a second in-session "discovery" step
// is an ordering accident, not a safety property. Capability, approval, and
// execution stay with the shared action lifecycle.
func (e *PulseToolExecutor) resolveControlTarget(ref, action string) (controlTarget, *CallToolResult) {
	target := controlTarget{}
	if e.resolvedContext != nil {
		if res, ok := e.resolvedContext.GetResolvedResourceByAlias(ref); ok && res != nil {
			target.session = res
		} else if res, ok := e.resolvedContext.GetResolvedResourceByID(ref); ok && res != nil {
			target.session = res
		}
	}

	if e.unifiedResourceProvider != nil {
		if target.session != nil {
			if resource, ok := canonicalResourceForResolved(e.unifiedResourceProvider, target.session); ok {
				target.canonical = &resource
			}
		}
		if target.canonical == nil {
			candidates := canonicalControlCandidates(e.unifiedResourceProvider, ref)
			switch len(candidates) {
			case 0:
			case 1:
				resource := candidates[0]
				target.canonical = &resource
				if target.session == nil && e.resolvedContext != nil {
					if reg, ok := CanonicalHandoffResourceRegistration(e.unifiedResourceProvider, resource.ID, "", string(unifiedresources.ContractResourceType(resource)), ""); ok {
						e.registerResolvedResourceWithExplicitAccess(reg)
						if res, ok := e.resolvedContext.GetResolvedResourceByID(resource.ID); ok && res != nil {
							target.session = res
						} else if res, ok := e.resolvedContext.GetResolvedResourceByAlias(reg.Name); ok && res != nil {
							target.session = res
						}
					}
				}
			default:
				ids := make([]string, 0, len(candidates))
				for _, candidate := range candidates {
					ids = append(ids, fmt.Sprintf("%s (%s)", candidate.ID, unifiedresources.ContractResourceType(candidate)))
				}
				sort.Strings(ids)
				result := NewToolResponseResult(NewToolBlockedError(
					agentcapabilities.ErrCodeInvalidInput,
					fmt.Sprintf("%d canonical resources are named %q; call pulse_control again with one of these canonical resource ids: %s.", len(candidates), ref, strings.Join(ids, ", ")),
					map[string]interface{}{
						"resource_id":     ref,
						"action":          action,
						"candidates":      ids,
						"policy_boundary": "Ambiguous target reference. Retry with a canonical resource id from this list; this is a lookup detail, not a missing prerequisite.",
					},
				))
				return target, &result
			}
		}
	}

	if target.session == nil && target.canonical == nil {
		if isStrictResolutionEnabled() && isWriteAction(action) {
			if e.telemetryCallback != nil {
				e.telemetryCallback.RecordStrictResolutionBlock("pulse_control", action)
			}
			strict := &ErrStrictResolution{
				ResourceID: ref,
				Action:     action,
				Message:    fmt.Sprintf("No resolved or canonical resource matches %q. Call pulse_query action=search query=%q, then retry pulse_control with a returned name or canonical resource id before performing %q.", ref, ref, action),
			}
			result := NewToolResponseResult(strict.ToToolResponse())
			return target, &result
		}
		result := NewToolResponseResult(NewToolBlockedError(
			agentcapabilities.ErrCodeNotFound,
			fmt.Sprintf("No canonical resource matches %q. Call pulse_query action=search query=%q to list matching resources, then call pulse_control again with a returned name or canonical resource id.", ref, ref),
			map[string]interface{}{
				"resource_id":     ref,
				"action":          action,
				"policy_boundary": "Target lookup miss. Retry with a name or canonical id returned by pulse_query; this is a lookup detail, not a missing prerequisite, and the user does not need to do anything.",
			},
		))
		return target, &result
	}

	return target, nil
}

// controlPlanFailureResult turns a planning error into tool evidence the model
// can report faithfully. A capability the resource does not advertise is a
// real boundary and is described with the resource's current capability list;
// anything else is passed through unchanged.
func controlPlanFailureResult(target controlTarget, action string, err error) CallToolResult {
	if err == nil {
		return NewErrorResult(fmt.Errorf("canonical action planning failed"))
	}
	if !errors.Is(err, actionplanner.ErrCapabilityNotFound) {
		return NewErrorResult(err)
	}
	name := target.displayName()
	details := map[string]interface{}{
		"resource_id":      target.canonicalID(),
		"requested_action": action,
		"policy_boundary":  "The resource does not currently advertise this capability. Report exactly this boundary; do not invent other prerequisites or redirect the user to manual commands.",
	}
	message := fmt.Sprintf("%q is not permitted on %s: the resource does not currently advertise that capability.", action, name)
	if target.canonical != nil {
		advertised := advertisedCapabilityNames(*target.canonical)
		details["advertised_capabilities"] = advertised
		details["status"] = string(target.canonical.Status)
		if len(advertised) > 0 {
			message = fmt.Sprintf("%q is not permitted on %s: it does not currently advertise that capability; its advertised capabilities right now are %s (status %s).", action, name, strings.Join(advertised, ", "), target.canonical.Status)
		} else {
			message = fmt.Sprintf("%q is not permitted on %s: it does not currently advertise that capability or any other lifecycle capability (status %s).", action, name, target.canonical.Status)
		}
	}
	return NewToolResponseResult(NewToolBlockedError(agentcapabilities.ErrCodeActionNotAllowed, message, details))
}

// SessionTargetsAdvertisingAction lists the session-resolved resources whose
// unified-inventory record currently advertises the requested lifecycle
// action (or its lifecycle synonym). It is the evidence behind the agentic
// loop's advertised-action gate.
func (e *PulseToolExecutor) SessionTargetsAdvertisingAction(action string) []AdvertisedActionTarget {
	if e == nil || e.resolvedContext == nil || e.unifiedResourceProvider == nil {
		return nil
	}
	lister, ok := e.resolvedContext.(resolvedResourceLister)
	if !ok {
		return nil
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var targets []AdvertisedActionTarget
	for _, resolved := range lister.ListResolvedResources() {
		if resolved == nil {
			continue
		}
		resource, ok := canonicalResourceForResolved(e.unifiedResourceProvider, resolved)
		if !ok {
			continue
		}
		capability, ok := advertisedActionName(resource, action)
		if !ok {
			continue
		}
		id := unifiedresources.CanonicalResourceID(resource.ID)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		targets = append(targets, AdvertisedActionTarget{
			CanonicalID: id,
			Name:        firstNonEmptyString(strings.TrimSpace(resourceDisplayName(resource)), id),
			Kind:        string(unifiedresources.ContractResourceType(resource)),
			Capability:  capability,
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Name == targets[j].Name {
			return targets[i].CanonicalID < targets[j].CanonicalID
		}
		return targets[i].Name < targets[j].Name
	})
	return targets
}
