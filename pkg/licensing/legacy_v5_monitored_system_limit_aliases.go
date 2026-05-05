package licensing

import "strings"

const legacyV5AgentLimitKey = "max_agents"
const legacyV5NodeLimitKey = "max_nodes"

var legacyV5MonitoredSystemLimitAliasKeys = [...]string{
	legacyV5AgentLimitKey,
	legacyV5NodeLimitKey,
}

func canonicalizeLegacyV5MonitoredSystemLimitKey(key string) (string, bool) {
	switch strings.TrimSpace(key) {
	case legacyV5AgentLimitKey, legacyV5NodeLimitKey:
		return MaxMonitoredSystemsLicenseGateKey, true
	default:
		return "", false
	}
}
