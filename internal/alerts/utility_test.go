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
