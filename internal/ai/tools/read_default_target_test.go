package tools

import (
	"context"
	"testing"
)

// On Docker fault set D6 an investigation's first pulse_read disk_usage call
// on the finding's own mount named no host and was refused with "target_host
// is required", which cost one of its ten evidence calls. An investigation
// now attaches its finding's resource, and a read that names neither a host
// nor a resource runs on it.
func TestReadThatNamesNoHostRunsOnTheAttachedResource(t *testing.T) {
	provider := newCommandCapableAgentQueryProvider()
	executor := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: provider, ReadState: provider.ResourceRegistry})
	executor.SetResolvedContext(&mockResolvedContext{
		resources: make(map[string]ResolvedResourceInfo),
		aliases:   make(map[string]ResolvedResourceInfo),
	})

	// Nothing attached: the read still has to name its host.
	if got := executor.readTargetHost("", ""); got != "" {
		t.Fatalf("with nothing attached, readTargetHost = %q, want empty (target_host is required)", got)
	}

	// A wrong finding type with a canonical "<type>-<hash>" ID is covered in
	// chat's TestInvestigationAttachesItsFindingsResource, over real inventory IDs.
	if !executor.AttachResource(context.Background(), "lab-host", "agent") {
		t.Fatal("AttachResource did not attach the agent host")
	}
	if got := executor.readTargetHost("", ""); got != currentResourceHandle {
		t.Fatalf("readTargetHost with no host = %q, want %q", got, currentResourceHandle)
	}
	host, err := executor.resolveCurrentResourceCommandTarget(currentResourceHandle)
	if err != nil || (host != "lab-host" && host != "agent:lab-host") {
		t.Fatalf("current resource command target = %q, %v; want the lab-host agent", host, err)
	}

	// A host the model names, and a resource it names, are left as they are.
	if got := executor.readTargetHost("other-host", ""); got != "other-host" {
		t.Errorf("named host replaced: %q", got)
	}
	if got := executor.readTargetHost("", "app-container-0123456789abcdef"); got != "" {
		t.Errorf("a read naming another resource was sent to the attached host: %q", got)
	}
	if got := executor.readTargetHost("", currentResourceHandle); got != currentResourceHandle {
		t.Errorf("current_resource resource_id = %q, want %q", got, currentResourceHandle)
	}

	// A resource that does not exist attaches nothing.
	fresh := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: provider, ReadState: provider.ResourceRegistry})
	fresh.SetResolvedContext(&mockResolvedContext{resources: make(map[string]ResolvedResourceInfo), aliases: make(map[string]ResolvedResourceInfo)})
	if fresh.AttachResource(context.Background(), "agent-ffffffffffffffff", "agent") {
		t.Error("an unknown resource was attached")
	}
	if got := fresh.readTargetHost("", ""); got != "" {
		t.Errorf("after a failed attach, readTargetHost = %q, want empty", got)
	}
}
