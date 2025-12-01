package monitoring

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/tempproxy"
)

type stubProxyResponse struct {
	output string
	err    error
}

type stubTemperatureProxy struct {
	mu           sync.Mutex
	available    bool
	responses    []stubProxyResponse
	responseFunc func(call int) stubProxyResponse
	callCount    int
}

func (s *stubTemperatureProxy) IsAvailable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.available
}

func (s *stubTemperatureProxy) GetTemperature(host string) (string, error) {
	s.mu.Lock()
	call := s.callCount
	s.callCount++

	resp := stubProxyResponse{}
	switch {
	case call < len(s.responses):
		resp = s.responses[call]
	case s.responseFunc != nil:
		resp = s.responseFunc(call)
	case len(s.responses) > 0:
		resp = s.responses[len(s.responses)-1]
	}
	s.mu.Unlock()

	return resp.output, resp.err
}

func (s *stubTemperatureProxy) setAvailable(v bool) {
	s.mu.Lock()
	s.available = v
	s.mu.Unlock()
}

func TestParseSensorsJSON_NoTemperatureData(t *testing.T) {
	collector := &TemperatureCollector{}

	// Test with a chip that doesn't match any known CPU or NVMe patterns
	jsonStr := `{
		"unknown-sensor-0": {
			"Adapter": "Unknown interface",
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
		t.Fatalf("expected temperature to be unavailable when no CPU or NVMe chips are detected")
	}
	if temp.HasCPU {
		t.Fatalf("expected HasCPU to be false when no CPU chip detected")
	}
	if temp.HasNVMe {
		t.Fatalf("expected HasNVMe to be false when no NVMe chip detected")
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
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when CPU data present")
	}
	if !temp.HasNVMe {
		t.Fatalf("expected HasNVMe to be true when NVMe data present")
	}
}

func TestParseSensorsJSON_WithAmdTctlOnly(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"k10temp-pci-00c3": {
			"Tctl": {"temp1_input": 55.4}
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
		t.Fatalf("expected temperature to be available when Tctl reading present")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when AMD Tctl is present")
	}
	if temp.CPUPackage != 55.4 {
		t.Fatalf("expected cpu package temperature 55.4, got %.2f", temp.CPUPackage)
	}
	if temp.CPUMax != 55.4 {
		t.Fatalf("expected cpu max temperature to follow Tctl value, got %.2f", temp.CPUMax)
	}
}

func TestParseSensorsJSON_RPiWrapper(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{"rpitemp-virtual":{"temp1":{"temp1_input":47.5}}}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing wrapper output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true for wrapper output")
	}
	if temp.CPUPackage != 47.5 {
		t.Fatalf("expected cpu package temperature 47.5, got %.2f", temp.CPUPackage)
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available for wrapper output")
	}
}

func TestParseSensorsJSON_SMARTWithNullTemperature(t *testing.T) {
	collector := &TemperatureCollector{}

	lastUpdated := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	jsonStr := fmt.Sprintf(`{
		"sensors": {
			"coretemp-isa-0000": {
				"Package id 0": {"temp1_input": 55.0}
			}
		},
		"smart": [
			{
				"device": "/dev/sda",
				"serial": "S1",
				"wwn": "WWN1",
				"model": "Model1",
				"type": "sat",
				"temperature": 34,
				"lastUpdated": "%s",
				"standbySkipped": false
			},
			{
				"device": "/dev/zd0",
				"temperature": null,
				"standbySkipped": true
			}
		]
	}`, lastUpdated)

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing SMART wrapper output: %v", err)
	}

	if temp == nil || !temp.Available {
		t.Fatalf("expected temperature data to be available when SMART data present")
	}
	if !temp.HasSMART {
		t.Fatalf("expected HasSMART to be true when SMART data present")
	}
	if len(temp.SMART) != 2 {
		t.Fatalf("expected two SMART entries, got %d", len(temp.SMART))
	}
	if temp.SMART[0].Temperature != 34 {
		t.Fatalf("expected first SMART temperature 34, got %d", temp.SMART[0].Temperature)
	}
	if temp.SMART[0].LastUpdated.IsZero() {
		t.Fatalf("expected first SMART entry to include parsed lastUpdated timestamp")
	}
	if temp.SMART[1].Temperature != 0 {
		t.Fatalf("expected standby SMART entry to default to temperature 0, got %d", temp.SMART[1].Temperature)
	}
	if !temp.SMART[1].StandbySkipped {
		t.Fatalf("expected standbySkipped to be true for second SMART entry")
	}
}

func TestShouldDisableProxy(t *testing.T) {
	collector := &TemperatureCollector{}

	if !collector.shouldDisableProxy(fmt.Errorf("plain")) {
		t.Fatalf("expected plain errors to disable proxy")
	}

	transportErr := &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport}
	if !collector.shouldDisableProxy(transportErr) {
		t.Fatalf("expected transport errors to disable proxy")
	}

	sensorErr := &tempproxy.ProxyError{Type: tempproxy.ErrorTypeSensor}
	if collector.shouldDisableProxy(sensorErr) {
		t.Fatalf("sensor errors should not disable proxy")
	}
}

// TestParseSensorsJSON_NVMeOnly tests that NVMe-only systems don't show "No CPU sensor"
func TestParseSensorsJSON_NVMeOnly(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"nvme-pci-0400": {
			"Composite": {"temp1_input": 42.5}
		},
		"nvme-pci-0500": {
			"Composite": {"temp1_input": 38.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	// available should be true (any temperature data exists)
	if !temp.Available {
		t.Fatalf("expected temperature to be available when NVMe readings are present")
	}
	// hasCPU should be false (no CPU temperature data)
	if temp.HasCPU {
		t.Fatalf("expected HasCPU to be false when only NVMe data present")
	}
	// hasNVMe should be true
	if !temp.HasNVMe {
		t.Fatalf("expected HasNVMe to be true when NVMe data present")
	}
	// Verify NVMe data was parsed correctly
	if len(temp.NVMe) != 2 {
		t.Fatalf("expected two NVMe temperatures, got %d", len(temp.NVMe))
	}
	// Check that both expected temperatures are present (order may vary)
	foundTemps := make(map[float64]bool)
	for _, nvme := range temp.NVMe {
		foundTemps[nvme.Temp] = true
	}
	if !foundTemps[42.5] {
		t.Fatalf("expected to find NVMe temperature 42.5")
	}
	if !foundTemps[38.0] {
		t.Fatalf("expected to find NVMe temperature 38.0")
	}
}

// TestParseSensorsJSON_ZeroTemperature tests that HasCPU is true even when sensor reports 0°C
func TestParseSensorsJSON_ZeroTemperature(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"coretemp-isa-0000": {
			"Package id 0": {"temp1_input": 0.0},
			"Core 0": {"temp2_input": 0.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	// hasCPU should be true because coretemp chip was detected, even though values are 0
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when CPU chip is detected (even with 0°C readings)")
	}
	// available should be true because we have a CPU sensor
	if !temp.Available {
		t.Fatalf("expected temperature to be available when CPU chip is detected")
	}
	// Values should be accepted (not filtered out)
	if temp.CPUPackage != 0.0 {
		t.Fatalf("expected CPUPackage to be 0.0, got %.2f", temp.CPUPackage)
	}
	if len(temp.Cores) != 1 {
		t.Fatalf("expected one core temperature, got %d", len(temp.Cores))
	}
	if temp.Cores[0].Temp != 0.0 {
		t.Fatalf("expected core temperature to be 0.0, got %.2f", temp.Cores[0].Temp)
	}
}

func TestParseRPiTemperature(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantErr     bool
		errContains string
		wantTempC   float64
	}{
		{
			name:        "empty output returns error",
			output:      "",
			wantErr:     true,
			errContains: "empty RPi temperature output",
		},
		{
			name:        "whitespace-only output returns error",
			output:      "   \t\n  ",
			wantErr:     true,
			errContains: "empty RPi temperature output",
		},
		{
			name:        "invalid non-numeric output returns error",
			output:      "not-a-number",
			wantErr:     true,
			errContains: "failed to parse RPi temperature",
		},
		{
			name:      "valid millidegrees 45678 returns 45.678°C",
			output:    "45678",
			wantErr:   false,
			wantTempC: 45.678,
		},
		{
			name:      "valid millidegrees with whitespace returns correct temp",
			output:    "  45678\n",
			wantErr:   false,
			wantTempC: 45.678,
		},
		{
			name:      "zero value returns 0°C",
			output:    "0",
			wantErr:   false,
			wantTempC: 0.0,
		},
		{
			name:      "negative value returns negative temp",
			output:    "-5000",
			wantErr:   false,
			wantTempC: -5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &TemperatureCollector{}
			temp, err := collector.parseRPiTemperature(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if temp == nil {
				t.Fatalf("expected temperature struct, got nil")
			}
			if !temp.Available {
				t.Errorf("expected temperature to be marked available")
			}
			if !temp.HasCPU {
				t.Errorf("expected HasCPU to be true")
			}
			if diff := temp.CPUPackage - tt.wantTempC; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("expected CPUPackage %.3f, got %.3f", tt.wantTempC, temp.CPUPackage)
			}
			if temp.CPUMax != temp.CPUPackage {
				t.Errorf("expected CPUMax to equal CPUPackage (%.3f), got %.3f", temp.CPUPackage, temp.CPUMax)
			}
			if len(temp.Cores) != 0 {
				t.Errorf("expected empty Cores slice, got %d entries", len(temp.Cores))
			}
			if len(temp.NVMe) != 0 {
				t.Errorf("expected empty NVMe slice, got %d entries", len(temp.NVMe))
			}
			if temp.LastUpdate.IsZero() {
				t.Errorf("expected LastUpdate to be set")
			}
			if elapsed := time.Since(temp.LastUpdate); elapsed > 2*time.Second {
				t.Errorf("expected LastUpdate to be recent, got %s", elapsed)
			}
		})
	}
}


func TestParseSensorsJSON_PiPartialSensors(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"cpu_thermal-virtual-0": {
			"Adapter": "Virtual device",
			"temp1": {"temp1_input": 51.625}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing Pi sensors output: %v", err)
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available when cpu_thermal sensor present")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when cpu_thermal sensor present")
	}
	if temp.CPUPackage != 51.625 {
		t.Fatalf("expected cpu package temperature 51.625, got %.3f", temp.CPUPackage)
	}
	if temp.CPUMax != 51.625 {
		t.Fatalf("expected cpu max temperature 51.625, got %.3f", temp.CPUMax)
	}
	if len(temp.Cores) != 0 {
		t.Fatalf("expected no per-core temperatures, got %d entries", len(temp.Cores))
	}
}

func TestParseSensorsJSON_CoretempAndRPiFallback(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"coretemp-isa-0000": {
			"Package id 0": {"temp1_input": 65.0},
			"Core 0": {"temp2_input": 63.0},
			"Core 1": {"temp3_input": 62.5}
		},
		"cpu_thermal-virtual-0": {
			"temp1": {"temp1_input": 50.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing mixed sensors output: %v", err)
	}
	if temp.CPUPackage != 65.0 {
		t.Fatalf("expected cpu package temperature 65.0 from coretemp, got %.2f", temp.CPUPackage)
	}
	if temp.CPUMax < 63.0 {
		t.Fatalf("expected cpu max to reflect hottest core (>=63.0), got %.2f", temp.CPUMax)
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when CPU sensors present")
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available when CPU sensors present")
	}
}

func TestTemperatureCollector_DisablesProxyAfterFailures(t *testing.T) {
	stub := &stubTemperatureProxy{
		responses: []stubProxyResponse{
			{err: &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "transport failure 1"}},
			{err: &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "transport failure 2"}},
			{err: &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "transport failure 3"}},
		},
	}
	stub.setAvailable(true)

	collector := &TemperatureCollector{
		proxyClient: stub,
		useProxy:    true,
	}

	ctx := context.Background()
	for i := 0; i < proxyFailureThreshold; i++ {
		temp, err := collector.CollectTemperature(ctx, "https://node.example", "node")
		if err != nil {
			t.Fatalf("unexpected error on proxy failure %d: %v", i+1, err)
		}
		if temp.Available {
			t.Fatalf("expected temperature to be unavailable after proxy failure %d", i+1)
		}
	}

	if collector.useProxy {
		t.Fatalf("expected proxy to be disabled after %d failures", proxyFailureThreshold)
	}
	if collector.proxyFailures != 0 {
		t.Fatalf("expected proxy failure counter to reset after disable, got %d", collector.proxyFailures)
	}
	if collector.proxyCooldownUntil.IsZero() {
		t.Fatalf("expected proxy cooldown to be scheduled after disable")
	}
	if time.Until(collector.proxyCooldownUntil) <= 0 {
		t.Fatalf("expected proxy cooldown to be in the future, got %s", collector.proxyCooldownUntil)
	}
}

func TestTemperatureCollector_ProxyReenablesAfterCooldown(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(true)

	collector := &TemperatureCollector{
		proxyClient:        stub,
		useProxy:           false,
		proxyCooldownUntil: time.Now().Add(-time.Minute),
	}

	if !collector.isProxyEnabled() {
		t.Fatalf("expected proxy to re-enable when available after cooldown")
	}
	if !collector.useProxy {
		t.Fatalf("expected useProxy to be true after proxy restored")
	}
	if !collector.proxyCooldownUntil.IsZero() {
		t.Fatalf("expected cooldown to reset after proxy restoration, got %s", collector.proxyCooldownUntil)
	}
	if collector.proxyFailures != 0 {
		t.Fatalf("expected proxy failure counter to reset after restoration, got %d", collector.proxyFailures)
	}
}

func TestTemperatureCollector_ProxyCooldownExtendsWhenUnavailable(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(false)

	collector := &TemperatureCollector{
		proxyClient:        stub,
		useProxy:           false,
		proxyCooldownUntil: time.Now().Add(-time.Minute),
	}

	before := time.Now()
	if collector.isProxyEnabled() {
		t.Fatalf("expected proxy to remain disabled while unavailable")
	}
	if collector.useProxy {
		t.Fatalf("expected useProxy to remain false while proxy unavailable")
	}
	if !collector.proxyCooldownUntil.After(before) {
		t.Fatalf("expected cooldown to be pushed into the future, got %s", collector.proxyCooldownUntil)
	}
}

func TestTemperatureCollector_SuccessResetsFailureCount(t *testing.T) {
	successJSON := `{"coretemp-isa-0000":{"Package id 0":{"temp1_input": 45.0}}}`
	stub := &stubTemperatureProxy{
		responses: []stubProxyResponse{
			{err: &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "transient failure"}},
			{output: successJSON},
		},
	}
	stub.setAvailable(true)

	collector := &TemperatureCollector{
		proxyClient: stub,
		useProxy:    true,
	}

	ctx := context.Background()
	if temp, err := collector.CollectTemperature(ctx, "https://node.example", "node"); err != nil {
		t.Fatalf("unexpected error during proxy failure: %v", err)
	} else if temp.Available {
		t.Fatalf("expected unavailable temperature on proxy failure")
	}
	if collector.proxyFailures != 1 {
		t.Fatalf("expected proxy failure counter to increment to 1, got %d", collector.proxyFailures)
	}

	temp, err := collector.CollectTemperature(ctx, "https://node.example", "node")
	if err != nil {
		t.Fatalf("unexpected error on proxy success: %v", err)
	}
	if temp == nil || !temp.Available {
		t.Fatalf("expected valid temperature after proxy success")
	}
	if collector.proxyFailures != 0 {
		t.Fatalf("expected proxy failure counter reset after success, got %d", collector.proxyFailures)
	}
	if !collector.useProxy {
		t.Fatalf("expected proxy to remain enabled after success")
	}
}

func TestTemperatureCollector_ConcurrentCollectTemperature(t *testing.T) {
	successJSON := `{"coretemp-isa-0000":{"Package id 0":{"temp1_input": 55.0}}}`
	var callCounter int32
	stub := &stubTemperatureProxy{
		responseFunc: func(int) stubProxyResponse {
			n := atomic.AddInt32(&callCounter, 1)
			if n%2 == 1 {
				return stubProxyResponse{
					err: &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "transient transport error"},
				}
			}
			return stubProxyResponse{output: successJSON}
		},
	}
	stub.setAvailable(true)

	collector := &TemperatureCollector{
		proxyClient: stub,
		useProxy:    true,
	}

	const goroutines = 16
	const iterations = 32

	var wg sync.WaitGroup
	wg.Add(goroutines)

	ctx := context.Background()
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				temp, err := collector.CollectTemperature(ctx, "https://node.example", "node")
				if err != nil {
					t.Errorf("collect temperature returned error: %v", err)
					return
				}
				if temp == nil {
					t.Errorf("expected non-nil temperature result")
					return
				}
			}
		}()
	}

	wg.Wait()

	if !collector.useProxy {
		t.Fatalf("expected proxy to remain enabled during concurrent collection")
	}
	if collector.proxyFailures >= proxyFailureThreshold {
		t.Fatalf("expected proxy failures to stay below disable threshold, got %d", collector.proxyFailures)
	}
}

func TestDisableLegacySSHOnAuthFailure(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "non-auth error connection refused returns false",
			err:      fmt.Errorf("connection refused"),
			expected: false,
		},
		{
			name:     "non-auth error connection timed out returns false",
			err:      fmt.Errorf("connection timed out"),
			expected: false,
		},
		{
			name:     "non-auth error network unreachable returns false",
			err:      fmt.Errorf("network unreachable"),
			expected: false,
		},
		{
			name:     "permission denied returns true",
			err:      fmt.Errorf("permission denied"),
			expected: true,
		},
		{
			name:     "authentication failed returns true",
			err:      fmt.Errorf("authentication failed"),
			expected: true,
		},
		{
			name:     "publickey returns true",
			err:      fmt.Errorf("publickey"),
			expected: true,
		},
		{
			name:     "case insensitive PERMISSION DENIED returns true",
			err:      fmt.Errorf("PERMISSION DENIED"),
			expected: true,
		},
		{
			name:     "case insensitive Authentication Failed returns true",
			err:      fmt.Errorf("Authentication Failed"),
			expected: true,
		},
		{
			name:     "case insensitive PUBLICKEY returns true",
			err:      fmt.Errorf("PUBLICKEY"),
			expected: true,
		},
		{
			name:     "mixed case Permission Denied returns true",
			err:      fmt.Errorf("Permission Denied"),
			expected: true,
		},
		{
			name:     "embedded permission denied in message returns true",
			err:      fmt.Errorf("ssh command failed: Permission denied (publickey)."),
			expected: true,
		},
		{
			name:     "embedded authentication failed in message returns true",
			err:      fmt.Errorf("ssh: authentication failed: no supported methods remain"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &TemperatureCollector{}
			result := collector.disableLegacySSHOnAuthFailure(tt.err, "test-node", "test-host")
			if result != tt.expected {
				t.Errorf("disableLegacySSHOnAuthFailure() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestParseSensorsJSON_NCT6687SuperIO tests NCT6687 SuperIO chip detection
func TestParseSensorsJSON_NCT6687SuperIO(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"nct6687-isa-0a20": {
			"CPUTIN": {"temp1_input": 48.5},
			"SYSTIN": {"temp2_input": 35.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing NCT6687 sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available when NCT6687 CPUTIN is present")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when NCT6687 chip is detected")
	}
	if temp.CPUPackage != 48.5 {
		t.Fatalf("expected cpu package temperature 48.5 from CPUTIN, got %.2f", temp.CPUPackage)
	}
}

// TestParseSensorsJSON_AmdChipletTemps tests AMD Tccd chiplet temperature detection
func TestParseSensorsJSON_AmdChipletTemps(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"k10temp-pci-00c3": {
			"Tccd1": {"temp3_input": 62.5},
			"Tccd2": {"temp4_input": 58.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing AMD chiplet sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available when AMD chiplet temps are present")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when K10temp chip is detected")
	}
	// Should use highest chiplet temp as package temp
	if temp.CPUPackage != 62.5 {
		t.Fatalf("expected cpu package temperature to be highest chiplet temp (62.5), got %.2f", temp.CPUPackage)
	}
	// CPUMax should also be 62.5
	if temp.CPUMax != 62.5 {
		t.Fatalf("expected cpu max temperature 62.5, got %.2f", temp.CPUMax)
	}
}

// TestParseSensorsJSON_AmdTctlAndChiplets tests AMD with both Tctl and chiplet temps
func TestParseSensorsJSON_AmdTctlAndChiplets(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"k10temp-pci-00c3": {
			"Tctl": {"temp1_input": 65.0},
			"Tccd1": {"temp3_input": 62.5},
			"Tccd2": {"temp4_input": 58.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing AMD full sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true")
	}
	// Tctl should take precedence over chiplet temps for package temperature
	if temp.CPUPackage != 65.0 {
		t.Fatalf("expected cpu package temperature from Tctl (65.0), got %.2f", temp.CPUPackage)
	}
	// CPUMax should be Tctl since it's highest
	if temp.CPUMax != 65.0 {
		t.Fatalf("expected cpu max temperature 65.0, got %.2f", temp.CPUMax)
	}
}

// TestParseSensorsJSON_MultipleSuperioCPUFields tests SuperIO chips with multiple CPU temp fields
func TestParseSensorsJSON_MultipleSuperioCPUFields(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"nct6775-isa-0290": {
			"CPU Temperature": {"temp1_input": 52.0},
			"SYSTIN": {"temp2_input": 38.0},
			"AUXTIN0": {"temp3_input": 40.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing NCT6775 sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true")
	}
	if temp.CPUPackage != 52.0 {
		t.Fatalf("expected cpu package temperature from 'CPU Temperature' field (52.0), got %.2f", temp.CPUPackage)
	}
}

// =============================================================================
// Unit tests for utility functions
// =============================================================================

func TestExtractTempInput(t *testing.T) {
	tests := []struct {
		name      string
		sensorMap map[string]interface{}
		wantTemp  float64
		wantNaN   bool
	}{
		{
			name: "float64 temp1_input",
			sensorMap: map[string]interface{}{
				"temp1_input": 45.5,
			},
			wantTemp: 45.5,
		},
		{
			name: "float64 temp2_input",
			sensorMap: map[string]interface{}{
				"temp2_input": 72.3,
			},
			wantTemp: 72.3,
		},
		{
			name: "int value converted to float64",
			sensorMap: map[string]interface{}{
				"temp1_input": 55,
			},
			wantTemp: 55.0,
		},
		{
			name: "string value parseable",
			sensorMap: map[string]interface{}{
				"temp1_input": "62.5",
			},
			wantTemp: 62.5,
		},
		{
			name: "string value non-numeric",
			sensorMap: map[string]interface{}{
				"temp1_input": "N/A",
			},
			wantNaN: true,
		},
		{
			name: "no _input suffix",
			sensorMap: map[string]interface{}{
				"temp1":     45.5,
				"temp1_max": 100.0,
			},
			wantNaN: true,
		},
		{
			name:      "empty map",
			sensorMap: map[string]interface{}{},
			wantNaN:   true,
		},
		{
			name:      "nil map",
			sensorMap: nil,
			wantNaN:   true,
		},
		{
			name: "zero temperature",
			sensorMap: map[string]interface{}{
				"temp1_input": 0.0,
			},
			wantTemp: 0.0,
		},
		{
			name: "negative temperature",
			sensorMap: map[string]interface{}{
				"temp1_input": -10.5,
			},
			wantTemp: -10.5,
		},
		{
			name: "mixed valid and invalid fields",
			sensorMap: map[string]interface{}{
				"temp1":       45.0,
				"temp1_input": 50.0,
				"temp1_max":   100.0,
			},
			wantTemp: 50.0,
		},
		{
			name: "boolean value (invalid type)",
			sensorMap: map[string]interface{}{
				"temp1_input": true,
			},
			wantNaN: true,
		},
		{
			name: "nil value",
			sensorMap: map[string]interface{}{
				"temp1_input": nil,
			},
			wantNaN: true,
		},
		{
			name: "very high temperature",
			sensorMap: map[string]interface{}{
				"temp1_input": 125.5,
			},
			wantTemp: 125.5,
		},
		{
			name: "fractional precision",
			sensorMap: map[string]interface{}{
				"temp1_input": 45.123456789,
			},
			wantTemp: 45.123456789,
		},
		{
			name: "temp_crit_input also matches",
			sensorMap: map[string]interface{}{
				"temp1_crit_input": 95.0,
			},
			wantTemp: 95.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTempInput(tt.sensorMap)

			if tt.wantNaN {
				if !math.IsNaN(got) {
					t.Errorf("extractTempInput() = %v, want NaN", got)
				}
				return
			}

			if got != tt.wantTemp {
				t.Errorf("extractTempInput() = %v, want %v", got, tt.wantTemp)
			}
		})
	}
}

func TestExtractCoreNumber(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"Core 0", 0},
		{"Core 1", 1},
		{"Core 10", 10},
		{"Core 99", 99},
		{"Core 127", 127},
		{"Core", 0},           // missing number
		{"Core ", 0},          // trailing space, no number
		{"core 5", 5},         // lowercase
		{"CORE 7", 7},         // uppercase
		{"Core  12", 12},      // extra space (Fields handles this)
		{"", 0},               // empty string
		{"   ", 0},            // whitespace only
		{"Core abc", 0},       // non-numeric
		{"Package id 0", 0},   // last part is "0"
		{"temp1", 0},          // no spaces
		{"Core 1000", 1000},   // large core number
		{"Prefix Core 5", 5},  // core not at start
		{"Core 0 extra", 0},   // text after number - "extra" is last field
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCoreNumber(tt.name)
			if got != tt.want {
				t.Errorf("extractCoreNumber(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		name    string
		hostURL string
		want    string
	}{
		{
			name:    "https with port",
			hostURL: "https://192.168.1.100:8006",
			want:    "192.168.1.100",
		},
		{
			name:    "https without port",
			hostURL: "https://192.168.1.100",
			want:    "192.168.1.100",
		},
		{
			name:    "http with port",
			hostURL: "http://192.168.1.100:8006",
			want:    "192.168.1.100",
		},
		{
			name:    "http without port",
			hostURL: "http://192.168.1.100",
			want:    "192.168.1.100",
		},
		{
			name:    "hostname with port",
			hostURL: "https://proxmox.local:8006",
			want:    "proxmox.local",
		},
		{
			name:    "hostname without port",
			hostURL: "https://proxmox.local",
			want:    "proxmox.local",
		},
		{
			name:    "bare IP",
			hostURL: "192.168.1.100",
			want:    "192.168.1.100",
		},
		{
			name:    "bare IP with port",
			hostURL: "192.168.1.100:8006",
			want:    "192.168.1.100",
		},
		{
			name:    "bare hostname",
			hostURL: "proxmox.local",
			want:    "proxmox.local",
		},
		{
			name:    "bare hostname with port",
			hostURL: "proxmox.local:8006",
			want:    "proxmox.local",
		},
		{
			name:    "with path",
			hostURL: "https://192.168.1.100:8006/api2/json",
			want:    "192.168.1.100",
		},
		{
			name:    "empty string",
			hostURL: "",
			want:    "",
		},
		{
			name:    "protocol only",
			hostURL: "https://",
			want:    "",
		},
		{
			name:    "FQDN",
			hostURL: "https://pve1.example.com:8006",
			want:    "pve1.example.com",
		},
		{
			name:    "localhost",
			hostURL: "http://localhost:8006",
			want:    "localhost",
		},
		{
			name:    "127.0.0.1",
			hostURL: "https://127.0.0.1:8006",
			want:    "127.0.0.1",
		},
		{
			name:    "uppercase protocol not stripped",
			hostURL: "HTTPS://192.168.1.100:8006",
			want:    "HTTPS", // TrimPrefix is case-sensitive, so "HTTPS:" becomes hostname part
		},
		{
			name:    "trailing slash",
			hostURL: "https://192.168.1.100/",
			want:    "192.168.1.100",
		},
		{
			name:    "query string",
			hostURL: "https://192.168.1.100:8006/api?key=value",
			want:    "192.168.1.100",
		},
		{
			name:    "double protocol",
			hostURL: "https://https://192.168.1.100",
			want:    "https",
		},
		{
			name:    "port only",
			hostURL: ":8006",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHostname(tt.hostURL)
			if got != tt.want {
				t.Errorf("extractHostname(%q) = %q, want %q", tt.hostURL, got, tt.want)
			}
		})
	}
}

func TestNormalizeSMARTEntries(t *testing.T) {
	tests := []struct {
		name string
		raw  []smartEntryRaw
		want []models.DiskTemp
	}{
		{
			name: "nil input",
			raw:  nil,
			want: nil,
		},
		{
			name: "empty slice",
			raw:  []smartEntryRaw{},
			want: nil,
		},
		{
			name: "single entry with all fields",
			raw: []smartEntryRaw{
				{
					Device:         "/dev/sda",
					Serial:         "WD-WMC1T0123456",
					WWN:            "5 0014ee 2b1234567",
					Model:          "WDC WD40EFRX-68N32N0",
					Type:           "sata",
					Temperature:    intPtr(38),
					LastUpdated:    "2024-01-15T10:30:00Z",
					StandbySkipped: false,
				},
			},
			want: []models.DiskTemp{
				{
					Device:         "/dev/sda",
					Serial:         "WD-WMC1T0123456",
					WWN:            "5 0014ee 2b1234567",
					Model:          "WDC WD40EFRX-68N32N0",
					Type:           "sata",
					Temperature:    38,
					LastUpdated:    mustParseTime("2024-01-15T10:30:00Z"),
					StandbySkipped: false,
				},
			},
		},
		{
			name: "entry with nil temperature",
			raw: []smartEntryRaw{
				{
					Device:      "/dev/sdb",
					Temperature: nil,
				},
			},
			want: []models.DiskTemp{
				{
					Device:      "/dev/sdb",
					Temperature: 0, // nil becomes 0
				},
			},
		},
		{
			name: "entry with standby skipped",
			raw: []smartEntryRaw{
				{
					Device:         "/dev/sdc",
					StandbySkipped: true,
					Temperature:    nil,
				},
			},
			want: []models.DiskTemp{
				{
					Device:         "/dev/sdc",
					StandbySkipped: true,
					Temperature:    0,
				},
			},
		},
		{
			name: "empty device skipped",
			raw: []smartEntryRaw{
				{
					Device:      "",
					Temperature: intPtr(40),
				},
			},
			want: []models.DiskTemp{},
		},
		{
			name: "whitespace-only device skipped",
			raw: []smartEntryRaw{
				{
					Device:      "   ",
					Temperature: intPtr(40),
				},
			},
			want: []models.DiskTemp{},
		},
		{
			name: "invalid timestamp ignored",
			raw: []smartEntryRaw{
				{
					Device:      "/dev/sda",
					LastUpdated: "not-a-timestamp",
					Temperature: intPtr(42),
				},
			},
			want: []models.DiskTemp{
				{
					Device:      "/dev/sda",
					Temperature: 42,
					LastUpdated: time.Time{}, // zero time
				},
			},
		},
		{
			name: "empty timestamp",
			raw: []smartEntryRaw{
				{
					Device:      "/dev/sda",
					LastUpdated: "",
					Temperature: intPtr(42),
				},
			},
			want: []models.DiskTemp{
				{
					Device:      "/dev/sda",
					Temperature: 42,
					LastUpdated: time.Time{},
				},
			},
		},
		{
			name: "multiple entries",
			raw: []smartEntryRaw{
				{Device: "/dev/sda", Temperature: intPtr(38), Type: "sata"},
				{Device: "/dev/sdb", Temperature: intPtr(40), Type: "sata"},
				{Device: "/dev/nvme0n1", Temperature: intPtr(45), Type: "nvme"},
			},
			want: []models.DiskTemp{
				{Device: "/dev/sda", Temperature: 38, Type: "sata"},
				{Device: "/dev/sdb", Temperature: 40, Type: "sata"},
				{Device: "/dev/nvme0n1", Temperature: 45, Type: "nvme"},
			},
		},
		{
			name: "whitespace trimmed from fields",
			raw: []smartEntryRaw{
				{
					Device: "  /dev/sda  ",
					Serial: "  ABC123  ",
					WWN:    "  1234  ",
					Model:  "  Model X  ",
					Type:   "  sata  ",
				},
			},
			want: []models.DiskTemp{
				{
					Device: "/dev/sda",
					Serial: "ABC123",
					WWN:    "1234",
					Model:  "Model X",
					Type:   "sata",
				},
			},
		},
		{
			name: "mixed valid and empty devices",
			raw: []smartEntryRaw{
				{Device: "/dev/sda", Temperature: intPtr(38)},
				{Device: "", Temperature: intPtr(40)},
				{Device: "/dev/sdc", Temperature: intPtr(42)},
			},
			want: []models.DiskTemp{
				{Device: "/dev/sda", Temperature: 38},
				{Device: "/dev/sdc", Temperature: 42},
			},
		},
		{
			name: "zero temperature",
			raw: []smartEntryRaw{
				{Device: "/dev/sda", Temperature: intPtr(0)},
			},
			want: []models.DiskTemp{
				{Device: "/dev/sda", Temperature: 0},
			},
		},
		{
			name: "negative temperature",
			raw: []smartEntryRaw{
				{Device: "/dev/sda", Temperature: intPtr(-10)},
			},
			want: []models.DiskTemp{
				{Device: "/dev/sda", Temperature: -10},
			},
		},
		{
			name: "high temperature",
			raw: []smartEntryRaw{
				{Device: "/dev/sda", Temperature: intPtr(85)},
			},
			want: []models.DiskTemp{
				{Device: "/dev/sda", Temperature: 85},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSMARTEntries(tt.raw)

			if tt.want == nil {
				if got != nil {
					t.Errorf("normalizeSMARTEntries() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("normalizeSMARTEntries() returned %d entries, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i].Device != tt.want[i].Device {
					t.Errorf("entry[%d].Device = %q, want %q", i, got[i].Device, tt.want[i].Device)
				}
				if got[i].Serial != tt.want[i].Serial {
					t.Errorf("entry[%d].Serial = %q, want %q", i, got[i].Serial, tt.want[i].Serial)
				}
				if got[i].WWN != tt.want[i].WWN {
					t.Errorf("entry[%d].WWN = %q, want %q", i, got[i].WWN, tt.want[i].WWN)
				}
				if got[i].Model != tt.want[i].Model {
					t.Errorf("entry[%d].Model = %q, want %q", i, got[i].Model, tt.want[i].Model)
				}
				if got[i].Type != tt.want[i].Type {
					t.Errorf("entry[%d].Type = %q, want %q", i, got[i].Type, tt.want[i].Type)
				}
				if got[i].Temperature != tt.want[i].Temperature {
					t.Errorf("entry[%d].Temperature = %d, want %d", i, got[i].Temperature, tt.want[i].Temperature)
				}
				if !got[i].LastUpdated.Equal(tt.want[i].LastUpdated) {
					t.Errorf("entry[%d].LastUpdated = %v, want %v", i, got[i].LastUpdated, tt.want[i].LastUpdated)
				}
				if got[i].StandbySkipped != tt.want[i].StandbySkipped {
					t.Errorf("entry[%d].StandbySkipped = %v, want %v", i, got[i].StandbySkipped, tt.want[i].StandbySkipped)
				}
			}
		})
	}
}

// =============================================================================
// Tests for shouldSkipProxyHost
// =============================================================================

func TestShouldSkipProxyHost_EmptyHost(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: make(map[string]*proxyHostState),
	}

	if tc.shouldSkipProxyHost("") {
		t.Error("expected empty host to return false")
	}
}

func TestShouldSkipProxyHost_WhitespaceOnlyHost(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: make(map[string]*proxyHostState),
	}

	if tc.shouldSkipProxyHost("   ") {
		t.Error("expected whitespace-only host (trimmed to empty) to return false")
	}
	if tc.shouldSkipProxyHost("\t\n") {
		t.Error("expected tab/newline host (trimmed to empty) to return false")
	}
}

func TestShouldSkipProxyHost_NotInMap(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: make(map[string]*proxyHostState),
	}

	if tc.shouldSkipProxyHost("192.168.1.100") {
		t.Error("expected host not in map to return false")
	}
}

func TestShouldSkipProxyHost_NilState(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": nil,
		},
	}

	if tc.shouldSkipProxyHost("192.168.1.100") {
		t.Error("expected host with nil state to return false")
	}
}

func TestShouldSkipProxyHost_ZeroCooldownUntil(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {
				failures:      2,
				cooldownUntil: time.Time{}, // zero value
			},
		},
	}

	if tc.shouldSkipProxyHost("192.168.1.100") {
		t.Error("expected host with zero cooldownUntil to return false")
	}

	// Verify the host was cleaned up from the map
	tc.proxyMu.Lock()
	_, exists := tc.proxyHostStates["192.168.1.100"]
	tc.proxyMu.Unlock()

	if exists {
		t.Error("expected host with zero cooldownUntil to be deleted from map")
	}
}

func TestShouldSkipProxyHost_ExpiredCooldown(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {
				failures:      2,
				cooldownUntil: time.Now().Add(-time.Minute), // expired
				lastError:     "some error",
			},
		},
	}

	if tc.shouldSkipProxyHost("192.168.1.100") {
		t.Error("expected host with expired cooldown to return false")
	}

	// Verify the state was reset and host was deleted
	tc.proxyMu.Lock()
	_, exists := tc.proxyHostStates["192.168.1.100"]
	tc.proxyMu.Unlock()

	if exists {
		t.Error("expected host with expired cooldown to be deleted from map")
	}
}

func TestShouldSkipProxyHost_ActiveCooldown(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {
				failures:      0,
				cooldownUntil: time.Now().Add(5 * time.Minute), // active
				lastError:     "connection refused",
			},
		},
	}

	if !tc.shouldSkipProxyHost("192.168.1.100") {
		t.Error("expected host with active cooldown to return true")
	}

	// Verify the host is still in the map
	tc.proxyMu.Lock()
	state, exists := tc.proxyHostStates["192.168.1.100"]
	tc.proxyMu.Unlock()

	if !exists {
		t.Error("expected host with active cooldown to remain in map")
	}
	if state.lastError != "connection refused" {
		t.Errorf("expected lastError to be preserved, got %q", state.lastError)
	}
}

func TestShouldSkipProxyHost_ExpiredCooldownResetsState(t *testing.T) {
	initialState := &proxyHostState{
		failures:      5,
		cooldownUntil: time.Now().Add(-time.Second), // just expired
		lastError:     "previous error",
	}
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": initialState,
		},
	}

	// This call should reset the state and delete the host
	result := tc.shouldSkipProxyHost("192.168.1.100")
	if result {
		t.Error("expected expired cooldown to return false")
	}

	// After the call, the host should be deleted from the map
	tc.proxyMu.Lock()
	_, exists := tc.proxyHostStates["192.168.1.100"]
	tc.proxyMu.Unlock()

	if exists {
		t.Error("expected host to be deleted after cooldown expired")
	}
}

func TestShouldSkipProxyHost_TrimsWhitespace(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {
				cooldownUntil: time.Now().Add(5 * time.Minute),
			},
		},
	}

	// Host with leading/trailing whitespace should match after trimming
	if !tc.shouldSkipProxyHost("  192.168.1.100  ") {
		t.Error("expected trimmed host to match entry in map")
	}
}

// =============================================================================
// Tests for handleProxySuccess
// =============================================================================

func TestHandleProxySuccess_NilProxyClient(t *testing.T) {
	tc := &TemperatureCollector{
		proxyClient:   nil,
		proxyFailures: 5, // should remain unchanged
	}

	tc.handleProxySuccess()

	if tc.proxyFailures != 5 {
		t.Errorf("expected proxyFailures to remain 5 when proxyClient is nil, got %d", tc.proxyFailures)
	}
}

func TestHandleProxySuccess_ResetsFailures(t *testing.T) {
	stub := &stubTemperatureProxy{}

	tc := &TemperatureCollector{
		proxyClient:   stub,
		proxyFailures: 3,
	}

	tc.handleProxySuccess()

	if tc.proxyFailures != 0 {
		t.Errorf("expected proxyFailures to be reset to 0, got %d", tc.proxyFailures)
	}
}

func TestHandleProxySuccess_AlreadyZeroFailures(t *testing.T) {
	stub := &stubTemperatureProxy{}

	tc := &TemperatureCollector{
		proxyClient:   stub,
		proxyFailures: 0,
	}

	tc.handleProxySuccess()

	if tc.proxyFailures != 0 {
		t.Errorf("expected proxyFailures to remain 0, got %d", tc.proxyFailures)
	}
}

// =============================================================================
// Tests for handleProxyHostSuccess
// =============================================================================

func TestHandleProxyHostSuccess_EmptyHost(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {failures: 2, cooldownUntil: time.Now().Add(time.Minute)},
		},
	}

	tc.handleProxyHostSuccess("")

	// Map should be unchanged
	tc.proxyMu.Lock()
	if len(tc.proxyHostStates) != 1 {
		t.Errorf("expected map to have 1 entry, got %d", len(tc.proxyHostStates))
	}
	tc.proxyMu.Unlock()
}

func TestHandleProxyHostSuccess_WhitespaceOnlyHost(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {failures: 2, cooldownUntil: time.Now().Add(time.Minute)},
		},
	}

	tc.handleProxyHostSuccess("   ")

	// Map should be unchanged
	tc.proxyMu.Lock()
	if len(tc.proxyHostStates) != 1 {
		t.Errorf("expected map to have 1 entry, got %d", len(tc.proxyHostStates))
	}
	tc.proxyMu.Unlock()
}

func TestHandleProxyHostSuccess_RemovesHostFromMap(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {failures: 5, cooldownUntil: time.Now().Add(time.Minute)},
			"192.168.1.101": {failures: 3, cooldownUntil: time.Now().Add(time.Minute)},
		},
	}

	tc.handleProxyHostSuccess("192.168.1.100")

	tc.proxyMu.Lock()
	defer tc.proxyMu.Unlock()

	if _, exists := tc.proxyHostStates["192.168.1.100"]; exists {
		t.Error("expected host 192.168.1.100 to be removed from map")
	}
	if _, exists := tc.proxyHostStates["192.168.1.101"]; !exists {
		t.Error("expected host 192.168.1.101 to remain in map")
	}
}

func TestHandleProxyHostSuccess_HostNotInMap(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {failures: 2},
		},
	}

	// Should not panic when host doesn't exist
	tc.handleProxyHostSuccess("192.168.1.200")

	tc.proxyMu.Lock()
	if len(tc.proxyHostStates) != 1 {
		t.Errorf("expected map to have 1 entry, got %d", len(tc.proxyHostStates))
	}
	tc.proxyMu.Unlock()
}

func TestHandleProxyHostSuccess_TrimsWhitespace(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: map[string]*proxyHostState{
			"192.168.1.100": {failures: 5, cooldownUntil: time.Now().Add(time.Minute)},
		},
	}

	tc.handleProxyHostSuccess("  192.168.1.100  ")

	tc.proxyMu.Lock()
	defer tc.proxyMu.Unlock()

	if _, exists := tc.proxyHostStates["192.168.1.100"]; exists {
		t.Error("expected host to be removed after trimming whitespace from input")
	}
}

func TestHandleProxyHostSuccess_NilMap(t *testing.T) {
	tc := &TemperatureCollector{
		proxyHostStates: nil,
	}

	// Should not panic with nil map
	tc.handleProxyHostSuccess("192.168.1.100")
}

// =============================================================================
// Tests for parseNVMeTemps
// =============================================================================

func TestParseNVMeTemps(t *testing.T) {
	tests := []struct {
		name       string
		chipName   string
		chipMap    map[string]interface{}
		wantNVMe   []models.NVMeTemp
		wantDevice string
	}{
		{
			name:     "empty chipMap does nothing",
			chipName: "nvme-pci-0400",
			chipMap:  map[string]interface{}{},
			wantNVMe: nil,
		},
		{
			name:     "sensorData is not a map (skipped)",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": "not a map",
				"Sensor 1":  12345,
			},
			wantNVMe: nil,
		},
		{
			name:     "sensor name doesn't contain Composite or Sensor 1 (skipped)",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Temperature": map[string]interface{}{
					"temp1_input": 42.5,
				},
				"Sensor 2": map[string]interface{}{
					"temp1_input": 38.0,
				},
			},
			wantNVMe: nil,
		},
		{
			name:     "Composite with valid temp_input is added",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp1_input": 42.5,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0400", Temp: 42.5},
			},
		},
		{
			name:     "Sensor 1 with valid temp_input is added",
			chipName: "nvme-pci-0500",
			chipMap: map[string]interface{}{
				"Sensor 1": map[string]interface{}{
					"temp1_input": 38.75,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0500", Temp: 38.75},
			},
		},
		{
			name:     "invalid/NaN temp value is skipped",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp1_input": "not-a-number",
				},
			},
			wantNVMe: nil,
		},
		{
			name:     "zero temp value is skipped",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp1_input": 0.0,
				},
			},
			wantNVMe: nil,
		},
		{
			name:     "negative temp value is skipped",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp1_input": -5.0,
				},
			},
			wantNVMe: nil,
		},
		{
			name:     "device name extraction from chip name works correctly",
			chipName: "nvme-pci-0100",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp1_input": 35.0,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0100", Temp: 35.0},
			},
		},
		{
			name:     "only first valid sensor is used (Composite before Sensor 1)",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp1_input": 40.0,
				},
				"Sensor 1": map[string]interface{}{
					"temp1_input": 45.0,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0400", Temp: 40.0},
			},
		},
		{
			name:     "Composite substring match (e.g., 'Composite temp')",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite temp": map[string]interface{}{
					"temp1_input": 41.0,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0400", Temp: 41.0},
			},
		},
		{
			name:     "Sensor 1 substring match (e.g., 'Sensor 1 temp')",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Sensor 1 temp": map[string]interface{}{
					"temp1_input": 39.0,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0400", Temp: 39.0},
			},
		},
		{
			name:     "nil sensorData is skipped",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": nil,
			},
			wantNVMe: nil,
		},
		{
			name:     "temp2_input also works",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp2_input": 44.0,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0400", Temp: 44.0},
			},
		},
		{
			name:     "skips invalid Composite and uses valid Sensor 1",
			chipName: "nvme-pci-0400",
			chipMap: map[string]interface{}{
				"Composite": map[string]interface{}{
					"temp1_input": 0.0, // invalid (zero)
				},
				"Sensor 1": map[string]interface{}{
					"temp1_input": 42.0,
				},
			},
			wantNVMe: []models.NVMeTemp{
				{Device: "nvme0400", Temp: 42.0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &TemperatureCollector{}
			temp := &models.Temperature{}

			collector.parseNVMeTemps(tt.chipName, tt.chipMap, temp)

			if tt.wantNVMe == nil {
				if len(temp.NVMe) != 0 {
					t.Errorf("parseNVMeTemps() added %d NVMe entries, want 0", len(temp.NVMe))
				}
				return
			}

			if len(temp.NVMe) != len(tt.wantNVMe) {
				t.Fatalf("parseNVMeTemps() added %d NVMe entries, want %d", len(temp.NVMe), len(tt.wantNVMe))
			}

			for i, want := range tt.wantNVMe {
				got := temp.NVMe[i]
				if got.Device != want.Device {
					t.Errorf("NVMe[%d].Device = %q, want %q", i, got.Device, want.Device)
				}
				if got.Temp != want.Temp {
					t.Errorf("NVMe[%d].Temp = %v, want %v", i, got.Temp, want.Temp)
				}
			}
		})
	}
}

func TestParseNVMeTemps_AppendsToExisting(t *testing.T) {
	collector := &TemperatureCollector{}
	temp := &models.Temperature{
		NVMe: []models.NVMeTemp{
			{Device: "nvme0300", Temp: 30.0},
		},
	}

	chipMap := map[string]interface{}{
		"Composite": map[string]interface{}{
			"temp1_input": 42.5,
		},
	}

	collector.parseNVMeTemps("nvme-pci-0400", chipMap, temp)

	if len(temp.NVMe) != 2 {
		t.Fatalf("expected 2 NVMe entries after append, got %d", len(temp.NVMe))
	}

	if temp.NVMe[0].Device != "nvme0300" || temp.NVMe[0].Temp != 30.0 {
		t.Errorf("first NVMe entry was modified: got %+v", temp.NVMe[0])
	}

	if temp.NVMe[1].Device != "nvme0400" || temp.NVMe[1].Temp != 42.5 {
		t.Errorf("second NVMe entry incorrect: got %+v", temp.NVMe[1])
	}
}

// =============================================================================
// Tests for isProxyEnabled
// =============================================================================

func TestIsProxyEnabled_NilProxyClient(t *testing.T) {
	tc := &TemperatureCollector{
		proxyClient: nil,
		useProxy:    true, // even if this is true, nil client should return false
	}

	if tc.isProxyEnabled() {
		t.Error("expected isProxyEnabled() to return false when proxyClient is nil")
	}
}

func TestIsProxyEnabled_UseProxyAlreadyTrue(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(false) // shouldn't matter since useProxy is already true

	tc := &TemperatureCollector{
		proxyClient: stub,
		useProxy:    true,
	}

	if !tc.isProxyEnabled() {
		t.Error("expected isProxyEnabled() to return true when useProxy is already true")
	}

	// Verify useProxy remains true (unchanged)
	if !tc.useProxy {
		t.Error("expected useProxy to remain true")
	}
}

func TestIsProxyEnabled_UseProxyFalseStillInCooldown(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(true) // shouldn't be checked since still in cooldown

	cooldownTime := time.Now().Add(5 * time.Minute)
	tc := &TemperatureCollector{
		proxyClient:        stub,
		useProxy:           false,
		proxyCooldownUntil: cooldownTime,
		proxyFailures:      2,
	}

	if tc.isProxyEnabled() {
		t.Error("expected isProxyEnabled() to return false while in cooldown")
	}

	// Verify state is unchanged
	if tc.useProxy {
		t.Error("expected useProxy to remain false during cooldown")
	}
	if tc.proxyFailures != 2 {
		t.Errorf("expected proxyFailures to remain 2, got %d", tc.proxyFailures)
	}
	if !tc.proxyCooldownUntil.Equal(cooldownTime) {
		t.Errorf("expected proxyCooldownUntil to remain unchanged")
	}
}

func TestIsProxyEnabled_CooldownExpiredProxyAvailable(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(true)

	tc := &TemperatureCollector{
		proxyClient:        stub,
		useProxy:           false,
		proxyCooldownUntil: time.Now().Add(-time.Minute), // expired
		proxyFailures:      2,
	}

	if !tc.isProxyEnabled() {
		t.Error("expected isProxyEnabled() to return true when cooldown expired and proxy available")
	}

	// Verify state was restored
	if !tc.useProxy {
		t.Error("expected useProxy to be set to true after restoration")
	}
	if tc.proxyFailures != 0 {
		t.Errorf("expected proxyFailures to be reset to 0, got %d", tc.proxyFailures)
	}
	if !tc.proxyCooldownUntil.IsZero() {
		t.Errorf("expected proxyCooldownUntil to be zero after restoration, got %s", tc.proxyCooldownUntil)
	}
}

func TestIsProxyEnabled_CooldownExpiredProxyUnavailable(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(false)

	expiredCooldown := time.Now().Add(-time.Minute)
	tc := &TemperatureCollector{
		proxyClient:        stub,
		useProxy:           false,
		proxyCooldownUntil: expiredCooldown,
		proxyFailures:      2,
	}

	before := time.Now()
	if tc.isProxyEnabled() {
		t.Error("expected isProxyEnabled() to return false when proxy unavailable")
	}

	// Verify state
	if tc.useProxy {
		t.Error("expected useProxy to remain false when proxy unavailable")
	}
	// Cooldown should be extended by proxyRetryInterval (5 minutes)
	if !tc.proxyCooldownUntil.After(before) {
		t.Errorf("expected proxyCooldownUntil to be extended into the future, got %s", tc.proxyCooldownUntil)
	}
	expectedMinCooldown := before.Add(proxyRetryInterval - time.Second)
	if tc.proxyCooldownUntil.Before(expectedMinCooldown) {
		t.Errorf("expected proxyCooldownUntil to be at least %s, got %s", expectedMinCooldown, tc.proxyCooldownUntil)
	}
}

func TestIsProxyEnabled_ZeroCooldownProxyAvailable(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(true)

	tc := &TemperatureCollector{
		proxyClient:        stub,
		useProxy:           false,
		proxyCooldownUntil: time.Time{}, // zero value - time.Now().After(zero) is true
		proxyFailures:      0,
	}

	if !tc.isProxyEnabled() {
		t.Error("expected isProxyEnabled() to return true with zero cooldown and available proxy")
	}

	if !tc.useProxy {
		t.Error("expected useProxy to be set to true")
	}
}

func TestIsProxyEnabled_ZeroCooldownProxyUnavailable(t *testing.T) {
	stub := &stubTemperatureProxy{}
	stub.setAvailable(false)

	tc := &TemperatureCollector{
		proxyClient:        stub,
		useProxy:           false,
		proxyCooldownUntil: time.Time{}, // zero value
	}

	before := time.Now()
	if tc.isProxyEnabled() {
		t.Error("expected isProxyEnabled() to return false when proxy unavailable")
	}

	// Should set a new cooldown since proxy is unavailable
	if !tc.proxyCooldownUntil.After(before) {
		t.Errorf("expected proxyCooldownUntil to be set in the future, got %s", tc.proxyCooldownUntil)
	}
}

// =============================================================================
// Tests for handleProxyFailure
// =============================================================================

func TestHandleProxyFailure_NilProxyClient(t *testing.T) {
	tc := &TemperatureCollector{
		proxyClient:     nil,
		proxyFailures:   2,
		proxyHostStates: make(map[string]*proxyHostState),
	}

	// Should return early without panic
	tc.handleProxyFailure("192.168.1.100", fmt.Errorf("some error"))

	// State should remain unchanged
	if tc.proxyFailures != 2 {
		t.Errorf("expected proxyFailures to remain 2, got %d", tc.proxyFailures)
	}
	if len(tc.proxyHostStates) != 0 {
		t.Errorf("expected proxyHostStates to remain empty, got %d entries", len(tc.proxyHostStates))
	}
}

func TestHandleProxyFailure_ShouldDisableProxyFalse_CallsHandleProxyHostFailure(t *testing.T) {
	stub := &stubTemperatureProxy{}

	tc := &TemperatureCollector{
		proxyClient:     stub,
		proxyFailures:   0,
		useProxy:        true,
		proxyHostStates: make(map[string]*proxyHostState),
	}

	// ErrorTypeSensor does NOT trigger shouldDisableProxy (returns false)
	sensorErr := &tempproxy.ProxyError{Type: tempproxy.ErrorTypeSensor, Message: "sensor not found"}
	tc.handleProxyFailure("192.168.1.100", sensorErr)

	// proxyFailures should NOT be incremented (that's for global disable path)
	if tc.proxyFailures != 0 {
		t.Errorf("expected proxyFailures to remain 0, got %d", tc.proxyFailures)
	}

	// handleProxyHostFailure should have been called - check host state
	tc.proxyMu.Lock()
	state, exists := tc.proxyHostStates["192.168.1.100"]
	tc.proxyMu.Unlock()

	if !exists {
		t.Fatal("expected host to be added to proxyHostStates")
	}
	if state.failures != 1 {
		t.Errorf("expected host failures to be 1, got %d", state.failures)
	}
	if state.lastError != "sensor not found" {
		t.Errorf("expected lastError to be 'sensor not found', got %q", state.lastError)
	}
}

func TestHandleProxyFailure_ShouldDisableProxyTrue_IncrementsFailures(t *testing.T) {
	stub := &stubTemperatureProxy{}

	tc := &TemperatureCollector{
		proxyClient:     stub,
		proxyFailures:   0,
		useProxy:        true,
		proxyHostStates: make(map[string]*proxyHostState),
	}

	// ErrorTypeTransport triggers shouldDisableProxy (returns true)
	transportErr := &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "connection refused"}
	tc.handleProxyFailure("192.168.1.100", transportErr)

	if tc.proxyFailures != 1 {
		t.Errorf("expected proxyFailures to be 1, got %d", tc.proxyFailures)
	}

	// Proxy should still be enabled (threshold not reached)
	if !tc.useProxy {
		t.Error("expected useProxy to remain true (threshold not reached)")
	}

	// handleProxyHostFailure should NOT have been called
	tc.proxyMu.Lock()
	hostStateCount := len(tc.proxyHostStates)
	tc.proxyMu.Unlock()

	if hostStateCount != 0 {
		t.Errorf("expected proxyHostStates to remain empty, got %d entries", hostStateCount)
	}
}

func TestHandleProxyFailure_ReachesThreshold_DisablesProxy(t *testing.T) {
	stub := &stubTemperatureProxy{}

	tc := &TemperatureCollector{
		proxyClient:     stub,
		proxyFailures:   proxyFailureThreshold - 1, // one failure away from threshold
		useProxy:        true,
		proxyHostStates: make(map[string]*proxyHostState),
	}

	transportErr := &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "connection refused"}
	before := time.Now()
	tc.handleProxyFailure("192.168.1.100", transportErr)

	// Proxy should now be disabled
	if tc.useProxy {
		t.Error("expected useProxy to be false after reaching threshold")
	}

	// proxyFailures should be reset to 0
	if tc.proxyFailures != 0 {
		t.Errorf("expected proxyFailures to be reset to 0, got %d", tc.proxyFailures)
	}

	// proxyCooldownUntil should be set in the future
	if !tc.proxyCooldownUntil.After(before) {
		t.Errorf("expected proxyCooldownUntil to be in the future, got %s", tc.proxyCooldownUntil)
	}
}

func TestHandleProxyFailure_ReachesThreshold_UseProxyFalse_NoDoubleDisable(t *testing.T) {
	stub := &stubTemperatureProxy{}

	originalCooldown := time.Now().Add(10 * time.Minute)
	tc := &TemperatureCollector{
		proxyClient:        stub,
		proxyFailures:      proxyFailureThreshold - 1, // one failure away from threshold
		useProxy:           false,                     // already disabled
		proxyCooldownUntil: originalCooldown,
		proxyHostStates:    make(map[string]*proxyHostState),
	}

	transportErr := &tempproxy.ProxyError{Type: tempproxy.ErrorTypeTransport, Message: "connection refused"}
	tc.handleProxyFailure("192.168.1.100", transportErr)

	// proxyFailures should increment (we still track failures)
	if tc.proxyFailures != proxyFailureThreshold {
		t.Errorf("expected proxyFailures to be %d, got %d", proxyFailureThreshold, tc.proxyFailures)
	}

	// useProxy should remain false
	if tc.useProxy {
		t.Error("expected useProxy to remain false")
	}

	// proxyCooldownUntil should NOT be changed (no double-disable)
	if !tc.proxyCooldownUntil.Equal(originalCooldown) {
		t.Errorf("expected proxyCooldownUntil to remain %s, got %s", originalCooldown, tc.proxyCooldownUntil)
	}
}

func TestHandleProxyFailure_PlainError_TriggersDisablePath(t *testing.T) {
	stub := &stubTemperatureProxy{}

	tc := &TemperatureCollector{
		proxyClient:     stub,
		proxyFailures:   0,
		useProxy:        true,
		proxyHostStates: make(map[string]*proxyHostState),
	}

	// Plain errors (not ProxyError) should trigger shouldDisableProxy (returns true)
	plainErr := fmt.Errorf("connection refused")
	tc.handleProxyFailure("192.168.1.100", plainErr)

	if tc.proxyFailures != 1 {
		t.Errorf("expected proxyFailures to be 1, got %d", tc.proxyFailures)
	}

	// handleProxyHostFailure should NOT be called
	tc.proxyMu.Lock()
	hostStateCount := len(tc.proxyHostStates)
	tc.proxyMu.Unlock()

	if hostStateCount != 0 {
		t.Errorf("expected proxyHostStates to remain empty, got %d entries", hostStateCount)
	}
}

// Helper functions for test setup

func intPtr(i int) *int {
	return &i
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
