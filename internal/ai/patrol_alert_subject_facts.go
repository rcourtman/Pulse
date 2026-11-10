package ai

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// patrolAlertSubjectFacts tells a run that looks at an alert what the scan has
// no other way to see about the resource the alert is on: which actions Pulse
// declares for it, whether a Proxmox guest is set to start on boot, and
// whether Pulse has already reported that the guest stopped.
//
// The alert note asks for a finding when Pulse has an action that would fix
// the condition, and for silence when the condition is expected. The seed
// carried neither fact. A scan of a dead guest wrote that it found no action,
// reported nothing about the stop, and filed the one other fact on the guest's
// row, that it had never been backed up.
//
// The actions are the ones the inventory declares. Whether one can run now is
// decided when it is planned: a Proxmox guest declares Start on a node Pulse
// has no runner on. So the line says "declares", and tells the scan not to
// promise. An investigation gets the checked list and the refusal.
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
		if actions := patrolDeclaredActionNames(subject.Capabilities); len(actions) > 0 {
			lines = append(lines, fmt.Sprintf("Pulse declares these actions for %q: %s. Whether one can run here is checked when it is proposed, so do not promise that it will work.",
				name, strings.Join(actions, ", ")))
		} else {
			lines = append(lines, fmt.Sprintf("Pulse declares no action for %q.", name))
		}
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

func patrolDeclaredActionNames(capabilities []unifiedresources.ResourceCapability) []string {
	seen := make(map[string]bool, len(capabilities))
	var names []string
	for _, capability := range capabilities {
		name := strings.TrimSpace(capability.Name)
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
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
