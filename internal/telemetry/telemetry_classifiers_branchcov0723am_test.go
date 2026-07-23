package telemetry

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

// TestBranchcov0723Am_DeploymentMethod covers every branch of the deployment
// method precedence chain, including the cfg-takes-precedence-over-env rule,
// whitespace/case normalization, the env-var fallback, and both IsDocker
// fallbacks when nothing matches.
func TestBranchcov0723Am_DeploymentMethod(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		envMethod string
		want      string
	}{
		{
			name:      "cfg closed value docker_compose returned verbatim",
			cfg:       Config{DeploymentMethod: "docker_compose"},
			envMethod: "",
			want:      "docker_compose",
		},
		{
			name:      "cfg closed value docker_run returned verbatim",
			cfg:       Config{DeploymentMethod: "docker_run"},
			envMethod: "",
			want:      "docker_run",
		},
		{
			name:      "cfg closed value container_other returned verbatim",
			cfg:       Config{DeploymentMethod: "container_other"},
			envMethod: "",
			want:      "container_other",
		},
		{
			name:      "cfg closed value systemd returned verbatim",
			cfg:       Config{DeploymentMethod: "systemd"},
			envMethod: "",
			want:      "systemd",
		},
		{
			name:      "cfg closed value binary_other returned verbatim",
			cfg:       Config{DeploymentMethod: "binary_other"},
			envMethod: "",
			want:      "binary_other",
		},
		{
			name:      "cfg closed value other returned verbatim",
			cfg:       Config{DeploymentMethod: "other"},
			envMethod: "",
			want:      "other",
		},
		{
			name:      "cfg value trimmed and lowercased before matching",
			cfg:       Config{DeploymentMethod: "  Docker_Compose  "},
			envMethod: "",
			want:      "docker_compose",
		},
		{
			name:      "cfg empty falls back to env closed value",
			cfg:       Config{},
			envMethod: "docker_run",
			want:      "docker_run",
		},
		{
			name:      "cfg whitespace-only falls back to env closed value",
			cfg:       Config{DeploymentMethod: "   "},
			envMethod: "systemd",
			want:      "systemd",
		},
		{
			name:      "cfg value wins over conflicting env value",
			cfg:       Config{DeploymentMethod: "systemd"},
			envMethod: "docker_run",
			want:      "systemd",
		},
		{
			name:      "cfg invalid value ignores env and falls back to docker branch",
			cfg:       Config{DeploymentMethod: "tarball", IsDocker: true},
			envMethod: "systemd",
			want:      "container_other",
		},
		{
			name:      "cfg invalid value ignores env and falls back to binary branch",
			cfg:       Config{DeploymentMethod: "tarball", IsDocker: false},
			envMethod: "systemd",
			want:      "binary_other",
		},
		{
			name:      "nothing set and docker falls back to container_other",
			cfg:       Config{IsDocker: true},
			envMethod: "",
			want:      "container_other",
		},
		{
			name:      "nothing set and binary falls back to binary_other",
			cfg:       Config{IsDocker: false},
			envMethod: "",
			want:      "binary_other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULSE_DEPLOYMENT_METHOD", tt.envMethod)
			if got := deploymentMethod(tt.cfg); got != tt.want {
				t.Fatalf("deploymentMethod(%+v) = %q, want %q", tt.cfg, got, tt.want)
			}
		})
	}
}

// TestBranchcov0723Am_DurationBucket covers the empty-boundaries case, values
// below/between/past boundaries, the exact-at-boundary semantics (the source
// uses a strict `<` comparison so a value equal to an upper bound falls through
// to the next bucket), one-unit-either-side of every boundary, and a negative
// duration.
func TestBranchcov0723Am_DurationBucket(t *testing.T) {
	boundaries := []durationBoundary{
		{upper: 10 * time.Second, label: "a"},
		{upper: 1 * time.Minute, label: "b"},
		{upper: 1 * time.Hour, label: "c"},
	}
	const overflow = "over"

	tests := []struct {
		name  string
		value time.Duration
		want  string
	}{
		{"empty boundaries slice returns overflow regardless of value", 5 * time.Second, overflow}, // boundaries == nil below
		{"value well below first boundary", 5 * time.Second, "a"},
		{"value one nanosecond below first boundary", 10*time.Second - time.Nanosecond, "a"},
		{"value exactly on first boundary falls through to next bucket", 10 * time.Second, "b"},
		{"value one nanosecond above first boundary", 10*time.Second + time.Nanosecond, "b"},
		{"value strictly between first and second boundary", 30 * time.Second, "b"},
		{"value one nanosecond below second boundary", 1*time.Minute - time.Nanosecond, "b"},
		{"value exactly on second boundary falls through to next bucket", 1 * time.Minute, "c"},
		{"value one nanosecond above second boundary", 1*time.Minute + time.Nanosecond, "c"},
		{"value one nanosecond below last boundary", 1*time.Hour - time.Nanosecond, "c"},
		{"value exactly on last boundary returns overflow", 1 * time.Hour, overflow},
		{"value one nanosecond above last boundary returns overflow", 1*time.Hour + time.Nanosecond, overflow},
		{"value well past last boundary returns overflow", 2 * time.Hour, overflow},
		{"negative duration lands in the first bucket", -1 * time.Second, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if tt.name == "empty boundaries slice returns overflow regardless of value" {
				got = durationBucket(tt.value, nil, overflow)
			} else {
				got = durationBucket(tt.value, boundaries, overflow)
			}
			if got != tt.want {
				t.Fatalf("durationBucket(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}

	// Anchor the strict-`<` boundary semantics against the real
	// knownInstallAge boundaries used in production so the at-boundary
	// behaviour is asserted on actual production values.
	t.Run("real knownInstallAge 24h boundary lands one bucket up", func(t *testing.T) {
		realBoundaries := []durationBoundary{
			{24 * time.Hour, "under_1d"},
			{7 * 24 * time.Hour, "1_7d"},
			{30 * 24 * time.Hour, "8_30d"},
			{90 * 24 * time.Hour, "31_90d"},
			{365 * 24 * time.Hour, "91_365d"},
		}
		if got := durationBucket(24*time.Hour-time.Nanosecond, realBoundaries, "over_365d"); got != "under_1d" {
			t.Fatalf("just below 24h = %q, want under_1d", got)
		}
		if got := durationBucket(24*time.Hour, realBoundaries, "over_365d"); got != "1_7d" {
			t.Fatalf("exactly 24h = %q, want 1_7d (strict < falls through)", got)
		}
	})
}

// TestBranchcov0723Am_ActivationStage covers every stage the function can
// return in the precedence order the source uses: each outcome trigger
// individually, then monitoring, connected, secured, the default started stage
// for a zero-value Ping, and a precedence check that the highest stage wins.
func TestBranchcov0723Am_ActivationStage(t *testing.T) {
	tests := []struct {
		name string
		ping Ping
		want string
	}{
		{"zero ping defaults to started", Ping{}, "started"},
		{"auth configured reaches secured", Ping{AuthConfigured: true}, "secured"},
		{"configured connections reach connected", Ping{ConfiguredConnections: 1}, "connected"},
		{"a single monitored resource reaches monitoring", Ping{PVENodes: 1}, "monitoring"},
		{"active alerts trigger outcome_observed", Ping{ActiveAlerts: 1}, "outcome_observed"},
		{"fired alerts trigger outcome_observed", Ping{AlertsFired30d: 1}, "outcome_observed"},
		{"resolved alerts trigger outcome_observed", Ping{AlertsResolved30d: 1}, "outcome_observed"},
		{"notification deliveries trigger outcome_observed", Ping{NotificationDeliveries7d: 1}, "outcome_observed"},
		{"monitoring outranks connected", Ping{PVENodes: 1, ConfiguredConnections: 9}, "monitoring"},
		{
			"outcome outranks every lower stage",
			Ping{ActiveAlerts: 1, AlertsFired30d: 1, AlertsResolved30d: 1, NotificationDeliveries7d: 1, PVENodes: 5, ConfiguredConnections: 9, AuthConfigured: true},
			"outcome_observed",
		},
		// AlertsAcknowledged30d is intentionally NOT a trigger in the source;
		// with only that field set the stage must not jump to outcome_observed.
		{"alerts acknowledged alone is not an outcome trigger", Ping{AlertsAcknowledged30d: 1}, "started"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := activationStage(tt.ping); got != tt.want {
				t.Fatalf("activationStage(%+v) = %q, want %q", tt.ping, got, tt.want)
			}
		})
	}
}

// TestBranchcov0723Am_ActivationStageRank covers every known stage plus an
// unknown and empty string, asserts the exact sentinel ranks, and proves the
// ranks are strictly ordered (and that unknown collapses to the started rank).
func TestBranchcov0723Am_ActivationStageRank(t *testing.T) {
	tests := []struct {
		stage string
		want  int
	}{
		{"started", 1},
		{"secured", 2},
		{"connected", 3},
		{"monitoring", 4},
		{"outcome_observed", 5},
		{"", 1},
		{"bogus_stage", 1},
	}
	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			if got := activationStageRank(tt.stage); got != tt.want {
				t.Fatalf("activationStageRank(%q) = %d, want %d", tt.stage, got, tt.want)
			}
		})
	}

	t.Run("ranks strictly ordered by precedence", func(t *testing.T) {
		stages := []string{"started", "secured", "connected", "monitoring", "outcome_observed"}
		prev := -1
		for _, s := range stages {
			r := activationStageRank(s)
			if r <= prev {
				t.Fatalf("rank(%q) = %d not strictly greater than previous %d", s, r, prev)
			}
			prev = r
		}
		if rUnknown, rStarted := activationStageRank("nope"), activationStageRank("started"); rUnknown != rStarted {
			t.Fatalf("unknown rank %d should equal started rank %d", rUnknown, rStarted)
		}
	})
}

// TestBranchcov0723Am_ValidActivationStage returns true for each known stage
// and false for unknown, empty, whitespace, and wrong-case inputs.
func TestBranchcov0723Am_ValidActivationStage(t *testing.T) {
	tests := []struct {
		stage string
		want  bool
	}{
		{"started", true},
		{"secured", true},
		{"connected", true},
		{"monitoring", true},
		{"outcome_observed", true},
		{"", false},
		{"   ", false},
		{"Started", false},
		{"outcome_observed ", false},
		{"bogus", false},
	}
	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			if got := validActivationStage(tt.stage); got != tt.want {
				t.Fatalf("validActivationStage(%q) = %v, want %v", tt.stage, got, tt.want)
			}
		})
	}
}

// TestBranchcov0723Am_MonitoredResourceCount covers a zero Ping, a Ping with
// each counted field populated individually (proving each field contributes
// exactly one), and a Ping with all counted fields populated.
func TestBranchcov0723Am_MonitoredResourceCount(t *testing.T) {
	resourceFields := []struct {
		name string
		set  func(p *Ping)
	}{
		{"PVENodes", func(p *Ping) { p.PVENodes = 1 }},
		{"PBSInstances", func(p *Ping) { p.PBSInstances = 1 }},
		{"PMGInstances", func(p *Ping) { p.PMGInstances = 1 }},
		{"VMs", func(p *Ping) { p.VMs = 1 }},
		{"Containers", func(p *Ping) { p.Containers = 1 }},
		{"AgentHosts", func(p *Ping) { p.AgentHosts = 1 }},
		{"DockerHosts", func(p *Ping) { p.DockerHosts = 1 }},
		{"DockerContainers", func(p *Ping) { p.DockerContainers = 1 }},
		{"KubernetesClusters", func(p *Ping) { p.KubernetesClusters = 1 }},
		{"KubernetesNodes", func(p *Ping) { p.KubernetesNodes = 1 }},
		{"KubernetesPods", func(p *Ping) { p.KubernetesPods = 1 }},
		{"KubernetesDeployments", func(p *Ping) { p.KubernetesDeployments = 1 }},
		{"StoragePools", func(p *Ping) { p.StoragePools = 1 }},
		{"PhysicalDisks", func(p *Ping) { p.PhysicalDisks = 1 }},
		{"CephClusters", func(p *Ping) { p.CephClusters = 1 }},
		{"NetworkShares", func(p *Ping) { p.NetworkShares = 1 }},
		{"TrueNASSystems", func(p *Ping) { p.TrueNASSystems = 1 }},
		{"TrueNASVMs", func(p *Ping) { p.TrueNASVMs = 1 }},
		{"TrueNASApps", func(p *Ping) { p.TrueNASApps = 1 }},
		{"VMwareHosts", func(p *Ping) { p.VMwareHosts = 1 }},
		{"VMwareVMs", func(p *Ping) { p.VMwareVMs = 1 }},
		{"VMwareDatastores", func(p *Ping) { p.VMwareDatastores = 1 }},
		{"AvailabilityTargets", func(p *Ping) { p.AvailabilityTargets = 1 }},
	}

	t.Run("zero ping counts nothing", func(t *testing.T) {
		if got := monitoredResourceCount(Ping{}); got != 0 {
			t.Fatalf("monitoredResourceCount(zero Ping) = %d, want 0", got)
		}
	})

	for _, f := range resourceFields {
		t.Run(f.name+"/contributes_one", func(t *testing.T) {
			var p Ping
			f.set(&p)
			if got := monitoredResourceCount(p); got != 1 {
				t.Fatalf("monitoredResourceCount with only %s set = %d, want 1", f.name, got)
			}
		})
	}

	t.Run("all counted fields sum to total", func(t *testing.T) {
		var p Ping
		for _, f := range resourceFields {
			f.set(&p)
		}
		if got, want := monitoredResourceCount(p), len(resourceFields); got != want {
			t.Fatalf("monitoredResourceCount(all fields) = %d, want %d", got, want)
		}
	})
}

// TestBranchcov0723Am_EstateSizeBucket covers 0, a negative value, each bucket
// interior, every boundary value, and a very large count.
func TestBranchcov0723Am_EstateSizeBucket(t *testing.T) {
	tests := []struct {
		name      string
		resources int
		want      string
	}{
		{"zero is empty", 0, "empty"},
		{"negative is empty", -5, "empty"},
		{"one is in 1_10", 1, "1_10"},
		{"upper boundary of 1_10 inclusive", 10, "1_10"},
		{"lower boundary of 11_50 inclusive", 11, "11_50"},
		{"upper boundary of 11_50 inclusive", 50, "11_50"},
		{"lower boundary of 51_200 inclusive", 51, "51_200"},
		{"upper boundary of 51_200 inclusive", 200, "51_200"},
		{"lower boundary of 201_1000 inclusive", 201, "201_1000"},
		{"upper boundary of 201_1000 inclusive", 1000, "201_1000"},
		{"one above top boundary is over_1000", 1001, "over_1000"},
		{"very large count is over_1000", 1_000_000, "over_1000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := estateSizeBucket(tt.resources); got != tt.want {
				t.Fatalf("estateSizeBucket(%d) = %q, want %q", tt.resources, got, tt.want)
			}
		})
	}
}

// TestBranchcov0723Am_ClassifyUpdateFailureCategory covers every category arm
// the source can return, including the status-precedence over error text, both
// OR operands of the disk_space and extract arms, the lowercasing that makes
// matching case-insensitive, first-match precedence between categories, and the
// unknown/default arm via nil error, unmatched text, and a non-failure status.
func TestBranchcov0723Am_ClassifyUpdateFailureCategory(t *testing.T) {
	errOf := func(code, message, details string) *updates.UpdateError {
		return &updates.UpdateError{Code: code, Message: message, Details: details}
	}

	tests := []struct {
		name  string
		entry updates.UpdateHistoryEntry
		want  string
	}{
		{
			name:  "rolled back status short circuits before error text",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusRolledBack, Error: errOf("", "download timed out", "")},
			want:  "rolled_back",
		},
		{
			name:  "cancelled status short circuits before error text",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusCancelled},
			want:  "cancelled",
		},
		{
			name:  "failed with nil error is unknown",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed},
			want:  "unknown",
		},
		{
			name:  "signature substring in code",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("SIGNATURE_INVALID", "", "")},
			want:  "signature",
		},
		{
			name:  "checksum substring in message",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "checksum verify failed", "")},
			want:  "checksum",
		},
		{
			name:  "download substring in details",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "", "download stream reset")},
			want:  "download",
		},
		{
			name:  "disk space substring in message",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "not enough disk space to extract", "")},
			want:  "disk_space",
		},
		{
			name:  "insufficient disk substring without disk space",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "insufficient disk quota remaining", "")},
			want:  "disk_space",
		},
		{
			name:  "extract substring in message",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "could not extract layer", "")},
			want:  "extract",
		},
		{
			name:  "archive substring in code",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("ARCHIVE_CORRUPT", "", "")},
			want:  "extract",
		},
		{
			name:  "backup substring in details",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "", "backup snapshot failed")},
			want:  "backup",
		},
		{
			name:  "apply substring in message",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "apply step rolled back", "")},
			want:  "apply",
		},
		{
			name:  "restart substring in message",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "service restart timed out", "")},
			want:  "restart",
		},
		{
			name:  "unmatched failure text falls through to unknown",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("HTTP_503", "upstream refused", "raw log stays local")},
			want:  "unknown",
		},
		{
			name:  "uppercase error text is lowercased before matching",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "DOWNLOAD FAILED", "")},
			want:  "download",
		},
		{
			name:  "first matching category wins between signature and checksum",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusFailed, Error: errOf("", "signature then checksum", "")},
			want:  "signature",
		},
		{
			name:  "success status with nil error is unknown",
			entry: updates.UpdateHistoryEntry{Status: updates.StatusSuccess},
			want:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyUpdateFailureCategory(tt.entry); got != tt.want {
				t.Fatalf("classifyUpdateFailureCategory(status=%q, err=%+v) = %q, want %q",
					tt.entry.Status, tt.entry.Error, got, tt.want)
			}
		})
	}
}
