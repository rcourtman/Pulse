package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// ActionCapabilityLookup answers which actions Pulse can run for a resource
// now, and why not when it can run none. It is the catalog an investigation
// reads from the action broker, so the scan and the investigation that follows
// it cannot disagree about what is within reach.
type ActionCapabilityLookup func(ctx context.Context, resourceID string) (aicontracts.ActionCapabilityCatalog, error)

// patrolActionCapabilityLookupTimeout bounds the lookup. It reads the
// registry and the executors' in-memory availability checks, and runs no
// agent preflight.
const patrolActionCapabilityLookupTimeout = 5 * time.Second

// patrolAlertSubjectFacts tells a run that looks at an alert what the scan has
// no other way to see about the resource the alert is on: which actions Pulse
// can run for it, whether Pulse can read logs on its host, whether a Proxmox
// guest is set to start on boot, and whether Pulse has already reported that
// the guest stopped.
//
// The alert note asks for a finding when Pulse has an action that would fix
// the condition, and for silence when the condition is expected. The seed
// carried neither fact. A scan of a dead guest wrote that it found no action,
// reported nothing about the stop, and filed the one other fact on the guest's
// row, that it had never been backed up.
//
// The actions are the ones that can run, not the ones the inventory declares.
// A stopped Proxmox guest declares Start on a node Pulse has no runner on, and
// a container declares Start on a host whose agent was installed without
// command access. Told the declared list, a scan on such a host wrote that
// Pulse "lists start and update as available actions". When nothing can run,
// the line carries the reason Pulse wrote for the refusal. When Pulse cannot
// check, the seed says nothing about actions.
func (p *PatrolService) patrolAlertSubjectFacts(scope *PatrolScope, snap patrolRuntimeState) string {
	if scope == nil || scope.AlertContext == nil || scope.Reason == TriggerReasonAlertCleared {
		return ""
	}
	var lines []string

	subject := patrolAlertSubjectResource(scope, snap)
	if subject != nil {
		name := strings.TrimSpace(subject.Name)
		if name == "" {
			name = strings.TrimSpace(scope.AlertContext.ResourceName)
		}
		lines = append(lines, p.patrolRunnableActionLines(subject.ID, name)...)
	}

	if onBoot, stopped, isGuest := patrolAlertGuestStartOnBoot(scope, snap); isGuest && onBoot != nil {
		switch {
		case *onBoot && stopped:
			lines = append(lines, "Start on boot is on for this guest, so it is meant to be running.")
		case *onBoot:
			lines = append(lines, "Start on boot is on for this guest.")
		default:
			lines = append(lines, "Start on boot is off for this guest.")
		}
	}

	if subject != nil && p != nil && p.findings != nil {
		if stopped := p.findings.Get(proxmoxGuestStoppedFindingIDPrefix + subject.ID); stopped != nil && stopped.IsActive() {
			lines = append(lines, "Pulse has already reported that this guest stopped. Do not report the stop again.")
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return "About the resource this alert is on: " + strings.Join(lines, " ") + "\n\n"
}

// patrolAlertSubjectResource returns the one inventory resource an alert run
// was asked about. A run asked about several has no single subject.
func patrolAlertSubjectResource(scope *PatrolScope, snap patrolRuntimeState) *unifiedresources.Resource {
	if scope == nil || snap.unifiedResourceProvider == nil {
		return nil
	}
	requested := make(map[string]bool, len(scope.ResourceIDs))
	for _, id := range scope.ResourceIDs {
		if id = strings.TrimSpace(id); id != "" {
			requested[id] = true
		}
	}
	var subject *unifiedresources.Resource
	for _, resource := range snap.unifiedResourceProvider.GetAll() {
		if !requested[strings.TrimSpace(resource.ID)] {
			continue
		}
		if subject != nil {
			return nil
		}
		found := resource
		subject = &found
	}
	return subject
}

// patrolRunnableActionLines states what Pulse can run for a resource, from the
// same catalog an investigation is given.
func (p *PatrolService) patrolRunnableActionLines(resourceID, name string) []string {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	lookup := p.actionCapabilityLookup
	p.mu.RUnlock()
	if lookup == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), patrolActionCapabilityLookupTimeout)
	defer cancel()
	catalog, err := lookup(ctx, resourceID)
	if err != nil {
		return nil
	}

	var lines []string
	runnable := make([]string, 0, len(catalog.Capabilities))
	for _, capability := range catalog.Capabilities {
		runnable = append(runnable, capability.Name)
	}
	runnable = patrolSortedDistinct(runnable)
	switch {
	case len(runnable) > 0:
		lines = append(lines, fmt.Sprintf("Pulse can run these actions for %q: %s.", name, strings.Join(runnable, ", ")))
	case catalog.GuestActionsUnavailable != nil && strings.TrimSpace(catalog.GuestActionsUnavailable.Reason) != "":
		lines = append(lines, fmt.Sprintf("Pulse cannot run an action for %q. %s That is how this install is set up, not a fault.",
			name, strings.TrimSpace(catalog.GuestActionsUnavailable.Reason)))
	case catalog.CommandsDisabled != nil:
		// The commands line below is the reason.
		lines = append(lines, fmt.Sprintf("Pulse cannot run an action for %q.", name))
	case len(catalog.Unavailable) > 0:
		reasons := make([]string, 0, len(catalog.Unavailable))
		for _, refusal := range catalog.Unavailable {
			reasons = append(reasons, strings.TrimSpace(refusal.Reason))
		}
		reasons = patrolSortedDistinct(reasons)
		if len(reasons) > 2 {
			reasons = reasons[:2]
		}
		line := fmt.Sprintf("Pulse cannot run an action for %q now.", name)
		if len(reasons) > 0 {
			line += " " + strings.Join(reasons, " ")
		}
		lines = append(lines, line)
	default:
		lines = append(lines, fmt.Sprintf("Pulse has no action for %q.", name))
	}
	if host := catalog.CommandsDisabled; host != nil {
		hostName := strings.TrimSpace(host.HostName)
		if hostName == "" {
			hostName = "its host"
		}
		reason := strings.TrimSpace(host.Reason)
		if reason == "" {
			reason = fmt.Sprintf("Commands are not enabled on the Pulse agent on %s.", hostName)
		}
		lines = append(lines, fmt.Sprintf("%s Pulse cannot read logs or files on %s or run an action that needs its agent. That is how the agent was installed, not a fault, so do not say an action is available and do not tell the user to reconnect or reinstall an agent.",
			reason, hostName))
	}
	return lines
}

func patrolSortedDistinct(values []string) []string {
	seen := make(map[string]bool, len(values))
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// patrolAlertGuestStartOnBoot reads the start-on-boot setting of the Proxmox
// guest an alert names. The setting is nil when Proxmox did not report it.
func patrolAlertGuestStartOnBoot(scope *PatrolScope, snap patrolRuntimeState) (onBoot *bool, stopped, isGuest bool) {
	reference := strings.TrimSpace(scope.AlertContext.ResourceID)
	if reference == "" {
		return nil, false, false
	}
	for _, vm := range snap.VMs {
		if vm.ID == reference {
			return vm.OnBoot, !strings.EqualFold(strings.TrimSpace(vm.Status), "running"), true
		}
	}
	for _, ct := range snap.Containers {
		if ct.ID == reference {
			return ct.OnBoot, !strings.EqualFold(strings.TrimSpace(ct.Status), "running"), true
		}
	}
	return nil, false, false
}
