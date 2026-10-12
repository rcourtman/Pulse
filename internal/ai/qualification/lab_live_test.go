package qualification

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These opt-in cases exercise the checked-in fault and recovery contracts with
// real Docker containers. No Pulse request, approval decision or model turn is
// made. Passing them qualifies the lab oracle only, not remediation outcomes.
func TestDockerDependencyAndRestartOraclesLive(t *testing.T) {
	dockerContext := os.Getenv("PULSE_QUALIFY_ORACLE_DOCKER_CONTEXT")
	if dockerContext == "" {
		t.Skip("set PULSE_QUALIFY_ORACLE_DOCKER_CONTEXT to an explicit disposable Docker context")
	}
	for _, scenario := range []string{
		"investigation.docker-dependency",
		"remediation.docker-unhealthy-restart-approved",
		"remediation.docker-unhealthy-restart-rejected",
		"remediation.docker-unhealthy-restart-autonomous",
	} {
		t.Run(scenario, func(t *testing.T) {
			manifest, err := LoadManifest(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "scenarios", scenario+".json"))
			if err != nil {
				t.Fatal(err)
			}
			driver := NewDockerLab(nil, DockerTarget{Context: dockerContext})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			lab, prepareErr := driver.Prepare(ctx, manifest, "oracle-"+time.Now().UTC().Format("20060102t150405.000000000"))
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
			for _, fault := range manifest.Faults {
				if err := driver.ApplyFault(ctx, manifest, lab, fault); err != nil {
					t.Fatal(err)
				}
				observe("fault", fault.Oracle)
				if err := driver.ApplyFault(ctx, manifest, lab, fault); err == nil {
					t.Fatal("duplicate fault application was accepted")
				}
				// Observation and a refused duplicate injection must not repair the
				// workload. Recovery below is explicit fixture teardown, never an
				// approved action or a claim that a rejected action was executed.
				observe("fault-after-refused-duplicate", fault.Oracle)
				if err := driver.RevertFault(ctx, manifest, lab, fault); err != nil {
					t.Fatal(err)
				}
				observe("recovery", fault.RevertOracle)
			}
			observe("restored-baseline", manifest.Baseline)
		})
	}
}
