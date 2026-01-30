package providers

import (
	"net/http"
	"sort"
	"strings"
)

func rateLimitInfo(resp *http.Response) string {
	if resp == nil {
		return ""
	}

	entries := make([]string, 0)
	for key, values := range resp.Header {
		if len(values) == 0 {
			continue
		}
		lower := strings.ToLower(key)
		if !strings.Contains(lower, "ratelimit") &&
			!strings.Contains(lower, "rate-limit") &&
			!strings.Contains(lower, "retry-after") &&
			!strings.Contains(lower, "quota") {
			continue
		}
		value := strings.Join(values, ",")
		if value == "" {
			continue
		}
		entries = append(entries, lower+"="+value)
	}

	if len(entries) == 0 {
		return ""
	}

	sort.Strings(entries)
	const maxEntries = 6
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}
	return "rate_limit: " + strings.Join(entries, ", ")
}

func appendRateLimitInfo(message string, resp *http.Response) string {
	info := rateLimitInfo(resp)
	if info == "" {
		return message
	}
	return message + " (" + info + ")"
}
