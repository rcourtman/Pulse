package unifiedresources

import "strings"

func statusFromNode(status string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "running", "ok":
		return StatusOnline
	case "offline", "down", "stopped":
		return StatusOffline
	case "warning", "degraded":
		return StatusWarning
	default:
		return StatusUnknown
	}
}

func statusFromHost(status string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "running", "ok":
		return StatusOnline
	case "offline", "down", "stopped":
		return StatusOffline
	case "warning", "degraded":
		return StatusWarning
	default:
		return StatusUnknown
	}
}

func statusFromGuest(status string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "online":
		return StatusOnline
	case "stopped", "offline", "paused":
		return StatusOffline
	case "warning", "degraded":
		return StatusWarning
	default:
		return StatusUnknown
	}
}

func statusFromDockerState(state string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return StatusOnline
	case "created", "exited", "dead", "paused":
		return StatusOffline
	case "restarting":
		return StatusWarning
	default:
		return StatusUnknown
	}
}
