package dockeragent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/rs/zerolog"
)

func TestIntegrationContainerUpdateComposeNetworkLifecycle(t *testing.T) {
	if os.Getenv("PULSE_RUN_DOCKER_INTEGRATION") != "1" {
		t.Skip("set PULSE_RUN_DOCKER_INTEGRATION=1 to run live Docker update lifecycle proof")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI is unavailable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	suffix := updateIntegrationSuffix(t)
	project := "pulseit1564" + suffix
	testLabel := "pulse.integration.issue1564=" + suffix

	if os.Getenv("DOCKER_HOST") == "" {
		contextHost, contextErr := exec.CommandContext(ctx, "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
		if contextErr == nil {
			if host := strings.TrimSpace(string(contextHost)); strings.HasPrefix(host, "unix://") {
				t.Setenv("DOCKER_HOST", host)
			}
		}
	}
	rawClient, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rawClient.Close() })
	if _, err := rawClient.Info(ctx, client.InfoOptions{}); err != nil {
		t.Skipf("Docker daemon is unavailable: %v", err)
	}

	pull, err := rawClient.ImagePull(ctx, "busybox:1.36", client.ImagePullOptions{})
	if err != nil {
		t.Fatalf("prepare integration image: %v", err)
	}
	_, _ = io.Copy(io.Discard, pull)
	_ = pull.Close()

	externalOwnerName := project + "-external-owner"
	externalOwner, err := rawClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: externalOwnerName,
		Config: &containertypes.Config{
			Image:  "busybox:1.36",
			Cmd:    []string{"sleep", "300"},
			Labels: map[string]string{"pulse.integration.issue1564": suffix},
		},
		HostConfig: &containertypes.HostConfig{},
	})
	if err != nil {
		t.Fatalf("create external namespace owner: %v", err)
	}
	if _, err := rawClient.ContainerStart(ctx, externalOwner.ID, client.ContainerStartOptions{}); err != nil {
		t.Fatalf("start external namespace owner: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		result, listErr := rawClient.ContainerList(cleanupCtx, client.ContainerListOptions{
			All: true,
			Filters: client.Filters{
				"label": {testLabel: true},
			},
		})
		if listErr == nil {
			for _, item := range result.Items {
				_, _ = rawClient.ContainerRemove(cleanupCtx, item.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			}
		}
		_, _ = rawClient.ContainerRemove(cleanupCtx, externalOwnerName, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	})

	composeFile := filepath.Join(t.TempDir(), "compose.yaml")
	composeYAML := fmt.Sprintf(`services:
  owner:
    image: busybox:1.36
    command: ["sleep", "300"]
    labels:
      pulse.integration.issue1564: %q
    networks:
      - primary
  bridge:
    image: busybox:1.36
    command: ["sleep", "300"]
    network_mode: bridge
    labels:
      pulse.integration.issue1564: %q
    volumes:
      - data:/data
  service_shared:
    image: busybox:1.36
    command: ["sleep", "300"]
    network_mode: service:owner
    depends_on:
      owner:
        condition: service_started
    labels:
      pulse.integration.issue1564: %q
    volumes:
      - data:/data
  container_shared:
    image: busybox:1.36
    command: ["sleep", "300"]
    network_mode: container:%s
    labels:
      pulse.integration.issue1564: %q
    volumes:
      - data:/data
  host:
    image: busybox:1.36
    command: ["sleep", "300"]
    network_mode: host
    labels:
      pulse.integration.issue1564: %q
    volumes:
      - data:/data
  custom:
    image: busybox:1.36
    command: ["sleep", "300"]
    hostname: explicit-custom-host
    depends_on:
      owner:
        condition: service_started
    labels:
      pulse.integration.issue1564: %q
    volumes:
      - data:/data
    networks:
      primary:
        aliases:
          - application
        gw_priority: 100
      secondary:
        aliases:
          - metrics
        gw_priority: 10
volumes:
  data:
networks:
  primary:
  secondary:
`, suffix, suffix, suffix, externalOwner.ID, suffix, suffix, suffix)
	if err := os.WriteFile(composeFile, []byte(composeYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	runCompose := func(args ...string) ([]byte, error) {
		commandArgs := append([]string{"compose", "-f", composeFile, "-p", project}, args...)
		command := exec.CommandContext(ctx, "docker", commandArgs...)
		return command.CombinedOutput()
	}
	if output, err := runCompose("up", "-d"); err != nil {
		t.Fatalf("docker compose up: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		command := exec.CommandContext(cleanupCtx, "docker", "compose", "-f", composeFile, "-p", project, "down", "--volumes", "--remove-orphans")
		_ = command.Run()
	})

	moduleClient, err := newMobyDockerClient(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	agent := &Agent{docker: moduleClient, logger: zerolog.Nop()}
	t.Cleanup(func() { _ = agent.Close() })
	swap(t, &sleepFn, func(time.Duration) {})

	serviceID := func(service string) string {
		t.Helper()
		output, psErr := runCompose("ps", "-q", service)
		if psErr != nil {
			t.Fatalf("docker compose ps %s: %v\n%s", service, psErr, output)
		}
		id := strings.TrimSpace(string(output))
		if id == "" {
			t.Fatalf("docker compose service %s has no container", service)
		}
		return id
	}
	ownerID := serviceID("owner")

	for _, tc := range []struct {
		service        string
		wantMode       string
		wantHostname   string
		wantNetworks   []string
		wantDependency bool
	}{
		{service: "bridge", wantMode: "bridge", wantNetworks: []string{"bridge"}},
		{service: "service_shared", wantMode: "container:" + ownerID, wantHostname: ownerID[:12], wantDependency: true},
		{service: "container_shared", wantMode: "container:" + externalOwner.ID, wantHostname: externalOwner.ID[:12]},
		{service: "host", wantMode: "host"},
		{
			service: "custom", wantMode: project + "_primary", wantHostname: "explicit-custom-host",
			wantNetworks: []string{project + "_primary", project + "_secondary"}, wantDependency: true,
		},
	} {
		t.Run(tc.service, func(t *testing.T) {
			oldID := serviceID(tc.service)
			beforeResult, inspectErr := rawClient.ContainerInspect(ctx, oldID, client.ContainerInspectOptions{})
			if inspectErr != nil {
				t.Fatal(inspectErr)
			}
			before := beforeResult.Container

			var progress []string
			result := agent.updateContainerWithProgress(ctx, oldID, func(step string) {
				progress = append(progress, step)
			})
			if !result.Success {
				t.Fatalf("update failed: %+v (progress=%v)", result, progress)
			}
			if result.NewContainerID == "" || result.NewContainerID == oldID || !result.BackupCreated {
				t.Fatalf("update did not recreate with a backup: %+v", result)
			}

			afterResult, inspectErr := rawClient.ContainerInspect(ctx, result.NewContainerID, client.ContainerInspectOptions{})
			if inspectErr != nil {
				t.Fatal(inspectErr)
			}
			after := afterResult.Container
			if after.State == nil || !after.State.Running {
				t.Fatalf("replacement is not running: %#v", after.State)
			}
			if after.HostConfig == nil || string(after.HostConfig.NetworkMode) != tc.wantMode {
				t.Fatalf("network mode = %#v, want %q", after.HostConfig, tc.wantMode)
			}
			if after.Config == nil || after.Config.Labels["pulse.integration.issue1564"] != suffix ||
				after.Config.Labels["com.docker.compose.project"] != project ||
				after.Config.Labels["com.docker.compose.service"] != tc.service {
				t.Fatalf("Compose and integration labels were not preserved: %#v", after.Config)
			}
			if tc.wantDependency && after.Config.Labels["com.docker.compose.depends_on"] == "" {
				t.Fatalf("Compose dependency metadata was not preserved: %#v", after.Config.Labels)
			}
			if !hasMountTarget(after.Mounts, "/data") {
				t.Fatalf("named volume mount was not preserved: %#v", after.Mounts)
			}
			if tc.wantHostname != "" && after.Config.Hostname != tc.wantHostname {
				t.Fatalf("hostname = %q, want %q", after.Config.Hostname, tc.wantHostname)
			}
			if tc.service == "bridge" {
				if before.Config == nil || before.Config.Hostname != oldID[:12] {
					t.Fatalf("bridge fixture did not start with a generated hostname: %#v", before.Config)
				}
				if after.Config.Hostname != result.NewContainerID[:12] || after.Config.Hostname == before.Config.Hostname {
					t.Fatalf("generated hostname was not regenerated: before=%q after=%q", before.Config.Hostname, after.Config.Hostname)
				}
			}
			for _, networkName := range tc.wantNetworks {
				if after.NetworkSettings == nil || after.NetworkSettings.Networks[networkName] == nil {
					t.Fatalf("network %q was not restored: %#v", networkName, after.NetworkSettings)
				}
			}
			if tc.service == "custom" {
				if !containsString(after.NetworkSettings.Networks[project+"_primary"].Aliases, "application") ||
					!containsString(after.NetworkSettings.Networks[project+"_secondary"].Aliases, "metrics") {
					t.Fatalf("custom network aliases were not preserved: %#v", after.NetworkSettings.Networks)
				}
			}
		})
	}
}

func updateIntegrationSuffix(t *testing.T) string {
	t.Helper()
	var value [4]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value[:])
}

func hasMountTarget(mounts []containertypes.MountPoint, target string) bool {
	for _, item := range mounts {
		if item.Destination == target {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
