package smartctl

import (
	"encoding/json"
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
			id:       0x123456789,
			expected: "0x5000c50123456789",
		},
		{
			name:     "Zero values",
			naa:      0,
			oui:      0,
			id:       0,
			expected: "",
		},
		{
			name:     "Simple values",
			naa:      5,
			oui:      1234,
			id:       5678,
			expected: "0x50004d200000162e",
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

func TestFormatNVMeEUI64(t *testing.T) {
	tests := []struct {
		name     string
		oui      uint64
		extID    uint64
		expected string
	}{
		{
			name:     "standard EUI64",
			oui:      0x002538,
			extID:    0x8211b67f9f,
			expected: "eui.0025388211b67f9f",
		},
		{
			name:     "zero values",
			oui:      0,
			extID:    0,
			expected: "",
		},
		{
			name:     "padded values",
			oui:      0x1,
			extID:    0x2,
			expected: "eui.0000010000000002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNVMeEUI64(tt.oui, tt.extID)
			if result != tt.expected {
				t.Errorf("formatNVMeEUI64(%d, %d) = %q, want %q", tt.oui, tt.extID, result, tt.expected)
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
		WWN:         "0x5000abc000000123",
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

func TestParseSMARTOutputNVMeEUI64(t *testing.T) {
	payload := smartctlJSON{
		SerialNumber: "S5GXNX0R123456",
		Device: struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		}{
			Protocol: "NVMe",
		},
		NVMeNamespaces: []nvmeNamespaceJSON{{
			EUI64: nvmeEUI64JSON{
				OUI:   0x002538,
				ExtID: 0x8211b67f9f,
			},
		}},
	}
	payload.SmartStatus = &struct {
		Passed bool `json:"passed"`
	}{Passed: true}
	payload.NVMeSmartHealthInformationLog.Temperature = 304

	output, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := parseSMARTOutput(output, smartctlTarget{Path: "/dev/nvme0n1"})
	if err != nil {
		t.Fatalf("parseSMARTOutput error: %v", err)
	}
	if result.Type != "nvme" {
		t.Fatalf("expected nvme type, got %#v", result)
	}
	if result.WWN != "eui.0025388211b67f9f" {
		t.Fatalf("expected NVMe EUI64 identity, got %#v", result)
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
	data.SmartStatus = &struct {
		Passed bool `json:"passed"`
	}{Passed: true}
	data.Temperature.Current = 35

	if data.ModelName != "Samsung SSD 870 EVO 1TB" {
		t.Errorf("unexpected ModelName: %s", data.ModelName)
	}
	if data.SmartStatus == nil || data.SmartStatus.Passed != true {
		t.Errorf("expected Passed to be true")
	}
	if data.Temperature.Current != 35 {
		t.Errorf("unexpected Temperature: %d", data.Temperature.Current)
	}
}

func TestLinuxSMARTSkipReason(t *testing.T) {
	tests := []struct {
		name   string
		device lsblkDevice
		skip   bool
	}{
		{
			name: "physical sata disk",
			device: lsblkDevice{
				Name:   "sda",
				Type:   "disk",
				Model:  "Samsung SSD 870 EVO",
				Vendor: "ATA",
			},
			skip: false,
		},
		{
			name: "zfs zvol device",
			device: lsblkDevice{
				Name: "zd16",
				Type: "disk",
			},
			skip: true,
		},
		{
			name: "device mapper device",
			device: lsblkDevice{
				Name: "dm-0",
				Type: "disk",
			},
			skip: true,
		},
		{
			name: "virtio transport",
			device: lsblkDevice{
				Name: "vda",
				Type: "disk",
				Tran: "virtio",
			},
			skip: true,
		},
		{
			name: "vmware virtual disk",
			device: lsblkDevice{
				Name:   "sdb",
				Type:   "disk",
				Model:  "Virtual disk",
				Vendor: "VMware",
			},
			skip: true,
		},
		{
			name: "virtual subsystem",
			device: lsblkDevice{
				Name:       "sdc",
				Type:       "disk",
				Subsystems: "block:scsi:vmbus:pci",
			},
			skip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := linuxSMARTSkipReason(tt.device)
			if tt.skip && reason == "" {
				t.Fatalf("expected device to be skipped")
			}
			if !tt.skip && reason != "" {
				t.Fatalf("expected device to be collected, got skip reason %q", reason)
			}
		})
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

func TestParseSMARTAttributes_SATA(t *testing.T) {
	data := &smartctlJSON{}
	data.ATASmartAttributes.Table = []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Value  int    `json:"value"`
		Worst  int    `json:"worst"`
		Thresh int    `json:"thresh"`
		Raw    struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		} `json:"raw"`
	}{
		{ID: 5, Name: "Reallocated_Sector_Ct", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 3}},
		{ID: 9, Name: "Power_On_Hours", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 12345}},
		{ID: 12, Name: "Power_Cycle_Count", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 42}},
		{ID: 197, Name: "Current_Pending_Sector", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 0}},
		{ID: 198, Name: "Offline_Uncorrectable", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 1}},
		{ID: 199, Name: "UDMA_CRC_Error_Count", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 7}},
	}

	attrs := parseSMARTAttributes(data, "sata")
	if attrs == nil {
		t.Fatal("expected non-nil attributes for SATA disk with data")
	}
	if attrs.PowerOnHours == nil || *attrs.PowerOnHours != 12345 {
		t.Errorf("PowerOnHours: got %v, want 12345", attrs.PowerOnHours)
	}
	if attrs.PowerCycles == nil || *attrs.PowerCycles != 42 {
		t.Errorf("PowerCycles: got %v, want 42", attrs.PowerCycles)
	}
	if attrs.ReallocatedSectors == nil || *attrs.ReallocatedSectors != 3 {
		t.Errorf("ReallocatedSectors: got %v, want 3", attrs.ReallocatedSectors)
	}
	if attrs.PendingSectors == nil || *attrs.PendingSectors != 0 {
		t.Errorf("PendingSectors: got %v, want 0", attrs.PendingSectors)
	}
	if attrs.OfflineUncorrectable == nil || *attrs.OfflineUncorrectable != 1 {
		t.Errorf("OfflineUncorrectable: got %v, want 1", attrs.OfflineUncorrectable)
	}
	if attrs.UDMACRCErrors == nil || *attrs.UDMACRCErrors != 7 {
		t.Errorf("UDMACRCErrors: got %v, want 7", attrs.UDMACRCErrors)
	}
	// NVMe fields should be nil
	if attrs.PercentageUsed != nil {
		t.Errorf("PercentageUsed should be nil for SATA disk")
	}
	if attrs.AvailableSpare != nil {
		t.Errorf("AvailableSpare should be nil for SATA disk")
	}
}

func TestParseSMARTAttributes_NVMe(t *testing.T) {
	data := &smartctlJSON{}
	data.NVMeSmartHealthInformationLog.PowerOnHours = 5000
	data.NVMeSmartHealthInformationLog.PowerCycles = 100
	data.NVMeSmartHealthInformationLog.PercentageUsed = 5
	data.NVMeSmartHealthInformationLog.AvailableSpare = 100
	data.NVMeSmartHealthInformationLog.MediaErrors = 0
	data.NVMeSmartHealthInformationLog.UnsafeShutdowns = 12

	attrs := parseSMARTAttributes(data, "nvme")
	if attrs == nil {
		t.Fatal("expected non-nil attributes for NVMe disk")
	}
	if attrs.PowerOnHours == nil || *attrs.PowerOnHours != 5000 {
		t.Errorf("PowerOnHours: got %v, want 5000", attrs.PowerOnHours)
	}
	if attrs.PowerCycles == nil || *attrs.PowerCycles != 100 {
		t.Errorf("PowerCycles: got %v, want 100", attrs.PowerCycles)
	}
	if attrs.PercentageUsed == nil || *attrs.PercentageUsed != 5 {
		t.Errorf("PercentageUsed: got %v, want 5", attrs.PercentageUsed)
	}
	if attrs.AvailableSpare == nil || *attrs.AvailableSpare != 100 {
		t.Errorf("AvailableSpare: got %v, want 100", attrs.AvailableSpare)
	}
	if attrs.MediaErrors == nil || *attrs.MediaErrors != 0 {
		t.Errorf("MediaErrors: got %v, want 0", attrs.MediaErrors)
	}
	if attrs.UnsafeShutdowns == nil || *attrs.UnsafeShutdowns != 12 {
		t.Errorf("UnsafeShutdowns: got %v, want 12", attrs.UnsafeShutdowns)
	}
	// SATA fields should be nil
	if attrs.ReallocatedSectors != nil {
		t.Errorf("ReallocatedSectors should be nil for NVMe disk")
	}
}

func TestParseSMARTAttributes_PartialData(t *testing.T) {
	// SATA disk with only power-on hours — other attributes missing
	data := &smartctlJSON{}
	data.ATASmartAttributes.Table = []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Value  int    `json:"value"`
		Worst  int    `json:"worst"`
		Thresh int    `json:"thresh"`
		Raw    struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		} `json:"raw"`
	}{
		{ID: 9, Name: "Power_On_Hours", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 100}},
	}

	attrs := parseSMARTAttributes(data, "sata")
	if attrs == nil {
		t.Fatal("expected non-nil attributes")
	}
	if attrs.PowerOnHours == nil || *attrs.PowerOnHours != 100 {
		t.Errorf("PowerOnHours: got %v, want 100", attrs.PowerOnHours)
	}
	if attrs.ReallocatedSectors != nil {
		t.Errorf("ReallocatedSectors should be nil when not in table")
	}
	if attrs.PendingSectors != nil {
		t.Errorf("PendingSectors should be nil when not in table")
	}
}

func TestParseSMARTAttributes_Standby(t *testing.T) {
	// No data at all — simulates standby or no SMART support
	data := &smartctlJSON{}
	attrs := parseSMARTAttributes(data, "sata")
	if attrs != nil {
		t.Errorf("expected nil attributes for empty data, got %+v", attrs)
	}

	attrs = parseSMARTAttributes(data, "nvme")
	if attrs != nil {
		t.Errorf("expected nil attributes for empty NVMe data, got %+v", attrs)
	}
}

func TestParseSMARTOutputStandbyPowerMode(t *testing.T) {
	payload := smartctlJSON{
		ModelName:    "WDC WD40EFRX",
		SerialNumber: "WD-123",
		PowerMode:    "STANDBY",
	}
	payload.Device.Protocol = "ATA"

	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := parseSMARTOutput(out, smartctlTarget{Path: "/dev/ada0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.Standby {
		t.Fatalf("expected standby result, got %#v", result)
	}
	if result.Model != "WDC WD40EFRX" || result.Serial != "WD-123" {
		t.Fatalf("expected model/serial to be preserved, got %#v", result)
	}
}

func TestParseSMARTOutputUsesATASCTTemperature(t *testing.T) {
	payload := smartctlJSON{
		ModelName:    "WDC WD40EFRX",
		SerialNumber: "WD-456",
	}
	payload.Device.Protocol = "ATA"
	payload.SmartStatus = &struct {
		Passed bool `json:"passed"`
	}{Passed: true}
	payload.ATASCTStatus.Current.Value = 41

	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := parseSMARTOutput(out, smartctlTarget{Path: "/dev/ada0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Temperature != 41 {
		t.Fatalf("expected SCT temperature 41, got %#v", result)
	}
}

func TestParseSMARTOutputFallsBackToOriginalTextTemperature(t *testing.T) {
	payload := smartctlJSON{
		ModelName:    "WDC WD40EFRX",
		SerialNumber: "WD-789",
	}
	payload.Device.Protocol = "ATA"
	payload.SmartStatus = &struct {
		Passed bool `json:"passed"`
	}{Passed: true}
	payload.Smartctl.Output = []string{
		"=== START OF READ SMART DATA SECTION ===",
		"194 Temperature_Celsius     0x0022   120   099   000    Old_age   Always       -       37 (Min/Max 20/45)",
	}

	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := parseSMARTOutput(out, smartctlTarget{Path: "/dev/ada0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Temperature != 37 {
		t.Fatalf("expected text-fallback temperature 37, got %#v", result)
	}
	if result.Model != "WDC WD40EFRX" || result.Serial != "WD-789" || result.Health != "PASSED" {
		t.Fatalf("expected model/serial/health to be preserved, got %#v", result)
	}
}

func TestParseSMARTOutputFallsBackToPlainTextOutput(t *testing.T) {
	output := []byte(`
=== START OF INFORMATION SECTION ===
Device Model:     WDC WD40EFRX
Serial Number:    WD-123
SMART overall-health self-assessment test result: PASSED
Current Temperature:                    39 C
`)

	result, err := parseSMARTOutput(output, smartctlTarget{Path: "/dev/ada0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Model != "WDC WD40EFRX" || result.Serial != "WD-123" {
		t.Fatalf("expected text model/serial, got %#v", result)
	}
	if result.Health != "PASSED" || result.Temperature != 39 {
		t.Fatalf("expected text-fallback health/temp, got %#v", result)
	}
	if result.Type != "sata" {
		t.Fatalf("expected sata fallback type, got %#v", result)
	}
}

func TestParseSMARTOutputFallsBackToPlainTextATAAttributes(t *testing.T) {
	output := []byte(`
=== START OF INFORMATION SECTION ===
Device Model:     WDC WD80EFAX
Serial Number:    WD-ATA-123
SMART overall-health self-assessment test result: PASSED
ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE
  5 Reallocated_Sector_Ct   0x0033   100   100   010    Pre-fail  Always       -       2
  9 Power_On_Hours          0x0032   099   099   000    Old_age   Always       -       16,951 (223 173 0)
 12 Power_Cycle_Count       0x0032   100   100   000    Old_age   Always       -       94
197 Current_Pending_Sector  0x0012   100   100   000    Old_age   Always       -       3
198 Offline_Uncorrectable   0x0010   100   100   000    Old_age   Offline      -       1
199 UDMA_CRC_Error_Count    0x003e   200   200   000    Old_age   Always       -       7
`)

	result, err := parseSMARTOutput(output, smartctlTarget{Path: "/dev/sda"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Attributes == nil {
		t.Fatalf("expected text SMART attributes, got %#v", result)
	}
	attrs := result.Attributes
	if attrs.ReallocatedSectors == nil || *attrs.ReallocatedSectors != 2 {
		t.Fatalf("expected reallocated sectors 2, got %#v", attrs.ReallocatedSectors)
	}
	if attrs.PowerOnHours == nil || *attrs.PowerOnHours != 16951 {
		t.Fatalf("expected power-on hours 16951, got %#v", attrs.PowerOnHours)
	}
	if attrs.PowerCycles == nil || *attrs.PowerCycles != 94 {
		t.Fatalf("expected power cycles 94, got %#v", attrs.PowerCycles)
	}
	if attrs.PendingSectors == nil || *attrs.PendingSectors != 3 {
		t.Fatalf("expected pending sectors 3, got %#v", attrs.PendingSectors)
	}
	if attrs.OfflineUncorrectable == nil || *attrs.OfflineUncorrectable != 1 {
		t.Fatalf("expected offline uncorrectable 1, got %#v", attrs.OfflineUncorrectable)
	}
	if attrs.UDMACRCErrors == nil || *attrs.UDMACRCErrors != 7 {
		t.Fatalf("expected CRC errors 7, got %#v", attrs.UDMACRCErrors)
	}
}

func TestParseSMARTOutputFallsBackToPlainTextNVMeAttributes(t *testing.T) {
	output := []byte(`
=== START OF INFORMATION SECTION ===
Model Number:                       Samsung SSD 970 EVO Plus 1TB
Serial Number:                      NVME-123
Transport protocol:                 NVMe
SMART overall-health self-assessment test result: PASSED
SMART/Health Information (NVMe Log 0x02)
Percentage Used:                    97%
Available Spare:                    19%
Power Cycles:                       42
Power On Hours:                     5,000
Unsafe Shutdowns:                   12
Media and Data Integrity Errors:    3
`)

	result, err := parseSMARTOutput(output, smartctlTarget{Path: "/dev/nvme0n1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Attributes == nil {
		t.Fatalf("expected NVMe text SMART attributes, got %#v", result)
	}
	if result.Type != "nvme" {
		t.Fatalf("expected nvme type, got %#v", result)
	}
	attrs := result.Attributes
	if attrs.PercentageUsed == nil || *attrs.PercentageUsed != 97 {
		t.Fatalf("expected percentage used 97, got %#v", attrs.PercentageUsed)
	}
	if attrs.AvailableSpare == nil || *attrs.AvailableSpare != 19 {
		t.Fatalf("expected available spare 19, got %#v", attrs.AvailableSpare)
	}
	if attrs.PowerCycles == nil || *attrs.PowerCycles != 42 {
		t.Fatalf("expected power cycles 42, got %#v", attrs.PowerCycles)
	}
	if attrs.PowerOnHours == nil || *attrs.PowerOnHours != 5000 {
		t.Fatalf("expected power-on hours 5000, got %#v", attrs.PowerOnHours)
	}
	if attrs.UnsafeShutdowns == nil || *attrs.UnsafeShutdowns != 12 {
		t.Fatalf("expected unsafe shutdowns 12, got %#v", attrs.UnsafeShutdowns)
	}
	if attrs.MediaErrors == nil || *attrs.MediaErrors != 3 {
		t.Fatalf("expected media errors 3, got %#v", attrs.MediaErrors)
	}
}

func TestParseSMARTOutputMergesTextAttributesFromJSONOutput(t *testing.T) {
	payload := smartctlJSON{
		ModelName:    "WDC WD80EFAX",
		SerialNumber: "WD-JSON-TEXT",
	}
	payload.Device.Protocol = "ATA"
	payload.SmartStatus = &struct {
		Passed bool `json:"passed"`
	}{Passed: true}
	payload.ATASmartAttributes.Table = []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Value  int    `json:"value"`
		Worst  int    `json:"worst"`
		Thresh int    `json:"thresh"`
		Raw    struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		} `json:"raw"`
	}{
		{ID: 9, Name: "Power_On_Hours", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 100}},
	}
	payload.Smartctl.Output = []string{
		"ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE",
		"197 Current_Pending_Sector  0x0012   100   100   000    Old_age   Always       -       4",
	}

	output, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := parseSMARTOutput(output, smartctlTarget{Path: "/dev/sda"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Attributes == nil {
		t.Fatalf("expected merged SMART attributes, got %#v", result)
	}
	if result.Attributes.PowerOnHours == nil || *result.Attributes.PowerOnHours != 100 {
		t.Fatalf("expected JSON power-on hours 100, got %#v", result.Attributes.PowerOnHours)
	}
	if result.Attributes.PendingSectors == nil || *result.Attributes.PendingSectors != 4 {
		t.Fatalf("expected text pending sectors 4, got %#v", result.Attributes.PendingSectors)
	}
}

func TestParseSMARTOutputFallsBackToCurrentDriveTemperatureText(t *testing.T) {
	payload := smartctlJSON{
		ModelName:    "SEAGATE EXOS",
		SerialNumber: "SG-123",
	}
	payload.Device.Protocol = "SCSI"
	payload.SmartStatus = &struct {
		Passed bool `json:"passed"`
	}{Passed: true}
	payload.Smartctl.Output = []string{
		"=== START OF INFORMATION SECTION ===",
		"Current Drive Temperature: 35 C",
	}

	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := parseSMARTOutput(out, smartctlTarget{Path: "/dev/da0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Temperature != 35 {
		t.Fatalf("expected current drive temperature 35, got %#v", result)
	}
	if result.Model != "SEAGATE EXOS" || result.Serial != "SG-123" || result.Health != "PASSED" {
		t.Fatalf("expected model/serial/health to be preserved, got %#v", result)
	}
}

func TestParseRawValue(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		value    int64
		expected int64
	}{
		{"plain number", "16951", 150323855943, 16951},
		{"comma number", "16,951", 150323855943, 16951},
		{"seagate power-on hours", "16951 (223 173 0)", 150323855943, 16951},
		{"simple value matches", "12345", 12345, 12345},
		{"empty string fallback", "", 42, 42},
		{"whitespace only fallback", "   ", 42, 42},
		{"zero", "0", 999, 0},
		{"non-numeric fallback", "unknown", 77, 77},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRawValue(tt.str, tt.value)
			if got != tt.expected {
				t.Errorf("parseRawValue(%q, %d) = %d, want %d", tt.str, tt.value, got, tt.expected)
			}
		})
	}
}

func TestParseSMARTAttributes_SeagateRawValues(t *testing.T) {
	// Seagate drives pack vendor-specific data in upper bytes of the 48-bit raw
	// value, making raw.value huge. raw.string contains the correct interpretation.
	data := &smartctlJSON{}
	data.ATASmartAttributes.Table = []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Value  int    `json:"value"`
		Worst  int    `json:"worst"`
		Thresh int    `json:"thresh"`
		Raw    struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		} `json:"raw"`
	}{
		{ID: 9, Name: "Power_On_Hours", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 150323855943, String: "16951 (223 173 0)"}},
		{ID: 12, Name: "Power_Cycle_Count", Raw: struct {
			Value  int64  `json:"value"`
			String string `json:"string"`
		}{Value: 94, String: "94"}},
	}

	attrs := parseSMARTAttributes(data, "sata")
	if attrs == nil {
		t.Fatal("expected non-nil attributes")
	}
	if attrs.PowerOnHours == nil || *attrs.PowerOnHours != 16951 {
		t.Errorf("PowerOnHours: got %v, want 16951 (should parse from raw.string, not raw.value)", attrs.PowerOnHours)
	}
	if attrs.PowerCycles == nil || *attrs.PowerCycles != 94 {
		t.Errorf("PowerCycles: got %v, want 94", attrs.PowerCycles)
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
