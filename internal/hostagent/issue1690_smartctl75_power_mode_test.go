package hostagent

// Discussion #1690: smartmontools 7.5 started emitting power_mode as a JSON
// object whenever the -n power guard runs, which is every rotational-disk
// probe. The parser declared the field as a string, so json.Unmarshal failed
// for the entire document and healthy spinning disks behind a SAS HBA were
// reported as having no usable SMART data while guard-free SSD probes kept
// working. These tests pin the smartctl 7.5 shapes end to end.

import (
	"errors"
	"testing"
)

func TestIssue1690PowerModeObjectActiveDiskParsesFully(t *testing.T) {
	result, err := parseSMARTOutput([]byte(`{
		"device":{"name":"/dev/sda","type":"sat","protocol":"ATA"},
		"model_name":"Hitachi HUA722020ALA330",
		"serial_number":"JK11A1YAJNX41V",
		"power_mode":{"ata_value":255,"name":"ACTIVE or IDLE"},
		"smart_status":{"passed":true},
		"temperature":{"current":60},
		"ata_smart_attributes":{"table":[
			{"id":194,"raw":{"value":60,"string":"60 (Min/Max 22/68)"}}
		]}
	}`), smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	if err != nil {
		t.Fatalf("parse smartctl 7.5 output with power_mode object: %v", err)
	}
	if result.Health != "PASSED" || result.Temperature != 60 {
		t.Fatalf("active disk lost SMART data behind power_mode object: %+v", result)
	}
	if result.Standby {
		t.Fatalf("ACTIVE or IDLE misread as standby: %+v", result)
	}
	if result.Serial != "JK11A1YAJNX41V" {
		t.Fatalf("serial lost behind power_mode object: %+v", result)
	}
}

func TestIssue1690PowerModeObjectStandbyAbortReportsStandby(t *testing.T) {
	for _, name := range []string{"STANDBY", "STANDBY_Y", "SLEEP"} {
		result, err := parseSMARTOutput([]byte(`{
			"device":{"name":"/dev/sda","type":"sat","protocol":"ATA"},
			"power_mode":{"ata_value":0,"name":"`+name+`"}
		}`), smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
		if err != nil {
			t.Fatalf("power_mode %s: aborted probe dropped instead of standby: %v", name, err)
		}
		if !result.Standby {
			t.Fatalf("power_mode %s not reported as standby: %+v", name, result)
		}
	}
}

func TestIssue1690PowerModeLegacyStringStillAccepted(t *testing.T) {
	result, err := parseSMARTOutput([]byte(`{
		"device":{"name":"/dev/sda","type":"sat","protocol":"ATA"},
		"power_mode":"STANDBY"
	}`), smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	if err != nil {
		t.Fatalf("legacy string power_mode rejected: %v", err)
	}
	if !result.Standby {
		t.Fatalf("legacy string power_mode not reported as standby: %+v", result)
	}
}

func TestIssue1690PowerModeUnknownShapeCostsOnlyTheField(t *testing.T) {
	result, err := parseSMARTOutput([]byte(`{
		"device":{"name":"/dev/sda","type":"sat","protocol":"ATA"},
		"model_name":"Hitachi HUA722020ALA330",
		"serial_number":"JK11A1YAJNX41V",
		"power_mode":42,
		"smart_status":{"passed":true},
		"temperature":{"current":60}
	}`), smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	if err != nil {
		t.Fatalf("numeric power_mode failed the whole document: %v", err)
	}
	if result.Health != "PASSED" || result.Temperature != 60 || result.Standby {
		t.Fatalf("unknown power_mode shape corrupted the result: %+v", result)
	}
}

func TestIssue1690TextFallbackCatchesEPCStandbyVariants(t *testing.T) {
	for _, line := range []string{
		"Device is in STANDBY mode, exit(3)",
		"Device is in STANDBY_Y mode, exit(3)",
		"Device is in STANDBY (OS) mode, exit(3)",
		"Device is in SLEEP mode, exit(3)",
	} {
		fallback := parseSMARTTextFallback(line)
		if !fallback.Standby {
			t.Fatalf("text fallback missed standby line %q", line)
		}
	}

	if parseSMARTTextFallback("SMART overall-health self-assessment test result: PASSED").Standby {
		t.Fatal("text fallback fabricated standby from a healthy line")
	}
}

func TestIssue1690EmptyDocumentStillUnavailable(t *testing.T) {
	_, err := parseSMARTOutput([]byte(`{
		"device":{"name":"/dev/sda","type":"sat","protocol":"ATA"}
	}`), smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	if !errors.Is(err, errSMARTDataUnavailable) {
		t.Fatalf("document with no SMART evidence must stay unavailable, got %v", err)
	}
}
