package monitoring

import "testing"

func TestParseSensorsJSON_NoTemperatureData(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"acpitz-acpi-0": {
			"Adapter": "ACPI interface",
			"temp1": {
				"temp1_label": "temp1"
			}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if temp.Available {
		t.Fatalf("expected temperature to be unavailable when no readings are present")
	}
}

func TestParseSensorsJSON_WithCpuAndNvmeData(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"coretemp-isa-0000": {
			"Package id 0": {"temp1_input": 45.5},
			"Core 0": {"temp2_input": 43.0},
			"Core 1": {"temp3_input": 44.2}
		},
		"nvme-pci-0400": {
			"Composite": {"temp1_input": 38.75}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available when readings are present")
	}
	if temp.CPUPackage != 45.5 {
		t.Fatalf("expected cpu package temperature 45.5, got %.2f", temp.CPUPackage)
	}
	if temp.CPUMax <= 0 {
		t.Fatalf("expected cpu max temperature to be greater than zero, got %.2f", temp.CPUMax)
	}
	if len(temp.Cores) != 2 {
		t.Fatalf("expected two core temperatures, got %d", len(temp.Cores))
	}
	if len(temp.NVMe) != 1 {
		t.Fatalf("expected one NVMe temperature, got %d", len(temp.NVMe))
	}
	if temp.NVMe[0].Temp != 38.75 {
		t.Fatalf("expected NVMe temperature 38.75, got %.2f", temp.NVMe[0].Temp)
	}
}
