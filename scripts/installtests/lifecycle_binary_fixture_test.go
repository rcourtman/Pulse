package installtests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Each fixture shares only its real source build, and only within this test
// process. A caller can replace or remove its executable without affecting
// another test. Reusing Go build objects still leaves a link per invocation.
var lifecycleAgentFixture = newLifecycleBinaryFixture("pulse-agent", func(binary string) error {
	// Keep the URL and removal checks' existing three-minute build deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/pulse-agent")
	build.Dir = repoFile()
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build pulse-agent lifecycle binary: %w\n%s", errors.Join(err, ctx.Err()), output)
	}
	return nil
})

var lifecycleRunnerFixture = newLifecycleBinaryFixture("pulse-agent-runner", func(binary string) error {
	build := exec.Command("go", "build", "-o", binary, "./cmd/pulse-agent-runner")
	build.Dir = repoFile()
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build action runner: %w\n%s", err, output)
	}
	return nil
})

func buildPulseAgentRunnerLifecycleBinary(t *testing.T, dir string) string {
	t.Helper()
	binary, err := lifecycleRunnerFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	return binary
}

func newLifecycleBinaryFixture(name string, build func(string) error) func(string) (string, error) {
	load := sync.OnceValues(func() (payload []byte, resultErr error) {
		dir, err := os.MkdirTemp("", "pulse-installtest-"+name+"-")
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("clean %s build directory: %w", name, err))
			}
		}()
		binary := filepath.Join(dir, name)
		if err := build(binary); err != nil {
			return nil, err
		}
		payload, err = os.ReadFile(binary)
		if err != nil {
			return nil, err
		}
		if len(payload) == 0 {
			return nil, fmt.Errorf("%s build produced an empty executable", name)
		}
		return payload, nil
	})
	return func(dir string) (string, error) {
		payload, err := load()
		if err != nil {
			return "", err
		}
		binary := filepath.Join(dir, name)
		if err := os.WriteFile(binary, payload, 0755); err != nil {
			return "", err
		}
		return binary, nil
	}
}

func TestLifecycleBinaryFixtureBuildsOnceWithIndependentCopies(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"pulse-agent", "pulse-agent-runner"} {
		t.Run(name, func(t *testing.T) { checkLifecycleBinaryCopies(t, name) })
	}
}

func checkLifecycleBinaryCopies(t *testing.T, name string) {
	t.Helper()
	var builds atomic.Int32
	var buildDir string
	payload := []byte("fixture executable bytes")
	fixture := newLifecycleBinaryFixture(name, func(binary string) error {
		builds.Add(1)
		buildDir = filepath.Dir(binary)
		return os.WriteFile(binary, payload, 0755)
	})
	const callers = 8
	dirs := make([]string, callers)
	paths := make([]string, callers)
	errs := make([]error, callers)
	for i := range dirs {
		dirs[i] = t.TempDir()
	}
	var wg sync.WaitGroup
	for i := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			paths[i], errs[i] = fixture(dirs[i])
		}()
	}
	wg.Wait()
	if builds.Load() != 1 {
		t.Fatalf("builds = %d, want 1", builds.Load())
	}
	if _, err := os.Stat(buildDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary build directory still exists: %v", err)
	}
	for i := range paths {
		if errs[i] != nil || paths[i] != filepath.Join(dirs[i], name) {
			t.Fatalf("copy %d: path=%q err=%v", i, paths[i], errs[i])
		}
		if info, err := os.Stat(paths[i]); err != nil || info.Mode().Perm()&0100 == 0 {
			t.Fatalf("copy %d is not executable: %v", i, err)
		}
	}
	if err := os.WriteFile(paths[0], []byte("replaced by one test"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths[1]); err != nil {
		t.Fatal(err)
	}
	for i := 2; i < callers; i++ {
		got, err := os.ReadFile(paths[i])
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("copy %d changed with another caller: bytes=%q err=%v", i, got, err)
		}
	}
	// A new caller must also get the original, not the mutated first copy.
	path, err := fixture(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("later copy: bytes=%q err=%v", got, err)
	}
}

func TestLifecycleBinaryFixtureKeepsBuildFailures(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("compiler failed after partial output")
	builds := 0
	var buildDir string
	fixture := newLifecycleBinaryFixture("pulse-agent", func(binary string) error {
		builds++
		buildDir = filepath.Dir(binary)
		if err := os.WriteFile(binary, []byte("partial"), 0755); err != nil {
			return err
		}
		return wantErr
	})
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		path, err := fixture(dir)
		if path != "" || !errors.Is(err, wantErr) {
			t.Fatalf("failed build was replaced: path=%q err=%v", path, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "pulse-agent")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("partial executable reached caller: %v", err)
		}
	}
	if builds != 1 {
		t.Fatalf("failed build retried %d times", builds)
	}
	if _, err := os.Stat(buildDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed build directory still exists: %v", err)
	}
}

func TestLifecycleBinaryFixtureRejectsMissingOrEmptyOutput(t *testing.T) {
	t.Parallel()
	for _, empty := range []bool{false, true} {
		fixture := newLifecycleBinaryFixture("pulse-agent", func(binary string) error {
			if empty {
				return os.WriteFile(binary, nil, 0755)
			}
			return nil
		})
		if path, err := fixture(t.TempDir()); path != "" || err == nil {
			t.Fatalf("invalid executable accepted: empty=%t path=%q err=%v", empty, path, err)
		}
	}
}

func TestLifecycleBinaryFixtureKeepsCopyFailuresLocal(t *testing.T) {
	t.Parallel()
	builds := 0
	fixture := newLifecycleBinaryFixture("pulse-agent", func(binary string) error {
		builds++
		return os.WriteFile(binary, []byte("complete"), 0755)
	})
	if path, err := fixture(filepath.Join(t.TempDir(), "absent")); path != "" || err == nil {
		t.Fatalf("failed copy accepted: path=%q err=%v", path, err)
	}
	if path, err := fixture(t.TempDir()); path == "" || err != nil {
		t.Fatalf("one failed copy poisoned the build: path=%q err=%v", path, err)
	}
	if builds != 1 {
		t.Fatalf("copy failure retried the build %d times", builds)
	}
}
