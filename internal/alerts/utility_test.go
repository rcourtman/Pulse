package alerts

import (
	"testing"
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
