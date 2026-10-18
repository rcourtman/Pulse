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
// its overall Go timeout. Discovery runs every method, including future ones,
// and the required list prevents deleting a safety control unnoticed.
func runPythonFixtureControls(t *testing.T, script, class string, required []string, bound time.Duration, env ...string) {
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
