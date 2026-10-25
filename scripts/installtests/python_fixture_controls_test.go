package installtests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// A suite's cumulative runtime is not a service or installer deadline. Give
// each independent control the existing bound, while the native phase keeps
// its overall Go timeout. Default to serial execution: private files/processes
// alone do not isolate a control that reads the real machine-wide inventory.
// Discovery runs every method, including future ones, and the required list
// prevents deleting a safety control unnoticed.
func runPythonFixtureControls(t *testing.T, script, class string, required []string, bound time.Duration, env ...string) {
	t.Helper()
	runScheduledPythonFixtureControls(t, script, class, required, bound, false, env...)
}

// Opt in only classes with independent fixture state and modelled manager /
// process inventories. Go's existing -parallel limit bounds process admission.
func runParallelPythonFixtureControls(t *testing.T, script, class string, required []string, bound time.Duration, env ...string) {
	t.Helper()
	runScheduledPythonFixtureControls(t, script, class, required, bound, true, env...)
}

func runScheduledPythonFixtureControls(t *testing.T, script, class string, required []string, bound time.Duration, parallel bool, env ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	controls, err := discoverPythonFixtureControls(ctx, script, class, required, env...)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Discovered %d %s controls; each retains its %s bound", len(controls), class, bound)
	for _, name := range controls {
		t.Run(name, func(t *testing.T) {
			if parallel {
				t.Parallel()
			}
			// Start the existing per-control deadline after scheduler admission,
			// never while this method is waiting for a parallel worker.
			ctx, cancel := context.WithTimeout(context.Background(), bound)
			defer cancel()
			output, err := executePythonFixtureControl(ctx, script, class, name, env...)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s", output)
		})
	}
}

func TestPythonFixtureControlsBoundedParallelExecution(t *testing.T) {
	const probeRoot = "PULSE_TEST_PYTHON_CONTROL_PROBE_ROOT"
	if root := os.Getenv(probeRoot); root != "" {
		run := runParallelPythonFixtureControls
		if _, err := os.Stat(filepath.Join(root, "serial-helper")); err == nil {
			run = runPythonFixtureControls
		}
		run(t, filepath.Join(root, "controls.py"), "Controls",
			[]string{"test_first", "test_second", "test_third", "test_fourth"}, 15*time.Second,
			probeRoot+"="+root)
		return
	}

	// The first control must observe a second, independently running process.
	// Serial execution is an explicit adverse control, not a timing speed claim.
	// Every process records its invocation, including failed/skipped methods;
	// one adverse result must not suppress the other discovered safety controls.
	const source = `import os, pathlib, time, unittest
root = pathlib.Path(os.environ["PULSE_TEST_PYTHON_CONTROL_PROBE_ROOT"])
class Controls(unittest.TestCase):
    def control(self):
        name = self._testMethodName
        (root / ("started-" + name)).write_text("started")
        active = root / ("active-" + name)
        active.write_text("active")
        try:
            deadline = time.monotonic() + 5
            while len(list(root.glob("started-*"))) < 2:
                self.assertLess(time.monotonic(), deadline, "serial control execution")
                time.sleep(0.01)
            self.assertLessEqual(len(list(root.glob("active-*"))), 2, "parallel worker budget exceeded")
            if name == "test_first":
                fault = (root / "fault").read_text()
                if fault == "fail": self.fail("retained adverse control")
                if fault == "skip": self.skipTest("must not count as passing")
            (root / ("completed-" + name)).write_text("completed")
        finally:
            active.unlink()
    def test_first(self): self.control()
    def test_second(self): self.control()
    def test_third(self): self.control()
    def test_fourth(self): self.control()
if __name__ == "__main__": unittest.main(verbosity=2)
`
	for _, tc := range []struct {
		name, fault, parallel, diagnostic string
		passed                            bool
		serialHelper                      bool
	}{
		{"bounded parallel", "", "2", "", true, false},
		{"failed control", "fail", "2", "retained adverse control", false, false},
		{"skipped control", "skip", "2", "did not execute exactly one passing control", false, false},
		{"serial scheduler adverse control", "", "1", "serial control execution", false, false},
		{"serial helper preserves ordering", "", "2", "serial control execution", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "controls.py"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "fault"), []byte(tc.fault), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.serialHelper {
				if err := os.WriteFile(filepath.Join(root, "serial-helper"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary,
				"-test.run=^TestPythonFixtureControlsBoundedParallelExecution$",
				"-test.count=1", "-test.parallel="+tc.parallel, "-test.v")
			cmd.Env = append(os.Environ(), probeRoot+"="+root)
			output, runErr := cmd.CombinedOutput()
			if ctx.Err() != nil || (runErr == nil) != tc.passed {
				t.Fatalf("parallel fixture verdict: %v (context %v), want pass=%t\n%s", runErr, ctx.Err(), tc.passed, output)
			}
			if tc.diagnostic != "" && !strings.Contains(string(output), tc.diagnostic) {
				t.Fatalf("lost adverse control diagnostic %q:\n%s", tc.diagnostic, output)
			}
			started, err := filepath.Glob(filepath.Join(root, "started-*"))
			if err != nil || len(started) != 4 {
				t.Fatalf("not every discovered control ran: %v, %v\n%s", started, err, output)
			}
			completed, err := filepath.Glob(filepath.Join(root, "completed-*"))
			want := 3
			if tc.passed {
				want = 4
			}
			if err != nil || len(completed) != want {
				t.Fatalf("wrong completed control inventory: %v, %v; want %d\n%s", completed, err, want, output)
			}
			active, err := filepath.Glob(filepath.Join(root, "active-*"))
			if err != nil || len(active) != 0 {
				t.Fatalf("control cleanup incomplete: %v, %v", active, err)
			}
		})
	}
}

func discoverPythonFixtureControls(ctx context.Context, script, class string, required []string, env ...string) ([]string, error) {
	const discover = `import json, runpy, sys, unittest
namespace = runpy.run_path(sys.argv[1])
suite = unittest.defaultTestLoader.loadTestsFromTestCase(namespace[sys.argv[2]])
print(json.dumps([test._testMethodName for test in suite]))
`
	cmd := exec.CommandContext(ctx, "python3", "-I", "-S", "-c", discover, script, class)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("discover %s controls: %w (context %v)\n%s", class, err, ctx.Err(), output)
	}
	var controls []string
	if err := json.Unmarshal(output, &controls); err != nil {
		return nil, fmt.Errorf("decode %s controls: %w\n%s", class, err, output)
	}
	if len(controls) == 0 {
		return nil, fmt.Errorf("no controls discovered in %s", class)
	}
	for _, name := range required {
		found := false
		for _, control := range controls {
			found = found || control == name
		}
		if !found {
			return nil, fmt.Errorf("required %s control %s is absent from discovery", class, name)
		}
	}
	return controls, nil
}

func executePythonFixtureControl(ctx context.Context, script, class, name string, env ...string) ([]byte, error) {
	selector := class + "." + name
	cmd := exec.CommandContext(ctx, "python3", "-I", "-S", script, "-v", selector)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s: %w (context %v)\n%s", selector, err, ctx.Err(), output)
	}
	if !strings.Contains(string(output), name+" (__main__."+selector+") ... ok") ||
		!strings.Contains(string(output), "\nRan 1 test in ") ||
		!strings.Contains(string(output), "\nOK\n") {
		return output, fmt.Errorf("%s did not execute exactly one passing control:\n%s", selector, output)
	}
	return output, nil
}

func TestPythonFixtureControlsRetainDiscoveryAndRejectFalsePasses(t *testing.T) {
	script := filepath.Join(t.TempDir(), "controls.py")
	const source = `import unittest
class Controls(unittest.TestCase):
    def test_added(self): pass
    def test_fail(self): self.fail("retained adverse control")
    def test_pass(self): pass
    @unittest.skip("must not be credited")
    def test_skip(self): pass
if __name__ == "__main__": unittest.main(verbosity=2)
`
	if err := os.WriteFile(script, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	controls, err := discoverPythonFixtureControls(ctx, script, "Controls", []string{"test_pass"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"test_added", "test_fail", "test_pass", "test_skip"}; !reflect.DeepEqual(controls, want) {
		t.Fatalf("new controls must be discovered too: got %v, want %v", controls, want)
	}
	if _, err := discoverPythonFixtureControls(ctx, script, "Controls", []string{"test_removed"}); err == nil {
		t.Fatal("a deleted required control must fail discovery")
	}
	for _, name := range []string{"test_pass", "test_skip", "test_fail", "test_missing"} {
		t.Run(name, func(t *testing.T) {
			_, err := executePythonFixtureControl(ctx, script, "Controls", name)
			if (err == nil) != (name == "test_pass") {
				t.Fatalf("execution verdict for %s: %v", name, err)
			}
		})
	}
}
