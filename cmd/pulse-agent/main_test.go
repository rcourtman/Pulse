package main

import (
	"reflect"
	"testing"

	"github.com/rs/zerolog"
)

func TestGatherTags(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		flags    []string
		expected []string
	}{
		// Empty inputs
		{
			name:     "empty env and flags returns empty slice",
			env:      "",
			flags:    nil,
			expected: []string{},
		},
		{
			name:     "empty env and empty flags returns empty slice",
			env:      "",
			flags:    []string{},
			expected: []string{},
		},

		// Environment only
		{
			name:     "single env tag",
			env:      "prod",
			flags:    nil,
			expected: []string{"prod"},
		},
		{
			name:     "multiple env tags comma separated",
			env:      "prod,us-west",
			flags:    nil,
			expected: []string{"prod", "us-west"},
		},
		{
			name:     "env tags with whitespace trimmed",
			env:      " prod , us-west ",
			flags:    nil,
			expected: []string{"prod", "us-west"},
		},
		{
			name:     "env empty items filtered",
			env:      "prod,,us-west,",
			flags:    nil,
			expected: []string{"prod", "us-west"},
		},
		{
			name:     "env whitespace-only items filtered",
			env:      "prod,   ,us-west",
			flags:    nil,
			expected: []string{"prod", "us-west"},
		},

		// Flags only
		{
			name:     "single flag tag",
			env:      "",
			flags:    []string{"staging"},
			expected: []string{"staging"},
		},
		{
			name:     "multiple flag tags",
			env:      "",
			flags:    []string{"staging", "eu-central"},
			expected: []string{"staging", "eu-central"},
		},
		{
			name:     "flag tags with whitespace trimmed",
			env:      "",
			flags:    []string{" staging ", " eu-central "},
			expected: []string{"staging", "eu-central"},
		},
		{
			name:     "flag empty items filtered",
			env:      "",
			flags:    []string{"staging", "", "eu-central"},
			expected: []string{"staging", "eu-central"},
		},
		{
			name:     "flag whitespace-only items filtered",
			env:      "",
			flags:    []string{"staging", "   ", "eu-central"},
			expected: []string{"staging", "eu-central"},
		},

		// Both env and flags (env first, then flags)
		{
			name:     "env tags come before flags",
			env:      "prod",
			flags:    []string{"app1"},
			expected: []string{"prod", "app1"},
		},
		{
			name:     "multiple env and multiple flags",
			env:      "prod,us-west",
			flags:    []string{"app1", "critical"},
			expected: []string{"prod", "us-west", "app1", "critical"},
		},
		{
			name:     "duplicates preserved (no dedup)",
			env:      "prod,prod",
			flags:    []string{"prod"},
			expected: []string{"prod", "prod", "prod"},
		},

		// Edge cases
		{
			name:     "only commas in env",
			env:      ",,,",
			flags:    nil,
			expected: []string{},
		},
		{
			name:     "single comma",
			env:      ",",
			flags:    nil,
			expected: []string{},
		},
		{
			name:     "env with tabs",
			env:      "\tprod\t,\tstaging\t",
			flags:    nil,
			expected: []string{"prod", "staging"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gatherTags(tt.env, tt.flags)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestGatherCSV(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		flags    []string
		expected []string
	}{
		{"empty", "", nil, []string{}},
		{"env only", "a,b", nil, []string{"a", "b"}},
		{"env trims", " a , b ", nil, []string{"a", "b"}},
		{"flags only", "", []string{"x", " y "}, []string{"x", "y"}},
		{"both", "a", []string{"b"}, []string{"a", "b"}},
		{"filters empties", "a,,", []string{"", "b", "  "}, []string{"a", "b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := gatherCSV(tc.env, tc.flags)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestDefaultInt(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback int
		expected int
	}{
		{"empty uses fallback", "", 5, 5},
		{"whitespace uses fallback", "   ", 5, 5},
		{"valid int", "12", 5, 12},
		{"invalid uses fallback", "nope", 5, 5},
		{"leading whitespace", " 7", 5, 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := defaultInt(tc.value, tc.fallback)
			if got != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLevel zerolog.Level
		wantErr   bool
	}{
		// Valid levels
		{
			name:      "debug level",
			input:     "debug",
			wantLevel: zerolog.DebugLevel,
		},
		{
			name:      "info level",
			input:     "info",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "warn level",
			input:     "warn",
			wantLevel: zerolog.WarnLevel,
		},
		{
			name:      "error level",
			input:     "error",
			wantLevel: zerolog.ErrorLevel,
		},
		{
			name:      "trace level (accepted in unified agent)",
			input:     "trace",
			wantLevel: zerolog.TraceLevel,
		},
		{
			name:      "fatal level (accepted in unified agent)",
			input:     "fatal",
			wantLevel: zerolog.FatalLevel,
		},
		{
			name:      "panic level (accepted in unified agent)",
			input:     "panic",
			wantLevel: zerolog.PanicLevel,
		},

		// Case insensitivity
		{
			name:      "uppercase DEBUG",
			input:     "DEBUG",
			wantLevel: zerolog.DebugLevel,
		},
		{
			name:      "mixed case Info",
			input:     "Info",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "uppercase WARN",
			input:     "WARN",
			wantLevel: zerolog.WarnLevel,
		},
		{
			name:      "uppercase ERROR",
			input:     "ERROR",
			wantLevel: zerolog.ErrorLevel,
		},
		{
			name:      "uppercase TRACE",
			input:     "TRACE",
			wantLevel: zerolog.TraceLevel,
		},

		// Whitespace handling
		{
			name:      "leading whitespace",
			input:     "  debug",
			wantLevel: zerolog.DebugLevel,
		},
		{
			name:      "trailing whitespace",
			input:     "warn  ",
			wantLevel: zerolog.WarnLevel,
		},
		{
			name:      "both whitespace",
			input:     "  error  ",
			wantLevel: zerolog.ErrorLevel,
		},
		{
			name:      "tabs",
			input:     "\tinfo\t",
			wantLevel: zerolog.InfoLevel,
		},

		// Empty string defaults to info
		{
			name:      "empty string defaults to info",
			input:     "",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "whitespace only defaults to info",
			input:     "   ",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "tabs only defaults to info",
			input:     "\t\t",
			wantLevel: zerolog.InfoLevel,
		},

		// Numeric levels (zerolog supports these)
		{
			name:      "numeric -1 maps to trace level",
			input:     "-1",
			wantLevel: zerolog.TraceLevel,
		},
		{
			name:      "numeric 0 maps to debug level",
			input:     "0",
			wantLevel: zerolog.DebugLevel,
		},
		{
			name:      "numeric 1 maps to info level",
			input:     "1",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "numeric 2 maps to warn level",
			input:     "2",
			wantLevel: zerolog.WarnLevel,
		},
		{
			name:      "numeric 3 maps to error level",
			input:     "3",
			wantLevel: zerolog.ErrorLevel,
		},
		{
			name:      "numeric 4 maps to fatal level",
			input:     "4",
			wantLevel: zerolog.FatalLevel,
		},
		{
			name:      "numeric 5 maps to panic level",
			input:     "5",
			wantLevel: zerolog.PanicLevel,
		},

		// Invalid levels
		{
			name:      "invalid level returns error",
			input:     "invalid",
			wantLevel: zerolog.NoLevel, // zerolog.ParseLevel returns NoLevel on error
			wantErr:   true,
		},
		{
			name:      "typo returns error",
			input:     "debuf",
			wantLevel: zerolog.NoLevel,
			wantErr:   true,
		},
		{
			name:      "verbose returns error",
			input:     "verbose",
			wantLevel: zerolog.NoLevel,
			wantErr:   true,
		},
		{
			name:      "numeric out of range accepted (zerolog accepts any int)",
			input:     "99",
			wantLevel: zerolog.Level(99),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := parseLogLevel(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if level != tt.wantLevel {
				t.Fatalf("expected level %v, got %v", tt.wantLevel, level)
			}
		})
	}
}

func TestDefaultLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		// Empty returns "info"
		{
			name:     "empty string returns info",
			envValue: "",
			expected: "info",
		},
		{
			name:     "whitespace only returns info",
			envValue: "   ",
			expected: "info",
		},
		{
			name:     "tabs only returns info",
			envValue: "\t\t",
			expected: "info",
		},
		{
			name:     "newline only returns info",
			envValue: "\n",
			expected: "info",
		},

		// Non-empty returns as-is (no validation)
		{
			name:     "debug returns debug",
			envValue: "debug",
			expected: "debug",
		},
		{
			name:     "error returns error",
			envValue: "error",
			expected: "error",
		},
		{
			name:     "trace returns trace",
			envValue: "trace",
			expected: "trace",
		},
		{
			name:     "invalid value passed through",
			envValue: "invalid",
			expected: "invalid",
		},
		{
			name:     "mixed case passed through",
			envValue: "DEBUG",
			expected: "DEBUG",
		},
		{
			name:     "value with surrounding whitespace NOT trimmed",
			envValue: "  debug  ",
			expected: "  debug  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultLogLevel(tt.envValue)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestMultiValue(t *testing.T) {
	t.Run("String joins with comma", func(t *testing.T) {
		mv := multiValue{"a", "b", "c"}
		if got := mv.String(); got != "a,b,c" {
			t.Fatalf("expected %q, got %q", "a,b,c", got)
		}
	})

	t.Run("String empty slice returns empty string", func(t *testing.T) {
		mv := multiValue{}
		if got := mv.String(); got != "" {
			t.Fatalf("expected %q, got %q", "", got)
		}
	})

	t.Run("String single item no comma", func(t *testing.T) {
		mv := multiValue{"single"}
		if got := mv.String(); got != "single" {
			t.Fatalf("expected %q, got %q", "single", got)
		}
	})

	t.Run("Set appends values", func(t *testing.T) {
		mv := multiValue{}
		if err := mv.Set("first"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mv.Set("second"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mv.Set("third"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := multiValue{"first", "second", "third"}
		if !reflect.DeepEqual(mv, expected) {
			t.Fatalf("expected %v, got %v", expected, mv)
		}
	})

	t.Run("Set preserves empty strings", func(t *testing.T) {
		mv := multiValue{}
		_ = mv.Set("")
		_ = mv.Set("value")
		_ = mv.Set("")

		if len(mv) != 3 {
			t.Fatalf("expected 3 items, got %d", len(mv))
		}
	})

	t.Run("Set always returns nil error", func(t *testing.T) {
		mv := multiValue{}
		// Set always returns nil, testing various inputs
		inputs := []string{"", "normal", "with spaces", "special!@#$%", "unicode日本語"}
		for _, input := range inputs {
			if err := mv.Set(input); err != nil {
				t.Fatalf("expected nil error for input %q, got %v", input, err)
			}
		}
	})
}
