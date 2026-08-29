package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

func TestNewActionRunnerClientIsTypedOnlyAndEmitsExplicitRole(t *testing.T) {
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: "https://pulse.example", APIToken: "separate-secret",
		StateDir: t.TempDir(), HealthPath: filepath.Join(t.TempDir(), "health.json"), Logger: &logger,
	}, "agent-1", "host-1", "v1")
	t.Cleanup(func() { _ = client.Close() })
	if !client.actionRunnerOnly || client.runtimeRole != agentexec.RuntimeRoleActionRunner || client.actionCapability != agentexec.ActionCapabilityTypedV1 {
		t.Fatalf("runner protocol ceiling not configured: %+v", client)
	}
	for _, message := range []messageType{msgTypeExecuteCmd, msgTypeReadFile, msgTypeDeployPreflight, msgTypeDeployInstall, msgTypeDeployCancel} {
		if allowedActionRunnerMessage(message) {
			t.Errorf("forbidden message %q was admitted", message)
		}
	}
	for _, message := range []messageType{msgTypeHostUpdate, msgTypeHostStorageCleanup, msgTypeDockerContainerLifecycle, msgTypeDockerContainerUpdate, msgTypeOperationQuery, msgTypeCancelCmd} {
		if !allowedActionRunnerMessage(message) {
			t.Errorf("typed message %q was rejected", message)
		}
	}
}

func TestActionRunnerTransportRegistersRoleWritesHealthAndRejectsGenericExec(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	registration := make(chan registerPayload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(w, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message wsMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		var payload registerPayload
		if json.Unmarshal(message.Payload, &payload) != nil {
			return
		}
		registration <- payload
		ack, _ := json.Marshal(registeredPayload{Success: true})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: ack})
		forbidden, _ := json.Marshal(executeCommandPayload{RequestID: "r1", Command: "id", TargetType: "agent"})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeExecuteCmd, Timestamp: time.Now(), Payload: forbidden})
	}))
	defer server.Close()
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: server.URL, APIToken: "runner-token", StateDir: filepath.Join(dir, "state"),
		HealthPath: healthPath, InsecureSkipVerify: true, Logger: &logger,
	}, "agent-1", "host-1", "v1")
	defer client.Close()
	err := client.connectAndHandle(context.Background())
	if err == nil || !strings.Contains(err.Error(), "forbidden message") {
		t.Fatalf("transport error = %v", err)
	}
	payload := <-registration
	if payload.RuntimeRole != agentexec.RuntimeRoleActionRunner || payload.ActionCapability != agentexec.ActionCapabilityTypedV1 || payload.Token != "runner-token" {
		t.Fatalf("registration = %+v", payload)
	}
	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	var health actionRunnerHealth
	if json.Unmarshal(data, &health) != nil || !health.Registered || health.HostID != "agent-1" {
		t.Fatalf("health = %s", data)
	}
}

func TestActionRunnerHealthIsAtomicBoundedAndSecretFree(t *testing.T) {
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: "https://pulse.example", APIToken: "must-not-appear",
		StateDir: filepath.Join(dir, "state"), HealthPath: healthPath, Logger: &logger,
	}, "agent-1", "host-1", "v1")
	t.Cleanup(func() { _ = client.Close() })
	if err := client.writeActionRunnerHealth(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	if json.Valid(data) == false || string(data) == "" {
		t.Fatalf("invalid health marker: %q", data)
	}
	if contains := string(data); contains == "must-not-appear" || jsonContains(data, "must-not-appear") {
		t.Fatal("health marker leaked the action credential")
	}
	var health actionRunnerHealth
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatal(err)
	}
	if !health.Registered || health.RuntimeRole != agentexec.RuntimeRoleActionRunner || health.HostID != "agent-1" || health.Server != "https://pulse.example" || health.RegisteredAt.IsZero() {
		t.Fatalf("health = %+v", health)
	}
	info, err := os.Stat(healthPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("health mode = %v, %v", info.Mode().Perm(), err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".health-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary health files = %v, %v", matches, err)
	}
}

func jsonContains(data []byte, value string) bool {
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return false
	}
	return containsJSONValue(decoded, value)
}

func containsJSONValue(value any, secret string) bool {
	switch typed := value.(type) {
	case string:
		return typed == secret
	case []any:
		for _, item := range typed {
			if containsJSONValue(item, secret) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if containsJSONValue(item, secret) {
				return true
			}
		}
	}
	return false
}
