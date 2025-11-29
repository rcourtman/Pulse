package api

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestValidateSystemSettings provides comprehensive coverage of all validation paths
// in the validateSystemSettings function using table-driven tests.
func TestValidateSystemSettings(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expectError bool
		errorText   string // key term to check with strings.Contains, empty means don't check
	}{
		// =================================================================
		// pvePollingInterval validation
		// =================================================================
		{
			name:        "pvePollingInterval: valid minimum (10)",
			input:       map[string]interface{}{"pvePollingInterval": float64(10)},
			expectError: false,
		},
		{
			name:        "pvePollingInterval: valid maximum (3600)",
			input:       map[string]interface{}{"pvePollingInterval": float64(3600)},
			expectError: false,
		},
		{
			name:        "pvePollingInterval: valid middle value (60)",
			input:       map[string]interface{}{"pvePollingInterval": float64(60)},
			expectError: false,
		},
		{
			name:        "pvePollingInterval: zero is invalid",
			input:       map[string]interface{}{"pvePollingInterval": float64(0)},
			expectError: true,
			errorText:   "positive",
		},
		{
			name:        "pvePollingInterval: negative is invalid",
			input:       map[string]interface{}{"pvePollingInterval": float64(-1)},
			expectError: true,
			errorText:   "positive",
		},
		{
			name:        "pvePollingInterval: below minimum (9)",
			input:       map[string]interface{}{"pvePollingInterval": float64(9)},
			expectError: true,
			errorText:   "10 seconds",
		},
		{
			name:        "pvePollingInterval: above maximum (3601)",
			input:       map[string]interface{}{"pvePollingInterval": float64(3601)},
			expectError: true,
			errorText:   "3600",
		},
		{
			name:        "pvePollingInterval: string instead of number",
			input:       map[string]interface{}{"pvePollingInterval": "60"},
			expectError: true,
			errorText:   "number",
		},
		{
			name:        "pvePollingInterval: boolean instead of number",
			input:       map[string]interface{}{"pvePollingInterval": true},
			expectError: true,
			errorText:   "number",
		},

		// =================================================================
		// pbsPollingInterval validation (same rules as PVE)
		// =================================================================
		{
			name:        "pbsPollingInterval: valid minimum (10)",
			input:       map[string]interface{}{"pbsPollingInterval": float64(10)},
			expectError: false,
		},
		{
			name:        "pbsPollingInterval: valid maximum (3600)",
			input:       map[string]interface{}{"pbsPollingInterval": float64(3600)},
			expectError: false,
		},
		{
			name:        "pbsPollingInterval: zero is invalid",
			input:       map[string]interface{}{"pbsPollingInterval": float64(0)},
			expectError: true,
			errorText:   "positive",
		},
		{
			name:        "pbsPollingInterval: below minimum (9)",
			input:       map[string]interface{}{"pbsPollingInterval": float64(9)},
			expectError: true,
			errorText:   "10 seconds",
		},
		{
			name:        "pbsPollingInterval: above maximum (3601)",
			input:       map[string]interface{}{"pbsPollingInterval": float64(3601)},
			expectError: true,
			errorText:   "3600",
		},
		{
			name:        "pbsPollingInterval: string instead of number",
			input:       map[string]interface{}{"pbsPollingInterval": "60"},
			expectError: true,
			errorText:   "number",
		},

		// =================================================================
		// pmgPollingInterval validation (same rules as PVE)
		// =================================================================
		{
			name:        "pmgPollingInterval: valid minimum (10)",
			input:       map[string]interface{}{"pmgPollingInterval": float64(10)},
			expectError: false,
		},
		{
			name:        "pmgPollingInterval: valid maximum (3600)",
			input:       map[string]interface{}{"pmgPollingInterval": float64(3600)},
			expectError: false,
		},
		{
			name:        "pmgPollingInterval: zero is invalid",
			input:       map[string]interface{}{"pmgPollingInterval": float64(0)},
			expectError: true,
			errorText:   "positive",
		},
		{
			name:        "pmgPollingInterval: below minimum (9)",
			input:       map[string]interface{}{"pmgPollingInterval": float64(9)},
			expectError: true,
			errorText:   "10 seconds",
		},
		{
			name:        "pmgPollingInterval: above maximum (3601)",
			input:       map[string]interface{}{"pmgPollingInterval": float64(3601)},
			expectError: true,
			errorText:   "3600",
		},
		{
			name:        "pmgPollingInterval: string instead of number",
			input:       map[string]interface{}{"pmgPollingInterval": "60"},
			expectError: true,
			errorText:   "number",
		},

		// =================================================================
		// backupPollingInterval validation
		// NOTE: backupPollingInterval allows 0 (to disable backup polling),
		// unlike PVE/PBS/PMG intervals which must be positive.
		// This is intentional - see lines 172-185 in system_settings.go.
		// =================================================================
		{
			name:        "backupPollingInterval: zero is valid (disables polling)",
			input:       map[string]interface{}{"backupPollingInterval": float64(0)},
			expectError: false,
		},
		{
			name:        "backupPollingInterval: valid minimum positive (10)",
			input:       map[string]interface{}{"backupPollingInterval": float64(10)},
			expectError: false,
		},
		{
			name:        "backupPollingInterval: valid maximum (604800)",
			input:       map[string]interface{}{"backupPollingInterval": float64(604800)},
			expectError: false,
		},
		{
			name:        "backupPollingInterval: negative is invalid",
			input:       map[string]interface{}{"backupPollingInterval": float64(-1)},
			expectError: true,
			errorText:   "negative",
		},
		{
			name:        "backupPollingInterval: below minimum if positive (9)",
			input:       map[string]interface{}{"backupPollingInterval": float64(9)},
			expectError: true,
			errorText:   "10 seconds",
		},
		{
			name:        "backupPollingInterval: above maximum (604801)",
			input:       map[string]interface{}{"backupPollingInterval": float64(604801)},
			expectError: true,
			errorText:   "604800",
		},
		{
			name:        "backupPollingInterval: string instead of number",
			input:       map[string]interface{}{"backupPollingInterval": "60"},
			expectError: true,
			errorText:   "number",
		},

		// =================================================================
		// Boolean field type validation
		// =================================================================
		{
			name:        "autoUpdateEnabled: true is valid",
			input:       map[string]interface{}{"autoUpdateEnabled": true},
			expectError: false,
		},
		{
			name:        "autoUpdateEnabled: false is valid",
			input:       map[string]interface{}{"autoUpdateEnabled": false},
			expectError: false,
		},
		{
			name:        "autoUpdateEnabled: string instead of bool",
			input:       map[string]interface{}{"autoUpdateEnabled": "true"},
			expectError: true,
			errorText:   "boolean",
		},
		{
			name:        "autoUpdateEnabled: number instead of bool",
			input:       map[string]interface{}{"autoUpdateEnabled": float64(1)},
			expectError: true,
			errorText:   "boolean",
		},
		{
			name:        "discoveryEnabled: true is valid",
			input:       map[string]interface{}{"discoveryEnabled": true},
			expectError: false,
		},
		{
			name:        "discoveryEnabled: false is valid",
			input:       map[string]interface{}{"discoveryEnabled": false},
			expectError: false,
		},
		{
			name:        "discoveryEnabled: string instead of bool",
			input:       map[string]interface{}{"discoveryEnabled": "true"},
			expectError: true,
			errorText:   "boolean",
		},
		{
			name:        "allowEmbedding: true is valid",
			input:       map[string]interface{}{"allowEmbedding": true},
			expectError: false,
		},
		{
			name:        "allowEmbedding: false is valid",
			input:       map[string]interface{}{"allowEmbedding": false},
			expectError: false,
		},
		{
			name:        "allowEmbedding: string instead of bool",
			input:       map[string]interface{}{"allowEmbedding": "false"},
			expectError: true,
			errorText:   "boolean",
		},
		{
			name:        "backupPollingEnabled: true is valid",
			input:       map[string]interface{}{"backupPollingEnabled": true},
			expectError: false,
		},
		{
			name:        "backupPollingEnabled: false is valid",
			input:       map[string]interface{}{"backupPollingEnabled": false},
			expectError: false,
		},
		{
			name:        "backupPollingEnabled: string instead of bool",
			input:       map[string]interface{}{"backupPollingEnabled": "true"},
			expectError: true,
			errorText:   "boolean",
		},
		{
			name:        "temperatureMonitoringEnabled: true is valid",
			input:       map[string]interface{}{"temperatureMonitoringEnabled": true},
			expectError: false,
		},
		{
			name:        "temperatureMonitoringEnabled: false is valid",
			input:       map[string]interface{}{"temperatureMonitoringEnabled": false},
			expectError: false,
		},
		{
			name:        "temperatureMonitoringEnabled: string instead of bool",
			input:       map[string]interface{}{"temperatureMonitoringEnabled": "true"},
			expectError: true,
			errorText:   "boolean",
		},

		// =================================================================
		// autoUpdateCheckInterval validation
		// NOTE: Like backupPollingInterval, this allows 0 to disable.
		// =================================================================
		{
			name:        "autoUpdateCheckInterval: zero is valid (disables check)",
			input:       map[string]interface{}{"autoUpdateCheckInterval": float64(0)},
			expectError: false,
		},
		{
			name:        "autoUpdateCheckInterval: valid minimum positive (1)",
			input:       map[string]interface{}{"autoUpdateCheckInterval": float64(1)},
			expectError: false,
		},
		{
			name:        "autoUpdateCheckInterval: valid maximum (168)",
			input:       map[string]interface{}{"autoUpdateCheckInterval": float64(168)},
			expectError: false,
		},
		{
			name:        "autoUpdateCheckInterval: negative is invalid",
			input:       map[string]interface{}{"autoUpdateCheckInterval": float64(-1)},
			expectError: true,
			errorText:   "negative",
		},
		{
			name:        "autoUpdateCheckInterval: below minimum if positive (0.5)",
			input:       map[string]interface{}{"autoUpdateCheckInterval": float64(0.5)},
			expectError: true,
			errorText:   "1 hour",
		},
		{
			name:        "autoUpdateCheckInterval: above maximum (169)",
			input:       map[string]interface{}{"autoUpdateCheckInterval": float64(169)},
			expectError: true,
			errorText:   "168",
		},
		{
			name:        "autoUpdateCheckInterval: string instead of number",
			input:       map[string]interface{}{"autoUpdateCheckInterval": "24"},
			expectError: true,
			errorText:   "number",
		},

		// =================================================================
		// discoveryConfig validation - field name variants (camelCase and snake_case)
		// =================================================================
		{
			name:        "discoveryConfig: camelCase field name is valid",
			input:       map[string]interface{}{"discoveryConfig": map[string]interface{}{"maxHostsPerScan": float64(100)}},
			expectError: false,
		},
		{
			name:        "discoveryConfig: snake_case field name is valid",
			input:       map[string]interface{}{"discovery_config": map[string]interface{}{"max_hosts_per_scan": float64(100)}},
			expectError: false,
		},
		{
			name:        "discoveryConfig: nil/null is invalid (must be object)",
			input:       map[string]interface{}{"discoveryConfig": nil},
			expectError: true,
			errorText:   "object",
		},
		{
			name:        "discoveryConfig: string instead of object",
			input:       map[string]interface{}{"discoveryConfig": "invalid"},
			expectError: true,
			errorText:   "object",
		},
		{
			name:        "discoveryConfig: array instead of object",
			input:       map[string]interface{}{"discoveryConfig": []interface{}{}},
			expectError: true,
			errorText:   "object",
		},
		{
			name:        "discoveryConfig: empty object is valid",
			input:       map[string]interface{}{"discoveryConfig": map[string]interface{}{}},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.environment_override validation
		// =================================================================
		{
			name: "discoveryConfig.environment_override: empty string is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": ""},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.environment_override: 'auto' is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": "auto"},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.environment_override: 'native' is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": "native"},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.environment_override: 'docker_host' is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": "docker_host"},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.environment_override: 'docker_bridge' is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": "docker_bridge"},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.environment_override: 'lxc_privileged' is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": "lxc_privileged"},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.environment_override: 'lxc_unprivileged' is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": "lxc_unprivileged"},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.environment_override: invalid value",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": "invalid_env"},
			},
			expectError: true,
			errorText:   "invalid discovery environment",
		},
		{
			name: "discoveryConfig.environment_override: number instead of string",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environment_override": float64(123)},
			},
			expectError: true,
			errorText:   "string",
		},
		{
			name: "discoveryConfig.environmentOverride: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"environmentOverride": "auto"},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.subnet_allowlist validation
		// =================================================================
		{
			name: "discoveryConfig.subnet_allowlist: empty array is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_allowlist": []interface{}{}},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.subnet_allowlist: valid CIDR",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_allowlist": []interface{}{"192.168.1.0/24"}},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.subnet_allowlist: multiple valid CIDRs",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_allowlist": []interface{}{"192.168.1.0/24", "10.0.0.0/8"}},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.subnet_allowlist: invalid CIDR format",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_allowlist": []interface{}{"192.168.1.0"}},
			},
			expectError: true,
			errorText:   "invalid CIDR",
		},
		{
			name: "discoveryConfig.subnet_allowlist: invalid CIDR in array",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_allowlist": []interface{}{"192.168.1.0/24", "not-a-cidr"}},
			},
			expectError: true,
			errorText:   "invalid CIDR",
		},
		{
			name: "discoveryConfig.subnet_allowlist: number in array instead of string",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_allowlist": []interface{}{float64(123)}},
			},
			expectError: true,
			errorText:   "string",
		},
		{
			name: "discoveryConfig.subnet_allowlist: not an array",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_allowlist": "192.168.1.0/24"},
			},
			expectError: true,
			errorText:   "array",
		},
		{
			name: "discoveryConfig.subnetAllowlist: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnetAllowlist": []interface{}{"192.168.1.0/24"}},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.subnet_blocklist validation
		// =================================================================
		{
			name: "discoveryConfig.subnet_blocklist: empty array is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_blocklist": []interface{}{}},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.subnet_blocklist: valid CIDR",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_blocklist": []interface{}{"169.254.0.0/16"}},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.subnet_blocklist: multiple valid CIDRs",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_blocklist": []interface{}{"169.254.0.0/16", "127.0.0.0/8"}},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.subnet_blocklist: invalid CIDR format",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_blocklist": []interface{}{"169.254.0.0"}},
			},
			expectError: true,
			errorText:   "invalid CIDR",
		},
		{
			name: "discoveryConfig.subnet_blocklist: number in array instead of string",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_blocklist": []interface{}{float64(456)}},
			},
			expectError: true,
			errorText:   "string",
		},
		{
			name: "discoveryConfig.subnet_blocklist: not an array",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnet_blocklist": "169.254.0.0/16"},
			},
			expectError: true,
			errorText:   "array",
		},
		{
			name: "discoveryConfig.subnetBlocklist: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"subnetBlocklist": []interface{}{"169.254.0.0/16"}},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.max_hosts_per_scan validation
		// =================================================================
		{
			name: "discoveryConfig.max_hosts_per_scan: valid positive value",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_hosts_per_scan": float64(100)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.max_hosts_per_scan: large value",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_hosts_per_scan": float64(10000)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.max_hosts_per_scan: zero is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_hosts_per_scan": float64(0)},
			},
			expectError: true,
			errorText:   "greater than zero",
		},
		{
			name: "discoveryConfig.max_hosts_per_scan: negative is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_hosts_per_scan": float64(-1)},
			},
			expectError: true,
			errorText:   "greater than zero",
		},
		{
			name: "discoveryConfig.max_hosts_per_scan: string instead of number",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_hosts_per_scan": "100"},
			},
			expectError: true,
			errorText:   "number",
		},
		{
			name: "discoveryConfig.maxHostsPerScan: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"maxHostsPerScan": float64(100)},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.max_concurrent validation
		// =================================================================
		{
			name: "discoveryConfig.max_concurrent: valid minimum (1)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_concurrent": float64(1)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.max_concurrent: valid maximum (1000)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_concurrent": float64(1000)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.max_concurrent: valid middle value (50)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_concurrent": float64(50)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.max_concurrent: zero is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_concurrent": float64(0)},
			},
			expectError: true,
			errorText:   "between 1 and 1000",
		},
		{
			name: "discoveryConfig.max_concurrent: negative is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_concurrent": float64(-1)},
			},
			expectError: true,
			errorText:   "between 1 and 1000",
		},
		{
			name: "discoveryConfig.max_concurrent: above maximum (1001)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_concurrent": float64(1001)},
			},
			expectError: true,
			errorText:   "between 1 and 1000",
		},
		{
			name: "discoveryConfig.max_concurrent: string instead of number",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"max_concurrent": "50"},
			},
			expectError: true,
			errorText:   "number",
		},
		{
			name: "discoveryConfig.maxConcurrent: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"maxConcurrent": float64(50)},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.enable_reverse_dns validation
		// =================================================================
		{
			name: "discoveryConfig.enable_reverse_dns: true is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"enable_reverse_dns": true},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.enable_reverse_dns: false is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"enable_reverse_dns": false},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.enable_reverse_dns: string instead of bool",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"enable_reverse_dns": "true"},
			},
			expectError: true,
			errorText:   "boolean",
		},
		{
			name: "discoveryConfig.enableReverseDns: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"enableReverseDns": true},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.scan_gateways validation
		// =================================================================
		{
			name: "discoveryConfig.scan_gateways: true is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"scan_gateways": true},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.scan_gateways: false is valid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"scan_gateways": false},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.scan_gateways: string instead of bool",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"scan_gateways": "true"},
			},
			expectError: true,
			errorText:   "boolean",
		},
		{
			name: "discoveryConfig.scanGateways: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"scanGateways": true},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.dial_timeout_ms validation
		// =================================================================
		{
			name: "discoveryConfig.dial_timeout_ms: valid positive value (1000)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"dial_timeout_ms": float64(1000)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.dial_timeout_ms: minimum positive (1)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"dial_timeout_ms": float64(1)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.dial_timeout_ms: zero is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"dial_timeout_ms": float64(0)},
			},
			expectError: true,
			errorText:   "greater than zero",
		},
		{
			name: "discoveryConfig.dial_timeout_ms: negative is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"dial_timeout_ms": float64(-1)},
			},
			expectError: true,
			errorText:   "greater than zero",
		},
		{
			name: "discoveryConfig.dial_timeout_ms: string instead of number",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"dial_timeout_ms": "1000"},
			},
			expectError: true,
			errorText:   "number",
		},
		{
			name: "discoveryConfig.dialTimeoutMs: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"dialTimeoutMs": float64(1000)},
			},
			expectError: false,
		},

		// =================================================================
		// discoveryConfig.http_timeout_ms validation
		// =================================================================
		{
			name: "discoveryConfig.http_timeout_ms: valid positive value (2000)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"http_timeout_ms": float64(2000)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.http_timeout_ms: minimum positive (1)",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"http_timeout_ms": float64(1)},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig.http_timeout_ms: zero is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"http_timeout_ms": float64(0)},
			},
			expectError: true,
			errorText:   "greater than zero",
		},
		{
			name: "discoveryConfig.http_timeout_ms: negative is invalid",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"http_timeout_ms": float64(-1)},
			},
			expectError: true,
			errorText:   "greater than zero",
		},
		{
			name: "discoveryConfig.http_timeout_ms: string instead of number",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"http_timeout_ms": "2000"},
			},
			expectError: true,
			errorText:   "number",
		},
		{
			name: "discoveryConfig.httpTimeoutMs: camelCase variant works",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{"httpTimeoutMs": float64(2000)},
			},
			expectError: false,
		},

		// =================================================================
		// connectionTimeout validation
		// =================================================================
		{
			name:        "connectionTimeout: zero is valid (disables timeout)",
			input:       map[string]interface{}{"connectionTimeout": float64(0)},
			expectError: false,
		},
		{
			name:        "connectionTimeout: valid minimum positive (1)",
			input:       map[string]interface{}{"connectionTimeout": float64(1)},
			expectError: false,
		},
		{
			name:        "connectionTimeout: valid maximum (300)",
			input:       map[string]interface{}{"connectionTimeout": float64(300)},
			expectError: false,
		},
		{
			name:        "connectionTimeout: negative is invalid",
			input:       map[string]interface{}{"connectionTimeout": float64(-1)},
			expectError: true,
			errorText:   "negative",
		},
		{
			name:        "connectionTimeout: below minimum if positive (0.5)",
			input:       map[string]interface{}{"connectionTimeout": float64(0.5)},
			expectError: true,
			errorText:   "1 second",
		},
		{
			name:        "connectionTimeout: above maximum (301)",
			input:       map[string]interface{}{"connectionTimeout": float64(301)},
			expectError: true,
			errorText:   "300",
		},
		{
			name:        "connectionTimeout: string instead of number",
			input:       map[string]interface{}{"connectionTimeout": "30"},
			expectError: true,
			errorText:   "number",
		},

		// =================================================================
		// theme validation
		// =================================================================
		{
			name:        "theme: empty string is valid",
			input:       map[string]interface{}{"theme": ""},
			expectError: false,
		},
		{
			name:        "theme: 'light' is valid",
			input:       map[string]interface{}{"theme": "light"},
			expectError: false,
		},
		{
			name:        "theme: 'dark' is valid",
			input:       map[string]interface{}{"theme": "dark"},
			expectError: false,
		},
		{
			name:        "theme: invalid value",
			input:       map[string]interface{}{"theme": "purple"},
			expectError: true,
			errorText:   "light", // error message contains valid options
		},
		{
			name:        "theme: number instead of string",
			input:       map[string]interface{}{"theme": float64(1)},
			expectError: true,
			errorText:   "string",
		},
		{
			name:        "theme: boolean instead of string",
			input:       map[string]interface{}{"theme": true},
			expectError: true,
			errorText:   "string",
		},

		// =================================================================
		// updateChannel validation
		// =================================================================
		{
			name:        "updateChannel: empty string is valid",
			input:       map[string]interface{}{"updateChannel": ""},
			expectError: false,
		},
		{
			name:        "updateChannel: 'stable' is valid",
			input:       map[string]interface{}{"updateChannel": "stable"},
			expectError: false,
		},
		{
			name:        "updateChannel: 'rc' is valid",
			input:       map[string]interface{}{"updateChannel": "rc"},
			expectError: false,
		},
		{
			name:        "updateChannel: invalid value",
			input:       map[string]interface{}{"updateChannel": "beta"},
			expectError: true,
			errorText:   "stable", // error message contains valid options
		},
		{
			name:        "updateChannel: number instead of string",
			input:       map[string]interface{}{"updateChannel": float64(1)},
			expectError: true,
			errorText:   "string",
		},

		// =================================================================
		// Complex/edge case scenarios
		// =================================================================
		{
			name: "multiple valid fields together",
			input: map[string]interface{}{
				"pvePollingInterval":      float64(60),
				"pbsPollingInterval":      float64(120),
				"autoUpdateEnabled":       true,
				"theme":                   "dark",
				"connectionTimeout":       float64(30),
				"backupPollingInterval":   float64(3600),
				"backupPollingEnabled":    true,
				"autoUpdateCheckInterval": float64(24),
			},
			expectError: false,
		},
		{
			name: "complex discoveryConfig with multiple valid fields",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{
					"environment_override": "docker_host",
					"subnet_allowlist":     []interface{}{"192.168.1.0/24", "10.0.0.0/8"},
					"subnet_blocklist":     []interface{}{"169.254.0.0/16"},
					"max_hosts_per_scan":   float64(500),
					"max_concurrent":       float64(100),
					"enable_reverse_dns":   true,
					"scan_gateways":        false,
					"dial_timeout_ms":      float64(1500),
					"http_timeout_ms":      float64(3000),
				},
			},
			expectError: false,
		},
		{
			name: "discoveryConfig: mixing camelCase and snake_case fields",
			input: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{
					"environment_override": "native",
					"maxHostsPerScan":      float64(1024),
					"subnet_allowlist":     []interface{}{"192.168.0.0/16"},
					"enableReverseDns":     true,
				},
			},
			expectError: false,
		},
		{
			name:        "no fields provided (empty request)",
			input:       map[string]interface{}{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSystemSettings(&config.SystemSettings{}, tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errorText != "" && !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("expected error containing %q, got: %v", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestValidateSystemSettings_DiscoveryConfigMissing tests the behavior when
// discoveryConfig is completely missing from the request (not provided at all).
func TestValidateSystemSettings_DiscoveryConfigMissing(t *testing.T) {
	input := map[string]interface{}{
		"pvePollingInterval": float64(60),
		// discoveryConfig is not provided at all
	}

	err := validateSystemSettings(&config.SystemSettings{}, input)
	if err != nil {
		t.Errorf("expected no error when discoveryConfig is missing, got: %v", err)
	}
}

// TestValidateSystemSettings_BoundaryConditions tests exact boundary conditions
// to ensure off-by-one errors are caught.
func TestValidateSystemSettings_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expectError bool
	}{
		// PVE interval boundaries
		{name: "pvePollingInterval: 9.999", input: map[string]interface{}{"pvePollingInterval": 9.999}, expectError: true},
		{name: "pvePollingInterval: 10.001", input: map[string]interface{}{"pvePollingInterval": 10.001}, expectError: false},
		{name: "pvePollingInterval: 3599.999", input: map[string]interface{}{"pvePollingInterval": 3599.999}, expectError: false},
		{name: "pvePollingInterval: 3600.001", input: map[string]interface{}{"pvePollingInterval": 3600.001}, expectError: true},

		// Backup interval boundaries
		{name: "backupPollingInterval: -0.001", input: map[string]interface{}{"backupPollingInterval": -0.001}, expectError: true},
		{name: "backupPollingInterval: 0.001", input: map[string]interface{}{"backupPollingInterval": 0.001}, expectError: true},
		{name: "backupPollingInterval: 9.999", input: map[string]interface{}{"backupPollingInterval": 9.999}, expectError: true},
		{name: "backupPollingInterval: 10.001", input: map[string]interface{}{"backupPollingInterval": 10.001}, expectError: false},
		{name: "backupPollingInterval: 604799.999", input: map[string]interface{}{"backupPollingInterval": 604799.999}, expectError: false},
		{name: "backupPollingInterval: 604800.001", input: map[string]interface{}{"backupPollingInterval": 604800.001}, expectError: true},

		// Auto-update check interval boundaries
		{name: "autoUpdateCheckInterval: 0.999", input: map[string]interface{}{"autoUpdateCheckInterval": 0.999}, expectError: true},
		{name: "autoUpdateCheckInterval: 1.001", input: map[string]interface{}{"autoUpdateCheckInterval": 1.001}, expectError: false},
		{name: "autoUpdateCheckInterval: 167.999", input: map[string]interface{}{"autoUpdateCheckInterval": 167.999}, expectError: false},
		{name: "autoUpdateCheckInterval: 168.001", input: map[string]interface{}{"autoUpdateCheckInterval": 168.001}, expectError: true},

		// Connection timeout boundaries
		{name: "connectionTimeout: 0.999", input: map[string]interface{}{"connectionTimeout": 0.999}, expectError: true},
		{name: "connectionTimeout: 1.001", input: map[string]interface{}{"connectionTimeout": 1.001}, expectError: false},
		{name: "connectionTimeout: 299.999", input: map[string]interface{}{"connectionTimeout": 299.999}, expectError: false},
		{name: "connectionTimeout: 300.001", input: map[string]interface{}{"connectionTimeout": 300.001}, expectError: true},

		// Max concurrent boundaries - fractional values are rejected since it's a goroutine count
		{
			name:        "max_concurrent: 0.999 fractional rejected",
			input:       map[string]interface{}{"discoveryConfig": map[string]interface{}{"max_concurrent": 0.999}},
			expectError: true,
		},
		{
			name:        "max_concurrent: 1000.001 fractional rejected",
			input:       map[string]interface{}{"discoveryConfig": map[string]interface{}{"max_concurrent": 1000.001}},
			expectError: true,
		},
		{
			name:        "max_concurrent: 5.5 fractional rejected",
			input:       map[string]interface{}{"discoveryConfig": map[string]interface{}{"max_concurrent": 5.5}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSystemSettings(&config.SystemSettings{}, tt.input)
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
