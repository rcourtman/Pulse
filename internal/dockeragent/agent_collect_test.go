package dockeragent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	imagetypes "github.com/moby/moby/api/types/image"
	networktypes "github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
	volumetypes "github.com/moby/moby/api/types/volume"
	dockerclient "github.com/moby/moby/client"
	"github.com/rs/zerolog"
)

func TestBuildHostSecurityInfoAuthorizationPlugins(t *testing.T) {
	security := buildHostSecurityInfo(systemtypes.Info{
		Plugins: systemtypes.PluginsInfo{
			Authorization: []string{"opa", " audit ", "opa", ""},
		},
	})

	if security == nil {
		t.Fatalf("expected host security info")
	}
	if got := security.AuthorizationPlugins; len(got) != 2 || got[0] != "opa" || got[1] != "audit" {
		t.Fatalf("expected normalized authorization plugins, got %#v", got)
	}
}

func TestCollectDockerNativeInventory(t *testing.T) {
	createdAt := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	agent := &Agent{
		logger: zerolog.Nop(),
		docker: &fakeDockerClient{
			imageListFn: func(context.Context, dockerImageListOptions) ([]imagetypes.Summary, error) {
				return []imagetypes.Summary{{
					ID:          " sha256:image1 ",
					RepoTags:    []string{"repo/app:latest", " "},
					RepoDigests: []string{"repo/app@sha256:abc"},
					Size:        1024,
					SharedSize:  256,
					Containers:  2,
					Created:     createdAt.Unix(),
					Labels:      map[string]string{"tier": "web"},
				}}, nil
			},
			volumeListFn: func(context.Context, dockerVolumeListOptions) ([]volumetypes.Volume, error) {
				return []volumetypes.Volume{{
					Name:       " app-data ",
					Driver:     " local ",
					Mountpoint: "/var/lib/docker/volumes/app-data",
					Scope:      " local ",
					Labels:     map[string]string{"backup": "true"},
				}}, nil
			},
			networkListFn: func(context.Context, dockerNetworkListOptions) ([]networktypes.Summary, error) {
				return []networktypes.Summary{{
					Network: networktypes.Network{
						ID:         " net1 ",
						Name:       " app-net ",
						Driver:     " bridge ",
						Scope:      " local ",
						EnableIPv4: true,
						Attachable: true,
						IPAM: networktypes.IPAM{Config: []networktypes.IPAMConfig{{
							Subnet:  netip.MustParsePrefix("10.88.0.0/24"),
							Gateway: netip.MustParseAddr("10.88.0.1"),
						}}},
						Labels:  map[string]string{"env": "prod"},
						Options: map[string]string{"mtu": "1500"},
					},
				}}, nil
			},
			diskUsageFn: func(context.Context, dockerDiskUsageOptions) (dockerclient.DiskUsageResult, error) {
				return dockerclient.DiskUsageResult{
					Images: dockerclient.ImagesDiskUsage{
						TotalCount: 3, ActiveCount: 2, TotalSize: 4096, Reclaimable: 512,
					},
					Volumes: dockerclient.VolumesDiskUsage{
						TotalCount: 1,
						Items: []volumetypes.Volume{{
							Name:      "app-data",
							UsageData: &volumetypes.UsageData{Size: 2048, RefCount: 4},
						}},
					},
				}, nil
			},
		},
	}

	images, err := agent.collectImages(context.Background())
	if err != nil {
		t.Fatalf("collectImages: %v", err)
	}
	if len(images) != 1 || images[0].ID != "sha256:image1" || images[0].CreatedAt != createdAt {
		t.Fatalf("unexpected images: %+v", images)
	}
	if len(images[0].RepoTags) != 1 || images[0].RepoTags[0] != "repo/app:latest" {
		t.Fatalf("expected normalized repo tags, got %#v", images[0].RepoTags)
	}

	usageResult, storageUsage, err := agent.collectStorageUsage(context.Background())
	if err != nil {
		t.Fatalf("collectStorageUsage: %v", err)
	}
	if storageUsage.Images.TotalCount != 3 || storageUsage.Images.ReclaimableBytes != 512 {
		t.Fatalf("unexpected storage usage: %+v", storageUsage)
	}

	volumes, err := agent.collectVolumes(context.Background(), usageResult.Volumes.Items)
	if err != nil {
		t.Fatalf("collectVolumes: %v", err)
	}
	if len(volumes) != 1 || volumes[0].Name != "app-data" || volumes[0].SizeBytes != 2048 || volumes[0].RefCount != 4 {
		t.Fatalf("unexpected volumes: %+v", volumes)
	}

	networks, err := agent.collectNetworks(context.Background())
	if err != nil {
		t.Fatalf("collectNetworks: %v", err)
	}
	if len(networks) != 1 || networks[0].Name != "app-net" || !networks[0].EnableIPv4 || !networks[0].Attachable {
		t.Fatalf("unexpected networks: %+v", networks)
	}
	if len(networks[0].Subnets) != 1 || networks[0].Subnets[0].Subnet != "10.88.0.0/24" || networks[0].Subnets[0].Gateway != "10.88.0.1" {
		t.Fatalf("unexpected network subnets: %+v", networks[0].Subnets)
	}
}

func TestCollectSwarmNodesMapsManagerInventory(t *testing.T) {
	createdAt := time.Date(2026, 5, 24, 8, 15, 0, 0, time.UTC)
	updatedAt := createdAt.Add(2 * time.Minute)
	agent := &Agent{
		logger: zerolog.Nop(),
		docker: &fakeDockerClient{
			nodeListFn: func(context.Context, dockerNodeListOptions) ([]swarmtypes.Node, error) {
				return []swarmtypes.Node{{
					ID: " node-1 ",
					Spec: swarmtypes.NodeSpec{
						Role:         swarmtypes.NodeRoleManager,
						Availability: swarmtypes.NodeAvailabilityActive,
						Annotations: swarmtypes.Annotations{
							Labels: map[string]string{"zone": "rack-a"},
						},
					},
					Description: swarmtypes.NodeDescription{
						Hostname: " manager-1 ",
						Platform: swarmtypes.Platform{
							OS:           "linux",
							Architecture: "amd64",
						},
						Resources: swarmtypes.Resources{
							NanoCPUs:    8_000_000_000,
							MemoryBytes: 32 * 1024 * 1024 * 1024,
						},
						Engine: swarmtypes.EngineDescription{
							EngineVersion: "27.5.1",
							Labels:        map[string]string{"engine": "primary"},
						},
					},
					Status: swarmtypes.NodeStatus{
						State:   swarmtypes.NodeStateReady,
						Message: "ready",
						Addr:    "192.0.2.10",
					},
					ManagerStatus: &swarmtypes.ManagerStatus{
						Leader:       true,
						Reachability: swarmtypes.ReachabilityReachable,
						Addr:         "192.0.2.10:2377",
					},
					Meta: swarmtypes.Meta{
						CreatedAt: createdAt,
						UpdatedAt: updatedAt,
					},
				}}, nil
			},
		},
	}

	nodes, err := agent.collectSwarmNodes(context.Background())
	if err != nil {
		t.Fatalf("collectSwarmNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected one node, got %+v", nodes)
	}
	node := nodes[0]
	if node.ID != "node-1" || node.Hostname != "manager-1" || node.Role != string(swarmtypes.NodeRoleManager) {
		t.Fatalf("unexpected node identity: %+v", node)
	}
	if node.ManagerReachability != string(swarmtypes.ReachabilityReachable) || node.ManagerAddress != "192.0.2.10:2377" || !node.Leader {
		t.Fatalf("expected manager status to be preserved, got %+v", node)
	}
	if node.EngineVersion != "27.5.1" || node.NanoCPUs != 8_000_000_000 || node.MemoryBytes == 0 {
		t.Fatalf("expected engine capacity to be preserved, got %+v", node)
	}
	if node.Labels["zone"] != "rack-a" || node.EngineLabels["engine"] != "primary" {
		t.Fatalf("expected node labels to be preserved, got labels=%+v engine=%+v", node.Labels, node.EngineLabels)
	}
	if !node.CreatedAt.Equal(createdAt) || node.UpdatedAt == nil || !node.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected timestamps to be preserved, got %+v", node)
	}
}

func TestCollectContainer(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success with running container", func(t *testing.T) {
		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200000000},
				SystemUsage: 2000000000,
				OnlineCPUs:  2,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100000000},
				SystemUsage: 1000000000,
			},
			MemoryStats: containertypes.MemoryStats{
				Usage: 1000000,
				Limit: 4000000,
				Stats: map[string]uint64{"cache": 200000},
			},
			Networks: map[string]containertypes.NetworkStats{
				"eth0": {RxBytes: 2048, TxBytes: 1024},
				"eth1": {RxBytes: 512, TxBytes: 256},
			},
			BlkioStats: containertypes.BlkioStats{
				IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
					{Op: "Read", Value: 100},
					{Op: "Write", Value: 200},
				},
			},
		}

		sizeRw := int64(1234)
		sizeRoot := int64(5678)
		inspect := baseInspect()
		inspect.SizeRw = &sizeRw
		inspect.SizeRootFs = &sizeRoot
		inspect.State = &containertypes.State{
			Running:   true,
			StartedAt: time.Now().Add(-time.Minute).Format(time.RFC3339Nano),
			Health:    &containertypes.Health{Status: "healthy"},
		}
		inspect.Config.Env = []string{"PASSWORD=secret", "PATH=/bin"}
		inspect.Config.Image = "nginx@sha256:abc123"
		inspect.NetworkSettings.Networks["net1"].IPAddress = netip.MustParseAddr("10.0.0.2")
		inspect.Mounts = []containertypes.MountPoint{
			{Type: "bind", Source: "/data", Destination: "/data", RW: true},
		}

		agent := &Agent{
			cfg: Config{
				CollectDiskMetrics: true,
			},
			runtime:          RuntimePodman,
			logger:           logger,
			prevContainerCPU: make(map[string]cpuSample),
			registryChecker:  NewRegistryChecker(logger),
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
					return statsReader(t, stats), nil
				},
			},
		}

		summary := containertypes.Summary{
			ID:      "container-123456",
			Names:   []string{"/app"},
			Image:   "nginx@sha256:abc123",
			ImageID: "sha256:local",
			Created: time.Now().Add(-time.Hour).Unix(),
			State:   "running",
			Status:  "Up",
			Ports: []containertypes.PortSummary{
				{PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: netip.MustParseAddr("0.0.0.0")},
			},
			Labels: map[string]string{
				"io.podman.annotations.pod.name": "mypod",
			},
		}

		container, err := agent.collectContainer(context.Background(), summary)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if container.Name != "app" {
			t.Fatalf("expected name app, got %q", container.Name)
		}
		if container.Health != "healthy" {
			t.Fatalf("expected health status, got %q", container.Health)
		}
		if container.BlockIO == nil {
			t.Fatalf("expected block IO to be populated")
		}
		if container.Podman == nil || container.Podman.PodName != "mypod" {
			t.Fatalf("expected podman metadata")
		}
		if container.UpdateStatus == nil {
			t.Fatal("expected update status for digest-pinned image, got nil")
		}
		if container.UpdateStatus.Error == "" {
			t.Fatalf("expected update status for digest-pinned image, got empty error. Status: %+v", container.UpdateStatus)
		}
		if len(container.Networks) == 0 {
			t.Fatalf("expected networks to be populated")
		}
		if container.NetworkRXBytes != 2560 || container.NetworkTXBytes != 1280 {
			t.Fatalf("unexpected network totals: rx=%d tx=%d", container.NetworkRXBytes, container.NetworkTXBytes)
		}
	})

	t.Run("podman uses reported cpu percent from compat stats payload", func(t *testing.T) {
		inspect := baseInspect()
		inspect.State = &containertypes.State{
			Running:   true,
			StartedAt: time.Now().Add(-time.Minute).Format(time.RFC3339Nano),
		}

		agent := &Agent{
			runtime: RuntimePodman,
			logger:  logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  100_000_000,
					systemUsage: 1_000_000_000,
					onlineCPUs:  16,
					read:        time.Now().Add(-time.Second),
				},
			},
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
					payload := `{
						"read":"2026-04-09T12:00:00Z",
						"cpu_stats":{
							"cpu_usage":{"total_usage":130000000},
							"system_cpu_usage":1010000000,
							"online_cpus":16,
							"cpu":0.32,
							"throttling_data":{}
						},
						"precpu_stats":{},
						"memory_stats":{"usage":1000000,"limit":4000000,"stats":{"cache":200000}},
						"blkio_stats":{"io_service_bytes_recursive":[]}
					}`
					return dockerStatsResponseReader{Body: io.NopCloser(strings.NewReader(payload))}, nil
				},
			},
		}

		container, err := agent.collectContainer(context.Background(), containertypes.Summary{
			ID:      "container-123456",
			Names:   []string{"/app"},
			Image:   "nginx:latest",
			ImageID: "sha256:local",
			Created: time.Now().Add(-time.Hour).Unix(),
			State:   "running",
			Status:  "Up",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if container.CPUPercent != 0.32 {
			t.Fatalf("expected podman cpu percent 0.32 from payload, got %f", container.CPUPercent)
		}
	})

	t.Run("stopped container clears sample", func(t *testing.T) {
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {totalUsage: 1},
			},
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					inspect := baseInspect()
					inspect.State = &containertypes.State{Running: false}
					return inspect, nil, nil
				},
			},
		}

		summary := containertypes.Summary{ID: "container-123456", Names: []string{"/app"}, State: "exited"}
		if _, err := agent.collectContainer(context.Background(), summary); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := agent.prevContainerCPU["container1"]; ok {
			t.Fatalf("expected sample to be removed")
		}
	})

	t.Run("inspect error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return containertypes.InspectResponse{}, nil, errors.New("inspect failed")
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("stats error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					inspect := baseInspect()
					inspect.State = &containertypes.State{Running: true}
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
					return dockerStatsResponseReader{}, errors.New("stats failed")
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("stats decode error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					inspect := baseInspect()
					inspect.State = &containertypes.State{Running: true}
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
					return dockerStatsResponseReader{Body: io.NopCloser(strings.NewReader("{"))}, nil
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("uptime negative clamped", func(t *testing.T) {
		future := time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano)
		inspect := baseInspect()
		inspect.State = &containertypes.State{
			Running:   true,
			StartedAt: future,
		}

		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
				OnlineCPUs:  1,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
			},
		}

		agent := &Agent{
			logger:           logger,
			prevContainerCPU: make(map[string]cpuSample),
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
					return statsReader(t, stats), nil
				},
			},
		}

		container, err := agent.collectContainer(context.Background(), containertypes.Summary{ID: "container-123456", Names: []string{"/app"}, State: "running"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if container.UptimeSeconds != 0 {
			t.Fatalf("expected uptime to be clamped to 0, got %d", container.UptimeSeconds)
		}
	})
}

func TestCollectContainers(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("list error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
					return nil, errors.New("list failed")
				},
			},
			logger: logger,
		}

		if _, err := agent.collectContainers(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("filters and prune", func(t *testing.T) {
		agent := &Agent{
			logger:       logger,
			stateFilters: []string{"running"},
			allowedStates: map[string]struct{}{
				"running": {},
			},
			prevContainerCPU: map[string]cpuSample{
				"stale": {totalUsage: 1},
			},
			docker: &fakeDockerClient{
				containerListFunc: func(_ context.Context, opts dockerContainerListOptions) ([]containertypes.Summary, error) {
					if opts.Filters.Len() == 0 {
						t.Fatal("expected filters to be set")
					}
					return []containertypes.Summary{
						{ID: "running1", Names: []string{"/run"}, State: "running"},
						{ID: "exited1", Names: []string{"/exit"}, State: "exited"},
					}, nil
				},
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return containertypes.InspectResponse{}, nil, errors.New("inspect failed")
				},
			},
		}

		containers, err := agent.collectContainers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(containers) != 0 {
			t.Fatalf("expected no containers, got %d", len(containers))
		}
		if _, ok := agent.prevContainerCPU["stale"]; ok {
			t.Fatalf("expected stale sample to be pruned")
		}
	})
}

func TestPrimaryTargetAndHTTPClient(t *testing.T) {
	t.Run("primary target empty", func(t *testing.T) {
		agent := &Agent{}
		if target := agent.primaryTarget(); target.URL != "" {
			t.Fatal("expected empty target")
		}
	})

	t.Run("http client selection", func(t *testing.T) {
		secure := &http.Client{}
		insecure := &http.Client{}
		agent := &Agent{
			httpClients: map[bool]*http.Client{
				false: secure,
				true:  insecure,
			},
		}
		if got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: true}); got != insecure {
			t.Fatal("expected insecure client")
		}
		if got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: false}); got != secure {
			t.Fatal("expected secure client")
		}
	})

	t.Run("http client fallback", func(t *testing.T) {
		agent := &Agent{
			httpClients: map[bool]*http.Client{},
		}
		got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: true})
		if got == nil {
			t.Fatal("expected fallback client")
		}
	})

	t.Run("http client fallback prefers secure", func(t *testing.T) {
		secure := &http.Client{}
		agent := &Agent{
			httpClients: map[bool]*http.Client{
				false: secure,
			},
		}
		got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: true})
		if got != secure {
			t.Fatal("expected secure fallback client")
		}
	})

	t.Run("http client fallback prefers insecure", func(t *testing.T) {
		insecure := &http.Client{}
		agent := &Agent{
			httpClients: map[bool]*http.Client{
				true: insecure,
			},
		}
		got := agent.httpClientFor(TargetConfig{InsecureSkipVerify: false})
		if got != insecure {
			t.Fatal("expected insecure fallback client")
		}
	})
}

func TestNewHTTPClient(t *testing.T) {
	client := newHTTPClient(true)
	if client.Transport == nil {
		t.Fatal("expected transport")
	}
	transport := client.Transport.(*http.Transport)
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected insecure skip verify true")
	}
}

func TestAgentClose(t *testing.T) {
	closed := false
	agent := &Agent{
		docker: &fakeDockerClient{
			closeFn: func() error {
				closed = true
				return nil
			},
		},
	}

	if err := agent.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !closed {
		t.Fatal("expected docker Close to be called")
	}
}
