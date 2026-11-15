package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
//
// It also says when Pulse can read inside a Proxmox guest through its node's
// agent. A guest disk alert on a node with no action runner was told only
// "Pulse cannot run an action ... Use Proxmox for this", and a run wrote that
// Pulse had no command or log access there and raised nothing, though a read
// inside the guest names the folder that filled it.
//
// A guest's power is left out of a space or age alert: no start, stop or
// restart changes what fills a disk or how old a backup is, and a refusal of
// them read as Pulse being able to do nothing for the guest at all.
//
// askedAlert is the alert active on the resource a person's targeted check
// was asked about (patrolAskedResourceAlert). That run is given the same
// facts.
func (p *PatrolService) patrolAlertSubjectFacts(scope *PatrolScope, snap patrolRuntimeState, askedAlert *models.Alert) string {
	if scope == nil || scope.Reason == TriggerReasonAlertCleared {
		return ""
	}
	var alertResourceID, alertResourceName, alertType string
	switch {
	case scope.AlertContext != nil:
		alertResourceID, alertResourceName, alertType = scope.AlertContext.ResourceID, scope.AlertContext.ResourceName, scope.AlertContext.AlertType
	case askedAlert != nil:
		alertResourceID, alertResourceName, alertType = askedAlert.ResourceID, askedAlert.ResourceName, askedAlert.Type
	default:
		return ""
	}
	powerMatters := !patrolAlertIsAboutSpaceOrAge(alertType)
	var lines []string

	subject := patrolAlertSubjectResource(scope, snap)
	// Somebody stopped this guest through Proxmox. That is the whole answer
	// to its alert. Saying which actions Pulse could run, or that the guest
	// starts on boot, would only argue for starting it again.
	if subject != nil {
		if record, deliberate := p.proxmoxGuestStoppedOnPurpose(subject.ID); deliberate {
			return "About the resource this alert is on: This guest was " + record.requestedSentence() +
				". That was deliberate. Report nothing about it being stopped and do not propose starting it.\n\n"
		}
	}
	if subject != nil {
		name := strings.TrimSpace(subject.Name)
		if name == "" {
			name = strings.TrimSpace(alertResourceName)
		}
		lines = append(lines, p.patrolRunnableActionLines(subject, name, alertType)...)
	}

	if onBoot, stopped, isGuest := patrolAlertGuestStartOnBoot(alertResourceID, snap); powerMatters && isGuest && onBoot != nil {
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

// patrolAlertIsAboutSpaceOrAge reports whether an alert is about how full
// something is or how old a backup or snapshot is. Starting, stopping or
// restarting a guest changes neither.
func patrolAlertIsAboutSpaceOrAge(alertType string) bool {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case "disk", "usage", "storage", "backup-age", "snapshot-age":
		return true
	}
	return false
}

// patrolRunnableActionLines states what Pulse can run for a resource, and
// what it can read inside a Proxmox guest, from the same catalog an
// investigation is given. For a space or age alert the guest power actions
// its node refuses are left out.
func (p *PatrolService) patrolRunnableActionLines(subject *unifiedresources.Resource, name, alertType string) []string {
	if p == nil || subject == nil {
		return nil
	}
	powerMatters := !patrolAlertIsAboutSpaceOrAge(alertType)
	catalog, ok := p.patrolActionCatalog(subject.ID)
	if !ok {
		return nil
	}

	refusedForPower := false
	if !powerMatters && catalog.GuestActionsUnavailable != nil {
		power := make(map[string]bool, len(catalog.GuestActionsUnavailable.Actions))
		for _, action := range catalog.GuestActionsUnavailable.Actions {
			power[strings.TrimSpace(action)] = true
		}
		kept := catalog.Unavailable[:0:0]
		for _, refusal := range catalog.Unavailable {
			if !power[strings.TrimSpace(refusal.Name)] {
				kept = append(kept, refusal)
			}
		}
		catalog.Unavailable = kept
		catalog.GuestActionsUnavailable = nil
		refusedForPower = true
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
	case refusedForPower:
		// The node refuses only power actions, which do not bear on this alert.
	default:
		lines = append(lines, fmt.Sprintf("Pulse has no action for %q.", name))
	}
	if reads := catalog.Reads; reads != nil && catalog.CommandsDisabled == nil && patrolResourceIsRunningProxmoxGuest(subject) {
		hostName := strings.TrimSpace(reads.HostName)
		if hostName == "" {
			hostName = "its node"
		}
		through := "through the Pulse agent on " + hostName
		if reads.Guest == "vm" {
			through += " when the QEMU guest agent runs in it"
		}
		// This check has no agent reads (PatrolDetectionToolNamesForResourceTypes);
		// the investigation a finding starts has them.
		line := fmt.Sprintf("Pulse can read disk usage, files and logs inside %q %s. This check cannot read them; an investigation of a finding can.", name, through)
		// Told only that a read was possible, a sustained check on a guest
		// disk alert still kept to "do not report a finding that only repeats
		// the alert's reading" and raised nothing. Here the finding is what
		// gets the folder named.
		if strings.EqualFold(strings.TrimSpace(alertType), "disk") {
			line += " So when you cannot tell from here what fills the disk, report a finding: its investigation reads what does, and that is not a finding that only repeats the alert."
		}
		lines = append(lines, line)
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

// patrolActionCatalog asks the action broker's catalog for a resource. It
// reports false when Pulse has no lookup or the lookup failed.
func (p *PatrolService) patrolActionCatalog(resourceID string) (aicontracts.ActionCapabilityCatalog, bool) {
	if p == nil {
		return aicontracts.ActionCapabilityCatalog{}, false
	}
	p.mu.RLock()
	lookup := p.actionCapabilityLookup
	p.mu.RUnlock()
	if lookup == nil {
		return aicontracts.ActionCapabilityCatalog{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), patrolActionCapabilityLookupTimeout)
	defer cancel()
	catalog, err := lookup(ctx, resourceID)
	if err != nil {
		return aicontracts.ActionCapabilityCatalog{}, false
	}
	return catalog, true
}

// patrolAlertSubjectGuestReadable reports whether the resource an alert run
// looks at is a running Proxmox guest Pulse can read inside.
func (p *PatrolService) patrolAlertSubjectGuestReadable(scope *PatrolScope, snap patrolRuntimeState) bool {
	subject := patrolAlertSubjectResource(scope, snap)
	if !patrolResourceIsRunningProxmoxGuest(subject) {
		return false
	}
	catalog, ok := p.patrolActionCatalog(subject.ID)
	return ok && catalog.Reads != nil && catalog.CommandsDisabled == nil
}

// patrolResourceIsRunningProxmoxGuest reports whether a resource is a Proxmox
// guest that is up. A read inside a guest runs in the guest, so a stopped one
// cannot be read.
func patrolResourceIsRunningProxmoxGuest(resource *unifiedresources.Resource) bool {
	if resource == nil || resource.Status == unifiedresources.StatusOffline {
		return false
	}
	return resource.Type == unifiedresources.ResourceTypeSystemContainer || resource.Type == unifiedresources.ResourceTypeVM
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
func patrolAlertGuestStartOnBoot(alertResourceID string, snap patrolRuntimeState) (onBoot *bool, stopped, isGuest bool) {
	reference := strings.TrimSpace(alertResourceID)
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
