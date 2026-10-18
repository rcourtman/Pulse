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
// unified-inventory record only when its identity is unambiguous. Provider
// IDs and node/host names can recur across independent installations.
func canonicalResourceForResolved(provider UnifiedResourceProvider, resolved ResolvedResourceInfo) (unifiedresources.Resource, bool) {
	candidates := canonicalResourcesForResolved(provider, resolved)
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return unifiedresources.Resource{}, false
}

// canonicalResourcesForResolved prefers canonical IDs over placement/name
// hints, but collects every match at each tier. Conflicting canonical aliases
// and ambiguous legacy placement must not be resolved by listing order.
func canonicalResourcesForResolved(provider UnifiedResourceProvider, resolved ResolvedResourceInfo) []unifiedresources.Resource {
	if provider == nil || resolved == nil {
		return nil
	}
	resourceType, ok := controlResourceTypeForKind(firstNonEmptyString(resolved.GetKind(), resolved.GetResourceType()))
	if !ok {
		return nil
	}
	candidates := provider.GetByType(resourceType)
	var matches []unifiedresources.Resource
	seen := make(map[string]bool)
	addMatch := func(resource unifiedresources.Resource) {
		id := strings.TrimSpace(resource.ID)
		if id != "" && !seen[id] {
			seen[id] = true
			matches = append(matches, resource)
		}
	}
	for _, alias := range append([]string{resolved.GetResourceID()}, resolved.GetAliases()...) {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		for _, resource := range candidates {
			if strings.EqualFold(strings.TrimSpace(resource.ID), alias) {
				addMatch(resource)
			}
		}
	}
	if len(matches) > 0 {
		return matches
	}
	switch resourceType {
	case unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeSystemContainer:
		vmid := resolved.GetVMID()
		node := strings.TrimSpace(resolved.GetNode())
		if vmid <= 0 {
			return nil
		}
		for _, resource := range candidates {
			if resource.Proxmox == nil || resource.Proxmox.VMID != vmid {
				continue
			}
			if node == "" || strings.EqualFold(strings.TrimSpace(resource.Proxmox.NodeName), node) {
				addMatch(resource)
			}
		}
	case unifiedresources.ResourceTypeAppContainer:
		providerUID := strings.TrimSpace(resolved.GetProviderUID())
		host := strings.TrimSpace(resolved.GetTargetHost())
		if providerUID == "" {
			return nil
		}
		for _, resource := range candidates {
			if !strings.EqualFold(strings.TrimSpace(appContainerProviderID(resource)), providerUID) {
				continue
			}
			if host == "" || strings.EqualFold(strings.TrimSpace(canonicalAppContainerHost(resource)), host) {
				addMatch(resource)
			}
		}
	case unifiedresources.ResourceTypeAgent:
		for _, resource := range candidates {
			name := strings.TrimSpace(resourceDisplayName(resource))
			for _, alias := range resolved.GetAliases() {
				if name != "" && strings.EqualFold(name, strings.TrimSpace(alias)) {
					addMatch(resource)
				}
			}
		}
	}
	return matches
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
// resource. An explicit canonical ID wins over stale session aliases. Other
// session references must have unambiguous current identity. A reference absent
// from the session but unique in inventory is still accepted without a second
// discovery step. Capability, approval and execution stay with the shared
// action lifecycle.
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
		inventoryCandidates := canonicalControlCandidates(e.unifiedResourceProvider, ref)
		if len(inventoryCandidates) == 1 && strings.EqualFold(strings.TrimSpace(inventoryCandidates[0].ID), strings.TrimSpace(ref)) {
			resource := inventoryCandidates[0]
			target.canonical = &resource
		}
		if target.canonical == nil && target.session != nil {
			candidates := canonicalResourcesForResolved(e.unifiedResourceProvider, target.session)
			switch len(candidates) {
			case 0:
			case 1:
				resource := candidates[0]
				target.canonical = &resource
			default:
				result := ambiguousControlTargetResult(ref, action, candidates)
				return target, &result
			}
		}
		if target.canonical == nil {
			candidates := inventoryCandidates
			switch len(candidates) {
			case 0:
			case 1:
				resource := candidates[0]
				target.canonical = &resource
			default:
				result := ambiguousControlTargetResult(ref, action, candidates)
				return target, &result
			}
		}
		if target.canonical != nil && target.session == nil && e.resolvedContext != nil {
			resource := *target.canonical
			if reg, ok := CanonicalHandoffResourceRegistration(e.unifiedResourceProvider, resource.ID, "", string(unifiedresources.ContractResourceType(resource)), ""); ok {
				e.registerResolvedResourceWithExplicitAccess(reg)
				if res, ok := e.resolvedContext.GetResolvedResourceByID(resource.ID); ok && res != nil {
					target.session = res
				} else if res, ok := e.resolvedContext.GetResolvedResourceByAlias(reg.Name); ok && res != nil {
					target.session = res
				}
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

func ambiguousControlTargetResult(ref, action string, candidates []unifiedresources.Resource) CallToolResult {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, fmt.Sprintf("%s (%s)", candidate.ID, unifiedresources.ContractResourceType(candidate)))
	}
	sort.Strings(ids)
	return NewToolResponseResult(NewToolBlockedError(
		agentcapabilities.ErrCodeInvalidInput,
		fmt.Sprintf("%d canonical resources match %q; call pulse_control again with one of these canonical resource ids: %s.", len(candidates), ref, strings.Join(ids, ", ")),
		map[string]interface{}{
			"resource_id":     ref,
			"action":          action,
			"candidates":      ids,
			"policy_boundary": "Ambiguous target reference. Retry with a canonical resource id from this list; this is a lookup detail, not a missing prerequisite.",
		},
	))
}

// controlLockedResult is the tool evidence for a target whose operator state
// blocks all remediation (Never auto-remediate, or a retired lifecycle). The
// block is the operator's explicit "Pulse must not act on this resource"
// decision, so the model is told to report it and not to look for another way
// to act.
func controlLockedResult(target controlTarget, action string) CallToolResult {
	name := target.displayName()
	return NewToolResponseResult(NewToolBlockedError(
		agentcapabilities.ErrCodeActionNotAllowed,
		fmt.Sprintf("%q is not permitted on %s: an operator blocked all remediation for this resource (Never auto-remediate is on, or its lifecycle is Retired), so Pulse will not run any action on it.", action, name),
		map[string]interface{}{
			"resource_id":      target.canonicalID(),
			"requested_action": action,
			"reason_code":      agentcapabilities.AgentErrCodeResourceRemediationLocked,
			"policy_boundary":  "An operator blocked all remediation for this resource, even with approval. Report exactly this boundary. It is cleared from the resource's Operator overrides in Pulse: turn off Never auto-remediate and, if the lifecycle is Retired, set it back to Active, then save. Do not plan the action another way or suggest workarounds.",
		},
	))
}

// controlPlanFailureResult turns a planning error into tool evidence the model
// can report faithfully. An operator remediation block and a capability the
// resource does not advertise are real boundaries and are described as such
// (the latter with the resource's current capability list); anything else is
// passed through unchanged.
func controlPlanFailureResult(target controlTarget, action string, err error) CallToolResult {
	if err == nil {
		return NewErrorResult(fmt.Errorf("canonical action planning failed"))
	}
	if errors.Is(err, unifiedresources.ErrResourceRemediationLocked) {
		return controlLockedResult(target, action)
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
