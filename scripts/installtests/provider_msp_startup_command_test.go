package installtests

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Exercise the real command helper with separate child-process output streams.
// Compose emits unset-variable warnings on stderr even when a read succeeds.
func TestProviderMSPStartupCommandReadbackSeparatesStderr(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, stdout string
	}{
		{"container_identity", strings.Repeat("a", 64) + "\n"},
		{"mapped_port", "127.0.0.1:38443\n"},
		{"mount_digest", strings.Repeat("b", 64) + "  /run/secrets/provider_msp_license\n"},
		{"bootstrap_facts", "provider_msp_bootstrap_ok=true\naccount_id=synthetic\n"},
		{"stopped_state", "false 0\n"},
		{"no_owned_resources", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := append(os.Environ(),
				"PULSE_TEST_MSP_COMMAND_HELPER=emit",
				"PULSE_TEST_MSP_COMMAND_STDOUT="+tc.stdout,
				`PULSE_TEST_MSP_COMMAND_STDERR=time="synthetic" level=warning msg="The RESEND_API_KEY variable is not set. Defaulting to a blank string."`+"\n",
			)
			got := startupCommand(t, env, t.TempDir(), binary, "-test.run=^TestProviderMSPStartupCommandHelper$")
			if got != tc.stdout {
				t.Fatalf("machine readback contains stderr or loses stdout: got %q, want %q", got, tc.stdout)
			}
		})
	}
}

func TestProviderMSPStartupCommandFailureRetainsBothStreams(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestProviderMSPStartupCommandHelper$")
	cmd.Env = append(os.Environ(), "PULSE_TEST_MSP_COMMAND_HELPER=fail-wrapper")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("a failed command must fail acceptance")
	}
	for _, want := range []string{"command failed: exit status 23", "synthetic stdout diagnostic", "synthetic stderr diagnostic"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("failed command loses diagnostic %q: %s", want, output)
		}
	}
}

func TestProviderMSPStartupCommandHelper(t *testing.T) {
	switch os.Getenv("PULSE_TEST_MSP_COMMAND_HELPER") {
	case "":
		return
	case "emit":
		fmt.Fprint(os.Stdout, os.Getenv("PULSE_TEST_MSP_COMMAND_STDOUT"))
		fmt.Fprint(os.Stderr, os.Getenv("PULSE_TEST_MSP_COMMAND_STDERR"))
		os.Exit(0)
	case "emit-failure":
		fmt.Fprintln(os.Stdout, "synthetic stdout diagnostic")
		fmt.Fprintln(os.Stderr, "synthetic stderr diagnostic")
		os.Exit(23)
	case "fail-wrapper":
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		env := append(os.Environ(), "PULSE_TEST_MSP_COMMAND_HELPER=emit-failure")
		startupCommand(t, env, t.TempDir(), binary, "-test.run=^TestProviderMSPStartupCommandHelper$")
		t.Fatal("failed command was accepted")
	default:
		t.Fatal("unknown synthetic command helper mode")
	}
}
