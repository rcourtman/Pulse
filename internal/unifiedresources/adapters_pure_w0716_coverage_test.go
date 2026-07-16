package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// ---------------------------------------------------------------------------
// maxNodeTemp — pure transform over *models.Temperature; exercises the nil,
// unavailable, CPUMax, CPUPackage, and Cores-loop branches.
// ---------------------------------------------------------------------------

func Test_w0716_ur_maxNodeTemp(t *testing.T) {
	core42 := 42.0
	core55 := 55.0
	cpuMax := 60.0
	cpuPkg := 58.0

	tests := []struct {
		name string
		in   *models.Temperature
		want *float64
	}{
		{name: "nil input returns nil", in: nil, want: nil},
		{name: "available=false returns nil", in: &models.Temperature{Available: false, CPUMax: 90}, want: nil},
		{name: "CPUMax > 0 preferred", in: &models.Temperature{Available: true, CPUMax: cpuMax, CPUPackage: cpuPkg}, want: &cpuMax},
		{name: "CPUPackage used when CPUMax is zero", in: &models.Temperature{Available: true, CPUPackage: cpuPkg}, want: &cpuPkg},
		{name: "CPUPackage ignored when zero, falls to cores", in: &models.Temperature{
			Available: true,
			Cores:     []models.CoreTemp{{Core: 0, Temp: core42}, {Core: 1, Temp: core55}},
		}, want: &core55},
		{name: "negative core temps still picked (no filtering)", in: &models.Temperature{
			Available: true,
			Cores:     []models.CoreTemp{{Core: 0, Temp: -1}},
		}, want: ptrFloat(-1)},
		{name: "no CPU data at all returns nil", in: &models.Temperature{Available: true}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxNodeTemp(tt.in)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("maxNodeTemp() = %v, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("maxNodeTemp() = nil, want %v", *tt.want)
			}
			if *got != *tt.want {
				t.Fatalf("maxNodeTemp() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func ptrFloat(v float64) *float64 { return &v }

// ---------------------------------------------------------------------------
// convertCephServices — pure transform over []models.CephServiceStatus.
// ---------------------------------------------------------------------------

func Test_w0716_ur_convertCephServices(t *testing.T) {
	t.Run("nil slice returns nil", func(t *testing.T) {
		if got := convertCephServices(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
	t.Run("empty slice returns nil", func(t *testing.T) {
		if got := convertCephServices([]models.CephServiceStatus{}); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
	t.Run("populated slice copies fields", func(t *testing.T) {
		in := []models.CephServiceStatus{
			{Type: "mon", Running: 3, Total: 3, Message: "ok"},
			{Type: "osd", Running: 5, Total: 6, Message: "1 down"},
		}
		got := convertCephServices(in)
		if len(got) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(got))
		}
		if got[0].Type != "mon" || got[0].Running != 3 || got[0].Total != 3 {
			t.Fatalf("entry 0 mismatch: %+v", got[0])
		}
		if got[1].Type != "osd" || got[1].Running != 5 || got[1].Total != 6 {
			t.Fatalf("entry 1 mismatch: %+v", got[1])
		}
	})
	t.Run("message field is dropped", func(t *testing.T) {
		got := convertCephServices([]models.CephServiceStatus{{Type: "mgr", Running: 1, Total: 1, Message: "dropped"}})
		if len(got) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// convertGuestInterfaces — pure transform over []models.GuestNetworkInterface.
// ---------------------------------------------------------------------------

func Test_w0716_ur_convertGuestInterfaces(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := convertGuestInterfaces(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
	t.Run("empty returns nil", func(t *testing.T) {
		if got := convertGuestInterfaces([]models.GuestNetworkInterface{}); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
	t.Run("populated trims fields and deduplicates addresses", func(t *testing.T) {
		in := []models.GuestNetworkInterface{
			{Name: " eth0 ", MAC: " AA:BB:CC:DD:EE:FF ", Addresses: []string{"10.0.0.1", "10.0.0.2", "10.0.0.1"}, RXBytes: 1024, TXBytes: 2048},
		}
		got := convertGuestInterfaces(in)
		if len(got) != 1 {
			t.Fatalf("expected 1, got %d", len(got))
		}
		iface := got[0]
		if iface.Name != "eth0" {
			t.Fatalf("Name = %q", iface.Name)
		}
		if iface.MAC != "AA:BB:CC:DD:EE:FF" {
			t.Fatalf("MAC = %q", iface.MAC)
		}
		if len(iface.Addresses) != 2 || iface.Addresses[0] != "10.0.0.1" || iface.Addresses[1] != "10.0.0.2" {
			t.Fatalf("Addresses = %v", iface.Addresses)
		}
		if iface.RXBytes != 1024 || iface.TXBytes != 2048 {
			t.Fatalf("RX/TX = %d/%d", iface.RXBytes, iface.TXBytes)
		}
	})
	t.Run("negative RX/TX clamped to zero", func(t *testing.T) {
		in := []models.GuestNetworkInterface{
			{Name: "net0", RXBytes: -500, TXBytes: -100},
		}
		got := convertGuestInterfaces(in)
		if got[0].RXBytes != 0 || got[0].TXBytes != 0 {
			t.Fatalf("expected 0/0, got %d/%d", got[0].RXBytes, got[0].TXBytes)
		}
	})
}

// ---------------------------------------------------------------------------
// firstDockerImageReference — pure transform over models.DockerImage.
// ---------------------------------------------------------------------------

func Test_w0716_ur_firstDockerImageReference(t *testing.T) {
	tests := []struct {
		name  string
		image models.DockerImage
		want  string
	}{
		{name: "empty image returns empty", image: models.DockerImage{}, want: ""},
		{name: "first valid repo tag returned", image: models.DockerImage{
			RepoTags: []string{"nginx:latest", "nginx:1.21"},
		}, want: "nginx:latest"},
		{name: "none tag skipped falls to next tag", image: models.DockerImage{
			RepoTags: []string{"<none>:<none>", "redis:7"},
		}, want: "redis:7"},
		{name: "whitespace-only tag skipped", image: models.DockerImage{
			RepoTags: []string{"  ", "alpine:3"},
		}, want: "alpine:3"},
		{name: "all none tags fall to digests", image: models.DockerImage{
			RepoTags:    []string{"<none>:<none>"},
			RepoDigests: []string{"nginx@sha256:abc123"},
		}, want: "nginx@sha256:abc123"},
		{name: "no tags no digests returns empty", image: models.DockerImage{
			RepoTags: []string{"<none>:<none>"},
		}, want: ""},
		{name: "leading whitespace in tag is trimmed", image: models.DockerImage{
			RepoTags: []string{"  busybox:latest  "},
		}, want: "busybox:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstDockerImageReference(tt.image)
			if got != tt.want {
				t.Fatalf("firstDockerImageReference() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// hostUnraidCacheStorageIdentity — pure transform over (host, disk).
// ---------------------------------------------------------------------------

func Test_w0716_ur_hostUnraidCacheStorageIdentity(t *testing.T) {
	tests := []struct {
		name string
		host models.Host
		disk models.HostUnraidDisk
		want string
	}{
		{name: "no pool name returns empty", host: models.Host{MachineID: "m1"}, disk: models.HostUnraidDisk{}, want: ""},
		{name: "pool from device when name empty", host: models.Host{MachineID: "m1"}, disk: models.HostUnraidDisk{Device: "/dev/sdb"}, want: "m1/storage/unraid-cache/sdb"},
		{name: "machineID preferred", host: models.Host{MachineID: "mid", Hostname: "tower"}, disk: models.HostUnraidDisk{Name: "cache"}, want: "mid/storage/unraid-cache/cache"},
		{name: "hostname fallback when no machineID", host: models.Host{Hostname: "tower"}, disk: models.HostUnraidDisk{Name: "cache"}, want: "tower/storage/unraid-cache/cache"},
		{name: "whitespace machineID falls to hostname", host: models.Host{MachineID: "  ", Hostname: "nas"}, disk: models.HostUnraidDisk{Name: "nvme-cache"}, want: "nas/storage/unraid-cache/nvme-cache"},
		{name: "both empty returns empty", host: models.Host{}, disk: models.HostUnraidDisk{Name: "cache"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hostUnraidCacheStorageIdentity(tt.host, tt.disk)
			if got != tt.want {
				t.Fatalf("hostUnraidCacheStorageIdentity() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// unraidStorageCapacity — pure transform over models.Host (requires
// host.Unraid != nil). Exercises the /mnt/user short-circuit, the data-disk
// sum path, the deviceUsage fallback, and the zero-total arm.
// ---------------------------------------------------------------------------

func Test_w0716_ur_unraidStorageCapacity(t *testing.T) {
	t.Run("mnt/user mountpoint short-circuits", func(t *testing.T) {
		host := models.Host{
			Disks:  []models.Disk{{Mountpoint: "/mnt/user", Total: 10000, Used: 4000, Free: 6000, Usage: 40.0}},
			Unraid: &models.HostUnraidStorage{},
		}
		total, used, free, pct := unraidStorageCapacity(host)
		if total != 10000 || used != 4000 || free != 6000 || pct != 40.0 {
			t.Fatalf("got (%d,%d,%d,%f), want (10000,4000,6000,40.0)", total, used, free, pct)
		}
	})

	t.Run("mnt/user0 mountpoint short-circuits", func(t *testing.T) {
		host := models.Host{
			Disks:  []models.Disk{{Mountpoint: "/mnt/user0", Total: 5000, Used: 2500, Free: 2500, Usage: 50.0}},
			Unraid: &models.HostUnraidStorage{},
		}
		total, used, free, pct := unraidStorageCapacity(host)
		if total != 5000 || used != 2500 || free != 2500 || pct != 50.0 {
			t.Fatalf("got (%d,%d,%d,%f), want (5000,2500,2500,50.0)", total, used, free, pct)
		}
	})

	t.Run("data disk capacity summed from UsedBytes and FreeBytes", func(t *testing.T) {
		host := models.Host{
			Disks: []models.Disk{},
			Unraid: &models.HostUnraidStorage{Disks: []models.HostUnraidDisk{
				{Name: "disk1", Role: "data", UsedBytes: 3000, FreeBytes: 7000},
				{Name: "disk2", Role: "data", UsedBytes: 2000, FreeBytes: 8000},
			}},
		}
		total, used, free, pct := unraidStorageCapacity(host)
		if total != 20000 || used != 5000 || free != 15000 {
			t.Fatalf("got (%d,%d,%d), want (20000,5000,15000)", total, used, free)
		}
		wantPct := float64(5000) / float64(20000) * 100
		if pct != wantPct {
			t.Fatalf("pct = %f, want %f", pct, wantPct)
		}
	})

	t.Run("parity disk excluded from sum", func(t *testing.T) {
		host := models.Host{
			Disks: []models.Disk{},
			Unraid: &models.HostUnraidStorage{Disks: []models.HostUnraidDisk{
				{Name: "disk1", Role: "data", UsedBytes: 1000, FreeBytes: 1000},
				{Name: "parity1", Role: "parity", UsedBytes: 9999, FreeBytes: 9999},
			}},
		}
		total, _, _, _ := unraidStorageCapacity(host)
		if total != 2000 {
			t.Fatalf("total = %d, want 2000 (parity excluded)", total)
		}
	})

	t.Run("deviceUsage fallback when disk has no capacity", func(t *testing.T) {
		host := models.Host{
			Disks: []models.Disk{{Device: "/dev/sda", Total: 5000, Used: 2000, Free: 3000, Mountpoint: ""}},
			Unraid: &models.HostUnraidStorage{Disks: []models.HostUnraidDisk{
				{Name: "disk1", Role: "data", Device: "/dev/sda", SizeBytes: 0, UsedBytes: 0, FreeBytes: 0},
			}},
		}
		total, used, free, pct := unraidStorageCapacity(host)
		if total != 5000 || used != 2000 || free != 3000 {
			t.Fatalf("got (%d,%d,%d), want (5000,2000,3000)", total, used, free)
		}
		if pct != 40.0 {
			t.Fatalf("pct = %f, want 40.0", pct)
		}
	})

	t.Run("no data yields all zeros", func(t *testing.T) {
		host := models.Host{
			Disks:  []models.Disk{},
			Unraid: &models.HostUnraidStorage{Disks: []models.HostUnraidDisk{}},
		}
		total, used, free, pct := unraidStorageCapacity(host)
		if total != 0 || used != 0 || free != 0 || pct != 0 {
			t.Fatalf("got (%d,%d,%d,%f), want all zeros", total, used, free, pct)
		}
	})
}

// ---------------------------------------------------------------------------
// shouldScaleLegacyUnraidKiBSize — pure predicate with branches for size
// bounds, direct UsedBytes+FreeBytes match, and per-role array/cache checks.
// ---------------------------------------------------------------------------

func Test_w0716_ur_shouldScaleLegacyUnraidKiBSize(t *testing.T) {
	tests := []struct {
		name string
		host models.Host
		disk models.HostUnraidDisk
		size int64
		want bool
	}{
		{
			name: "size <= 0 returns false",
			host: models.Host{Unraid: &models.HostUnraidStorage{}},
			disk: models.HostUnraidDisk{Role: "data"},
			size: 0, want: false,
		},
		{
			name: "size negative returns false",
			host: models.Host{Unraid: &models.HostUnraidStorage{}},
			disk: models.HostUnraidDisk{Role: "data"},
			size: -1, want: false,
		},
		{
			name: "size >= max plausible returns false",
			host: models.Host{Unraid: &models.HostUnraidStorage{}},
			disk: models.HostUnraidDisk{Role: "data"},
			size: 100_000_000_000, want: false,
		},
		{
			name: "direct UsedBytes+FreeBytes match returns true",
			host: models.Host{Unraid: &models.HostUnraidStorage{}},
			disk: models.HostUnraidDisk{Role: "data", UsedBytes: 500_000, FreeBytes: 524_000},
			size: 1000, want: true,
		},
		{
			name: "data role no array mount returns false",
			host: models.Host{Disks: []models.Disk{}, Unraid: &models.HostUnraidStorage{}},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			size: 1000, want: false,
		},
		{
			name: "data role array total matches raw KiB total",
			host: models.Host{
				Disks:  []models.Disk{{Mountpoint: "/mnt/user", Total: 1_024_000_000_000}},
				Unraid: &models.HostUnraidStorage{Disks: []models.HostUnraidDisk{{Name: "disk1", Role: "data", SizeBytes: 1_000_000_000}}},
			},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			size: 1000, want: true,
		},
		{
			name: "data role array total does not match returns false",
			host: models.Host{
				Disks:  []models.Disk{{Mountpoint: "/mnt/user", Total: 999}},
				Unraid: &models.HostUnraidStorage{Disks: []models.HostUnraidDisk{{Name: "disk1", Role: "data", SizeBytes: 1_000_000_000}}},
			},
			disk: models.HostUnraidDisk{Name: "disk1", Role: "data"},
			size: 1000, want: false,
		},
		{
			name: "cache role mount total matches size",
			host: models.Host{
				Disks: []models.Disk{{Mountpoint: "/mnt/cache", Total: 1_024_000}},
			},
			disk: models.HostUnraidDisk{Name: "cache", Role: "cache"},
			size: 1000, want: true,
		},
		{
			name: "cache role mount total does not match returns false",
			host: models.Host{
				Disks: []models.Disk{{Mountpoint: "/mnt/cache", Total: 5}},
			},
			disk: models.HostUnraidDisk{Name: "cache", Role: "cache"},
			size: 1000, want: false,
		},
		{
			name: "unknown role returns false",
			host: models.Host{Unraid: &models.HostUnraidStorage{}},
			disk: models.HostUnraidDisk{Name: "mystery"},
			size: 1000, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldScaleLegacyUnraidKiBSize(tt.host, tt.disk, tt.size)
			if got != tt.want {
				t.Fatalf("shouldScaleLegacyUnraidKiBSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// normalizeDockerLifecycleRuntime — pure normalizer over runtime + podman flag.
// ---------------------------------------------------------------------------

func Test_w0716_ur_normalizeDockerLifecycleRuntime(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
		podman  bool
		want    string
	}{
		{name: "docker stays docker", runtime: "docker", podman: false, want: "docker"},
		{name: "podman stays podman", runtime: "podman", podman: false, want: "podman"},
		{name: "empty with podman flag yields podman", runtime: "", podman: true, want: "podman"},
		{name: "empty without podman yields docker", runtime: "", podman: false, want: "docker"},
		{name: "unknown runtime yields empty", runtime: "containerd", podman: false, want: ""},
		{name: "whitespace runtime treated as empty", runtime: "  ", podman: false, want: "docker"},
		{name: "uppercase docker normalized", runtime: "DOCKER", podman: false, want: "docker"},
		{name: "uppercase podman with whitespace", runtime: " Podman ", podman: true, want: "podman"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDockerLifecycleRuntime(tt.runtime, tt.podman)
			if got != tt.want {
				t.Fatalf("normalizeDockerLifecycleRuntime() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ContractResourceType — switch over ResourceType with per-case arms.
// ---------------------------------------------------------------------------

func Test_w0716_ur_ContractResourceType(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		want     ResourceType
	}{
		{
			name:     "agent with proxmox payload",
			resource: Resource{Type: ResourceTypeAgent, Proxmox: &ProxmoxData{}},
			want:     ResourceTypeAgent,
		},
		{
			name:     "agent with agent payload",
			resource: Resource{Type: ResourceTypeAgent, Agent: &AgentData{}},
			want:     ResourceTypeAgent,
		},
		{
			name:     "agent with truenas payload",
			resource: Resource{Type: ResourceTypeAgent, TrueNAS: &TrueNASData{}},
			want:     ResourceTypeAgent,
		},
		{
			name:     "agent with vmware payload",
			resource: Resource{Type: ResourceTypeAgent, VMware: &VMwareData{}},
			want:     ResourceTypeAgent,
		},
		{
			name:     "agent with docker payload maps to docker-host",
			resource: Resource{Type: ResourceTypeAgent, Docker: &DockerData{}},
			want:     ResourceType("docker-host"),
		},
		{
			name:     "agent with no payload defaults to agent",
			resource: Resource{Type: ResourceTypeAgent},
			want:     ResourceTypeAgent,
		},
		{
			name:     "system container",
			resource: Resource{Type: ResourceTypeSystemContainer},
			want:     ResourceTypeSystemContainer,
		},
		{
			name:     "app container",
			resource: Resource{Type: ResourceTypeAppContainer},
			want:     ResourceTypeAppContainer,
		},
		{
			name:     "storage falls through default arm",
			resource: Resource{Type: ResourceTypeStorage},
			want:     ResourceTypeStorage,
		},
		{
			name:     "vm falls through default arm",
			resource: Resource{Type: ResourceTypeVM},
			want:     ResourceTypeVM,
		},
		{
			name:     "unknown type normalized in default arm",
			resource: Resource{Type: ResourceType("  K8S-CLUSTER  ")},
			want:     ResourceType("k8s-cluster"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContractResourceType(tt.resource)
			if got != tt.want {
				t.Fatalf("ContractResourceType() = %q, want %q", got, tt.want)
			}
		})
	}
}
