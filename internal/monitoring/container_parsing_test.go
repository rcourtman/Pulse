package monitoring

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestSanitizeRootFSDevice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no comma",
			input: "local:100/vm-100-disk-0.raw",
			want:  "local:100/vm-100-disk-0.raw",
		},
		{
			name:  "with comma",
			input: "local:100/vm-100-disk-0.raw,size=8G",
			want:  "local:100/vm-100-disk-0.raw",
		},
		{
			name:  "with whitespace",
			input: "  local:100/vm-100-disk-0.raw  ",
			want:  "local:100/vm-100-disk-0.raw",
		},
		{
			name:  "with whitespace and comma",
			input: "  local:100/vm-100-disk-0.raw  ,size=8G",
			want:  "local:100/vm-100-disk-0.raw  ", // Trailing whitespace preserved after comma split
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  "",
		},
		{
			name:  "multiple commas",
			input: "local:100/vm-100-disk-0.raw,size=8G,format=raw",
			want:  "local:100/vm-100-disk-0.raw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeRootFSDevice(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeRootFSDevice(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Note: Tests for sanitizeGuestAddressStrings and dedupeStringsPreserveOrder
// are already present in helpers_test.go

func TestCollectIPsFromInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{
			name:  "nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "string valid IP",
			input: "192.168.1.100",
			want:  []string{"192.168.1.100"},
		},
		{
			name:  "string invalid IP",
			input: "dhcp",
			want:  nil,
		},
		{
			name:  "[]interface{} with strings",
			input: []interface{}{"192.168.1.100", "192.168.1.101"},
			want:  []string{"192.168.1.100", "192.168.1.101"},
		},
		{
			name:  "[]interface{} nested",
			input: []interface{}{"192.168.1.100", []interface{}{"192.168.1.101"}},
			want:  []string{"192.168.1.100", "192.168.1.101"},
		},
		{
			name:  "[]string",
			input: []string{"192.168.1.100", "192.168.1.101"},
			want:  []string{"192.168.1.100", "192.168.1.101"},
		},
		{
			name: "map with ip key",
			input: map[string]interface{}{
				"ip": "192.168.1.100",
			},
			want: []string{"192.168.1.100"},
		},
		{
			name: "map with ip6 key",
			input: map[string]interface{}{
				"ip6": "2001:db8::1",
			},
			want: []string{"2001:db8::1"},
		},
		{
			name: "map with ipv4 key",
			input: map[string]interface{}{
				"ipv4": "192.168.1.100",
			},
			want: []string{"192.168.1.100"},
		},
		{
			name: "map with ipv6 key",
			input: map[string]interface{}{
				"ipv6": "2001:db8::1",
			},
			want: []string{"2001:db8::1"},
		},
		{
			name: "map with address key",
			input: map[string]interface{}{
				"address": "192.168.1.100",
			},
			want: []string{"192.168.1.100"},
		},
		{
			name: "map with value key",
			input: map[string]interface{}{
				"value": "192.168.1.100",
			},
			want: []string{"192.168.1.100"},
		},
		{
			name: "map with multiple keys",
			input: map[string]interface{}{
				"ip":  "192.168.1.100",
				"ip6": "2001:db8::1",
			},
			want: []string{"192.168.1.100", "2001:db8::1"},
		},
		{
			name: "map with unrelated keys",
			input: map[string]interface{}{
				"name": "eth0",
				"mac":  "00:11:22:33:44:55",
			},
			want: nil,
		},
		{
			name:  "json.Number",
			input: json.Number("192"),
			want:  []string{"192"}, // json.Number is converted to string and passed through
		},
		{
			name:  "int",
			input: 12345,
			want:  nil,
		},
		{
			name:  "float64",
			input: 123.45,
			want:  nil,
		},
		{
			name:  "bool",
			input: true,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := collectIPsFromInterface(tt.input)
			if !stringSlicesEqual(got, tt.want) {
				t.Errorf("collectIPsFromInterface(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseContainerRawIPs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input json.RawMessage
		want  []string
	}{
		{
			name:  "empty",
			input: json.RawMessage(``),
			want:  nil,
		},
		{
			name:  "nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "invalid JSON",
			input: json.RawMessage(`{invalid`),
			want:  nil,
		},
		{
			name:  "string IP",
			input: json.RawMessage(`"192.168.1.100"`),
			want:  []string{"192.168.1.100"},
		},
		{
			name:  "array of IPs",
			input: json.RawMessage(`["192.168.1.100", "192.168.1.101"]`),
			want:  []string{"192.168.1.100", "192.168.1.101"},
		},
		{
			name:  "object with ip key",
			input: json.RawMessage(`{"ip": "192.168.1.100"}`),
			want:  []string{"192.168.1.100"},
		},
		{
			name:  "complex nested structure",
			input: json.RawMessage(`{"interfaces": [{"ip": "192.168.1.100"}, {"ip": "192.168.1.101"}]}`),
			want:  nil, // "interfaces" key is not checked
		},
		{
			name:  "object with multiple IP keys",
			input: json.RawMessage(`{"ip": "192.168.1.100", "ip6": "2001:db8::1"}`),
			want:  []string{"192.168.1.100", "2001:db8::1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseContainerRawIPs(tt.input)
			if !stringSlicesEqual(got, tt.want) {
				t.Errorf("parseContainerRawIPs(%s) = %v, want %v", string(tt.input), got, tt.want)
			}
		})
	}
}

func TestParseContainerConfigNetworks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]interface{}
		want  []containerNetworkDetails
	}{
		{
			name:  "empty config",
			input: map[string]interface{}{},
			want:  nil,
		},
		{
			name:  "nil config",
			input: nil,
			want:  nil,
		},
		{
			name: "no net keys",
			input: map[string]interface{}{
				"cores": 2,
				"memory": 2048,
			},
			want: nil,
		},
		{
			name: "single interface",
			input: map[string]interface{}{
				"net0": "name=eth0,hwaddr=AA:BB:CC:DD:EE:FF,ip=192.168.1.100",
			},
			want: []containerNetworkDetails{
				{
					Name:      "eth0",
					MAC:       "AA:BB:CC:DD:EE:FF",
					Addresses: []string{"192.168.1.100"},
				},
			},
		},
		{
			name: "MAC normalization to uppercase",
			input: map[string]interface{}{
				"net0": "name=eth0,hwaddr=aa:bb:cc:dd:ee:ff",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "multiple interfaces sorted",
			input: map[string]interface{}{
				"net1": "name=eth1,hwaddr=BB:BB:BB:BB:BB:BB",
				"net0": "name=eth0,hwaddr=AA:AA:AA:AA:AA:AA",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:AA:AA:AA:AA:AA",
				},
				{
					Name: "eth1",
					MAC:  "BB:BB:BB:BB:BB:BB",
				},
			},
		},
		{
			name: "interface with multiple IPs",
			input: map[string]interface{}{
				"net0": "name=eth0,ip=192.168.1.100,ip6=2001:db8::1",
			},
			want: []containerNetworkDetails{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100", "2001:db8::1"},
				},
			},
		},
		{
			name: "interface without name uses key",
			input: map[string]interface{}{
				"net0": "hwaddr=AA:BB:CC:DD:EE:FF,ip=192.168.1.100",
			},
			want: []containerNetworkDetails{
				{
					Name:      "net0",
					MAC:       "AA:BB:CC:DD:EE:FF",
					Addresses: []string{"192.168.1.100"},
				},
			},
		},
		{
			name: "interface with CIDR notation",
			input: map[string]interface{}{
				"net0": "name=eth0,ip=192.168.1.100/24",
			},
			want: []containerNetworkDetails{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100"},
				},
			},
		},
		{
			name: "interface with dhcp ignored",
			input: map[string]interface{}{
				"net0": "name=eth0,hwaddr=AA:BB:CC:DD:EE:FF,ip=dhcp",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "empty interface value",
			input: map[string]interface{}{
				"net0": "",
			},
			want: nil,
		},
		{
			name: "interface with whitespace",
			input: map[string]interface{}{
				"net0": "  name=eth0  ,  hwaddr=AA:BB:CC:DD:EE:FF  ",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "mixed case net keys",
			input: map[string]interface{}{
				"NET0": "name=eth0,hwaddr=AA:BB:CC:DD:EE:FF",
				"Net1": "name=eth1,hwaddr=BB:BB:BB:BB:BB:BB",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
				{
					Name: "eth1",
					MAC:  "BB:BB:BB:BB:BB:BB",
				},
			},
		},
		{
			name: "macaddr alternative key",
			input: map[string]interface{}{
				"net0": "name=eth0,macaddr=AA:BB:CC:DD:EE:FF",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "duplicate IPs deduplicated",
			input: map[string]interface{}{
				"net0": "name=eth0,ip=192.168.1.100,ip6=192.168.1.100",
			},
			want: []containerNetworkDetails{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100"}, // Deduplicated
				},
			},
		},
		{
			name: "parts without equals sign skipped",
			input: map[string]interface{}{
				"net0": "name=eth0,invalidpart,hwaddr=AA:BB:CC:DD:EE:FF",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "mac key variant",
			input: map[string]interface{}{
				"net0": "name=eth0,mac=AA:BB:CC:DD:EE:FF",
			},
			want: []containerNetworkDetails{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "ips key variant",
			input: map[string]interface{}{
				"net0": "name=eth0,ips=192.168.1.100",
			},
			want: []containerNetworkDetails{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100"},
				},
			},
		},
		{
			name: "ip6addr key variant",
			input: map[string]interface{}{
				"net0": "name=eth0,ip6addr=2001:db8::1",
			},
			want: []containerNetworkDetails{
				{
					Name:      "eth0",
					Addresses: []string{"2001:db8::1"},
				},
			},
		},
		{
			name: "ip6prefix key variant",
			input: map[string]interface{}{
				"net0": "name=eth0,ip6prefix=2001:db8::",
			},
			want: []containerNetworkDetails{
				{
					Name:      "eth0",
					Addresses: []string{"2001:db8::"},
				},
			},
		},
		{
			name: "whitespace only interface value",
			input: map[string]interface{}{
				"net0": "   ",
			},
			want: nil,
		},
		{
			name: "all empty net values returns nil",
			input: map[string]interface{}{
				"net0": "",
				"net1": "   ",
			},
			want: nil,
		},
		{
			name: "only unrecognized keys uses key as name",
			input: map[string]interface{}{
				"net0": "unknown=value,other=data",
			},
			want: []containerNetworkDetails{
				{
					Name: "net0",
				},
			},
		},
		{
			name: "value parts only without equals",
			input: map[string]interface{}{
				"net0": "noequals,alsonoequals",
			},
			want: []containerNetworkDetails{
				{
					Name: "net0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseContainerConfigNetworks(tt.input)
			if !networkDetailsSlicesEqual(got, tt.want) {
				t.Errorf("parseContainerConfigNetworks() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Note: Basic tests for parseContainerMountMetadata are in monitor_container_test.go
// These tests add additional edge case coverage

func TestParseContainerMountMetadataEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]interface{}
		want  map[string]containerMountMetadata
	}{
		{
			name:  "empty config",
			input: map[string]interface{}{},
			want:  nil,
		},
		{
			name:  "nil config",
			input: nil,
			want:  nil,
		},
		{
			name: "no mount keys",
			input: map[string]interface{}{
				"cores":  2,
				"memory": 2048,
			},
			want: nil,
		},
		{
			name: "rootfs with explicit mountpoint",
			input: map[string]interface{}{
				"rootfs": "local:100/vm-100-disk-0.raw,mp=/,size=8G",
			},
			want: map[string]containerMountMetadata{
				"rootfs": {
					Key:        "rootfs",
					Mountpoint: "/",
					Source:     "local:100/vm-100-disk-0.raw",
				},
			},
		},
		{
			name: "mount point with mountpoint key",
			input: map[string]interface{}{
				"mp0": "local:volume,mountpoint=/mnt/data",
			},
			want: map[string]containerMountMetadata{
				"mp0": {
					Key:        "mp0",
					Mountpoint: "/mnt/data",
					Source:     "local:volume",
				},
			},
		},
		{
			name: "empty value ignored",
			input: map[string]interface{}{
				"mp0": "",
			},
			want: nil,
		},
		{
			name: "whitespace trimmed",
			input: map[string]interface{}{
				"mp0": "  local:volume  ,  mp=/mnt/data  ",
			},
			want: map[string]containerMountMetadata{
				"mp0": {
					Key:        "mp0",
					Mountpoint: "/mnt/data",
					Source:     "local:volume",
				},
			},
		},
		{
			name: "part without equals sign skipped",
			input: map[string]interface{}{
				"mp0": "local:volume,readonly,mp=/data,backup",
			},
			want: map[string]containerMountMetadata{
				"mp0": {
					Key:        "mp0",
					Mountpoint: "/data",
					Source:     "local:volume",
				},
			},
		},
		{
			name: "rootfs without mountpoint defaults to slash",
			input: map[string]interface{}{
				"rootfs": "local:100/vm-100-disk-0.raw,size=8G",
			},
			want: map[string]containerMountMetadata{
				"rootfs": {
					Key:        "rootfs",
					Mountpoint: "/",
					Source:     "local:100/vm-100-disk-0.raw",
				},
			},
		},
		{
			name: "non-rootfs without mountpoint has empty mountpoint",
			input: map[string]interface{}{
				"mp1": "local:volume,size=10G",
			},
			want: map[string]containerMountMetadata{
				"mp1": {
					Key:        "mp1",
					Mountpoint: "",
					Source:     "local:volume",
				},
			},
		},
		{
			name: "key case insensitive",
			input: map[string]interface{}{
				"ROOTFS": "local:disk,size=8G",
				"MP0":    "local:vol,mp=/mnt",
			},
			want: map[string]containerMountMetadata{
				"rootfs": {
					Key:        "rootfs",
					Mountpoint: "/",
					Source:     "local:disk",
				},
				"mp0": {
					Key:        "mp0",
					Mountpoint: "/mnt",
					Source:     "local:vol",
				},
			},
		},
		{
			name: "whitespace-only value treated as empty",
			input: map[string]interface{}{
				"mp0": "   ",
			},
			want: nil,
		},
		{
			name: "single source value no comma parts",
			input: map[string]interface{}{
				"rootfs": "local:disk",
			},
			want: map[string]containerMountMetadata{
				"rootfs": {
					Key:        "rootfs",
					Mountpoint: "/",
					Source:     "local:disk",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseContainerMountMetadata(tt.input)
			if !mountMetadataMapsEqual(got, tt.want) {
				t.Errorf("parseContainerMountMetadata() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractContainerRootDeviceFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]interface{}
		want  string
	}{
		{
			name:  "empty config",
			input: map[string]interface{}{},
			want:  "",
		},
		{
			name:  "nil config",
			input: nil,
			want:  "",
		},
		{
			name: "no rootfs key",
			input: map[string]interface{}{
				"cores": 2,
			},
			want: "",
		},
		{
			name: "rootfs with device only",
			input: map[string]interface{}{
				"rootfs": "local:100/vm-100-disk-0.raw",
			},
			want: "local:100/vm-100-disk-0.raw",
		},
		{
			name: "rootfs with device and options",
			input: map[string]interface{}{
				"rootfs": "local:100/vm-100-disk-0.raw,size=8G",
			},
			want: "local:100/vm-100-disk-0.raw",
		},
		{
			name: "rootfs empty value",
			input: map[string]interface{}{
				"rootfs": "",
			},
			want: "",
		},
		{
			name: "rootfs whitespace only",
			input: map[string]interface{}{
				"rootfs": "   ",
			},
			want: "",
		},
		{
			name: "rootfs with whitespace",
			input: map[string]interface{}{
				"rootfs": "  local:100/vm-100-disk-0.raw  ,size=8G",
			},
			want: "local:100/vm-100-disk-0.raw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractContainerRootDeviceFromConfig(tt.input)
			if got != tt.want {
				t.Errorf("extractContainerRootDeviceFromConfig() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeContainerNetworkInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target []models.GuestNetworkInterface
		detail containerNetworkDetails
		want   []models.GuestNetworkInterface
	}{
		{
			name:   "nil target",
			target: nil,
			detail: containerNetworkDetails{
				Name: "eth0",
			},
			want: nil,
		},
		{
			name:   "empty target append new",
			target: []models.GuestNetworkInterface{},
			detail: containerNetworkDetails{
				Name:      "eth0",
				MAC:       "AA:BB:CC:DD:EE:FF",
				Addresses: []string{"192.168.1.100"},
			},
			want: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					MAC:       "AA:BB:CC:DD:EE:FF",
					Addresses: []string{"192.168.1.100"},
				},
			},
		},
		{
			name: "match by name merge MAC",
			target: []models.GuestNetworkInterface{
				{
					Name: "eth0",
				},
			},
			detail: containerNetworkDetails{
				Name: "eth0",
				MAC:  "AA:BB:CC:DD:EE:FF",
			},
			want: []models.GuestNetworkInterface{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "match by MAC merge name",
			target: []models.GuestNetworkInterface{
				{
					MAC: "AA:BB:CC:DD:EE:FF",
				},
			},
			detail: containerNetworkDetails{
				Name: "eth0",
				MAC:  "aa:bb:cc:dd:ee:ff", // case insensitive
			},
			want: []models.GuestNetworkInterface{
				{
					Name: "eth0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "match by name merge addresses",
			target: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100"},
				},
			},
			detail: containerNetworkDetails{
				Name:      "eth0",
				Addresses: []string{"192.168.1.101"},
			},
			want: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100", "192.168.1.101"},
				},
			},
		},
		{
			name: "deduplicate addresses",
			target: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100"},
				},
			},
			detail: containerNetworkDetails{
				Name:      "eth0",
				Addresses: []string{"192.168.1.100", "192.168.1.101"},
			},
			want: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					Addresses: []string{"192.168.1.100", "192.168.1.101"},
				},
			},
		},
		{
			name: "no match append",
			target: []models.GuestNetworkInterface{
				{
					Name: "eth0",
				},
			},
			detail: containerNetworkDetails{
				Name: "eth1",
				MAC:  "BB:BB:BB:BB:BB:BB",
			},
			want: []models.GuestNetworkInterface{
				{
					Name: "eth0",
				},
				{
					Name: "eth1",
					MAC:  "BB:BB:BB:BB:BB:BB",
				},
			},
		},
		{
			name: "match by name case insensitive",
			target: []models.GuestNetworkInterface{
				{
					Name: "ETH0",
				},
			},
			detail: containerNetworkDetails{
				Name: "eth0",
				MAC:  "AA:BB:CC:DD:EE:FF",
			},
			want: []models.GuestNetworkInterface{
				{
					Name: "ETH0",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "don't overwrite existing name",
			target: []models.GuestNetworkInterface{
				{
					Name: "existing",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
			detail: containerNetworkDetails{
				Name: "new",
				MAC:  "aa:bb:cc:dd:ee:ff",
			},
			want: []models.GuestNetworkInterface{
				{
					Name: "existing",
					MAC:  "AA:BB:CC:DD:EE:FF",
				},
			},
		},
		{
			name: "don't overwrite existing MAC",
			target: []models.GuestNetworkInterface{
				{
					Name: "eth0",
					MAC:  "AA:AA:AA:AA:AA:AA",
				},
			},
			detail: containerNetworkDetails{
				Name: "eth0",
				MAC:  "BB:BB:BB:BB:BB:BB",
			},
			want: []models.GuestNetworkInterface{
				{
					Name: "eth0",
					MAC:  "AA:AA:AA:AA:AA:AA",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var target *[]models.GuestNetworkInterface
			if tt.target != nil {
				// Make a copy to avoid test interference
				targetCopy := make([]models.GuestNetworkInterface, len(tt.target))
				copy(targetCopy, tt.target)
				target = &targetCopy
			}

			mergeContainerNetworkInterface(target, tt.detail)

			var got []models.GuestNetworkInterface
			if target != nil {
				got = *target
			}

			if !guestNetworkInterfaceSlicesEqual(got, tt.want) {
				t.Errorf("mergeContainerNetworkInterface() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestConvertContainerDiskInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   *proxmox.Container
		metadata map[string]containerMountMetadata
		want     []models.Disk
	}{
		{
			name:   "nil status",
			status: nil,
			want:   nil,
		},
		{
			name: "empty DiskInfo",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{},
			},
			want: nil,
		},
		{
			name: "nil DiskInfo",
			status: &proxmox.Container{
				DiskInfo: nil,
			},
			want: nil,
		},
		{
			name: "single rootfs disk",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"rootfs": {
						Total: 8589934592, // 8GB
						Used:  4294967296, // 4GB
					},
				},
				RootFS: "local:100/vm-100-disk-0.raw",
			},
			want: []models.Disk{
				{
					Total:      8589934592,
					Used:       4294967296,
					Free:       4294967296,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
					Device:     "local:100/vm-100-disk-0.raw",
				},
			},
		},
		{
			name: "rootfs with metadata",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"rootfs": {
						Total: 8589934592,
						Used:  4294967296,
					},
				},
			},
			metadata: map[string]containerMountMetadata{
				"rootfs": {
					Mountpoint: "/",
					Source:     "local-lvm:vm-100-disk-0",
				},
			},
			want: []models.Disk{
				{
					Total:      8589934592,
					Used:       4294967296,
					Free:       4294967296,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
					Device:     "local-lvm:vm-100-disk-0",
				},
			},
		},
		{
			name: "multiple disks sorted by mountpoint",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"mp0": {
						Total: 10737418240,
						Used:  5368709120,
					},
					"rootfs": {
						Total: 8589934592,
						Used:  4294967296,
					},
				},
			},
			metadata: map[string]containerMountMetadata{
				"mp0": {
					Mountpoint: "/mnt/data",
					Source:     "local:volume1",
				},
				"rootfs": {
					Mountpoint: "/",
					Source:     "local:100/vm-100-disk-0.raw",
				},
			},
			want: []models.Disk{
				{
					Total:      8589934592,
					Used:       4294967296,
					Free:       4294967296,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
					Device:     "local:100/vm-100-disk-0.raw",
				},
				{
					Total:      10737418240,
					Used:       5368709120,
					Free:       5368709120,
					Usage:      50.0,
					Mountpoint: "/mnt/data",
					Type:       "mp0",
					Device:     "local:volume1",
				},
			},
		},
		{
			name: "disk with used > total clamped",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"rootfs": {
						Total: 8589934592,
						Used:  10737418240, // More than total
					},
				},
			},
			want: []models.Disk{
				{
					Total:      8589934592,
					Used:       8589934592, // Clamped to total
					Free:       0,
					Usage:      100.0,
					Mountpoint: "/",
					Type:       "rootfs",
				},
			},
		},
		{
			name: "disk with zero total",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"rootfs": {
						Total: 0,
						Used:  0,
					},
				},
			},
			want: []models.Disk{
				{
					Total:      0,
					Used:       0,
					Free:       0,
					Usage:      0,
					Mountpoint: "/",
					Type:       "rootfs",
				},
			},
		},
		{
			name: "empty label defaults to rootfs",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"": {
						Total: 8589934592,
						Used:  4294967296,
					},
				},
			},
			want: []models.Disk{
				{
					Total:      8589934592,
					Used:       4294967296,
					Free:       4294967296,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
				},
			},
		},
		{
			name: "non-rootfs disk without metadata uses label as mountpoint",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"mp0": {
						Total: 10737418240,
						Used:  5368709120,
					},
				},
			},
			want: []models.Disk{
				{
					Total:      10737418240,
					Used:       5368709120,
					Free:       5368709120,
					Usage:      50.0,
					Mountpoint: "mp0",
					Type:       "mp0",
				},
			},
		},
		{
			name: "case insensitive rootfs",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"ROOTFS": {
						Total: 8589934592,
						Used:  4294967296,
					},
				},
			},
			want: []models.Disk{
				{
					Total:      8589934592,
					Used:       4294967296,
					Free:       4294967296,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
				},
			},
		},
		{
			name: "nil metadata does not panic",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"rootfs": {
						Total: 1000,
						Used:  500,
					},
				},
				RootFS: "local:disk",
			},
			metadata: nil,
			want: []models.Disk{
				{
					Total:      1000,
					Used:       500,
					Free:       500,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
					Device:     "local:disk",
				},
			},
		},
		{
			name: "disk gets device from metadata when not set",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"mp0": {
						Total: 1000,
						Used:  500,
					},
				},
			},
			metadata: map[string]containerMountMetadata{
				"mp0": {
					Mountpoint: "/data",
					Source:     "nfs:shared-volume",
				},
			},
			want: []models.Disk{
				{
					Total:      1000,
					Used:       500,
					Free:       500,
					Usage:      50.0,
					Mountpoint: "/data",
					Type:       "mp0",
					Device:     "nfs:shared-volume",
				},
			},
		},
		{
			name: "negative free clamped to zero",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"rootfs": {
						Total: 0,
						Used:  500, // used > total=0, free = -500 clamped to 0
					},
				},
			},
			want: []models.Disk{
				{
					Total:      0,
					Used:       500, // Not clamped because total == 0
					Free:       0,   // Clamped from -500 to 0
					Usage:      0,   // No calculation when total == 0
					Mountpoint: "/",
					Type:       "rootfs",
				},
			},
		},
		{
			name: "whitespace label trimmed and treated as rootfs",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"  ": { // whitespace only, trims to empty
						Total: 1000,
						Used:  500,
					},
				},
			},
			want: []models.Disk{
				{
					Total:      1000,
					Used:       500,
					Free:       500,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
				},
			},
		},
		{
			name: "rootfs gets device from RootFS when metadata has empty source",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"rootfs": {
						Total: 1000,
						Used:  500,
					},
				},
				RootFS: "local:100/disk.raw,size=8G",
			},
			metadata: map[string]containerMountMetadata{
				"rootfs": {
					Mountpoint: "/",
					Source:     "", // empty source
				},
			},
			want: []models.Disk{
				{
					Total:      1000,
					Used:       500,
					Free:       500,
					Usage:      50.0,
					Mountpoint: "/",
					Type:       "rootfs",
					Device:     "local:100/disk.raw", // Falls back to RootFS, sanitized
				},
			},
		},
		{
			name: "non-rootfs with whitespace label gets type disk",
			status: &proxmox.Container{
				DiskInfo: map[string]proxmox.ContainerDiskUsage{
					"mp0": {
						Total: 1000,
						Used:  500,
					},
				},
			},
			metadata: map[string]containerMountMetadata{
				"mp0": {
					Mountpoint: "/mnt/storage",
					Source:     "",
				},
			},
			want: []models.Disk{
				{
					Total:      1000,
					Used:       500,
					Free:       500,
					Usage:      50.0,
					Mountpoint: "/mnt/storage",
					Type:       "mp0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := convertContainerDiskInfo(tt.status, tt.metadata)
			if !diskSlicesEqual(got, tt.want) {
				t.Errorf("convertContainerDiskInfo() =\n%+v\nwant:\n%+v", got, tt.want)
			}
		})
	}
}

func TestEnsureContainerRootDiskEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		container *models.Container
		want      *models.Container
	}{
		{
			name:      "nil container",
			container: nil,
			want:      nil,
		},
		{
			name: "already has disks",
			container: &models.Container{
				Disks: []models.Disk{
					{
						Mountpoint: "/mnt/data",
						Total:      1000,
						Used:       500,
					},
				},
			},
			want: &models.Container{
				Disks: []models.Disk{
					{
						Mountpoint: "/mnt/data",
						Total:      1000,
						Used:       500,
					},
				},
			},
		},
		{
			name: "no disks creates root entry",
			container: &models.Container{
				Disk: models.Disk{
					Total: 8589934592,
					Used:  4294967296,
					Usage: 50.0,
				},
			},
			want: &models.Container{
				Disk: models.Disk{
					Total: 8589934592,
					Used:  4294967296,
					Usage: 50.0,
				},
				Disks: []models.Disk{
					{
						Total:      8589934592,
						Used:       4294967296,
						Free:       4294967296,
						Usage:      50.0,
						Mountpoint: "/",
						Type:       "rootfs",
					},
				},
			},
		},
		{
			name: "used > total clamped",
			container: &models.Container{
				Disk: models.Disk{
					Total: 8589934592,
					Used:  10737418240, // More than total
				},
			},
			want: &models.Container{
				Disk: models.Disk{
					Total: 8589934592,
					Used:  10737418240,
				},
				Disks: []models.Disk{
					{
						Total:      8589934592,
						Used:       8589934592, // Clamped
						Free:       0,
						Usage:      100.0,
						Mountpoint: "/",
						Type:       "rootfs",
					},
				},
			},
		},
		{
			name: "zero usage calculated",
			container: &models.Container{
				Disk: models.Disk{
					Total: 8589934592,
					Used:  4294967296,
					Usage: 0, // Will be calculated
				},
			},
			want: &models.Container{
				Disk: models.Disk{
					Total: 8589934592,
					Used:  4294967296,
					Usage: 0,
				},
				Disks: []models.Disk{
					{
						Total:      8589934592,
						Used:       4294967296,
						Free:       4294967296,
						Usage:      50.0, // Calculated
						Mountpoint: "/",
						Type:       "rootfs",
					},
				},
			},
		},
		{
			name: "zero total",
			container: &models.Container{
				Disk: models.Disk{
					Total: 0,
					Used:  0,
				},
			},
			want: &models.Container{
				Disk: models.Disk{
					Total: 0,
					Used:  0,
				},
				Disks: []models.Disk{
					{
						Total:      0,
						Used:       0,
						Free:       0,
						Usage:      0,
						Mountpoint: "/",
						Type:       "rootfs",
					},
				},
			},
		},
		{
			name: "used greater than total gets clamped when total positive",
			container: &models.Container{
				Disk: models.Disk{
					Total: 1000,
					Used:  1500, // More than total, will be clamped to 1000
				},
			},
			want: &models.Container{
				Disk: models.Disk{
					Total: 1000,
					Used:  1500,
				},
				Disks: []models.Disk{
					{
						Total:      1000,
						Used:       1000, // Clamped to total
						Free:       0,
						Usage:      100.0,
						Mountpoint: "/",
						Type:       "rootfs",
					},
				},
			},
		},
		{
			name: "negative free clamped to zero when total is zero but used is positive",
			container: &models.Container{
				Disk: models.Disk{
					Total: 0,
					Used:  500, // Used > 0, total = 0, so free = 0 - 500 = -500, clamped to 0
				},
			},
			want: &models.Container{
				Disk: models.Disk{
					Total: 0,
					Used:  500,
				},
				Disks: []models.Disk{
					{
						Total:      0,
						Used:       500, // Not clamped because total == 0 (clamping only when total > 0)
						Free:       0,   // Clamped from -500 to 0
						Usage:      0,   // No calculation when total == 0
						Mountpoint: "/",
						Type:       "rootfs",
					},
				},
			},
		},
		{
			name: "usage already set not recalculated",
			container: &models.Container{
				Disk: models.Disk{
					Total: 1000,
					Used:  500,
					Usage: 75.0, // Already set (even if wrong), should not be recalculated
				},
			},
			want: &models.Container{
				Disk: models.Disk{
					Total: 1000,
					Used:  500,
					Usage: 75.0,
				},
				Disks: []models.Disk{
					{
						Total:      1000,
						Used:       500,
						Free:       500,
						Usage:      75.0, // Preserved from original
						Mountpoint: "/",
						Type:       "rootfs",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Make a copy to avoid test interference
			var container *models.Container
			if tt.container != nil {
				containerCopy := *tt.container
				if tt.container.Disks != nil {
					containerCopy.Disks = make([]models.Disk, len(tt.container.Disks))
					copy(containerCopy.Disks, tt.container.Disks)
				}
				container = &containerCopy
			}

			ensureContainerRootDiskEntry(container)

			if !containersEqual(container, tt.want) {
				t.Errorf("ensureContainerRootDiskEntry() =\n%+v\nwant:\n%+v", container, tt.want)
			}
		})
	}
}

// Helper functions for test comparisons
// Note: stringSlicesEqual already exists in helpers_test.go

func networkDetailsSlicesEqual(a, b []containerNetworkDetails) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].MAC != b[i].MAC {
			return false
		}
		if !stringSlicesEqual(a[i].Addresses, b[i].Addresses) {
			return false
		}
	}
	return true
}

func mountMetadataMapsEqual(a, b map[string]containerMountMetadata) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	for key, valA := range a {
		valB, ok := b[key]
		if !ok {
			return false
		}
		if valA.Key != valB.Key || valA.Mountpoint != valB.Mountpoint || valA.Source != valB.Source {
			return false
		}
	}
	return true
}

func guestNetworkInterfaceSlicesEqual(a, b []models.GuestNetworkInterface) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].MAC != b[i].MAC {
			return false
		}
		if !stringSlicesEqual(a[i].Addresses, b[i].Addresses) {
			return false
		}
	}
	return true
}

func diskSlicesEqual(a, b []models.Disk) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	for i := range a {
		if !disksEqual(&a[i], &b[i]) {
			return false
		}
	}
	return true
}

func disksEqual(a, b *models.Disk) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Total == b.Total &&
		a.Used == b.Used &&
		a.Free == b.Free &&
		floatsEqual(a.Usage, b.Usage) &&
		a.Mountpoint == b.Mountpoint &&
		a.Type == b.Type &&
		a.Device == b.Device
}

func floatsEqual(a, b float64) bool {
	const epsilon = 0.0001
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

func containersEqual(a, b *models.Container) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Disk.Total != b.Disk.Total || a.Disk.Used != b.Disk.Used || !floatsEqual(a.Disk.Usage, b.Disk.Usage) {
		return false
	}
	return diskSlicesEqual(a.Disks, b.Disks)
}
