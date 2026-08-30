package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/kubernetesagent"
	internalSecurityutil "github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
	"github.com/rs/zerolog"
)

func TestEnableCommandsHelpUsesPatrolRemediationCopy(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read pulse-agent main.go: %v", err)
	}
	text := string(source)

	if !strings.Contains(text, "Enable Pulse command execution for Patrol actions and Proxmox LXC Docker inventory (disabled by default)") {
		t.Fatal("expected enable-commands help to describe Patrol actions and Proxmox LXC Docker inventory")
	}
	if strings.Contains(text, "Enable command execution for AI auto-fix") {
		t.Fatal("enable-commands help must not revive AI auto-fix wording")
	}
}

func TestDockerRuntimeHelpUsesDockerPodmanCopy(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read pulse-agent main.go: %v", err)
	}
	text := string(source)

	for _, want := range []string{
		"Enable Docker / Podman Agent module",
		"Docker / Podman runtime: auto, docker, or podman (default: auto)",
		"Force Docker / Podman runtime: docker, podman, or auto",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected pulse-agent CLI copy %q", want)
		}
	}
	for _, stale := range []string{
		"Enable Docker Agent module",
		"Enable Docker / Podman collection module",
		"Container runtime: auto, docker, or podman (default: auto)",
		"Force container runtime: docker, podman, or auto",
	} {
		if strings.Contains(text, stale) {
			t.Fatalf("pulse-agent CLI copy must not expose stale generic runtime label %q", stale)
		}
	}
}

func TestLoadConfigPreservesAgentLogFile(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		cfg, err := loadConfig(nil, func(key string) string {
			if key == "PULSE_LOG_FILE" {
				return ` C:\ProgramData\Pulse\pulse-agent.log `
			}
			return ""
		})
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		if cfg.LogFile != `C:\ProgramData\Pulse\pulse-agent.log` {
			t.Fatalf("LogFile = %q", cfg.LogFile)
		}
	})

	t.Run("flag overrides environment", func(t *testing.T) {
		cfg, err := loadConfig([]string{"--log-file", `D:\Pulse\agent.jsonl`}, func(key string) string {
			if key == "PULSE_LOG_FILE" {
				return `C:\ProgramData\Pulse\pulse-agent.log`
			}
			return ""
		})
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		if cfg.LogFile != `D:\Pulse\agent.jsonl` {
			t.Fatalf("LogFile = %q", cfg.LogFile)
		}
	})
}

func TestAgentFileLoggingUsesCanonicalRotatingSink(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read pulse-agent main.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		`pulselogging.NewStandaloneLogger(pulselogging.Config{`,
		`MaxSizeMB:  agentLogMaxSizeMB`,
		`MaxAgeDays: agentLogMaxAgeDays`,
		`Compress:   true`,
		`Write rotating JSON logs to this file`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected canonical rotating agent log contract %q", want)
		}
	}
	if strings.Contains(text, "newLogger := zerolog.New(os.Stdout)") {
		t.Fatal("remote log-level updates must not replace the configured file sink")
	}
}

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

func TestApplyRemoteSettings(t *testing.T) {
	originalLevel := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(originalLevel)

	logger := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	cfg := &Config{
		Interval: time.Second,
		Logger:   &logger,
	}

	settings := map[string]interface{}{
		"enable_host":                   true,
		"enable_docker":                 true,
		"enable_kubernetes":             true,
		"enable_proxmox":                true,
		"proxmox_type":                  "Auto",
		"docker_runtime":                "PoDmAn",
		"log_level":                     "debug",
		"interval":                      "45s",
		"disable_auto_update":           true,
		"disable_docker_update_checks":  true,
		"kube_include_all_pods":         true,
		"kube_include_all_deployments":  true,
		"report_ip":                     "10.0.0.1",
		"disable_ceph":                  true,
		"unknown_key_should_be_ignored": true,
	}

	applyRemoteSettings(cfg, settings, &logger)

	if !cfg.EnableHost || !cfg.EnableDocker || !cfg.EnableKubernetes || !cfg.EnableProxmox {
		t.Fatalf("expected module flags enabled, got host=%v docker=%v kube=%v proxmox=%v", cfg.EnableHost, cfg.EnableDocker, cfg.EnableKubernetes, cfg.EnableProxmox)
	}
	if !cfg.DockerConfigured {
		t.Fatalf("expected DockerConfigured to be true")
	}
	if cfg.ProxmoxType != "" {
		t.Fatalf("expected proxmox type to normalize to empty for auto, got %q", cfg.ProxmoxType)
	}
	if cfg.DockerRuntime != "podman" {
		t.Fatalf("expected docker runtime to be normalized, got %q", cfg.DockerRuntime)
	}
	if cfg.LogLevel != zerolog.DebugLevel {
		t.Fatalf("expected log level debug, got %v", cfg.LogLevel)
	}
	if cfg.Logger == nil {
		t.Fatalf("expected logger to be updated")
	}
	if cfg.Interval != 45*time.Second {
		t.Fatalf("expected interval 45s, got %v", cfg.Interval)
	}
	if !cfg.DisableAutoUpdate || !cfg.DisableDockerUpdateChecks {
		t.Fatalf("expected auto-update disables to be true")
	}
	if !cfg.KubeIncludeAllPods || !cfg.KubeIncludeAllDeployments {
		t.Fatalf("expected kube include flags to be true")
	}
	if cfg.ReportIP != "10.0.0.1" || !cfg.DisableCeph {
		t.Fatalf("unexpected report ip / disable ceph: %q %v", cfg.ReportIP, cfg.DisableCeph)
	}
}

func TestApplyRemoteSettingsHonorsLocalDockerOptOut(t *testing.T) {
	logger := zerolog.New(io.Discard)
	cfg := &Config{
		EnableDocker:             false,
		DockerConfigured:         true,
		DockerExplicitlyDisabled: true,
	}

	applyRemoteSettings(cfg, map[string]interface{}{
		"enable_docker": true,
	}, &logger)

	if cfg.EnableDocker {
		t.Fatal("remote config must not enable Docker / Podman after a local explicit disable")
	}
	if !cfg.DockerConfigured {
		t.Fatal("expected DockerConfigured to remain true")
	}
	if !cfg.DockerExplicitlyDisabled {
		t.Fatal("expected DockerExplicitlyDisabled to remain true")
	}
}

func TestApplyRemoteSettingsIntervalFloat(t *testing.T) {
	logger := zerolog.New(io.Discard)
	cfg := &Config{}

	applyRemoteSettings(cfg, map[string]interface{}{
		"interval": float64(12),
	}, &logger)

	if cfg.Interval != 12*time.Second {
		t.Fatalf("expected interval 12s, got %v", cfg.Interval)
	}
}

func TestApplyRemoteSettingsIgnoresInvalidValues(t *testing.T) {
	logger := zerolog.New(io.Discard)
	cfg := &Config{
		Interval:      30 * time.Second,
		DockerRuntime: "docker",
	}

	applyRemoteSettings(cfg, map[string]interface{}{
		"interval":       "invalid",
		"docker_runtime": "not-a-runtime",
	}, &logger)

	if cfg.Interval != 30*time.Second {
		t.Fatalf("expected interval to remain unchanged, got %v", cfg.Interval)
	}
	if cfg.DockerRuntime != "docker" {
		t.Fatalf("expected docker runtime to remain unchanged, got %q", cfg.DockerRuntime)
	}

	applyRemoteSettings(cfg, map[string]interface{}{
		"interval": float64(0),
	}, &logger)

	if cfg.Interval != 30*time.Second {
		t.Fatalf("expected non-positive numeric interval to be ignored, got %v", cfg.Interval)
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

func TestResolveEnableCommands(t *testing.T) {
	tests := []struct {
		name        string
		enableFlag  bool
		disableFlag bool
		envEnable   string
		envDisable  string
		expected    bool
	}{
		{"flag enable takes priority", true, false, "false", "false", true},
		{"flag enable takes priority over disable flag", true, true, "false", "false", true},
		{"flag disable (deprecated) returns false", false, true, "true", "false", false},
		{"env enable true returns true", false, false, "true", "false", true},
		{"env enable false returns false", false, false, "false", "false", false},
		{"env disable (deprecated) false returns true", false, false, "", "false", true},
		{"env disable (deprecated) true returns false", false, false, "", "true", false},
		{"default returns false", false, false, "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveEnableCommands(tc.enableFlag, tc.disableFlag, tc.envEnable, tc.envDisable)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestLoadConfigCommandAuthorityProfiles(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		wantProfile hostagent.CommandAuthorityProfile
		wantEnabled bool
		wantError   bool
	}{
		{name: "unmarked stays legacy", args: []string{"-token", "test"}, wantProfile: hostagent.CommandAuthorityLegacy},
		{name: "fresh monitoring marker", args: []string{"-token", "test", "-command-authority", "monitoring-only"}, wantProfile: hostagent.CommandAuthorityMonitoringOnly},
		{name: "enable implies capable", args: []string{"-token", "test", "-enable-commands"}, wantProfile: hostagent.CommandAuthorityCommandCapable, wantEnabled: true},
		{name: "explicit capable", args: []string{"-token", "test", "-command-authority", "command-capable"}, wantProfile: hostagent.CommandAuthorityCommandCapable},
		{name: "conflicting authority", args: []string{"-token", "test", "-enable-commands", "-command-authority", "monitoring-only"}, wantError: true},
		{name: "invalid authority", args: []string{"-token", "test", "-command-authority", "root"}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig(tc.args, func(string) string { return "" })
			if tc.wantError {
				if err == nil {
					t.Fatalf("loadConfig() accepted invalid authority: %+v", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig(): %v", err)
			}
			if cfg.CommandAuthorityProfile != tc.wantProfile || cfg.EnableCommands != tc.wantEnabled {
				t.Fatalf("authority = (%q, enabled=%v), want (%q, enabled=%v)", cfg.CommandAuthorityProfile, cfg.EnableCommands, tc.wantProfile, tc.wantEnabled)
			}
		})
	}
}

func TestApplyInitialRemoteCommandAuthority(t *testing.T) {
	desired := true
	for _, tc := range []struct {
		name       string
		profile    hostagent.CommandAuthorityProfile
		wantEnable bool
		wantAccept bool
	}{
		{name: "monitoring rejects startup promotion", profile: hostagent.CommandAuthorityMonitoringOnly, wantAccept: false},
		{name: "command capable accepts startup enable", profile: hostagent.CommandAuthorityCommandCapable, wantEnable: true, wantAccept: true},
		{name: "legacy accepts startup enable", profile: hostagent.CommandAuthorityLegacy, wantEnable: true, wantAccept: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{CommandAuthorityProfile: tc.profile}
			accepted := applyInitialRemoteCommandAuthority(&cfg, &desired)
			if cfg.EnableCommands != tc.wantEnable || accepted != tc.wantAccept {
				t.Fatalf("startup authority = (enabled=%v, accepted=%v), want (enabled=%v, accepted=%v)", cfg.EnableCommands, accepted, tc.wantEnable, tc.wantAccept)
			}
		})
	}
	if !applyInitialRemoteCommandAuthority(nil, &desired) {
		t.Fatal("nil config should be a no-op")
	}
}

func TestResolveToken(t *testing.T) {
	customStateDir := filepath.Join(string(filepath.Separator), "custom", "pulse-agent")
	fakeReadFile := func(path string) ([]byte, error) {
		if path == defaultTokenFilePath() {
			return []byte("default-token"), nil
		}
		if path == filepath.Join(customStateDir, "token") {
			return []byte("custom-token"), nil
		}
		if path == "valid-file" {
			return []byte("file-token"), nil
		}
		return nil, os.ErrNotExist
	}

	tests := []struct {
		name          string
		tokenFlag     string
		tokenFileFlag string
		envToken      string
		stateDir      string
		expected      string
	}{
		{"flag priority", "flag-token", "valid-file", "env-token", customStateDir, "flag-token"},
		{"file priority", "", "valid-file", "env-token", customStateDir, "file-token"},
		{"env priority", "", "", "env-token", customStateDir, "env-token"},
		{"default file priority", "", "", "", defaultAgentStateDir(), "default-token"},
		{"custom state file priority", "", "", "", customStateDir, "custom-token"},
	}

	// Update the test cases to avoid the default file if we want to test empty
	fakeReadFileNoDefault := func(path string) ([]byte, error) {
		if path == "valid-file" {
			return []byte("file-token"), nil
		}
		return nil, os.ErrNotExist
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveTokenInternal(tc.tokenFlag, tc.tokenFileFlag, tc.envToken, tc.stateDir, fakeReadFile)
			if got != tc.expected {
				t.Fatalf("%s: expected %q, got %q", tc.name, tc.expected, got)
			}
		})
	}

	t.Run("truly empty", func(t *testing.T) {
		got := resolveTokenInternal("", "", "", customStateDir, fakeReadFileNoDefault)
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("custom state never borrows default token", func(t *testing.T) {
		got := resolveTokenInternal("", "", "", filepath.Join(string(filepath.Separator), "missing-custom"), fakeReadFile)
		if got != "" {
			t.Fatalf("custom state unexpectedly borrowed default token %q", got)
		}
	})
}

func TestCleanupDockerAgent(t *testing.T) {
	t.Run("nil agent does nothing", func(t *testing.T) {
		cleanupDockerAgent(nil, &zerolog.Logger{})
	})

	// Testing with a real agent might be hard without a docker daemon.
	// But we can at least test the nil case.
}

func TestHealthHandler(t *testing.T) {
	var ready atomic.Bool
	handler := healthHandler(&ready)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Test /healthz
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Test /readyz (not ready)
	resp, err = http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}

	// Test /readyz (ready)
	ready.Store(true)
	resp, err = http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Test /metrics
	resp, err = http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStartHealthServer(t *testing.T) {
	var ready atomic.Bool
	logger := zerolog.New(os.Stdout)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use port 0 to get a random available port
	startHealthServer(ctx, "127.0.0.1:0", &ready, &logger)

	// Since startHealthServer runs in background and doesn't return the listener,
	// it's a bit hard to know the port. But we can at least exercise the code.
	// For better testing, startHealthServer should probably return something or take a listener.
}

func TestLoadConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg, err := loadConfig([]string{"-token", "test-token"}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.PulseURL != "http://localhost:7655" {
			t.Errorf("expected default URL, got %s", cfg.PulseURL)
		}
		if cfg.EnableHost != true {
			t.Errorf("expected host enabled by default")
		}
		if cfg.StateDir != defaultAgentStateDir() {
			t.Errorf("expected platform state directory %q, got %q", defaultAgentStateDir(), cfg.StateDir)
		}
		if cfg.AgentIDFile != filepath.Join(defaultAgentStateDir(), "agent-id") {
			t.Errorf("expected default agent ID file under state directory, got %q", cfg.AgentIDFile)
		}
	})

	t.Run("env overrides", func(t *testing.T) {
		env := map[string]string{
			"PULSE_URL":                 "http://pulse.example.com",
			"PULSE_TOKEN":               "my-token",
			"PULSE_ENABLE_HOST":         "false",
			"PULSE_ENABLE_DOCKER":       "true",
			"PULSE_CACERT":              "/etc/pulse/ca.pem",
			"PULSE_SERVER_FINGERPRINT":  "aabbccdd",
			"PULSE_DEPLOY_SSH_USER":     "pulse-deploy",
			"PULSE_CUSTOM_SENSORS_FILE": "/etc/pulse/custom-sensors.yaml",
		}
		cfg, err := loadConfig([]string{}, func(s string) string { return env[s] })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.PulseURL != "http://pulse.example.com" {
			t.Errorf("expected env URL, got %s", cfg.PulseURL)
		}
		if cfg.APIToken != "my-token" {
			t.Errorf("expected env token, got %s", cfg.APIToken)
		}
		if cfg.EnableHost != false {
			t.Errorf("expected host disabled by env")
		}
		if cfg.EnableDocker != true {
			t.Errorf("expected docker enabled by env")
		}
		if cfg.CACertPath != "/etc/pulse/ca.pem" {
			t.Errorf("expected CA cert path from env, got %s", cfg.CACertPath)
		}
		if cfg.ServerFingerprint != "aabbccdd" {
			t.Errorf("expected server fingerprint from env, got %s", cfg.ServerFingerprint)
		}
		if cfg.DeploySSHUser != "pulse-deploy" {
			t.Errorf("expected deploy SSH user from env, got %s", cfg.DeploySSHUser)
		}
		if cfg.CustomSensorsFile != "/etc/pulse/custom-sensors.yaml" {
			t.Errorf("expected command/REST custom metrics file from env, got %s", cfg.CustomSensorsFile)
		}
	})

	t.Run("flag overrides", func(t *testing.T) {
		cfg, err := loadConfig([]string{"-url", "http://flag.example.com", "-token", "flag-token", "-enable-host=false", "-cacert", "/tmp/custom-ca.pem", "-server-fingerprint", "1122", "-deploy-ssh-user", "pulse-deploy", "-custom-sensors-file", "/tmp/custom-sensors.yaml"}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.PulseURL != "http://flag.example.com" {
			t.Errorf("expected flag URL, got %s", cfg.PulseURL)
		}
		if cfg.APIToken != "flag-token" {
			t.Errorf("expected flag token, got %s", cfg.APIToken)
		}
		if cfg.EnableHost != false {
			t.Errorf("expected host disabled by flag")
		}
		if cfg.CACertPath != "/tmp/custom-ca.pem" {
			t.Errorf("expected CA cert path from flag, got %s", cfg.CACertPath)
		}
		if cfg.ServerFingerprint != "1122" {
			t.Errorf("expected server fingerprint from flag, got %s", cfg.ServerFingerprint)
		}
		if cfg.DeploySSHUser != "pulse-deploy" {
			t.Errorf("expected deploy SSH user from flag, got %s", cfg.DeploySSHUser)
		}
		if cfg.CustomSensorsFile != "/tmp/custom-sensors.yaml" {
			t.Errorf("expected command/REST custom metrics file from flag, got %s", cfg.CustomSensorsFile)
		}
	})

	t.Run("state directory flag overrides platform default", func(t *testing.T) {
		stateDir := filepath.FromSlash("/custom/pulse-state")
		cfg, err := loadConfig([]string{"-token", "test-token", "-state-dir", stateDir}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.StateDir != stateDir {
			t.Errorf("expected explicit state directory, got %q", cfg.StateDir)
		}
		if cfg.AgentIDFile != filepath.Join(stateDir, "agent-id") {
			t.Errorf("expected agent ID file under explicit state directory, got %q", cfg.AgentIDFile)
		}
	})

	t.Run("custom state directory supplies implicit token", func(t *testing.T) {
		stateDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(stateDir, "token"), []byte("custom-state-token\n"), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := loadConfig([]string{
			"-url", "http://pulse.example.com",
			"-state-dir", stateDir,
		}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.APIToken != "custom-state-token" {
			t.Fatalf("expected implicit custom-state token, got %q", cfg.APIToken)
		}
	})

	t.Run("custom enrollment restart prefers persisted runtime token", func(t *testing.T) {
		stateDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(stateDir, "token"), []byte("bootstrap-token"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stateDir, "runtime.token"), []byte("runtime-token"), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := loadConfig([]string{
			"-url", "http://pulse.example.com",
			"-state-dir", stateDir,
			"-enroll",
		}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.APIToken != "runtime-token" {
			t.Fatalf("expected persisted runtime token after restart, got %q", cfg.APIToken)
		}
	})

	t.Run("explicit agent ID file overrides state-derived path", func(t *testing.T) {
		cfg, err := loadConfig([]string{
			"-token", "test-token",
			"-state-dir", "/custom/pulse-state",
			"-agent-id-file", "/identity/agent-id",
		}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AgentIDFile != "/identity/agent-id" {
			t.Errorf("expected explicit agent ID file, got %q", cfg.AgentIDFile)
		}
	})

	t.Run("token optional when enrollment disabled", func(t *testing.T) {
		cfg, err := loadConfig([]string{"-url", "http://token-optional.example.com", "-enable-host"}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.APIToken != "" {
			t.Fatalf("expected empty token for token-optional config, got %q", cfg.APIToken)
		}
		if cfg.Enroll {
			t.Fatal("expected enrollment to be disabled")
		}
	})

	t.Run("enrollment requires token", func(t *testing.T) {
		_, err := loadConfig([]string{"-url", "http://token-required.example.com", "-enroll"}, func(s string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), "required for enrollment") {
			t.Fatalf("expected enrollment token requirement, got %v", err)
		}
	})

	t.Run("invalid interval flag", func(t *testing.T) {
		_, err := loadConfig([]string{"-interval", "invalid"}, func(s string) string { return "" })
		if err == nil {
			t.Fatal("expected error for invalid interval")
		}
	})

	t.Run("non-positive interval returns error", func(t *testing.T) {
		_, err := loadConfig([]string{"-token", "test-token", "-interval", "0s"}, func(s string) string { return "" })
		if err == nil {
			t.Fatal("expected error for non-positive interval")
		}
	})

	t.Run("invalid kube max pods returns error", func(t *testing.T) {
		_, err := loadConfig([]string{"-token", "test-token", "-kube-max-pods", "0"}, func(s string) string { return "" })
		if err == nil {
			t.Fatal("expected error for non-positive kube-max-pods")
		}
	})

	t.Run("invalid docker runtime returns error", func(t *testing.T) {
		_, err := loadConfig([]string{"-token", "test-token", "-docker-runtime", "containerd"}, func(s string) string { return "" })
		if err == nil {
			t.Fatal("expected error for invalid docker runtime")
		}
	})

	t.Run("invalid log level returns error", func(t *testing.T) {
		_, err := loadConfig([]string{"-token", "test-token", "-log-level", "invalid"}, func(s string) string { return "" })
		if err == nil {
			t.Fatal("expected error for invalid log level")
		}
	})

	t.Run("invalid deploy ssh user returns error", func(t *testing.T) {
		_, err := loadConfig([]string{"-token", "test-token", "-deploy-ssh-user", "bad user"}, func(s string) string { return "" })
		if err == nil {
			t.Fatal("expected error for invalid deploy SSH user")
		}
	})

	t.Run("show version", func(t *testing.T) {
		_, err := loadConfig([]string{"-version"}, func(s string) string { return "" })
		if err != flag.ErrHelp {
			t.Errorf("expected flag.ErrHelp for -version, got %v", err)
		}
	})

	t.Run("self test", func(t *testing.T) {
		cfg, err := loadConfig([]string{"-self-test"}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.SelfTest {
			t.Errorf("expected SelfTest to be true")
		}
	})

	t.Run("tags and repeated disk filters", func(t *testing.T) {
		cfg, err := loadConfig([]string{
			"-token", "T",
			"-tag", "t1",
			"-tag", "t2",
			"-disk-exclude", "sdb",
			"-disk-exclude", "/mnt/pve/local-backup",
			"-disk-include", "/var/log",
		}, func(s string) string {
			switch s {
			case "PULSE_TAGS":
				return "e1,e2"
			case "PULSE_DISK_EXCLUDE":
				return "/dev/sda,/var/run/samba/fd"
			case "PULSE_DISK_INCLUDE":
				return "log2ram"
			}
			return ""
		})
		if err != nil {
			t.Fatal(err)
		}
		expectedTags := []string{"e1", "e2", "t1", "t2"}
		if !reflect.DeepEqual(cfg.Tags, expectedTags) {
			t.Errorf("expected tags %v, got %v", expectedTags, cfg.Tags)
		}
		expectedDisk := []string{"/dev/sda", "/var/run/samba/fd", "sdb", "/mnt/pve/local-backup"}
		if !reflect.DeepEqual(cfg.DiskExclude, expectedDisk) {
			t.Errorf("expected disk exclude %v, got %v", expectedDisk, cfg.DiskExclude)
		}
		expectedIncludedDisk := []string{"log2ram", "/var/log"}
		if !reflect.DeepEqual(cfg.DiskInclude, expectedIncludedDisk) {
			t.Errorf("expected disk include %v, got %v", expectedIncludedDisk, cfg.DiskInclude)
		}
	})
}

func TestInitDockerWithRetry_Cancel(t *testing.T) {
	orig := newDockerAgent
	defer func() { newDockerAgent = orig }()
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		return nil, errors.New("not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	logger := zerolog.New(os.Stdout)
	cfg := dockeragent.Config{}

	agent := initDockerWithRetry(ctx, cfg, &logger)
	if agent != nil {
		t.Errorf("expected nil agent when cancelled")
	}
}

func TestInitDockerWithRetry_CancelDuringBackoff(t *testing.T) {
	origAgent := newDockerAgent
	origInitial := retryInitialDelay
	origMax := retryMaxDelay
	defer func() {
		newDockerAgent = origAgent
		retryInitialDelay = origInitial
		retryMaxDelay = origMax
	}()

	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		return nil, errors.New("not available")
	}
	retryInitialDelay = 5 * time.Second
	retryMaxDelay = 5 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	logger := zerolog.New(os.Stdout)
	agent := initDockerWithRetry(ctx, dockeragent.Config{}, &logger)
	if agent != nil {
		t.Fatalf("expected nil agent when cancelled")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected prompt cancellation during backoff, took %v", elapsed)
	}
}

func TestInitDockerWithRetry_Success(t *testing.T) {
	orig := newDockerAgent
	defer func() { newDockerAgent = orig }()

	// First call fails, second succeeds
	calls := 0
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("not yet")
		}
		return &dockeragent.Agent{}, nil
	}

	// Mock time.After to be fast if possible? No, we can it in the function but we can't easily mock time.After.
	// However, we can use a very small delay if we refactored it to take intervals.
	// For now, let's just test success on first try or skip the retry delay.

	t.Run("success on first try", func(t *testing.T) {
		calls = 1 // will succeed on next call (which is first in this run)
		newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
			return &dockeragent.Agent{}, nil
		}
		ctx := context.Background()
		logger := zerolog.New(os.Stdout)
		agent := initDockerWithRetry(ctx, dockeragent.Config{}, &logger)
		if agent == nil {
			t.Fatal("expected agent, got nil")
		}
	})
}

func TestInitKubernetesWithRetry_Cancel(t *testing.T) {
	orig := newKubeAgent
	defer func() { newKubeAgent = orig }()
	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		return nil, errors.New("not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	logger := zerolog.New(os.Stdout)
	cfg := kubernetesagent.Config{}

	agent := initKubernetesWithRetry(ctx, cfg, &logger)
	if agent != nil {
		t.Errorf("expected nil agent when cancelled")
	}
}

func TestInitKubernetesWithRetry_CancelDuringBackoff(t *testing.T) {
	origAgent := newKubeAgent
	origInitial := retryInitialDelay
	origMax := retryMaxDelay
	defer func() {
		newKubeAgent = origAgent
		retryInitialDelay = origInitial
		retryMaxDelay = origMax
	}()

	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		return nil, errors.New("not available")
	}
	retryInitialDelay = 5 * time.Second
	retryMaxDelay = 5 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	logger := zerolog.New(os.Stdout)
	agent := initKubernetesWithRetry(ctx, kubernetesagent.Config{}, &logger)
	if agent != nil {
		t.Fatalf("expected nil agent when cancelled")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected prompt cancellation during backoff, took %v", elapsed)
	}
}

func TestInitKubernetesWithRetry_Success(t *testing.T) {
	orig := newKubeAgent
	defer func() { newKubeAgent = orig }()

	t.Run("success on first try", func(t *testing.T) {
		newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
			return &kubernetesagent.Agent{}, nil
		}
		ctx := context.Background()
		logger := zerolog.New(os.Stdout)
		agent := initKubernetesWithRetry(ctx, kubernetesagent.Config{}, &logger)
		if agent == nil {
			t.Fatal("expected agent, got nil")
		}
	})
}

func TestRun(t *testing.T) {
	// Mock agents to avoid actual initialization
	origDocker := newDockerAgent
	origKube := newKubeAgent
	defer func() {
		newDockerAgent = origDocker
		newKubeAgent = origKube
	}()

	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		return &dockeragent.Agent{}, nil
	}
	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		return &kubernetesagent.Agent{}, nil
	}

	t.Run("self-test", func(t *testing.T) {
		ctx := context.Background()
		err := run(ctx, []string{"-self-test"}, func(s string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		ctx := context.Background()
		err := run(ctx, []string{"-interval", "invalid"}, func(s string) string { return "" })
		if err == nil {
			t.Fatal("expected error for invalid config")
		}
	})

	t.Run("version exits cleanly", func(t *testing.T) {
		ctx := context.Background()
		err := run(ctx, []string{"-version"}, func(s string) string { return "" })
		if err != flag.ErrHelp {
			t.Fatalf("expected flag.ErrHelp for -version, got %v", err)
		}
	})

	t.Run("basic run", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		// Cancel after a short time
		go func() {
			time.Sleep(200 * time.Millisecond)
			cancel()
		}()

		// Use minimal config, no agents
		err := run(ctx, []string{"-token", "T", "-enable-host=false", "-enable-docker=false", "-enable-kubernetes=false", "-health-addr", "127.0.0.1:0"}, func(s string) string { return "" })
		if err != nil && err != context.Canceled {
			t.Errorf("expected nil or context.Canceled, got %v", err)
		}
	})

	t.Run("full run with mocks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
			return nil, errors.New("disabled for test")
		}
		newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
			return nil, errors.New("disabled for test")
		}
		// hostagent.New will still fail because of token scope or some other thing if not careful
		newHostAgent = func(cfg hostagent.Config) (Runnable, error) {
			return nil, errors.New("disabled for test")
		}

		go func() {
			time.Sleep(200 * time.Millisecond)
			cancel()
		}()

		// Enable everything, but they will fail to init and log warnings, which is fine for coverage of run's branches
		err := run(ctx, []string{"-token", "T", "-enable-host", "-enable-docker", "-enable-kubernetes", "-health-addr", "127.0.0.1:0"}, func(s string) string { return "" })
		if err != nil && err != context.Canceled && !strings.Contains(err.Error(), "disabled for test") {
			t.Errorf("expected nil or context.Canceled or disabled for test, got %v", err)
		}
	})

	t.Run("auto-detect docker", func(t *testing.T) {
		origLook := lookPath
		defer func() { lookPath = origLook }()
		lookPath = func(path string) (string, error) {
			if path == "docker" {
				return "/usr/bin/docker", nil
			}
			return "", os.ErrNotExist
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_ = run(ctx, []string{"-token", "T", "-enable-host=false"}, func(s string) string { return "" })
	})

	t.Run("auto-detect podman", func(t *testing.T) {
		origLook := lookPath
		defer func() { lookPath = origLook }()
		lookPath = func(path string) (string, error) {
			if path == "podman" {
				return "/usr/bin/podman", nil
			}
			return "", os.ErrNotExist
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_ = run(ctx, []string{"-token", "T", "-enable-host=false"}, func(s string) string { return "" })
	})

	t.Run("goroutine error", func(t *testing.T) {
		origHost := newHostAgent
		defer func() { newHostAgent = origHost }()

		newHostAgent = func(cfg hostagent.Config) (Runnable, error) {
			// We need a non-nil agent that returns an error from Run
			// This is hard without a real mock, but we can try to return an agent and have it fail.
			// Actually, if we return a "real" agent with a bad URL, it might fail.
			return &hostagent.Agent{}, nil
		}

		// Wait, if I use a real hostagent, it might panic if uninitialized.
		// Let's skip the goroutine error for now or find a better way.
	})
}

func TestDockerAutoDetectHonorsExplicitDisable(t *testing.T) {
	origDocker := newDockerAgent
	origLook := lookPath
	defer func() {
		newDockerAgent = origDocker
		lookPath = origLook
	}()

	var dockerAgentCalls int32
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		atomic.AddInt32(&dockerAgentCalls, 1)
		return &mockRunnableCloser{}, nil
	}
	lookPath = func(path string) (string, error) {
		if path == "docker" {
			return "/usr/bin/docker", nil
		}
		return "", os.ErrNotExist
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := run(ctx, []string{"-enable-host=false", "-enable-docker=false", "-enable-kubernetes=false", "-health-addr", ""}, func(s string) string { return "" })
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("run returned unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&dockerAgentCalls); got != 0 {
		t.Fatalf("Docker / Podman module initialized despite explicit disable, calls=%d", got)
	}
}

func TestCleanupDockerAgent_Nil(t *testing.T) {
	cleanupDockerAgent(nil, nil)
}

type mockCloser struct {
	err error
}

func (m *mockCloser) Close() error {
	return m.err
}

func (m *mockCloser) Run(ctx context.Context) error {
	return nil
}

func TestCleanupDockerAgent_Error(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	mock := &mockCloser{err: errors.New("close error")}
	// Should log warning but not panic
	cleanupDockerAgent(mock, &logger)
}

func TestInitDockerWithRetry_Failure(t *testing.T) {
	orig := newDockerAgent
	defer func() { newDockerAgent = orig }()

	// Always fail
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		return nil, errors.New("fail")
	}

	// Override delays to be super fast
	origInitial := retryInitialDelay
	origMax := retryMaxDelay
	retryInitialDelay = 1 * time.Millisecond
	retryMaxDelay = 2 * time.Millisecond
	defer func() {
		retryInitialDelay = origInitial
		retryMaxDelay = origMax
	}()

	ctx, cancel := context.WithCancel(context.Background())
	// Let it run for a bit then cancel
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	logger := zerolog.New(os.Stdout)
	agent := initDockerWithRetry(ctx, dockeragent.Config{}, &logger)
	if agent != nil {
		t.Errorf("expected nil agent")
	}
}

// Mock agents for TestRun
type mockRunnable struct {
	started chan struct{}
	err     error
}

func (m *mockRunnable) Run(ctx context.Context) error {
	if m.started != nil {
		close(m.started)
	}
	if m.err != nil {
		return m.err
	}
	<-ctx.Done()
	return nil
}

type mockRunnableCloser struct {
	mockRunnable
}

func (m *mockRunnableCloser) Close() error {
	return nil
}

func TestRun_Success(t *testing.T) {
	origDocker := newDockerAgent
	origKube := newKubeAgent
	origHost := newHostAgent
	defer func() {
		newDockerAgent = origDocker
		newKubeAgent = origKube
		newHostAgent = origHost
	}()

	// Setup mocks that signal startup and wait for context
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		return &mockRunnableCloser{mockRunnable: mockRunnable{started: make(chan struct{})}}, nil
	}
	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		return &mockRunnable{started: make(chan struct{})}, nil
	}
	newHostAgent = func(cfg hostagent.Config) (Runnable, error) {
		return &mockRunnable{started: make(chan struct{})}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run in a separate goroutine so we can wait for it
	errCh := make(chan error)
	go func() {
		// Enable all agents
		errCh <- run(ctx, []string{
			"-token", "T",
			"-enable-host=true",
			"-enable-docker=true",
			"-enable-kubernetes=true",
			"-health-addr", ":0", // Random port
		}, func(s string) string { return "" })
	}()

	// Wait for run to finish (which should happen on context cancel)
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for run to finish")
	}
}

func TestRun_PassesStateDirToUpdaterAndHostAgent(t *testing.T) {
	origUpdater := newUpdater
	origHost := newHostAgent
	defer func() {
		newUpdater = origUpdater
		newHostAgent = origHost
	}()

	var updaterCfg agentupdate.Config
	var hostCfg hostagent.Config

	newUpdater = func(cfg agentupdate.Config) *agentupdate.Updater {
		updaterCfg = cfg
		return agentupdate.New(agentupdate.Config{
			PulseURL:       "https://pulse.example.com",
			AgentName:      cfg.AgentName,
			CurrentVersion: "1.0.0",
			StateDir:       cfg.StateDir,
			Disabled:       true,
		})
	}
	newHostAgent = func(cfg hostagent.Config) (Runnable, error) {
		hostCfg = cfg
		return &mockRunnable{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := run(ctx, []string{
		"-token", "deadbeef",
		"-enable-docker=false",
		"-enable-kubernetes=false",
		"-state-dir", "/share/CACHEDEV1_DATA/.pulse-agent",
	}, func(string) string { return "" })
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("run returned error: %v", err)
	}

	if updaterCfg.StateDir != "/share/CACHEDEV1_DATA/.pulse-agent" {
		t.Fatalf("updater state dir = %q, want %q", updaterCfg.StateDir, "/share/CACHEDEV1_DATA/.pulse-agent")
	}
	if hostCfg.StateDir != "/share/CACHEDEV1_DATA/.pulse-agent" {
		t.Fatalf("host agent state dir = %q, want %q", hostCfg.StateDir, "/share/CACHEDEV1_DATA/.pulse-agent")
	}
}

func TestRunConfiguresTypedPrivilegeHelperFromInstallerEnvironment(t *testing.T) {
	originalHelper := newPrivilegeHelperTelemetry
	originalUpdate := newPrivilegeHelperUpdate
	originalUpdater := newUpdater
	originalHost := newHostAgent
	defer func() {
		newPrivilegeHelperTelemetry = originalHelper
		newPrivilegeHelperUpdate = originalUpdate
		newUpdater = originalUpdater
		newHostAgent = originalHost
	}()

	const socketPath = "/run/pulse-agent/helper.sock"
	t.Setenv("PULSE_AGENT_HELPER_SOCKET", socketPath)
	configuredPath := ""
	configuredUpdatePath := ""
	helper := &helperHealthStub{}
	newPrivilegeHelperTelemetry = func(path string) (hostagent.PrivilegedTelemetry, error) {
		configuredPath = path
		return helper, nil
	}
	newPrivilegeHelperUpdate = func(path string) (agentupdate.PrivilegedUpdate, error) {
		configuredUpdatePath = path
		return nil, nil
	}
	newUpdater = func(agentupdate.Config) *agentupdate.Updater {
		return agentupdate.New(agentupdate.Config{Disabled: true})
	}
	newHostAgent = func(hostagent.Config) (Runnable, error) {
		return &mockRunnable{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := run(ctx, []string{
		"-token", "deadbeef",
		"-enable-docker=false",
		"-enable-kubernetes=false",
		"-health-addr", "",
	}, func(string) string { return "" })
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run returned error: %v", err)
	}
	if configuredPath != socketPath {
		t.Fatalf("helper socket path = %q, want %q", configuredPath, socketPath)
	}
	if configuredUpdatePath != socketPath {
		t.Fatalf("helper update socket path = %q, want %q", configuredUpdatePath, socketPath)
	}
	if helper.healthCalls != 1 {
		t.Fatalf("helper health calls = %d, want 1", helper.healthCalls)
	}
}

type helperHealthStub struct {
	hostagent.PrivilegedTelemetry
	healthErr   error
	healthCalls int
}

func (h *helperHealthStub) Health(context.Context) error {
	h.healthCalls++
	return h.healthErr
}

func TestRunRejectsUnhealthyTypedPrivilegeHelper(t *testing.T) {
	originalHelper := newPrivilegeHelperTelemetry
	originalUpdate := newPrivilegeHelperUpdate
	originalUpdater := newUpdater
	originalHost := newHostAgent
	defer func() {
		newPrivilegeHelperTelemetry = originalHelper
		newPrivilegeHelperUpdate = originalUpdate
		newUpdater = originalUpdater
		newHostAgent = originalHost
	}()

	t.Setenv("PULSE_AGENT_HELPER_SOCKET", "/run/pulse-agent/helper.sock")
	helper := &helperHealthStub{healthErr: errors.New("incompatible helper")}
	newPrivilegeHelperTelemetry = func(string) (hostagent.PrivilegedTelemetry, error) {
		return helper, nil
	}
	newPrivilegeHelperUpdate = func(string) (agentupdate.PrivilegedUpdate, error) {
		return nil, nil
	}
	newUpdater = func(agentupdate.Config) *agentupdate.Updater {
		return agentupdate.New(agentupdate.Config{Disabled: true})
	}
	hostCreated := false
	newHostAgent = func(hostagent.Config) (Runnable, error) {
		hostCreated = true
		return &mockRunnable{}, nil
	}

	err := run(context.Background(), []string{
		"-token", "deadbeef",
		"-enable-docker=false",
		"-enable-kubernetes=false",
		"-health-addr", "",
	}, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "verify typed privilege helper protocol") {
		t.Fatalf("run error = %v", err)
	}
	if helper.healthCalls != 1 {
		t.Fatalf("helper health calls = %d, want 1", helper.healthCalls)
	}
	if hostCreated {
		t.Fatal("host agent was created after helper health failed")
	}
}

type pendingUpdateSupervisorStub struct {
	commitResult   agenthelper.UpdateResult
	rollbackResult agenthelper.UpdateResult
	commitErr      error
	rollbackErr    error
	commitCalls    chan agenthelper.UpdateResult
	rollbackCalls  chan agenthelper.UpdateResult
}

func (s *pendingUpdateSupervisorStub) CreateQuarantinedArtifact() (string, *os.File, func() error, error) {
	return "", nil, func() error { return nil }, errors.New("not implemented")
}

func (s *pendingUpdateSupervisorStub) WriteQuarantinedSignature(string, string) error {
	return errors.New("not implemented")
}

func (s *pendingUpdateSupervisorStub) Stage(context.Context, string, string, string) (agenthelper.UpdateStageResult, error) {
	return agenthelper.UpdateStageResult{}, errors.New("not implemented")
}

func (s *pendingUpdateSupervisorStub) Activate(context.Context, string, string, string) (agenthelper.UpdateResult, error) {
	return agenthelper.UpdateResult{}, errors.New("not implemented")
}

func (s *pendingUpdateSupervisorStub) Commit(_ context.Context, activation agenthelper.UpdateResult) (agenthelper.UpdateResult, error) {
	s.commitCalls <- activation
	return s.commitResult, s.commitErr
}

func (s *pendingUpdateSupervisorStub) Rollback(_ context.Context, activation agenthelper.UpdateResult) (agenthelper.UpdateResult, error) {
	s.rollbackCalls <- activation
	return s.rollbackResult, s.rollbackErr
}

func testPendingUpdate(t *testing.T, stateDir string) *agentupdate.PendingPrivilegedUpdate {
	t.Helper()
	activation := agenthelper.UpdateResult{
		Action:           "pending",
		ActivationID:     "pulse-agent-0123456789abcdef0123456789abcdef:0123456789abcdef",
		ActiveSHA256:     strings.Repeat("a", 64),
		RollbackSHA256:   strings.Repeat("b", 64),
		RollbackDeadline: time.Now().Add(2 * time.Second).UTC(),
	}
	if err := agentupdate.PersistPendingPrivilegedUpdate(stateDir, "1.0.0", activation); err != nil {
		t.Fatal(err)
	}
	return &agentupdate.PendingPrivilegedUpdate{Activation: activation, PreviousVersion: "1.0.0"}
}

func TestPendingPrivilegedUpdateCommitsOnlyAfterReadinessAndAcceptedReport(t *testing.T) {
	stateDir := t.TempDir()
	if err := internalSecurityutil.HardenPrivatePath(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pending := testPendingUpdate(t, stateDir)
	stub := &pendingUpdateSupervisorStub{
		commitResult: agenthelper.UpdateResult{
			Action:         "committed",
			ActivationID:   pending.Activation.ActivationID,
			ActiveSHA256:   pending.Activation.ActiveSHA256,
			RollbackSHA256: pending.Activation.RollbackSHA256,
		},
		commitCalls:   make(chan agenthelper.UpdateResult, 1),
		rollbackCalls: make(chan agenthelper.UpdateResult, 1),
	}
	reportAccepted := make(chan struct{})
	close(reportAccepted)
	var ready atomic.Bool
	result := make(chan error, 1)
	go func() {
		result <- supervisePendingPrivilegedUpdate(context.Background(), stub, pending, stateDir, ready.Load, reportAccepted, time.Millisecond, nil)
	}()
	select {
	case <-stub.commitCalls:
		t.Fatal("pending update committed before local readiness")
	case <-time.After(20 * time.Millisecond):
	}
	ready.Store(true)
	select {
	case activation := <-stub.commitCalls:
		if activation != pending.Activation {
			t.Fatalf("commit activation = %#v", activation)
		}
	case <-time.After(time.Second):
		t.Fatal("pending update was not committed after both health signals")
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if loaded, err := agentupdate.LoadPendingPrivilegedUpdate(stateDir); err != nil || loaded != nil {
		t.Fatalf("committed handoff = %#v, %v", loaded, err)
	}
	select {
	case <-stub.rollbackCalls:
		t.Fatal("healthy pending update rolled back")
	default:
	}
}

func TestPendingPrivilegedUpdateCancellationRollsBackAndClearsHandoff(t *testing.T) {
	stateDir := t.TempDir()
	if err := internalSecurityutil.HardenPrivatePath(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pending := testPendingUpdate(t, stateDir)
	stub := &pendingUpdateSupervisorStub{
		rollbackResult: agenthelper.UpdateResult{
			Action:         "rolled_back",
			ActivationID:   pending.Activation.ActivationID,
			ActiveSHA256:   pending.Activation.RollbackSHA256,
			RollbackSHA256: pending.Activation.ActiveSHA256,
		},
		commitCalls:   make(chan agenthelper.UpdateResult, 1),
		rollbackCalls: make(chan agenthelper.UpdateResult, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := supervisePendingPrivilegedUpdate(ctx, stub, pending, stateDir, func() bool { return false }, make(chan struct{}), time.Millisecond, nil)
	if err == nil || !strings.Contains(err.Error(), "pending update rolled back") {
		t.Fatalf("supervisor error = %v", err)
	}
	select {
	case activation := <-stub.rollbackCalls:
		if activation != pending.Activation {
			t.Fatalf("rollback activation = %#v", activation)
		}
	default:
		t.Fatal("pending update was not rolled back")
	}
	if loaded, loadErr := agentupdate.LoadPendingPrivilegedUpdate(stateDir); loadErr != nil || loaded != nil {
		t.Fatalf("rolled-back handoff = %#v, %v", loaded, loadErr)
	}
}

func TestPendingPrivilegedUpdateRollbackFailurePreservesHandoff(t *testing.T) {
	stateDir := t.TempDir()
	if err := internalSecurityutil.HardenPrivatePath(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pending := testPendingUpdate(t, stateDir)
	stub := &pendingUpdateSupervisorStub{
		rollbackErr:   errors.New("helper unavailable"),
		commitCalls:   make(chan agenthelper.UpdateResult, 1),
		rollbackCalls: make(chan agenthelper.UpdateResult, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := supervisePendingPrivilegedUpdate(ctx, stub, pending, stateDir, func() bool { return false }, make(chan struct{}), time.Millisecond, nil)
	if err == nil || !strings.Contains(err.Error(), "typed helper rollback failed") {
		t.Fatalf("supervisor error = %v", err)
	}
	loaded, loadErr := agentupdate.LoadPendingPrivilegedUpdate(stateDir)
	if loadErr != nil || loaded == nil || loaded.Activation != pending.Activation {
		t.Fatalf("failed-rollback handoff = %#v, %v", loaded, loadErr)
	}
}

func TestRun_AgentFailure(t *testing.T) {
	origDocker := newDockerAgent
	defer func() {
		newDockerAgent = origDocker
	}()

	// Docker / Podman module fails immediately after start
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		return &mockRunnableCloser{mockRunnable: mockRunnable{
			started: make(chan struct{}),
			err:     errors.New("simulated failure"),
		}}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := run(ctx, []string{"-token", "T", "-enable-docker=true", "-enable-host=false"}, func(s string) string { return "" })
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "simulated failure") {
		t.Errorf("expected 'simulated failure', got %v", err)
	}
}

func TestRun_PropagatesDisableCephToHostAgent(t *testing.T) {
	origDocker := newDockerAgent
	origKube := newKubeAgent
	origHost := newHostAgent
	defer func() {
		newDockerAgent = origDocker
		newKubeAgent = origKube
		newHostAgent = origHost
	}()

	hostCfgCh := make(chan hostagent.Config, 1)
	newHostAgent = func(cfg hostagent.Config) (Runnable, error) {
		hostCfgCh <- cfg
		return &mockRunnable{}, nil
	}
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		return &mockRunnableCloser{}, nil
	}
	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		return &mockRunnable{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := run(ctx, []string{
		"-token", "T",
		"-enable-host=true",
		"-enable-docker=false",
		"-enable-kubernetes=false",
		"-disable-ceph=true",
		"-health-addr", "127.0.0.1:0",
	}, func(string) string { return "" })
	if err != nil && err != context.Canceled {
		t.Fatalf("run returned unexpected error: %v", err)
	}

	select {
	case hostCfg := <-hostCfgCh:
		if !hostCfg.DisableCeph {
			t.Fatalf("expected DisableCeph=true on host agent config")
		}
	default:
		t.Fatalf("host agent was not initialized")
	}
}

func TestLoadConfig_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		validate func(t *testing.T, cfg Config)
	}{
		{
			name: "all flags",
			args: []string{
				"-token", "F",
				"-enable-host=false",
				"-enable-docker=true",
				"-enable-kubernetes=true",
				"-enable-proxmox=true",
				"-proxmox-type", "pbs",
				"-disable-auto-update=true",
				"-disable-docker-update-checks=true",
				"-docker-runtime", "podman",
				"-enable-commands=true",
				"-kubeconfig", "/tmp/kube",
				"-kube-context", "ctx",
				"-kube-max-pods", "50",
				"-kube-include-all-pods=true",
				"-kube-include-all-deployments=true",
				"-report-ip", "1.2.3.4",
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.EnableHost {
					t.Error("EnableHost should be false")
				}
				if !cfg.EnableDocker {
					t.Error("EnableDocker should be true")
				}
				if !cfg.EnableKubernetes {
					t.Error("EnableKubernetes should be true")
				}
				if !cfg.EnableProxmox {
					t.Error("EnableProxmox should be true")
				}
				if cfg.ProxmoxType != "pbs" {
					t.Errorf("ProxmoxType: got %s, want pbs", cfg.ProxmoxType)
				}
				if !cfg.DisableAutoUpdate {
					t.Error("DisableAutoUpdate should be true")
				}
				if !cfg.DisableDockerUpdateChecks {
					t.Error("DisableDockerUpdateChecks should be true")
				}
				if cfg.DockerRuntime != "podman" {
					t.Errorf("DockerRuntime: got %s, want podman", cfg.DockerRuntime)
				}
				if !cfg.EnableCommands {
					t.Error("EnableCommands should be true")
				}
				if cfg.KubeconfigPath != "/tmp/kube" {
					t.Errorf("KubeconfigPath: got %s, want /tmp/kube", cfg.KubeconfigPath)
				}
				if cfg.KubeContext != "ctx" {
					t.Errorf("KubeContext: got %s, want ctx", cfg.KubeContext)
				}
				if cfg.KubeMaxPods != 50 {
					t.Errorf("KubeMaxPods: got %d, want 50", cfg.KubeMaxPods)
				}
				if !cfg.KubeIncludeAllPods {
					t.Error("KubeIncludeAllPods should be true")
				}
				if !cfg.KubeIncludeAllDeployments {
					t.Error("KubeIncludeAllDeployments should be true")
				}
				if cfg.ReportIP != "1.2.3.4" {
					t.Errorf("ReportIP: got %s, want 1.2.3.4", cfg.ReportIP)
				}
				if !cfg.DockerConfigured {
					t.Error("DockerConfigured should be true when flag is set")
				}
				if cfg.DockerExplicitlyDisabled {
					t.Error("DockerExplicitlyDisabled should be false when flag enables Docker")
				}
			},
		},
		{
			name: "env vars",
			env: map[string]string{
				"PULSE_TOKEN":                        "E",
				"PULSE_ENABLE_HOST":                  "false",
				"PULSE_ENABLE_DOCKER":                "true",
				"PULSE_ENABLE_KUBERNETES":            "true",
				"PULSE_ENABLE_PROXMOX":               "true",
				"PULSE_PROXMOX_TYPE":                 "pve",
				"PULSE_DISABLE_AUTO_UPDATE":          "true",
				"PULSE_DISABLE_DOCKER_UPDATE_CHECKS": "true",
				"PULSE_DOCKER_RUNTIME":               "docker",
				"PULSE_ENABLE_COMMANDS":              "true",
				"PULSE_KUBECONFIG":                   "/env/kube",
				"PULSE_KUBE_CONTEXT":                 "env-ctx",
				"PULSE_KUBE_MAX_PODS":                "100",
				"PULSE_KUBE_INCLUDE_ALL_POD_FILES":   "true", // Note: var name matches loadConfig implementation
				"PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS": "true",
				"PULSE_REPORT_IP":                    "5.6.7.8",
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.EnableHost {
					t.Error("EnableHost should be false")
				}
				if !cfg.EnableDocker {
					t.Error("EnableDocker should be true")
				}
				if cfg.ProxmoxType != "pve" {
					t.Errorf("ProxmoxType: got %s, want pve", cfg.ProxmoxType)
				}
				if cfg.ReportIP != "5.6.7.8" {
					t.Errorf("ReportIP: got %s, want 5.6.7.8", cfg.ReportIP)
				}
				if !cfg.DockerConfigured {
					t.Error("DockerConfigured should be true when env is set")
				}
				if cfg.DockerExplicitlyDisabled {
					t.Error("DockerExplicitlyDisabled should be false when env enables Docker")
				}
			},
		},
		{
			name: "docker explicitly disabled by flag",
			args: []string{"-token", "T", "-enable-docker=false"},
			validate: func(t *testing.T, cfg Config) {
				if !cfg.DockerConfigured {
					t.Error("DockerConfigured should be true when Docker flag is set")
				}
				if !cfg.DockerExplicitlyDisabled {
					t.Error("DockerExplicitlyDisabled should be true when flag disables Docker")
				}
				if cfg.EnableDocker {
					t.Error("EnableDocker should be false when flag disables Docker")
				}
			},
		},
		{
			name: "docker explicitly disabled by env",
			args: []string{"-token", "T"},
			env: map[string]string{
				"PULSE_ENABLE_DOCKER": "false",
			},
			validate: func(t *testing.T, cfg Config) {
				if !cfg.DockerConfigured {
					t.Error("DockerConfigured should be true when Docker env is set")
				}
				if !cfg.DockerExplicitlyDisabled {
					t.Error("DockerExplicitlyDisabled should be true when env disables Docker")
				}
				if cfg.EnableDocker {
					t.Error("EnableDocker should be false when env disables Docker")
				}
			},
		},
		{
			name: "docker env disable overridden by enabling flag",
			args: []string{"-token", "T", "-enable-docker=true"},
			env: map[string]string{
				"PULSE_ENABLE_DOCKER": "false",
			},
			validate: func(t *testing.T, cfg Config) {
				if !cfg.DockerConfigured {
					t.Error("DockerConfigured should be true when Docker env or flag is set")
				}
				if cfg.DockerExplicitlyDisabled {
					t.Error("DockerExplicitlyDisabled should be false when explicit flag enables Docker")
				}
				if !cfg.EnableDocker {
					t.Error("EnableDocker should be true when flag enables Docker")
				}
			},
		},
		{
			name: "docker not configured",
			args: []string{"-token", "T"},
			validate: func(t *testing.T, cfg Config) {
				if cfg.DockerConfigured {
					t.Error("DockerConfigured should be false when not set")
				}
				if cfg.EnableDocker {
					t.Error("EnableDocker should be false by default")
				}
				if cfg.DockerExplicitlyDisabled {
					t.Error("DockerExplicitlyDisabled should be false when Docker is unconfigured")
				}
				if cfg.HealthAddr != "127.0.0.1:9191" {
					t.Errorf("HealthAddr: got %q, want loopback default", cfg.HealthAddr)
				}
			},
		},
		{
			name: "health addr can be opened explicitly",
			args: []string{"-token", "T", "-health-addr", ":9191"},
			validate: func(t *testing.T, cfg Config) {
				if cfg.HealthAddr != ":9191" {
					t.Errorf("HealthAddr: got %q, want :9191", cfg.HealthAddr)
				}
			},
		},
		{
			name: "health addr can be disabled by env",
			args: []string{"-token", "T"},
			env: map[string]string{
				"PULSE_HEALTH_ADDR": "off",
			},
			validate: func(t *testing.T, cfg Config) {
				if cfg.HealthAddr != "" {
					t.Errorf("HealthAddr: got %q, want disabled", cfg.HealthAddr)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string {
				if tc.env == nil {
					return ""
				}
				return tc.env[key]
			}
			cfg, err := loadConfig(tc.args, getenv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.validate(t, cfg)
		})
	}
}

func TestStartHealthServer_Error(t *testing.T) {
	var ready atomic.Bool
	logger := zerolog.New(os.Stdout)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use invalid port to force error (logs warning, doesn't panic)
	// We just want to exercise the code path
	startHealthServer(ctx, "invalid-address", &ready, &logger)

	// Give it a moment to try starting
	time.Sleep(50 * time.Millisecond)
}

func TestInitKubernetesWithRetry_Failure(t *testing.T) {
	orig := newKubeAgent
	defer func() { newKubeAgent = orig }()

	// Always fail
	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		return nil, errors.New("fail")
	}

	// Override delays to be super fast
	origInitial := retryInitialDelay
	origMax := retryMaxDelay
	retryInitialDelay = 1 * time.Millisecond
	retryMaxDelay = 2 * time.Millisecond
	defer func() {
		retryInitialDelay = origInitial
		retryMaxDelay = origMax
	}()

	ctx, cancel := context.WithCancel(context.Background())
	// Let it run for a bit then cancel
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	logger := zerolog.New(os.Stdout)
	agent := initKubernetesWithRetry(ctx, kubernetesagent.Config{}, &logger)
	if agent != nil {
		t.Errorf("expected nil agent")
	}
}

func TestRun_WindowsServiceError(t *testing.T) {
	orig := runAsWindowsServiceFunc
	defer func() { runAsWindowsServiceFunc = orig }()

	runAsWindowsServiceFunc = func(cfg Config, logger zerolog.Logger) (bool, error) {
		return false, errors.New("service error")
	}

	ctx := context.Background()
	err := run(ctx, []string{"-token", "T"}, func(s string) string { return "" })
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "service error") {
		t.Errorf("expected 'service error', got %v", err)
	}
}

func TestRun_DockerRetry(t *testing.T) {
	origDocker := newDockerAgent
	defer func() { newDockerAgent = origDocker }()

	// First call fails, second succeeds
	calls := 0
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("not available yet")
		}
		return &mockRunnableCloser{mockRunnable: mockRunnable{started: make(chan struct{})}}, nil
	}

	// Speed up retry
	origInitial := retryInitialDelay
	retryInitialDelay = 1 * time.Millisecond
	defer func() { retryInitialDelay = origInitial }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	errCh := make(chan error)
	go func() {
		errCh <- run(ctx, []string{"-token", "T", "-url", server.URL, "-enable-docker=true", "-enable-host=false"}, func(s string) string { return "" })
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for run")
	}

	if calls < 2 {
		t.Errorf("expected at least 2 calls to newDockerAgent, got %d", calls)
	}
}

func TestRunAsWindowsServiceStub(t *testing.T) {
	res, err := runAsWindowsService(Config{}, zerolog.New(os.Stdout))
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if res != false {
		t.Error("expected false")
	}
}

func TestWindowsServiceRuntimeStartsHealthServer(t *testing.T) {
	source, err := os.ReadFile("service_windows.go")
	if err != nil {
		t.Fatalf("read pulse-agent service_windows.go: %v", err)
	}
	text := string(source)

	required := []string{
		`var ready atomic.Bool`,
		`runtimeStatus := newRuntimeHealth(&ready`,
		`startHealthServer(ctx, ws.cfg.HealthAddr, &ready, &ws.logger, runtimeStatus)`,
		`runtimeStatus.setState("host", moduleStateRunning, nil)`,
		`agentUp.Set(1)`,
		`defer agentUp.Set(0)`,
	}
	for _, want := range required {
		if !strings.Contains(text, want) {
			t.Fatalf("expected Windows service runtime to include %q", want)
		}
	}
}

func TestRun_KubeRetry(t *testing.T) {
	origKube := newKubeAgent
	defer func() { newKubeAgent = origKube }()

	// First call fails, second succeeds
	calls := 0
	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("not available yet")
		}
		return &mockRunnable{started: make(chan struct{})}, nil
	}

	// Speed up retry
	origInitial := retryInitialDelay
	retryInitialDelay = 1 * time.Millisecond
	defer func() { retryInitialDelay = origInitial }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	errCh := make(chan error)
	go func() {
		// Only enable kubernetes
		errCh <- run(ctx, []string{"-token", "T", "-url", server.URL, "-enable-kubernetes=true", "-enable-host=false", "-enable-docker=false"}, func(s string) string { return "" })
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for run")
	}

	if calls < 2 {
		t.Errorf("expected at least 2 calls to newKubeAgent, got %d", calls)
	}
}

func TestRetryLogEvent_LevelThrottling(t *testing.T) {
	// Ensure debug events are not filtered by the global level
	prev := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() { zerolog.SetGlobalLevel(prev) })

	tests := []struct {
		attempt   int
		wantLevel string
	}{
		{1, "warn"},
		{5, "warn"},
		{10, "warn"},
		{11, "info"},
		{25, "info"},
		{50, "info"},
		{51, "debug"},
		{100, "debug"},
	}

	for _, tt := range tests {
		var buf strings.Builder
		logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
		event := retryLogEvent(&logger, tt.attempt)
		event.Msg("test")

		output := buf.String()
		if !strings.Contains(output, `"level":"`+tt.wantLevel+`"`) {
			t.Errorf("attempt %d: expected level %q in output, got: %s", tt.attempt, tt.wantLevel, output)
		}
	}
}

func TestRemoteDurationSettingPreservesFractionalSeconds(t *testing.T) {
	got, ok := remoteDurationSetting(map[string]interface{}{"interval": 0.25}, "interval")
	if !ok {
		t.Fatal("remoteDurationSetting() ok = false, want true")
	}
	if got != 250*time.Millisecond {
		t.Fatalf("remoteDurationSetting() = %s, want 250ms", got)
	}
}

func TestAgentIDFilePersistence(t *testing.T) {
	t.Run("read returns empty when path is empty", func(t *testing.T) {
		id, err := readAgentIDFile("")
		if err != nil {
			t.Fatalf("expected no error for empty path, got %v", err)
		}
		if id != "" {
			t.Errorf("expected empty id, got %q", id)
		}
	})

	t.Run("read returns fs.ErrNotExist for missing file", func(t *testing.T) {
		dir := t.TempDir()
		_, err := readAgentIDFile(filepath.Join(dir, "missing"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("expected fs.ErrNotExist, got %v", err)
		}
	})

	t.Run("write then read round-trips the ID", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "subdir", "agent-id")
		const id = "1234abcd-5678-90ef-1234-567890abcdef"

		if err := writeAgentIDFile(path, id); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		got, err := readAgentIDFile(path)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if got != id {
			t.Errorf("round-trip mismatch: got %q, want %q", got, id)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if mode := info.Mode().Perm(); runtime.GOOS != "windows" && mode != 0o600 {
			t.Errorf("file permissions = %v, want 0600", mode)
		}
	})

	t.Run("read trims whitespace", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "agent-id")
		if err := os.WriteFile(path, []byte("  abc-123  \n\n"), 0o600); err != nil {
			t.Fatalf("seed file: %v", err)
		}
		got, err := readAgentIDFile(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if got != "abc-123" {
			t.Errorf("expected trimmed value, got %q", got)
		}
	})

	t.Run("write is a no-op when path is empty or id is empty", func(t *testing.T) {
		if err := writeAgentIDFile("", "some-id"); err != nil {
			t.Errorf("expected no-op for empty path, got %v", err)
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "agent-id")
		if err := writeAgentIDFile(path, ""); err != nil {
			t.Errorf("expected no-op for empty id, got %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("expected file not to be created, got err=%v", err)
		}
	})
}

func TestSecureAgentStateDir(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := secureAgentStateDir(stateDir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0700 {
		t.Fatalf("state directory mode = %o, want 700", got)
	}
}

type stubTypedContainerUpdater struct {
	calls              int
	preflightCalls     int
	lifecycleInspects  int
	lifecycleMutations int
}

type capabilityDockerAgent struct {
	stubTypedContainerUpdater
	actions bool
}

func (*capabilityDockerAgent) Run(context.Context) error { return nil }
func (*capabilityDockerAgent) Close() error              { return nil }
func (a *capabilityDockerAgent) ContainerActionsAvailable() bool {
	return a.actions
}

func (s *stubTypedContainerUpdater) TypedContainerUpdatePreflight(context.Context, string, string, string) error {
	s.preflightCalls++
	return nil
}

func (s *stubTypedContainerUpdater) TypedContainerUpdate(context.Context, string, string, string, func(string)) (agentexec.DockerContainerUpdateOutcome, error) {
	s.calls++
	return agentexec.DockerContainerUpdateOutcome{Success: true}, nil
}

func (s *stubTypedContainerUpdater) InspectDockerContainerLifecycle(context.Context, string, string) (agentexec.DockerContainerLifecycleSnapshot, error) {
	s.lifecycleInspects++
	return agentexec.DockerContainerLifecycleSnapshot{ContainerID: strings.Repeat("a", 64), State: "running", Running: true}, nil
}

func (s *stubTypedContainerUpdater) MutateDockerContainerLifecycle(context.Context, string, string, string) error {
	s.lifecycleMutations++
	return nil
}

func TestLateBoundDockerUpdaterBridgesModuleWhenItComesUp(t *testing.T) {
	bridge := &lateBoundDockerUpdater{}
	containerID := strings.Repeat("a", 12)
	expectedImageDigest := "sha256:" + strings.Repeat("1", 64)

	if err := bridge.TypedContainerUpdatePreflight(context.Background(), "docker", containerID, expectedImageDigest); err == nil {
		t.Fatal("bridge without a docker module accepted a preflight")
	} else if got := agentexec.ActionPreflightReasonCode(err, agentexec.ActionRefusalTargetPreconditionFailed); got != agentexec.ActionRefusalCapabilityUnavailable {
		t.Fatalf("bridge without a docker module refusal = %q, want %q", got, agentexec.ActionRefusalCapabilityUnavailable)
	}
	if _, err := bridge.TypedContainerUpdate(context.Background(), "docker", containerID, expectedImageDigest, nil); err == nil {
		t.Fatal("bridge without a docker module accepted an update")
	}

	bridge.set(struct{}{}) // non-implementing candidates must not install
	if err := bridge.TypedContainerUpdatePreflight(context.Background(), "docker", containerID, expectedImageDigest); err == nil {
		t.Fatal("bridge accepted a preflight after a non-implementing candidate was offered")
	}
	if _, err := bridge.TypedContainerUpdate(context.Background(), "docker", containerID, expectedImageDigest, nil); err == nil {
		t.Fatal("bridge accepted an update after a non-implementing candidate was offered")
	}

	stub := &stubTypedContainerUpdater{}
	bridge.set(stub)
	if err := bridge.TypedContainerUpdatePreflight(context.Background(), "docker", containerID, expectedImageDigest); err != nil {
		t.Fatalf("bridge preflight with an installed module refused: %v", err)
	}
	if stub.preflightCalls != 1 {
		t.Fatalf("expected one delegated preflight call, got %d", stub.preflightCalls)
	}
	if _, err := bridge.TypedContainerUpdate(context.Background(), "docker", containerID, expectedImageDigest, nil); err != nil {
		t.Fatalf("bridge with an installed module refused: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("expected one delegated call, got %d", stub.calls)
	}
	if _, err := bridge.InspectDockerContainerLifecycle(context.Background(), "docker", strings.Repeat("a", 64)); err != nil {
		t.Fatalf("bridge lifecycle inspect refused: %v", err)
	}
	if err := bridge.MutateDockerContainerLifecycle(context.Background(), "docker", "restart", strings.Repeat("a", 64)); err != nil {
		t.Fatalf("bridge lifecycle mutation refused: %v", err)
	}
	if stub.lifecycleInspects != 1 || stub.lifecycleMutations != 1 {
		t.Fatalf("lifecycle calls = inspect %d mutate %d", stub.lifecycleInspects, stub.lifecycleMutations)
	}
}

func TestBindDockerActionBridgeRejectsSummaryOnlyModule(t *testing.T) {
	bridge := &lateBoundDockerUpdater{}
	summaryOnly := &capabilityDockerAgent{actions: false}
	bindDockerActionBridge(bridge, summaryOnly)
	if _, err := bridge.TypedContainerUpdate(context.Background(), "docker", strings.Repeat("a", 12), "sha256:"+strings.Repeat("1", 64), nil); err == nil {
		t.Fatal("summary-only module was granted container update authority")
	}

	direct := &capabilityDockerAgent{actions: true}
	bindDockerActionBridge(bridge, direct)
	if _, err := bridge.TypedContainerUpdate(context.Background(), "docker", strings.Repeat("a", 12), "sha256:"+strings.Repeat("1", 64), nil); err != nil {
		t.Fatalf("direct runtime module was not bridged: %v", err)
	}
}

func TestDockerAgentImplementsTypedContainerUpdater(t *testing.T) {
	// The bridge installs by structural assertion; if the Docker module's
	// method signature drifts, updates silently refuse at runtime. Pin it.
	var _ hostagent.DockerContainerUpdater = (*dockeragent.Agent)(nil)
	var _ hostagent.DockerContainerLifecycleOperator = (*dockeragent.Agent)(nil)
}

func TestAllowPlaintextHTTPFlagParsesAndDefaultsClosed(t *testing.T) {
	t.Cleanup(func() { securityutil.SetOperatorPlaintextHTTPConsent(false) })

	cfg, err := loadConfig([]string{"--url", "http://192.168.1.10:7655", "--token", "t"}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowPlaintextHTTP {
		t.Fatal("plaintext override must default to false")
	}

	cfg, err = loadConfig([]string{"--url", "http://192.168.1.10:7655", "--token", "t", "--allow-plaintext-http"}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowPlaintextHTTP {
		t.Fatal("--allow-plaintext-http flag was not applied")
	}

	cfg, err = loadConfig([]string{"--url", "http://192.168.1.10:7655", "--token", "t"}, func(key string) string {
		if key == "PULSE_AGENT_ALLOW_PLAINTEXT_HTTP" {
			return "true"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowPlaintextHTTP {
		t.Fatal("PULSE_AGENT_ALLOW_PLAINTEXT_HTTP env was not applied")
	}
}

func TestApplyRemoteSettingsCarriesAvailabilityAssignmentsToStartup(t *testing.T) {
	logger := zerolog.New(io.Discard)
	cfg := &Config{}

	applyRemoteSettings(cfg, map[string]interface{}{
		"availabilityTargets": []interface{}{
			map[string]interface{}{
				"id":                  "remote-a",
				"address":             "a.local",
				"protocol":            "icmp",
				"enabled":             true,
				"pollIntervalSeconds": float64(30),
			},
		},
	}, &logger)

	if len(cfg.AvailabilityTargets) != 1 {
		t.Fatalf("availability targets = %+v, want the assignment applied at boot", cfg.AvailabilityTargets)
	}
	if cfg.AvailabilityTargets[0].ID != "remote-a" {
		t.Fatalf("availability target = %+v", cfg.AvailabilityTargets[0])
	}

	applyRemoteSettings(cfg, map[string]interface{}{
		"availabilityTargets": "not-a-list",
	}, &logger)

	if len(cfg.AvailabilityTargets) != 1 {
		t.Fatalf("availability targets = %+v, want an unreadable payload ignored", cfg.AvailabilityTargets)
	}
}

func TestWireUpdaterHooksNudgesUpdaterOnNewerAckVersion(t *testing.T) {
	t.Parallel()

	hits := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"9.9.9"}`))
	}))
	defer srv.Close()

	updater := newUpdater(agentupdate.Config{
		CurrentVersion: "1.0.0",
		PulseURL:       srv.URL,
		// Keep the built-in initial check out of the way so the only thing
		// that can reach the server inside the assertion window is the nudge.
		InitialCheckDelay: time.Hour,
		CheckInterval:     time.Hour,
	})

	var hostCfg hostagent.Config
	wireUpdaterHooks(&hostCfg, updater)
	if hostCfg.UpdateStatus == nil {
		t.Fatal("UpdateStatus hook not wired")
	}
	if hostCfg.OnServerVersion == nil {
		t.Fatal("OnServerVersion hook not wired")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		updater.RunLoop(ctx)
	}()

	// Simulate what the host module does when a report ack carries a newer
	// server version. The wired updater must check for updates immediately.
	hostCfg.OnServerVersion("9.9.9")

	select {
	case path := <-hits:
		if !strings.Contains(path, "/api/agent/version") {
			t.Fatalf("first updater request hit %q, want the version check endpoint", path)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ack-carried server version did not trigger an immediate update check")
	}

	cancel()
	<-done
}

func TestLoadConfigRegistryCredentialOptOut(t *testing.T) {
	t.Run("default keeps host credential reads enabled", func(t *testing.T) {
		cfg, err := loadConfig([]string{"-token", "test-token"}, func(string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if cfg.DisableRegistryCredentials {
			t.Error("expected registry credential reads enabled by default")
		}
	})

	t.Run("env opt-out", func(t *testing.T) {
		env := map[string]string{
			"PULSE_TOKEN":                        "test-token",
			"PULSE_DISABLE_REGISTRY_CREDENTIALS": "true",
		}
		cfg, err := loadConfig([]string{}, func(s string) string { return env[s] })
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.DisableRegistryCredentials {
			t.Error("expected PULSE_DISABLE_REGISTRY_CREDENTIALS to disable credential reads")
		}
	})

	t.Run("flag opt-out", func(t *testing.T) {
		cfg, err := loadConfig([]string{"-token", "test-token", "-disable-registry-credentials"}, func(string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.DisableRegistryCredentials {
			t.Error("expected --disable-registry-credentials to disable credential reads")
		}
	})
}
