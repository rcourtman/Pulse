package monitoring

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

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

func TestSortContent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "single value", input: "images", want: "images"},
		{name: "already sorted", input: "backup,images,rootdir", want: "backup,images,rootdir"},
		{name: "unsorted values", input: "rootdir,images,backup", want: "backup,images,rootdir"},
		{name: "reverse sorted", input: "vztmpl,rootdir,images,backup", want: "backup,images,rootdir,vztmpl"},
		{name: "duplicates preserved", input: "images,backup,images", want: "backup,images,images"},
		{name: "single character values", input: "c,a,b", want: "a,b,c"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sortContent(tc.input); got != tc.want {
				t.Fatalf("sortContent(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatSeconds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		total int
		want  string
	}{
		{name: "zero", total: 0, want: ""},
		{name: "negative", total: -1, want: ""},
		{name: "one second", total: 1, want: "00:00:01"},
		{name: "one minute", total: 60, want: "00:01:00"},
		{name: "one hour", total: 3600, want: "01:00:00"},
		{name: "mixed time", total: 3661, want: "01:01:01"},
		{name: "59 seconds", total: 59, want: "00:00:59"},
		{name: "59 minutes 59 seconds", total: 3599, want: "00:59:59"},
		{name: "many hours", total: 36000, want: "10:00:00"},
		{name: "over 24 hours", total: 90061, want: "25:01:01"},
		{name: "complex time", total: 7384, want: "02:03:04"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatSeconds(tc.total); got != tc.want {
				t.Fatalf("formatSeconds(%d) = %q, want %q", tc.total, got, tc.want)
			}
		})
	}
}

func TestDedupeStringsPreserveOrder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{name: "nil input", input: nil, want: nil},
		{name: "empty slice", input: []string{}, want: nil},
		{name: "single value", input: []string{"a"}, want: []string{"a"}},
		{name: "no duplicates", input: []string{"a", "b", "c"}, want: []string{"a", "b", "c"}},
		{name: "with duplicates", input: []string{"a", "b", "a", "c", "b"}, want: []string{"a", "b", "c"}},
		{name: "all duplicates", input: []string{"x", "x", "x"}, want: []string{"x"}},
		{name: "preserves order", input: []string{"c", "a", "b", "a"}, want: []string{"c", "a", "b"}},
		{name: "empty strings filtered", input: []string{"a", "", "b", "  ", "c"}, want: []string{"a", "b", "c"}},
		{name: "whitespace trimmed", input: []string{"  a  ", "a", " b "}, want: []string{"a", "b"}},
		{name: "only empty strings", input: []string{"", "  ", "   "}, want: nil},
		{name: "mixed empty and values", input: []string{"", "a", "", "a", ""}, want: []string{"a"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := dedupeStringsPreserveOrder(tc.input)
			if !stringSlicesEqual(got, tc.want) {
				t.Fatalf("dedupeStringsPreserveOrder(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeGuestAddressStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty string", input: "", want: nil},
		{name: "whitespace only", input: "   ", want: nil},
		{name: "valid ipv4", input: "192.168.1.100", want: []string{"192.168.1.100"}},
		{name: "valid ipv6", input: "2001:db8::1", want: []string{"2001:db8::1"}},
		{name: "dhcp placeholder", input: "dhcp", want: nil},
		{name: "DHCP uppercase", input: "DHCP", want: nil},
		{name: "manual placeholder", input: "manual", want: nil},
		{name: "static placeholder", input: "static", want: nil},
		{name: "auto placeholder", input: "auto", want: nil},
		{name: "none placeholder", input: "none", want: nil},
		{name: "n/a placeholder", input: "n/a", want: nil},
		{name: "unknown placeholder", input: "unknown", want: nil},
		{name: "zero ipv4", input: "0.0.0.0", want: nil},
		{name: "zero ipv6", input: "::", want: nil},
		{name: "loopback ipv6", input: "::1", want: nil},
		{name: "loopback ipv4", input: "127.0.0.1", want: nil},
		{name: "loopback subnet", input: "127.0.0.2", want: nil},
		{name: "link local ipv6", input: "fe80::1", want: nil},
		{name: "link local with zone", input: "fe80::1%eth0", want: nil},
		{name: "ip with cidr", input: "192.168.1.100/24", want: []string{"192.168.1.100"}},
		{name: "ipv6 with cidr", input: "2001:db8::1/64", want: []string{"2001:db8::1"}},
		{name: "comma separated", input: "192.168.1.1,192.168.1.2", want: []string{"192.168.1.1", "192.168.1.2"}},
		{name: "semicolon separated", input: "192.168.1.1;192.168.1.2", want: []string{"192.168.1.1", "192.168.1.2"}},
		{name: "space separated", input: "192.168.1.1 192.168.1.2", want: []string{"192.168.1.1", "192.168.1.2"}},
		{name: "mixed valid and invalid", input: "192.168.1.1,dhcp,10.0.0.1", want: []string{"192.168.1.1", "10.0.0.1"}},
		{name: "filters loopback from list", input: "192.168.1.1,127.0.0.1,10.0.0.1", want: []string{"192.168.1.1", "10.0.0.1"}},
		{name: "ipv6 zone identifier stripped", input: "2001:db8::1%eth0", want: []string{"2001:db8::1"}},
		{name: "whitespace trimmed", input: "  192.168.1.100  ", want: []string{"192.168.1.100"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeGuestAddressStrings(tc.input)
			if !stringSlicesEqual(got, tc.want) {
				t.Fatalf("sanitizeGuestAddressStrings(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestCopyFloatPointer(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		if got := copyFloatPointer(nil); got != nil {
			t.Fatalf("copyFloatPointer(nil) = %v, want nil", got)
		}
	})

	t.Run("copies value", func(t *testing.T) {
		t.Parallel()
		original := 42.5
		copy := copyFloatPointer(&original)
		if copy == nil {
			t.Fatal("copyFloatPointer returned nil for non-nil input")
		}
		if *copy != original {
			t.Fatalf("copyFloatPointer value = %v, want %v", *copy, original)
		}
	})

	t.Run("independent copy", func(t *testing.T) {
		t.Parallel()
		original := 100.0
		copy := copyFloatPointer(&original)
		original = 200.0
		if *copy != 100.0 {
			t.Fatalf("copy was modified when original changed: got %v, want 100.0", *copy)
		}
	})

	t.Run("different pointer", func(t *testing.T) {
		t.Parallel()
		original := 50.0
		copy := copyFloatPointer(&original)
		if copy == &original {
			t.Fatal("copyFloatPointer returned same pointer as input")
		}
	})
}

func TestClampInterval(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value time.Duration
		min   time.Duration
		max   time.Duration
		want  time.Duration
	}{
		{name: "within range", value: 30 * time.Second, min: 10 * time.Second, max: 60 * time.Second, want: 30 * time.Second},
		{name: "below min", value: 5 * time.Second, min: 10 * time.Second, max: 60 * time.Second, want: 10 * time.Second},
		{name: "above max", value: 120 * time.Second, min: 10 * time.Second, max: 60 * time.Second, want: 60 * time.Second},
		{name: "at min boundary", value: 10 * time.Second, min: 10 * time.Second, max: 60 * time.Second, want: 10 * time.Second},
		{name: "at max boundary", value: 60 * time.Second, min: 10 * time.Second, max: 60 * time.Second, want: 60 * time.Second},
		{name: "zero value below min", value: 0, min: 10 * time.Second, max: 60 * time.Second, want: 10 * time.Second},
		{name: "negative below min", value: -5 * time.Second, min: 10 * time.Second, max: 60 * time.Second, want: 10 * time.Second},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := clampInterval(tc.value, tc.min, tc.max); got != tc.want {
				t.Fatalf("clampInterval(%v, %v, %v) = %v, want %v", tc.value, tc.min, tc.max, got, tc.want)
			}
		})
	}
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
