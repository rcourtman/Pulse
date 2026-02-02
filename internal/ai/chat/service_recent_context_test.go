package chat

import (
	"strings"
	"testing"
	"time"
)

func TestShouldInjectRecentContext(t *testing.T) {
	if !shouldInjectRecentContext("show me its status") {
		t.Fatalf("expected pronoun prompt to trigger recent context")
	}
	if !shouldInjectRecentContext("restart the service") {
		t.Fatalf("expected noun prompt to trigger recent context")
	}
	if shouldInjectRecentContext("what is the uptime") {
		t.Fatalf("expected unrelated prompt to skip recent context")
	}
}

func TestInjectRecentContextIfNeeded_InjectsSummaryAndInstruction(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	resolved := store.GetResolvedContext(session.ID)

	secondary := &ResolvedResource{
		ResourceID:   "vm:node-2:201",
		Name:         "db",
		ResourceType: "vm",
		Scope:        ResourceScope{HostName: "node-2"},
	}
	resolved.AddResourceWithExplicitAccess(secondary.Name, secondary)

	time.Sleep(10 * time.Millisecond)

	primary := &ResolvedResource{
		ResourceID: "docker_container:minipc:abc",
		Name:       "api",
		Kind:       "docker_container",
		Node:       "minipc",
		TargetHost: "host-1",
	}
	resolved.AddResourceWithExplicitAccess(primary.Name, primary)

	messages := []Message{{Role: "user", Content: "show its logs"}}
	service := &Service{}
	service.injectRecentContextIfNeeded("show its logs", session.ID, messages, store)

	content := messages[0].Content
	if content == "show its logs" {
		t.Fatalf("expected recent context to be injected")
	}
	if !strings.Contains(content, "Context: The most recently referenced resource is api (docker_container on minipc).") {
		t.Fatalf("expected primary resource summary, got: %s", content)
	}
	if !strings.Contains(content, "Other recent resources:\n- db (vm on node-2)") {
		t.Fatalf("expected secondary resource summary, got: %s", content)
	}
	if !strings.Contains(content, "Use target_host=\"host-1\".") {
		t.Fatalf("expected target host hint, got: %s", content)
	}
	if !strings.Contains(content, "Instruction: Show logs for api (last 50 lines).") {
		t.Fatalf("expected log instruction, got: %s", content)
	}
	if !strings.Contains(content, "Use pulse_read action=logs target_host=\"host-1\" lines=50.") {
		t.Fatalf("expected log tool instruction, got: %s", content)
	}
	if !strings.Contains(content, "Explicit target: api") {
		t.Fatalf("expected explicit target, got: %s", content)
	}
	if !strings.Contains(content, "User question (targeted): show its logs") {
		t.Fatalf("expected targeted user question, got: %s", content)
	}
}

func TestInjectRecentContextIfNeeded_NoRecentDoesNotModify(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	resolved := store.GetResolvedContext(session.ID)
	resolved.AddResource("alpha", &ResolvedResource{ResourceID: "node:alpha", Name: "alpha"})

	messages := []Message{{Role: "user", Content: "restart it"}}
	service := &Service{}
	service.injectRecentContextIfNeeded("restart it", session.ID, messages, store)

	if messages[0].Content != "restart it" {
		t.Fatalf("expected message to remain unchanged")
	}
}

func TestInjectRecentContextIfNeeded_PrimaryNameFallback(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	resolved := store.GetResolvedContext(session.ID)
	resource := &ResolvedResource{ResourceID: "node:alpha"}
	resolved.AddResourceWithExplicitAccess("alpha", resource)

	messages := []Message{{Role: "user", Content: "show its logs"}}
	service := &Service{}
	service.injectRecentContextIfNeeded("show its logs", session.ID, messages, store)

	content := messages[0].Content
	if !strings.Contains(content, "Context: The most recently referenced resource is node:alpha.") {
		t.Fatalf("expected resource ID label, got: %s", content)
	}
	if !strings.Contains(content, "Explicit target: node:alpha") {
		t.Fatalf("expected fallback explicit target, got: %s", content)
	}
	if !strings.Contains(content, "Use pulse_read action=logs target_host=\"node:alpha\" lines=50.") {
		t.Fatalf("expected log instruction to use primary name, got: %s", content)
	}
}
