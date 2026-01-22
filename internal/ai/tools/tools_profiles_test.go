package tools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type mockProfileManager struct {
	scope *AgentScope

	getErr    error
	assignErr error
	applyErr  error

	assignName string
	applyID    string
	applyName  string
	applyNew   bool

	lastGetAgent      string
	lastAssignAgent   string
	lastAssignProfile string
	lastApplyAgent    string
	lastApplyLabel    string
	lastApplySettings map[string]interface{}
}

func (m *mockProfileManager) GetAgentScope(ctx context.Context, agentID string) (*AgentScope, error) {
	m.lastGetAgent = agentID
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.scope, nil
}

func (m *mockProfileManager) AssignProfile(ctx context.Context, agentID, profileID string) (string, error) {
	m.lastAssignAgent = agentID
	m.lastAssignProfile = profileID
	if m.assignErr != nil {
		return "", m.assignErr
	}
	return m.assignName, nil
}

func (m *mockProfileManager) ApplyAgentScope(ctx context.Context, agentID, agentLabel string, settings map[string]interface{}) (string, string, bool, error) {
	m.lastApplyAgent = agentID
	m.lastApplyLabel = agentLabel
	m.lastApplySettings = settings
	if m.applyErr != nil {
		return "", "", false, m.applyErr
	}
	return m.applyID, m.applyName, m.applyNew, nil
}

func TestResolveAgentFromHostname(t *testing.T) {
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "host-1", Hostname: "alpha", DisplayName: "Alpha"},
		},
		DockerHosts: []models.DockerHost{
			{ID: "dock-1", Hostname: "beta", DisplayName: "Beta", CustomDisplayName: "Beta Custom"},
		},
	}

	id, label := resolveAgentFromHostname(state, "ALPHA")
	if id != "host-1" || label != "Alpha" {
		t.Fatalf("expected host match, got id=%q label=%q", id, label)
	}

	id, label = resolveAgentFromHostname(state, "beta")
	if id != "dock-1" || label != "Beta Custom" {
		t.Fatalf("expected docker host match, got id=%q label=%q", id, label)
	}

	id, label = resolveAgentFromHostname(state, "missing")
	if id != "" || label != "" {
		t.Fatalf("expected no match, got id=%q label=%q", id, label)
	}
}

func TestResolveAgentLabel(t *testing.T) {
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "host-1", Hostname: "alpha", DisplayName: "Alpha"},
		},
		DockerHosts: []models.DockerHost{
			{ID: "dock-2", AgentID: "agent-2", Hostname: "dock", DisplayName: "Dock", CustomDisplayName: "Docker Host"},
		},
	}

	if label := resolveAgentLabel(state, "host-1"); label != "Alpha" {
		t.Fatalf("expected host label, got %q", label)
	}

	if label := resolveAgentLabel(state, "agent-2"); label != "Docker Host" {
		t.Fatalf("expected docker label by agent ID, got %q", label)
	}

	if label := resolveAgentLabel(state, "dock-2"); label != "Docker Host" {
		t.Fatalf("expected docker label by host ID, got %q", label)
	}
}

func TestFormatSettingsSummary(t *testing.T) {
	if summary := formatSettingsSummary(nil); summary != "none" {
		t.Fatalf("expected none for empty settings, got %q", summary)
	}

	settings := map[string]interface{}{
		"beta":  true,
		"alpha": 1,
	}
	summary := formatSettingsSummary(settings)
	if summary != "alpha=1, beta=true" {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestDetectAgentModules(t *testing.T) {
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "agent-1", Hostname: "alpha", CommandsEnabled: true, LinkedNodeID: "node-1"},
		},
		DockerHosts: []models.DockerHost{
			{ID: "dock-1", AgentID: "agent-1", Hostname: "dock"},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k8s-1", AgentID: "agent-1", Name: "cluster"},
		},
	}

	modules, commandsEnabled := detectAgentModules(state, "agent-1")
	expected := []string{"docker", "host", "kubernetes", "proxmox"}
	if !reflect.DeepEqual(modules, expected) {
		t.Fatalf("expected modules %v, got %v", expected, modules)
	}
	if commandsEnabled == nil || !*commandsEnabled {
		t.Fatalf("expected commandsEnabled true, got %v", commandsEnabled)
	}

	modules, commandsEnabled = detectAgentModules(state, "missing")
	if modules != nil || commandsEnabled != nil {
		t.Fatalf("expected no matches, got modules=%v commandsEnabled=%v", modules, commandsEnabled)
	}
}

func TestExecuteGetAgentScopeErrors(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})

	result, err := executor.executeGetAgentScope(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing agent args")
	}

	result, err = executor.executeGetAgentScope(context.Background(), map[string]interface{}{
		"hostname": "alpha",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when state provider missing")
	}
}

func TestExecuteGetAgentScopeNotFound(t *testing.T) {
	stateProv := &mockStateProvider{}
	stateProv.On("GetState").Return(models.StateSnapshot{})
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: stateProv,
	})

	result, err := executor.executeGetAgentScope(context.Background(), map[string]interface{}{
		"hostname": "ghost",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected not_found response, got error result")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["error"] != "not_found" {
		t.Fatalf("expected not_found error, got %v", payload["error"])
	}
}

func TestExecuteGetAgentScopeProfileManagerError(t *testing.T) {
	manager := &mockProfileManager{
		getErr: errors.New("boom"),
	}
	stateProv := &mockStateProvider{}
	stateProv.On("GetState").Return(models.StateSnapshot{})
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:       stateProv,
		AgentProfileManager: manager,
	})

	result, err := executor.executeGetAgentScope(context.Background(), map[string]interface{}{
		"agent_id": "agent-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected JSON error payload, got error result")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["error"] != "failed_to_load" {
		t.Fatalf("expected failed_to_load error, got %v", payload["error"])
	}
}

func TestExecuteGetAgentScopeWithProfile(t *testing.T) {
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "agent-1", Hostname: "alpha", DisplayName: "Alpha", CommandsEnabled: true, LinkedNodeID: "node-1"},
		},
		DockerHosts: []models.DockerHost{
			{ID: "dock-1", AgentID: "agent-1", Hostname: "dock"},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k8s-1", AgentID: "agent-1", Name: "cluster"},
		},
	}
	manager := &mockProfileManager{
		scope: &AgentScope{
			AgentID:        "agent-1",
			ProfileID:      "profile-1",
			ProfileName:    "Default",
			ProfileVersion: 2,
			Settings: map[string]interface{}{
				"enable_docker": true,
			},
		},
	}

	stateProv := &mockStateProvider{}
	stateProv.On("GetState").Return(state)
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:       stateProv,
		AgentProfileManager: manager,
	})

	result, err := executor.executeGetAgentScope(context.Background(), map[string]interface{}{
		"agent_id": "agent-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected successful JSON response")
	}

	var response AgentScopeResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response.ProfileID != "profile-1" || response.ProfileName != "Default" || response.ProfileVersion != 2 {
		t.Fatalf("unexpected profile info: %+v", response)
	}
	if response.AgentLabel != "Alpha" {
		t.Fatalf("expected agent label Alpha, got %q", response.AgentLabel)
	}
	if response.CommandsEnabled == nil || !*response.CommandsEnabled {
		t.Fatalf("expected commands enabled true, got %v", response.CommandsEnabled)
	}
}

func TestExecuteSetAgentScopeErrors(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, err := executor.executeSetAgentScope(context.Background(), map[string]interface{}{
		"agent_id": "agent-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError || !strings.Contains(result.Content[0].Text, "not available") {
		t.Fatalf("expected not available message, got %+v", result)
	}

	executor = NewPulseToolExecutor(ExecutorConfig{
		AgentProfileManager: &mockProfileManager{},
	})
	result, err = executor.executeSetAgentScope(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing agent args")
	}

	result, err = executor.executeSetAgentScope(context.Background(), map[string]interface{}{
		"agent_id":   "agent-1",
		"profile_id": "profile-1",
		"settings": map[string]interface{}{
			"enable_host": true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for profile_id + settings")
	}
}

func TestExecuteSetAgentScopeSuggestProfile(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		ControlLevel:        ControlLevelSuggest,
		AgentProfileManager: &mockProfileManager{},
	})

	result, err := executor.executeSetAgentScope(context.Background(), map[string]interface{}{
		"agent_id":   "agent-1",
		"profile_id": "profile-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected suggestion response")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["type"] != "suggestion" || payload["action"] != "assign_profile" {
		t.Fatalf("unexpected suggestion payload: %v", payload)
	}
}

func TestExecuteSetAgentScopeSuggestSettings(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		ControlLevel:        ControlLevelSuggest,
		AgentProfileManager: &mockProfileManager{},
	})

	result, err := executor.executeSetAgentScope(context.Background(), map[string]interface{}{
		"agent_id": "agent-1",
		"settings": map[string]interface{}{
			"alpha": 1,
			"beta":  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected suggestion response")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["type"] != "suggestion" || payload["action"] != "apply_settings" {
		t.Fatalf("unexpected suggestion payload: %v", payload)
	}
	message, _ := payload["message"].(string)
	if !strings.Contains(message, "alpha=1, beta=true") {
		t.Fatalf("expected settings summary, got %q", message)
	}
}

func TestExecuteSetAgentScopeAssignProfile(t *testing.T) {
	manager := &mockProfileManager{
		assignName: "Gold",
	}
	executor := NewPulseToolExecutor(ExecutorConfig{
		ControlLevel:        ControlLevelAutonomous,
		AgentProfileManager: manager,
	})

	result, err := executor.executeSetAgentScope(context.Background(), map[string]interface{}{
		"agent_id":   "agent-1",
		"profile_id": "profile-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success response")
	}
	if manager.lastAssignAgent != "agent-1" || manager.lastAssignProfile != "profile-1" {
		t.Fatalf("assign not called with expected values: %+v", manager)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["action"] != "assigned" || payload["profile_name"] != "Gold" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestExecuteSetAgentScopeApplySettings(t *testing.T) {
	manager := &mockProfileManager{
		applyID:   "profile-2",
		applyName: "Custom",
		applyNew:  true,
	}
	executor := NewPulseToolExecutor(ExecutorConfig{
		ControlLevel:        ControlLevelAutonomous,
		AgentProfileManager: manager,
	})

	result, err := executor.executeSetAgentScope(context.Background(), map[string]interface{}{
		"agent_id": "agent-1",
		"settings": map[string]interface{}{
			"enable_host": true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success response")
	}
	if manager.lastApplyAgent != "agent-1" || manager.lastApplySettings == nil {
		t.Fatalf("apply not called with expected values: %+v", manager)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["action"] != "created" || payload["profile_name"] != "Custom" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestExecuteSetAgentScopeApplySettingsUpdated(t *testing.T) {
	manager := &mockProfileManager{
		applyID:   "profile-3",
		applyName: "Existing",
		applyNew:  false,
	}
	executor := NewPulseToolExecutor(ExecutorConfig{
		ControlLevel:        ControlLevelAutonomous,
		AgentProfileManager: manager,
	})

	result, err := executor.executeSetAgentScope(context.Background(), map[string]interface{}{
		"agent_id": "agent-1",
		"settings": map[string]interface{}{
			"enable_docker": true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success response")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["action"] != "updated" || payload["profile_name"] != "Existing" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}
