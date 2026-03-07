package api

import (
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		// Server errors (5xx)
		{"500 internal server error", 500, "server_error"},
		{"501 not implemented", 501, "server_error"},
		{"502 bad gateway", 502, "server_error"},
		{"503 service unavailable", 503, "server_error"},
		{"599 edge case", 599, "server_error"},

		// Client errors (4xx)
		{"400 bad request", 400, "client_error"},
		{"401 unauthorized", 401, "client_error"},
		{"403 forbidden", 403, "client_error"},
		{"404 not found", 404, "client_error"},
		{"429 too many requests", 429, "client_error"},
		{"499 edge case", 499, "client_error"},

		// Success (2xx)
		{"200 OK", 200, "none"},
		{"201 created", 201, "none"},
		{"204 no content", 204, "none"},

		// Redirects (3xx)
		{"301 moved permanently", 301, "none"},
		{"302 found", 302, "none"},
		{"304 not modified", 304, "none"},

		// Informational (1xx)
		{"100 continue", 100, "none"},
		{"101 switching protocols", 101, "none"},

		// Edge cases
		{"0 zero status", 0, "none"},
		{"399 boundary below client error", 399, "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyStatus(tt.status)
			if got != tt.want {
				t.Errorf("classifyStatus(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid numeric strings
		{"single digit", "0", true},
		{"single digit 9", "9", true},
		{"multiple digits", "123", true},
		{"large number", "9876543210", true},
		{"leading zeros", "007", true},

		// Invalid strings
		{"empty string", "", false},
		{"letter", "a", false},
		{"mixed alphanumeric", "123abc", false},
		{"space", " ", false},
		{"number with space", "12 34", false},
		{"negative number", "-123", false},
		{"decimal number", "12.34", false},
		{"hexadecimal prefix", "0x10", false},
		{"special characters", "12@34", false},
		{"unicode digits", "１２３", false}, // fullwidth digits
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNumeric(tt.input)
			if got != tt.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLooksLikeUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid UUIDs
		{"lowercase uuid", "550e8400-e29b-41d4-a716-446655440000", true},
		{"uppercase uuid", "550E8400-E29B-41D4-A716-446655440000", true},
		{"mixed case uuid", "550e8400-E29B-41d4-A716-446655440000", true},
		{"nil uuid", "00000000-0000-0000-0000-000000000000", true},
		{"max uuid", "ffffffff-ffff-ffff-ffff-ffffffffffff", true},

		// Invalid UUIDs - wrong length
		{"empty string", "", false},
		{"too short", "550e8400-e29b-41d4-a716", false},
		{"too long", "550e8400-e29b-41d4-a716-4466554400001", false},
		{"35 chars", "550e8400-e29b-41d4-a716-44665544000", false},
		{"37 chars", "550e8400-e29b-41d4-a716-4466554400001", false},

		// Invalid UUIDs - wrong dash positions
		{"dash at wrong position 0", "-50e8400-e29b-41d4-a716-44665544000", false},
		{"dash at wrong position 7", "550e840-0e29b-41d4-a716-446655440000", false},
		{"no dashes", "550e8400e29b41d4a716446655440000xxxx", false},
		{"extra dash", "550e8400-e29b-41d4-a716-4466-5544000", false},

		// Invalid UUIDs - invalid characters
		{"letter g", "g50e8400-e29b-41d4-a716-446655440000", false},
		{"letter z", "550e8400-z29b-41d4-a716-446655440000", false},
		{"space in uuid", "550e8400 e29b-41d4-a716-446655440000", false},
		{"underscore", "550e8400_e29b-41d4-a716-446655440000", false},
		{"special char", "550e8400-e29b-41d4-a716-44665544000!", false},

		// Edge cases
		{"all zeros no dashes wrong length", "00000000000000000000000000000000xxxx", false},
		{"uuid with braces", "{550e8400-e29b-41d4-a716-446655440000}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeUUID(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeSegment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Numeric segments -> :id
		{"numeric id", "123", ":id"},
		{"single digit", "5", ":id"},
		{"large number", "9999999999", ":id"},

		// UUID segments -> :uuid
		{"uuid", "550e8400-e29b-41d4-a716-446655440000", ":uuid"},
		{"uppercase uuid", "550E8400-E29B-41D4-A716-446655440000", ":uuid"},

		// Long tokens -> :token
		{"33 char string", "abcdefghijklmnopqrstuvwxyz1234567", ":token"},
		{"64 char string", "abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz12", ":token"},

		// Regular segments preserved
		{"api", "api", "api"},
		{"v1", "v1", "v1"},
		{"users", "users", "users"},
		{"settings", "settings", "settings"},
		{"exactly 32 chars", "abcdefghijklmnopqrstuvwxyz123456", "abcdefghijklmnopqrstuvwxyz123456"},

		// Edge cases
		{"empty string", "", ""},
		{"single letter", "a", "a"},
		{"mixed alphanumeric short", "user123", "user123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSegment(tt.input)
			if got != tt.want {
				t.Errorf("normalizeSegment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeRoute(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Root and empty paths
		{"empty string", "", "/"},
		{"root path", "/", "/"},
		{"root with trailing slash", "//", "/"},

		// Simple paths
		{"api path", "/api", "/api"},
		{"two segments", "/api/v1", "/api/v1"},
		{"three segments", "/api/v1/users", "/api/v1/users"},

		// Paths with numeric IDs
		{"path with numeric id", "/api/users/123", "/api/users/:id"},
		{"path with multiple ids", "/api/users/123/posts/456", "/api/users/:id/posts/:id"},

		// Paths with UUIDs
		{"path with uuid", "/api/users/550e8400-e29b-41d4-a716-446655440000", "/api/users/:uuid"},

		// Paths with long tokens
		{"path with long token", "/api/auth/abcdefghijklmnopqrstuvwxyz1234567", "/api/auth/:token"},

		// Query parameters stripped
		{"path with query params", "/api/users?page=1&limit=10", "/api/users"},
		{"path with id and query", "/api/users/123?include=posts", "/api/users/:id"},

		// Segment limit (max 5)
		{"exactly 5 segments", "/a/b/c/d/e", "/a/b/c/d/e"},
		{"6 segments truncated", "/a/b/c/d/e/f", "/a/b/c/d/e"},
		{"7 segments truncated", "/a/b/c/d/e/f/g", "/a/b/c/d/e"},

		// Mixed content
		{"complex path", "/api/v1/users/123/posts/550e8400-e29b-41d4-a716-446655440000", "/api/v1/users/:id/posts"},

		// Edge cases
		{"double slashes", "/api//users", "/api/users"},
		{"trailing slash", "/api/users/", "/api/users"},
		{"leading double slash", "//api/users", "/api/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRoute(tt.input)
			if got != tt.want {
				t.Errorf("normalizeRoute(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// recordAPIRequest integration tests — verify that the core recording
// function correctly populates all three Prometheus metric vectors.
// ---------------------------------------------------------------------------

func TestRecordAPIRequest_SuccessMetrics(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)

	route := "/test/record/success"
	status := "200"

	totalBefore := httpCounterValue(t, apiRequestTotal, "GET", route, status)
	durCountBefore := httpHistogramSampleCount(t, apiRequestDuration, "GET", route, status)

	recordAPIRequest("GET", route, 200, 42*time.Millisecond)

	if delta := httpCounterValue(t, apiRequestTotal, "GET", route, status) - totalBefore; delta != 1 {
		t.Errorf("requests_total delta: want 1, got %v", delta)
	}
	if delta := httpHistogramSampleCount(t, apiRequestDuration, "GET", route, status) - durCountBefore; delta != 1 {
		t.Errorf("request_duration_seconds observation count delta: want 1, got %v", delta)
	}
}

func TestRecordAPIRequest_ClientErrorCounter(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)

	route := "/test/record/client-err"
	errBefore := httpCounterValue(t, apiRequestErrors, "GET", route, "client_error")
	totalBefore := httpCounterValue(t, apiRequestTotal, "GET", route, "404")

	recordAPIRequest("GET", route, 404, 5*time.Millisecond)

	if delta := httpCounterValue(t, apiRequestErrors, "GET", route, "client_error") - errBefore; delta != 1 {
		t.Errorf("request_errors_total (client_error) delta: want 1, got %v", delta)
	}
	if delta := httpCounterValue(t, apiRequestTotal, "GET", route, "404") - totalBefore; delta != 1 {
		t.Errorf("requests_total (404) delta: want 1, got %v", delta)
	}
}

func TestRecordAPIRequest_ServerErrorCounter(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)

	route := "/test/record/server-err"
	errBefore := httpCounterValue(t, apiRequestErrors, "POST", route, "server_error")

	recordAPIRequest("POST", route, 500, 100*time.Millisecond)

	if delta := httpCounterValue(t, apiRequestErrors, "POST", route, "server_error") - errBefore; delta != 1 {
		t.Errorf("request_errors_total (server_error) delta: want 1, got %v", delta)
	}
}

func TestRecordAPIRequest_SuccessSkipsErrorCounter(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)

	route := "/test/record/no-error"
	clientBefore := httpCounterValue(t, apiRequestErrors, "GET", route, "client_error")
	serverBefore := httpCounterValue(t, apiRequestErrors, "GET", route, "server_error")

	recordAPIRequest("GET", route, 200, 5*time.Millisecond)
	recordAPIRequest("GET", route, 301, 3*time.Millisecond)

	if delta := httpCounterValue(t, apiRequestErrors, "GET", route, "client_error") - clientBefore; delta != 0 {
		t.Errorf("client_error counter changed for 2xx/3xx: delta=%v", delta)
	}
	if delta := httpCounterValue(t, apiRequestErrors, "GET", route, "server_error") - serverBefore; delta != 0 {
		t.Errorf("server_error counter changed for 2xx/3xx: delta=%v", delta)
	}
}

func TestRecordAPIRequest_DurationAccuracy(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)

	route := "/test/record/duration"
	status := "200"
	sumBefore := httpHistogramSum(t, apiRequestDuration, "GET", route, status)

	recordAPIRequest("GET", route, 200, 100*time.Millisecond)

	delta := httpHistogramSum(t, apiRequestDuration, "GET", route, status) - sumBefore
	// 100ms = 0.1s; allow small float tolerance.
	if delta < 0.099 || delta > 0.101 {
		t.Errorf("duration_seconds sum delta: want ~0.1, got %v", delta)
	}
}

func TestRecordAPIRequest_MultipleStatusCodes(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)

	route := "/test/record/multi-status"
	codes := []int{200, 201, 400, 401, 404, 500, 502, 503}

	for _, code := range codes {
		status := strconv.Itoa(code)
		before := httpCounterValue(t, apiRequestTotal, "GET", route, status)
		recordAPIRequest("GET", route, code, time.Millisecond)
		if delta := httpCounterValue(t, apiRequestTotal, "GET", route, status) - before; delta != 1 {
			t.Errorf("requests_total for status %d: want delta=1, got %v", code, delta)
		}
	}
}

func TestRecordDeprecatedAPIUsage(t *testing.T) {
	httpMetricsOnce.Do(initHTTPMetrics)

	feature := "host_agent_api_alias"
	route := "/api/agents/host/report"
	before := httpCounterValue(t, deprecatedAPIUsage, feature, route)

	recordDeprecatedAPIUsage(feature, route)

	if delta := httpCounterValue(t, deprecatedAPIUsage, feature, route) - before; delta != 1 {
		t.Errorf("deprecated_api_usage_total delta: want 1, got %v", delta)
	}
}

// ---------------------------------------------------------------------------
// Test helpers for reading Prometheus metric values (delta-safe pattern).
// ---------------------------------------------------------------------------

func httpCounterValue(t *testing.T, vec *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	counter, err := vec.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(%v): %v", labels, err)
	}
	m := &dto.Metric{}
	if err := counter.Write(m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Counter == nil {
		return 0
	}
	return m.Counter.GetValue()
}

func httpHistogramSampleCount(t *testing.T, vec *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	obs, err := vec.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(%v): %v", labels, err)
	}
	m := &dto.Metric{}
	if err := obs.(prometheus.Metric).Write(m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Histogram == nil {
		return 0
	}
	return m.Histogram.GetSampleCount()
}

func httpHistogramSum(t *testing.T, vec *prometheus.HistogramVec, labels ...string) float64 {
	t.Helper()
	obs, err := vec.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(%v): %v", labels, err)
	}
	m := &dto.Metric{}
	if err := obs.(prometheus.Metric).Write(m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Histogram == nil {
		return 0
	}
	return m.Histogram.GetSampleSum()
}
