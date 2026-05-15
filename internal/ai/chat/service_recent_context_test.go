package chat

import (
	"strings"
	"testing"
	"time"
)

func TestInjectRecentSessionContext_InjectsNeutralResourceFacts(t *testing.T) {
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
		ResourceID: "app-container:minipc:abc",
		Name:       "api",
		Kind:       "app-container",
		Node:       "minipc",
		TargetHost: "host-1",
	}
	resolved.AddResourceWithExplicitAccess(primary.Name, primary)

	messages := []Message{{Role: "user", Content: "show its logs"}}
	service := &Service{}
	service.injectRecentSessionContext(session.ID, messages, store)

	content := messages[0].Content
	if content == "show its logs" {
		t.Fatalf("expected recent context to be injected")
	}
	if !strings.Contains(content, "Session context from earlier Assistant turns. Use only if relevant to the user's message; otherwise ignore it or ask a clarifying question.") {
		t.Fatalf("expected neutral session context framing, got: %s", content)
	}
	if !strings.Contains(content, "- api (app-container on minipc); tool addressing fact: target_host=\"host-1\"") {
		t.Fatalf("expected primary resource facts, got: %s", content)
	}
	if !strings.Contains(content, "- db (vm on node-2)") {
		t.Fatalf("expected secondary resource summary, got: %s", content)
	}
	if strings.Contains(content, "Log routing context") || strings.Contains(content, "User question (targeted)") {
		t.Fatalf("expected no prompt-keyword routing instruction, got: %s", content)
	}
	if !strings.Contains(content, "User message:\nshow its logs") {
		t.Fatalf("expected original user message to remain neutral, got: %s", content)
	}
}

func TestInjectRecentSessionContext_NoRecentDoesNotModify(t *testing.T) {
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
	service.injectRecentSessionContext(session.ID, messages, store)

	if messages[0].Content != "restart it" {
		t.Fatalf("expected message to remain unchanged")
	}
}

func TestInjectRecentSessionContext_PrimaryNameFallback(t *testing.T) {
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
	service.injectRecentSessionContext(session.ID, messages, store)

	content := messages[0].Content
	if !strings.Contains(content, "- node:alpha; tool addressing fact: target_host=\"node:alpha\"") {
		t.Fatalf("expected resource ID label, got: %s", content)
	}
	if strings.Contains(content, "Explicit target") || strings.Contains(content, "Log routing context") {
		t.Fatalf("expected no targeted rewrite or log routing instruction, got: %s", content)
	}
}

func TestInjectRecentSessionContext_UsesResourceIDFactForTrueNASApp(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	resolved := store.GetResolvedContext(session.ID)
	resource := &ResolvedResource{
		ResourceID: "app-container:truenas-main:nextcloud",
		Name:       "Nextcloud",
		Kind:       "app-container",
		Node:       "truenas-main",
		TargetHost: "truenas-main",
		Adapter:    "truenas",
	}
	resolved.AddResourceWithExplicitAccess(resource.Name, resource)

	messages := []Message{{Role: "user", Content: "show its logs"}}
	service := &Service{}
	service.injectRecentSessionContext(session.ID, messages, store)

	content := messages[0].Content
	if !strings.Contains(content, "- Nextcloud (app-container on truenas-main); tool addressing fact: resource_id=\"Nextcloud\"") {
		t.Fatalf("expected resource_id fact, got: %s", content)
	}
}

func TestInjectRecentSessionContext_UsesQueryResourceIDFactForVMware(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	resolved := store.GetResolvedContext(session.ID)
	resource := &ResolvedResource{
		ResourceID: "agent:vc-1:host:host-101",
		Name:       "esxi-01.lab.local",
		Kind:       "agent",
		Node:       "Lab VC",
		Adapter:    "vmware-vsphere",
	}
	resolved.AddResourceWithExplicitAccess(resource.Name, resource)

	messages := []Message{{Role: "user", Content: "show its logs"}}
	service := &Service{}
	service.injectRecentSessionContext(session.ID, messages, store)

	content := messages[0].Content
	if !strings.Contains(content, "- esxi-01.lab.local (agent on Lab VC); tool addressing fact: resource_id=\"esxi-01.lab.local\"") {
		t.Fatalf("expected resource_id fact, got: %s", content)
	}
	if strings.Contains(content, "target_host=\"esxi-01.lab.local\"") {
		t.Fatalf("expected VMware recent context to avoid target_host routing, got: %s", content)
	}
}
