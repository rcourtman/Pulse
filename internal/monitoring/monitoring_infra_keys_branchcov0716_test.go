package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run BranchCov`) for a group of pure/near-pure helpers in the monitoring
// package whose conditional and switch arms were previously uncovered:
//
//   - monitorClusterEndpointDefaults      (monitor_pve_cluster.go)
//   - monitorBuildClusterEndpointHost     (monitor_pve_cluster.go)
//   - monitorExistingClusterGuestURL      (monitor_pve_cluster.go)
//   - dockerStateFromStatus               (docker_detection.go)
//   - connectedInfrastructureMachineID    (connected_infrastructure.go)
//
// Conventions match sibling in-package tests in this directory: stdlib
// `testing` only, table-driven, no testify.
func TestBranchCovMonitorClusterEndpointDefaults(t *testing.T) {
	cases := []struct {
		name       string
		rawHost    string
		wantScheme string
		wantPort   string
	}{
		// Branch: trimmed value is empty -> default scheme+port.
		{"empty input defaults", "", "https", config.DefaultPVEPort},
		{"whitespace-only input defaults", "   ", "https", config.DefaultPVEPort},

		// Branch: no http(s):// prefix -> "https://" prepended before parse.
		{"no scheme no port uses defaults after prepend", "host.local", "https", config.DefaultPVEPort},
		{"no scheme with explicit port preserves port", "host.local:9000", "https", "9000"},
		{"bare IPv4 with port after prepend", "10.0.0.5:8006", "https", "8006"},

		// Branch: input already has http(s):// prefix (no prepend).
		{"https prefix with port", "https://host.local:8006", "https", "8006"},
		{"https prefix without port falls back to default", "https://host.local", "https", config.DefaultPVEPort},
		{"http prefix preserved with port", "http://host.local:80", "http", "80"},
		{"http prefix without port falls back to default", "http://host.local", "http", config.DefaultPVEPort},

		// Branch: case-insensitive scheme-prefix detection; net/url lowercases
		// the parsed scheme.
		{"uppercase HTTPS prefix treated as https", "HTTPS://Host:9000", "https", "9000"},
		{"uppercase HTTP prefix treated as http", "HTTP://Host", "http", config.DefaultPVEPort},

		// Branch: surrounding whitespace is trimmed before scheme detection.
		{"whitespace around https url trimmed", "  https://host.local:8006  ", "https", "8006"},

		// Branch: url.Parse returns an error -> default scheme+port fallback.
		// A "%" with no hex digits after it is an invalid URL escape.
		{"invalid URL escape falls back to defaults", "https://host.local/%", "https", config.DefaultPVEPort},
		// NUL byte control character in the host makes url.Parse fail; this
		// input has no http(s):// prefix so the "https://" is prepended
		// before the parse, exercising prepend + parse-error together.
		{"control character in host triggers parse error", "host\x00name", "https", config.DefaultPVEPort},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotScheme, gotPort := monitorClusterEndpointDefaults(tc.rawHost)
			if gotScheme != tc.wantScheme || gotPort != tc.wantPort {
				t.Fatalf("monitorClusterEndpointDefaults(%q) = (%q, %q), want (%q, %q)",
					tc.rawHost, gotScheme, gotPort, tc.wantScheme, tc.wantPort)
			}
		})
	}
}

func TestBranchCovMonitorBuildClusterEndpointHost(t *testing.T) {
	cases := []struct {
		name   string
		scheme string
		host   string
		port   string
		want   string
	}{
		// Branch: empty host after trim -> empty result.
		{"empty host returns empty", "https", "", "8006", ""},
		{"whitespace-only host returns empty", "https", "   ", "8006", ""},
		// Branch: host already contains a port (SplitHostPort succeeds) ->
		// host is used verbatim, port arg ignored.
		{"host with matching port used verbatim", "https", "host.local:8006", "8006", "https://host.local:8006"},
		{"host with non-default port overrides port arg", "https", "host.local:9000", "8006", "https://host.local:9000"},
		{"bare IPv4 with port used verbatim", "https", "10.0.0.5:8006", "8006", "https://10.0.0.5:8006"},
		{"bracketed IPv6 with port used verbatim", "https", "[::1]:8006", "8006", "https://[::1]:8006"},
		// Branch: host has no port (SplitHostPort fails) -> JoinHostPort
		// appends the supplied port.
		{"host without port joins supplied port", "https", "host.local", "8006", "https://host.local:8006"},
		{"bare hostname joins default port", "https", "host", "8006", "https://host:8006"},
		// Empty scheme is preserved as-is (results in a "://" prefix); this
		// documents real behavior rather than desirable behavior.
		{"empty scheme produces bare scheme separator", "", "host.local", "8006", "://host.local:8006"},
		// Whitespace around the host is trimmed before the port check.
		{"host whitespace trimmed before join", "https", "  host.local  ", "8006", "https://host.local:8006"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := monitorBuildClusterEndpointHost(tc.scheme, tc.host, tc.port)
			if got != tc.want {
				t.Fatalf("monitorBuildClusterEndpointHost(%q, %q, %q) = %q, want %q",
					tc.scheme, tc.host, tc.port, got, tc.want)
			}
		})
	}
}

func TestBranchCovMonitorExistingClusterGuestURL(t *testing.T) {
	endpoints := []config.ClusterEndpoint{
		{NodeName: "pve-alpha", GuestURL: "  https://alpha.local:8006  "},
		{NodeName: "  pve-beta  ", GuestURL: "https://beta.local:8006"},
		{NodeName: "pve-empty", GuestURL: "   "},
		{NodeName: "pve-duplicate", GuestURL: "https://first.local"},
		{NodeName: "pve-duplicate", GuestURL: "https://second.local"},
	}

	cases := []struct {
		name     string
		nodeName string
		existing []config.ClusterEndpoint
		want     string
	}{
		// Branch: nil/empty existing slice -> "".
		{"nil existing slice returns empty", "pve-alpha", nil, ""},
		{"empty existing slice returns empty", "pve-alpha", []config.ClusterEndpoint{}, ""},

		// Branch: matching endpoint -> GuestURL returned (and trimmed).
		{"exact match returns trimmed guest url", "pve-alpha", endpoints, "https://alpha.local:8006"},

		// Branch: EqualFold case-insensitive comparison.
		{"case-insensitive upper match", "PVE-ALPHA", endpoints, "https://alpha.local:8006"},
		{"case-insensitive mixed match", "pve-Alpha", endpoints, "https://alpha.local:8006"},

		// Branch: surrounding whitespace on the stored NodeName is trimmed
		// before comparison, and on the lookup nodeName too.
		{"stored nodename whitespace trimmed on match", "pve-beta", endpoints, "https://beta.local:8006"},
		{"lookup nodename whitespace trimmed on match", "  pve-beta  ", endpoints, "https://beta.local:8006"},

		// Branch: matching entry whose GuestURL is whitespace-only -> "".
		{"matching entry with whitespace-only guest url returns empty", "pve-empty", endpoints, ""},

		// Branch: first match wins when two endpoints share a NodeName.
		{"first matching entry wins on duplicate", "pve-duplicate", endpoints, "https://first.local"},

		// Branch: no matching entry -> "".
		{"no match returns empty", "pve-missing", endpoints, ""},
		{"whitespace-only lookup with no empty-nodename match returns empty", "   ", endpoints, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := monitorExistingClusterGuestURL(tc.nodeName, tc.existing)
			if got != tc.want {
				t.Fatalf("monitorExistingClusterGuestURL(%q, ...) = %q, want %q",
					tc.nodeName, got, tc.want)
			}
		})
	}
}

func TestBranchCovDockerStateFromStatus(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		// Branch: "up " prefix -> "running".
		{"up status maps to running", "Up 2 hours", "running"},
		{"lowercase up status maps to running", "up 3 minutes", "running"},
		{"up status with surrounding whitespace maps to running", "  Up About a minute  ", "running"},

		// Boundary: "up" WITHOUT a trailing space is NOT recognized as
		// running and falls through to the default arm. Note that
		// strings.TrimSpace runs first, so an input that is ONLY "Up " is
		// trimmed to "up" and therefore does NOT match the "up " prefix.
		{"up without trailing space is not running", "Up", "up"},
		{"up with only trailing space trims to bare up and is not running", "Up ", "up"},
		{"upword without space is not running", "Updated", "updated"},

		// Branch: "exited" prefix -> "exited".
		{"exited status with reason maps to exited", "Exited (0) 5 minutes ago", "exited"},
		{"bare exited maps to exited", "exited", "exited"},
		{"case-insensitive Exited maps to exited", "EXITED (137)", "exited"},

		// Branch: "created" prefix -> "created".
		{"created status maps to created", "Created", "created"},
		{"lowercase created maps to created", "created", "created"},
		{"case-insensitive Created maps to created", "CREATED", "created"},

		// Branch: default arm returns the normalized (lowercased+trimmed)
		// input verbatim.
		{"paused falls through to default normalized", "Paused", "paused"},
		{"restarting falls through to default normalized", "Restarting", "restarting"},
		{"empty status falls through to default empty", "   ", ""},
		{"unknown custom status falls through default normalized", "  SomeCustomState  ", "somecustomstate"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dockerStateFromStatus(tc.status)
			if got != tc.want {
				t.Fatalf("dockerStateFromStatus(%q) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}

func TestBranchCovConnectedInfrastructureMachineID(t *testing.T) {
	cases := []struct {
		name     string
		resource unifiedresources.Resource
		want     string
	}{
		// Branch: default arm -> "" when nothing is populated.
		{"zero resource returns empty", unifiedresources.Resource{}, ""},

		// Branch: Agent.MachineID wins (highest precedence).
		{"agent machine id returned trimmed",
			unifiedresources.Resource{
				Agent: &unifiedresources.AgentData{MachineID: "  agent-machine  "},
			}, "agent-machine"},

		// Branch: Docker.MachineID when Agent is nil.
		{"docker machine id returned trimmed",
			unifiedresources.Resource{
				Docker: &unifiedresources.DockerData{MachineID: "  docker-machine  "},
			}, "docker-machine"},

		// Branch: Identity.MachineID when Agent and Docker are both nil.
		{"identity machine id returned trimmed",
			unifiedresources.Resource{
				Identity: unifiedresources.ResourceIdentity{MachineID: "  identity-machine  "},
			}, "identity-machine"},

		// Precedence: Agent wins over Docker AND Identity when all three set.
		{"agent wins over docker and identity",
			unifiedresources.Resource{
				Agent:    &unifiedresources.AgentData{MachineID: "agent-machine"},
				Docker:   &unifiedresources.DockerData{MachineID: "docker-machine"},
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "agent-machine"},

		// Precedence: Docker wins over Identity when Agent is nil.
		{"docker wins over identity",
			unifiedresources.Resource{
				Docker:   &unifiedresources.DockerData{MachineID: "docker-machine"},
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "docker-machine"},

		// Branch: Agent present but MachineID empty -> falls through to Docker.
		{"agent with empty machine id falls through to docker",
			unifiedresources.Resource{
				Agent:  &unifiedresources.AgentData{MachineID: ""},
				Docker: &unifiedresources.DockerData{MachineID: "docker-machine"},
			}, "docker-machine"},

		// Branch: Agent present but MachineID whitespace-only -> falls through
		// to Identity (whitespace is treated as empty by the trim check).
		{"agent with whitespace-only machine id falls through to identity",
			unifiedresources.Resource{
				Agent:    &unifiedresources.AgentData{MachineID: "   "},
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "identity-machine"},

		// Branch: Docker present but MachineID whitespace-only -> falls through
		// to Identity.
		{"docker with whitespace-only machine id falls through to identity",
			unifiedresources.Resource{
				Docker:   &unifiedresources.DockerData{MachineID: "   "},
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "identity-machine"},

		// Branch: all three populated but all MachineIDs are whitespace-only
		// -> default arm returns "".
		{"all sources whitespace-only machine ids returns empty",
			unifiedresources.Resource{
				Agent:    &unifiedresources.AgentData{MachineID: "   "},
				Docker:   &unifiedresources.DockerData{MachineID: "   "},
				Identity: unifiedresources.ResourceIdentity{MachineID: "   "},
			}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectedInfrastructureMachineID(tc.resource)
			if got != tc.want {
				t.Fatalf("connectedInfrastructureMachineID() = %q, want %q", got, tc.want)
			}
		})
	}
}
