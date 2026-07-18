package models

import (
	"testing"
	"time"
)

// This file exercises previously-uncovered branches of the ToFrontend converters
// in converters.go. It focuses on nested-collection mapping and conditional arms
// (nil vs populated, zero vs non-zero timestamps), not trivial field copies.

func TestDockerSecretToFrontend_BranchCov0718(t *testing.T) {
	now := time.Now()
	updatedAt := now.Add(-time.Hour)

	t.Run("empty normalizes labels and omits timestamps", func(t *testing.T) {
		s := DockerSecret{ID: "sec-1", Name: "empty"}
		f := s.ToFrontend()
		if f.ID != "sec-1" || f.Name != "empty" {
			t.Fatalf("identity fields not mapped: %#v", f)
		}
		if f.CreatedAt != nil {
			t.Fatalf("CreatedAt = %#v, want nil for zero time", f.CreatedAt)
		}
		if f.UpdatedAt != nil {
			t.Fatalf("UpdatedAt = %#v, want nil when source pointer is nil", f.UpdatedAt)
		}
		if f.Labels == nil {
			t.Fatal("Labels should normalize to a non-nil empty map")
		}
		if len(f.Labels) != 0 {
			t.Fatalf("Labels = %#v, want empty map", f.Labels)
		}
	})

	t.Run("populated maps all fields and clones labels", func(t *testing.T) {
		s := DockerSecret{
			ID:               "sec-2",
			Name:             "tls-cert",
			DriverName:       "file",
			TemplatingDriver: "golang",
			Labels:           map[string]string{"env": "prod", "team": "infra"},
			CreatedAt:        now,
			UpdatedAt:        &updatedAt,
		}
		f := s.ToFrontend()
		if f.DriverName != "file" || f.TemplatingDriver != "golang" {
			t.Fatalf("driver fields not mapped: %#v", f)
		}
		if f.CreatedAt == nil || *f.CreatedAt != now.Unix()*1000 {
			t.Fatalf("CreatedAt = %#v, want %d", f.CreatedAt, now.Unix()*1000)
		}
		if f.UpdatedAt == nil || *f.UpdatedAt != updatedAt.Unix()*1000 {
			t.Fatalf("UpdatedAt = %#v, want %d", f.UpdatedAt, updatedAt.Unix()*1000)
		}
		if len(f.Labels) != 2 || f.Labels["env"] != "prod" || f.Labels["team"] != "infra" {
			t.Fatalf("Labels not copied: %#v", f.Labels)
		}
		// Labels must be a copy, not an alias of the source map.
		f.Labels["env"] = "MUTATED"
		if s.Labels["env"] != "prod" {
			t.Fatal("frontend Labels must not alias the source map")
		}
	})

	t.Run("zero updatedAt pointer is dropped", func(t *testing.T) {
		zero := time.Time{}
		s := DockerSecret{ID: "sec-3", UpdatedAt: &zero}
		f := s.ToFrontend()
		if f.UpdatedAt != nil {
			t.Fatalf("UpdatedAt = %#v, want nil for zero time", f.UpdatedAt)
		}
	})
}

func TestDockerConfigToFrontend_BranchCov0718(t *testing.T) {
	now := time.Now()
	updatedAt := now.Add(-30 * time.Minute)

	t.Run("empty normalizes labels and omits timestamps", func(t *testing.T) {
		c := DockerConfig{ID: "cfg-1", Name: "empty"}
		f := c.ToFrontend()
		if f.ID != "cfg-1" || f.Name != "empty" {
			t.Fatalf("identity fields not mapped: %#v", f)
		}
		if f.CreatedAt != nil || f.UpdatedAt != nil {
			t.Fatalf("timestamps should be nil for empty config: %#v", f)
		}
		if f.Labels == nil || len(f.Labels) != 0 {
			t.Fatalf("Labels should normalize to non-nil empty map: %#v", f.Labels)
		}
	})

	t.Run("populated maps all fields and clones labels", func(t *testing.T) {
		c := DockerConfig{
			ID:               "cfg-2",
			Name:             "nginx-conf",
			TemplatingDriver: "golang",
			Labels:           map[string]string{"tier": "web", "owner": "sre"},
			CreatedAt:        now,
			UpdatedAt:        &updatedAt,
		}
		f := c.ToFrontend()
		if f.TemplatingDriver != "golang" {
			t.Fatalf("TemplatingDriver = %q, want golang", f.TemplatingDriver)
		}
		if f.CreatedAt == nil || *f.CreatedAt != now.Unix()*1000 {
			t.Fatalf("CreatedAt = %#v, want %d", f.CreatedAt, now.Unix()*1000)
		}
		if f.UpdatedAt == nil || *f.UpdatedAt != updatedAt.Unix()*1000 {
			t.Fatalf("UpdatedAt = %#v, want %d", f.UpdatedAt, updatedAt.Unix()*1000)
		}
		if len(f.Labels) != 2 || f.Labels["tier"] != "web" {
			t.Fatalf("Labels not copied: %#v", f.Labels)
		}
		f.Labels["tier"] = "MUTATED"
		if c.Labels["tier"] != "web" {
			t.Fatal("frontend Labels must not alias the source map")
		}
	})
}

func TestNodeToFrontend_TemperatureBranchCov0718(t *testing.T) {
	temp := &Temperature{CPUPackage: 72.5, Available: true}
	node := Node{
		ID:          "node-temp",
		Name:        "pve-temp",
		LastSeen:    time.Now(),
		Temperature: temp,
	}
	f := node.ToFrontend()
	if f.Temperature == nil {
		t.Fatal("Temperature should be mapped when Temperature.Available is true")
	}
	if f.Temperature != temp {
		t.Fatalf("Temperature pointer = %p, want %p (should pass through)", f.Temperature, temp)
	}
	if f.Temperature.CPUPackage != 72.5 {
		t.Fatalf("Temperature.CPUPackage = %v, want 72.5", f.Temperature.CPUPackage)
	}

	// Available=false arm: temperature must NOT be mapped.
	nodeUnavailable := Node{
		Name:        "pve-no-temp",
		LastSeen:    time.Now(),
		Temperature: &Temperature{CPUPackage: 50, Available: false},
	}
	if fu := nodeUnavailable.ToFrontend(); fu.Temperature != nil {
		t.Fatalf("Temperature should be nil when Available=false, got %#v", fu.Temperature)
	}

	// nil arm: temperature must NOT be mapped.
	nodeNil := Node{Name: "pve-nil-temp", LastSeen: time.Now()}
	if fn := nodeNil.ToFrontend(); fn.Temperature != nil {
		t.Fatalf("Temperature should be nil when source is nil, got %#v", fn.Temperature)
	}
}

func TestVMToFrontend_CollectionsBranchCov0718(t *testing.T) {
	vm := VM{
		ID:           "vm-coll",
		Name:         "vm-a",
		LastSeen:     time.Now(),
		AgentVersion: "agent-9.9",
		Disks:        []Disk{{Total: 100, Used: 50}, {Total: 200, Used: 80}},
		NetworkInterfaces: []GuestNetworkInterface{
			{Name: "net0", Addresses: []string{"10.0.0.5"}},
		},
	}
	f := vm.ToFrontend()
	if f.AgentVersion != "agent-9.9" {
		t.Fatalf("AgentVersion = %q, want agent-9.9", f.AgentVersion)
	}
	if len(f.Disks) != 2 || f.Disks[1].Total != 200 {
		t.Fatalf("Disks array not mapped: %#v", f.Disks)
	}
	if len(f.NetworkInterfaces) != 1 || f.NetworkInterfaces[0].Name != "net0" {
		t.Fatalf("NetworkInterfaces not mapped: %#v", f.NetworkInterfaces)
	}
	// NetworkInterfaces are copied element-wise; mutating a copy element must not affect source.
	f.NetworkInterfaces[0].Name = "mutated"
	if vm.NetworkInterfaces[0].Name != "net0" {
		t.Fatal("frontend NetworkInterfaces must not alias the source slice elements")
	}
}

func TestContainerToFrontend_CollectionsBranchCov0718(t *testing.T) {
	ct := Container{
		ID:       "ct-coll",
		Name:     "ct-a",
		LastSeen: time.Now(),
		OSName:   "Alpine",
		Disks:    []Disk{{Total: 300, Used: 100}},
		NetworkInterfaces: []GuestNetworkInterface{
			{Name: "eth0", Addresses: []string{"172.16.0.2"}},
		},
	}
	f := ct.ToFrontend()
	if f.OSName != "Alpine" {
		t.Fatalf("OSName = %q, want Alpine", f.OSName)
	}
	if len(f.Disks) != 1 || f.Disks[0].Total != 300 {
		t.Fatalf("Disks array not mapped: %#v", f.Disks)
	}
	if len(f.NetworkInterfaces) != 1 || f.NetworkInterfaces[0].Name != "eth0" {
		t.Fatalf("NetworkInterfaces not mapped: %#v", f.NetworkInterfaces)
	}
}

func TestDockerHostToFrontend_SecretsConfigsSecurity_BranchCov0718(t *testing.T) {
	now := time.Now()
	updated := now.Add(-time.Hour)
	host := DockerHost{
		ID:       "dh-swarm",
		Hostname: "swarm-node",
		LastSeen: now,
		Secrets: []DockerSecret{
			{ID: "sec-1", Name: "db-password", Labels: map[string]string{"env": "prod"}, CreatedAt: now, UpdatedAt: &updated},
		},
		Configs: []DockerConfig{
			{ID: "cfg-1", Name: "nginx-conf", Labels: map[string]string{"tier": "web"}, CreatedAt: now, UpdatedAt: &updated},
		},
		Security: &DockerHostSecurity{
			AuthorizationPlugins:          []string{"authz", "opa"},
			MutatingCommandsBlocked:       true,
			MutatingCommandsBlockedReason: "policy",
		},
	}
	f := host.ToFrontend()

	// Secrets nested-conversion arm.
	if len(f.Secrets) != 1 || f.Secrets[0].ID != "sec-1" {
		t.Fatalf("Secrets not mapped via nested ToFrontend: %#v", f.Secrets)
	}
	if f.Secrets[0].CreatedAt == nil || *f.Secrets[0].CreatedAt != now.Unix()*1000 {
		t.Fatalf("nested Secret CreatedAt not converted: %#v", f.Secrets[0].CreatedAt)
	}
	if f.Secrets[0].Labels["env"] != "prod" {
		t.Fatalf("nested Secret labels not mapped: %#v", f.Secrets[0].Labels)
	}

	// Configs nested-conversion arm.
	if len(f.Configs) != 1 || f.Configs[0].ID != "cfg-1" {
		t.Fatalf("Configs not mapped via nested ToFrontend: %#v", f.Configs)
	}
	if f.Configs[0].UpdatedAt == nil || *f.Configs[0].UpdatedAt != updated.Unix()*1000 {
		t.Fatalf("nested Config UpdatedAt not converted: %#v", f.Configs[0].UpdatedAt)
	}
	if f.Configs[0].Labels["tier"] != "web" {
		t.Fatalf("nested Config labels not mapped: %#v", f.Configs[0].Labels)
	}

	// Security arm + AuthorizationPlugins clone independence.
	if f.Security == nil || !f.Security.MutatingCommandsBlocked || f.Security.MutatingCommandsBlockedReason != "policy" {
		t.Fatalf("Security not mapped: %#v", f.Security)
	}
	if len(f.Security.AuthorizationPlugins) != 2 || f.Security.AuthorizationPlugins[0] != "authz" {
		t.Fatalf("AuthorizationPlugins not mapped: %#v", f.Security.AuthorizationPlugins)
	}
	f.Security.AuthorizationPlugins[0] = "MUTATED"
	if host.Security.AuthorizationPlugins[0] != "authz" {
		t.Fatal("frontend AuthorizationPlugins must not alias the source slice")
	}
}

func TestHostToFrontend_DiskIOAndCloneBranches_BranchCov0718(t *testing.T) {
	lastChecked := time.Now().Add(-time.Hour)
	lastAttempt := time.Now().Add(-30 * time.Minute)
	lastSuccess := time.Now().Add(-15 * time.Minute)

	host := Host{
		ID:            "host-full",
		Hostname:      "server-full",
		LastSeen:      time.Now(),
		DiskIO:        []DiskIO{{Device: "sda", ReadBytes: 1000, WriteBytes: 2000}, {Device: "sdb", ReadBytes: 3000}},
		AppliedConfig: &AgentConfigFingerprint{Version: "v1", Hash: "abc123"},
		AgentUpdate: &AgentUpdateStatus{
			State:            "available",
			AvailableVersion: "2.0.0",
			LastCheckedAt:    &lastChecked,
			LastAttemptAt:    &lastAttempt,
			LastSuccessAt:    &lastSuccess,
		},
		AgentModules: []AgentModuleStatus{{Name: "docker", Enabled: true, State: "ok"}},
	}
	f := host.ToFrontend()

	// DiskIO copy arm.
	if len(f.DiskIO) != 2 || f.DiskIO[0].Device != "sda" || f.DiskIO[1].ReadBytes != 3000 {
		t.Fatalf("DiskIO not mapped: %#v", f.DiskIO)
	}

	// AppliedConfig deep clone (non-nil arm of cloneAgentConfigFingerprint).
	if f.AppliedConfig == nil || f.AppliedConfig.Hash != "abc123" || f.AppliedConfig.Version != "v1" {
		t.Fatalf("AppliedConfig not cloned: %#v", f.AppliedConfig)
	}
	f.AppliedConfig.Hash = "MUTATED"
	if host.AppliedConfig.Hash != "abc123" {
		t.Fatal("frontend AppliedConfig must not alias the source struct")
	}

	// AgentUpdate deep clone with nested timestamp pointers.
	if f.AgentUpdate == nil || f.AgentUpdate.AvailableVersion != "2.0.0" {
		t.Fatalf("AgentUpdate not cloned: %#v", f.AgentUpdate)
	}
	if f.AgentUpdate.LastCheckedAt == nil || !f.AgentUpdate.LastCheckedAt.Equal(lastChecked) {
		t.Fatalf("AgentUpdate.LastCheckedAt not deep-cloned: %#v", f.AgentUpdate.LastCheckedAt)
	}
	if f.AgentUpdate.LastAttemptAt == nil || !f.AgentUpdate.LastAttemptAt.Equal(lastAttempt) {
		t.Fatalf("AgentUpdate.LastAttemptAt not deep-cloned: %#v", f.AgentUpdate.LastAttemptAt)
	}
	if f.AgentUpdate.LastSuccessAt == nil || !f.AgentUpdate.LastSuccessAt.Equal(lastSuccess) {
		t.Fatalf("AgentUpdate.LastSuccessAt not deep-cloned: %#v", f.AgentUpdate.LastSuccessAt)
	}
	// The cloned timestamp pointers must not alias the source pointers.
	*f.AgentUpdate.LastCheckedAt = time.Time{}
	if !host.AgentUpdate.LastCheckedAt.Equal(lastChecked) {
		t.Fatal("frontend AgentUpdate.LastCheckedAt pointer must not alias the source pointer")
	}

	// AgentModules clone (non-empty arm of cloneAgentModuleStatuses).
	if len(f.AgentModules) != 1 || f.AgentModules[0].Name != "docker" {
		t.Fatalf("AgentModules not cloned: %#v", f.AgentModules)
	}
	f.AgentModules[0].Name = "MUTATED"
	if host.AgentModules[0].Name != "docker" {
		t.Fatal("frontend AgentModules must not alias the source slice elements")
	}
}

func TestDockerContainerToFrontend_FinishedLabelsUpdate_BranchCov0718(t *testing.T) {
	now := time.Now()
	finished := now.Add(-5 * time.Minute)
	lastChecked := now.Add(-1 * time.Minute)

	c := DockerContainer{
		ID:         "ct-branches",
		Name:       "web",
		CreatedAt:  now,
		FinishedAt: &finished,
		Labels: map[string]string{
			"com.docker.compose.service": "api",
			"app":                        "shop",
		},
		UpdateStatus: &DockerContainerUpdateStatus{
			UpdateAvailable: true,
			CurrentDigest:   "sha256:aaa",
			LatestDigest:    "sha256:bbb",
			LastChecked:     lastChecked,
			Error:           "rate limited",
		},
	}
	f := c.ToFrontend()

	// FinishedAt arm.
	if f.FinishedAt == nil || *f.FinishedAt != finished.Unix()*1000 {
		t.Fatalf("FinishedAt = %#v, want %d", f.FinishedAt, finished.Unix()*1000)
	}

	// Labels copy arm + independence.
	if len(f.Labels) != 2 || f.Labels["app"] != "shop" {
		t.Fatalf("Labels not copied: %#v", f.Labels)
	}
	f.Labels["app"] = "MUTATED"
	if c.Labels["app"] != "shop" {
		t.Fatal("frontend Labels must not alias the source map")
	}

	// UpdateStatus arm.
	if f.UpdateStatus == nil || !f.UpdateStatus.UpdateAvailable || f.UpdateStatus.Error != "rate limited" {
		t.Fatalf("UpdateStatus not mapped: %#v", f.UpdateStatus)
	}
	if f.UpdateStatus.CurrentDigest != "sha256:aaa" || f.UpdateStatus.LatestDigest != "sha256:bbb" {
		t.Fatalf("UpdateStatus digests not mapped: %#v", f.UpdateStatus)
	}
	if f.UpdateStatus.LastChecked != lastChecked.Unix()*1000 {
		t.Fatalf("UpdateStatus.LastChecked = %d, want %d", f.UpdateStatus.LastChecked, lastChecked.Unix()*1000)
	}
}

func TestDockerServiceToFrontend_UpdateStatusBranchCov0718(t *testing.T) {
	completed := time.Now().Add(-time.Hour)
	svc := DockerService{
		ID:   "svc-upd",
		Name: "api",
		UpdateStatus: &DockerServiceUpdate{
			State:       "completed",
			Message:     "done",
			CompletedAt: &completed,
		},
	}
	f := svc.ToFrontend()
	if f.UpdateStatus == nil {
		t.Fatal("UpdateStatus should be mapped when source pointer is non-nil")
	}
	if f.UpdateStatus.State != "completed" || f.UpdateStatus.Message != "done" {
		t.Fatalf("UpdateStatus fields not mapped: %#v", f.UpdateStatus)
	}
	if f.UpdateStatus.CompletedAt == nil || *f.UpdateStatus.CompletedAt != completed.Unix()*1000 {
		t.Fatalf("UpdateStatus.CompletedAt = %#v, want %d", f.UpdateStatus.CompletedAt, completed.Unix()*1000)
	}
}

func TestDockerTaskToFrontend_CompletedAtBranchCov0718(t *testing.T) {
	now := time.Now()
	completed := now.Add(-2 * time.Minute)
	task := DockerTask{
		ID:          "task-done",
		ServiceName: "web",
		CreatedAt:   now,
		CompletedAt: &completed,
	}
	f := task.ToFrontend()
	if f.CompletedAt == nil || *f.CompletedAt != completed.Unix()*1000 {
		t.Fatalf("CompletedAt = %#v, want %d", f.CompletedAt, completed.Unix()*1000)
	}
}

func TestHostSensorSummaryToFrontend_GPUSmartBranchCov0718(t *testing.T) {
	gpuTemp := 75.0
	gpuUtil := 50.0
	src := HostSensorSummary{
		GPU: []HostGPUSensor{
			{
				ID:                 "gpu0",
				Name:               "nvidia-3080",
				TemperatureCelsius: &gpuTemp,
				UtilizationPercent: &gpuUtil,
			},
		},
		SMART: []HostDiskSMART{
			{Device: "sda", Model: "Samsung SSD", Temperature: 42, Health: "PASSED"},
		},
	}
	dest := hostSensorSummaryToFrontend(src)
	if dest == nil {
		t.Fatal("expected non-nil frontend for populated GPU+SMART sensors")
	}

	// GPU mapping arm.
	if len(dest.GPU) != 1 || dest.GPU[0].ID != "gpu0" || dest.GPU[0].Name != "nvidia-3080" {
		t.Fatalf("GPU not mapped: %#v", dest.GPU)
	}
	if dest.GPU[0].TemperatureCelsius == nil || *dest.GPU[0].TemperatureCelsius != 75.0 {
		t.Fatalf("GPU.TemperatureCelsius not cloned: %#v", dest.GPU[0].TemperatureCelsius)
	}
	if dest.GPU[0].UtilizationPercent == nil || *dest.GPU[0].UtilizationPercent != 50.0 {
		t.Fatalf("GPU.UtilizationPercent not cloned: %#v", dest.GPU[0].UtilizationPercent)
	}
	// Cloned metric pointer must not alias the source pointer.
	*dest.GPU[0].TemperatureCelsius = 999
	if gpuTemp != 75.0 {
		t.Fatal("frontend GPU TemperatureCelsius pointer must not alias the source")
	}

	// SMART mapping arm.
	if len(dest.SMART) != 1 || dest.SMART[0].Device != "sda" || dest.SMART[0].Health != "PASSED" {
		t.Fatalf("SMART not mapped: %#v", dest.SMART)
	}
	if dest.SMART[0].Temperature != 42 || dest.SMART[0].Model != "Samsung SSD" {
		t.Fatalf("SMART fields not mapped: %#v", dest.SMART[0])
	}
}
