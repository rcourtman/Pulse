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
		if patrolAlertIsAboutStorageSpace(alertType) && patrolStorageHoldsBackups(subject) {
			lines = append(lines, fmt.Sprintf("Pulse lists the backups kept on %q: pulse_storage with type \"backups\" shows each guest's backups there, with their sizes and dates. "+
				"Look there for what fills it. When you cannot tell from here, report a finding: its investigation reads the list, and that is not a finding that only repeats the alert.", name))
		}
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
// was asked about. A run asked about several has no single subject. An alert
// on part of a resource, such as an agent's filesystem
// ("agent:<host>/disk:<mount>"), is about the resource that owns the part:
// the D6 disk alert named no inventory resource, so its seed said nothing
// about what Pulse could read on the host.
func patrolAlertSubjectResource(scope *PatrolScope, snap patrolRuntimeState) *unifiedresources.Resource {
	if scope == nil || snap.unifiedResourceProvider == nil {
		return nil
	}
	requested := make(map[string]bool, len(scope.ResourceIDs))
	var resolver unifiedresources.AlertResourceReferenceResolver
	for _, id := range scope.ResourceIDs {
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		requested[id] = true
		if !strings.Contains(id, "/") {
			continue
		}
		if resolver == nil {
			resolver = patrolAlertReferenceResolver(snap)
		}
		if binding, _ := resolver.ResolveAlertResourceReference(id); binding.Component && strings.TrimSpace(binding.ResourceID) != "" {
			requested[strings.TrimSpace(binding.ResourceID)] = true
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

// patrolAlertIsAboutStorageSpace reports whether an alert is about how full a
// storage is.
func patrolAlertIsAboutStorageSpace(alertType string) bool {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case "usage", "storage":
		return true
	}
	return false
}

// patrolStorageHoldsBackups reports whether a resource is a storage that
// holds backups, which pulse_storage lists with their guests and sizes.
//
// On pve-v1 P3, a node's local storage filled with old backups of one guest.
// With commands off the sustained check on its usage alert can read nothing
// on the node, and was told nothing about the backup list. Runs that listed
// the backups named the guest and filed; runs that read only the pool's size
// recorded nothing, because the rule forbids a finding that only repeats the
// reading.
func patrolStorageHoldsBackups(resource *unifiedresources.Resource) bool {
	if resource == nil || resource.Type != unifiedresources.ResourceTypeStorage || resource.Storage == nil {
		return false
	}
	for _, content := range resource.Storage.ContentTypes {
		if strings.EqualFold(strings.TrimSpace(content), "backup") {
			return true
		}
	}
	return false
}

// patrolAlertStorageBackupsListed returns the name of the storage a run on a
// space alert looks at, when that storage holds backups Pulse lists.
func patrolAlertStorageBackupsListed(scope *PatrolScope, snap patrolRuntimeState) (string, bool) {
	if scope == nil || scope.AlertContext == nil || !patrolAlertIsAboutStorageSpace(scope.AlertContext.AlertType) {
		return "", false
	}
	subject := patrolAlertSubjectResource(scope, snap)
	if !patrolStorageHoldsBackups(subject) {
		return "", false
	}
	name := strings.TrimSpace(subject.Name)
	if name == "" {
		name = strings.TrimSpace(scope.AlertContext.ResourceName)
	}
	return name, true
}

// patrolRunnableActionLines states what Pulse can run for a resource, and
// where it can read, from the same catalog an
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
	if place, through, ok := patrolReadPlace(subject, name, catalog); ok {
		// This check has no agent reads (PatrolDetectionToolNamesForResourceTypes);
		// the investigation a finding starts has them.
		line := fmt.Sprintf("Pulse can read disk usage, files and logs %s %s. This check cannot read them; an investigation of a finding can.", place, through)
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

// patrolReadPlace says where Pulse can read for an alert's subject, from the
// catalog's read fact: inside a running Proxmox guest through its node's
// agent, or on an agent host through its own agent. Reads on a host run on
// that host, so a host is readable only when the read fact names the subject
// itself; a Docker container's reads would run on its host, not in it.
func patrolReadPlace(subject *unifiedresources.Resource, name string, catalog aicontracts.ActionCapabilityCatalog) (place, through string, ok bool) {
	reads := catalog.Reads
	if subject == nil || reads == nil || catalog.CommandsDisabled != nil {
		return "", "", false
	}
	hostName := strings.TrimSpace(reads.HostName)
	if patrolResourceIsRunningProxmoxGuest(subject) {
		if hostName == "" {
			hostName = "its node"
		}
		through = "through the Pulse agent on " + hostName
		if reads.Guest == "vm" {
			through += " when the QEMU guest agent runs in it"
		}
		return fmt.Sprintf("inside %q", name), through, true
	}
	if subject.Status == unifiedresources.StatusOffline || strings.TrimSpace(reads.Guest) != "" ||
		strings.TrimSpace(reads.HostResourceID) != strings.TrimSpace(subject.ID) {
		return "", "", false
	}
	return fmt.Sprintf("on %q", name), "through its Pulse agent", true
}

// patrolAlertSubjectReadPlace says where Pulse can read for the resource an
// alert run looks at (inside "gitea", on "docker01"), or reports false.
func (p *PatrolService) patrolAlertSubjectReadPlace(scope *PatrolScope, snap patrolRuntimeState) (string, bool) {
	subject := patrolAlertSubjectResource(scope, snap)
	if subject == nil {
		return "", false
	}
	catalog, ok := p.patrolActionCatalog(subject.ID)
	if !ok {
		return "", false
	}
	place, _, readable := patrolReadPlace(subject, strings.TrimSpace(subject.Name), catalog)
	return place, readable
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
