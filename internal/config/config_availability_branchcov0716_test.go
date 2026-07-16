package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBranchCovEffectivePollIntervalSecs covers both branches of
// AvailabilityTarget.EffectivePollIntervalSecs: the "> 0" arm returns the
// configured value, and the <= 0 (zero and negative) arm falls back to the
// package default.
func TestBranchCovEffectivePollIntervalSecs(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"positive value returned as-is", 45, 45},
		{"minimum positive value returned", 1, 1},
		{"zero falls back to default", 0, DefaultAvailabilityPollIntervalSecs},
		{"negative falls back to default", -5, DefaultAvailabilityPollIntervalSecs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := AvailabilityTarget{PollIntervalSecs: tt.value}
			assert.Equal(t, tt.want, target.EffectivePollIntervalSecs())
		})
	}
}

// TestBranchCovEffectiveTimeoutMillis covers both branches of
// AvailabilityTarget.EffectiveTimeoutMillis.
func TestBranchCovEffectiveTimeoutMillis(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"positive value returned as-is", 1500, 1500},
		{"minimum positive value returned", 1, 1},
		{"zero falls back to default", 0, DefaultAvailabilityTimeoutMillis},
		{"negative falls back to default", -100, DefaultAvailabilityTimeoutMillis},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := AvailabilityTarget{TimeoutMillis: tt.value}
			assert.Equal(t, tt.want, target.EffectiveTimeoutMillis())
		})
	}
}

// TestBranchCovEffectiveFailureThreshold covers both branches of
// AvailabilityTarget.EffectiveFailureThreshold.
func TestBranchCovEffectiveFailureThreshold(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"positive value returned as-is", 7, 7},
		{"minimum positive value returned", 1, 1},
		{"zero falls back to default", 0, DefaultAvailabilityFailureThreshold},
		{"negative falls back to default", -3, DefaultAvailabilityFailureThreshold},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := AvailabilityTarget{FailureThreshold: tt.value}
			assert.Equal(t, tt.want, target.EffectiveFailureThreshold())
		})
	}
}

// TestBranchCovDisplayName covers every branch of
// AvailabilityTarget.DisplayName: a present (trimmed) name wins; a blank name
// falls back to the trimmed address; both blank yields an empty string.
func TestBranchCovDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		target AvailabilityTarget
		want   string
	}{
		{
			name:   "name present is returned trimmed",
			target: AvailabilityTarget{Name: "  Energy Monitor  ", Address: "device.local"},
			want:   "Energy Monitor",
		},
		{
			name:   "whitespace-only name falls back to trimmed address",
			target: AvailabilityTarget{Name: "   ", Address: "  device.local  "},
			want:   "device.local",
		},
		{
			name:   "empty name falls back to address",
			target: AvailabilityTarget{Name: "", Address: "10.0.0.1"},
			want:   "10.0.0.1",
		},
		{
			name:   "both blank yields empty string",
			target: AvailabilityTarget{Name: "  ", Address: "\t\n"},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.target.DisplayName())
		})
	}
}

// TestBranchCovValidate exercises every branch of AvailabilityTarget.Validate:
// the address-presence check, each arm of the target-kind and protocol
// switches (including their defaults), the three numeric bound checks
// (including the exact boundary that is accepted), the HTTP/HTTPS URL
// validation path (both success and error propagation), and the non-HTTP
// host presence/whitespace checks.
func TestBranchCovValidate(t *testing.T) {
	tests := []struct {
		name            string
		target          AvailabilityTarget
		wantErr         bool
		wantErrContains string
	}{
		// --- Address presence (first guard). ---
		{
			name:            "empty address rejected",
			target:          AvailabilityTarget{TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP},
			wantErr:         true,
			wantErrContains: "address is required",
		},
		{
			name:            "whitespace-only address rejected",
			target:          AvailabilityTarget{Address: "   ", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP},
			wantErr:         true,
			wantErrContains: "address is required",
		},

		// --- TargetKind switch: default arm. ---
		{
			name:            "unsupported target kind rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetKind("database"), Protocol: AvailabilityProbeICMP},
			wantErr:         true,
			wantErrContains: "unsupported availability target kind",
		},
		{
			name:            "empty target kind hits default arm",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetKind(""), Protocol: AvailabilityProbeICMP},
			wantErr:         true,
			wantErrContains: "unsupported availability target kind",
		},

		// --- Protocol switch: ICMP arm. ---
		{
			name:            "icmp with port set rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 80},
			wantErr:         true,
			wantErrContains: "icmp availability targets must not set a port",
		},
		{
			name:    "icmp machine kind valid happy path",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetMachine, Protocol: AvailabilityProbeICMP, Port: 0, PollIntervalSecs: 30, TimeoutMillis: 1000, FailureThreshold: 3},
			wantErr: false,
		},
		{
			name:    "icmp device kind valid happy path",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetDevice, Protocol: AvailabilityProbeICMP, Port: 0, PollIntervalSecs: 30, TimeoutMillis: 1000, FailureThreshold: 3},
			wantErr: false,
		},

		// --- Protocol switch: TCP arm. ---
		{
			name:    "tcp with valid port accepted",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeTCP, Port: 443},
			wantErr: false,
		},
		{
			name:            "tcp with zero port rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeTCP, Port: 0},
			wantErr:         true,
			wantErrContains: "tcp availability targets require a valid port",
		},
		{
			name:            "tcp with negative port rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeTCP, Port: -1},
			wantErr:         true,
			wantErrContains: "tcp availability targets require a valid port",
		},
		{
			name:            "tcp with out-of-range high port rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeTCP, Port: 70000},
			wantErr:         true,
			wantErrContains: "tcp availability targets require a valid port",
		},

		// --- Protocol switch: HTTP/HTTPS arm. ---
		{
			name:    "http valid address accepted returns nil at http branch",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeHTTP, Port: 0},
			wantErr: false,
		},
		{
			name:    "https valid address accepted returns nil at http branch",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeHTTPS, Port: 0},
			wantErr: false,
		},
		{
			name:            "http negative port rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeHTTP, Port: -1},
			wantErr:         true,
			wantErrContains: "http availability target port must be valid",
		},
		{
			name:            "http out-of-range high port rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeHTTP, Port: 70000},
			wantErr:         true,
			wantErrContains: "http availability target port must be valid",
		},
		{
			name:            "http address with non-http scheme rejected via HTTPURL",
			target:          AvailabilityTarget{Address: "ftp://device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeHTTP, Port: 0},
			wantErr:         true,
			wantErrContains: "http availability targets require http or https scheme",
		},

		// --- Protocol switch: default arm. ---
		{
			name:            "unsupported protocol rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeProtocol("udp")},
			wantErr:         true,
			wantErrContains: "unsupported availability protocol",
		},
		{
			name:            "empty protocol hits default arm",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeProtocol("")},
			wantErr:         true,
			wantErrContains: "unsupported availability protocol",
		},

		// --- Numeric bound checks (below-min rejected, boundary accepted). ---
		{
			name:            "poll interval below minimum rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0, PollIntervalSecs: 9},
			wantErr:         true,
			wantErrContains: "availability poll interval must be at least 10 seconds",
		},
		{
			name:    "poll interval at minimum boundary accepted",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0, PollIntervalSecs: 10},
			wantErr: false,
		},
		{
			name:            "timeout below minimum rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0, TimeoutMillis: 100},
			wantErr:         true,
			wantErrContains: "availability timeout must be at least 250 milliseconds",
		},
		{
			name:    "timeout at minimum boundary accepted",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0, TimeoutMillis: 250},
			wantErr: false,
		},
		{
			name:            "failure threshold above maximum rejected",
			target:          AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0, FailureThreshold: 11},
			wantErr:         true,
			wantErrContains: "availability failure threshold must be 10 or less",
		},
		{
			name:    "failure threshold at maximum boundary accepted",
			target:  AvailabilityTarget{Address: "device.local", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0, FailureThreshold: 10},
			wantErr: false,
		},

		// --- Non-HTTP host checks (reached only for icmp/tcp protocols). ---
		{
			name:            "non-http address normalizing to empty host rejected",
			target:          AvailabilityTarget{Address: "[]", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0},
			wantErr:         true,
			wantErrContains: "address is required",
		},
		{
			name:            "non-http address containing whitespace rejected",
			target:          AvailabilityTarget{Address: "host with space", TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeICMP, Port: 0},
			wantErr:         true,
			wantErrContains: "must not contain whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.Validate()
			if tt.wantErr {
				assert.Error(t, err, "expected an error for %s", tt.name)
				if tt.wantErrContains != "" && err != nil {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
			} else {
				assert.NoError(t, err, "expected no error for %s", tt.name)
			}
		})
	}
}
