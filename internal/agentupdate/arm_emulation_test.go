//go:build linux

package agentupdate

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The optional worker proof runs this test binary compiled with GOARM=6 under
// QEMU. Child commands explicitly use that same emulator without system binfmt
// registration, so the actual --version and --self-test execute on each CPU.
func TestARMv6ExecutableUpdateUnderEmulation(t *testing.T) {
	qemu, assets, cpu := os.Getenv("PULSE_TEST_QEMU_ARM"), os.Getenv("PULSE_TEST_ARM_ASSETS"), os.Getenv("PULSE_TEST_ARM_CPU")
	if runtime.GOARCH != "arm" || qemu == "" || assets == "" {
		t.Skip("requires worker ARMv6/QEMU proof packet")
	}
	if got := determineArch(); got != "linux-armv6" {
		t.Fatalf("GOARM=6 selected %s", got)
	}
	old, err := os.ReadFile(filepath.Join(assets, "old-armv6"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"new-armv6", "new-armv7"} {
		t.Run(name, func(t *testing.T) {
			binary, err := os.ReadFile(filepath.Join(assets, name))
			if err != nil {
				t.Fatal(err)
			}
			var requested string
			downloads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/agent/version" {
					fmt.Fprint(w, `{"version":"6.2.1"}`)
					return
				}
				requested = r.URL.Query().Get("arch")
				downloads++
				w.Header().Set(checksumSHA256Header, checksum(binary))
				w.Write(binary)
			}))
			defer server.Close()
			dir := t.TempDir()
			installed := filepath.Join(dir, "pulse-agent")
			if err := os.WriteFile(installed, old, 0700); err != nil {
				t.Fatal(err)
			}
			oldExec, oldCommand, oldRestart, oldExit := osExecutableFn, execCommandContextFn, restartProcessFn, exitFn
			t.Cleanup(func() {
				osExecutableFn, execCommandContextFn, restartProcessFn, exitFn = oldExec, oldCommand, oldRestart, oldExit
			})
			osExecutableFn = func() (string, error) { return installed, nil }
			probes := 0
			execCommandContextFn = func(ctx context.Context, path string, args ...string) *exec.Cmd {
				probes++
				return exec.CommandContext(ctx, qemu, append([]string{"-cpu", cpu, path}, args...)...)
			}
			restarts, exits := 0, 0
			restartProcessFn = func(string) error { restarts++; return nil }
			exitFn = func(int) { exits++ }
			u := New(Config{PulseURL: server.URL, AgentName: "pulse-agent", CurrentVersion: "6.1.0", StateDir: filepath.Join(dir, "state"), APIToken: "local-proof-credential"})
			u.CheckAndUpdate(context.Background())
			status := u.Snapshot()
			installedBytes, readErr := os.ReadFile(installed)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if name == "new-armv6" {
				if probes != 3 {
					t.Fatalf("expected version, release identity and self-test probes, got %d", probes)
				}
				if status.LastError != "" || status.LastSuccessAt == nil || restarts != 1 || !bytes.Equal(installedBytes, binary) {
					t.Fatalf("ARMv6 upgrade: %+v restarts=%d exits=%d", status, restarts, exits)
				}
				output, err := oldCommand(context.Background(), qemu, "-cpu", cpu, installed, "--version").CombinedOutput()
				if err != nil || string(output) != "v6.2.1\n" {
					t.Fatalf("activated executable: %v %q", err, output)
				}
				successor := New(Config{PulseURL: server.URL, AgentName: "pulse-agent", CurrentVersion: strings.TrimSpace(string(output)), StateDir: filepath.Join(dir, "state"), APIToken: "local-proof-credential"})
				successor.CheckAndUpdate(context.Background())
				server.Close()
				if downloads != 1 {
					t.Fatalf("current successor downloaded again: %d", downloads)
				}
			} else {
				server.Close()
				if probes != 0 {
					t.Fatal("ARMv7 candidate executed before target rejection")
				}
				if _, err := os.Stat(filepath.Join(dir, ".pulse-update-info")); !os.IsNotExist(err) {
					t.Fatal("refusal created executable continuity state")
				}
				if status.LastError == "" || restarts != 0 || exits != 0 || !bytes.Equal(installedBytes, old) {
					t.Fatalf("ARMv7 refusal: %+v restarts=%d exits=%d", status, restarts, exits)
				}
				if _, err := os.Stat(filepath.Join(dir, "state")); !os.IsNotExist(err) {
					t.Fatal("refusal created saved state")
				}
			}
			if requested != "linux-armv6" {
				t.Fatalf("requested %q", requested)
			}
		})
	}
}
