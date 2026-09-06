package dockeragent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/qualification"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// This opt-in proof calls the real collector against the existing bounded lab.
// It does not enroll an agent, send reports to Pulse, or invoke a model.
func TestCollectContainerStorageFaultLive(t *testing.T) {
	dockerContext := os.Getenv("PULSE_QUALIFY_ORACLE_DOCKER_CONTEXT")
	if dockerContext == "" {
		t.Skip("set PULSE_QUALIFY_ORACLE_DOCKER_CONTEXT to an explicit disposable Docker context")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	endpoint, err := exec.CommandContext(ctx, "docker", "context", "inspect", dockerContext,
		"--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		t.Fatal(err)
	}
	host := strings.TrimSpace(string(endpoint))
	if !strings.HasPrefix(host, "unix://") {
		t.Fatal("run this proof beside the disposable Docker daemon using a Unix socket context")
	}
	moduleClient, err := newMobyDockerClient(client.WithHost(host), client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = moduleClient.Close() })
	agent := &Agent{docker: moduleClient, runtime: RuntimeDocker, logger: zerolog.Nop(),
		cfg: Config{CollectDiskMetrics: true}, prevContainerCPU: make(map[string]cpuSample)}
	manifest, err := qualification.LoadManifest(filepath.Join("..", "..", "tests", "qualification", "patrol",
		"scenarios", "investigation.docker-storage-pressure.json"))
	if err != nil {
		t.Fatal(err)
	}
	driver := qualification.NewDockerLab(nil, qualification.DockerTarget{Context: dockerContext})
	lab, prepareErr := driver.Prepare(ctx, manifest, "collector-"+time.Now().UTC().Format("20060102t150405.000000000"))
	if lab != nil {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
			defer cleanupCancel()
			result := driver.Cleanup(cleanupCtx, manifest, lab)
			encoded, _ := json.Marshal(result)
			t.Logf("cleanup=%s", encoded)
			if !result.Passed || !result.SecondCleanupNoop || !result.InventoryUnchanged {
				t.Errorf("disposable inventory cleanup failed: %+v", result)
			}
		})
	}
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}

	check := func(phase, serviceHealth string, predicates []qualification.Predicate) {
		t.Helper()
		observations, err := driver.Observe(ctx, manifest, lab, predicates)
		encoded, _ := json.Marshal(observations)
		t.Logf("%s oracle=%s", phase, encoded)
		if err != nil {
			t.Fatal(err)
		}
		for _, alias := range []string{"service", "control"} {
			id := lab.ResourceIDs[alias]
			filters := newDockerFilters()
			filters.Add("id", id)
			for _, key := range []string{"io.pulse.owner", "io.pulse.profile", "io.pulse.component"} {
				value := lab.BaselineStates[alias].Labels[key]
				if value == "" {
					t.Fatalf("missing fixture ownership label %s", key)
				}
				filters.Add("label", key+"="+value)
			}
			containers, err := moduleClient.ContainerList(ctx, dockerContainerListOptions{All: true, Filters: filters})
			if err != nil || len(containers) != 1 || containers[0].ID != id {
				t.Fatalf("exact owned fixture %s unavailable: count=%d error=%v", alias, len(containers), err)
			}
			collected, err := agent.collectContainer(ctx, containers[0])
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(collected)
			if err != nil {
				t.Fatal(err)
			}
			var report agentsdocker.Container
			if err := json.Unmarshal(encoded, &report); err != nil {
				t.Fatal(err)
			}
			wantHealth := "healthy"
			if alias == "service" {
				wantHealth = serviceHealth
			}
			if report.ID != id || report.State != "running" || report.Health != wantHealth {
				t.Fatalf("%s %s report identity/state/health mismatch: %s", phase, alias, encoded)
			}
			if alias == "service" {
				const destination = "/var/lib/service-cache"
				inspect, err := moduleClient.ContainerInspect(ctx, id)
				if err != nil || inspect.HostConfig == nil {
					t.Fatalf("inspect storage configuration: %v", err)
				}
				options, ok := inspect.HostConfig.Tmpfs[destination]
				if !ok || !strings.Contains(options, "size=8388608") {
					t.Fatalf("fixture tmpfs configuration missing: %q", options)
				}
				matches := 0
				for _, mount := range report.Mounts {
					if mount.Destination == destination {
						matches++
						if mount.Type != "tmpfs" || !mount.RW || mount.Mode != options {
							t.Fatalf("collector changed tmpfs configuration: %+v", mount)
						}
					}
				}
				if matches != 1 {
					t.Fatalf("expected one storage mount in report, got %d", matches)
				}
				t.Logf("%s raw_mount_count=%d native_tmpfs_options=%q", phase, len(inspect.Mounts), options)
			}
			t.Logf("%s %s report=%s", phase, alias, encoded)
		}
	}
	check("baseline", "healthy", manifest.Baseline)
	for _, fault := range manifest.Faults {
		if err := driver.ApplyFault(ctx, manifest, lab, fault); err != nil {
			t.Fatal(err)
		}
		check("storage-full", "unhealthy", fault.Oracle)
		if err := driver.RevertFault(ctx, manifest, lab, fault); err != nil {
			t.Fatal(err)
		}
		check("recovered", "healthy", fault.RevertOracle)
	}
	check("restored-baseline", "healthy", manifest.Baseline)
}
