package pmg

import (
	"encoding/json"
	"math"
	"testing"
)

func TestFlexibleFloat_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
		wantErr  bool
	}{
		// JSON number values
		{
			name:     "integer as number",
			input:    `42`,
			expected: 42.0,
		},
		{
			name:     "float as number",
			input:    `3.14159`,
			expected: 3.14159,
		},
		{
			name:     "negative number",
			input:    `-273.15`,
			expected: -273.15,
		},
		{
			name:     "zero as number",
			input:    `0`,
			expected: 0,
		},
		{
			name:     "scientific notation",
			input:    `1.5e10`,
			expected: 1.5e10,
		},
		{
			name:     "negative exponent",
			input:    `1.5e-3`,
			expected: 0.0015,
		},

		// String values
		{
			name:     "integer as string",
			input:    `"100"`,
			expected: 100.0,
		},
		{
			name:     "float as string",
			input:    `"2.718"`,
			expected: 2.718,
		},
		{
			name:     "negative as string",
			input:    `"-42.5"`,
			expected: -42.5,
		},
		{
			name:     "zero as string",
			input:    `"0"`,
			expected: 0,
		},
		{
			name:     "string with whitespace",
			input:    `"  123.45  "`,
			expected: 123.45,
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: 0,
		},
		{
			name:     "string null",
			input:    `"null"`,
			expected: 0,
		},
		{
			name:     "string NULL uppercase",
			input:    `"NULL"`,
			expected: 0,
		},

		// Null values
		{
			name:     "json null",
			input:    `null`,
			expected: 0,
		},

		// Edge cases
		{
			name:     "very large number",
			input:    `999999999999999`,
			expected: 999999999999999,
		},
		{
			name:     "very small number",
			input:    `0.000000001`,
			expected: 0.000000001,
		},

		// Error cases
		{
			name:    "invalid string",
			input:   `"not a number"`,
			wantErr: true,
		},
		{
			name:    "array",
			input:   `[1, 2, 3]`,
			wantErr: true,
		},
		{
			name:    "object",
			input:   `{"value": 42}`,
			wantErr: true,
		},
		{
			name:    "boolean true",
			input:   `true`,
			wantErr: true,
		},
		{
			name:    "boolean false",
			input:   `false`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var f flexibleFloat
			err := json.Unmarshal([]byte(tc.input), &f)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %s, got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input %s: %v", tc.input, err)
				return
			}

			if math.Abs(f.Float64()-tc.expected) > 1e-9 {
				t.Errorf("flexibleFloat = %v, want %v", f.Float64(), tc.expected)
			}
		})
	}
}

func TestFlexibleFloat_Float64Method(t *testing.T) {
	f := flexibleFloat(3.14)
	if f.Float64() != 3.14 {
		t.Errorf("Float64() = %v, want 3.14", f.Float64())
	}
}

func TestFlexibleFloat_EmptyInput(t *testing.T) {
	var f flexibleFloat
	// Empty byte slice should default to 0
	err := f.UnmarshalJSON([]byte{})
	if err != nil {
		t.Errorf("unexpected error for empty input: %v", err)
	}
	if f != 0 {
		t.Errorf("expected 0 for empty input, got %v", f)
	}
}

func TestFlexibleFloat_WhitespaceOnly(t *testing.T) {
	var f flexibleFloat
	// Whitespace around null
	err := f.UnmarshalJSON([]byte("  null  "))
	if err != nil {
		t.Errorf("unexpected error for whitespace-padded null: %v", err)
	}
	if f != 0 {
		t.Errorf("expected 0 for whitespace-padded null, got %v", f)
	}
}

func TestFlexibleInt_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		// JSON integer values
		{
			name:     "positive integer",
			input:    `42`,
			expected: 42,
		},
		{
			name:     "negative integer",
			input:    `-100`,
			expected: -100,
		},
		{
			name:     "zero",
			input:    `0`,
			expected: 0,
		},
		{
			name:     "large integer",
			input:    `9223372036854775807`,
			expected: 9223372036854775807,
		},

		// Float values (truncated to int)
		{
			name:     "float truncated",
			input:    `42.9`,
			expected: 42,
		},
		{
			name:     "negative float truncated",
			input:    `-42.9`,
			expected: -42,
		},
		{
			name:     "float with .0",
			input:    `100.0`,
			expected: 100,
		},

		// String values
		{
			name:     "integer as string",
			input:    `"256"`,
			expected: 256,
		},
		{
			name:     "negative integer as string",
			input:    `"-512"`,
			expected: -512,
		},
		{
			name:     "float string truncated",
			input:    `"99.9"`,
			expected: 99,
		},
		{
			name:     "zero as string",
			input:    `"0"`,
			expected: 0,
		},
		{
			name:     "string with whitespace",
			input:    `"  1024  "`,
			expected: 1024,
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: 0,
		},
		{
			name:     "string null",
			input:    `"null"`,
			expected: 0,
		},
		{
			name:     "string NULL uppercase",
			input:    `"NULL"`,
			expected: 0,
		},

		// Null values
		{
			name:     "json null",
			input:    `null`,
			expected: 0,
		},

		// Error cases
		{
			name:    "invalid string",
			input:   `"not a number"`,
			wantErr: true,
		},
		{
			name:    "array",
			input:   `[1, 2, 3]`,
			wantErr: true,
		},
		{
			name:    "object",
			input:   `{"value": 42}`,
			wantErr: true,
		},
		{
			name:    "boolean true",
			input:   `true`,
			wantErr: true,
		},
		{
			name:    "boolean false",
			input:   `false`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var i flexibleInt
			err := json.Unmarshal([]byte(tc.input), &i)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %s, got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input %s: %v", tc.input, err)
				return
			}

			if i.Int64() != tc.expected {
				t.Errorf("flexibleInt = %v, want %v", i.Int64(), tc.expected)
			}
		})
	}
}

func TestFlexibleInt_Methods(t *testing.T) {
	i := flexibleInt(12345)

	if i.Int64() != 12345 {
		t.Errorf("Int64() = %v, want 12345", i.Int64())
	}

	if i.Int() != 12345 {
		t.Errorf("Int() = %v, want 12345", i.Int())
	}
}

func TestFlexibleInt_EmptyInput(t *testing.T) {
	var i flexibleInt
	// Empty byte slice should default to 0
	err := i.UnmarshalJSON([]byte{})
	if err != nil {
		t.Errorf("unexpected error for empty input: %v", err)
	}
	if i != 0 {
		t.Errorf("expected 0 for empty input, got %v", i)
	}
}

func TestFlexibleInt_WhitespaceOnly(t *testing.T) {
	var i flexibleInt
	// Whitespace around null
	err := i.UnmarshalJSON([]byte("  null  "))
	if err != nil {
		t.Errorf("unexpected error for whitespace-padded null: %v", err)
	}
	if i != 0 {
		t.Errorf("expected 0 for whitespace-padded null, got %v", i)
	}
}

// Test struct unmarshaling to ensure types work in real JSON structures
func TestFlexibleTypes_InStruct(t *testing.T) {
	type TestData struct {
		FloatVal flexibleFloat `json:"float_val"`
		IntVal   flexibleInt   `json:"int_val"`
	}

	tests := []struct {
		name          string
		input         string
		expectedFloat float64
		expectedInt   int64
	}{
		{
			name:          "numbers",
			input:         `{"float_val": 3.14, "int_val": 42}`,
			expectedFloat: 3.14,
			expectedInt:   42,
		},
		{
			name:          "strings",
			input:         `{"float_val": "2.718", "int_val": "100"}`,
			expectedFloat: 2.718,
			expectedInt:   100,
		},
		{
			name:          "nulls",
			input:         `{"float_val": null, "int_val": null}`,
			expectedFloat: 0,
			expectedInt:   0,
		},
		{
			name:          "mixed",
			input:         `{"float_val": "1.5", "int_val": 99}`,
			expectedFloat: 1.5,
			expectedInt:   99,
		},
		{
			name:          "missing fields",
			input:         `{}`,
			expectedFloat: 0,
			expectedInt:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data TestData
			err := json.Unmarshal([]byte(tc.input), &data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(data.FloatVal.Float64()-tc.expectedFloat) > 1e-9 {
				t.Errorf("FloatVal = %v, want %v", data.FloatVal.Float64(), tc.expectedFloat)
			}

			if data.IntVal.Int64() != tc.expectedInt {
				t.Errorf("IntVal = %v, want %v", data.IntVal.Int64(), tc.expectedInt)
			}
		})
	}
}

// Test that PMG API response patterns work correctly
func TestFlexibleTypes_PMGResponsePatterns(t *testing.T) {
	// Simulates real PMG API response patterns observed in production
	type MailStats struct {
		Count     flexibleFloat `json:"count"`
		CountIn   flexibleFloat `json:"count_in"`
		SpamCount flexibleFloat `json:"spamcount_in"`
		BouncesIn flexibleFloat `json:"bounces_in"`
		BytesIn   flexibleFloat `json:"bytes_in"`
		AvgTime   flexibleFloat `json:"avptime"`
	}

	input := `{
		"count": null,
		"count_in": "25",
		"spamcount_in": 7,
		"bounces_in": "",
		"bytes_in": "1024",
		"avptime": "0.75"
	}`

	var stats MailStats
	err := json.Unmarshal([]byte(input), &stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Count.Float64() != 0 {
		t.Errorf("Count (null) = %v, want 0", stats.Count.Float64())
	}
	if stats.CountIn.Float64() != 25 {
		t.Errorf("CountIn (string) = %v, want 25", stats.CountIn.Float64())
	}
	if stats.SpamCount.Float64() != 7 {
		t.Errorf("SpamCount (number) = %v, want 7", stats.SpamCount.Float64())
	}
	if stats.BouncesIn.Float64() != 0 {
		t.Errorf("BouncesIn (empty string) = %v, want 0", stats.BouncesIn.Float64())
	}
	if stats.BytesIn.Float64() != 1024 {
		t.Errorf("BytesIn (string) = %v, want 1024", stats.BytesIn.Float64())
	}
	if stats.AvgTime.Float64() != 0.75 {
		t.Errorf("AvgTime (string float) = %v, want 0.75", stats.AvgTime.Float64())
	}
}

// Test queue status pattern with flexibleInt
func TestFlexibleInt_QueueStatusPattern(t *testing.T) {
	type QueueStatus struct {
		Active    flexibleInt `json:"active"`
		Deferred  flexibleInt `json:"deferred"`
		Hold      flexibleInt `json:"hold"`
		Incoming  flexibleInt `json:"incoming"`
		OldestAge flexibleInt `json:"oldest_age"`
	}

	input := `{
		"active": "3",
		"deferred": null,
		"hold": 1,
		"incoming": "2",
		"oldest_age": "600"
	}`

	var queue QueueStatus
	err := json.Unmarshal([]byte(input), &queue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if queue.Active.Int() != 3 {
		t.Errorf("Active (string) = %v, want 3", queue.Active.Int())
	}
	if queue.Deferred.Int() != 0 {
		t.Errorf("Deferred (null) = %v, want 0", queue.Deferred.Int())
	}
	if queue.Hold.Int() != 1 {
		t.Errorf("Hold (number) = %v, want 1", queue.Hold.Int())
	}
	if queue.Incoming.Int() != 2 {
		t.Errorf("Incoming (string) = %v, want 2", queue.Incoming.Int())
	}
	if queue.OldestAge.Int64() != 600 {
		t.Errorf("OldestAge (string) = %v, want 600", queue.OldestAge.Int64())
	}
}

// Test boundary values
func TestFlexibleFloat_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "max float64 approximation",
			input:    `1.7976931348623157e+308`,
			expected: 1.7976931348623157e+308,
		},
		{
			name:     "min positive float64 approximation",
			input:    `2.2250738585072014e-308`,
			expected: 2.2250738585072014e-308,
		},
		{
			name:     "negative max",
			input:    `-1.7976931348623157e+308`,
			expected: -1.7976931348623157e+308,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var f flexibleFloat
			err := json.Unmarshal([]byte(tc.input), &f)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Use relative comparison for very large/small numbers
			if tc.expected != 0 {
				relDiff := math.Abs((f.Float64() - tc.expected) / tc.expected)
				if relDiff > 1e-15 {
					t.Errorf("flexibleFloat = %v, want %v (relative diff: %v)", f.Float64(), tc.expected, relDiff)
				}
			}
		})
	}
}

// Test that string parsing handles various formats
func TestFlexibleFloat_StringFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "leading zeros",
			input:    `"007"`,
			expected: 7,
		},
		{
			name:     "trailing zeros",
			input:    `"42.00"`,
			expected: 42,
		},
		{
			name:     "decimal only",
			input:    `".5"`,
			expected: 0.5,
		},
		{
			name:     "integer with decimal point",
			input:    `"42."`,
			expected: 42,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var f flexibleFloat
			err := json.Unmarshal([]byte(tc.input), &f)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(f.Float64()-tc.expected) > 1e-9 {
				t.Errorf("flexibleFloat = %v, want %v", f.Float64(), tc.expected)
			}
		})
	}
}
