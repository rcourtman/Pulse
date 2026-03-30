package api

import "strings"

func normalizeAITransportResourceType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "truenas":
		return "agent"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}
