package licensing

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ConversionValidationReason(err error) string {
	if err == nil {
		return "unknown"
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "type is required"):
		return "missing_type"
	case strings.Contains(msg, "is not supported"):
		return "unsupported_type"
	case strings.Contains(msg, "surface is required"):
		return "missing_surface"
	case strings.Contains(msg, "timestamp is required"):
		return "missing_timestamp"
	case strings.Contains(msg, "idempotency_key is required"):
		return "missing_idempotency_key"
	case strings.Contains(msg, "tenant_mode must be"):
		return "invalid_tenant_mode"
	case strings.Contains(msg, "capability is required"):
		return "missing_capability"
	case strings.Contains(msg, "limit_key is required"):
		return "missing_limit_key"
	default:
		return "validation_error"
	}
}

func ParseOptionalTimeParam(raw string, defaultValue time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue.UTC(), nil
	}

	// RFC3339 / RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}

	// Date-only (local midnight is ambiguous; use UTC midnight).
	if t, err := time.ParseInLocation("2006-01-02", raw, time.UTC); err == nil {
		return t.UTC(), nil
	}

	// Unix seconds or milliseconds.
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		// Heuristic: >= 10^12 is likely ms.
		if i >= 1_000_000_000_000 {
			return time.UnixMilli(i).UTC(), nil
		}
		return time.Unix(i, 0).UTC(), nil
	}

	return time.Time{}, fmt.Errorf("unsupported time format")
}
