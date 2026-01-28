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
