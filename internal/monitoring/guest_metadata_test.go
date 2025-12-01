package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestGuestMetadataCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		instanceName string
		nodeName     string
		vmid         int
		want         string
	}{
		{
			name:         "simple values",
			instanceName: "pve",
			nodeName:     "node1",
			vmid:         100,
			want:         "pve|node1|100",
		},
		{
			name:         "empty instance name",
			instanceName: "",
			nodeName:     "node1",
			vmid:         100,
			want:         "|node1|100",
		},
		{
			name:         "empty node name",
			instanceName: "pve",
			nodeName:     "",
			vmid:         100,
			want:         "pve||100",
		},
		{
			name:         "zero vmid",
			instanceName: "pve",
			nodeName:     "node1",
			vmid:         0,
			want:         "pve|node1|0",
		},
		{
			name:         "large vmid",
			instanceName: "cluster-01",
			nodeName:     "pve-node-2",
			vmid:         999999,
			want:         "cluster-01|pve-node-2|999999",
		},
		{
			name:         "all empty with zero",
			instanceName: "",
			nodeName:     "",
			vmid:         0,
			want:         "||0",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := guestMetadataCacheKey(tc.instanceName, tc.nodeName, tc.vmid)
			if got != tc.want {
				t.Fatalf("guestMetadataCacheKey(%q, %q, %d) = %q, want %q",
					tc.instanceName, tc.nodeName, tc.vmid, got, tc.want)
			}
		})
	}
}

func TestCloneStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  []string
		want []string
	}{
		{
			name: "nil slice",
			src:  nil,
			want: nil,
		},
		{
			name: "empty slice",
			src:  []string{},
			want: nil,
		},
		{
			name: "single element",
			src:  []string{"a"},
			want: []string{"a"},
		},
		{
			name: "multiple elements",
			src:  []string{"a", "b", "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "with empty strings",
			src:  []string{"", "a", ""},
			want: []string{"", "a", ""},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cloneStringSlice(tc.src)

			// Check equality
			if len(got) != len(tc.want) {
				t.Fatalf("cloneStringSlice returned slice of len %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("cloneStringSlice()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}

			// Verify it's a copy (not same backing array) for non-empty slices
			if len(tc.src) > 0 && got != nil {
				tc.src[0] = "modified"
				if got[0] == "modified" {
					t.Fatal("cloneStringSlice did not create independent copy")
				}
			}
		})
	}
}

func TestCloneGuestNetworkInterfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  []models.GuestNetworkInterface
		want []models.GuestNetworkInterface
	}{
		{
			name: "nil slice",
			src:  nil,
			want: nil,
		},
		{
			name: "empty slice",
			src:  []models.GuestNetworkInterface{},
			want: nil,
		},
		{
			name: "single interface no addresses",
			src: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55"},
			},
			want: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55"},
			},
		},
		{
			name: "single interface with addresses",
			src: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					MAC:       "00:11:22:33:44:55",
					Addresses: []string{"192.168.1.10", "10.0.0.5"},
					RXBytes:   1024,
					TXBytes:   512,
				},
			},
			want: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					MAC:       "00:11:22:33:44:55",
					Addresses: []string{"192.168.1.10", "10.0.0.5"},
					RXBytes:   1024,
					TXBytes:   512,
				},
			},
		},
		{
			name: "multiple interfaces",
			src: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"10.0.0.5"}},
			},
			want: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"10.0.0.5"}},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cloneGuestNetworkInterfaces(tc.src)

			if len(got) != len(tc.want) {
				t.Fatalf("cloneGuestNetworkInterfaces returned slice of len %d, want %d", len(got), len(tc.want))
			}

			for i := range got {
				if got[i].Name != tc.want[i].Name {
					t.Errorf("interface[%d].Name = %q, want %q", i, got[i].Name, tc.want[i].Name)
				}
				if got[i].MAC != tc.want[i].MAC {
					t.Errorf("interface[%d].MAC = %q, want %q", i, got[i].MAC, tc.want[i].MAC)
				}
				if got[i].RXBytes != tc.want[i].RXBytes {
					t.Errorf("interface[%d].RXBytes = %d, want %d", i, got[i].RXBytes, tc.want[i].RXBytes)
				}
				if got[i].TXBytes != tc.want[i].TXBytes {
					t.Errorf("interface[%d].TXBytes = %d, want %d", i, got[i].TXBytes, tc.want[i].TXBytes)
				}
				if len(got[i].Addresses) != len(tc.want[i].Addresses) {
					t.Errorf("interface[%d].Addresses length = %d, want %d", i, len(got[i].Addresses), len(tc.want[i].Addresses))
				}
				for j := range got[i].Addresses {
					if got[i].Addresses[j] != tc.want[i].Addresses[j] {
						t.Errorf("interface[%d].Addresses[%d] = %q, want %q", i, j, got[i].Addresses[j], tc.want[i].Addresses[j])
					}
				}
			}

			// Verify addresses are deep-copied (not shared)
			if len(tc.src) > 0 && len(tc.src[0].Addresses) > 0 && got != nil {
				original := tc.src[0].Addresses[0]
				tc.src[0].Addresses[0] = "modified"
				if got[0].Addresses[0] == "modified" {
					t.Fatal("cloneGuestNetworkInterfaces did not deep copy addresses")
				}
				tc.src[0].Addresses[0] = original // restore for parallel safety
			}
		})
	}
}

func TestProcessGuestNetworkInterfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        []proxmox.VMNetworkInterface
		wantIPs    []string
		wantIfaces []models.GuestNetworkInterface
	}{
		{
			name:       "nil input",
			raw:        nil,
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name:       "empty input",
			raw:        []proxmox.VMNetworkInterface{},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "single interface with valid IP",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "filter loopback 127.x.x.x",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "lo",
					HardwareAddr: "00:00:00:00:00:00",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "127.0.0.1", Prefix: 8},
						{Address: "127.0.0.2", Prefix: 8},
					},
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "filter link-local fe80",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "fe80::1", Prefix: 64},
						{Address: "FE80::abcd", Prefix: 64},
					},
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "filter IPv6 loopback ::1",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "lo",
					HardwareAddr: "00:00:00:00:00:00",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "::1", Prefix: 128},
					},
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "mixed valid and filtered IPs",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
						{Address: "127.0.0.1", Prefix: 8},
						{Address: "fe80::1", Prefix: 64},
						{Address: "10.0.0.5", Prefix: 8},
					},
				},
			},
			wantIPs: []string{"10.0.0.5", "192.168.1.10"}, // sorted
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5", "192.168.1.10"}},
			},
		},
		{
			name: "deduplicate IPs within interface",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
						{Address: "192.168.1.10", Prefix: 24},
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "deduplicate IPs across interfaces",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
				{
					Name:         "eth1",
					HardwareAddr: "AA:BB:CC:DD:EE:FF",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24}, // same IP on different interface
					},
				},
			},
			wantIPs: []string{"192.168.1.10"}, // deduplicated globally
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "multiple interfaces sorted by name",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth1",
					HardwareAddr: "AA:BB:CC:DD:EE:FF",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "10.0.0.5", Prefix: 8},
					},
				},
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"10.0.0.5", "192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"10.0.0.5"}},
			},
		},
		{
			name: "interface with only traffic (no IPs) is included",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses:  nil,
					Statistics:   map[string]interface{}{"rx-bytes": float64(1024), "tx-bytes": float64(512)},
				},
			},
			wantIPs: []string{},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: nil, RXBytes: 1024, TXBytes: 512},
			},
		},
		{
			name: "interface with no IPs and no traffic is excluded",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses:  nil,
					Statistics:   nil,
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "whitespace trimmed from name and MAC",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "  eth0  ",
					HardwareAddr: "  00:11:22:33:44:55  ",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "whitespace trimmed from IP addresses",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "  192.168.1.10  ", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "empty IP addresses are skipped",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "", Prefix: 0},
						{Address: "   ", Prefix: 0},
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotIPs, gotIfaces := processGuestNetworkInterfaces(tc.raw)

			// Check IPs
			if len(gotIPs) != len(tc.wantIPs) {
				t.Errorf("processGuestNetworkInterfaces() IPs length = %d, want %d", len(gotIPs), len(tc.wantIPs))
			}
			for i := range gotIPs {
				if i >= len(tc.wantIPs) {
					break
				}
				if gotIPs[i] != tc.wantIPs[i] {
					t.Errorf("IPs[%d] = %q, want %q", i, gotIPs[i], tc.wantIPs[i])
				}
			}

			// Check interfaces
			if len(gotIfaces) != len(tc.wantIfaces) {
				t.Errorf("processGuestNetworkInterfaces() interfaces length = %d, want %d", len(gotIfaces), len(tc.wantIfaces))
			}
			for i := range gotIfaces {
				if i >= len(tc.wantIfaces) {
					break
				}
				if gotIfaces[i].Name != tc.wantIfaces[i].Name {
					t.Errorf("interface[%d].Name = %q, want %q", i, gotIfaces[i].Name, tc.wantIfaces[i].Name)
				}
				if gotIfaces[i].MAC != tc.wantIfaces[i].MAC {
					t.Errorf("interface[%d].MAC = %q, want %q", i, gotIfaces[i].MAC, tc.wantIfaces[i].MAC)
				}
				if gotIfaces[i].RXBytes != tc.wantIfaces[i].RXBytes {
					t.Errorf("interface[%d].RXBytes = %d, want %d", i, gotIfaces[i].RXBytes, tc.wantIfaces[i].RXBytes)
				}
				if gotIfaces[i].TXBytes != tc.wantIfaces[i].TXBytes {
					t.Errorf("interface[%d].TXBytes = %d, want %d", i, gotIfaces[i].TXBytes, tc.wantIfaces[i].TXBytes)
				}
				if len(gotIfaces[i].Addresses) != len(tc.wantIfaces[i].Addresses) {
					t.Errorf("interface[%d].Addresses length = %d, want %d", i, len(gotIfaces[i].Addresses), len(tc.wantIfaces[i].Addresses))
				}
				for j := range gotIfaces[i].Addresses {
					if j >= len(tc.wantIfaces[i].Addresses) {
						break
					}
					if gotIfaces[i].Addresses[j] != tc.wantIfaces[i].Addresses[j] {
						t.Errorf("interface[%d].Addresses[%d] = %q, want %q", i, j, gotIfaces[i].Addresses[j], tc.wantIfaces[i].Addresses[j])
					}
				}
			}
		})
	}
}
