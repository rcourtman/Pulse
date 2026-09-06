package qualification

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScratchMeasurementRejectsUnboundedOrInvalidFilesystems(t *testing.T) {
	for _, output := range []string{
		"overlayfs 4096 2048 0", "tmpfs 4096 4096 0", "tmpfs 4096 2048 2049",
		"tmpfs -1 2048 0", "tmpfs 0 2048 0", "tmpfs 4096 2048 -1",
		"tmpfs 9223372036854775807 2048 0", "tmpfs 4096 unknown 0", "tmpfs 4096 2048",
	} {
		if _, err := parseScratchAvailable(output); err == nil {
			t.Errorf("accepted unsafe filesystem measurement %q", output)
		}
	}
	for _, tc := range []struct {
		output string
		want   int64
	}{{"tmpfs 4096 2048 2048", scratchStorageBytes}, {"tmpfs 4096 2048 0", 0}, {"tmpfs 4096 2048 12", 49152}} {
		got, err := parseScratchAvailable(tc.output)
		if err != nil || got != tc.want {
			t.Errorf("measurement %q = %d, %v", tc.output, got, err)
		}
	}
}

func TestScratchProbeRequiresPreparedContainerOwnership(t *testing.T) {
	lab := &PreparedLab{RunID: "storage-ownership", ResourceIDs: map[string]string{"service": "expected-id"}}
	for _, state := range []DockerState{
		{Alias: "service", ID: "different-id", Labels: map[string]string{labRunLabel: labRunToken(lab.RunID), labAliasLabel: "service"}},
		{Alias: "service", ID: "expected-id", Labels: map[string]string{labRunLabel: "another-run", labAliasLabel: "service"}},
		{Alias: "service", ID: "expected-id", Labels: map[string]string{labRunLabel: labRunToken(lab.RunID), labAliasLabel: "control"}},
		{Alias: "unknown"},
	} {
		runner := &recordingCommandRunner{}
		driver := NewDockerLab(runner, DockerTarget{Context: "test"})
		if _, err := driver.scratchAvailable(context.Background(), lab, state); err == nil {
			t.Errorf("accepted unowned target %+v", state)
		}
		if len(runner.calls) != 0 {
			t.Errorf("issued commands before validating ownership: %v", runner.calls)
		}
	}
}

// This opt-in test verifies only independent lab ground truth. It neither
// contacts Pulse nor invokes a model and cannot qualify diagnostic outcomes.
func TestDockerStorageOracleLive(t *testing.T) {
	dockerContext := os.Getenv("PULSE_QUALIFY_ORACLE_DOCKER_CONTEXT")
	if dockerContext == "" {
		t.Skip("set PULSE_QUALIFY_ORACLE_DOCKER_CONTEXT to an explicit disposable Docker context")
	}
	manifest, err := LoadManifest(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "scenarios", "investigation.docker-storage-pressure.json"))
	if err != nil {
		t.Fatal(err)
	}
	driver := NewDockerLab(nil, DockerTarget{Context: dockerContext})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	lab, prepareErr := driver.Prepare(ctx, manifest, "storage-oracle-"+time.Now().UTC().Format("20060102t150405.000000000"))
	if lab != nil {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
			defer cleanupCancel()
			result := driver.Cleanup(cleanupCtx, manifest, lab)
			payload, _ := json.Marshal(result)
			t.Logf("cleanup=%s", payload)
			if !result.Passed || !result.SecondCleanupNoop || !result.InventoryUnchanged {
				t.Errorf("disposable inventory cleanup failed: %+v", result)
			}
		})
	}
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}
	observe := func(phase string, predicates []Predicate) {
		t.Helper()
		observations, err := driver.Observe(ctx, manifest, lab, predicates)
		payload, _ := json.Marshal(observations)
		t.Logf("%s=%s", phase, payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	observe("baseline", manifest.Baseline)
	fault := manifest.Faults[0]
	if err := driver.ApplyFault(ctx, manifest, lab, fault); err != nil {
		t.Fatal(err)
	}
	observe("fault", fault.Oracle)
	// A second write must not overwrite the existing fill file, even if called
	// outside ApplyFault's duplicate-ID guard.
	if err := driver.setScratchStorage(ctx, lab, fault.Target, true); err == nil {
		t.Fatal("scratch fill overwrote its existing file")
	}
	observe("fault-after-refused-overwrite", fault.Oracle)
	if err := driver.RevertFault(ctx, manifest, lab, fault); err != nil {
		t.Fatal(err)
	}
	observe("recovery", fault.RevertOracle)
	// Verify noclobber also protects a symlink pointing outside the scratch
	// mount. The target is a disposable control file inside this container.
	containerID := lab.ResourceIDs[fault.Target]
	if _, err := driver.docker(ctx, "exec", containerID, "/bin/sh", "-c",
		"printf preserved > /tmp/storage-oracle-control && ln -s /tmp/storage-oracle-control "+scratchStorageFile); err != nil {
		t.Fatal(err)
	}
	if err := driver.setScratchStorage(ctx, lab, fault.Target, true); err == nil {
		t.Fatal("scratch fill followed a symlink outside its mount")
	}
	if err := driver.RevertFault(ctx, manifest, lab, fault); err != nil {
		t.Fatal(err)
	}
	control, err := driver.docker(ctx, "exec", containerID, "cat", "/tmp/storage-oracle-control")
	if err != nil || control.Stdout != "preserved" {
		t.Fatalf("symlink target was modified: %q, %v", control.Stdout, err)
	}
	observe("recovery-after-refused-symlink", fault.RevertOracle)
}
