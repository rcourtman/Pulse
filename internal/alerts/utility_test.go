package alerts

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestSanitizeAlertKey tests the sanitizeAlertKey function
func TestSanitizeAlertKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Basic cases
		{
			name:  "empty string returns empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only returns empty",
			input: "   ",
			want:  "",
		},
		{
			name:  "simple lowercase passes through",
			input: "disk",
			want:  "disk",
		},
		{
			name:  "uppercase converted to lowercase",
			input: "DISK",
			want:  "disk",
		},
		{
			name:  "mixed case normalized",
			input: "MyDisk",
			want:  "mydisk",
		},

		// Root handling
		{
			name:  "single slash becomes root",
			input: "/",
			want:  "root",
		},
		{
			name:  "slashes trimmed",
			input: "/disk/",
			want:  "disk",
		},
		{
			name:  "leading slashes trimmed",
			input: "/mnt/data",
			want:  "mnt-data",
		},

		// Special character handling
		{
			name:  "spaces become dashes",
			input: "my disk",
			want:  "my-disk",
		},
		{
			name:  "multiple spaces become single dash",
			input: "my   disk",
			want:  "my-disk",
		},
		{
			name:  "underscores become dashes",
			input: "my_disk",
			want:  "my-disk",
		},
		{
			name:  "backslashes handled",
			input: "C:\\Users\\Data",
			want:  "c-users-data",
		},
		{
			name:  "dots preserved",
			input: "disk.local",
			want:  "disk.local",
		},
		{
			name:  "numbers preserved",
			input: "disk123",
			want:  "disk123",
		},
		{
			name:  "alphanumeric with dots",
			input: "nvme0n1p1",
			want:  "nvme0n1p1",
		},

		// Edge cases
		{
			name:  "only slashes and backslashes becomes root",
			input: "//\\\\",
			want:  "root",
		},
		{
			name:  "only special chars becomes disk",
			input: "@#$%",
			want:  "disk",
		},
		{
			name:  "trailing dashes trimmed",
			input: "disk--",
			want:  "disk",
		},
		{
			name:  "trailing dots trimmed",
			input: "disk..",
			want:  "disk",
		},
		{
			name:  "leading and trailing trimmed",
			input: "--disk--",
			want:  "disk",
		},

		// Real-world examples
		{
			name:  "linux mount path",
			input: "/mnt/storage/backup",
			want:  "mnt-storage-backup",
		},
		{
			name:  "linux device path",
			input: "/dev/sda1",
			want:  "dev-sda1",
		},
		{
			name:  "nvme device",
			input: "/dev/nvme0n1",
			want:  "dev-nvme0n1",
		},
		{
			name:  "windows drive letter",
			input: "C:",
			want:  "c",
		},
		{
			name:  "docker volume name",
			input: "my_app_data",
			want:  "my-app-data",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeAlertKey(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeAlertKey(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestAbs tests the abs function for float64
func TestAbs(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{
			name:  "positive returns unchanged",
			input: 5.5,
			want:  5.5,
		},
		{
			name:  "negative becomes positive",
			input: -5.5,
			want:  5.5,
		},
		{
			name:  "zero returns zero",
			input: 0,
			want:  0,
		},
		{
			name:  "negative zero returns zero",
			input: -0.0,
			want:  0,
		},
		{
			name:  "small positive",
			input: 0.001,
			want:  0.001,
		},
		{
			name:  "small negative",
			input: -0.001,
			want:  0.001,
		},
		{
			name:  "large positive",
			input: 1e10,
			want:  1e10,
		},
		{
			name:  "large negative",
			input: -1e10,
			want:  1e10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := abs(tc.input)
			if got != tc.want {
				t.Errorf("abs(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestIsQueueOutlier tests the isQueueOutlier function
func TestIsQueueOutlier(t *testing.T) {
	tests := []struct {
		name   string
		value  int
		median int
		want   bool
	}{
		// Zero median cases
		{
			name:   "zero median with zero value is not outlier",
			value:  0,
			median: 0,
			want:   false,
		},
		{
			name:   "zero median with positive value is outlier",
			value:  1,
			median: 0,
			want:   true,
		},
		{
			name:   "zero median with large value is outlier",
			value:  100,
			median: 0,
			want:   true,
		},

		// Normal cases (threshold is 40% above median)
		{
			name:   "value equal to median is not outlier",
			value:  100,
			median: 100,
			want:   false,
		},
		{
			name:   "value 20% above median is not outlier",
			value:  120,
			median: 100,
			want:   false,
		},
		{
			name:   "value 40% above median is not outlier (boundary)",
			value:  140,
			median: 100,
			want:   false,
		},
		{
			name:   "value 41% above median is outlier",
			value:  141,
			median: 100,
			want:   true,
		},
		{
			name:   "value 50% above median is outlier",
			value:  150,
			median: 100,
			want:   true,
		},
		{
			name:   "value 100% above median is outlier",
			value:  200,
			median: 100,
			want:   true,
		},

		// Value below median
		{
			name:   "value below median is not outlier",
			value:  50,
			median: 100,
			want:   false,
		},
		{
			name:   "value at zero with nonzero median is not outlier",
			value:  0,
			median: 100,
			want:   false,
		},

		// Small numbers
		{
			name:   "small median value at boundary",
			value:  7,
			median: 5,
			want:   false, // 7/5 = 1.4 = 40%, at boundary
		},
		{
			name:   "small median value just over",
			value:  8,
			median: 5,
			want:   true, // 8/5 = 1.6 = 60% above
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isQueueOutlier(tc.value, tc.median)
			if got != tc.want {
				t.Errorf("isQueueOutlier(%d, %d) = %v, want %v", tc.value, tc.median, got, tc.want)
			}
		})
	}
}

// TestScaleThreshold tests the scaleThreshold function
func TestScaleThreshold(t *testing.T) {
	tests := []struct {
		name        string
		threshold   int
		scaleFactor float64
		want        int
	}{
		// Zero threshold
		{
			name:        "zero threshold returns zero",
			threshold:   0,
			scaleFactor: 2.0,
			want:        0,
		},
		{
			name:        "negative threshold returns zero",
			threshold:   -5,
			scaleFactor: 2.0,
			want:        0,
		},

		// Normal scaling
		{
			name:        "scale by 1.0 returns same",
			threshold:   100,
			scaleFactor: 1.0,
			want:        100,
		},
		{
			name:        "scale by 2.0 doubles",
			threshold:   100,
			scaleFactor: 2.0,
			want:        200,
		},
		{
			name:        "scale by 0.5 halves",
			threshold:   100,
			scaleFactor: 0.5,
			want:        50,
		},
		{
			name:        "scale by 1.5",
			threshold:   100,
			scaleFactor: 1.5,
			want:        150,
		},

		// Ceiling behavior
		{
			name:        "ceiling applied for fractional result",
			threshold:   10,
			scaleFactor: 1.1,
			want:        11, // 10 * 1.1 = 11.0 (exact)
		},
		{
			name:        "ceiling rounds up",
			threshold:   10,
			scaleFactor: 0.33,
			want:        4, // 10 * 0.33 = 3.3 -> ceil = 4
		},
		{
			name:        "small threshold with small factor",
			threshold:   3,
			scaleFactor: 0.5,
			want:        2, // 3 * 0.5 = 1.5 -> ceil = 2
		},

		// Minimum value of 1
		{
			name:        "very small result floors to 1",
			threshold:   1,
			scaleFactor: 0.1,
			want:        1, // 1 * 0.1 = 0.1 -> ceil = 1, min = 1
		},
		{
			name:        "result near zero floors to 1",
			threshold:   5,
			scaleFactor: 0.001,
			want:        1, // 5 * 0.001 = 0.005 -> ceil = 1
		},

		// Edge cases
		{
			name:        "large threshold",
			threshold:   10000,
			scaleFactor: 10.0,
			want:        100000,
		},
		{
			name:        "zero scale factor gives 1",
			threshold:   100,
			scaleFactor: 0.0,
			want:        1, // 100 * 0 = 0 -> ceil(0) = 0, min = 1
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scaleThreshold(tc.threshold, tc.scaleFactor)
			if got != tc.want {
				t.Errorf("scaleThreshold(%d, %v) = %d, want %d", tc.threshold, tc.scaleFactor, got, tc.want)
			}
		})
	}
}

// TestCalculateMedianInt tests the calculateMedianInt function
func TestCalculateMedianInt(t *testing.T) {
	tests := []struct {
		name   string
		values []int
		want   int
	}{
		// Empty and single element
		{
			name:   "empty slice returns 0",
			values: []int{},
			want:   0,
		},
		{
			name:   "nil slice returns 0",
			values: nil,
			want:   0,
		},
		{
			name:   "single element returns that element",
			values: []int{5},
			want:   5,
		},
		{
			name:   "single zero returns 0",
			values: []int{0},
			want:   0,
		},

		// Odd number of elements
		{
			name:   "three elements returns middle",
			values: []int{1, 2, 3},
			want:   2,
		},
		{
			name:   "three unsorted elements",
			values: []int{3, 1, 2},
			want:   2,
		},
		{
			name:   "five elements returns middle",
			values: []int{1, 2, 3, 4, 5},
			want:   3,
		},
		{
			name:   "five unsorted elements",
			values: []int{5, 1, 3, 2, 4},
			want:   3,
		},

		// Even number of elements
		{
			name:   "two elements returns average",
			values: []int{2, 4},
			want:   3, // (2 + 4) / 2 = 3
		},
		{
			name:   "four elements returns average of middle two",
			values: []int{1, 2, 3, 4},
			want:   2, // (2 + 3) / 2 = 2 (integer division)
		},
		{
			name:   "four unsorted elements",
			values: []int{4, 1, 3, 2},
			want:   2, // sorted: 1,2,3,4 -> (2+3)/2 = 2
		},
		{
			name:   "six elements",
			values: []int{1, 2, 3, 4, 5, 6},
			want:   3, // (3 + 4) / 2 = 3 (integer division)
		},

		// Duplicates
		{
			name:   "all same values",
			values: []int{5, 5, 5, 5, 5},
			want:   5,
		},
		{
			name:   "some duplicates odd count",
			values: []int{1, 2, 2, 3, 3},
			want:   2,
		},
		{
			name:   "some duplicates even count",
			values: []int{1, 1, 3, 3},
			want:   2, // (1 + 3) / 2 = 2
		},

		// Negative numbers
		{
			name:   "negative numbers",
			values: []int{-5, -3, -1},
			want:   -3,
		},
		{
			name:   "mixed positive and negative",
			values: []int{-5, 0, 5},
			want:   0,
		},
		{
			name:   "mixed even count",
			values: []int{-4, -2, 2, 4},
			want:   0, // (-2 + 2) / 2 = 0
		},

		// Large values
		{
			name:   "large values",
			values: []int{1000000, 2000000, 3000000},
			want:   2000000,
		},

		// Edge case for integer division
		{
			name:   "average truncates down",
			values: []int{1, 4},
			want:   2, // (1 + 4) / 2 = 2 (not 2.5)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy to ensure original isn't modified
			var input []int
			if tc.values != nil {
				input = make([]int, len(tc.values))
				copy(input, tc.values)
			}

			got := calculateMedianInt(input)
			if got != tc.want {
				t.Errorf("calculateMedianInt(%v) = %d, want %d", tc.values, got, tc.want)
			}

			// Verify original slice wasn't modified (if non-nil)
			if tc.values != nil && input != nil {
				for i := range input {
					if input[i] != tc.values[i] {
						t.Errorf("calculateMedianInt modified input slice")
						break
					}
				}
			}
		})
	}
}

// TestCalculateMedianInt_DoesNotModifyInput verifies the input slice is copied
func TestCalculateMedianInt_DoesNotModifyInput(t *testing.T) {
	input := []int{5, 1, 3, 2, 4}
	original := make([]int, len(input))
	copy(original, input)

	_ = calculateMedianInt(input)

	for i := range input {
		if input[i] != original[i] {
			t.Errorf("calculateMedianInt modified input: got %v, original was %v", input, original)
			return
		}
	}
}

// TestHostResourceID tests the hostResourceID function
func TestHostResourceID(t *testing.T) {
	tests := []struct {
		name   string
		hostID string
		want   string
	}{
		{
			name:   "normal host ID",
			hostID: "host-123",
			want:   "host:host-123",
		},
		{
			name:   "empty string returns unknown",
			hostID: "",
			want:   "host:unknown",
		},
		{
			name:   "whitespace only returns unknown",
			hostID: "   ",
			want:   "host:unknown",
		},
		{
			name:   "whitespace is trimmed",
			hostID: "  host-456  ",
			want:   "host:host-456",
		},
		{
			name:   "UUID format",
			hostID: "550e8400-e29b-41d4-a716-446655440000",
			want:   "host:550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:   "simple hostname",
			hostID: "server1",
			want:   "host:server1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hostResourceID(tc.hostID)
			if got != tc.want {
				t.Errorf("hostResourceID(%q) = %q, want %q", tc.hostID, got, tc.want)
			}
		})
	}
}

// TestHostDisplayName tests the hostDisplayName function
func TestHostDisplayName(t *testing.T) {
	tests := []struct {
		name string
		host models.Host
		want string
	}{
		{
			name: "display name preferred",
			host: models.Host{
				ID:          "id-123",
				DisplayName: "My Server",
				Hostname:    "server.local",
			},
			want: "My Server",
		},
		{
			name: "hostname when no display name",
			host: models.Host{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "server.local",
			},
			want: "server.local",
		},
		{
			name: "ID when no display name or hostname",
			host: models.Host{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "",
			},
			want: "id-123",
		},
		{
			name: "fallback to Host literal",
			host: models.Host{
				ID:          "",
				DisplayName: "",
				Hostname:    "",
			},
			want: "Host",
		},
		{
			name: "whitespace display name ignored",
			host: models.Host{
				ID:          "id-123",
				DisplayName: "   ",
				Hostname:    "server.local",
			},
			want: "server.local",
		},
		{
			name: "whitespace hostname ignored",
			host: models.Host{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "   ",
			},
			want: "id-123",
		},
		{
			name: "display name with whitespace trimmed",
			host: models.Host{
				ID:          "id-123",
				DisplayName: "  Server Name  ",
				Hostname:    "server.local",
			},
			want: "Server Name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hostDisplayName(tc.host)
			if got != tc.want {
				t.Errorf("hostDisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestHostInstanceName tests the hostInstanceName function
func TestHostInstanceName(t *testing.T) {
	tests := []struct {
		name string
		host models.Host
		want string
	}{
		{
			name: "platform preferred",
			host: models.Host{
				Platform: "linux",
				OSName:   "Ubuntu 22.04",
			},
			want: "linux",
		},
		{
			name: "os name when no platform",
			host: models.Host{
				Platform: "",
				OSName:   "Ubuntu 22.04",
			},
			want: "Ubuntu 22.04",
		},
		{
			name: "fallback to Host Agent",
			host: models.Host{
				Platform: "",
				OSName:   "",
			},
			want: "Host Agent",
		},
		{
			name: "whitespace platform ignored",
			host: models.Host{
				Platform: "   ",
				OSName:   "Windows Server",
			},
			want: "Windows Server",
		},
		{
			name: "whitespace os name ignored",
			host: models.Host{
				Platform: "",
				OSName:   "   ",
			},
			want: "Host Agent",
		},
		{
			name: "platform with whitespace trimmed",
			host: models.Host{
				Platform: "  darwin  ",
				OSName:   "macOS",
			},
			want: "darwin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hostInstanceName(tc.host)
			if got != tc.want {
				t.Errorf("hostInstanceName() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSanitizeHostComponent tests the sanitizeHostComponent function
func TestSanitizeHostComponent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Empty/whitespace
		{
			name:  "empty returns unknown",
			input: "",
			want:  "unknown",
		},
		{
			name:  "whitespace only returns unknown",
			input: "   ",
			want:  "unknown",
		},

		// Basic strings
		{
			name:  "lowercase passes through",
			input: "myhost",
			want:  "myhost",
		},
		{
			name:  "uppercase converted to lowercase",
			input: "MYHOST",
			want:  "myhost",
		},
		{
			name:  "mixed case normalized",
			input: "MyHost",
			want:  "myhost",
		},
		{
			name:  "numbers preserved",
			input: "host123",
			want:  "host123",
		},

		// Special characters become hyphens
		{
			name:  "spaces become hyphen",
			input: "my host",
			want:  "my-host",
		},
		{
			name:  "multiple spaces become single hyphen",
			input: "my   host",
			want:  "my-host",
		},
		{
			name:  "underscores become hyphen",
			input: "my_host",
			want:  "my-host",
		},
		{
			name:  "slashes become hyphen",
			input: "mnt/data",
			want:  "mnt-data",
		},
		{
			name:  "dots become hyphen",
			input: "host.local",
			want:  "host-local",
		},
		{
			name:  "mixed special chars",
			input: "host.local/data_01",
			want:  "host-local-data-01",
		},

		// Trimming leading/trailing hyphens
		{
			name:  "leading special chars trimmed",
			input: "--host",
			want:  "host",
		},
		{
			name:  "trailing special chars trimmed",
			input: "host--",
			want:  "host",
		},
		{
			name:  "both ends trimmed",
			input: "/host/",
			want:  "host",
		},

		// Only special chars
		{
			name:  "only special chars returns unknown",
			input: "@#$%",
			want:  "unknown",
		},
		{
			name:  "only hyphens returns unknown",
			input: "---",
			want:  "unknown",
		},

		// Real-world examples
		{
			name:  "linux mount path",
			input: "/mnt/storage",
			want:  "mnt-storage",
		},
		{
			name:  "device path",
			input: "/dev/sda1",
			want:  "dev-sda1",
		},
		{
			name:  "nvme device",
			input: "nvme0n1p1",
			want:  "nvme0n1p1",
		},
		{
			name:  "IP address",
			input: "192.168.1.1",
			want:  "192-168-1-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeHostComponent(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeHostComponent(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestSanitizeRAIDDevice tests the sanitizeRAIDDevice function
func TestSanitizeRAIDDevice(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "with dev prefix",
			input: "/dev/md0",
			want:  "md0",
		},
		{
			name:  "without dev prefix",
			input: "md0",
			want:  "md0",
		},
		{
			name:  "nvme device",
			input: "/dev/nvme0n1",
			want:  "nvme0n1",
		},
		{
			name:  "sda device",
			input: "/dev/sda",
			want:  "sda",
		},
		{
			name:  "empty returns unknown",
			input: "",
			want:  "unknown",
		},
		{
			name:  "only dev prefix",
			input: "/dev/",
			want:  "unknown",
		},
		{
			name:  "md device with partition",
			input: "/dev/md127p1",
			want:  "md127p1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeRAIDDevice(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeRAIDDevice(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestHostDiskResourceID tests the hostDiskResourceID function
func TestHostDiskResourceID(t *testing.T) {
	tests := []struct {
		name         string
		host         models.Host
		disk         models.Disk
		wantID       string
		wantNamePart string // Part of the name to check
	}{
		{
			name: "mountpoint preferred",
			host: models.Host{ID: "host-123"},
			disk: models.Disk{
				Mountpoint: "/mnt/data",
				Device:     "/dev/sda1",
			},
			wantID:       "host:host-123/disk:mnt-data",
			wantNamePart: "/mnt/data",
		},
		{
			name: "device when no mountpoint",
			host: models.Host{ID: "host-123"},
			disk: models.Disk{
				Mountpoint: "",
				Device:     "/dev/sda1",
			},
			wantID:       "host:host-123/disk:dev-sda1",
			wantNamePart: "/dev/sda1",
		},
		{
			name: "fallback to disk literal",
			host: models.Host{ID: "host-123"},
			disk: models.Disk{
				Mountpoint: "",
				Device:     "",
			},
			wantID:       "host:host-123/disk:disk",
			wantNamePart: "disk",
		},
		{
			name: "root mount sanitizes to unknown",
			host: models.Host{ID: "host-123"},
			disk: models.Disk{
				Mountpoint: "/",
				Device:     "/dev/sda1",
			},
			wantID:       "host:host-123/disk:unknown",
			wantNamePart: "/",
		},
		{
			name: "whitespace mountpoint uses device",
			host: models.Host{ID: "host-123"},
			disk: models.Disk{
				Mountpoint: "   ",
				Device:     "/dev/sda1",
			},
			wantID:       "host:host-123/disk:dev-sda1",
			wantNamePart: "/dev/sda1",
		},
		{
			name: "includes host display name",
			host: models.Host{
				ID:          "host-123",
				DisplayName: "My Server",
			},
			disk: models.Disk{
				Mountpoint: "/data",
			},
			wantID:       "host:host-123/disk:data",
			wantNamePart: "My Server",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotName := hostDiskResourceID(tc.host, tc.disk)
			if gotID != tc.wantID {
				t.Errorf("hostDiskResourceID() ID = %q, want %q", gotID, tc.wantID)
			}
			if !strings.Contains(gotName, tc.wantNamePart) {
				t.Errorf("hostDiskResourceID() Name = %q, want to contain %q", gotName, tc.wantNamePart)
			}
		})
	}
}

// TestIsMonitorOnlyAlert tests the isMonitorOnlyAlert function
func TestIsMonitorOnlyAlert(t *testing.T) {
	tests := []struct {
		name  string
		alert *Alert
		want  bool
	}{
		{
			name:  "nil alert returns false",
			alert: nil,
			want:  false,
		},
		{
			name: "nil metadata returns false",
			alert: &Alert{
				ID:       "test-1",
				Metadata: nil,
			},
			want: false,
		},
		{
			name: "empty metadata returns false",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{},
			},
			want: false,
		},
		{
			name: "no monitorOnly key returns false",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"other": "value"},
			},
			want: false,
		},
		{
			name: "monitorOnly bool true returns true",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": true},
			},
			want: true,
		},
		{
			name: "monitorOnly bool false returns false",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": false},
			},
			want: false,
		},
		{
			name: "monitorOnly string 'true' returns true",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": "true"},
			},
			want: true,
		},
		{
			name: "monitorOnly string 'TRUE' returns true (case insensitive)",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": "TRUE"},
			},
			want: true,
		},
		{
			name: "monitorOnly string 'True' returns true (case insensitive)",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": "True"},
			},
			want: true,
		},
		{
			name: "monitorOnly string 'false' returns false",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": "false"},
			},
			want: false,
		},
		{
			name: "monitorOnly string 'yes' returns false (not 'true')",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": "yes"},
			},
			want: false,
		},
		{
			name: "monitorOnly int value returns false (not bool or string)",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": 1},
			},
			want: false,
		},
		{
			name: "monitorOnly nil value returns false",
			alert: &Alert{
				ID:       "test-1",
				Metadata: map[string]interface{}{"monitorOnly": nil},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isMonitorOnlyAlert(tc.alert)
			if got != tc.want {
				t.Errorf("isMonitorOnlyAlert() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestQuietHoursCategoryForAlert tests the quietHoursCategoryForAlert function
func TestQuietHoursCategoryForAlert(t *testing.T) {
	tests := []struct {
		name  string
		alert *Alert
		want  string
	}{
		// Nil alert
		{
			name:  "nil alert returns empty",
			alert: nil,
			want:  "",
		},

		// Performance metrics
		{
			name:  "cpu type returns performance",
			alert: &Alert{Type: "cpu"},
			want:  "performance",
		},
		{
			name:  "memory type returns performance",
			alert: &Alert{Type: "memory"},
			want:  "performance",
		},
		{
			name:  "disk type returns performance",
			alert: &Alert{Type: "disk"},
			want:  "performance",
		},
		{
			name:  "diskRead type returns performance",
			alert: &Alert{Type: "diskRead"},
			want:  "performance",
		},
		{
			name:  "diskWrite type returns performance",
			alert: &Alert{Type: "diskWrite"},
			want:  "performance",
		},
		{
			name:  "networkIn type returns performance",
			alert: &Alert{Type: "networkIn"},
			want:  "performance",
		},
		{
			name:  "networkOut type returns performance",
			alert: &Alert{Type: "networkOut"},
			want:  "performance",
		},
		{
			name:  "temperature type returns performance",
			alert: &Alert{Type: "temperature"},
			want:  "performance",
		},
		{
			name:  "queue-depth returns performance",
			alert: &Alert{Type: "queue-depth"},
			want:  "performance",
		},
		{
			name:  "queue-deferred returns performance",
			alert: &Alert{Type: "queue-deferred"},
			want:  "performance",
		},
		{
			name:  "queue-hold returns performance",
			alert: &Alert{Type: "queue-hold"},
			want:  "performance",
		},
		{
			name:  "message-age returns performance",
			alert: &Alert{Type: "message-age"},
			want:  "performance",
		},
		{
			name:  "docker-container-health returns performance",
			alert: &Alert{Type: "docker-container-health"},
			want:  "performance",
		},
		{
			name:  "docker-container-restart-loop returns performance",
			alert: &Alert{Type: "docker-container-restart-loop"},
			want:  "performance",
		},
		{
			name:  "docker-container-oom-kill returns performance",
			alert: &Alert{Type: "docker-container-oom-kill"},
			want:  "performance",
		},
		{
			name:  "docker-container-memory-limit returns performance",
			alert: &Alert{Type: "docker-container-memory-limit"},
			want:  "performance",
		},

		// Storage metrics
		{
			name:  "usage type returns storage",
			alert: &Alert{Type: "usage"},
			want:  "storage",
		},
		{
			name:  "disk-health returns storage",
			alert: &Alert{Type: "disk-health"},
			want:  "storage",
		},
		{
			name:  "disk-wearout returns storage",
			alert: &Alert{Type: "disk-wearout"},
			want:  "storage",
		},
		{
			name:  "zfs-pool-state returns storage",
			alert: &Alert{Type: "zfs-pool-state"},
			want:  "storage",
		},
		{
			name:  "zfs-pool-errors returns storage",
			alert: &Alert{Type: "zfs-pool-errors"},
			want:  "storage",
		},
		{
			name:  "zfs-device returns storage",
			alert: &Alert{Type: "zfs-device"},
			want:  "storage",
		},

		// Offline metrics
		{
			name:  "connectivity type returns offline",
			alert: &Alert{Type: "connectivity"},
			want:  "offline",
		},
		{
			name:  "offline type returns offline",
			alert: &Alert{Type: "offline"},
			want:  "offline",
		},
		{
			name:  "powered-off type returns offline",
			alert: &Alert{Type: "powered-off"},
			want:  "offline",
		},
		{
			name:  "docker-host-offline returns offline",
			alert: &Alert{Type: "docker-host-offline"},
			want:  "offline",
		},

		// Docker container prefix handling
		{
			name:  "docker-container-state returns offline",
			alert: &Alert{Type: "docker-container-state"},
			want:  "offline",
		},
		{
			name:  "docker-container-cpu returns performance (prefix match)",
			alert: &Alert{Type: "docker-container-cpu"},
			want:  "performance",
		},
		{
			name:  "docker-container-disk returns performance (prefix match)",
			alert: &Alert{Type: "docker-container-disk"},
			want:  "performance",
		},

		// Unknown types
		{
			name:  "unknown type returns empty",
			alert: &Alert{Type: "unknown-type"},
			want:  "",
		},
		{
			name:  "empty type returns empty",
			alert: &Alert{Type: ""},
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quietHoursCategoryForAlert(tc.alert)
			if got != tc.want {
				t.Errorf("quietHoursCategoryForAlert(%v) = %q, want %q", tc.alert, got, tc.want)
			}
		})
	}
}

// TestCanonicalResourceTypeKeys tests the canonicalResourceTypeKeys function
func TestCanonicalResourceTypeKeys(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         []string
	}{
		// Guest types
		{
			name:         "guest returns guest",
			resourceType: "guest",
			want:         []string{"guest"},
		},
		{
			name:         "qemu returns guest",
			resourceType: "qemu",
			want:         []string{"guest"},
		},
		{
			name:         "vm returns guest",
			resourceType: "vm",
			want:         []string{"guest"},
		},
		{
			name:         "ct returns guest",
			resourceType: "ct",
			want:         []string{"guest"},
		},
		{
			name:         "container returns guest",
			resourceType: "container",
			want:         []string{"guest"},
		},
		{
			name:         "lxc returns guest",
			resourceType: "lxc",
			want:         []string{"guest"},
		},

		// Docker container types
		{
			name:         "docker returns docker and guest",
			resourceType: "docker",
			want:         []string{"docker", "guest"},
		},
		{
			name:         "docker container with space returns docker and guest",
			resourceType: "docker container",
			want:         []string{"docker", "guest"},
		},
		{
			name:         "dockercontainer returns docker and guest",
			resourceType: "dockercontainer",
			want:         []string{"docker", "guest"},
		},

		// Docker host types
		{
			name:         "docker host with space returns dockerhost, docker, node",
			resourceType: "docker host",
			want:         []string{"dockerhost", "docker", "node"},
		},
		{
			name:         "dockerhost returns dockerhost, docker, node",
			resourceType: "dockerhost",
			want:         []string{"dockerhost", "docker", "node"},
		},

		// Node type
		{
			name:         "node returns node",
			resourceType: "node",
			want:         []string{"node"},
		},

		// PBS types
		{
			name:         "pbs returns pbs and node",
			resourceType: "pbs",
			want:         []string{"pbs", "node"},
		},
		{
			name:         "pbs server with space returns pbs and node",
			resourceType: "pbs server",
			want:         []string{"pbs", "node"},
		},
		{
			name:         "pbsserver returns pbs and node",
			resourceType: "pbsserver",
			want:         []string{"pbs", "node"},
		},

		// Storage type
		{
			name:         "storage returns storage",
			resourceType: "storage",
			want:         []string{"storage"},
		},

		// Case insensitivity
		{
			name:         "GUEST uppercase returns guest",
			resourceType: "GUEST",
			want:         []string{"guest"},
		},
		{
			name:         "Docker mixed case returns docker and guest",
			resourceType: "Docker",
			want:         []string{"docker", "guest"},
		},
		{
			name:         "NODE uppercase returns node",
			resourceType: "NODE",
			want:         []string{"node"},
		},

		// Whitespace handling
		{
			name:         "guest with leading whitespace",
			resourceType: "  guest",
			want:         []string{"guest"},
		},
		{
			name:         "guest with trailing whitespace",
			resourceType: "guest  ",
			want:         []string{"guest"},
		},
		{
			name:         "guest with surrounding whitespace",
			resourceType: "  guest  ",
			want:         []string{"guest"},
		},

		// Unknown types return self
		{
			name:         "unknown type returns itself",
			resourceType: "custom",
			want:         []string{"custom"},
		},
		{
			name:         "pmg returns itself as unknown",
			resourceType: "pmg",
			want:         []string{"pmg"},
		},

		// Empty and whitespace-only
		{
			name:         "empty string returns empty slice",
			resourceType: "",
			want:         []string{},
		},
		{
			name:         "whitespace only returns empty slice",
			resourceType: "   ",
			want:         []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalResourceTypeKeys(tc.resourceType)

			// Check length
			if len(got) != len(tc.want) {
				t.Errorf("canonicalResourceTypeKeys(%q) = %v (len %d), want %v (len %d)",
					tc.resourceType, got, len(got), tc.want, len(tc.want))
				return
			}

			// Check each element
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("canonicalResourceTypeKeys(%q)[%d] = %q, want %q",
						tc.resourceType, i, v, tc.want[i])
				}
			}
		})
	}
}

// TestEnsureValidHysteresis tests the ensureValidHysteresis function
func TestEnsureValidHysteresis(t *testing.T) {
	tests := []struct {
		name         string
		threshold    *HysteresisThreshold
		metricName   string
		wantTrigger  float64
		wantClear    float64
		expectChange bool
	}{
		{
			name:         "nil threshold does nothing",
			threshold:    nil,
			metricName:   "cpu",
			expectChange: false,
		},
		{
			name:         "valid threshold unchanged",
			threshold:    &HysteresisThreshold{Trigger: 90, Clear: 85},
			metricName:   "cpu",
			wantTrigger:  90,
			wantClear:    85,
			expectChange: false,
		},
		{
			name:         "clear < trigger is valid",
			threshold:    &HysteresisThreshold{Trigger: 80, Clear: 70},
			metricName:   "memory",
			wantTrigger:  80,
			wantClear:    70,
			expectChange: false,
		},
		{
			name:         "clear == trigger is auto-fixed",
			threshold:    &HysteresisThreshold{Trigger: 90, Clear: 90},
			metricName:   "disk",
			wantTrigger:  90,
			wantClear:    85, // 90 - 5 = 85
			expectChange: true,
		},
		{
			name:         "clear > trigger is auto-fixed",
			threshold:    &HysteresisThreshold{Trigger: 80, Clear: 90},
			metricName:   "network",
			wantTrigger:  80,
			wantClear:    75, // 80 - 5 = 75
			expectChange: true,
		},
		{
			name:         "auto-fix clamps clear to 0 for low trigger",
			threshold:    &HysteresisThreshold{Trigger: 3, Clear: 5},
			metricName:   "low",
			wantTrigger:  3,
			wantClear:    0, // 3 - 5 = -2, clamped to 0
			expectChange: true,
		},
		{
			name:         "trigger at 5 with invalid clear",
			threshold:    &HysteresisThreshold{Trigger: 5, Clear: 10},
			metricName:   "edge",
			wantTrigger:  5,
			wantClear:    0, // 5 - 5 = 0
			expectChange: true,
		},
		{
			name:         "zero trigger with positive clear is fixed",
			threshold:    &HysteresisThreshold{Trigger: 0, Clear: 5},
			metricName:   "zero",
			wantTrigger:  0,
			wantClear:    0, // 0 - 5 = -5, clamped to 0
			expectChange: true,
		},
		{
			name:         "both zero triggers auto-fix (clear >= trigger)",
			threshold:    &HysteresisThreshold{Trigger: 0, Clear: 0},
			metricName:   "disabled",
			wantTrigger:  0,
			wantClear:    0, // 0 - 5 = -5, clamped to 0 (same value, but fix attempted)
			expectChange: false, // Result same as input, even though fix was attempted
		},
		{
			name:         "large trigger with equal clear",
			threshold:    &HysteresisThreshold{Trigger: 100, Clear: 100},
			metricName:   "max",
			wantTrigger:  100,
			wantClear:    95, // 100 - 5 = 95
			expectChange: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.threshold == nil {
				// Just ensure it doesn't panic
				ensureValidHysteresis(nil, tc.metricName)
				return
			}

			// Make a copy to check if it changed
			originalTrigger := tc.threshold.Trigger
			originalClear := tc.threshold.Clear

			ensureValidHysteresis(tc.threshold, tc.metricName)

			if tc.threshold.Trigger != tc.wantTrigger {
				t.Errorf("Trigger = %v, want %v", tc.threshold.Trigger, tc.wantTrigger)
			}
			if tc.threshold.Clear != tc.wantClear {
				t.Errorf("Clear = %v, want %v", tc.threshold.Clear, tc.wantClear)
			}

			// Verify expectChange matches reality
			changed := tc.threshold.Trigger != originalTrigger || tc.threshold.Clear != originalClear
			if changed != tc.expectChange {
				t.Errorf("expectChange = %v, but changed = %v", tc.expectChange, changed)
			}
		})
	}
}

// TestCloneThreshold tests the cloneThreshold function
func TestCloneThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		threshold *HysteresisThreshold
	}{
		{
			name:      "nil threshold returns nil",
			threshold: nil,
		},
		{
			name:      "basic threshold is cloned",
			threshold: &HysteresisThreshold{Trigger: 80, Clear: 70},
		},
		{
			name:      "zero values threshold",
			threshold: &HysteresisThreshold{Trigger: 0, Clear: 0},
		},
		{
			name:      "large values threshold",
			threshold: &HysteresisThreshold{Trigger: 100, Clear: 95},
		},
		{
			name:      "fractional values threshold",
			threshold: &HysteresisThreshold{Trigger: 85.5, Clear: 80.25},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := cloneThreshold(tc.threshold)

			if tc.threshold == nil {
				if result != nil {
					t.Errorf("cloneThreshold(nil) = %v, want nil", result)
				}
				return
			}

			// Result should not be nil
			if result == nil {
				t.Fatalf("cloneThreshold() returned nil for non-nil input")
			}

			// Result should be a different pointer
			if result == tc.threshold {
				t.Error("cloneThreshold() returned same pointer, not a clone")
			}

			// Values should match
			if result.Trigger != tc.threshold.Trigger {
				t.Errorf("cloneThreshold().Trigger = %v, want %v", result.Trigger, tc.threshold.Trigger)
			}
			if result.Clear != tc.threshold.Clear {
				t.Errorf("cloneThreshold().Clear = %v, want %v", result.Clear, tc.threshold.Clear)
			}

			// Modifying clone should not affect original
			result.Trigger = 999
			result.Clear = 888
			if tc.threshold.Trigger == 999 || tc.threshold.Clear == 888 {
				t.Error("modifying clone affected original threshold")
			}
		})
	}
}

// TestCloneStringPtr tests the cloneStringPtr function
func TestCloneStringPtr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value *string
	}{
		{
			name:  "nil returns nil",
			value: nil,
		},
		{
			name:  "empty string is cloned",
			value: strPtr(""),
		},
		{
			name:  "basic string is cloned",
			value: strPtr("hello"),
		},
		{
			name:  "string with spaces",
			value: strPtr("hello world"),
		},
		{
			name:  "unicode string",
			value: strPtr("こんにちは"),
		},
		{
			name:  "long string",
			value: strPtr(strings.Repeat("a", 1000)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := cloneStringPtr(tc.value)

			if tc.value == nil {
				if result != nil {
					t.Errorf("cloneStringPtr(nil) = %v, want nil", result)
				}
				return
			}

			// Result should not be nil
			if result == nil {
				t.Fatalf("cloneStringPtr() returned nil for non-nil input")
			}

			// Result should be a different pointer
			if result == tc.value {
				t.Error("cloneStringPtr() returned same pointer, not a clone")
			}

			// Values should match
			if *result != *tc.value {
				t.Errorf("cloneStringPtr() value = %q, want %q", *result, *tc.value)
			}

			// Modifying clone should not affect original
			originalValue := *tc.value
			*result = "modified"
			if *tc.value != originalValue {
				t.Error("modifying clone affected original string")
			}
		})
	}
}

// strPtr is a helper to create a string pointer
func strPtr(s string) *string {
	return &s
}

// TestCloneThresholdConfig tests the cloneThresholdConfig function
func TestCloneThresholdConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config ThresholdConfig
	}{
		{
			name:   "empty config",
			config: ThresholdConfig{},
		},
		{
			name: "config with CPU threshold",
			config: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 80, Clear: 70},
			},
		},
		{
			name: "config with all thresholds",
			config: ThresholdConfig{
				CPU:         &HysteresisThreshold{Trigger: 80, Clear: 70},
				Memory:      &HysteresisThreshold{Trigger: 85, Clear: 75},
				Disk:        &HysteresisThreshold{Trigger: 90, Clear: 85},
				DiskRead:    &HysteresisThreshold{Trigger: 50, Clear: 40},
				DiskWrite:   &HysteresisThreshold{Trigger: 50, Clear: 40},
				NetworkIn:   &HysteresisThreshold{Trigger: 80, Clear: 70},
				NetworkOut:  &HysteresisThreshold{Trigger: 80, Clear: 70},
				Temperature: &HysteresisThreshold{Trigger: 70, Clear: 60},
				Usage:       &HysteresisThreshold{Trigger: 85, Clear: 75},
			},
		},
		{
			name: "config with note",
			config: ThresholdConfig{
				CPU:  &HysteresisThreshold{Trigger: 80, Clear: 70},
				Note: strPtr("Test note for this config"),
			},
		},
		{
			name: "config with disabled flag",
			config: ThresholdConfig{
				Disabled: true,
				CPU:      &HysteresisThreshold{Trigger: 80, Clear: 70},
			},
		},
		{
			name: "config with disable connectivity flag",
			config: ThresholdConfig{
				DisableConnectivity: true,
				Memory:              &HysteresisThreshold{Trigger: 90, Clear: 80},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := cloneThresholdConfig(tc.config)

			// Check disabled flags
			if result.Disabled != tc.config.Disabled {
				t.Errorf("Disabled = %v, want %v", result.Disabled, tc.config.Disabled)
			}
			if result.DisableConnectivity != tc.config.DisableConnectivity {
				t.Errorf("DisableConnectivity = %v, want %v", result.DisableConnectivity, tc.config.DisableConnectivity)
			}

			// Check that threshold pointers are different but values match
			checkThresholdClone(t, "CPU", result.CPU, tc.config.CPU)
			checkThresholdClone(t, "Memory", result.Memory, tc.config.Memory)
			checkThresholdClone(t, "Disk", result.Disk, tc.config.Disk)
			checkThresholdClone(t, "DiskRead", result.DiskRead, tc.config.DiskRead)
			checkThresholdClone(t, "DiskWrite", result.DiskWrite, tc.config.DiskWrite)
			checkThresholdClone(t, "NetworkIn", result.NetworkIn, tc.config.NetworkIn)
			checkThresholdClone(t, "NetworkOut", result.NetworkOut, tc.config.NetworkOut)
			checkThresholdClone(t, "Temperature", result.Temperature, tc.config.Temperature)
			checkThresholdClone(t, "Usage", result.Usage, tc.config.Usage)

			// Check Note cloning
			if tc.config.Note == nil {
				if result.Note != nil {
					t.Errorf("Note should be nil")
				}
			} else {
				if result.Note == nil {
					t.Errorf("Note should not be nil")
				} else if result.Note == tc.config.Note {
					t.Error("Note pointer should be different")
				} else if *result.Note != *tc.config.Note {
					t.Errorf("Note value = %q, want %q", *result.Note, *tc.config.Note)
				}
			}

			// Verify modifying clone doesn't affect original
			if result.CPU != nil {
				result.CPU.Trigger = 999
				if tc.config.CPU != nil && tc.config.CPU.Trigger == 999 {
					t.Error("modifying cloned CPU affected original")
				}
			}
			if result.Note != nil {
				*result.Note = "modified"
				if tc.config.Note != nil && *tc.config.Note == "modified" {
					t.Error("modifying cloned Note affected original")
				}
			}
		})
	}
}

// checkThresholdClone is a helper to verify a threshold was properly cloned
func checkThresholdClone(t *testing.T, name string, result, original *HysteresisThreshold) {
	t.Helper()
	if original == nil {
		if result != nil {
			t.Errorf("%s should be nil", name)
		}
		return
	}
	if result == nil {
		t.Errorf("%s should not be nil", name)
		return
	}
	if result == original {
		t.Errorf("%s pointer should be different", name)
	}
	if result.Trigger != original.Trigger {
		t.Errorf("%s.Trigger = %v, want %v", name, result.Trigger, original.Trigger)
	}
	if result.Clear != original.Clear {
		t.Errorf("%s.Clear = %v, want %v", name, result.Clear, original.Clear)
	}
}

// TestEnsureHysteresisThreshold tests the ensureHysteresisThreshold function
func TestEnsureHysteresisThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		threshold   *HysteresisThreshold
		wantTrigger float64
		wantClear   float64
	}{
		{
			name:      "nil threshold returns nil",
			threshold: nil,
		},
		{
			name:        "threshold with valid clear unchanged",
			threshold:   &HysteresisThreshold{Trigger: 80, Clear: 70},
			wantTrigger: 80,
			wantClear:   70,
		},
		{
			name:        "threshold with zero clear gets default",
			threshold:   &HysteresisThreshold{Trigger: 80, Clear: 0},
			wantTrigger: 80,
			wantClear:   75, // 80 - 5
		},
		{
			name:        "threshold with negative clear gets default",
			threshold:   &HysteresisThreshold{Trigger: 80, Clear: -10},
			wantTrigger: 80,
			wantClear:   75, // 80 - 5
		},
		{
			name:        "low trigger value",
			threshold:   &HysteresisThreshold{Trigger: 10, Clear: 0},
			wantTrigger: 10,
			wantClear:   5, // 10 - 5
		},
		{
			name:        "trigger at 5 with zero clear",
			threshold:   &HysteresisThreshold{Trigger: 5, Clear: 0},
			wantTrigger: 5,
			wantClear:   0, // 5 - 5 = 0
		},
		{
			name:        "trigger below 5 with zero clear",
			threshold:   &HysteresisThreshold{Trigger: 3, Clear: 0},
			wantTrigger: 3,
			wantClear:   -2, // 3 - 5 = -2 (function doesn't clamp)
		},
		{
			name:        "trigger at 100 with zero clear",
			threshold:   &HysteresisThreshold{Trigger: 100, Clear: 0},
			wantTrigger: 100,
			wantClear:   95, // 100 - 5
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := ensureHysteresisThreshold(tc.threshold)

			if tc.threshold == nil {
				if result != nil {
					t.Errorf("ensureHysteresisThreshold(nil) = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("ensureHysteresisThreshold returned nil for non-nil input")
			}

			// Note: function modifies in place, so result == tc.threshold
			if result.Trigger != tc.wantTrigger {
				t.Errorf("Trigger = %v, want %v", result.Trigger, tc.wantTrigger)
			}
			if result.Clear != tc.wantClear {
				t.Errorf("Clear = %v, want %v", result.Clear, tc.wantClear)
			}
		})
	}
}

// TestParsePulseTags tests the parsePulseTags function
func TestParsePulseTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags []string
		want pulseTagSettings
	}{
		{
			name: "nil tags",
			tags: nil,
			want: pulseTagSettings{},
		},
		{
			name: "empty tags",
			tags: []string{},
			want: pulseTagSettings{},
		},
		{
			name: "no pulse tags",
			tags: []string{"production", "web-server", "ubuntu"},
			want: pulseTagSettings{},
		},
		{
			name: "pulse-no-alerts tag",
			tags: []string{"pulse-no-alerts"},
			want: pulseTagSettings{Suppress: true},
		},
		{
			name: "pulse-monitor-only tag",
			tags: []string{"pulse-monitor-only"},
			want: pulseTagSettings{MonitorOnly: true},
		},
		{
			name: "pulse-relaxed tag",
			tags: []string{"pulse-relaxed"},
			want: pulseTagSettings{Relaxed: true},
		},
		{
			name: "all pulse tags",
			tags: []string{"pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed"},
			want: pulseTagSettings{Suppress: true, MonitorOnly: true, Relaxed: true},
		},
		{
			name: "mixed tags with pulse tags",
			tags: []string{"production", "pulse-no-alerts", "web-server", "pulse-relaxed"},
			want: pulseTagSettings{Suppress: true, Relaxed: true},
		},
		{
			name: "uppercase pulse tag",
			tags: []string{"PULSE-NO-ALERTS"},
			want: pulseTagSettings{Suppress: true},
		},
		{
			name: "mixed case pulse tag",
			tags: []string{"Pulse-Monitor-Only"},
			want: pulseTagSettings{MonitorOnly: true},
		},
		{
			name: "pulse tag with whitespace",
			tags: []string{"  pulse-no-alerts  "},
			want: pulseTagSettings{Suppress: true},
		},
		{
			name: "pulse tag with leading whitespace",
			tags: []string{"  pulse-relaxed"},
			want: pulseTagSettings{Relaxed: true},
		},
		{
			name: "pulse tag with trailing whitespace",
			tags: []string{"pulse-monitor-only  "},
			want: pulseTagSettings{MonitorOnly: true},
		},
		{
			name: "similar but not pulse tag",
			tags: []string{"pulse-alerts", "pulse-monitor", "pulse"},
			want: pulseTagSettings{},
		},
		{
			name: "pulse tag substring does not match",
			tags: []string{"mypulse-no-alerts", "pulse-no-alerts-extra"},
			want: pulseTagSettings{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := parsePulseTags(tc.tags)

			if result.Suppress != tc.want.Suppress {
				t.Errorf("Suppress = %v, want %v", result.Suppress, tc.want.Suppress)
			}
			if result.MonitorOnly != tc.want.MonitorOnly {
				t.Errorf("MonitorOnly = %v, want %v", result.MonitorOnly, tc.want.MonitorOnly)
			}
			if result.Relaxed != tc.want.Relaxed {
				t.Errorf("Relaxed = %v, want %v", result.Relaxed, tc.want.Relaxed)
			}
		})
	}
}

// TestNormalizeMetricTimeThresholds tests the normalizeMetricTimeThresholds function
func TestNormalizeMetricTimeThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]map[string]int
		want  map[string]map[string]int
	}{
		{
			name:  "nil input returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty input returns nil",
			input: map[string]map[string]int{},
			want:  nil,
		},
		{
			name: "valid input is normalized",
			input: map[string]map[string]int{
				"guest": {"cpu": 60, "memory": 120},
			},
			want: map[string]map[string]int{
				"guest": {"cpu": 60, "memory": 120},
			},
		},
		{
			name: "keys are lowercased",
			input: map[string]map[string]int{
				"GUEST": {"CPU": 60, "MEMORY": 120},
			},
			want: map[string]map[string]int{
				"guest": {"cpu": 60, "memory": 120},
			},
		},
		{
			name: "keys are trimmed",
			input: map[string]map[string]int{
				"  guest  ": {"  cpu  ": 60},
			},
			want: map[string]map[string]int{
				"guest": {"cpu": 60},
			},
		},
		{
			name: "negative delays are dropped",
			input: map[string]map[string]int{
				"guest": {"cpu": 60, "memory": -1},
			},
			want: map[string]map[string]int{
				"guest": {"cpu": 60},
			},
		},
		{
			name: "zero delay is valid",
			input: map[string]map[string]int{
				"guest": {"cpu": 0},
			},
			want: map[string]map[string]int{
				"guest": {"cpu": 0},
			},
		},
		{
			name: "empty type key is dropped",
			input: map[string]map[string]int{
				"":      {"cpu": 60},
				"guest": {"memory": 120},
			},
			want: map[string]map[string]int{
				"guest": {"memory": 120},
			},
		},
		{
			name: "whitespace-only type key is dropped",
			input: map[string]map[string]int{
				"   ":   {"cpu": 60},
				"guest": {"memory": 120},
			},
			want: map[string]map[string]int{
				"guest": {"memory": 120},
			},
		},
		{
			name: "empty metric key is dropped",
			input: map[string]map[string]int{
				"guest": {"": 60, "cpu": 120},
			},
			want: map[string]map[string]int{
				"guest": {"cpu": 120},
			},
		},
		{
			name: "empty metrics map is dropped",
			input: map[string]map[string]int{
				"guest": {},
				"node":  {"cpu": 60},
			},
			want: map[string]map[string]int{
				"node": {"cpu": 60},
			},
		},
		{
			name: "all invalid results in nil",
			input: map[string]map[string]int{
				"":      {"cpu": 60},
				"guest": {"": 60, "memory": -1},
			},
			want: nil,
		},
		{
			name: "multiple resource types",
			input: map[string]map[string]int{
				"guest": {"cpu": 60, "memory": 120},
				"node":  {"disk": 30},
			},
			want: map[string]map[string]int{
				"guest": {"cpu": 60, "memory": 120},
				"node":  {"disk": 30},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := NormalizeMetricTimeThresholds(tc.input)

			if tc.want == nil {
				if result != nil {
					t.Errorf("normalizeMetricTimeThresholds() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("normalizeMetricTimeThresholds() = nil, want %v", tc.want)
			}

			// Check all expected keys exist with correct values
			for typeKey, metrics := range tc.want {
				resultMetrics, exists := result[typeKey]
				if !exists {
					t.Errorf("missing type key %q", typeKey)
					continue
				}
				for metricKey, delay := range metrics {
					resultDelay, exists := resultMetrics[metricKey]
					if !exists {
						t.Errorf("missing metric key %q for type %q", metricKey, typeKey)
						continue
					}
					if resultDelay != delay {
						t.Errorf("result[%q][%q] = %d, want %d", typeKey, metricKey, resultDelay, delay)
					}
				}
				// Check no extra metric keys
				if len(resultMetrics) != len(metrics) {
					t.Errorf("result[%q] has %d keys, want %d", typeKey, len(resultMetrics), len(metrics))
				}
			}
			// Check no extra type keys
			if len(result) != len(tc.want) {
				t.Errorf("result has %d type keys, want %d", len(result), len(tc.want))
			}
		})
	}
}

// TestGetThresholdForMetric tests the getThresholdForMetric function
func TestGetThresholdForMetric(t *testing.T) {
	t.Parallel()

	cpuThreshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	memoryThreshold := &HysteresisThreshold{Trigger: 85, Clear: 75}
	diskThreshold := &HysteresisThreshold{Trigger: 90, Clear: 85}
	diskReadThreshold := &HysteresisThreshold{Trigger: 50, Clear: 40}
	diskWriteThreshold := &HysteresisThreshold{Trigger: 55, Clear: 45}
	networkInThreshold := &HysteresisThreshold{Trigger: 70, Clear: 60}
	networkOutThreshold := &HysteresisThreshold{Trigger: 75, Clear: 65}
	temperatureThreshold := &HysteresisThreshold{Trigger: 65, Clear: 55}
	usageThreshold := &HysteresisThreshold{Trigger: 88, Clear: 78}

	config := ThresholdConfig{
		CPU:         cpuThreshold,
		Memory:      memoryThreshold,
		Disk:        diskThreshold,
		DiskRead:    diskReadThreshold,
		DiskWrite:   diskWriteThreshold,
		NetworkIn:   networkInThreshold,
		NetworkOut:  networkOutThreshold,
		Temperature: temperatureThreshold,
		Usage:       usageThreshold,
	}

	tests := []struct {
		name       string
		metricType string
		want       *HysteresisThreshold
	}{
		{"cpu", "cpu", cpuThreshold},
		{"memory", "memory", memoryThreshold},
		{"disk", "disk", diskThreshold},
		{"diskRead", "diskRead", diskReadThreshold},
		{"diskWrite", "diskWrite", diskWriteThreshold},
		{"networkIn", "networkIn", networkInThreshold},
		{"networkOut", "networkOut", networkOutThreshold},
		{"temperature", "temperature", temperatureThreshold},
		{"usage", "usage", usageThreshold},
		{"unknown metric", "unknown", nil},
		{"empty metric", "", nil},
		{"case sensitive - CPU", "CPU", nil},
		{"case sensitive - Memory", "Memory", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := getThresholdForMetric(config, tc.metricType)
			if result != tc.want {
				t.Errorf("getThresholdForMetric(%q) = %v, want %v", tc.metricType, result, tc.want)
			}
		})
	}
}

// TestGetThresholdForMetric_EmptyConfig tests getThresholdForMetric with empty config
func TestGetThresholdForMetric_EmptyConfig(t *testing.T) {
	t.Parallel()

	config := ThresholdConfig{}

	metricTypes := []string{"cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut", "temperature", "usage"}

	for _, metricType := range metricTypes {
		t.Run(metricType, func(t *testing.T) {
			t.Parallel()

			result := getThresholdForMetric(config, metricType)
			if result != nil {
				t.Errorf("getThresholdForMetric(%q) with empty config = %v, want nil", metricType, result)
			}
		})
	}
}

// TestGetThresholdForMetricFromConfig tests the getThresholdForMetricFromConfig function
func TestGetThresholdForMetricFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      ThresholdConfig
		metricType  string
		wantNil     bool
		wantTrigger float64
		wantClear   float64
	}{
		{
			name:       "empty config returns nil",
			config:     ThresholdConfig{},
			metricType: "cpu",
			wantNil:    true,
		},
		{
			name: "cpu threshold returned",
			config: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 80, Clear: 70},
			},
			metricType:  "cpu",
			wantTrigger: 80,
			wantClear:   70,
		},
		{
			name: "memory threshold returned",
			config: ThresholdConfig{
				Memory: &HysteresisThreshold{Trigger: 85, Clear: 75},
			},
			metricType:  "memory",
			wantTrigger: 85,
			wantClear:   75,
		},
		{
			name: "disk threshold returned",
			config: ThresholdConfig{
				Disk: &HysteresisThreshold{Trigger: 90, Clear: 85},
			},
			metricType:  "disk",
			wantTrigger: 90,
			wantClear:   85,
		},
		{
			name: "diskRead threshold returned",
			config: ThresholdConfig{
				DiskRead: &HysteresisThreshold{Trigger: 50, Clear: 40},
			},
			metricType:  "diskRead",
			wantTrigger: 50,
			wantClear:   40,
		},
		{
			name: "diskWrite threshold returned",
			config: ThresholdConfig{
				DiskWrite: &HysteresisThreshold{Trigger: 55, Clear: 45},
			},
			metricType:  "diskWrite",
			wantTrigger: 55,
			wantClear:   45,
		},
		{
			name: "networkIn threshold returned",
			config: ThresholdConfig{
				NetworkIn: &HysteresisThreshold{Trigger: 70, Clear: 60},
			},
			metricType:  "networkIn",
			wantTrigger: 70,
			wantClear:   60,
		},
		{
			name: "networkOut threshold returned",
			config: ThresholdConfig{
				NetworkOut: &HysteresisThreshold{Trigger: 75, Clear: 65},
			},
			metricType:  "networkOut",
			wantTrigger: 75,
			wantClear:   65,
		},
		{
			name: "temperature threshold returned",
			config: ThresholdConfig{
				Temperature: &HysteresisThreshold{Trigger: 65, Clear: 55},
			},
			metricType:  "temperature",
			wantTrigger: 65,
			wantClear:   55,
		},
		{
			name: "usage threshold returned",
			config: ThresholdConfig{
				Usage: &HysteresisThreshold{Trigger: 88, Clear: 78},
			},
			metricType:  "usage",
			wantTrigger: 88,
			wantClear:   78,
		},
		{
			name: "unknown metric returns nil",
			config: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 80, Clear: 70},
			},
			metricType: "unknown",
			wantNil:    true,
		},
		{
			name: "threshold with zero clear gets default hysteresis",
			config: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 80, Clear: 0},
			},
			metricType:  "cpu",
			wantTrigger: 80,
			wantClear:   75, // 80 - 5 default margin
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := getThresholdForMetricFromConfig(tc.config, tc.metricType)

			if tc.wantNil {
				if result != nil {
					t.Errorf("getThresholdForMetricFromConfig() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("getThresholdForMetricFromConfig() = nil, want non-nil")
			}

			if result.Trigger != tc.wantTrigger {
				t.Errorf("Trigger = %v, want %v", result.Trigger, tc.wantTrigger)
			}
			if result.Clear != tc.wantClear {
				t.Errorf("Clear = %v, want %v", result.Clear, tc.wantClear)
			}
		})
	}
}
