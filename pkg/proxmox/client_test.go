package proxmox

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func TestDiskUnmarshalWearout(t *testing.T) {
	tests := []struct {
		name     string
		wearout  json.RawMessage
		expected int
	}{
		{
			name:     "numeric",
			wearout:  json.RawMessage(`81`),
			expected: 81,
		},
		{
			name:     "numeric string",
			wearout:  json.RawMessage(`"81"`),
			expected: 81,
		},
		{
			name:     "escaped numeric string",
			wearout:  json.RawMessage(`"\"81\""`),
			expected: 81,
		},
		{
			name:     "percentage string",
			wearout:  json.RawMessage(`"81%"`),
			expected: 81,
		},
		{
			name:     "percentage with spaces",
			wearout:  json.RawMessage(`"  82 % "`),
			expected: 82,
		},
		{
			name:     "not applicable string",
			wearout:  json.RawMessage(`"N/A"`),
			expected: wearoutUnknown,
		},
		{
			name:     "empty string",
			wearout:  json.RawMessage(`""`),
			expected: wearoutUnknown,
		},
		{
			name:     "null value",
			wearout:  json.RawMessage(`null`),
			expected: wearoutUnknown,
		},
		{
			name:     "unknown string",
			wearout:  json.RawMessage(`"Unknown"`),
			expected: wearoutUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"devpath":"/dev/sda","model":"Example","serial":"123","type":"hdd","health":"OK","wearout":%s,"size":1000,"rpm":7200,"used":"LVM","vendor":"Example","wwn":"example"}`, tc.wearout)
			var disk Disk
			if err := json.Unmarshal([]byte(payload), &disk); err != nil {
				t.Fatalf("unexpected error unmarshalling disk: %v", err)
			}
			if disk.Wearout != tc.expected {
				t.Fatalf("wearout: got %d, want %d", disk.Wearout, tc.expected)
			}
		})
	}
}

func TestVMFileSystemUnmarshalFlexibleNumbers(t *testing.T) {
	t.Run("accepts numeric values", func(t *testing.T) {
		payload := `{"name":"rootfs","type":"zfs","mountpoint":"/","total-bytes":8589934592,"used-bytes":3221225472,"disk":[{"dev":"/dev/vtbd0p2"}]}`
		var fs VMFileSystem
		if err := json.Unmarshal([]byte(payload), &fs); err != nil {
			t.Fatalf("unexpected error unmarshalling numeric fields: %v", err)
		}
		if fs.TotalBytes != 8589934592 || fs.UsedBytes != 3221225472 {
			t.Fatalf("unexpected numeric values: got total=%d used=%d", fs.TotalBytes, fs.UsedBytes)
		}
	})

	t.Run("accepts numeric strings", func(t *testing.T) {
		payload := `{"name":"rootfs","type":"ufs","mountpoint":"/","total-bytes":"5368709120","used-bytes":"2147483648","disk":[{"dev":"/dev/vtbd0p3"}]}`
		var fs VMFileSystem
		if err := json.Unmarshal([]byte(payload), &fs); err != nil {
			t.Fatalf("unexpected error unmarshalling string fields: %v", err)
		}
		if fs.TotalBytes != 5368709120 || fs.UsedBytes != 2147483648 {
			t.Fatalf("unexpected string values: got total=%d used=%d", fs.TotalBytes, fs.UsedBytes)
		}
	})

	t.Run("accepts float-like strings", func(t *testing.T) {
		payload := `{"name":"rootfs","type":"ufs","mountpoint":"/","total-bytes":"1073741824.0","used-bytes":"536870912.0","disk":[{"dev":"/dev/vtbd0p4"}]}`
		var fs VMFileSystem
		if err := json.Unmarshal([]byte(payload), &fs); err != nil {
			t.Fatalf("unexpected error unmarshalling float-like strings: %v", err)
		}
		if fs.TotalBytes != 1073741824 || fs.UsedBytes != 536870912 {
			t.Fatalf("unexpected float string values: got total=%d used=%d", fs.TotalBytes, fs.UsedBytes)
		}
	})
}

func TestMemoryStatusEffectiveAvailable(t *testing.T) {
	t.Run("nil receiver returns zero", func(t *testing.T) {
		var status *MemoryStatus
		if status.EffectiveAvailable() != 0 {
			t.Fatalf("expected nil receiver to return 0")
		}
	})

	tests := []struct {
		name   string
		status MemoryStatus
		want   uint64
	}{
		{
			name:   "uses available field when set",
			status: MemoryStatus{Total: 16 * 1024, Available: 6 * 1024},
			want:   6 * 1024,
		},
		{
			name:   "uses avail field fallback",
			status: MemoryStatus{Total: 16 * 1024, Avail: 5 * 1024},
			want:   5 * 1024,
		},
		{
			name:   "derives from free buffers cached",
			status: MemoryStatus{Total: 32 * 1024, Free: 4 * 1024, Buffers: 2 * 1024, Cached: 6 * 1024},
			want:   12 * 1024,
		},
		{
			name: "available zero but buffers and cache present",
			status: MemoryStatus{
				Total:   64 * 1024,
				Used:    40 * 1024,
				Free:    6 * 1024,
				Buffers: 8 * 1024,
				Cached:  10 * 1024,
			},
			want: 24 * 1024,
		},
		{
			name:   "caps derived value at total",
			status: MemoryStatus{Total: 8 * 1024, Free: 4 * 1024, Buffers: 4 * 1024, Cached: 4 * 1024},
			want:   8 * 1024,
		},
		{
			name:   "returns zero when no data",
			status: MemoryStatus{Total: 24 * 1024},
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.EffectiveAvailable(); got != tc.want {
				t.Fatalf("EffectiveAvailable: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMemoryStatusUnmarshalFlexibleValues(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    MemoryStatus
	}{
		{
			name:    "numeric strings",
			payload: `{"total":"16549875712","used":"6050492416","free":"2467389440","available":"10499383296","buffers":"0","cached":"0","shared":"0"}`,
			want: MemoryStatus{
				Total:     16549875712,
				Used:      6050492416,
				Free:      2467389440,
				Available: 10499383296,
				Buffers:   0,
				Cached:    0,
				Shared:    0,
			},
		},
		{
			name:    "scientific notation",
			payload: `{"total":1.6549875712e+10,"used":6.050492416e+09,"free":2.46738944e+09,"available":1.0499383296e+10,"buffers":0,"cached":0,"shared":0}`,
			want: MemoryStatus{
				Total:     16549875712,
				Used:      6050492416,
				Free:      2467389440,
				Available: 10499383296,
			},
		},
		{
			name:    "float-like strings with spaces",
			payload: `{"total":" 8589934592.0 ","used":"3221225472.0","free":"536870912.0","avail":"4831838208.0","buffers":"67108864","cached":"4026531840","shared":"0"}`,
			want: MemoryStatus{
				Total:   8589934592,
				Used:    3221225472,
				Free:    536870912,
				Avail:   4831838208,
				Buffers: 67108864,
				Cached:  4026531840,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var status MemoryStatus
			if err := json.Unmarshal([]byte(tc.payload), &status); err != nil {
				t.Fatalf("unexpected error unmarshalling %s payload: %v", tc.name, err)
			}

			if status.Total != tc.want.Total {
				t.Fatalf("total: got %d, want %d", status.Total, tc.want.Total)
			}
			if status.Used != tc.want.Used {
				t.Fatalf("used: got %d, want %d", status.Used, tc.want.Used)
			}
			if status.Free != tc.want.Free {
				t.Fatalf("free: got %d, want %d", status.Free, tc.want.Free)
			}
			if status.Available != tc.want.Available {
				t.Fatalf("available: got %d, want %d", status.Available, tc.want.Available)
			}
			if status.Avail != tc.want.Avail {
				t.Fatalf("avail: got %d, want %d", status.Avail, tc.want.Avail)
			}
			if status.Buffers != tc.want.Buffers {
				t.Fatalf("buffers: got %d, want %d", status.Buffers, tc.want.Buffers)
			}
			if status.Cached != tc.want.Cached {
				t.Fatalf("cached: got %d, want %d", status.Cached, tc.want.Cached)
			}
			if status.Shared != tc.want.Shared {
				t.Fatalf("shared: got %d, want %d", status.Shared, tc.want.Shared)
			}
		})
	}
}

func TestFlexIntUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    FlexInt
		wantErr bool
	}{
		{
			name:  "integer",
			input: "42",
			want:  FlexInt(42),
		},
		{
			name:  "zero",
			input: "0",
			want:  FlexInt(0),
		},
		{
			name:  "negative integer",
			input: "-10",
			want:  FlexInt(-10),
		},
		{
			name:  "float truncates",
			input: "1.5",
			want:  FlexInt(1),
		},
		{
			name:  "float 2.9 truncates",
			input: "2.9",
			want:  FlexInt(2),
		},
		{
			name:  "string integer",
			input: `"123"`,
			want:  FlexInt(123),
		},
		{
			name:  "string float",
			input: `"1.5"`,
			want:  FlexInt(1),
		},
		{
			name:  "string float 3.7",
			input: `"3.7"`,
			want:  FlexInt(3),
		},
		{
			name:  "large integer",
			input: "1000000",
			want:  FlexInt(1000000),
		},
		{
			name:    "invalid string",
			input:   `"not a number"`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   `""`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var f FlexInt
			err := f.UnmarshalJSON([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f != tc.want {
				t.Fatalf("got %d, want %d", f, tc.want)
			}
		})
	}
}

func TestParseUint64Flexible(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    uint64
		wantErr bool
	}{
		// nil
		{name: "nil", value: nil, want: 0},
		// uint64
		{name: "uint64", value: uint64(42), want: 42},
		{name: "uint64 max", value: uint64(18446744073709551615), want: 18446744073709551615},
		// int
		{name: "int positive", value: int(100), want: 100},
		{name: "int zero", value: int(0), want: 0},
		{name: "int negative", value: int(-5), want: 0},
		// int64
		{name: "int64 positive", value: int64(200), want: 200},
		{name: "int64 negative", value: int64(-10), want: 0},
		// float64
		{name: "float64 positive", value: float64(3.7), want: 3},
		{name: "float64 truncates down", value: float64(9.9), want: 9},
		{name: "float64 negative", value: float64(-1.5), want: 0},
		{name: "float64 zero", value: float64(0.0), want: 0},
		// json.Number
		{name: "json.Number integer", value: json.Number("99"), want: 99},
		{name: "json.Number float", value: json.Number("3.14"), want: 3},
		{name: "json.Number invalid", value: json.Number("abc"), wantErr: true},
		// string - empty/whitespace
		{name: "string empty", value: "", want: 0},
		{name: "string whitespace", value: "   ", want: 0},
		// string - decimal
		{name: "string decimal", value: "12345", want: 12345},
		{name: "string with whitespace", value: "  678  ", want: 678},
		{name: "string invalid decimal", value: "abc", wantErr: true},
		{name: "string negative decimal", value: "-100", wantErr: true},
		// string - hex
		{name: "string hex lowercase", value: "0x10", want: 16},
		{name: "string hex uppercase", value: "0X1F", want: 31},
		{name: "string hex invalid", value: "0xGG", wantErr: true},
		// string - float notation
		{name: "string float", value: "3.14", want: 3},
		{name: "string scientific", value: "1e3", want: 1000},
		{name: "string scientific uppercase", value: "1.5E2", want: 150},
		{name: "string negative float", value: "-2.5", want: 0},
		{name: "string invalid float", value: "1.2.3", wantErr: true},
		// unsupported type
		{name: "unsupported bool", value: true, wantErr: true},
		{name: "unsupported slice", value: []int{1, 2}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseUint64Flexible(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil with value %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCoerceUint64(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   interface{}
		want    uint64
		wantErr bool
	}{
		// nil handling
		{
			name:  "nil returns zero",
			field: "test",
			value: nil,
			want:  0,
		},
		// float64 handling
		{
			name:  "float64 positive",
			field: "test",
			value: float64(100.0),
			want:  100,
		},
		{
			name:  "float64 rounds",
			field: "test",
			value: float64(100.6),
			want:  101,
		},
		{
			name:  "float64 negative returns zero",
			field: "test",
			value: float64(-10),
			want:  0,
		},
		{
			name:  "float64 zero",
			field: "test",
			value: float64(0),
			want:  0,
		},
		{
			name:    "float64 NaN returns error",
			field:   "test",
			value:   math.NaN(),
			wantErr: true,
		},
		{
			name:    "float64 positive infinity returns error",
			field:   "test",
			value:   math.Inf(1),
			wantErr: true,
		},
		// int handling
		{
			name:  "int positive",
			field: "test",
			value: int(42),
			want:  42,
		},
		{
			name:  "int negative returns zero",
			field: "test",
			value: int(-5),
			want:  0,
		},
		{
			name:  "int zero",
			field: "test",
			value: int(0),
			want:  0,
		},
		// int64 handling
		{
			name:  "int64 positive",
			field: "test",
			value: int64(1000000000000),
			want:  1000000000000,
		},
		{
			name:  "int64 negative returns zero",
			field: "test",
			value: int64(-100),
			want:  0,
		},
		// int32 handling
		{
			name:  "int32 positive",
			field: "test",
			value: int32(12345),
			want:  12345,
		},
		{
			name:  "int32 negative returns zero",
			field: "test",
			value: int32(-1),
			want:  0,
		},
		// uint32 handling
		{
			name:  "uint32",
			field: "test",
			value: uint32(4294967295),
			want:  4294967295,
		},
		// uint64 handling
		{
			name:  "uint64",
			field: "test",
			value: uint64(18446744073709551615),
			want:  18446744073709551615,
		},
		// json.Number handling
		{
			name:  "json.Number integer",
			field: "test",
			value: json.Number("12345"),
			want:  12345,
		},
		{
			name:  "json.Number float",
			field: "test",
			value: json.Number("123.45"),
			want:  123,
		},
		// string handling
		{
			name:  "string integer",
			field: "test",
			value: "12345",
			want:  12345,
		},
		{
			name:  "string with whitespace",
			field: "test",
			value: "  12345  ",
			want:  12345,
		},
		{
			name:  "string empty",
			field: "test",
			value: "",
			want:  0,
		},
		{
			name:  "string null",
			field: "test",
			value: "null",
			want:  0,
		},
		{
			name:  "string NULL uppercase",
			field: "test",
			value: "NULL",
			want:  0,
		},
		{
			name:  "string with quotes",
			field: "test",
			value: `"12345"`,
			want:  12345,
		},
		{
			name:  "string with single quotes",
			field: "test",
			value: `'12345'`,
			want:  12345,
		},
		{
			name:  "string with commas",
			field: "test",
			value: "1,000,000",
			want:  1000000,
		},
		{
			name:  "string float notation",
			field: "test",
			value: "123.45",
			want:  123,
		},
		{
			name:  "string scientific notation",
			field: "test",
			value: "1e6",
			want:  1000000,
		},
		{
			name:  "string scientific notation uppercase",
			field: "test",
			value: "1E6",
			want:  1000000,
		},
		{
			name:    "string invalid",
			field:   "test",
			value:   "not a number",
			wantErr: true,
		},
		// unsupported type
		{
			name:    "unsupported type bool",
			field:   "test",
			value:   true,
			wantErr: true,
		},
		{
			name:    "unsupported type slice",
			field:   "test",
			value:   []int{1, 2, 3},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := coerceUint64(tc.field, tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// TestMemoryStatusEffectiveAvailable_RegressionIssue435 tests the specific scenarios
// reported in GitHub issue #435 where memory calculations incorrectly included cache/buffers
func TestMemoryStatusEffectiveAvailable_RegressionIssue435(t *testing.T) {
	tests := []struct {
		name          string
		status        MemoryStatus
		wantAvailable uint64
		wantUsedPct   float64 // Expected usage percentage when using EffectiveAvailable
		description   string
	}{
		{
			name: "real proxmox 8.x node with available field",
			status: MemoryStatus{
				Total:     16466186240, // 16GB
				Used:      13981696000, // ~14GB (includes cache)
				Free:      1558323200,  // ~1.5GB
				Available: 7422545920,  // ~7GB (cache-aware - this is what we should use!)
				Buffers:   23592960,    // ~23MB
				Cached:    5478891520,  // ~5.4GB
			},
			wantAvailable: 7422545920,
			wantUsedPct:   54.92, // Should be ~55%, not ~85%!
			description:   "Proxmox 8.x returns 'available' field - most accurate",
		},
		{
			name: "older proxmox with avail field",
			status: MemoryStatus{
				Total: 8589934592, // 8GB
				Used:  6871947674, // ~6.4GB (includes cache)
				Free:  805306368,  // ~768MB
				Avail: 3221225472, // ~3GB (cache-aware)
			},
			wantAvailable: 3221225472,
			wantUsedPct:   62.5, // Should be ~62.5%, not ~80%
			description:   "Older Proxmox uses 'avail' field as fallback",
		},
		{
			name: "proxmox without available/avail - derive from components",
			status: MemoryStatus{
				Total:   16777216000, // ~16GB
				Used:    13421772800, // ~12.5GB (includes cache)
				Free:    2147483648,  // 2GB
				Buffers: 536870912,   // 512MB
				Cached:  4294967296,  // 4GB
			},
			wantAvailable: 6979321856, // Free + Buffers + Cached = ~6.5GB
			wantUsedPct:   58.4,       // Should be ~58%, not ~80%
			description:   "When available/avail missing, derive from free+buffers+cached",
		},
		{
			name: "proxmox 8.4 hides cache fields - derive from total-minus-used gap",
			status: MemoryStatus{
				Total:   134794743808, // ~125.6GB
				Used:    107351023616, // ~100GB actual usage
				Free:    6471057408,   // ~6GB bare free reported
				Buffers: 0,
				Cached:  0,
			},
			wantAvailable: 27443720192, // total - used => ~25.6GB reclaimable (free + cache)
			wantUsedPct:   79.6,        // Matches Proxmox node dashboard
			description:   "Proxmox 8.4 stops reporting buffers/cached; use total-used gap to recover cache-aware metric",
		},
		{
			name: "issue #435 specific case - 86% vs 42% real usage",
			status: MemoryStatus{
				Total:     33554432000, // ~32GB
				Used:      28857589760, // ~27GB (includes cache - WRONG!)
				Free:      1073741824,  // 1GB
				Available: 19327352832, // ~18GB (cache-aware - CORRECT!)
			},
			wantAvailable: 19327352832,
			wantUsedPct:   42.4, // Should be ~42%, not 86%!
			description:   "Real user report: 86% shown when actual usage is 42%",
		},
		{
			name: "missing available but buffers/cached still present (issue 553 guard)",
			status: MemoryStatus{
				Total:   34359738368, // 32GB
				Used:    27917287424, // ~26GB reported used (includes cache)
				Free:    2147483648,  // 2GB
				Buffers: 3221225472,  // 3GB
				Cached:  8053063680,  // 7.5GB
			},
			wantAvailable: 13421772800, // Free + Buffers + Cached
			wantUsedPct:   60.94,
			description:   "When available=0 but buffers/cached exist, derive reclaimable memory instead of alerting",
		},
		{
			name: "missing all cache-aware fields - fallback to zero",
			status: MemoryStatus{
				Total: 8589934592,
				Used:  6871947674,
				Free:  0, // All fields missing
			},
			wantAvailable: 1717986918, // Derived from total - used
			wantUsedPct:   80.0,       // Still aligns with cache-inclusive calculation when nothing else reported
			description:   "When all cache fields missing, fall back to total-used gap instead of zero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.status.EffectiveAvailable()
			if got != tc.wantAvailable {
				t.Errorf("EffectiveAvailable() = %d, want %d\nScenario: %s",
					got, tc.wantAvailable, tc.description)
			}

			// Calculate what the usage percentage would be with cache-aware calculation
			if tc.wantAvailable > 0 && tc.wantAvailable <= tc.status.Total {
				actualUsed := tc.status.Total - tc.wantAvailable
				usagePct := (float64(actualUsed) / float64(tc.status.Total)) * 100

				// Allow 0.5% tolerance for floating point differences
				if usagePct < tc.wantUsedPct-0.5 || usagePct > tc.wantUsedPct+0.5 {
					t.Errorf("Calculated usage = %.2f%%, want ~%.2f%%\nScenario: %s",
						usagePct, tc.wantUsedPct, tc.description)
				}
			}
		})
	}
}
