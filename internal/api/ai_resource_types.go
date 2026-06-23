package api

import "github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"

func normalizeAITransportResourceType(raw string) string {
	return agentcapabilities.NormalizeActionResourceType(raw)
}
