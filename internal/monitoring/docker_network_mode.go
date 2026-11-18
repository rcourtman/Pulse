package monitoring

import (
	"strings"
	"unicode"
)

// maxDockerNetworkModeLength bounds a reported container network mode. The
// longest value a runtime writes itself is "container:" and a 64-character
// ID. A network name can be longer, and Docker caps none, so anything past
// this is dropped instead of stored.
const maxDockerNetworkModeLength = 255

// dockerNetworkModeFromReport keeps the runtime's network mode as the agent
// sent it. The value is the runtime's own word for how a container is
// networked (bridge, host, none, a network name, or container:<id> when it
// uses another container's network), so it is trimmed and bounded but never
// rewritten: a network name is case-sensitive, and a container reference only
// means something while it still matches a reported container ID. An empty
// result means the mode is not known, which is what an older agent sends.
func dockerNetworkModeFromReport(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" || len(mode) > maxDockerNetworkModeLength {
		return ""
	}
	if strings.IndexFunc(mode, unicode.IsControl) >= 0 {
		return ""
	}
	return mode
}
