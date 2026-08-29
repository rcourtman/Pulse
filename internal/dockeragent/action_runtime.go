package dockeragent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// NewActionRuntime connects the separate action runner directly to the Docker
// or Podman API using the module's existing typed lifecycle/update managers.
// It configures no Pulse target, report loop, command poller, or generic daemon
// proxy; callers receive only the narrow Agent methods used by the typed bridge.
func NewActionRuntime(runtimeName string, logger *zerolog.Logger) (*Agent, error) {
	runtimePreference, err := normalizeRuntime(runtimeName)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}
	client, info, runtimeKind, err := connectRuntimeFn(runtimePreference, logger)
	if err != nil {
		return nil, fmt.Errorf("connect action runtime: %w", err)
	}
	agent := &Agent{
		docker:             newSwappableDockerClient(client),
		daemonHost:         client.DaemonHost(),
		daemonID:           strings.TrimSpace(info.ID),
		runtime:            runtimeKind,
		runtimePref:        runtimePreference,
		runtimeVer:         strings.TrimSpace(info.ServerVersion),
		logger:             *logger,
		httpClients:        make(map[bool]*http.Client),
		trustedHTTPClients: make(map[string]*http.Client),
	}
	agent.ensureAsyncLifecycle()
	return agent, nil
}
