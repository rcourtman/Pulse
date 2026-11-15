package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func dockerSuggestionHost() models.DockerHost {
	return models.DockerHost{
		ID:       "host-1",
		Hostname: "docker-vm",
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "lo", Addresses: []string{"127.0.0.1/8"}},
			{Name: "docker0", Addresses: []string{"172.17.0.1/16"}},
			{Name: "eth0", Addresses: []string{"fe80::1/64", "192.168.1.20/24"}},
		},
		Containers: []models.DockerContainer{
			{ID: "c-web", Name: "/web", State: "running", Ports: []models.DockerContainerPort{
				// Docker reports the IPv4 and IPv6 wildcard bindings separately.
				{PrivatePort: 80, PublicPort: 8081, Protocol: "tcp", IP: "0.0.0.0"},
				{PrivatePort: 80, PublicPort: 8081, Protocol: "tcp", IP: "::"},
			}},
			{ID: "c-admin", Name: "admin", State: "running", Ports: []models.DockerContainerPort{
				{PrivatePort: 9000, PublicPort: 9000, Protocol: "tcp", IP: "127.0.0.1"},
			}},
			{ID: "c-dns", Name: "dns", State: "running", Ports: []models.DockerContainerPort{
				{PrivatePort: 53, PublicPort: 53, Protocol: "udp", IP: "0.0.0.0"},
				{PrivatePort: 5380, PublicPort: 5380, Protocol: "tcp", IP: "192.168.1.21"},
			}},
			{ID: "c-cache", Name: "cache", State: "running", Ports: []models.DockerContainerPort{
				{PrivatePort: 6379, Protocol: "tcp"}, // exposed, not published
			}},
			{ID: "c-kv", Name: "kv", State: "running", Ports: []models.DockerContainerPort{
				{PrivatePort: 6379, PublicPort: 6379, Protocol: "tcp", IP: "0.0.0.0"},
			}},
			{ID: "c-pulse", Name: "pulse", Image: "ghcr.io/rcourtman/pulse:v6.8.0", State: "running", Ports: []models.DockerContainerPort{
				{PrivatePort: 7655, PublicPort: 7655, Protocol: "tcp", IP: "0.0.0.0"},
			}},
			{ID: "c-old", Name: "old", State: "exited", Ports: []models.DockerContainerPort{
				{PrivatePort: 80, PublicPort: 8099, Protocol: "tcp", IP: "0.0.0.0"},
			}},
		},
	}
}

func TestDockerPortCheckCandidatesFollowWhatDockerPublishes(t *testing.T) {
	candidates := DockerPortCheckCandidates([]models.DockerHost{dockerSuggestionHost()}, nil)

	type row struct {
		container string
		port      int
		address   string
		skip      string
	}
	want := []row{
		// Loopback only: Docker's proxy would answer for a dead container.
		{container: "admin", port: 9000, skip: DockerPortSkipLoopbackOnly},
		// Bound to one address: check it there, not on the host's main one.
		{container: "dns", port: 5380, address: "192.168.1.21"},
		// A Redis port is not a web port: only HTTP checks are suggested.
		{container: "kv", port: 6379, skip: DockerPortSkipNotHTTP},
		// Published everywhere: one check, on the host's LAN address, never
		// on the bridge or a link-local address.
		{container: "web", port: 8081, address: "192.168.1.20"},
	}
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %+v, want %d rows", candidates, len(want))
	}
	for i, w := range want {
		got := candidates[i]
		if got.ContainerName != w.container || got.PublicPort != w.port || got.Address != w.address || got.SkipReason != w.skip {
			t.Errorf("candidate %d = %+v, want %+v", i, got, w)
		}
		if got.HostID != "host-1" || got.HostName != "docker-vm" {
			t.Errorf("candidate %d host = %q/%q", i, got.HostID, got.HostName)
		}
	}
}

func TestDockerPortCheckCandidatesLeaveOutPortsAlreadyChecked(t *testing.T) {
	host := dockerSuggestionHost()
	targets := []config.AvailabilityTarget{
		// Created from the web port earlier. Its address was edited since;
		// the port still counts as covered.
		{ID: "t1", Protocol: config.AvailabilityProbeHTTP, Address: "web.example.test", Port: 80,
			DockerPort: &config.AvailabilityDockerPortSource{HostID: "host-1", ContainerName: "web", PrivatePort: 80}},
		// Typed in by hand against the same endpoint the dns port resolves to.
		{ID: "t2", Protocol: config.AvailabilityProbeTCP, Address: "192.168.1.21", Port: 5380},
	}
	candidates := DockerPortCheckCandidates([]models.DockerHost{host}, targets)
	if len(candidates) != 2 || candidates[0].ContainerName != "admin" || candidates[1].ContainerName != "kv" {
		t.Fatalf("candidates = %+v, want only the two ports Pulse cannot check", candidates)
	}
}

// The bind address comes from an agent's report. A link-local one, where
// cloud metadata services live, is never turned into a check target.
func TestDockerPortCheckCandidatesRefuseALinkLocalBind(t *testing.T) {
	host := dockerSuggestionHost()
	host.Containers = []models.DockerContainer{
		{ID: "c-meta", Name: "meta", State: "running", Ports: []models.DockerContainerPort{
			{PrivatePort: 80, PublicPort: 80, Protocol: "tcp", IP: "169.254.169.254"},
		}},
	}
	candidates := DockerPortCheckCandidates([]models.DockerHost{host}, nil)
	if len(candidates) != 1 || candidates[0].Address != "" || candidates[0].SkipReason != DockerPortSkipNoHostAddress {
		t.Fatalf("candidates = %+v, want the link-local bind skipped with no address", candidates)
	}
}

func TestDockerPortCheckCandidatesNeedAHostAddress(t *testing.T) {
	host := dockerSuggestionHost()
	host.NetworkInterfaces = []models.HostNetworkInterface{{Name: "lo", Addresses: []string{"127.0.0.1/8"}}}
	for _, candidate := range DockerPortCheckCandidates([]models.DockerHost{host}, nil) {
		if candidate.ContainerName == "web" {
			if candidate.SkipReason != DockerPortSkipNoHostAddress || candidate.Address != "" {
				t.Fatalf("web candidate = %+v, want it skipped for no host address", candidate)
			}
			return
		}
	}
	t.Fatal("web candidate missing")
}

// Pulse checking its own port proves nothing, whatever registry or tag the
// image came from. A namesake image from someone else is still offered.
func TestIsPulseServerImage(t *testing.T) {
	for image, want := range map[string]bool{
		"rcourtman/pulse":                       true,
		"rcourtman/pulse:latest":                true,
		"ghcr.io/rcourtman/pulse:v6.8.0":        true,
		"docker.io/rcourtman/pulse@sha256:abcd": true,
		"registry.local:5000/rcourtman/pulse:1": true,
		"someone/pulse:latest":                  false,
		"rcourtman/pulse-agent:latest":          false,
		"nginx:alpine":                          false,
		"":                                      false,
	} {
		if got := isPulseServerImage(image); got != want {
			t.Errorf("isPulseServerImage(%q) = %v, want %v", image, got, want)
		}
	}
}

func TestDockerHostCheckAddressPrefersARealInterface(t *testing.T) {
	cases := []struct {
		name       string
		interfaces []models.HostNetworkInterface
		want       string
	}{
		{name: "bridge only is still usable", interfaces: []models.HostNetworkInterface{
			{Name: "docker0", Addresses: []string{"172.17.0.1/16"}},
		}, want: "172.17.0.1"},
		{name: "LAN beats tunnel and bridge", interfaces: []models.HostNetworkInterface{
			{Name: "tailscale0", Addresses: []string{"100.64.0.5/32"}},
			{Name: "br-1a2b", Addresses: []string{"172.18.0.1/16"}},
			{Name: "enp3s0", Addresses: []string{"10.0.0.7/24"}},
		}, want: "10.0.0.7"},
		{name: "private IPv4 beats public IPv4 and IPv6", interfaces: []models.HostNetworkInterface{
			{Name: "eth0", Addresses: []string{"2001:db8::7/64", "203.0.113.9/24"}},
			{Name: "eth1", Addresses: []string{"192.168.5.4/24"}},
		}, want: "192.168.5.4"},
		{name: "nothing routable", interfaces: []models.HostNetworkInterface{
			{Name: "eth0", Addresses: []string{"fe80::1/64", "169.254.3.4/16"}},
		}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dockerHostCheckAddress(models.DockerHost{NetworkInterfaces: tc.interfaces}); got != tc.want {
				t.Fatalf("address = %q, want %q", got, tc.want)
			}
		})
	}
}

// dockerContainerLinkResource is a container that publishes its port 80 on
// publicPort.
func dockerContainerLinkResource(hostID, containerID, name string, publicPort int) unifiedresources.Resource {
	return unifiedresources.Resource{
		ID:   unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAppContainer, unifiedresources.SourceDocker, hostID+"/container/"+containerID),
		Type: unifiedresources.ResourceTypeAppContainer,
		Name: name,
		Docker: &unifiedresources.DockerData{
			HostSourceID: hostID,
			ContainerID:  containerID,
			Ports:        []unifiedresources.DockerPortMeta{{PrivatePort: 80, PublicPort: publicPort, Protocol: "tcp", IP: "0.0.0.0"}},
		},
	}
}

// A container recreated by `docker compose up` gets a new Docker ID and so a
// new resource ID. A check created from its published port follows the
// container's name on the same host and keeps its history.
func TestMigrateAvailabilityLinkedResourcesFollowsARecreatedContainer(t *testing.T) {
	oldWeb := dockerContainerLinkResource("host-1", "old-id", "web", 8081)
	newWeb := dockerContainerLinkResource("host-1", "new-id", "web", 8081)
	otherHostWeb := dockerContainerLinkResource("host-2", "x-id", "web", 8081)
	twinA := dockerContainerLinkResource("host-1", "twin-a", "twin", 8081)
	twinB := dockerContainerLinkResource("host-1", "twin-b", "twin", 8081)
	// Recreated with a different published port: 8081 may now belong to
	// another service, so the old check must not report against it.
	moved := dockerContainerLinkResource("host-1", "moved-id", "moved", 9090)
	source := func(host, name string) *config.AvailabilityDockerPortSource {
		return &config.AvailabilityDockerPortSource{HostID: host, ContainerName: name, PrivatePort: 80}
	}

	targets := []config.AvailabilityTarget{
		{ID: "recreated", Port: 8081, LinkedResourceID: oldWeb.ID, DockerPort: source("host-1", "/web")},
		{ID: "never-linked", Port: 8081, DockerPort: source("host-1", "web")},
		{ID: "still-live", Port: 8081, LinkedResourceID: otherHostWeb.ID, DockerPort: source("host-2", "web")},
		{ID: "two-namesakes", Port: 8081, LinkedResourceID: "gone", DockerPort: source("host-1", "twin")},
		{ID: "container-gone", Port: 8081, LinkedResourceID: "gone", DockerPort: source("host-1", "removed")},
		{ID: "port-moved", Port: 8081, LinkedResourceID: "gone", DockerPort: source("host-1", "moved")},
		{ID: "typed-by-hand", Port: 8081, LinkedResourceID: oldWeb.ID},
	}
	migrated, changed := migrateAvailabilityLinkedResources(targets, []unifiedresources.Resource{newWeb, otherHostWeb, twinA, twinB, moved})
	if !changed {
		t.Fatal("expected the recreated container's check to be re-attached")
	}
	want := map[string]string{
		"recreated":      newWeb.ID,
		"never-linked":   newWeb.ID,
		"still-live":     otherHostWeb.ID,
		"two-namesakes":  "gone",
		"container-gone": "gone",
		"port-moved":     "gone",
		// No source: an explicit link never follows a name.
		"typed-by-hand": oldWeb.ID,
	}
	for _, target := range migrated {
		if target.LinkedResourceID != want[target.ID] {
			t.Errorf("%s linked to %q, want %q", target.ID, target.LinkedResourceID, want[target.ID])
		}
	}
	if targets[0].LinkedResourceID != oldWeb.ID {
		t.Fatal("migration mutated its input")
	}
}

// A container recreated with a different published port is offered a new
// check: the old one still connects to the old port.
func TestDockerPortCheckCandidatesOfferARepublishedPortAgain(t *testing.T) {
	host := dockerSuggestionHost()
	stale := []config.AvailabilityTarget{
		{ID: "old", Protocol: config.AvailabilityProbeHTTP, Address: "192.168.1.20", Port: 8080,
			DockerPort: &config.AvailabilityDockerPortSource{HostID: "host-1", ContainerName: "web", PrivatePort: 80}},
	}
	for _, candidate := range DockerPortCheckCandidates([]models.DockerHost{host}, stale) {
		if candidate.ContainerName == "web" && candidate.PublicPort == 8081 && candidate.SkipReason == "" {
			return
		}
	}
	t.Fatal("web on its new port 8081 was not offered while the old check still points at 8080")
}
