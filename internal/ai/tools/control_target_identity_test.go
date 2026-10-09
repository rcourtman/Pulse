package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// VMIDs and node/host names are scoped to an installation, not globally
// unique. An older session without a canonical alias cannot choose whichever
// matching installation happens to be listed first.
func TestCanonicalResourceForResolvedRefusesCrossInstallationAmbiguity(t *testing.T) {
	for _, kind := range []string{"vm", "system-container", "app-container", "agent"} {
		t.Run(kind, func(t *testing.T) {
			home := controlIdentityResource(kind, "home")
			remote := controlIdentityResource(kind, "remote")
			resolved := controlIdentitySession(kind)
			for _, resources := range [][]unifiedresources.Resource{{home, remote}, {remote, home}} {
				provider := &stubUnifiedResourceProvider{resources: resources}
				if resource, ok := canonicalResourceForResolved(provider, resolved); ok {
					t.Fatalf("ambiguous %s identity selected %s", kind, resource.ID)
				}
			}
		})
	}
}

func TestCanonicalResourceForResolvedKeepsUniqueIdentityAndCanonicalAliases(t *testing.T) {
	for _, kind := range []string{"vm", "system-container", "app-container", "agent"} {
		t.Run(kind, func(t *testing.T) {
			home := controlIdentityResource(kind, "home")
			remote := controlIdentityResource(kind, "remote")
			resolved := controlIdentitySession(kind)
			assertID := func(provider *stubUnifiedResourceProvider, want string) {
				t.Helper()
				resource, ok := canonicalResourceForResolved(provider, resolved)
				if !ok || resource.ID != want {
					t.Fatalf("resolved %s identity = %q (ok=%t), want %q", kind, resource.ID, ok, want)
				}
			}
			assertID(&stubUnifiedResourceProvider{resources: []unifiedresources.Resource{home}}, home.ID)
			resolved.aliases = append(resolved.aliases, remote.ID)
			assertID(&stubUnifiedResourceProvider{resources: []unifiedresources.Resource{home, remote}}, remote.ID)
			resolved.aliases = []string{"shared-name", remote.ID, remote.ID}
			assertID(&stubUnifiedResourceProvider{resources: []unifiedresources.Resource{remote, home}}, remote.ID)
		})
	}
}

func TestCanonicalResourceForResolvedKeepsPlacementDisambiguation(t *testing.T) {
	for _, kind := range []string{"vm", "system-container", "app-container"} {
		t.Run(kind, func(t *testing.T) {
			home := controlIdentityResource(kind, "home")
			remote := controlIdentityResource(kind, "remote")
			if remote.Proxmox != nil {
				remote.Proxmox.NodeName = "pve2"
			} else {
				remote.ParentName = "other-host"
			}
			provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{remote, home}}
			resource, ok := canonicalResourceForResolved(provider, controlIdentitySession(kind))
			if !ok || resource.ID != home.ID {
				t.Fatalf("unique placement resolved %q (ok=%t), want %q", resource.ID, ok, home.ID)
			}
		})
	}
}

func TestCanonicalResourceForResolvedRefusesConflictingCanonicalAliases(t *testing.T) {
	home := controlIdentityResource("vm", "home")
	remote := controlIdentityResource("vm", "remote")
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{home, remote}}
	resolved := controlIdentitySession("vm")
	for _, aliases := range [][]string{{home.ID, remote.ID}, {remote.ID, home.ID}} {
		resolved.aliases = aliases
		if resource, ok := canonicalResourceForResolved(provider, resolved); ok {
			t.Fatalf("conflicting canonical aliases selected %s", resource.ID)
		}
	}
}

func TestExecuteControlResourceRefusesAmbiguousSessionPlacementBeforePlanning(t *testing.T) {
	for _, kind := range []string{"vm", "system-container", "app-container", "agent"} {
		for _, ref := range []string{"shared-name", "old-session-target"} {
			t.Run(kind+"/"+ref, func(t *testing.T) {
				home := controlIdentityResource(kind, "home")
				remote := controlIdentityResource(kind, "remote")
				provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{home, remote}}
				resolved := newControlTestResolvedContext()
				resolved.aliases[ref] = controlIdentitySession(kind)
				plans := &recordedPlan{}
				executor := NewPulseToolExecutor(ExecutorConfig{
					UnifiedResourceProvider: provider,
					ControlLevel:            ControlLevelControlled,
					TypedActionPlanner:      plans.planner(nil),
				})
				executor.SetResolvedContext(resolved)
				result, err := executor.executeControl(context.Background(), map[string]interface{}{
					"type": "resource", "resource_id": ref, "action": "reboot",
				})
				if err != nil || !result.IsError || len(plans.requests) != 0 {
					t.Fatalf("ambiguous session reached planning: err=%v isError=%t requests=%+v", err, result.IsError, plans.requests)
				}
				text := result.Content[0].Text
				if !strings.Contains(text, home.ID) || !strings.Contains(text, remote.ID) {
					t.Fatalf("ambiguity did not return both canonical alternatives: %s", text)
				}
			})
		}
	}
}

func TestExecuteControlResourceExplicitCanonicalIDBeatsStaleSessionAlias(t *testing.T) {
	home := controlIdentityResource("vm", "home")
	remote := controlIdentityResource("vm", "remote")
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{home, remote}}
	resolved := newControlTestResolvedContext()
	stale := controlIdentitySession("vm")
	stale.aliases = []string{home.ID}
	resolved.aliases[remote.ID] = stale
	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(nil),
	})
	executor.SetResolvedContext(resolved)
	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type": "resource", "resource_id": remote.ID, "action": "reboot",
	})
	if err != nil || result.IsError || len(plans.requests) != 1 || plans.requests[0].ResourceID != remote.ID {
		t.Fatalf("explicit canonical identity was replaced by session history: err=%v isError=%t requests=%+v", err, result.IsError, plans.requests)
	}
	payload := decodeControlPayload(t, result)
	if payload["execution_requested"] != false || payload["requires_approval"] != true {
		t.Fatalf("canonical selection changed approval/execution boundaries: %+v", payload)
	}
}

func TestExecuteControlResourceCanonicalLookupKeepsSessionRegistration(t *testing.T) {
	vm := controlTestProxmoxVM("win-unique", 101, "pve", true)
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{vm}}
	resolved := newControlTestResolvedContext()
	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(nil),
	})
	executor.SetResolvedContext(resolved)
	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type": "resource", "resource_id": vm.ID, "action": "reboot",
	})
	if err != nil || result.IsError || len(plans.requests) != 1 || plans.requests[0].ResourceID != vm.ID {
		t.Fatalf("unique canonical identity failed to plan: err=%v isError=%t requests=%+v", err, result.IsError, plans.requests)
	}
	if targets := executor.SessionTargetsAdvertisingAction("reboot"); len(targets) != 1 || targets[0].CanonicalID != vm.ID {
		t.Fatalf("canonical lookup lost session continuity: %+v", targets)
	}
}

func TestSessionTargetsAdvertisingActionOmitsAmbiguousPlacement(t *testing.T) {
	home := controlIdentityResource("vm", "home")
	remote := controlIdentityResource("vm", "remote")
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{home, remote}}
	resolved := newControlTestResolvedContext()
	resolved.resources["old-session-target"] = controlIdentitySession("vm")
	executor := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: provider})
	executor.SetResolvedContext(resolved)
	if targets := executor.SessionTargetsAdvertisingAction("reboot"); len(targets) != 0 {
		t.Fatalf("ambiguous session advertised a guessed action target: %+v", targets)
	}
}

func controlIdentitySession(kind string) *mockResource {
	return &mockResource{
		resourceID: "old-session-target", resourceType: kind, kind: kind,
		vmID: 100, node: "pve1", targetHost: "same-host", providerUID: "same-container-id",
		aliases: []string{"shared-name"},
	}
}

func controlIdentityResource(kind, instance string) unifiedresources.Resource {
	resourceType, _ := controlResourceTypeForKind(kind)
	resource := unifiedresources.Resource{
		ID: instance + "-" + kind, Type: resourceType, Name: "shared-name",
		Status:       unifiedresources.StatusOnline,
		Capabilities: []unifiedresources.ResourceCapability{{Name: "reboot"}},
	}
	switch kind {
	case "vm", "system-container":
		resource.Proxmox = &unifiedresources.ProxmoxData{Instance: instance, NodeName: "pve1", VMID: 100}
	case "app-container":
		resource.ParentName = "same-host"
		resource.Docker = &unifiedresources.DockerData{ContainerID: "same-container-id", HostSourceID: instance + "-machine"}
	}
	return resource
}
