package unifiedresources

import (
	"reflect"
	"testing"
	"time"
)

// TestDockerContainerViewBranchcov0719late covers every requested
// DockerContainerView accessor across all three relevant arms:
//   - the nil-receiver arm (v.r == nil)
//   - the nil-nested arm (v.r != nil but v.r.Docker == nil)
//   - the fully-populated arm (with cloned slice/map/ptr independence checks)
//
// The empty-nested arm additionally exercises the slice/map/ptr fields when
// the *DockerData payload exists but the nested collections are unset/empty.
func TestDockerContainerViewBranchcov0719late(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	parentID := "docker-host-1"

	r := &Resource{
		ID:       "app-container-1",
		Type:     ResourceTypeAppContainer,
		Name:     "nextcloud",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"app", "tier:web"},
		ParentID: &parentID,
		Docker: &DockerData{
			HostSourceID:   " docker-host-src-1 ",
			ContainerID:    "container-123",
			Image:          "docker.io/library/nextcloud:29",
			ContainerState: "running",
			Health:         "healthy",
			RestartCount:   3,
			ExitCode:       137,
			UptimeSeconds:  9999,
			Ports: []DockerPortMeta{
				{PrivatePort: 80, PublicPort: 8080, Protocol: "tcp", IP: "0.0.0.0"},
			},
			Labels:   map[string]string{"com.example.stack": "web"},
			Networks: []DockerNetworkMeta{{Name: "frontend", IPv4: "172.20.0.2", IPv6: "fd00::2"}},
			Mounts:   []DockerMountMeta{{Type: "volume", Source: "data", Destination: "/var/www/html", Mode: "rw", RW: true}},
			UpdateStatus: &DockerUpdateStatusMeta{
				UpdateAvailable: true,
				CurrentDigest:   "sha256:abc",
				LatestDigest:    "sha256:def",
				LastChecked:     now,
				Error:           "",
			},
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 42.5},
			Memory: &MetricValue{Used: ptrInt64(512), Total: ptrInt64(1024), Percent: 50},
			Disk:   &MetricValue{Percent: 11},
			NetIn:  &MetricValue{Value: 1234.5},
			NetOut: &MetricValue{Value: 6789.0},
		},
	}

	v := NewDockerContainerView(r)

	t.Run("String_populated", func(t *testing.T) {
		if got, want := v.String(), `DockerContainerView(app-container-1, "nextcloud")`; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("Status_populated", func(t *testing.T) {
		if got := v.Status(); got != StatusOnline {
			t.Fatalf("expected Status %q, got %q", StatusOnline, got)
		}
	})

	t.Run("HostSourceID_populated_trims", func(t *testing.T) {
		if got, want := v.HostSourceID(), "docker-host-src-1"; got != want {
			t.Fatalf("expected HostSourceID %q, got %q", want, got)
		}
	})

	t.Run("Image_populated", func(t *testing.T) {
		if got, want := v.Image(), "docker.io/library/nextcloud:29"; got != want {
			t.Fatalf("expected Image %q, got %q", want, got)
		}
	})

	t.Run("ContainerState_populated", func(t *testing.T) {
		if got, want := v.ContainerState(), "running"; got != want {
			t.Fatalf("expected ContainerState %q, got %q", want, got)
		}
	})

	t.Run("Health_populated", func(t *testing.T) {
		if got, want := v.Health(), "healthy"; got != want {
			t.Fatalf("expected Health %q, got %q", want, got)
		}
	})

	t.Run("RestartCount_populated", func(t *testing.T) {
		if got, want := v.RestartCount(), 3; got != want {
			t.Fatalf("expected RestartCount %d, got %d", want, got)
		}
	})

	t.Run("ExitCode_populated", func(t *testing.T) {
		if got, want := v.ExitCode(), 137; got != want {
			t.Fatalf("expected ExitCode %d, got %d", want, got)
		}
	})

	t.Run("CPUPercent_populated", func(t *testing.T) {
		if got, want := v.CPUPercent(), 42.5; got != want {
			t.Fatalf("expected CPUPercent %v, got %v", want, got)
		}
	})

	t.Run("MemoryUsed_populated", func(t *testing.T) {
		if got, want := v.MemoryUsed(), int64(512); got != want {
			t.Fatalf("expected MemoryUsed %d, got %d", want, got)
		}
	})

	t.Run("MemoryTotal_populated", func(t *testing.T) {
		if got, want := v.MemoryTotal(), int64(1024); got != want {
			t.Fatalf("expected MemoryTotal %d, got %d", want, got)
		}
	})

	t.Run("MemoryPercent_populated", func(t *testing.T) {
		if got, want := v.MemoryPercent(), 50.0; got != want {
			t.Fatalf("expected MemoryPercent %v, got %v", want, got)
		}
	})

	t.Run("DiskPercent_populated", func(t *testing.T) {
		if got, want := v.DiskPercent(), 11.0; got != want {
			t.Fatalf("expected DiskPercent %v, got %v", want, got)
		}
	})

	t.Run("NetInRate_populated", func(t *testing.T) {
		if got, want := v.NetInRate(), 1234.5; got != want {
			t.Fatalf("expected NetInRate %v, got %v", want, got)
		}
	})

	t.Run("NetOutRate_populated", func(t *testing.T) {
		if got, want := v.NetOutRate(), 6789.0; got != want {
			t.Fatalf("expected NetOutRate %v, got %v", want, got)
		}
	})

	t.Run("UptimeSeconds_populated", func(t *testing.T) {
		if got, want := v.UptimeSeconds(), int64(9999); got != want {
			t.Fatalf("expected UptimeSeconds %d, got %d", want, got)
		}
	})

	t.Run("Ports_populated_and_cloned", func(t *testing.T) {
		got := v.Ports()
		if len(got) != 1 || got[0].PrivatePort != 80 || got[0].PublicPort != 8080 || got[0].Protocol != "tcp" || got[0].IP != "0.0.0.0" {
			t.Fatalf("expected populated port meta, got %+v", got)
		}
		if !reflect.DeepEqual(got, r.Docker.Ports) {
			t.Fatalf("Ports clone should equal source by value: got %+v want %+v", got, r.Docker.Ports)
		}
		got[0].PrivatePort = 9999
		if again := v.Ports(); len(again) != 1 || again[0].PrivatePort != 80 {
			t.Fatalf("Ports() must return an independent slice, got %+v", again)
		}
	})

	t.Run("Labels_populated_and_cloned", func(t *testing.T) {
		got := v.Labels()
		if len(got) != 1 || got["com.example.stack"] != "web" {
			t.Fatalf("expected populated labels, got %+v", got)
		}
		if !reflect.DeepEqual(got, r.Docker.Labels) {
			t.Fatalf("Labels clone should equal source by value: got %+v want %+v", got, r.Docker.Labels)
		}
		got["com.example.stack"] = "mutated"
		got["injected"] = "x"
		if again := v.Labels(); len(again) != 1 || again["com.example.stack"] != "web" {
			t.Fatalf("Labels() must return an independent map, got %+v", again)
		}
	})

	t.Run("Networks_populated_and_cloned", func(t *testing.T) {
		got := v.Networks()
		if len(got) != 1 || got[0].Name != "frontend" || got[0].IPv4 != "172.20.0.2" || got[0].IPv6 != "fd00::2" {
			t.Fatalf("expected populated network meta, got %+v", got)
		}
		if !reflect.DeepEqual(got, r.Docker.Networks) {
			t.Fatalf("Networks clone should equal source by value: got %+v want %+v", got, r.Docker.Networks)
		}
		got[0].Name = "mutated"
		if again := v.Networks(); len(again) != 1 || again[0].Name != "frontend" {
			t.Fatalf("Networks() must return an independent slice, got %+v", again)
		}
	})

	t.Run("Mounts_populated_and_cloned", func(t *testing.T) {
		got := v.Mounts()
		if len(got) != 1 || got[0].Type != "volume" || got[0].Source != "data" || got[0].Destination != "/var/www/html" || !got[0].RW {
			t.Fatalf("expected populated mount meta, got %+v", got)
		}
		if !reflect.DeepEqual(got, r.Docker.Mounts) {
			t.Fatalf("Mounts clone should equal source by value: got %+v want %+v", got, r.Docker.Mounts)
		}
		got[0].Source = "mutated"
		if again := v.Mounts(); len(again) != 1 || again[0].Source != "data" {
			t.Fatalf("Mounts() must return an independent slice, got %+v", again)
		}
	})

	t.Run("UpdateStatus_populated_and_cloned", func(t *testing.T) {
		got := v.UpdateStatus()
		if got == nil || !got.UpdateAvailable || got.CurrentDigest != "sha256:abc" || got.LatestDigest != "sha256:def" || !got.LastChecked.Equal(now) {
			t.Fatalf("expected populated update status, got %+v", got)
		}
		got.UpdateAvailable = false
		got.CurrentDigest = "mutated"
		if again := v.UpdateStatus(); again == nil || !again.UpdateAvailable || again.CurrentDigest != "sha256:abc" {
			t.Fatalf("UpdateStatus() must return an independent copy, got %+v", again)
		}
	})

	t.Run("Tags_populated_and_cloned", func(t *testing.T) {
		got := v.Tags()
		assertStringSlice(t, got, []string{"app", "tier:web"})
		got[0] = "mutated"
		if again := v.Tags(); len(again) != 2 || again[0] != "app" {
			t.Fatalf("Tags() must return an independent slice, got %+v", again)
		}
	})

	t.Run("LastSeen_populated", func(t *testing.T) {
		if got := v.LastSeen(); !got.Equal(now) {
			t.Fatalf("expected LastSeen %v, got %v", now, got)
		}
	})

	t.Run("NilNestedDockerArm", func(t *testing.T) {
		r2 := &Resource{
			ID:       "container-no-docker",
			Type:     ResourceTypeAppContainer,
			Name:     "stub",
			Status:   StatusOffline,
			LastSeen: now,
			Tags:     []string{"t1"},
			Docker:   nil,
			Metrics: &ResourceMetrics{
				CPU:    &MetricValue{Percent: 1},
				Memory: &MetricValue{Used: ptrInt64(2), Total: ptrInt64(4), Percent: 50},
				Disk:   &MetricValue{Percent: 5},
				NetIn:  &MetricValue{Value: 6},
				NetOut: &MetricValue{Value: 7},
			},
		}
		v2 := NewDockerContainerView(r2)

		if v2.ID() != "container-no-docker" || v2.Name() != "stub" || v2.Status() != StatusOffline {
			t.Fatalf("basic accessors should still reflect resource fields, got id=%q name=%q status=%q", v2.ID(), v2.Name(), v2.Status())
		}
		if got := v2.String(); got != `DockerContainerView(container-no-docker, "stub")` {
			t.Fatalf("expected String %q, got %q", `DockerContainerView(container-no-docker, "stub")`, got)
		}
		if v2.HostSourceID() != "" {
			t.Fatalf("expected empty HostSourceID when Docker is nil, got %q", v2.HostSourceID())
		}
		if v2.Image() != "" {
			t.Fatalf("expected empty Image when Docker is nil, got %q", v2.Image())
		}
		if v2.ContainerState() != "" {
			t.Fatalf("expected empty ContainerState when Docker is nil, got %q", v2.ContainerState())
		}
		if v2.Health() != "" {
			t.Fatalf("expected empty Health when Docker is nil, got %q", v2.Health())
		}
		if v2.RestartCount() != 0 {
			t.Fatalf("expected zero RestartCount when Docker is nil, got %d", v2.RestartCount())
		}
		if v2.ExitCode() != 0 {
			t.Fatalf("expected zero ExitCode when Docker is nil, got %d", v2.ExitCode())
		}
		if v2.UptimeSeconds() != 0 {
			t.Fatalf("expected zero UptimeSeconds when Docker is nil, got %d", v2.UptimeSeconds())
		}
		if v2.Ports() != nil {
			t.Fatalf("expected nil Ports when Docker is nil, got %+v", v2.Ports())
		}
		if v2.Labels() != nil {
			t.Fatalf("expected nil Labels when Docker is nil, got %+v", v2.Labels())
		}
		if v2.Networks() != nil {
			t.Fatalf("expected nil Networks when Docker is nil, got %+v", v2.Networks())
		}
		if v2.Mounts() != nil {
			t.Fatalf("expected nil Mounts when Docker is nil, got %+v", v2.Mounts())
		}
		if v2.UpdateStatus() != nil {
			t.Fatalf("expected nil UpdateStatus when Docker is nil, got %+v", v2.UpdateStatus())
		}
		if v2.CPUPercent() != 1 || v2.MemoryUsed() != 2 || v2.MemoryTotal() != 4 || v2.MemoryPercent() != 50 || v2.DiskPercent() != 5 || v2.NetInRate() != 6 || v2.NetOutRate() != 7 {
			t.Fatalf("metric accessors must still resolve via r.Metrics when Docker is nil, got cpu=%v memUsed=%d memTotal=%d memPct=%v diskPct=%v netIn=%v netOut=%v", v2.CPUPercent(), v2.MemoryUsed(), v2.MemoryTotal(), v2.MemoryPercent(), v2.DiskPercent(), v2.NetInRate(), v2.NetOutRate())
		}
		if !v2.LastSeen().Equal(now) {
			t.Fatalf("expected LastSeen %v, got %v", now, v2.LastSeen())
		}
		assertStringSlice(t, v2.Tags(), []string{"t1"})
	})

	t.Run("EmptyNestedCollections", func(t *testing.T) {
		r3 := &Resource{
			ID:     "container-empty-collections",
			Type:   ResourceTypeAppContainer,
			Name:   "empty",
			Status: StatusOnline,
			Docker: &DockerData{
				ContainerID:  "container-456",
				Ports:        []DockerPortMeta{},
				Labels:       map[string]string{},
				Networks:     []DockerNetworkMeta{},
				Mounts:       []DockerMountMeta{},
				UpdateStatus: nil,
			},
		}
		v3 := NewDockerContainerView(r3)

		if got := v3.Ports(); got == nil || len(got) != 0 {
			t.Fatalf("expected non-nil empty Ports for empty input, got %+v", got)
		}
		if got := v3.Labels(); got == nil || len(got) != 0 {
			t.Fatalf("expected non-nil empty Labels for empty input, got %+v", got)
		}
		if got := v3.Networks(); got == nil || len(got) != 0 {
			t.Fatalf("expected non-nil empty Networks for empty input, got %+v", got)
		}
		if got := v3.Mounts(); got == nil || len(got) != 0 {
			t.Fatalf("expected non-nil empty Mounts for empty input, got %+v", got)
		}
		if got := v3.UpdateStatus(); got != nil {
			t.Fatalf("expected nil UpdateStatus when nested ptr is nil, got %+v", got)
		}
	})

	t.Run("NilReceiverArm", func(t *testing.T) {
		var zero DockerContainerView

		if got := zero.String(); got != `DockerContainerView(, "")` {
			t.Fatalf("expected String %q for nil receiver, got %q", `DockerContainerView(, "")`, got)
		}
		if zero.ID() != "" || zero.Name() != "" || zero.Status() != "" {
			t.Fatalf("expected empty ID/Name/Status for nil receiver, got id=%q name=%q status=%q", zero.ID(), zero.Name(), zero.Status())
		}
		if zero.HostSourceID() != "" || zero.Image() != "" || zero.ContainerState() != "" || zero.Health() != "" || zero.RestartCount() != 0 || zero.ExitCode() != 0 || zero.UptimeSeconds() != 0 {
			t.Fatalf("expected docker accessors to return zero values for nil receiver, got source=%q image=%q state=%q health=%q restarts=%d exit=%d uptime=%d", zero.HostSourceID(), zero.Image(), zero.ContainerState(), zero.Health(), zero.RestartCount(), zero.ExitCode(), zero.UptimeSeconds())
		}
		if zero.CPUPercent() != 0 || zero.MemoryUsed() != 0 || zero.MemoryTotal() != 0 || zero.MemoryPercent() != 0 || zero.DiskPercent() != 0 || zero.NetInRate() != 0 || zero.NetOutRate() != 0 {
			t.Fatalf("expected metric accessors to return zero values for nil receiver, got cpu=%v memUsed=%d memTotal=%d memPct=%v diskPct=%v netIn=%v netOut=%v", zero.CPUPercent(), zero.MemoryUsed(), zero.MemoryTotal(), zero.MemoryPercent(), zero.DiskPercent(), zero.NetInRate(), zero.NetOutRate())
		}
		if zero.Ports() != nil || zero.Labels() != nil || zero.Networks() != nil || zero.Mounts() != nil || zero.UpdateStatus() != nil {
			t.Fatalf("expected nil slice/map/ptr accessors for nil receiver, got ports=%v labels=%v networks=%v mounts=%v update=%v", zero.Ports(), zero.Labels(), zero.Networks(), zero.Mounts(), zero.UpdateStatus())
		}
		if zero.Tags() != nil {
			t.Fatalf("expected nil Tags for nil receiver, got %v", zero.Tags())
		}
		if !zero.LastSeen().IsZero() {
			t.Fatalf("expected zero LastSeen for nil receiver, got %v", zero.LastSeen())
		}
	})
}
