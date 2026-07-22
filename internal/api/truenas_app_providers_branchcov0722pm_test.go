package api

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

// TestBranchcov0722PM_Ports exercises every branch of
// mapTrueNASAppPortsToToolPorts: empty/nil input, the no-HostPorts arm, the
// per-HostPort arm (including multiple host ports and trimmed/uppercased
// protocol and host IP), and a mixed slice that hits both arms in one call.
func TestBranchcov0722PM_Ports(t *testing.T) {
	cases := []struct {
		name   string
		ports  []truenas.AppPort
		expect []tools.PortInfo
	}{
		{
			name:   "nil_slice_returns_nil",
			ports:  nil,
			expect: nil,
		},
		{
			name:   "empty_slice_returns_nil",
			ports:  []truenas.AppPort{},
			expect: nil,
		},
		{
			name: "no_host_ports_emits_private_and_protocol_only",
			ports: []truenas.AppPort{
				{ContainerPort: 8080, Protocol: "  TCP "},
			},
			expect: []tools.PortInfo{
				{Private: 8080, Public: 0, Protocol: "tcp", IP: ""},
			},
		},
		{
			name: "no_host_ports_with_blank_protocol_normalizes_to_empty",
			ports: []truenas.AppPort{
				{ContainerPort: 53, Protocol: "   "},
			},
			expect: []tools.PortInfo{
				{Private: 53, Public: 0, Protocol: "", IP: ""},
			},
		},
		{
			name: "host_ports_emit_one_entry_each_with_trimmed_ip",
			ports: []truenas.AppPort{
				{
					ContainerPort: 443,
					Protocol:      "tcp",
					HostPorts: []truenas.AppHostPort{
						{HostPort: 8443, HostIP: " 10.0.0.1 "},
						{HostPort: 9443, HostIP: ""},
					},
				},
			},
			expect: []tools.PortInfo{
				{Private: 443, Public: 8443, Protocol: "tcp", IP: "10.0.0.1"},
				{Private: 443, Public: 9443, Protocol: "tcp", IP: ""},
			},
		},
		{
			name: "mixed_slice_hits_both_arms_in_order",
			ports: []truenas.AppPort{
				{ContainerPort: 8080, Protocol: "TCP"},
				{ContainerPort: 5432, Protocol: "tcp", HostPorts: []truenas.AppHostPort{{HostPort: 15432, HostIP: "172.16.0.1"}}},
			},
			expect: []tools.PortInfo{
				{Private: 8080, Public: 0, Protocol: "tcp", IP: ""},
				{Private: 5432, Public: 15432, Protocol: "tcp", IP: "172.16.0.1"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapTrueNASAppPortsToToolPorts(tc.ports)
			if tc.expect == nil {
				if got != nil {
					t.Fatalf("expected nil result, got %#v", got)
				}
				return
			}
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %d entries, got %d: %#v", len(tc.expect), len(got), got)
			}
			if !reflect.DeepEqual(got, tc.expect) {
				t.Fatalf("expected %#v, got %#v", tc.expect, got)
			}
		})
	}
}

// TestBranchcov0722PM_Networks exercises every branch of
// mapTrueNASAppNetworksToToolNetworks: empty/nil input, name-from-Name,
// fallback to ID when Name is blank, and both-blank.
func TestBranchcov0722PM_Networks(t *testing.T) {
	cases := []struct {
		name     string
		networks []truenas.AppNetwork
		expect   []tools.NetworkInfo
	}{
		{
			name:     "nil_slice_returns_nil",
			networks: nil,
			expect:   nil,
		},
		{
			name:     "empty_slice_returns_nil",
			networks: []truenas.AppNetwork{},
			expect:   nil,
		},
		{
			name: "uses_trimmed_name_when_present",
			networks: []truenas.AppNetwork{
				{Name: "  frontend  ", ID: "net-1"},
			},
			expect: []tools.NetworkInfo{{Name: "frontend"}},
		},
		{
			name: "falls_back_to_trimmed_id_when_name_blank",
			networks: []truenas.AppNetwork{
				{Name: "   ", ID: "  net-2 "},
			},
			expect: []tools.NetworkInfo{{Name: "net-2"}},
		},
		{
			name: "both_blank_yields_empty_name",
			networks: []truenas.AppNetwork{
				{Name: "", ID: ""},
			},
			expect: []tools.NetworkInfo{{Name: ""}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapTrueNASAppNetworksToToolNetworks(tc.networks)
			if tc.expect == nil {
				if got != nil {
					t.Fatalf("expected nil result, got %#v", got)
				}
				return
			}
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %d entries, got %d: %#v", len(tc.expect), len(got), got)
			}
			if !reflect.DeepEqual(got, tc.expect) {
				t.Fatalf("expected %#v, got %#v", tc.expect, got)
			}
		})
	}
}

// TestBranchcov0722PM_Volumes exercises every branch of
// mapTrueNASAppVolumesToToolMounts: empty/nil input, read-only detection
// (case-insensitive "ro"), read-write default, and field trimming.
func TestBranchcov0722PM_Volumes(t *testing.T) {
	cases := []struct {
		name    string
		volumes []truenas.AppVolume
		expect  []tools.MountInfo
	}{
		{
			name:    "nil_slice_returns_nil",
			volumes: nil,
			expect:  nil,
		},
		{
			name:    "empty_slice_returns_nil",
			volumes: []truenas.AppVolume{},
			expect:  nil,
		},
		{
			name: "ro_mode_lowercase_is_read_only",
			volumes: []truenas.AppVolume{
				{Source: "/host/path", Destination: "/container/path", Mode: "ro"},
			},
			expect: []tools.MountInfo{
				{Source: "/host/path", Destination: "/container/path", ReadWrite: false},
			},
		},
		{
			name: "RO_mode_uppercase_is_read_only",
			volumes: []truenas.AppVolume{
				{Source: "  /host/ro  ", Destination: " /data ", Mode: "  RO "},
			},
			expect: []tools.MountInfo{
				{Source: "/host/ro", Destination: "/data", ReadWrite: false},
			},
		},
		{
			name: "rw_mode_is_read_write",
			volumes: []truenas.AppVolume{
				{Source: "/host/rw", Destination: "/data", Mode: "rw"},
			},
			expect: []tools.MountInfo{
				{Source: "/host/rw", Destination: "/data", ReadWrite: true},
			},
		},
		{
			name: "empty_mode_defaults_to_read_write",
			volumes: []truenas.AppVolume{
				{Source: "/host/empty", Destination: "/data", Mode: ""},
			},
			expect: []tools.MountInfo{
				{Source: "/host/empty", Destination: "/data", ReadWrite: true},
			},
		},
		{
			name: "unknown_mode_defaults_to_read_write",
			volumes: []truenas.AppVolume{
				{Source: "/host/zfs", Destination: "/data", Mode: "zfs"},
			},
			expect: []tools.MountInfo{
				{Source: "/host/zfs", Destination: "/data", ReadWrite: true},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapTrueNASAppVolumesToToolMounts(tc.volumes)
			if tc.expect == nil {
				if got != nil {
					t.Fatalf("expected nil result, got %#v", got)
				}
				return
			}
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %d entries, got %d: %#v", len(tc.expect), len(got), got)
			}
			if !reflect.DeepEqual(got, tc.expect) {
				t.Fatalf("expected %#v, got %#v", tc.expect, got)
			}
		})
	}
}

// TestBranchcov0722PM_Containers exercises every branch of
// mapTrueNASAppContainersToToolContainers: empty/nil input, a fully-populated
// container (trimmed/lowercased fields with nested ports and mounts), and a
// container whose PortConfig/VolumeMounts are nil so the nested mappers return
// nil.
func TestBranchcov0722PM_Containers(t *testing.T) {
	cases := []struct {
		name       string
		containers []truenas.AppContainer
		expect     []tools.AppContainerConfigContainer
	}{
		{
			name:       "nil_slice_returns_nil",
			containers: nil,
			expect:     nil,
		},
		{
			name:       "empty_slice_returns_nil",
			containers: []truenas.AppContainer{},
			expect:     nil,
		},
		{
			name: "fully_populated_trims_and_maps_nested_ports_and_mounts",
			containers: []truenas.AppContainer{
				{
					ID:          "  c1 ",
					ServiceName: "  plex ",
					Image:       " plexinc/pms:1.2 ",
					State:       "  RUNNING ",
					PortConfig: []truenas.AppPort{
						{ContainerPort: 32400, Protocol: "TCP"},
						{ContainerPort: 3005, Protocol: "tcp", HostPorts: []truenas.AppHostPort{{HostPort: 13240, HostIP: "1.2.3.4"}}},
					},
					VolumeMounts: []truenas.AppVolume{
						{Source: "/mnt/media", Destination: "/data", Mode: "ro"},
					},
				},
			},
			expect: []tools.AppContainerConfigContainer{
				{
					ID:      "c1",
					Service: "plex",
					Image:   "plexinc/pms:1.2",
					State:   "running",
					Ports: []tools.PortInfo{
						{Private: 32400, Public: 0, Protocol: "tcp", IP: ""},
						{Private: 3005, Public: 13240, Protocol: "tcp", IP: "1.2.3.4"},
					},
					Mounts: []tools.MountInfo{
						{Source: "/mnt/media", Destination: "/data", ReadWrite: false},
					},
				},
			},
		},
		{
			name: "container_without_ports_or_mounts_has_nil_nested_slices",
			containers: []truenas.AppContainer{
				{
					ID:           "c2",
					ServiceName:  "bare",
					Image:        "alpine",
					State:        "EXITED",
					PortConfig:   nil,
					VolumeMounts: nil,
				},
			},
			expect: []tools.AppContainerConfigContainer{
				{
					ID:      "c2",
					Service: "bare",
					Image:   "alpine",
					State:   "exited",
					Ports:   nil,
					Mounts:  nil,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapTrueNASAppContainersToToolContainers(tc.containers)
			if tc.expect == nil {
				if got != nil {
					t.Fatalf("expected nil result, got %#v", got)
				}
				return
			}
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %d entries, got %d: %#v", len(tc.expect), len(got), got)
			}
			if !reflect.DeepEqual(got, tc.expect) {
				t.Fatalf("expected %#v, got %#v", tc.expect, got)
			}
		})
	}
}

// TestBranchcov0722PM_LogOutput exercises every branch of
// formatTrueNASAppLogOutput: empty/nil input, timestamp-less line, timestamped
// line (with trimming), ordering across multiple lines, and skipping of blank
// Data entries (including the all-blank case that yields "").
func TestBranchcov0722PM_LogOutput(t *testing.T) {
	cases := []struct {
		name   string
		lines  []truenas.AppLogLine
		expect string
	}{
		{
			name:   "nil_input_returns_empty",
			lines:  nil,
			expect: "",
		},
		{
			name:   "empty_input_returns_empty",
			lines:  []truenas.AppLogLine{},
			expect: "",
		},
		{
			name:   "single_line_without_timestamp_emits_text_only",
			lines:  []truenas.AppLogLine{{Data: "  started ok  "}},
			expect: "started ok",
		},
		{
			name:   "single_line_with_timestamp_prefixes_and_trims_both",
			lines:  []truenas.AppLogLine{{Timestamp: " 2024-01-02T03:04:05Z ", Data: "  boot "}},
			expect: "2024-01-02T03:04:05Z boot",
		},
		{
			name: "multiline_preserves_order_and_joins_with_newline",
			lines: []truenas.AppLogLine{
				{Data: "first"},
				{Timestamp: "2024-01-02", Data: "second"},
			},
			expect: "first\n2024-01-02 second",
		},
		{
			name: "blank_data_lines_are_skipped_but_order_preserved",
			lines: []truenas.AppLogLine{
				{Timestamp: "2024-01-02", Data: "   "},
				{Data: "kept"},
				{Data: ""},
			},
			expect: "kept",
		},
		{
			name: "all_blank_data_yields_empty_string",
			lines: []truenas.AppLogLine{
				{Data: "  "},
				{Timestamp: "2024-01-02", Data: ""},
			},
			expect: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatTrueNASAppLogOutput(tc.lines)
			if got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}
