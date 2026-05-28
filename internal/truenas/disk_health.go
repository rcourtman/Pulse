package truenas

import (
	"encoding/json"
	"fmt"
	"strings"
)

func diskSMARTHealthFromRaw(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "UNKNOWN", true
	}
	return normalizeExplicitDiskHealth(value), true
}

func diskSMARTHealthFromMap(record map[string]any) (string, bool) {
	if record == nil {
		return "", false
	}
	for _, key := range []string{"smart_status", "smartStatus", "smartstatus"} {
		value, ok := record[key]
		if ok {
			return normalizeExplicitDiskHealth(value), true
		}
	}
	return "", false
}

func normalizeExplicitDiskHealth(value any) string {
	switch typed := value.(type) {
	case nil:
		return "UNKNOWN"
	case string:
		return normalizeDiskHealthText(typed)
	case bool:
		if typed {
			return "PASSED"
		}
		return "FAILED"
	case json.Number:
		return normalizeDiskHealthText(typed.String())
	case float64:
		return normalizeDiskHealthText(fmt.Sprintf("%g", typed))
	case int:
		return normalizeDiskHealthText(fmt.Sprintf("%d", typed))
	case int64:
		return normalizeDiskHealthText(fmt.Sprintf("%d", typed))
	case map[string]any:
		for _, key := range []string{"passed", "healthy"} {
			if nested, ok := typed[key]; ok {
				return normalizeExplicitDiskHealth(nested)
			}
		}
		for _, key := range []string{"status", "health", "value", "rawvalue", "parsed", "raw"} {
			if nested, ok := typed[key]; ok {
				return normalizeExplicitDiskHealth(nested)
			}
		}
		return "UNKNOWN"
	default:
		return normalizeDiskHealthText(fmt.Sprint(typed))
	}
}

func normalizeDiskHealthText(health string) string {
	switch strings.ToUpper(strings.TrimSpace(health)) {
	case "PASSED", "PASS", "OK", "ONLINE", "HEALTHY", "TRUE", "1":
		return "PASSED"
	case "FAILED", "FAIL", "FAILING", "UNHEALTHY", "FALSE", "0":
		return "FAILED"
	case "DEGRADED":
		return "DEGRADED"
	case "", "UNKNOWN", "UNAVAILABLE", "N/A", "NA", "NOT AVAILABLE", "SMART UNAVAILABLE", "UNSUPPORTED", "NULL", "NONE":
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}
