package monitoring

import (
	"testing"
	"time"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func ptrF64Coverage(v float64) *float64 { return &v }

func TestMonitorLegacyNamesAdditional(t *testing.T) {
	t.Run("node prefers proxmox node name and preserves display name", func(t *testing.T) {
		name, display := monitorLegacyNames(unifiedresources.Resource{
			ID:   "node-1",
			Name: "Node Friendly",
			Proxmox: &unifiedresources.ProxmoxData{
				NodeName: "pve-node-1",
			},
		}, "node")

		if name != "pve-node-1" {
			t.Fatalf("name = %q, want %q", name, "pve-node-1")
		}
		if display != "Node Friendly" {
			t.Fatalf("display = %q, want %q", display, "Node Friendly")
		}
	})

	t.Run("host and docker host use source hostnames", func(t *testing.T) {
		hostName, hostDisplay := monitorLegacyNames(unifiedresources.Resource{
			ID:   "host-1",
			Name: "Friendly Host",
			Agent: &unifiedresources.AgentData{
				Hostname: "agent-host",
			},
		}, "host")
		if hostName != "agent-host" || hostDisplay != "Friendly Host" {
			t.Fatalf("host branch mismatch: name=%q display=%q", hostName, hostDisplay)
		}

		dockerName, dockerDisplay := monitorLegacyNames(unifiedresources.Resource{
			ID:   "docker-1",
			Name: "Friendly Docker",
			Docker: &unifiedresources.DockerData{
				Hostname: "docker-host",
			},
		}, "docker-host")
		if dockerName != "docker-host" || dockerDisplay != "Friendly Docker" {
			t.Fatalf("docker branch mismatch: name=%q display=%q", dockerName, dockerDisplay)
		}
	})

	t.Run("falls back to id when name is empty", func(t *testing.T) {
		name, display := monitorLegacyNames(unifiedresources.Resource{
			ID:   "fallback-id",
			Name: "   ",
		}, "unknown")
		if name != "fallback-id" {
			t.Fatalf("name = %q, want fallback id", name)
		}
		if display != "" {
			t.Fatalf("display = %q, want empty", display)
		}
	})
}

func TestMonitorTemperatureAndUptimeAdditional(t *testing.T) {
	t.Run("temperature follows expected source precedence", func(t *testing.T) {
		resource := unifiedresources.Resource{
			Agent:      &unifiedresources.AgentData{Temperature: ptrF64Coverage(50)},
			Proxmox:    &unifiedresources.ProxmoxData{Temperature: ptrF64Coverage(45)},
			Docker:     &unifiedresources.DockerData{Temperature: ptrF64Coverage(40)},
			Kubernetes: &unifiedresources.K8sData{Temperature: ptrF64Coverage(35)},
		}

		got := monitorTemperature(resource)
		if got == nil || *got != 50 {
			t.Fatalf("temperature = %v, want 50 from agent", got)
		}

		resource.Agent = nil
		got = monitorTemperature(resource)
		if got == nil || *got != 45 {
			t.Fatalf("temperature = %v, want 45 from proxmox", got)
		}

		resource.Proxmox = nil
		got = monitorTemperature(resource)
		if got == nil || *got != 40 {
			t.Fatalf("temperature = %v, want 40 from docker", got)
		}

		resource.Docker = nil
		got = monitorTemperature(resource)
		if got == nil || *got != 35 {
			t.Fatalf("temperature = %v, want 35 from kubernetes", got)
		}

		resource.Kubernetes = nil
		if got := monitorTemperature(resource); got != nil {
			t.Fatalf("temperature = %v, want nil when no source has temp", got)
		}
	})

	t.Run("uptime walks sources and ignores zero values", func(t *testing.T) {
		resource := unifiedresources.Resource{
			Agent:      &unifiedresources.AgentData{UptimeSeconds: 0},
			Proxmox:    &unifiedresources.ProxmoxData{Uptime: 0},
			Docker:     &unifiedresources.DockerData{UptimeSeconds: 0},
			Kubernetes: &unifiedresources.K8sData{UptimeSeconds: 0},
			PBS:        &unifiedresources.PBSData{UptimeSeconds: 11},
			PMG:        &unifiedresources.PMGData{UptimeSeconds: 12},
		}

		got := monitorUptime(resource)
		if got == nil || *got != 11 {
			t.Fatalf("uptime = %v, want 11 from PBS", got)
		}

		resource.PBS.UptimeSeconds = 0
		got = monitorUptime(resource)
		if got == nil || *got != 12 {
			t.Fatalf("uptime = %v, want 12 from PMG", got)
		}

		resource.PMG.UptimeSeconds = 0
		if got := monitorUptime(resource); got != nil {
			t.Fatalf("uptime = %v, want nil when all sources are zero", got)
		}
	})
}

func TestMonitorIdentityLabelsSourceTypeAndLastSeenAdditional(t *testing.T) {
	t.Run("labels are copied and nil is preserved", func(t *testing.T) {
		if got := monitorLabels(unifiedresources.Resource{}); got != nil {
			t.Fatalf("monitorLabels() = %#v, want nil", got)
		}

		resource := unifiedresources.Resource{
			Kubernetes: &unifiedresources.K8sData{
				Labels: map[string]string{"env": "prod"},
			},
		}
		labels := monitorLabels(resource)
		labels["env"] = "dev"
		if resource.Kubernetes.Labels["env"] != "prod" {
			t.Fatalf("expected labels to be copied, source mutated to %q", resource.Kubernetes.Labels["env"])
		}
	})

	t.Run("identity resolution trims values and uses fallbacks", func(t *testing.T) {
		resource := unifiedresources.Resource{
			Identity: unifiedresources.ResourceIdentity{
				MachineID:   " machine-1 ",
				Hostnames:   []string{"", "  host-from-identity  "},
				IPAddresses: []string{" 10.0.0.1 ", "", "10.0.0.2"},
			},
		}
		identity := monitorIdentity(resource, "fallback-name")
		if identity == nil {
			t.Fatal("expected non-nil identity")
		}
		if identity.Hostname != "host-from-identity" {
			t.Fatalf("hostname = %q, want host-from-identity", identity.Hostname)
		}
		if identity.MachineID != "machine-1" {
			t.Fatalf("machineID = %q, want machine-1", identity.MachineID)
		}
		if len(identity.IPs) != 2 || identity.IPs[0] != "10.0.0.1" || identity.IPs[1] != "10.0.0.2" {
			t.Fatalf("ips = %#v, want trimmed non-empty entries", identity.IPs)
		}

		none := monitorIdentity(unifiedresources.Resource{}, "")
		if none != nil {
			t.Fatalf("expected nil identity for empty input, got %#v", none)
		}
	})

	t.Run("source type and last seen branches", func(t *testing.T) {
		if got := monitorSourceType([]unifiedresources.DataSource{unifiedresources.SourceAgent, unifiedresources.SourceDocker}); got != "hybrid" {
			t.Fatalf("monitorSourceType(multi) = %q, want hybrid", got)
		}
		if got := monitorSourceType([]unifiedresources.DataSource{unifiedresources.SourceK8s}); got != "agent" {
			t.Fatalf("monitorSourceType(agent) = %q, want agent", got)
		}
		if got := monitorSourceType([]unifiedresources.DataSource{unifiedresources.SourcePBS}); got != "api" {
			t.Fatalf("monitorSourceType(api) = %q, want api", got)
		}
		if got := monitorSourceType(nil); got != "api" {
			t.Fatalf("monitorSourceType(nil) = %q, want api", got)
		}

		now := time.Now().UTC().Add(-time.Second).Truncate(time.Millisecond)
		if got := monitorLastSeenUnix(now); got != now.UnixMilli() {
			t.Fatalf("monitorLastSeenUnix(non-zero) = %d, want %d", got, now.UnixMilli())
		}

		before := time.Now().UTC().UnixMilli()
		zero := monitorLastSeenUnix(time.Time{})
		after := time.Now().UTC().UnixMilli()
		if zero < before || zero > after {
			t.Fatalf("monitorLastSeenUnix(zero) = %d, want between %d and %d", zero, before, after)
		}
	})

	t.Run("identity prefers direct source hostnames", func(t *testing.T) {
		identity := monitorIdentity(unifiedresources.Resource{
			Agent:    &unifiedresources.AgentData{Hostname: "agent-host"},
			Docker:   &unifiedresources.DockerData{Hostname: "docker-host"},
			Proxmox:  &unifiedresources.ProxmoxData{NodeName: "proxmox-node"},
			Identity: unifiedresources.ResourceIdentity{MachineID: "m"},
		}, "fallback")
		if identity == nil {
			t.Fatal("expected non-nil identity")
		}
		if identity.Hostname != "agent-host" {
			t.Fatalf("hostname = %q, want agent-host", identity.Hostname)
		}
	})
}
