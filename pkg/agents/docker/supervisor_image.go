package dockeragent

import "strings"

// UsesHomeAssistantSupervisorAlias identifies the Supervisor's local "latest"
// alias. Home Assistant chooses the installed release through its own update
// channel; GHCR's "latest" digest does not say whether that channel has an
// update. An explicit version or digest, or a Compose deployment, is not
// this alias. Keep this predicate shared by collection, ingest and alerting.
func UsesHomeAssistantSupervisorAlias(name, image string, labels map[string]string) bool {
	if strings.TrimPrefix(name, "/") != "hassio_supervisor" || labels["com.docker.compose.project"] != "" {
		return false
	}
	switch strings.TrimSuffix(image, ":latest") {
	case "ghcr.io/home-assistant/amd64-hassio-supervisor",
		"ghcr.io/home-assistant/aarch64-hassio-supervisor":
		return true
	default:
		return false
	}
}
