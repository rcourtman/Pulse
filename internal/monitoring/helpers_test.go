package monitoring

import (
	"encoding/json"
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
		name  string
		input string
		want  string
	}{
		// Empty and whitespace
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"whitespace with tabs", " \t ", ""},

		// Full URLs with scheme
		{"https URL with port and path", "https://example.com:8006/api", "example.com"},
		{"http URL with path", "http://host/path", "host"},
		{"https URL with trailing slash", "https://node.local:8006/", "node.local"},

		// URLs without scheme
		{"host with port", "example.com:8006", "example.com"},
		{"host with port no scheme", "node.local:8006", "node.local"},

		// Hostname only
		{"hostname only", "example.com", "example.com"},
		{"simple hostname", "node.local", "node.local"},

		// IP addresses
		{"IPv4 with port", "192.168.1.1:8006", "192.168.1.1"},
		{"IPv4 only", "192.168.1.100", "192.168.1.100"},
		{"IPv6 bracketed with port", "https://[2001:db8::1]:8006", "2001:db8::1"},

		// Host with path (no scheme)
		{"host with path no scheme", "node.local/path", "node.local"},
		{"host with deep path", "server.example.com/api/v1/resource", "server.example.com"},

		// Edge cases with just scheme prefix
		{"just https prefix", "https://", ""},
		{"just http prefix", "http://", ""},

		// Whitespace trimming
		{"whitespace around hostname", "  node.local  ", "node.local"},
		{"whitespace around URL", "  https://example.com:8006  ", "example.com"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
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
		// nil instance returns trimmed nodeName
		{"nil instance trims", nil, "  nodeX  ", "nodeX"},

		// empty nodeName returns "unknown-node"
		{"empty nodeName", nil, "", "unknown-node"},

		// whitespace-only nodeName returns "unknown-node"
		{"whitespace nodeName", nil, "   ", "unknown-node"},

		// non-cluster: instance.Name takes priority
		{"friendly standalone", &config.PVEInstance{Name: "Friendly"}, "nodeA", "Friendly"},

		// non-cluster: falls back to nodeName when Name empty
		{"non-cluster nodeName fallback", &config.PVEInstance{Name: "", Host: "pve.example.com"}, "node1", "node1"},

		// non-cluster: falls back to host label when nodeName is "unknown-node"
		{"host fallback", &config.PVEInstance{Host: "https://host.local:8006"}, "unknown-node", "host.local"},

		// non-cluster: returns unknown-node when host is IP address
		{"host IP fallback to unknown-node", &config.PVEInstance{Name: "", Host: "https://192.168.1.100:8006"}, "", "unknown-node"},

		// cluster: lookupClusterEndpointLabel result takes priority
		{"cluster host label", clusterInstance, "node1", "node1.local"},

		// cluster: falls back to baseName when no endpoint label
		{"cluster base fallback", clusterInstance, "node3", "node3"},

		// cluster: falls back to nodeName (IP fallback via endpoint)
		{"cluster ip fallback", clusterInstance, "node2", "node2"},

		// cluster: falls back to friendly name when baseName is "unknown-node"
		{"cluster friendly fallback", &config.PVEInstance{IsCluster: true, Name: "Cluster Name", ClusterEndpoints: []config.ClusterEndpoint{}}, "", "Cluster Name"},

		// cluster: returns unknown-node when no fallbacks available
		{"cluster no fallbacks", &config.PVEInstance{IsCluster: true, Name: "", Host: "pve.example.com", ClusterEndpoints: []config.ClusterEndpoint{}}, "", "unknown-node"},
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

// TestMergeNVMeTempsIntoDisks moved to merge_temps_test.go

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

// customStringer is a test type implementing fmt.Stringer for testing the fmt.Stringer case
type customStringer struct {
	value string
}

func (c customStringer) String() string {
	return c.value
}

func TestStringValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input interface{}
		want  string
	}{
		// String inputs
		{name: "plain string", input: "hello", want: "hello"},
		{name: "string with whitespace", input: "  hello  ", want: "hello"},
		{name: "empty string", input: "", want: ""},
		{name: "whitespace only string", input: "   ", want: ""},

		// Numeric inputs - integers
		{name: "int", input: 42, want: "42"},
		{name: "int zero", input: 0, want: "0"},
		{name: "int negative", input: -123, want: "-123"},
		{name: "int32", input: int32(2147483647), want: "2147483647"},
		{name: "int32 negative", input: int32(-1), want: "-1"},
		{name: "int64", input: int64(9223372036854775807), want: "9223372036854775807"},
		{name: "int64 negative", input: int64(-9223372036854775808), want: "-9223372036854775808"},
		{name: "uint32", input: uint32(4294967295), want: "4294967295"},
		{name: "uint64", input: uint64(18446744073709551615), want: "18446744073709551615"},

		// Numeric inputs - floats
		{name: "float64 whole", input: float64(42), want: "42"},
		{name: "float64 decimal", input: float64(3.14159), want: "3.14159"},
		{name: "float64 negative", input: float64(-1.5), want: "-1.5"},
		{name: "float64 zero", input: float64(0), want: "0"},
		{name: "float32 whole", input: float32(42), want: "42"},
		{name: "float32 decimal", input: float32(2.5), want: "2.5"},

		// json.Number
		{name: "json.Number int", input: json.Number("12345"), want: "12345"},
		{name: "json.Number float", input: json.Number("3.14"), want: "3.14"},

		// fmt.Stringer (custom type)
		{name: "fmt.Stringer", input: customStringer{value: "custom"}, want: "custom"},
		{name: "fmt.Stringer with whitespace", input: customStringer{value: "  trimmed  "}, want: "trimmed"},
		{name: "fmt.Stringer empty", input: customStringer{value: ""}, want: ""},

		// Unsupported types
		{name: "nil", input: nil, want: ""},
		{name: "bool true", input: true, want: ""},
		{name: "bool false", input: false, want: ""},
		{name: "slice", input: []int{1, 2, 3}, want: ""},
		{name: "map", input: map[string]int{"a": 1}, want: ""},
		{name: "struct", input: struct{ X int }{X: 1}, want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := stringValue(tc.input); got != tc.want {
				t.Fatalf("stringValue(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestAnyToInt64(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input interface{}
		want  int64
	}{
		// Integer types
		{name: "int positive", input: 42, want: 42},
		{name: "int zero", input: 0, want: 0},
		{name: "int negative", input: -123, want: -123},
		{name: "int32 positive", input: int32(100), want: 100},
		{name: "int32 max", input: int32(2147483647), want: 2147483647},
		{name: "int32 min", input: int32(-2147483648), want: -2147483648},
		{name: "int64 positive", input: int64(9223372036854775807), want: 9223372036854775807},
		{name: "int64 negative", input: int64(-9223372036854775808), want: -9223372036854775808},
		{name: "uint32", input: uint32(4294967295), want: 4294967295},

		// uint64 edge cases
		{name: "uint64 normal", input: uint64(1000), want: 1000},
		{name: "uint64 max int64", input: uint64(9223372036854775807), want: 9223372036854775807},
		{name: "uint64 overflow", input: uint64(18446744073709551615), want: math.MaxInt64},

		// Float types (truncated to int64)
		{name: "float64 whole", input: float64(42), want: 42},
		{name: "float64 truncated", input: float64(3.9), want: 3},
		{name: "float64 negative truncated", input: float64(-2.9), want: -2},
		{name: "float64 zero", input: float64(0), want: 0},
		{name: "float32 whole", input: float32(100), want: 100},
		{name: "float32 truncated", input: float32(5.7), want: 5},

		// String parsing
		{name: "string int", input: "12345", want: 12345},
		{name: "string negative", input: "-999", want: -999},
		{name: "string zero", input: "0", want: 0},
		{name: "string empty", input: "", want: 0},
		{name: "string float", input: "3.14", want: 3},
		{name: "string invalid", input: "abc", want: 0},
		{name: "string mixed", input: "123abc", want: 0},

		// json.Number
		{name: "json.Number int", input: json.Number("67890"), want: 67890},
		{name: "json.Number negative", input: json.Number("-500"), want: -500},
		{name: "json.Number float", input: json.Number("2.718"), want: 2},

		// Unsupported types
		{name: "nil", input: nil, want: 0},
		{name: "bool true", input: true, want: 0},
		{name: "bool false", input: false, want: 0},
		{name: "slice", input: []int{1, 2, 3}, want: 0},
		{name: "map", input: map[string]int{"a": 1}, want: 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := anyToInt64(tc.input); got != tc.want {
				t.Fatalf("anyToInt64(%v) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseInterfaceStat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		stats interface{}
		key   string
		want  int64
	}{
		// Nil and invalid stats
		{name: "nil stats", stats: nil, key: "bytes", want: 0},
		{name: "non-map stats", stats: "not a map", key: "bytes", want: 0},
		{name: "int stats", stats: 123, key: "bytes", want: 0},

		// Missing key
		{name: "missing key", stats: map[string]interface{}{"packets": 100}, key: "bytes", want: 0},
		{name: "empty map", stats: map[string]interface{}{}, key: "bytes", want: 0},

		// Valid keys with various types
		{name: "int value", stats: map[string]interface{}{"bytes": 1000}, key: "bytes", want: 1000},
		{name: "int64 value", stats: map[string]interface{}{"bytes": int64(5000000000)}, key: "bytes", want: 5000000000},
		{name: "float64 value", stats: map[string]interface{}{"bytes": float64(2048.5)}, key: "bytes", want: 2048},
		{name: "string value", stats: map[string]interface{}{"bytes": "4096"}, key: "bytes", want: 4096},
		{name: "json.Number value", stats: map[string]interface{}{"bytes": json.Number("8192")}, key: "bytes", want: 8192},

		// Different keys
		{name: "packets key", stats: map[string]interface{}{"packets": 500, "bytes": 1000}, key: "packets", want: 500},
		{name: "errors key", stats: map[string]interface{}{"errors": 3}, key: "errors", want: 3},

		// Edge cases
		{name: "zero value", stats: map[string]interface{}{"bytes": 0}, key: "bytes", want: 0},
		{name: "negative value", stats: map[string]interface{}{"bytes": -100}, key: "bytes", want: -100},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseInterfaceStat(tc.stats, tc.key); got != tc.want {
				t.Fatalf("parseInterfaceStat(%v, %q) = %d, want %d", tc.stats, tc.key, got, tc.want)
			}
		})
	}
}

func TestExtractGuestOSInfo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		data        map[string]interface{}
		wantName    string
		wantVersion string
	}{
		// Nil and empty
		{name: "nil data", data: nil, wantName: "", wantVersion: ""},
		{name: "empty map", data: map[string]interface{}{}, wantName: "", wantVersion: ""},

		// Standard Linux os-release fields
		{
			name: "standard linux",
			data: map[string]interface{}{
				"name":       "Debian GNU/Linux",
				"version":    "12 (bookworm)",
				"version-id": "12",
			},
			wantName:    "Debian GNU/Linux",
			wantVersion: "12 (bookworm)",
		},
		{
			name: "ubuntu with pretty-name",
			data: map[string]interface{}{
				"name":        "Ubuntu",
				"pretty-name": "Ubuntu 22.04.3 LTS",
				"version":     "22.04.3 LTS (Jammy Jellyfish)",
				"version-id":  "22.04",
			},
			wantName:    "Ubuntu",
			wantVersion: "22.04.3 LTS (Jammy Jellyfish)",
		},

		// Fallback scenarios
		{
			name: "name fallback to pretty-name",
			data: map[string]interface{}{
				"pretty-name": "Alpine Linux v3.18",
				"version-id":  "3.18",
			},
			wantName:    "Alpine Linux v3.18",
			wantVersion: "3.18",
		},
		{
			name: "name fallback to id",
			data: map[string]interface{}{
				"id":         "alpine",
				"version-id": "3.18",
			},
			wantName:    "alpine",
			wantVersion: "3.18",
		},
		{
			name: "version fallback to version-id",
			data: map[string]interface{}{
				"name":       "Fedora",
				"version-id": "38",
			},
			wantName:    "Fedora",
			wantVersion: "38",
		},
		{
			name: "version fallback to pretty-name when different",
			data: map[string]interface{}{
				"name":        "Rocky Linux",
				"pretty-name": "Rocky Linux 9.2 (Blue Onyx)",
			},
			wantName:    "Rocky Linux",
			wantVersion: "Rocky Linux 9.2 (Blue Onyx)",
		},
		{
			name: "version fallback to kernel-release",
			data: map[string]interface{}{
				"name":           "Linux",
				"kernel-release": "5.15.0-generic",
			},
			wantName:    "Linux",
			wantVersion: "5.15.0-generic",
		},

		// Special case: version equals name
		{
			name: "version equals name cleared",
			data: map[string]interface{}{
				"name":    "CentOS",
				"version": "CentOS",
			},
			wantName:    "CentOS",
			wantVersion: "",
		},

		// Wrapped in "result" field (QEMU guest agent format)
		{
			name: "wrapped in result",
			data: map[string]interface{}{
				"result": map[string]interface{}{
					"name":       "Arch Linux",
					"version-id": "rolling",
				},
			},
			wantName:    "Arch Linux",
			wantVersion: "rolling",
		},
		{
			name: "result not a map",
			data: map[string]interface{}{
				"result": "not a map",
				"name":   "Windows",
			},
			wantName:    "Windows",
			wantVersion: "",
		},

		// Windows-like data
		{
			name: "windows style",
			data: map[string]interface{}{
				"name":        "Microsoft Windows",
				"pretty-name": "Windows 11 Pro",
				"version":     "22H2",
			},
			wantName:    "Microsoft Windows",
			wantVersion: "22H2",
		},

		// FreeBSD
		{
			name: "freebsd",
			data: map[string]interface{}{
				"name":       "FreeBSD",
				"version":    "13.2-RELEASE",
				"version-id": "13.2",
			},
			wantName:    "FreeBSD",
			wantVersion: "13.2-RELEASE",
		},

		// Whitespace handling
		{
			name: "whitespace trimmed",
			data: map[string]interface{}{
				"name":    "  Debian  ",
				"version": "  12  ",
			},
			wantName:    "Debian",
			wantVersion: "12",
		},

		// Non-string types converted
		{
			name: "numeric version",
			data: map[string]interface{}{
				"name":    "Custom OS",
				"version": float64(10),
			},
			wantName:    "Custom OS",
			wantVersion: "10",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotName, gotVersion := extractGuestOSInfo(tc.data)
			if gotName != tc.wantName || gotVersion != tc.wantVersion {
				t.Fatalf("extractGuestOSInfo(%v) = (%q, %q), want (%q, %q)",
					tc.data, gotName, gotVersion, tc.wantName, tc.wantVersion)
			}
		})
	}
}

func TestCloneStringFloatMap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input map[string]float64
		want  map[string]float64
	}{
		{name: "nil input", input: nil, want: nil},
		{name: "empty map", input: map[string]float64{}, want: nil},
		{name: "single entry", input: map[string]float64{"cpu": 42.5}, want: map[string]float64{"cpu": 42.5}},
		{name: "multiple entries", input: map[string]float64{"temp1": 45.0, "temp2": 50.0}, want: map[string]float64{"temp1": 45.0, "temp2": 50.0}},
		{name: "zero value", input: map[string]float64{"zero": 0.0}, want: map[string]float64{"zero": 0.0}},
		{name: "negative value", input: map[string]float64{"neg": -10.5}, want: map[string]float64{"neg": -10.5}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cloneStringFloatMap(tc.input)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("cloneStringFloatMap(%v) = %v, want nil", tc.input, got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("cloneStringFloatMap(%v) length = %d, want %d", tc.input, len(got), len(tc.want))
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("cloneStringFloatMap(%v)[%q] = %v, want %v", tc.input, k, got[k], v)
				}
			}
			// Verify it's a deep copy
			if tc.input != nil && len(tc.input) > 0 {
				for k := range tc.input {
					tc.input[k] = 999.0
					if got[k] == 999.0 {
						t.Fatalf("cloneStringFloatMap() returned reference, not a copy")
					}
					break
				}
			}
		})
	}
}

func TestCloneStringMap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input map[string]string
		want  map[string]string
	}{
		{name: "nil input", input: nil, want: nil},
		{name: "empty map", input: map[string]string{}, want: nil},
		{name: "single entry", input: map[string]string{"key": "value"}, want: map[string]string{"key": "value"}},
		{name: "multiple entries", input: map[string]string{"a": "1", "b": "2"}, want: map[string]string{"a": "1", "b": "2"}},
		{name: "empty string value", input: map[string]string{"empty": ""}, want: map[string]string{"empty": ""}},
		{name: "empty string key", input: map[string]string{"": "value"}, want: map[string]string{"": "value"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cloneStringMap(tc.input)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("cloneStringMap(%v) = %v, want nil", tc.input, got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("cloneStringMap(%v) length = %d, want %d", tc.input, len(got), len(tc.want))
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("cloneStringMap(%v)[%q] = %q, want %q", tc.input, k, got[k], v)
				}
			}
			// Verify it's a deep copy
			if tc.input != nil && len(tc.input) > 0 {
				for k := range tc.input {
					tc.input[k] = "modified"
					if got[k] == "modified" {
						t.Fatalf("cloneStringMap() returned reference, not a copy")
					}
					break
				}
			}
		})
	}
}

func TestNormalizeAgentVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		// Empty/whitespace
		{"", ""},
		{"  ", ""},
		{"   \t  ", ""},

		// Already has v prefix
		{"v1.0.0", "v1.0.0"},
		{"V1.0.0", "v1.0.0"},

		// Needs v prefix
		{"1.0.0", "v1.0.0"},
		{"4.35.0", "v4.35.0"},

		// Multiple v prefixes trimmed
		{"vv1.0.0", "v1.0.0"},
		{"VVV1.0.0", "v1.0.0"},
		{"vVv1.0.0", "v1.0.0"},

		// Only v/V (edge case)
		{"v", ""},
		{"V", ""},
		{"vV", ""},

		// With whitespace
		{"  v1.0.0  ", "v1.0.0"},
		{"  1.0.0  ", "v1.0.0"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeAgentVersion(tc.input); got != tc.want {
				t.Fatalf("normalizeAgentVersion(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizePBSNamespacePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		// Root path normalizes to empty
		{"/", ""},

		// Other paths preserved
		{"", ""},
		{"backup", "backup"},
		{"/backup", "/backup"},
		{"backup/subdir", "backup/subdir"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizePBSNamespacePath(tc.input); got != tc.want {
				t.Fatalf("normalizePBSNamespacePath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNamespacePathsForDatastore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ds   models.PBSDatastore
		want []string
	}{
		{
			name: "no namespaces returns empty string",
			ds:   models.PBSDatastore{Name: "ds1", Namespaces: nil},
			want: []string{""},
		},
		{
			name: "empty namespaces returns empty string",
			ds:   models.PBSDatastore{Name: "ds1", Namespaces: []models.PBSNamespace{}},
			want: []string{""},
		},
		{
			name: "single namespace",
			ds: models.PBSDatastore{
				Name: "ds1",
				Namespaces: []models.PBSNamespace{
					{Path: "backup"},
				},
			},
			want: []string{"backup"},
		},
		{
			name: "multiple namespaces",
			ds: models.PBSDatastore{
				Name: "ds1",
				Namespaces: []models.PBSNamespace{
					{Path: "backup"},
					{Path: "archive"},
				},
			},
			want: []string{"backup", "archive"},
		},
		{
			name: "root namespace normalized",
			ds: models.PBSDatastore{
				Name: "ds1",
				Namespaces: []models.PBSNamespace{
					{Path: "/"},
				},
			},
			want: []string{""},
		},
		{
			name: "duplicate paths deduplicated",
			ds: models.PBSDatastore{
				Name: "ds1",
				Namespaces: []models.PBSNamespace{
					{Path: "backup"},
					{Path: "backup"},
					{Path: "archive"},
				},
			},
			want: []string{"backup", "archive"},
		},
		{
			name: "duplicate root paths deduplicated",
			ds: models.PBSDatastore{
				Name: "ds1",
				Namespaces: []models.PBSNamespace{
					{Path: "/"},
					{Path: "/"},
				},
			},
			want: []string{""},
		},
		{
			name: "all empty paths result in single empty",
			ds: models.PBSDatastore{
				Name: "ds1",
				Namespaces: []models.PBSNamespace{
					{Path: ""},
					{Path: ""},
				},
			},
			want: []string{""},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := namespacePathsForDatastore(tc.ds)
			if len(got) != len(tc.want) {
				t.Fatalf("namespacePathsForDatastore() = %v, want %v", got, tc.want)
			}
			for i, v := range tc.want {
				if got[i] != v {
					t.Fatalf("namespacePathsForDatastore()[%d] = %q, want %q", i, got[i], v)
				}
			}
		})
	}
}

func TestNormalizeDockerHostID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"host1", "host1"},
		{"  host1  ", "host1"},
		{"  ", ""},
		{"\t\n", ""},
		{"docker-host-123", "docker-host-123"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeDockerHostID(tc.input); got != tc.want {
				t.Fatalf("normalizeDockerHostID(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
