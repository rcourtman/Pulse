package licensing

import (
	"encoding/json"
	"strings"
)

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

func decodeLegacyV5MonitoredSystemLimitFromJSON(data []byte) (int, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, false, err
	}
	if _, hasCanonical := raw[MaxMonitoredSystemsLicenseGateKey]; hasCanonical {
		return 0, false, nil
	}
	for _, key := range legacyV5MonitoredSystemLimitAliasKeys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		var parsed int
		if err := json.Unmarshal(value, &parsed); err != nil {
			return 0, false, err
		}
		return parsed, true, nil
	}
	return 0, false, nil
}
