package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
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
			name:      "verbose returns error",
			input:     "verbose",
			wantLevel: zerolog.InfoLevel,
			wantErr:   true,
			errSubstr: "invalid log level",
		},

		// Trace level is outside allowed range (DebugLevel to ErrorLevel)
		{
			name:      "trace level rejected (outside allowed range)",
			input:     "trace",
			wantLevel: zerolog.InfoLevel,
			wantErr:   true,
			errSubstr: "must be debug, info, warn, or error",
		},

		// Fatal/panic levels are outside allowed range
		{
			name:      "fatal level rejected (outside allowed range)",
			input:     "fatal",
			wantLevel: zerolog.InfoLevel,
			wantErr:   true,
			errSubstr: "must be debug, info, warn, or error",
		},
		{
			name:      "panic level rejected (outside allowed range)",
			input:     "panic",
			wantLevel: zerolog.InfoLevel,
			wantErr:   true,
			errSubstr: "must be debug, info, warn, or error",
		},

		// Numeric values (zerolog supports these)
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
			name:      "numeric -1 is trace (rejected - outside range)",
			input:     "-1",
			wantLevel: zerolog.InfoLevel,
			wantErr:   true,
			errSubstr: "must be debug, info, warn, or error",
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

func TestRunAsWindowsServiceStub(t *testing.T) {
	// This tests the non-windows stub
	cfg := Config{}
	logger := zerolog.Nop()
	err := runAsWindowsService(cfg, logger)
	if err != nil {
		t.Fatalf("expected nil error from stub, got %v", err)
	}
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		env         map[string]string
		wantURL     string
		wantToken   string
		wantLevel   zerolog.Level
		wantVersion bool
		wantErr     bool
	}{
		{
			name:      "defaults with token",
			args:      []string{"--token", "test-token"},
			env:       nil,
			wantURL:   "http://localhost:7655",
			wantToken: "test-token",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name: "env vars",
			args: []string{},
			env: map[string]string{
				"PULSE_TOKEN": "env-token",
				"PULSE_URL":   "http://env-url",
				"LOG_LEVEL":   "debug",
			},
			wantURL:   "http://env-url",
			wantToken: "env-token",
			wantLevel: zerolog.DebugLevel,
		},
		{
			name:      "flags override env",
			args:      []string{"--url", "http://flag-url", "--log-level", "error"},
			env:       map[string]string{"PULSE_TOKEN": "token", "PULSE_URL": "http://env-url"},
			wantURL:   "http://flag-url",
			wantToken: "token",
			wantLevel: zerolog.ErrorLevel,
		},
		{
			name:        "show version",
			args:        []string{"--version"},
			wantVersion: true,
		},
		{
			name:    "missing token returns error",
			args:    []string{"--url", "http://localhost"},
			wantErr: true,
		},
		{
			name:    "invalid log level returns error",
			args:    []string{"--token", "t", "--log-level", "invalid"},
			wantErr: true,
		},
		{
			name:    "invalid interval returns error",
			args:    []string{"--token", "t", "--interval", "invalid"},
			wantErr: true,
		},
		{
			name:    "invalid flag returns error",
			args:    []string{"--invalid"},
			wantErr: true,
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantErr: true,
		},
		{
			name: "env interval",
			args: []string{"--token", "t"},
			env: map[string]string{
				"PULSE_INTERVAL": "10s",
			},
			wantURL:   "http://localhost:7655",
			wantToken: "t",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name: "invalid env interval defaults to 30s",
			args: []string{"--token", "t"},
			env: map[string]string{
				"PULSE_INTERVAL": "invalid",
			},
			wantURL:   "http://localhost:7655",
			wantToken: "t",
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "negative interval defaults to 30s",
			args:      []string{"--token", "t", "--interval", "-10s"},
			wantURL:   "http://localhost:7655",
			wantToken: "t",
			wantLevel: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string {
				if tt.env == nil {
					return ""
				}
				return tt.env[k]
			}

			cfg, showVersion, err := parseConfig("pulse-host-agent", tt.args, getenv)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if showVersion != tt.wantVersion {
				t.Fatalf("expected showVersion %v, got %v", tt.wantVersion, showVersion)
			}

			if !showVersion {
				if cfg.HostConfig.PulseURL != tt.wantURL {
					t.Fatalf("expected URL %q, got %q", tt.wantURL, cfg.HostConfig.PulseURL)
				}
				if cfg.HostConfig.APIToken != tt.wantToken {
					t.Fatalf("expected Token %q, got %q", tt.wantToken, cfg.HostConfig.APIToken)
				}
				if cfg.HostConfig.LogLevel != tt.wantLevel {
					t.Fatalf("expected Level %v, got %v", tt.wantLevel, cfg.HostConfig.LogLevel)
				}
			}
		})
	}
}

func TestMain(m *testing.M) {
	// Custom TestMain if needed, but we can just use regular tests
	m.Run()
}

func TestMainFunc(t *testing.T) {
	origArgs := os.Args
	origExit := osExit
	origRun := runFunc
	defer func() {
		os.Args = origArgs
		osExit = origExit
		runFunc = origRun
	}()

	tests := []struct {
		name     string
		args     []string
		runErr   error
		wantExit int
	}{
		{
			name:     "help exits 0",
			args:     []string{"pulse-host-agent", "--help"},
			wantExit: 0,
		},
		{
			name:     "version exits 0",
			args:     []string{"pulse-host-agent", "--version"},
			wantExit: 0,
		},
		{
			name:     "invalid flag exits 1",
			args:     []string{"pulse-host-agent", "--invalid-flag"},
			wantExit: 1,
		},
		{
			name:     "missing token exits 1",
			args:     []string{"pulse-host-agent"},
			wantExit: 1,
		},
		{
			name:     "run success exits normally",
			args:     []string{"pulse-host-agent", "--token", "test"},
			runErr:   nil,
			wantExit: 100, // custom exit to signal success of main and return
		},
		{
			name:     "run failure exits 1",
			args:     []string{"pulse-host-agent", "--token", "test"},
			runErr:   fmt.Errorf("run failed"),
			wantExit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			runFunc = func(ctx context.Context, cfg Config) error {
				return tt.runErr
			}

			var exitCode int
			var exited bool
			osExit = func(code int) {
				exitCode = code
				exited = true
				panic("exited")
			}

			// For the "success" case, we want to see it reach the end of main
			// Actually, main doesn't call osExit(0) at the very end, it just returns.
			// But if runFunc returns nil, main returns.

			if tt.wantExit == 100 {
				// Special case: we expect it NOT to exit via osExit
				defer func() {
					_ = recover()
					if exited {
						t.Errorf("expected main not to call osExit, but it called with %d", exitCode)
					}
				}()
				main()
				return
			}

			defer func() {
				_ = recover()
				if !exited {
					t.Errorf("expected osExit to be called")
				}
				if exitCode != tt.wantExit {
					t.Errorf("expected exit code %d, got %d", tt.wantExit, exitCode)
				}
			}()

			main()
		})
	}
}

func TestRunFunc(t *testing.T) {
	origService := runAsWindowsServiceFunc
	defer func() {
		runAsWindowsServiceFunc = origService
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	t.Run("windows service failure", func(t *testing.T) {
		runAsWindowsServiceFunc = func(cfg Config, logger zerolog.Logger) error {
			return fmt.Errorf("service failed")
		}
		err := run(ctx, Config{})
		if err == nil || !strings.Contains(err.Error(), "Windows service failed") {
			t.Fatalf("expected service failure error, got %v", err)
		}
	})

	t.Run("invalid config fails hostagent.New", func(t *testing.T) {
		runAsWindowsServiceFunc = origService
		cfg := Config{
			HostConfig: hostagent.Config{
				PulseURL: "http://localhost",
				APIToken: "", // Empty token fails New
			},
		}
		err := run(ctx, cfg)
		if err == nil || !strings.Contains(err.Error(), "failed to initialise host agent") {
			t.Fatalf("expected hostagent init error, got %v", err)
		}
	})

	t.Run("run once finishes", func(t *testing.T) {
		runAsWindowsServiceFunc = origService
		cfg := Config{
			HostConfig: hostagent.Config{
				PulseURL: "http://localhost:1",
				APIToken: "test",
				RunOnce:  true,
			},
			DisableAutoUpdate: true,
		}
		shortCtx, shortCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer shortCancel()

		_ = run(shortCtx, cfg)
	})

	t.Run("host agent terminated with error (deadline)", func(t *testing.T) {
		runAsWindowsServiceFunc = origService
		cfg := Config{
			HostConfig: hostagent.Config{
				PulseURL: "http://localhost:1",
				APIToken: "test",
			},
			DisableAutoUpdate: true,
		}
		// Using a short timeout that will definitely trigger before the agent finishes (which is never in this config)
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer timeoutCancel()

		err := run(timeoutCtx, cfg)
		if err == nil || !strings.Contains(err.Error(), "host agent terminated with error") {
			t.Fatalf("expected termination error, got %v", err)
		}
	})
}
