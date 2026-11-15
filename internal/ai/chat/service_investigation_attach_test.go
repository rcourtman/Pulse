package chat

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// An investigation attaches the resource of the finding it investigates as
// its current resource, so a read that names no host runs there
// (PulseToolExecutor.readTargetHost). On Docker fault set D6 the first
// disk_usage call of an investigation into a full mount named no host and
// was refused with "target_host is required".
func TestInvestigationAttachesItsFindingsResource(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{{
		ID: "host-docker01", Hostname: "docker01", Status: "online", Platform: "linux", CommandsEnabled: true, LastSeen: time.Now(),
	}}})
	_, hostID, found := registry.GetByReference("host-docker01")
	if !found {
		t.Fatal("fixture host missing from inventory")
	}

	run := func(t *testing.T, sessionID, resourceID, resourceType string) *SessionStore {
		t.Helper()
		store, err := NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		provider := &stubStreamingProvider{}
		provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "done"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
			return nil
		}
		service := &Service{
			started: true, sessions: store,
			executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{
				ReadState:               registry,
				UnifiedResourceProvider: unifiedresources.NewUnifiedAIAdapter(registry),
			}),
			cfg:                   &config.AIConfig{PatrolModel: "mock:model", ControlLevel: config.ControlLevelReadOnly},
			patrolProviderFactory: func(string) (providers.StreamingProvider, error) { return provider, nil },
		}
		if _, err := service.ExecuteInvestigationStream(context.Background(), InvestigationRunRequest{
			SessionID: sessionID, Prompt: "Investigate the full mount.", SystemPrompt: "Investigate.",
			MaxTurns: 2, MaxEvidenceCalls: 2, ResourceType: resourceType, ResourceID: resourceID,
			Identity: tools.ProposalIdentity{ProposalID: "p-1", FindingID: "f-1", InvestigationID: "i-1"},
		}, func(StreamEvent) {}); err != nil {
			t.Fatal(err)
		}
		return store
	}

	store := run(t, "attach-host", hostID, "agent")
	if !store.GetResolvedContext("attach-host").WasRecentlyAccessed(hostID, time.Minute) {
		t.Fatalf("the investigation did not attach its finding's resource %q", hostID)
	}

	// The finding's type is only a hint: a canonical ID names its own type.
	store = run(t, "attach-wrong-type", hostID, "system-container")
	if !store.GetResolvedContext("attach-wrong-type").WasRecentlyAccessed(hostID, time.Minute) {
		t.Fatalf("a wrong finding type kept the resource %q from being attached", hostID)
	}

	store = run(t, "attach-none", "", "agent")
	if recent := store.GetResolvedContext("attach-none").GetRecentlyAccessedResources(time.Minute); len(recent) != 0 {
		t.Fatalf("an investigation with no finding resource attached %v", recent)
	}
}
