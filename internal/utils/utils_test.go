package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	tests := []struct {
		prefix string
	}{
		{"test"},
		{"alert"},
		{"node"},
		{""},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			id := GenerateID(tc.prefix)

			// Should start with prefix
			if tc.prefix != "" && !strings.HasPrefix(id, tc.prefix+"-") {
				t.Errorf("GenerateID(%q) = %q, should start with %q-", tc.prefix, id, tc.prefix)
			}

			// Should be non-empty
			if id == "" {
				t.Error("GenerateID() returned empty string")
			}
		})
	}

	// IDs should be unique
	id1 := GenerateID("test")
	id2 := GenerateID("test")
	if id1 == id2 {
		t.Error("GenerateID() returned duplicate IDs")
	}
}

func TestWriteJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected string
	}{
		{
			name:     "simple object",
			data:     map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "array",
			data:     []int{1, 2, 3},
			expected: `[1,2,3]`,
		},
		{
			name:     "nested object",
			data:     map[string]interface{}{"outer": map[string]int{"inner": 42}},
			expected: `{"outer":{"inner":42}}`,
		},
		{
			name:     "empty object",
			data:     map[string]string{},
			expected: `{}`,
		},
		{
			name:     "null",
			data:     nil,
			expected: `null`,
		},
		{
			name: "struct",
			data: struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			}{Name: "test", Count: 5},
			expected: `{"name":"test","count":5}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			err := WriteJSONResponse(w, tc.data)
			if err != nil {
				t.Fatalf("WriteJSONResponse() error: %v", err)
			}

			// Check content type
			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			// Check body
			body := w.Body.String()
			if body != tc.expected {
				t.Errorf("Body = %q, want %q", body, tc.expected)
			}
		})
	}
}

func TestWriteJSONResponse_InvalidData(t *testing.T) {
	w := httptest.NewRecorder()

	// Channels cannot be marshaled to JSON
	ch := make(chan int)
	err := WriteJSONResponse(w, ch)
	if err == nil {
		t.Error("WriteJSONResponse() should fail on unmarshalable data")
	}
}

func TestWriteJSONResponse_StatusCode(t *testing.T) {
	w := httptest.NewRecorder()

	// Set status code before writing
	w.WriteHeader(http.StatusCreated)

	err := WriteJSONResponse(w, map[string]string{"status": "created"})
	if err != nil {
		t.Fatalf("WriteJSONResponse() error: %v", err)
	}

	if w.Code != http.StatusCreated {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Truthy values
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"Yes", true},
		{"y", true},
		{"Y", true},
		{"on", true},
		{"ON", true},
		{"On", true},

		// Falsy values
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"n", false},
		{"off", false},
		{"", false},
		{"random", false},
		{"2", false},

		// With whitespace
		{" true ", true},
		{" false ", false},
		{"\ttrue\n", true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseBool(tc.input)
			if result != tc.expected {
				t.Errorf("ParseBool(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGetenvTrim(t *testing.T) {
	// Set test environment variable
	testKey := "TEST_GETENVTRIM_VAR"

	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"no whitespace", "value", "value"},
		{"leading space", " value", "value"},
		{"trailing space", "value ", "value"},
		{"both sides", " value ", "value"},
		{"tabs", "\tvalue\t", "value"},
		{"newlines", "\nvalue\n", "value"},
		{"empty", "", ""},
		{"only whitespace", "   ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv(testKey, tc.value)
			defer os.Unsetenv(testKey)

			result := GetenvTrim(testKey)
			if result != tc.expected {
				t.Errorf("GetenvTrim(%q) with value %q = %q, want %q", testKey, tc.value, result, tc.expected)
			}
		})
	}

	// Test unset variable
	os.Unsetenv(testKey)
	result := GetenvTrim(testKey)
	if result != "" {
		t.Errorf("GetenvTrim() for unset var = %q, want empty string", result)
	}
}

func TestGetDataDir(t *testing.T) {
	envKey := "PULSE_DATA_DIR"
	originalValue := os.Getenv(envKey)
	defer func() {
		if originalValue != "" {
			os.Setenv(envKey, originalValue)
		} else {
			os.Unsetenv(envKey)
		}
	}()

	// Test with env var set
	os.Setenv(envKey, "/custom/data/dir")
	result := GetDataDir()
	if result != "/custom/data/dir" {
		t.Errorf("GetDataDir() with env = %q, want /custom/data/dir", result)
	}

	// Test with env var unset (default)
	os.Unsetenv(envKey)
	result = GetDataDir()
	if result != "/etc/pulse" {
		t.Errorf("GetDataDir() without env = %q, want /etc/pulse", result)
	}

	// Test with empty env var (should use default)
	os.Setenv(envKey, "")
	result = GetDataDir()
	if result != "/etc/pulse" {
		t.Errorf("GetDataDir() with empty env = %q, want /etc/pulse", result)
	}
}

func TestWriteJSONResponse_LargePayload(t *testing.T) {
	w := httptest.NewRecorder()

	// Create a large payload
	data := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		data[i] = map[string]interface{}{
			"index": i,
			"name":  strings.Repeat("x", 100),
		}
	}

	err := WriteJSONResponse(w, data)
	if err != nil {
		t.Fatalf("WriteJSONResponse() error on large payload: %v", err)
	}

	// Verify it's valid JSON
	var decoded []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}

	if len(decoded) != 1000 {
		t.Errorf("Decoded length = %d, want 1000", len(decoded))
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// With v prefix
		{"v4.33.1", "4.33.1"},
		{"v1.0.0", "1.0.0"},
		{"v0.0.1-rc1", "0.0.1-rc1"},

		// Without v prefix
		{"4.33.1", "4.33.1"},
		{"1.0.0", "1.0.0"},
		{"0.0.1-rc1", "0.0.1-rc1"},

		// With whitespace
		{" v4.33.1", "4.33.1"},
		{"v4.33.1 ", "4.33.1"},
		{" v4.33.1 ", "4.33.1"},
		{"\tv4.33.1\n", "4.33.1"},

		// Edge cases
		{"", ""},
		{"v", ""},
		{" ", ""},
		{"vv4.33.1", "v4.33.1"}, // Only removes one v
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeVersion(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		// Equal versions
		{"4.33.1", "4.33.1", 0},
		{"v4.33.1", "4.33.1", 0},
		{"4.33.1", "v4.33.1", 0},
		{"1.0.0", "1.0.0", 0},

		// a > b (a is newer)
		{"4.33.2", "4.33.1", 1},
		{"4.34.0", "4.33.1", 1},
		{"5.0.0", "4.33.1", 1},
		{"4.33.10", "4.33.9", 1},
		{"4.33.1", "4.33", 1}, // Missing patch = 0

		// a < b (b is newer)
		{"4.33.1", "4.33.2", -1},
		{"4.33.1", "4.34.0", -1},
		{"4.33.1", "5.0.0", -1},
		{"4.33.9", "4.33.10", -1},
		{"4.33", "4.33.1", -1},

		// With v prefix
		{"v4.34.0", "v4.33.1", 1},
		{"v4.33.1", "v4.34.0", -1},

		// Edge cases
		{"0.0.1", "0.0.0", 1},
		{"0.0.0", "0.0.1", -1},
		{"1.0", "0.9.9", 1},
	}

	for _, tc := range tests {
		name := tc.a + "_vs_" + tc.b
		t.Run(name, func(t *testing.T) {
			result := CompareVersions(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}
