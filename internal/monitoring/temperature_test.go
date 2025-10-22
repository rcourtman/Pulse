package monitoring

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	collector := &TemperatureCollector{}
	temp, err := collector.parseRPiTemperature("48720\n")
	if err != nil {
		t.Fatalf("unexpected error parsing RPi thermal zone output: %v", err)
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be marked available")
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true for RPi thermal zone output")
	}
	expected := 48.72
	if diff := temp.CPUPackage - expected; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("expected cpu package temperature %.2f, got %.2f", expected, temp.CPUPackage)
	}
	if temp.CPUMax != temp.CPUPackage {
		t.Fatalf("expected cpu max to match package temperature %.2f, got %.2f", temp.CPUPackage, temp.CPUMax)
	}
	if temp.LastUpdate.IsZero() {
		t.Fatalf("expected LastUpdate to be set")
	}
	if elapsed := time.Since(temp.LastUpdate); elapsed > 2*time.Second {
		t.Fatalf("expected LastUpdate to be recent, got %s", elapsed)
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
	collector := &TemperatureCollector{}

	if !collector.disableLegacySSHOnAuthFailure(fmt.Errorf("ssh command failed: Permission denied (publickey)."), "node-1", "host-1") {
		t.Fatalf("expected authentication errors to disable legacy SSH")
	}
	if !collector.legacySSHDisabled.Load() {
		t.Fatalf("expected legacy SSH to be marked disabled")
	}

	// Repeated auth errors should still return true but not change the flag.
	if !collector.disableLegacySSHOnAuthFailure(fmt.Errorf("permission denied"), "node-1", "host-1") {
		t.Fatalf("expected repeated authentication errors to continue reporting disabled state")
	}

	// Non-authentication errors should not trigger disablement.
	if collector.disableLegacySSHOnAuthFailure(fmt.Errorf("connection timed out"), "node-1", "host-1") {
		t.Fatalf("expected non-authentication errors to leave legacy SSH enabled")
	}
}
