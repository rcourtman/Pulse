package monitoring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCommandRunner implements CommandRunner for testing
type mockCommandRunner struct {
	outputs       map[string]string // map command substring to output
	errs          map[string]error  // map command substring to error
	callCount     int
	sawDeadline   bool
	lastDeadline  time.Time
	lastHasDeadln bool
}

func (m *mockCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.callCount++
	if deadline, ok := ctx.Deadline(); ok {
		m.sawDeadline = true
		m.lastHasDeadln = true
		m.lastDeadline = deadline
	}

	fullCmd := name + " " + strings.Join(args, " ")

	// Check for errors first
	for k, v := range m.errs {
		if strings.Contains(fullCmd, k) {
			return nil, v
		}
	}

	// Check for outputs
	for k, v := range m.outputs {
		if strings.Contains(fullCmd, k) {
			return []byte(v), nil
		}
	}

	return nil, nil
}

func TestTemperatureCollector_Parsing(t *testing.T) {
	// Create dummy key file
	tmpKey := t.TempDir() + "/id_rsa"
	os.WriteFile(tmpKey, []byte("dummy key"), 0600)

	tc := NewTemperatureCollectorWithPort("root", tmpKey, 22)
	tc.hostKeys = nil // Disable real network calls for host key verification
	runner := &mockCommandRunner{
		outputs: make(map[string]string),
		errs:    make(map[string]error),
	}
	tc.runner = runner

	// Test case: Valid sensors JSON (CPU + NVMe)
	sensorsJSON := `{
		"coretemp-isa-0000": {
			"Package id 0": { "temp1_input": 45.5 },
			"Core 0": { "temp2_input": 42.0 },
			"Core 1": { "temp3_input": 43.0 }
		},
		"nvme-pci-0100": {
			"Composite": { "temp1_input": 38.5 }
		}
	}`

	runner.outputs["sensors -j"] = sensorsJSON

	temp, err := tc.CollectTemperature(context.Background(), "node1", "node1")
	require.NoError(t, err)
	require.NotNil(t, temp)
	assert.True(t, temp.Available)
	assert.Equal(t, 45.5, temp.CPUPackage)
	assert.Len(t, temp.Cores, 2)
	assert.Len(t, temp.NVMe, 1)
	assert.Equal(t, 38.5, temp.NVMe[0].Temp)

	// Test case: Valid RPi fallback
	runner.outputs["sensors -j"] = ""         // Empty sensors output
	runner.outputs["thermal_zone0"] = "55123" // 55.123 C

	temp2, err := tc.CollectTemperature(context.Background(), "node2", "node2")
	require.NoError(t, err)
	require.NotNil(t, temp2)
	assert.InDelta(t, 55.123, temp2.CPUPackage, 0.001)

	// Test case: Both fail
	runner = &mockCommandRunner{
		errs: map[string]error{
			"sensors -j":    errors.New("command missing"),
			"thermal_zone0": errors.New("no file"),
		},
	}
	tc.runner = runner
	temp3, err := tc.CollectTemperature(context.Background(), "node3", "node3")
	require.NoError(t, err)
	assert.False(t, temp3.Available)
}

func TestTemperatureCollector_ParseSensorsJSON_Complex(t *testing.T) {
	tc := &TemperatureCollector{}

	// AMD GPU and specific chips
	jsonStr := `{
		"amdgpu-pci-0800": {
			"edge": { "temp1_input": 50.0 },
			"junction": { "temp2_input": 65.0 },
			"mem": { "temp3_input": 55.0 }
		},
		"k10temp-pci-00c3": {
			"Tctl": { "temp1_input": 60.5 },
			"Tccd1": { "temp3_input": 58.0 }
		}
	}`

	temp, err := tc.parseSensorsJSON(jsonStr)
	require.NoError(t, err)
	assert.True(t, temp.HasGPU)
	assert.Equal(t, 50.0, temp.GPU[0].Edge)
	assert.Equal(t, 65.0, temp.GPU[0].Junction)
	assert.Equal(t, 60.5, temp.CPUPackage) // Tctl mapped to package
}

func TestExtractTempInput_StringValues(t *testing.T) {
	t.Run("parses celsius strings with suffix", func(t *testing.T) {
		got := extractTempInput(map[string]interface{}{
			"temp1_input": "+44.5°C",
		})
		assert.InDelta(t, 44.5, got, 0.0001)
	})

	t.Run("parses millidegree strings", func(t *testing.T) {
		got := extractTempInput(map[string]interface{}{
			"temp1_input": "39000",
		})
		assert.InDelta(t, 39.0, got, 0.0001)
	})
}

func TestTemperatureCollector_ParseSensorsJSON_StringTemps(t *testing.T) {
	tc := &TemperatureCollector{}
	jsonStr := `{
		"coretemp-isa-0000": {
			"Package id 0": { "temp1_input": "+47.0°C" },
			"Core 0": { "temp2_input": "45.0" }
		},
		"nvme-pci-0100": {
			"Composite": { "temp1_input": "42000" }
		}
	}`

	temp, err := tc.parseSensorsJSON(jsonStr)
	require.NoError(t, err)
	require.NotNil(t, temp)
	assert.True(t, temp.Available)
	assert.InDelta(t, 47.0, temp.CPUPackage, 0.0001)
	require.Len(t, temp.NVMe, 1)
	assert.InDelta(t, 42.0, temp.NVMe[0].Temp, 0.0001)
}

func TestTemperatureCollector_HelperMethods(t *testing.T) {
	// extractCoreNumber
	// Private methods are hard to test directly from separate package if using _test,
	// but since we are in `monitoring`, we can access if same package.
	// But usually tests are `monitoring_test` package.
	// I will assume same package for now based on file declaration.
}

func TestDefaultCommandRunner_RejectsOversizedStdout(t *testing.T) {
	runner := &defaultCommandRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("head -c %d /dev/zero", maxTemperatureCommandOutputSize+1)
	_, err := runner.Run(ctx, "sh", "-c", cmd)
	require.Error(t, err)
	require.ErrorIs(t, err, errTemperatureCommandOutputTooLarge)
}

func TestDefaultCommandRunner_RejectsOversizedStderr(t *testing.T) {
	runner := &defaultCommandRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("head -c %d /dev/zero 1>&2; exit 1", maxTemperatureCommandOutputSize+1)
	_, err := runner.Run(ctx, "sh", "-c", cmd)
	require.Error(t, err)
	require.ErrorIs(t, err, errTemperatureCommandOutputTooLarge)
}

func TestTemperatureCollector_RunSSHCommand_OversizedOutput(t *testing.T) {
	tmpKey := t.TempDir() + "/id_rsa"
	require.NoError(t, os.WriteFile(tmpKey, []byte("dummy key"), 0600))

	tc := NewTemperatureCollectorWithPort("root", tmpKey, 22)
	tc.hostKeys = nil
	runner := &mockCommandRunner{
		outputs: make(map[string]string),
		errs:    make(map[string]error),
	}

	exitErr := buildExitErrorWithStderr(t, "Permission denied (publickey) from 10.0.0.2 using /home/pulse/.ssh/id_ed25519_sensors")
	runner.errs[""] = exitErr
	tc.runner = runner

	_, err := tc.runSSHCommand(context.Background(), "node1", "sensors -j")
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "authentication failed")
	assert.NotContains(t, err.Error(), "10.0.0.2")
	assert.NotContains(t, err.Error(), "id_ed25519_sensors")
}

func TestRunSSHCommand_AppliesDefaultTimeout(t *testing.T) {
	tmpKey := t.TempDir() + "/id_rsa"
	require.NoError(t, os.WriteFile(tmpKey, []byte("dummy key"), 0600))

	tc := NewTemperatureCollectorWithPort("root", tmpKey, 22)
	tc.hostKeys = nil
	runner := &mockCommandRunner{
		outputs: map[string]string{"": "{}"},
		errs:    make(map[string]error),
	}
	tc.runner = runner

	_, err := tc.runSSHCommand(context.Background(), "node1", "sensors -j")
	require.NoError(t, err)
	require.True(t, runner.sawDeadline, "runSSHCommand should enforce a timeout when caller context has no deadline")
	require.True(t, runner.lastHasDeadln)

	timeoutRemaining := time.Until(runner.lastDeadline)
	assert.Greater(t, timeoutRemaining, 0*time.Second)
	assert.LessOrEqual(t, timeoutRemaining, defaultSSHCommandTimeout+time.Second)
}

func buildExitErrorWithStderr(t *testing.T, stderr string) error {
	t.Helper()

	cmd := exec.Command("sh", "-c", "printf '%s' \"$PULSE_TEST_STDERR\" >&2; exit 255")
	cmd.Env = append(os.Environ(), "PULSE_TEST_STDERR="+stderr)
	_, err := cmd.Output()
	require.Error(t, err)

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)

	return err
}
