package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseLibreHardwareMonitorTemperaturesAcceptsOnlyCPUAndMotherboard(t *testing.T) {
	body := []byte(`{
		"id": 0,
		"Text": "Sensor",
		"Children": [{
			"id": 1,
			"Text": "Test computer",
			"Children": [{
				"id": 2,
				"Text": "AMD Ryzen 9",
				"HardwareId": "/amdcpu/0",
				"Children": [{
					"id": 3,
					"Text": "Temperatures",
					"Children": [
						{"id": 4, "Text": "Core (Tctl/Tdie)", "SensorId": "/amdcpu/0/temperature/0", "Type": "Temperature", "RawValue": 62.75},
						{"id": 5, "Text": "Invalid", "SensorId": "/amdcpu/0/temperature/1", "Type": "Temperature", "RawValue": 151},
						{"id": 6, "Text": "Load", "SensorId": "/amdcpu/0/load/0", "Type": "Load", "RawValue": 31}
					]
				}]
			}, {
				"id": 7,
				"Text": "Nuvoton NCT6798D",
				"HardwareId": "/lpc/nct6798d/0",
				"Children": [{
					"id": 8,
					"Text": "Motherboard",
					"SensorId": "/lpc/nct6798d/0/temperature/2",
					"Type": "Temperature",
					"RawValue": 37
				}]
			}, {
				"id": 9,
				"Text": "NVIDIA GPU",
				"HardwareId": "/gpu-nvidia/0",
				"Children": [{
					"id": 10,
					"Text": "GPU Core",
					"SensorId": "/gpu-nvidia/0/temperature/0",
					"Type": "Temperature",
					"RawValue": 55
				}]
			}, {
				"id": 11,
				"Text": "NVMe",
				"HardwareId": "/nvme/0",
				"Children": [{
					"id": 12,
					"Text": "Temperature",
					"SensorId": "/nvme/0/temperature/0",
					"Type": "Temperature",
					"RawValue": 42
				}]
			}]
		}]
	}`)

	got, err := parseLibreHardwareMonitorTemperatures(body)
	if err != nil {
		t.Fatalf("parseLibreHardwareMonitorTemperatures returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("temperatures = %+v, want exactly CPU and motherboard readings", got)
	}
	if got["cpu_lhm_amdcpu_0_temperature_0"] != 62.75 {
		t.Fatalf("CPU temperature = %+v, want 62.75", got)
	}
	if got["motherboard_lhm_lpc_nct6798d_0_temperature_2"] != 37 {
		t.Fatalf("motherboard temperature = %+v, want 37", got)
	}
}

func TestParseLibreHardwareMonitorTemperaturesRejectsMalformedAndBoundedTrees(t *testing.T) {
	if _, err := parseLibreHardwareMonitorTemperatures([]byte("{")); err == nil {
		t.Fatal("expected malformed JSON error")
	}

	root := libreHardwareMonitorNode{}
	current := &root
	for depth := 0; depth <= libreHardwareMonitorMaxDepth; depth++ {
		current.Children = []libreHardwareMonitorNode{{}}
		current = &current.Children[0]
	}
	body, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal deep tree: %v", err)
	}
	if _, err := parseLibreHardwareMonitorTemperatures(body); err == nil {
		t.Fatal("expected deep tree error")
	}

	root = libreHardwareMonitorNode{
		Children: make([]libreHardwareMonitorNode, libreHardwareMonitorMaxNodes),
	}
	body, err = json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal wide tree: %v", err)
	}
	if _, err := parseLibreHardwareMonitorTemperatures(body); err == nil {
		t.Fatal("expected oversized node tree error")
	}
}

func TestCollectLibreHardwareMonitorTemperaturesUsesBoundedNoRedirectHTTP(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/data.json", func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/json" {
			t.Fatalf("Accept header = %q, want application/json", request.Header.Get("Accept"))
		}
		_, _ = response.Write([]byte(`{
			"Children": [{
				"HardwareId": "/intelcpu/0",
				"Children": [{
					"SensorId": "/intelcpu/0/temperature/0",
					"Type": "Temperature",
					"RawValue": 48
				}]
			}]
		}`))
	})
	handler.HandleFunc("/redirect", func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/data.json", http.StatusFound)
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	got, err := collectLibreHardwareMonitorTemperatures(
		context.Background(),
		server.URL+"/data.json",
	)
	if err != nil {
		t.Fatalf("collectLibreHardwareMonitorTemperatures returned error: %v", err)
	}
	if got["cpu_lhm_intelcpu_0_temperature_0"] != 48 {
		t.Fatalf("HTTP temperatures = %+v, want CPU reading", got)
	}

	if _, err := collectLibreHardwareMonitorTemperatures(
		context.Background(),
		server.URL+"/redirect",
	); err == nil {
		t.Fatal("expected redirect rejection")
	}

	oversized := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(strings.Repeat("x", libreHardwareMonitorMaxOutputBytes+1)))
	}))
	t.Cleanup(oversized.Close)
	if _, err := collectLibreHardwareMonitorTemperatures(
		context.Background(),
		oversized.URL,
	); err == nil {
		t.Fatal("expected oversized response rejection")
	}
}
