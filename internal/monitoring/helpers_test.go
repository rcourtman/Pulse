package monitoring

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestNormalizeEndpointHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  node.local  ", "node.local"},
		{"https://node.local:8006/", "node.local"},
		{"node.local:8006", "node.local"},
		{"node.local/path", "node.local"},
		{"https://[2001:db8::1]:8006", "2001:db8::1"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeEndpointHost(tc.input); got != tc.want {
				t.Fatalf("normalizeEndpointHost(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsLikelyIPAddress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"example.local", false},
		{"10.0.0.1", true},
		{"2001:db8::1", true},
		{"fe80::1%eth0", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.value, func(t *testing.T) {
			t.Parallel()
			if got := isLikelyIPAddress(tc.value); got != tc.want {
				t.Fatalf("isLikelyIPAddress(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestGetNodeDisplayName(t *testing.T) {
	t.Parallel()

	clusterInstance := &config.PVEInstance{
		IsCluster: true,
		Name:      "cluster",
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "node1", Host: "https://node1.local:8006"},
			{NodeName: "node2", Host: "", IP: "10.0.0.2"},
		},
	}

	cases := []struct {
		name     string
		instance *config.PVEInstance
		node     string
		want     string
	}{
		{"nil instance trims", nil, "  nodeX  ", "nodeX"},
		{"friendly standalone", &config.PVEInstance{Name: "Friendly"}, "nodeA", "Friendly"},
		{"host fallback", &config.PVEInstance{Host: "https://host.local:8006"}, "unknown-node", "host.local"},
		{"cluster host label", clusterInstance, "node1", "node1.local"},
		{"cluster ip fallback", clusterInstance, "node2", "node2"},
		{"cluster base fallback", clusterInstance, "node3", "node3"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := getNodeDisplayName(tc.instance, tc.node); got != tc.want {
				t.Fatalf("getNodeDisplayName(%v, %q) = %q, want %q", tc.instance, tc.node, got, tc.want)
			}
		})
	}
}

func TestMergeNVMeTempsIntoDisks(t *testing.T) {
	t.Parallel()

	original := []models.PhysicalDisk{
		{Node: "nodeA", Instance: "inst", DevPath: "/dev/nvme1n1", Type: "nvme", Temperature: 0},
		{Node: "nodeA", Instance: "inst", DevPath: "/dev/nvme0n1", Type: "NVME", Temperature: 0},
		{Node: "nodeB", Instance: "inst", DevPath: "/dev/sda", Type: "sata", Temperature: 45},
	}

	nodes := []models.Node{
		{
			Name: "nodeA",
			Temperature: &models.Temperature{
				Available: true,
				NVMe: []models.NVMeTemp{
					{Device: "nvme0n1", Temp: 30.4},
					{Device: "nvme1n1", Temp: 31.6},
				},
			},
		},
	}

	merged := mergeNVMeTempsIntoDisks(original, nodes)

	if got, want := merged[0].Temperature, 32; got != want {
		t.Fatalf("disk 0 temperature = %d, want %d", got, want)
	}
	if got, want := merged[1].Temperature, 30; got != want {
		t.Fatalf("disk 1 temperature = %d, want %d", got, want)
	}
	if got, want := merged[2].Temperature, 45; got != want {
		t.Fatalf("non-nvme disk temperature changed: got %d want %d", got, want)
	}
	if got := original[0].Temperature; got != 0 {
		t.Fatalf("expected original slice unchanged, got %d", got)
	}
}

func TestMergeNVMeTempsIntoDisksClearsMissingOrInvalid(t *testing.T) {
	t.Parallel()

	disks := []models.PhysicalDisk{
		{Node: "nodeA", Instance: "inst", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
		{Node: "nodeC", Instance: "inst", DevPath: "/dev/nvme1n1", Type: "nvme", Temperature: 0},
	}

	nodes := []models.Node{
		{
			Name: "nodeA",
			Temperature: &models.Temperature{
				Available: true,
				NVMe: []models.NVMeTemp{
					{Device: "nvme0n1", Temp: math.NaN()},
				},
			},
		},
	}

	merged := mergeNVMeTempsIntoDisks(disks, nodes)

	if got := merged[0].Temperature; got != 0 {
		t.Fatalf("expected NaN temp to reset to 0, got %d", got)
	}
	if got := merged[1].Temperature; got != 0 {
		t.Fatalf("expected missing temps to reset to 0, got %d", got)
	}
}

func TestSafePercentage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		used, total float64
		want        float64
	}{
		{50, 100, 50},
		{0, 0, 0},
		{math.NaN(), 100, 0},
		{75, math.NaN(), 0},
		{10, 0, 0},
		{math.Inf(1), 100, 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%v/%v", tc.used, tc.total), func(t *testing.T) {
			t.Parallel()
			if got := safePercentage(tc.used, tc.total); got != tc.want {
				t.Fatalf("safePercentage(%v, %v) = %v, want %v", tc.used, tc.total, got, tc.want)
			}
		})
	}
}

func TestSafeFloat(t *testing.T) {
	t.Parallel()

	if got := safeFloat(math.NaN()); got != 0 {
		t.Fatalf("expected NaN to return 0, got %v", got)
	}
	if got := safeFloat(math.Inf(1)); got != 0 {
		t.Fatalf("expected +Inf to return 0, got %v", got)
	}
	if got := safeFloat(42.5); got != 42.5 {
		t.Fatalf("expected value preserved, got %v", got)
	}
}

func TestMaxInt64(t *testing.T) {
	t.Parallel()

	if got := maxInt64(5, 10); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}
	if got := maxInt64(-1, -5); got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
}

func TestConvertPoolInfoToModel(t *testing.T) {
	t.Parallel()

	info := proxmox.ZFSPoolInfo{
		Name:   "tank",
		Health: "ONLINE",
		State:  "ONLINE",
		Status: "OK",
		Scan:   "none requested",
		Devices: []proxmox.ZFSPoolDevice{
			{
				Name:  "mirror-0",
				State: "ONLINE",
				Leaf:  0,
				Children: []proxmox.ZFSPoolDevice{
					{
						Name:  "nvme0n1",
						State: "ONLINE",
						Leaf:  1,
						Read:  1,
						Write: 2,
						Cksum: 3,
					},
					{
						Name:  "nvme1n1",
						State: "ONLINE",
						Leaf:  1,
						Read:  4,
						Write: 5,
						Cksum: 6,
					},
				},
			},
		},
	}

	model := convertPoolInfoToModel(&info)
	if model == nil {
		t.Fatalf("expected pool model, got nil")
	}
	if model.Name != "tank" {
		t.Fatalf("expected pool name tank, got %s", model.Name)
	}
	if model.State != "ONLINE" {
		t.Fatalf("expected ONLINE state, got %s", model.State)
	}
	if len(model.Devices) != 2 {
		t.Fatalf("expected 2 leaf devices, got %d", len(model.Devices))
	}
	if model.ReadErrors != 5 || model.WriteErrors != 7 || model.ChecksumErrors != 9 {
		t.Fatalf("unexpected error totals: read=%d write=%d checksum=%d", model.ReadErrors, model.WriteErrors, model.ChecksumErrors)
	}
}

func TestConvertPoolInfoToModelNil(t *testing.T) {
	t.Parallel()

	if model := convertPoolInfoToModel(nil); model != nil {
		t.Fatalf("expected nil result for nil input")
	}
}

func TestIsGuestAgentOSInfoUnsupportedError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "unrelated error", err: errors.New("guest agent timeout"), want: false},
		{
			name: "missing os-release path",
			err:  errors.New(`API error 500: {"errors":{"message":"guest agent command failed: Failed to open file '/etc/os-release': No such file or directory"}}`),
			want: true,
		},
		{
			name: "missing usr lib os-release",
			err:  errors.New("API error 500: guest agent command failed: Failed to open file '/usr/lib/os-release': No such file or directory"),
			want: true,
		},
		{
			name: "unsupported command",
			err:  errors.New("API error 500: unsupported command: guest-get-osinfo"),
			want: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isGuestAgentOSInfoUnsupportedError(tc.err); got != tc.want {
				t.Fatalf("isGuestAgentOSInfoUnsupportedError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
