package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// Patrol's scan is told which actions can run by the action broker, the one
// an investigation asks. Starting the alert bridge connects them. A crashed
// container on a host whose agent has commands off declares Start and Update,
// and the scan must be told that neither can run and why.
func TestStartingTheAlertBridgeGivesPatrolTheBrokersRunnableActions(t *testing.T) {
	const containerID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	now := time.Now().UTC()
	// crashedWorker builds real monitor state, registry and Docker executor
	// for one exited container, and returns the container's inventory ID.
	crashedWorker := func(t *testing.T, commandsEnabled, sessionConnected bool) (*ResourceHandlers, string) {
		t.Helper()
		h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
		h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
			LastUpdate: now,
			Hosts: []models.Host{{
				ID: "agent-docker01", Hostname: "docker01", MachineID: "machine-docker01",
				Status: "online", LastSeen: now, CommandsEnabled: commandsEnabled,
			}},
			DockerHosts: []models.DockerHost{{
				ID: "agent-docker01", AgentID: "agent-docker01", Hostname: "docker01", MachineID: "machine-docker01",
				Status: "online", LastSeen: now, Runtime: "docker",
				Containers: []models.DockerContainer{{ID: containerID, Name: "worker", Image: "worker:1.0", State: "exited", Status: "Exited (139) 5 minutes ago", ExitCode: 139}},
			}},
		}})
		agents := &fakeDockerActionAgentCommander{connected: map[string]bool{"agent-docker01": sessionConnected}}
		h.SetActionExecutor(newRoutedActionExecutor(h, newDockerContainerActionExecutor(h, agents)))
		registry, err := h.buildRegistry("default")
		if err != nil {
			t.Fatalf("buildRegistry: %v", err)
		}
		for _, container := range registry.DockerContainers() {
			if container != nil && container.Name() == "worker" {
				return h, container.ID()
			}
		}
		t.Fatal("the worker container is not in the registry")
		return nil, ""
	}

	monitor, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(monitor.Stop)
	router := newTestRouter(t, &config.Config{DataPath: t.TempDir()}, monitor, nil, nil, func() error { return nil }, "test")
	aiService := ai.NewService(config.NewConfigPersistence(t.TempDir()), nil)
	aiService.SetStateProvider(monitor)
	patrol := aiService.GetPatrolService()
	if patrol == nil {
		t.Fatal("the AI service has no Patrol service")
	}
	if _, ok, _ := patrol.ActionCapabilities(context.Background(), "anything"); ok {
		t.Fatal("Patrol has an action lookup before the alert bridge started")
	}

	var handlers *ResourceHandlers
	router.aiSettingsHandler.SetActionBrokerFactory(func(orgID string) aicontracts.OrchestratorActionBroker {
		return NewPatrolActionBroker(orgID, handlers)
	})
	bridge := router.startUnifiedAlertBridge("default", aiService, monitor.GetAlertManager())
	if bridge == nil {
		t.Fatal("no alert bridge was started")
	}
	t.Cleanup(bridge.Stop)

	// Commands off: the declared actions are there and none can run.
	var workerID string
	handlers, workerID = crashedWorker(t, false, false)
	declared, err := handlers.ActionLifecycle().DeclaredCapabilities(context.Background(), "default", workerID)
	if err != nil || len(declared) == 0 {
		t.Fatalf("the crashed worker declares no action (%v), so this test proves nothing: %#v", err, declared)
	}
	catalog, ok, err := patrol.ActionCapabilities(context.Background(), workerID)
	if !ok || err != nil {
		t.Fatalf("Patrol could not ask what can run: ok=%v err=%v", ok, err)
	}
	if len(catalog.Capabilities) != 0 {
		t.Errorf("commands are off and Patrol is told these can run: %#v", catalog.Capabilities)
	}
	if len(catalog.Unavailable) != len(declared) {
		t.Errorf("%d declared actions, %d listed as unavailable", len(declared), len(catalog.Unavailable))
	}
	if host := catalog.CommandsDisabled; host == nil || host.HostName != "docker01" || host.Reason != "Commands are not enabled on the Pulse agent on docker01." {
		t.Errorf("Patrol is not told that commands are off on docker01: %#v", host)
	}

	// Commands on and the agent connected: the same lookup now lists them.
	handlers, workerID = crashedWorker(t, true, true)
	catalog, ok, err = patrol.ActionCapabilities(context.Background(), workerID)
	if !ok || err != nil {
		t.Fatalf("Patrol could not ask what can run: ok=%v err=%v", ok, err)
	}
	canStart := false
	for _, capability := range catalog.Capabilities {
		canStart = canStart || capability.Name == "start"
	}
	if !canStart || catalog.CommandsDisabled != nil {
		t.Errorf("commands are on and the agent is connected, and Patrol is not told Start can run: %#v", catalog)
	}
}
