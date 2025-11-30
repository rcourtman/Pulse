package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rs/zerolog"
)

func TestParseTargetSpec(t *testing.T) {
	target, err := parseTargetSpec("https://pulse.example.com|abc123|true")
	if err != nil {
		t.Fatalf("parseTargetSpec returned error: %v", err)
	}

	if target.URL != "https://pulse.example.com" {
		t.Fatalf("expected URL https://pulse.example.com, got %q", target.URL)
	}
	if target.Token != "abc123" {
		t.Fatalf("expected token abc123, got %q", target.Token)
	}
	if !target.InsecureSkipVerify {
		t.Fatalf("expected insecure flag true")
	}
}

func TestParseTargetSpecDefaults(t *testing.T) {
	target, err := parseTargetSpec(" https://pulse.example.com | token456 ")
	if err != nil {
		t.Fatalf("parseTargetSpec returned error: %v", err)
	}

	if target.URL != "https://pulse.example.com" {
		t.Fatalf("expected URL https://pulse.example.com, got %q", target.URL)
	}
	if target.Token != "token456" {
		t.Fatalf("expected token token456, got %q", target.Token)
	}
	if target.InsecureSkipVerify {
		t.Fatalf("expected insecure flag false")
	}
}

func TestParseTargetSpecInvalid(t *testing.T) {
	if _, err := parseTargetSpec("https://pulse.example.com"); err == nil {
		t.Fatalf("expected error for missing token")
	}
	if _, err := parseTargetSpec("https://pulse.example.com|token|maybe"); err == nil {
		t.Fatalf("expected error for invalid insecure flag")
	}
}

func TestParseTargetSpecsSkipsBlanks(t *testing.T) {
	specs, err := parseTargetSpecs([]string{"https://a|tokenA", "   ", "\n", "https://b|tokenB|true"})
	if err != nil {
		t.Fatalf("parseTargetSpecs returned error: %v", err)
	}

	if len(specs) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(specs))
	}

	expected := []dockeragent.TargetConfig{
		{URL: "https://a", Token: "tokenA", InsecureSkipVerify: false},
		{URL: "https://b", Token: "tokenB", InsecureSkipVerify: true},
	}

	for i, target := range specs {
		if target != expected[i] {
			t.Fatalf("target %d mismatch: expected %+v, got %+v", i, expected[i], target)
		}
	}
}

func TestSplitTargetSpecs(t *testing.T) {
	values := splitTargetSpecs("https://a|tokenA;https://b|tokenB\nhttps://c|tokenC")
	expected := []string{"https://a|tokenA", "https://b|tokenB", "https://c|tokenC"}

	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(values))
	}

	for i, v := range values {
		if v != expected[i] {
			t.Fatalf("value %d mismatch: expected %q, got %q", i, expected[i], v)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLevel zerolog.Level
		wantErr   bool
		errSubstr string
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
			name:      "trace level",
			input:     "trace",
			wantLevel: zerolog.TraceLevel,
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

		// Invalid levels
		{
			name:      "invalid level returns error",
			input:     "invalid",
			wantLevel: zerolog.InfoLevel,
			wantErr:   true,
			errSubstr: "invalid log level",
		},
		{
			name:      "typo returns error",
			input:     "debuf",
			wantLevel: zerolog.InfoLevel,
			wantErr:   true,
			errSubstr: "must be debug, info, warn, or error",
		},
		{
			name:      "numeric 1 maps to info level",
			input:     "1",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "numeric 0 maps to debug level",
			input:     "0",
			wantLevel: zerolog.DebugLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := parseLogLevel(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
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

func TestSplitStringList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		// Empty input
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},

		// Single item
		{
			name:  "single item",
			input: "foo",
			want:  []string{"foo"},
		},
		{
			name:  "single item with whitespace",
			input: "  foo  ",
			want:  []string{"foo"},
		},

		// Comma delimiter
		{
			name:  "comma separated",
			input: "foo,bar,baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "comma with spaces",
			input: "foo, bar, baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "comma with extra spaces",
			input: "  foo  ,  bar  ,  baz  ",
			want:  []string{"foo", "bar", "baz"},
		},

		// Semicolon delimiter
		{
			name:  "semicolon separated",
			input: "foo;bar;baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "semicolon with spaces",
			input: "foo; bar; baz",
			want:  []string{"foo", "bar", "baz"},
		},

		// Newline delimiter
		{
			name:  "newline separated",
			input: "foo\nbar\nbaz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "newline with spaces",
			input: "foo \n bar \n baz",
			want:  []string{"foo", "bar", "baz"},
		},

		// Carriage return delimiter
		{
			name:  "carriage return separated",
			input: "foo\rbar\rbaz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "CRLF (Windows line ending)",
			input: "foo\r\nbar\r\nbaz",
			want:  []string{"foo", "bar", "baz"},
		},

		// Mixed delimiters
		{
			name:  "mixed comma and semicolon",
			input: "foo,bar;baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "mixed all delimiters",
			input: "a,b;c\nd\re",
			want:  []string{"a", "b", "c", "d", "e"},
		},
		{
			name:  "mixed with spaces",
			input: "a , b ; c \n d \r e",
			want:  []string{"a", "b", "c", "d", "e"},
		},

		// Consecutive delimiters (should be filtered)
		{
			name:  "double comma",
			input: "foo,,bar",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "multiple consecutive delimiters",
			input: "foo,,,bar;;;baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "trailing delimiter",
			input: "foo,bar,",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "leading delimiter",
			input: ",foo,bar",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "only delimiters returns empty slice",
			input: ",;,;",
			want:  []string{},
		},

		// Whitespace-only items filtered
		{
			name:  "whitespace between delimiters filtered",
			input: "foo,   ,bar",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "tabs between delimiters filtered",
			input: "foo,\t\t,bar",
			want:  []string{"foo", "bar"},
		},

		// Real-world examples
		{
			name:  "network names list",
			input: "bridge, host, none",
			want:  []string{"bridge", "host", "none"},
		},
		{
			name:  "container IDs",
			input: "abc123;def456;ghi789",
			want:  []string{"abc123", "def456", "ghi789"},
		},
		{
			name:  "multiline config",
			input: "web\napi\nworker",
			want:  []string{"web", "api", "worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitStringList(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
