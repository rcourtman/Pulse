package monitoring

import (
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Reasons a published port gets no suggested check.
const (
	// DockerPortSkipLoopbackOnly: the port is published on 127.0.0.1 or ::1
	// only. Nothing outside the host can reach it, and on the host Docker's
	// proxy accepts the connection even when the container is not listening,
	// so a check there would report a dead service as reachable.
	DockerPortSkipLoopbackOnly = "loopback_only"
	// DockerPortSkipNoHostAddress: the port is published on every interface
	// and the agent reported no address Pulse could use for the host.
	DockerPortSkipNoHostAddress = "no_host_address"
	// DockerPortSkipNotHTTP: the container port carries a protocol other than
	// HTTP (a database, a mail or DNS port). Pulse suggests HTTP checks only:
	// measured on Docker 29, a plain connection to a published port is
	// accepted by Docker's proxy when Pulse runs in a container on the same
	// host, even with nothing listening, so it cannot tell a dead service.
	DockerPortSkipNotHTTP = "not_http"
)

// Container ports that carry a protocol other than HTTP. Some of them log an
// HTTP request as an attack, so Pulse does not send one.
var dockerNonHTTPPrivatePorts = map[int]struct{}{
	21: {}, 22: {}, 25: {}, 53: {}, 110: {}, 143: {}, 465: {}, 587: {}, 993: {}, 995: {},
	1883: {}, 3306: {}, 5432: {}, 5672: {}, 6379: {}, 8883: {}, 9042: {}, 11211: {}, 27017: {},
}

// DockerPortCheckCandidate is one published container port that has no
// availability check yet. SkipReason is empty when Pulse can check it.
type DockerPortCheckCandidate struct {
	HostID        string `json:"hostId"`
	HostName      string `json:"hostName"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	PrivatePort   int    `json:"privatePort"`
	PublicPort    int    `json:"publicPort"`
	Address       string `json:"address,omitempty"`
	SkipReason    string `json:"skipReason,omitempty"`
}

// Source is the durable key a check created from this candidate carries.
func (c DockerPortCheckCandidate) Source() config.AvailabilityDockerPortSource {
	return config.AvailabilityDockerPortSource{
		HostID:        c.HostID,
		ContainerName: c.ContainerName,
		PrivatePort:   c.PrivatePort,
	}
}

// DockerPortCheckCandidates lists the published TCP ports of running
// containers that no availability check covers yet, in host, container and
// port order. It reads only what the Docker agent already reports.
func DockerPortCheckCandidates(hosts []models.DockerHost, targets []config.AvailabilityTarget) []DockerPortCheckCandidate {
	coveredSources := make(map[string]struct{}, len(targets))
	coveredEndpoints := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.DockerPort != nil {
			coveredSources[target.DockerPort.Key()] = struct{}{}
		}
		if endpoint := availabilityTargetEndpoint(target); endpoint != "" {
			coveredEndpoints[endpoint] = struct{}{}
		}
	}

	var candidates []DockerPortCheckCandidate
	for _, host := range hosts {
		hostID := strings.TrimSpace(host.ID)
		if hostID == "" || host.Hidden || host.PendingUninstall {
			continue
		}
		hostAddress := dockerHostCheckAddress(host)
		for _, container := range host.Containers {
			name := config.NormalizeDockerContainerName(container.Name)
			if name == "" || !strings.EqualFold(strings.TrimSpace(container.State), "running") {
				continue
			}
			if container.Podman != nil && container.Podman.Infra {
				continue
			}
			// Pulse checking itself proves nothing: when it is down it cannot
			// say so.
			if isPulseServerImage(container.Image) {
				continue
			}
			for _, published := range publishedDockerTCPPorts(container.Ports) {
				candidate := DockerPortCheckCandidate{
					HostID:        hostID,
					HostName:      dockerHostCheckName(host),
					ContainerID:   strings.TrimSpace(container.ID),
					ContainerName: name,
					PrivatePort:   published.privatePort,
					PublicPort:    published.publicPort,
				}
				if _, covered := coveredSources[candidate.Source().Key()]; covered {
					continue
				}
				_, nonHTTP := dockerNonHTTPPrivatePorts[published.privatePort]
				switch {
				case nonHTTP:
					candidate.SkipReason = DockerPortSkipNotHTTP
				case published.loopbackOnly:
					candidate.SkipReason = DockerPortSkipLoopbackOnly
				case published.bindAddress != "":
					candidate.Address = published.bindAddress
				case hostAddress != "":
					candidate.Address = hostAddress
				default:
					candidate.SkipReason = DockerPortSkipNoHostAddress
				}
				if candidate.Address != "" {
					if _, covered := coveredEndpoints[endpointKey(candidate.Address, candidate.PublicPort)]; covered {
						continue
					}
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.HostName != b.HostName {
			return a.HostName < b.HostName
		}
		if a.HostID != b.HostID {
			return a.HostID < b.HostID
		}
		if a.ContainerName != b.ContainerName {
			return a.ContainerName < b.ContainerName
		}
		return a.PublicPort < b.PublicPort
	})
	return candidates
}

type publishedDockerPort struct {
	privatePort  int
	publicPort   int
	bindAddress  string // a specific non-loopback address the port is bound to
	loopbackOnly bool
}

// publishedDockerTCPPorts folds Docker's per-address bindings (it reports
// 0.0.0.0 and :: separately) into one entry per published port.
func publishedDockerTCPPorts(ports []models.DockerContainerPort) []publishedDockerPort {
	type key struct{ private, public int }
	type bindings struct {
		everywhere bool
		specific   string
		loopback   bool
	}
	seen := make(map[key]*bindings)
	var order []key
	for _, port := range ports {
		protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
		if port.PublicPort <= 0 || port.PrivatePort <= 0 || (protocol != "" && protocol != "tcp") {
			continue
		}
		k := key{private: port.PrivatePort, public: port.PublicPort}
		entry, ok := seen[k]
		if !ok {
			entry = &bindings{}
			seen[k] = entry
			order = append(order, k)
		}
		ip := net.ParseIP(strings.TrimSpace(port.IP))
		switch {
		case ip == nil || ip.IsUnspecified():
			entry.everywhere = true
		case ip.IsLoopback():
			entry.loopback = true
		default:
			// Prefer an IPv4 binding: it is what most Pulse servers can reach.
			if entry.specific == "" || (ip.To4() != nil && net.ParseIP(entry.specific).To4() == nil) {
				entry.specific = ip.String()
			}
		}
	}

	out := make([]publishedDockerPort, 0, len(order))
	for _, k := range order {
		entry := seen[k]
		published := publishedDockerPort{privatePort: k.private, publicPort: k.public}
		switch {
		case entry.everywhere:
		case entry.specific != "":
			published.bindAddress = entry.specific
		default:
			published.loopbackOnly = entry.loopback
		}
		out = append(out, published)
	}
	return out
}

// dockerHostCheckAddress picks the address Pulse checks a host's published
// ports on: a private IPv4 address on a real interface first, then any other
// routable IPv4, then IPv6. Container bridges and tunnels are passed over
// while a real interface has an address.
func dockerHostCheckAddress(host models.DockerHost) string {
	best, bestRank := "", 0
	for _, iface := range host.NetworkInterfaces {
		virtual := isVirtualHostInterface(iface.Name)
		for _, raw := range iface.Addresses {
			ip := parseInterfaceAddress(raw)
			if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
				continue
			}
			rank := 1
			if ip.To4() != nil {
				rank = 2
				if ip.IsPrivate() {
					rank = 3
				}
			}
			if !virtual {
				rank += 3
			}
			if rank > bestRank {
				best, bestRank = ip.String(), rank
			}
		}
	}
	return best
}

// isPulseServerImage reports whether an image reference is the Pulse server
// (rcourtman/pulse on any registry, any tag or digest).
func isPulseServerImage(image string) bool {
	image = strings.ToLower(strings.TrimSpace(image))
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	if colon := strings.LastIndex(image, ":"); colon > strings.LastIndex(image, "/") {
		image = image[:colon]
	}
	return image == "rcourtman/pulse" || strings.HasSuffix(image, "/rcourtman/pulse")
}

func parseInterfaceAddress(raw string) net.IP {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if ip, _, err := net.ParseCIDR(raw); err == nil {
		return ip
	}
	return net.ParseIP(raw)
}

func isVirtualHostInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"lo", "docker", "br-", "veth", "virbr", "cni", "flannel", "cali", "kube", "podman", "tailscale", "wg", "zt", "tun", "tap"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func dockerHostCheckName(host models.DockerHost) string {
	for _, name := range []string{host.CustomDisplayName, host.DisplayName, host.Hostname, host.ID} {
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}
	return ""
}

// availabilityTargetEndpoint is the host and port a saved check connects to,
// so a port an operator already checks by hand is not offered again.
func availabilityTargetEndpoint(target config.AvailabilityTarget) string {
	switch target.Protocol {
	case config.AvailabilityProbeTCP:
		return endpointKey(target.ProbeAddress(), target.Port)
	case config.AvailabilityProbeHTTP, config.AvailabilityProbeHTTPS:
		u, err := target.HTTPURL()
		if err != nil {
			return ""
		}
		port := u.Port()
		if port == "" {
			port = "80"
			if u.Scheme == "https" {
				port = "443"
			}
		}
		number, err := strconv.Atoi(port)
		if err != nil {
			return ""
		}
		return endpointKey(u.Hostname(), number)
	}
	return ""
}

func endpointKey(host string, port int) string {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" || port <= 0 {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
