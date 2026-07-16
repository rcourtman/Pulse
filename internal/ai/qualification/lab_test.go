package qualification

import (
	"context"
	"strings"
	"testing"
)

func TestShellQuotePreservesRemoteDockerArguments(t *testing.T) {
	got := shellQuote("while true; do echo 'safe'; done")
	if got != `'while true; do echo '"'"'safe'"'"'; done'` {
		t.Fatalf("quoted argument = %s", got)
	}
}

func TestDockerTargetRequiresExplicitDisposableLabSelection(t *testing.T) {
	manifest := validTestManifest()
	if err := (DockerTarget{}).Validate(manifest); err == nil {
		t.Fatal("implicit Docker daemon must be rejected")
	}
	if err := (DockerTarget{Context: "colima", SSHHost: "lab"}).Validate(manifest); err == nil {
		t.Fatal("context and SSH target together must be rejected")
	}
	if err := (DockerTarget{SSHHost: "lab", AllowSharedHost: true}).Validate(manifest); err == nil {
		t.Fatal("shared host must also be approved by the manifest")
	}
	manifest.Lab.SharedHostOK = true
	if err := (DockerTarget{SSHHost: "lab", AllowSharedHost: true}).Validate(manifest); err != nil {
		t.Fatalf("explicit manifest-approved shared lab rejected: %v", err)
	}
}

type recordingCommandRunner struct {
	calls []string
}

func (r *recordingCommandRunner) Run(_ context.Context, name string, args ...string) (CommandResult, error) {
	r.calls = append(r.calls, strings.Join(append([]string{name}, args...), " "))
	return CommandResult{}, nil
}

func TestCleanupDiscoversResourcesOnlyByExactRunLabel(t *testing.T) {
	runner := &recordingCommandRunner{}
	labDriver := NewDockerLab(runner, DockerTarget{Context: "colima"})
	lab := &PreparedLab{RunID: "q-20260714-deadbeef", PreInventory: DockerInventory{}}
	manifest := validTestManifest()
	result := labDriver.Cleanup(context.Background(), manifest, lab)
	if !result.Passed {
		t.Fatalf("empty exact-label cleanup failed: %+v", result)
	}
	want := "--filter label=" + labRunLabel + "=" + labRunToken(lab.RunID)
	filteredLists := 0
	for _, call := range runner.calls {
		if strings.Contains(call, " --filter ") {
			filteredLists++
			if !strings.Contains(call, want) {
				t.Fatalf("cleanup used a non-exact selector: %s", call)
			}
		}
	}
	if filteredLists != 6 {
		t.Fatalf("filtered cleanup list calls = %d, want 6 across two passes: %v", filteredLists, runner.calls)
	}
}

func TestDockerInventoryIncludesImagesAndUsesSetSemantics(t *testing.T) {
	a := DockerInventory{Containers: []string{"c"}, Volumes: []string{"v"}, Networks: []string{"n"}, Images: uniqueStrings([]string{"i", "i"})}
	b := DockerInventory{Containers: []string{"c"}, Volumes: []string{"v"}, Networks: []string{"n"}, Images: []string{"i"}}
	if !inventoryEqual(a, b) {
		t.Fatalf("inventories differ: a=%+v b=%+v", a, b)
	}
	b.Images = append(b.Images, "unexpected")
	if inventoryEqual(a, b) {
		t.Fatal("image drift must fail teardown inventory comparison")
	}
}
