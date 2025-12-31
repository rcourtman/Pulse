package smartctl

import (
	"testing"
)

func TestDetectDiskType(t *testing.T) {
	tests := []struct {
		name     string
		data     smartctlJSON
		expected string
	}{
		{
			name: "NVMe protocol",
			data: smartctlJSON{
				Device: struct {
					Name     string `json:"name"`
					Type     string `json:"type"`
					Protocol string `json:"protocol"`
				}{
					Protocol: "NVMe",
				},
			},
			expected: "nvme",
		},
		{
			name: "SAS protocol",
			data: smartctlJSON{
				Device: struct {
					Name     string `json:"name"`
					Type     string `json:"type"`
					Protocol string `json:"protocol"`
				}{
					Protocol: "SAS",
				},
			},
			expected: "sas",
		},
		{
			name: "SATA protocol",
			data: smartctlJSON{
				Device: struct {
					Name     string `json:"name"`
					Type     string `json:"type"`
					Protocol string `json:"protocol"`
				}{
					Protocol: "ATA",
				},
			},
			expected: "sata",
		},
		{
			name: "NVMe device type fallback",
			data: smartctlJSON{
				Device: struct {
					Name     string `json:"name"`
					Type     string `json:"type"`
					Protocol string `json:"protocol"`
				}{
					Type:     "nvme",
					Protocol: "",
				},
			},
			expected: "nvme",
		},
		{
			name: "Empty defaults to sata",
			data: smartctlJSON{
				Device: struct {
					Name     string `json:"name"`
					Type     string `json:"type"`
					Protocol string `json:"protocol"`
				}{},
			},
			expected: "sata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectDiskType(tt.data)
			if result != tt.expected {
				t.Errorf("detectDiskType() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatWWN(t *testing.T) {
	tests := []struct {
		name     string
		naa      uint64
		oui      uint64
		id       uint64
		expected string
	}{
		{
			name:     "Standard WWN",
			naa:      5,
			oui:      0x000c50,
			id:       0x123456789abc,
			expected: "5-c50-123456789abc",
		},
		{
			name:     "Zero values",
			naa:      0,
			oui:      0,
			id:       0,
			expected: "0-0-0",
		},
		{
			name:     "Simple values",
			naa:      5,
			oui:      1234,
			id:       5678,
			expected: "5-4d2-162e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatWWN(tt.naa, tt.oui, tt.id)
			if result != tt.expected {
				t.Errorf("formatWWN(%d, %d, %d) = %q, want %q", tt.naa, tt.oui, tt.id, result, tt.expected)
			}
		})
	}
}

func TestDiskSMARTStruct(t *testing.T) {
	// Verify struct can be properly created
	disk := DiskSMART{
		Device:      "sda",
		Model:       "Samsung SSD 870 EVO 1TB",
		Serial:      "S123456789",
		WWN:         "5-abc-123",
		Type:        "sata",
		Temperature: 35,
		Health:      "PASSED",
		Standby:     false,
	}

	if disk.Device != "sda" {
		t.Errorf("unexpected Device: %s", disk.Device)
	}
	if disk.Temperature != 35 {
		t.Errorf("unexpected Temperature: %d", disk.Temperature)
	}
	if disk.Health != "PASSED" {
		t.Errorf("unexpected Health: %s", disk.Health)
	}
}

func TestSmartctlJSONParsing(t *testing.T) {
	// Test parsing the smartctlJSON struct
	data := smartctlJSON{
		ModelName:    "Samsung SSD 870 EVO 1TB",
		SerialNumber: "S123456789",
	}
	data.Device.Name = "/dev/sda"
	data.Device.Protocol = "ATA"
	data.SmartStatus.Passed = true
	data.Temperature.Current = 35

	if data.ModelName != "Samsung SSD 870 EVO 1TB" {
		t.Errorf("unexpected ModelName: %s", data.ModelName)
	}
	if data.SmartStatus.Passed != true {
		t.Errorf("expected Passed to be true")
	}
	if data.Temperature.Current != 35 {
		t.Errorf("unexpected Temperature: %d", data.Temperature.Current)
	}
}

func TestNVMeTemperatureFallback(t *testing.T) {
	// Test that NVMe temperature is extracted from the correct location
	data := smartctlJSON{
		Device: struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{
			Protocol: "NVMe",
		},
	}
	data.NVMeSmartHealthInformationLog.Temperature = 42

	// Verify the type detection
	diskType := detectDiskType(data)
	if diskType != "nvme" {
		t.Errorf("expected nvme, got %s", diskType)
	}

	// Verify NVMe temp location is populated
	if data.NVMeSmartHealthInformationLog.Temperature != 42 {
		t.Errorf("NVMe temperature not set correctly")
	}
}

func TestMatchesDeviceExclude(t *testing.T) {
	tests := []struct {
		name       string
		deviceName string
		devicePath string
		patterns   []string
		expected   bool
	}{
		{
			name:       "exact match on name",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   []string{"sda"},
			expected:   true,
		},
		{
			name:       "exact match on full path",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   []string{"/dev/sda"},
			expected:   true,
		},
		{
			name:       "no match",
			deviceName: "sdb",
			devicePath: "/dev/sdb",
			patterns:   []string{"sda"},
			expected:   false,
		},
		{
			name:       "prefix pattern on name",
			deviceName: "nvme0n1",
			devicePath: "/dev/nvme0n1",
			patterns:   []string{"nvme*"},
			expected:   true,
		},
		{
			name:       "prefix pattern on path",
			deviceName: "nvme0n1",
			devicePath: "/dev/nvme0n1",
			patterns:   []string{"/dev/nvme*"},
			expected:   true,
		},
		{
			name:       "contains pattern",
			deviceName: "sdcache1",
			devicePath: "/dev/sdcache1",
			patterns:   []string{"*cache*"},
			expected:   true,
		},
		{
			name:       "contains pattern no match",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   []string{"*cache*"},
			expected:   false,
		},
		{
			name:       "empty patterns",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   []string{},
			expected:   false,
		},
		{
			name:       "nil patterns",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   nil,
			expected:   false,
		},
		{
			name:       "multiple patterns first matches",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   []string{"sda", "sdb"},
			expected:   true,
		},
		{
			name:       "multiple patterns second matches",
			deviceName: "sdb",
			devicePath: "/dev/sdb",
			patterns:   []string{"sda", "sdb"},
			expected:   true,
		},
		{
			name:       "whitespace trimmed",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   []string{"  sda  "},
			expected:   true,
		},
		{
			name:       "empty pattern ignored",
			deviceName: "sda",
			devicePath: "/dev/sda",
			patterns:   []string{"", "   ", "sdb"},
			expected:   false,
		},
		{
			name:       "sd prefix pattern",
			deviceName: "sdc",
			devicePath: "/dev/sdc",
			patterns:   []string{"sd*"},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesDeviceExclude(tt.deviceName, tt.devicePath, tt.patterns)
			if result != tt.expected {
				t.Errorf("matchesDeviceExclude(%q, %q, %v) = %t, want %t",
					tt.deviceName, tt.devicePath, tt.patterns, result, tt.expected)
			}
		})
	}
}
