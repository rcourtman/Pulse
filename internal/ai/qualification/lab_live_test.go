package qualification

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// TestFaultSetFixturesLive drives every fault set v1 fixture, injector and
// oracle against a real Docker daemon, and the host disk fault against a real
// filesystem. No Pulse request and no model turn is made.
//
// The daemon must be a private one: fixture images carry real image names.
// Set PULSE_QUALIFY_FIXTURE_DOCKER_CONTEXT to its Docker context. Set
// PULSE_QUALIFY_FIXTURE_HOST_FILESYSTEM to a dedicated, empty or seeded lab
// mount to include the host disk fault; without it that scenario is skipped.
func TestFaultSetFixturesLive(t *testing.T) {
	dockerContext := os.Getenv("PULSE_QUALIFY_FIXTURE_DOCKER_CONTEXT")
	if dockerContext == "" {
		t.Skip("set PULSE_QUALIFY_FIXTURE_DOCKER_CONTEXT to the Docker context of a private daemon")
	}
	hostFilesystem := os.Getenv("PULSE_QUALIFY_FIXTURE_HOST_FILESYSTEM")
	target := DockerTarget{Context: dockerContext}
	if hostFilesystem != "" {
		target.HostFilesystems = []string{hostFilesystem}
	}
	driver := NewDockerLab(nil, target)
	catalog, err := LoadCatalog(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	manifests := FaultSetManifests(catalog, "v1")
	setup, cancelSetup := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelSetup()
	statuses, err := driver.BuildFixtureImages(setup, manifests)
	payload, _ := json.Marshal(statuses)
	t.Logf("fixture images=%s", payload)
	if err != nil {
		t.Fatal(err)
	}
	if hostFilesystem != "" {
		seeded, err := driver.SeedHostFilesystem(setup, hostFilesystem, time.Now())
		t.Logf("host filesystem %s seeded=%t", hostFilesystem, seeded)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, manifest := range manifests {
		t.Run(manifest.ID, func(t *testing.T) {
			needsHost := false
			for i := range manifest.Resources {
				if manifest.Resources[i].Kind == resourceKindHostFilesystem {
					needsHost = true
					// The test may use any dedicated mount, not only the
					// path the checked-in scenario names.
					manifest.Resources[i].MountPath = hostFilesystem
				}
			}
			if needsHost && hostFilesystem == "" {
				t.Skip("set PULSE_QUALIFY_FIXTURE_HOST_FILESYSTEM to include the host disk fault")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			lab, prepareErr := driver.Prepare(ctx, manifest, "oracle-"+time.Now().UTC().Format("20060102t150405.000000000"))
			if lab != nil {
				t.Cleanup(func() {
					cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cleanupCancel()
					result := driver.Cleanup(cleanupCtx, manifest, lab)
					payload, _ := json.Marshal(result)
					t.Logf("cleanup=%s", payload)
					if !result.Passed || !result.SecondCleanupNoop || !result.InventoryUnchanged {
						t.Errorf("lab cleanup failed: %+v", result)
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
			// What a model can inspect must show a service, not a harness.
			for alias, name := range lab.ResourceNames {
				result, err := driver.docker(ctx, "inspect", "--format", "{{ .Name }}|{{ .Config.Image }}|{{ json .Config.Cmd }}|{{ json .Config.Entrypoint }}", name)
				if err != nil {
					t.Fatal(err)
				}
				spec, _ := manifest.resource(alias)
				fixture, _ := fixtureByKey(spec.Fixture)
				if want := "/" + spec.Name + "|" + fixture.Image + "|null|[\"/entrypoint.sh\"]"; strings.TrimSpace(result.Stdout) != want {
					t.Errorf("docker inspect of %s shows %q, want %q", alias, strings.TrimSpace(result.Stdout), want)
				}
			}
			for _, fault := range manifest.Faults {
				if err := driver.ApplyFault(ctx, manifest, lab, fault); err != nil {
					t.Fatal(err)
				}
				observe("fault", fault.Oracle)
				if err := driver.ApplyFault(ctx, manifest, lab, fault); err == nil {
					t.Fatal("duplicate fault application was accepted")
				}
				if state := lab.HostFilesystems[fault.Target]; state != nil {
					t.Logf("fill wrote %d files under %s", len(state.FaultFiles), state.FaultDir)
				}
				if manifest.Remediation != nil {
					// The fault is real only if the scenario's fix is still needed.
					if _, err := driver.Observe(ctx, manifest, lab, shortTimeouts(manifest.Remediation.Postconditions)); err == nil {
						t.Fatal("the remediation postconditions already hold while the fault is active")
					}
				}
				if err := driver.RevertFault(ctx, manifest, lab, fault); err != nil {
					t.Fatal(err)
				}
				observe("recovery", fault.RevertOracle)
			}
			observe("restored-baseline", manifest.Baseline)
		})
	}
}

func shortTimeouts(predicates []Predicate) []Predicate {
	out := make([]Predicate, len(predicates))
	for i, predicate := range predicates {
		predicate.Timeout = "3s"
		out[i] = predicate
	}
	return out
}
