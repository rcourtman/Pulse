package mutationregistry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoRawModelCommandOrRollbackAuthority(t *testing.T) {
	root := repoRoot(t)
	assertSourceOmits(t, filepath.Join(root, "internal/agentcapabilities/schema.go"),
		"legacyAssistantRunCommandTool()",
		"Execute a shell command. By default runs on the current target")
	assertSourceOmits(t, filepath.Join(root, "internal/ai/tools/tools_control.go"),
		"return e.executeRunCommand(ctx, args)")
	assertSourceOmits(t, filepath.Join(root, "internal/ai/tools/tools_file.go"),
		"return exec.executeFileEdit(ctx, args)")
}

func TestNoDirectDockerMutationQueueCaller(t *testing.T) {
	root := repoRoot(t)
	for _, path := range []string{
		"internal/api/docker_agents.go",
		"internal/ai/tools/adapters.go",
	} {
		assertSourceOmits(t, filepath.Join(root, path),
			".QueueDockerContainerUpdateCommand(",
			".QueueDockerUpdateAllCommand(")
	}
}

func TestWebSocketTransportCannotOriginateAuthority(t *testing.T) {
	for _, entry := range Entries() {
		if entry.Origin != OriginTransport {
			continue
		}
		if entry.Disposition == DispositionLifecycle && entry.Delivery != DeliveryCommittedLifecycle {
			t.Errorf("%s: transport may run only after committed lifecycle authority", entry.ID)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func assertSourceOmits(t *testing.T, path string, forbidden ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, text := range forbidden {
		if strings.Contains(source, text) {
			t.Errorf("%s still contains retired authority %q", path, text)
		}
	}
}
