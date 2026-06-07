package servicediscovery

import (
	"strings"
	"testing"
)

func TestCommandsAndTemplates(t *testing.T) {
	resourceTypes := []ResourceType{
		ResourceTypeSystemContainer,
		ResourceTypeVM,
		ResourceTypeDocker,
		ResourceTypeDockerVM,
		ResourceTypeDockerSystemContainer,
		ResourceTypeK8s,
		ResourceTypeAgent,
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

func TestGuestCommandSetsAreSurfaceOnly(t *testing.T) {
	// Discovery is a fast surface index (identity + how-to-reach), not a deep
	// scan. Guest command sets must stay light and must NOT include the deep
	// enumeration commands — the Assistant knows standard service layouts and
	// fetches specifics on demand. See discovery-assistant-goal.
	deep := map[string]bool{
		"installed_packages": true,
		"config_files":       true,
		"docker_mounts":      true,
		"hardware_info":      true,
		"gpu_devices":        true,
		"disk_usage":         true,
		"cron_jobs":          true,
		"docker_check":       true,
	}
	for _, rt := range []ResourceType{ResourceTypeSystemContainer, ResourceTypeVM, ResourceTypeDocker} {
		cmds := GetCommandsForResource(rt)
		if len(cmds) == 0 || len(cmds) > 6 {
			t.Errorf("%s surface command set should be small (1-6 commands), got %d", rt, len(cmds))
		}
		for _, c := range cmds {
			if deep[c.Name] {
				t.Errorf("%s should not run deep command %q in a surface scan", rt, c.Name)
			}
		}
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
