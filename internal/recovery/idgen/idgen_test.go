package idgen

import (
	"testing"
	"time"
)

func TestStableID(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		wantSame bool
	}{
		{
			name:     "nil variadic",
			parts:    nil,
			wantSame: true,
		},
		{
			name:     "empty variadic",
			parts:    []string{},
			wantSame: true,
		},
		{
			name:     "single part",
			parts:    []string{"foo"},
			wantSame: true,
		},
		{
			name:     "multiple parts",
			parts:    []string{"foo", "bar", "baz"},
			wantSame: true,
		},
		{
			name:     "whitespace trimmed",
			parts:    []string{"  foo  "},
			wantSame: true,
		},
		{
			name:     "empty string in middle",
			parts:    []string{"foo", "", "bar"},
			wantSame: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id1 := StableID(tc.parts...)
			id2 := StableID(tc.parts...)
			if id1 != id2 {
				t.Errorf("StableID() = %v, want same for identical inputs", id1)
			}
		})
	}
}

func TestStableID_DifferentInputs_DifferentOutputs(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
	}{
		{"single foo", []string{"foo"}},
		{"single bar", []string{"bar"}},
		{"foo bar", []string{"foo", "bar"}},
		{"foo bar baz", []string{"foo", "bar", "baz"}},
		{"bar foo", []string{"bar", "foo"}},
		{"empty", []string{}},
	}

	for i, tc1 := range tests {
		for j, tc2 := range tests {
			if i == j {
				continue
			}
			id1 := StableID(tc1.parts...)
			id2 := StableID(tc2.parts...)
			if id1 == id2 {
				t.Errorf("StableID(%v) = StableID(%v) = %v, want different", tc1.parts, tc2.parts, id1)
			}
		}
	}
}

func TestStableID_WhitespaceTrimmed(t *testing.T) {
	// " foo " should equal "foo"
	id1 := StableID(" foo ")
	id2 := StableID("foo")
	if id1 != id2 {
		t.Errorf("StableID(\" foo \") = %q, want %q (StableID(\"foo\"))", id1, id2)
	}
}

func TestTimeKey(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name     string
		primary  *time.Time
		fallback *time.Time
		expected string
	}{
		{
			name:     "both nil",
			primary:  nil,
			fallback: nil,
			expected: "",
		},
		{
			name:     "primary non-nil",
			primary:  &now,
			fallback: nil,
			expected: now.UTC().Format(time.RFC3339Nano),
		},
		{
			name:     "primary zero time",
			primary:  &time.Time{},
			fallback: &later,
			expected: later.UTC().Format(time.RFC3339Nano),
		},
		{
			name:     "primary nil fallback non-nil",
			primary:  nil,
			fallback: &now,
			expected: now.UTC().Format(time.RFC3339Nano),
		},
		{
			name:     "primary nil fallback zero time",
			primary:  nil,
			fallback: &time.Time{},
			expected: "",
		},
		{
			name:     "both zero time",
			primary:  &time.Time{},
			fallback: &time.Time{},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := TimeKey(tc.primary, tc.fallback)
			if result != tc.expected {
				t.Errorf("TimeKey() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestTimeKey_UTC(t *testing.T) {
	// Test that output is always UTC
	loc, _ := time.LoadLocation("America/New_York")
	nyTime := time.Date(2024, 1, 15, 12, 0, 0, 0, loc)

	result := TimeKey(&nyTime, nil)
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Parse and check it's in UTC
	parsed, err := time.Parse(time.RFC3339Nano, result)
	if err != nil {
		t.Fatalf("failed to parse RFC3339Nano: %v", err)
	}

	// The hour should be 17 (12 EST = 17 UTC)
	if parsed.Hour() != 17 {
		t.Errorf("expected UTC hour 17, got %d", parsed.Hour())
	}
}

func TestPtrTime(t *testing.T) {
	tests := []struct {
		name      string
		input     time.Time
		expectNil bool
	}{
		{
			name:      "zero time",
			input:     time.Time{},
			expectNil: true,
		},
		{
			name:      "non-zero time",
			input:     time.Now(),
			expectNil: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := PtrTime(tc.input)
			if tc.expectNil {
				if result != nil {
					t.Errorf("PtrTime() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Error("PtrTime() = nil, want non-nil")
					return
				}
				// Result should always be UTC
				if !result.UTC().Equal(*result) {
					t.Errorf("PtrTime() not UTC: %v", *result)
				}
			}
		})
	}
}

func TestPtrTime_AlwaysUTC(t *testing.T) {
	// Test that result is always UTC regardless of input timezone
	loc, _ := time.LoadLocation("Asia/Tokyo")
	tokyoTime := time.Date(2024, 1, 15, 12, 0, 0, 0, loc)

	result := PtrTime(tokyoTime)
	if result == nil {
		t.Fatal("expected non-nil for non-zero time")
	}

	// The UTC hour should be 3 (12 JST = 3 UTC)
	if result.Hour() != 3 {
		t.Errorf("expected UTC hour 3, got %d", result.Hour())
	}
}
