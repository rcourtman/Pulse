package dockeragent

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/zerolog"
)

func TestBuildContainerRecreatePlanNetworkModesAndDesiredConfiguration(t *testing.T) {
	const containerID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	base := func() containertypes.InspectResponse {
		inspect := baseInspect()
		inspect.ID = containerID
		inspect.Config.Hostname = containerID[:12]
		inspect.Config.Domainname = "internal.example"
		inspect.Config.Labels = map[string]string{
			"com.docker.compose.project":    "issue1564",
			"com.docker.compose.service":    "app",
			"com.docker.compose.depends_on": `db:service_started:false`,
			"pulse.test":                    "network-lifecycle",
		}
		inspect.Config.Volumes = map[string]struct{}{"/data": {}}
		inspect.HostConfig.Binds = []string{"issue1564-data:/data", "/tmp/issue1564:/host"}
		inspect.HostConfig.Mounts = []mount.Mount{{
			Type:   mount.TypeVolume,
			Source: "issue1564-cache",
			Target: "/cache",
		}}
		inspect.HostConfig.RestartPolicy = containertypes.RestartPolicy{Name: "unless-stopped"}
		return inspect
	}

	t.Run("standalone bridge clears only a generated hostname", func(t *testing.T) {
		inspect := base()
		inspect.HostConfig.NetworkMode = "bridge"
		inspect.NetworkSettings.Networks = map[string]*network.EndpointSettings{
			"bridge": {Aliases: []string{"app"}},
		}

		plan, err := buildContainerRecreatePlan(inspect)
		if err != nil {
			t.Fatal(err)
		}
		if plan.config.Hostname != "" {
			t.Fatalf("generated hostname was retained as explicit: %q", plan.config.Hostname)
		}
		assertRecreatePlanPreservesNonNetworkConfiguration(t, inspect, plan)
		assertPrimaryNetwork(t, plan, "bridge")
		if inspect.Config.Hostname != containerID[:12] {
			t.Fatal("building the recreate plan mutated inspect output")
		}
	})

	t.Run("standalone custom networks preserve explicit configuration deterministically", func(t *testing.T) {
		inspect := base()
		inspect.Config.Hostname = "operator-hostname"
		inspect.HostConfig.NetworkMode = "app-primary"
		primaryEndpoint := &network.EndpointSettings{
			IPAMConfig: &network.EndpointIPAMConfig{
				IPv4Address:  netip.MustParseAddr("172.30.0.20"),
				LinkLocalIPs: []netip.Addr{netip.MustParseAddr("169.254.20.1")},
			},
			Links:       []string{"db:database"},
			Aliases:     []string{"app", "api"},
			DriverOpts:  map[string]string{"com.example.mode": "isolated"},
			GwPriority:  100,
			NetworkID:   "observed-network-id",
			EndpointID:  "observed-endpoint-id",
			Gateway:     netip.MustParseAddr("172.30.0.1"),
			IPAddress:   netip.MustParseAddr("172.30.0.20"),
			MacAddress:  network.HardwareAddr{0x02, 0x42, 0xac, 0x1e, 0x00, 0x14},
			IPPrefixLen: 24,
			DNSNames:    []string{"observed-runtime-name"},
		}
		inspect.NetworkSettings.Networks = map[string]*network.EndpointSettings{
			"z-observer":  {Aliases: []string{"observer"}, GwPriority: 10},
			"app-primary": primaryEndpoint,
			"a-metrics":   {Aliases: []string{"metrics"}, GwPriority: 20},
		}

		plan, err := buildContainerRecreatePlan(inspect)
		if err != nil {
			t.Fatal(err)
		}
		if plan.config.Hostname != "operator-hostname" {
			t.Fatalf("explicit hostname changed: %q", plan.config.Hostname)
		}
		assertRecreatePlanPreservesNonNetworkConfiguration(t, inspect, plan)
		assertPrimaryNetwork(t, plan, "app-primary")
		if got := []string{plan.additionalNetworks[0].name, plan.additionalNetworks[1].name}; !reflect.DeepEqual(got, []string{"a-metrics", "z-observer"}) {
			t.Fatalf("additional network order = %v", got)
		}

		desired := plan.networkingConfig.EndpointsConfig["app-primary"]
		if !reflect.DeepEqual(desired.IPAMConfig, primaryEndpoint.IPAMConfig) ||
			!reflect.DeepEqual(desired.Links, primaryEndpoint.Links) ||
			!reflect.DeepEqual(desired.Aliases, primaryEndpoint.Aliases) ||
			!reflect.DeepEqual(desired.DriverOpts, primaryEndpoint.DriverOpts) ||
			desired.GwPriority != primaryEndpoint.GwPriority ||
			!reflect.DeepEqual(desired.MacAddress, primaryEndpoint.MacAddress) {
			t.Fatalf("desired endpoint configuration was not preserved: %#v", desired)
		}
		if desired.NetworkID != "" || desired.EndpointID != "" || desired.Gateway.IsValid() ||
			desired.IPAddress.IsValid() || desired.IPPrefixLen != 0 || len(desired.DNSNames) != 0 {
			t.Fatalf("observed endpoint state leaked into create input: %#v", desired)
		}
		desired.Aliases[0] = "changed"
		desired.DriverOpts["com.example.mode"] = "changed"
		if primaryEndpoint.Aliases[0] != "app" || primaryEndpoint.DriverOpts["com.example.mode"] != "isolated" {
			t.Fatal("desired endpoint aliases or driver options alias inspect output")
		}
	})

	for _, tc := range []struct {
		name        string
		networkMode containertypes.NetworkMode
	}{
		{name: "compose service shared namespace", networkMode: "container:compose-owner-id"},
		{name: "standalone container shared namespace", networkMode: "container:standalone-owner-id"},
		{name: "host namespace", networkMode: "host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspect := base()
			inspect.Config.ExposedPorts = network.PortSet{network.MustParsePort("8080/tcp"): {}}
			inspect.HostConfig.PortBindings = network.PortMap{
				network.MustParsePort("8080/tcp"): {{HostPort: "8080"}},
			}
			inspect.HostConfig.NetworkMode = tc.networkMode
			// Malformed/legacy inspect output must not cause an explicit
			// endpoint attachment for namespace-sharing modes.
			inspect.NetworkSettings.Networks = map[string]*network.EndpointSettings{
				"bridge": {Aliases: []string{"stale"}},
			}

			plan, err := buildContainerRecreatePlan(inspect)
			if err != nil {
				t.Fatal(err)
			}
			if plan.config.Hostname != "" || plan.config.Domainname != "" {
				t.Fatalf("namespace-owned hostname/domain retained: %q/%q", plan.config.Hostname, plan.config.Domainname)
			}
			if plan.networkingConfig != nil || len(plan.additionalNetworks) != 0 {
				t.Fatalf("shared namespace received endpoint configuration: %#v", plan)
			}
			assertRecreatePlanPreservesNonNetworkConfiguration(t, inspect, plan)
			if tc.networkMode.IsContainer() {
				if len(plan.config.ExposedPorts) != 0 || len(plan.hostConfig.PortBindings) != 0 {
					t.Fatal("container namespace retained conflicting port configuration")
				}
			} else if len(plan.config.ExposedPorts) != 1 || len(plan.hostConfig.PortBindings) != 1 {
				t.Fatal("host namespace lost otherwise-valid port configuration")
			}
		})
	}
}

func TestBuildContainerRecreatePlanRejectsIncompleteInspectBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inspect containertypes.InspectResponse
	}{
		{name: "missing config", inspect: containertypes.InspectResponse{HostConfig: &containertypes.HostConfig{}}},
		{name: "missing host config", inspect: containertypes.InspectResponse{Config: &containertypes.Config{Image: "busybox:1.36"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pullCalled := false
			agent := &Agent{
				docker: &fakeDockerClient{
					containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
						inspect := tc.inspect
						inspect.Name = "/app"
						return inspect, nil
					},
					imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
						pullCalled = true
						return nil, errors.New("must not pull")
					},
				},
				logger: zerolog.Nop(),
			}
			result := agent.updateContainerWithProgress(context.Background(), "container-id", nil)
			if result.Success || !strings.Contains(result.Error, "Failed to prepare container recreation") {
				t.Fatalf("result = %+v", result)
			}
			if pullCalled || result.BackupCreated || result.RollbackAttempted {
				t.Fatalf("invalid inspect crossed mutation boundary: %+v", result)
			}
		})
	}
}

func TestUpdateContainerRollsBackWhenAdditionalNetworkCannotBeRestored(t *testing.T) {
	inspect := baseInspect()
	inspect.ID = strings.Repeat("a", 64)
	inspect.HostConfig.NetworkMode = "primary"
	inspect.NetworkSettings.Networks = map[string]*network.EndpointSettings{
		"secondary": {Aliases: []string{"secondary-alias"}, GwPriority: 10},
		"primary":   {Aliases: []string{"primary-alias"}, GwPriority: 100},
	}

	var (
		createdPrimary string
		removedID      string
		renameCalls    [][2]string
		startedIDs     []string
		progress       []string
	)
	agent := &Agent{
		docker: &fakeDockerClient{
			containerInspectFn: func(context.Context, string) (containertypes.InspectResponse, error) {
				return inspect, nil
			},
			imagePullFn: func(context.Context, string, dockerImagePullOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("{}")), nil
			},
			containerStopFn: func(context.Context, string, dockerContainerStopOptions) error {
				return nil
			},
			containerRenameFn: func(_ context.Context, id, name string) error {
				renameCalls = append(renameCalls, [2]string{id, name})
				return nil
			},
			containerCreateFn: func(_ context.Context, _ *containertypes.Config, _ *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, _ *v1.Platform, _ string) (containertypes.CreateResponse, error) {
				for name := range networkingConfig.EndpointsConfig {
					createdPrimary = name
				}
				return containertypes.CreateResponse{ID: "replacement-id"}, nil
			},
			networkConnectFn: func(_ context.Context, name, id string, endpoint *network.EndpointSettings) error {
				if name != "secondary" || id != "replacement-id" || !reflect.DeepEqual(endpoint.Aliases, []string{"secondary-alias"}) {
					t.Fatalf("unexpected NetworkConnect(%q, %q, %#v)", name, id, endpoint)
				}
				return errors.New("network unavailable")
			},
			containerRemoveFn: func(_ context.Context, id string, _ dockerContainerRemoveOptions) error {
				removedID = id
				return nil
			},
			containerStartFn: func(_ context.Context, id string, _ dockerContainerStartOptions) error {
				startedIDs = append(startedIDs, id)
				return nil
			},
		},
		logger: zerolog.Nop(),
	}

	result := agent.updateContainerWithProgress(context.Background(), "original-id", func(step string) {
		progress = append(progress, step)
	})
	if result.Success || !result.BackupCreated || !result.RollbackAttempted || !result.RolledBack {
		t.Fatalf("network restoration failure did not report successful compensation: %+v", result)
	}
	if !strings.Contains(result.Error, "network secondary") {
		t.Fatalf("network error was not surfaced: %q", result.Error)
	}
	if createdPrimary != "primary" || removedID != "replacement-id" {
		t.Fatalf("primary=%q removed=%q", createdPrimary, removedID)
	}
	if len(renameCalls) != 2 || renameCalls[1][0] != result.BackupContainer || renameCalls[1][1] != "app" {
		t.Fatalf("rollback rename calls = %#v", renameCalls)
	}
	if !reflect.DeepEqual(startedIDs, []string{"original-id"}) {
		t.Fatalf("rollback starts = %v", startedIDs)
	}
	if !containsStringFragment(progress, "network secondary") {
		t.Fatalf("network progress was not reported: %v", progress)
	}
}

func assertRecreatePlanPreservesNonNetworkConfiguration(t *testing.T, inspect containertypes.InspectResponse, plan containerRecreatePlan) {
	t.Helper()
	if !reflect.DeepEqual(plan.config.Labels, inspect.Config.Labels) ||
		!reflect.DeepEqual(plan.config.Volumes, inspect.Config.Volumes) ||
		!reflect.DeepEqual(plan.hostConfig.Binds, inspect.HostConfig.Binds) ||
		!reflect.DeepEqual(plan.hostConfig.Mounts, inspect.HostConfig.Mounts) ||
		!reflect.DeepEqual(plan.hostConfig.RestartPolicy, inspect.HostConfig.RestartPolicy) ||
		plan.hostConfig.NetworkMode != inspect.HostConfig.NetworkMode {
		t.Fatalf("non-network desired configuration changed: plan=%#v inspect=%#v", plan, inspect)
	}
}

func assertPrimaryNetwork(t *testing.T, plan containerRecreatePlan, want string) {
	t.Helper()
	if plan.networkingConfig == nil || len(plan.networkingConfig.EndpointsConfig) != 1 {
		t.Fatalf("networking config = %#v", plan.networkingConfig)
	}
	if _, ok := plan.networkingConfig.EndpointsConfig[want]; !ok {
		t.Fatalf("primary network = %#v, want %q", plan.networkingConfig.EndpointsConfig, want)
	}
}

func containsStringFragment(values []string, fragment string) bool {
	for _, value := range values {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}
