package servicediscovery

import (
	"strings"
	"testing"
)

func TestCommandsAndTemplates(t *testing.T) {
	resourceTypes := []ResourceType{
		ResourceTypeLXC,
		ResourceTypeVM,
		ResourceTypeDocker,
		ResourceTypeDockerVM,
		ResourceTypeDockerLXC,
		ResourceTypeK8s,
		ResourceTypeHost,
	}

	for _, rt := range resourceTypes {
		cmds := GetCommandsForResource(rt)
		if len(cmds) == 0 {
			t.Fatalf("expected commands for %s", rt)
		}
	}

	if len(GetCommandsForResource(ResourceType("unknown"))) != 0 {
		t.Fatalf("expected no commands for unknown resource type")
	}

	if !strings.Contains(BuildLXCCommand("101", "echo hi"), "pct exec 101") {
		t.Fatalf("unexpected LXC command")
	}
	if !strings.Contains(BuildVMCommand("101", "echo hi"), "qm guest exec 101") {
		t.Fatalf("unexpected VM command")
	}
	// Docker commands now quote container names for safety
	dockerCmd := BuildDockerCommand("web", "echo hi")
	if !strings.Contains(dockerCmd, "docker exec") || !strings.Contains(dockerCmd, "web") {
		t.Fatalf("unexpected docker command: %s", dockerCmd)
	}

	nestedLXC := BuildNestedDockerCommand("201", true, "web", "echo hi")
	if !strings.Contains(nestedLXC, "pct exec 201") || !strings.Contains(nestedLXC, "docker exec") || !strings.Contains(nestedLXC, "web") {
		t.Fatalf("unexpected nested LXC command: %s", nestedLXC)
	}

	nestedVM := BuildNestedDockerCommand("301", false, "web", "echo hi")
	if !strings.Contains(nestedVM, "qm guest exec 301") || !strings.Contains(nestedVM, "docker exec") || !strings.Contains(nestedVM, "web") {
		t.Fatalf("unexpected nested VM command: %s", nestedVM)
	}

	// K8s commands now quote arguments for safety
	withContainer := BuildK8sCommand("default", "pod", "app", "echo hi")
	if !strings.Contains(withContainer, "-c") || !strings.Contains(withContainer, "app") || !strings.Contains(withContainer, "kubectl exec") {
		t.Fatalf("unexpected k8s command: %s", withContainer)
	}

	withoutContainer := BuildK8sCommand("default", "pod", "", "echo hi")
	if strings.Contains(withoutContainer, "-c") && strings.Contains(withoutContainer, "app") {
		t.Fatalf("unexpected container selector: %s", withoutContainer)
	}

	template := GetCLIAccessTemplate(ResourceTypeK8s)
	if !strings.Contains(template, "{namespace}") || !strings.Contains(template, "{pod}") {
		t.Fatalf("unexpected template: %s", template)
	}

	for _, rt := range resourceTypes {
		if tmpl := GetCLIAccessTemplate(rt); tmpl == "" {
			t.Fatalf("expected template for %s", rt)
		}
	}
	if tmpl := GetCLIAccessTemplate(ResourceType("unknown")); !strings.Contains(tmpl, "pulse_control") {
		t.Fatalf("expected default template to mention pulse_control, got: %s", tmpl)
	}

	formatted := FormatCLIAccess(ResourceTypeK8s, "101", "container", "default", "pod")
	if !strings.Contains(formatted, "default") || !strings.Contains(formatted, "pod") {
		t.Fatalf("unexpected formatted access: %s", formatted)
	}
}

func TestDockerMountsCommandAvoidsExtraTextUtilities(t *testing.T) {
	lxcCmds := GetCommandsForResource(ResourceTypeLXC)
	vmCmds := GetCommandsForResource(ResourceTypeVM)

	findByName := func(cmds []DiscoveryCommand, name string) string {
		for _, cmd := range cmds {
			if cmd.Name == name {
				return cmd.Command
			}
		}
		return ""
	}

	lxcMounts := findByName(lxcCmds, "docker_mounts")
	vmMounts := findByName(vmCmds, "docker_mounts")

	if lxcMounts == "" || vmMounts == "" {
		t.Fatalf("expected docker_mounts command for both LXC and VM")
	}
	if lxcMounts != vmMounts {
		t.Fatalf("expected shared docker_mounts command, got lxc=%q vm=%q", lxcMounts, vmMounts)
	}
	if strings.Contains(lxcMounts, "sed ") || strings.Contains(lxcMounts, "grep ") {
		t.Fatalf("docker_mounts should not depend on sed/grep: %s", lxcMounts)
	}
	if !strings.Contains(lxcMounts, "name=${name#/}") {
		t.Fatalf("expected shell-native container name trimming, got: %s", lxcMounts)
	}
}

func TestValidateResourceID_RejectsOptionLikeIDs(t *testing.T) {
	cases := []string{
		"-bad",
		"--help",
		"-1",
	}

	for _, tc := range cases {
		if err := ValidateResourceID(tc); err == nil {
			t.Fatalf("expected error for option-like resource ID %q", tc)
		}
	}
}

func TestBuildCommands_RejectOptionLikeIdentifiers(t *testing.T) {
	if cmd := BuildDockerCommand("-bad", "echo hi"); !strings.Contains(cmd, "invalid container name") {
		t.Fatalf("expected invalid container name error command, got %q", cmd)
	}
	if cmd := BuildK8sCommand("-ns", "pod", "", "echo hi"); !strings.Contains(cmd, "invalid namespace") {
		t.Fatalf("expected invalid namespace error command, got %q", cmd)
	}
	if cmd := BuildVMCommand("-1", "echo hi"); !strings.Contains(cmd, "invalid VM ID") {
		t.Fatalf("expected invalid VM ID error command, got %q", cmd)
	}
}
