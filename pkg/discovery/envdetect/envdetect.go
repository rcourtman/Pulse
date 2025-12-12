package envdetect

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Environment represents the runtime environment type.
type Environment int

const (
	// Unknown indicates the detector could not determine the environment.
	Unknown Environment = iota
	// Native represents bare metal or virtual machine deployments.
	Native
	// DockerHost indicates a container with host networking (shares host stack).
	DockerHost
	// DockerBridge indicates a container attached to a bridge/NAT network.
	DockerBridge
	// LXCPrivileged covers LXC containers with privileged UID mapping.
	LXCPrivileged
	// LXCUnprivileged covers LXC containers with remapped UIDs.
	LXCUnprivileged
)

// String provides a human readable representation of Environment.
func (e Environment) String() string {
	switch e {
	case Native:
		return "native"
	case DockerHost:
		return "docker_host"
	case DockerBridge:
		return "docker_bridge"
	case LXCPrivileged:
		return "lxc_privileged"
	case LXCUnprivileged:
		return "lxc_unprivileged"
	default:
		return "unknown"
	}
}

// ScanPolicy defines scanning behavior parameters for a detected environment.
type ScanPolicy struct {
	MaxConcurrent    int           // Maximum concurrent workers.
	DialTimeout      time.Duration // Timeout for TCP dials.
	HTTPTimeout      time.Duration // Timeout for HTTP requests.
	MaxHostsPerScan  int           // Maximum hosts per subnet phase.
	EnableReverseDNS bool          // Whether reverse DNS lookups are allowed.
	ScanGateways     bool          // Whether to probe inferred gateway networks.
}

// DefaultScanPolicy returns baseline scanning parameters.
func DefaultScanPolicy() ScanPolicy {
	return ScanPolicy{
		MaxConcurrent:    50,
		DialTimeout:      time.Second,
		HTTPTimeout:      2 * time.Second,
		MaxHostsPerScan:  1024,
		EnableReverseDNS: true,
		ScanGateways:     true,
	}
}

// SubnetPhase represents a single phase of network scanning.
type SubnetPhase struct {
	Name       string      // Name for logging/UI (e.g. "container_network").
	Subnets    []net.IPNet // Subnets to process during this phase.
	Confidence float64     // Confidence score (0.0 - 1.0).
	Priority   int         // Lower priority runs earlier.
}

// EnvironmentProfile captures detection results and scanning plan.
type EnvironmentProfile struct {
	Type         Environment       // Detected environment.
	Phases       []SubnetPhase     // Subnet scanning phases.
	ExtraTargets []net.IP          // IPs to always probe.
	IPBlocklist  []net.IP          // Individual IPs to skip (auto-populated with configured Proxmox hosts).
	Policy       ScanPolicy        // Applied scan policy.
	Confidence   float64           // Overall confidence (0.0 - 1.0).
	Warnings     []string          // Non-fatal detection warnings.
	Metadata     map[string]string // Misc metadata (container type, gateway, etc.).
}

// DetectEnvironment performs environment detection and returns a profile.
func DetectEnvironment() (*EnvironmentProfile, error) {
	profile := &EnvironmentProfile{
		Type:       Unknown,
		Phases:     []SubnetPhase{},
		Policy:     DefaultScanPolicy(),
		Confidence: 0.0,
		Warnings:   []string{},
		Metadata:   map[string]string{},
	}

	log.Info().Msg("Detecting runtime environment")

	isContainer, containerType := detectContainer()
	profile.Metadata["container_detected"] = strconv.FormatBool(isContainer)
	if containerType != "" {
		profile.Metadata["container_type"] = containerType
	}

	var err error
	switch {
	case !isContainer:
		profile, err = detectNativeEnvironment(profile)
	case containerType == "docker":
		profile, err = detectDockerEnvironment(profile)
	case containerType == "lxc":
		profile, err = detectLXCEnvironment(profile)
	default:
		profile.Type = Unknown
		profile.Confidence = 0.3
		if containerType == "" {
			profile.Warnings = append(profile.Warnings, "Unable to determine container type; using fallback subnets")
		} else {
			profile.Warnings = append(profile.Warnings, fmt.Sprintf("Unsupported container type %q; using fallback subnets", containerType))
		}
		profile, err = addFallbackSubnets(profile)
	}

	if err != nil {
		// Preserve the error for callers while ensuring we still provide a usable profile.
		profile.Warnings = append(profile.Warnings, err.Error())
	}

	subnetCount := 0
	for _, phase := range profile.Phases {
		subnetCount += len(phase.Subnets)
	}

	log.Info().
		Str("environment", profile.Type.String()).
		Int("phase_count", len(profile.Phases)).
		Int("subnet_count", subnetCount).
		Float64("confidence", profile.Confidence).
		Msg("Environment detection completed")

	return profile, err
}

// detectContainer inspects the host to determine whether we are inside a container.
func detectContainer() (bool, string) {
	containerType := ""

	// 1. systemd-detect-virt --container
	if _, err := exec.LookPath("systemd-detect-virt"); err == nil {
		cmd := exec.Command("systemd-detect-virt", "--container")
		output, err := cmd.CombinedOutput()
		if len(output) > 0 {
			result := strings.TrimSpace(string(output))
			if result != "" && result != "none" {
				log.Debug().Str("virt", result).Msg("systemd-detect-virt reported container environment")
				switch {
				case strings.Contains(result, "lxc"):
					return true, "lxc"
				case strings.Contains(result, "docker"), strings.Contains(result, "containerd"), strings.Contains(result, "podman"):
					return true, "docker"
				default:
					return true, result
				}
			}
		}
		if err != nil {
			log.Debug().Err(err).Msg("systemd-detect-virt --container check failed; continuing with heuristics")
		}
	}

	// 2. Marker files
	if _, err := os.Stat("/.dockerenv"); err == nil {
		log.Debug().Msg("Detected /.dockerenv marker (Docker container)")
		return true, "docker"
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		log.Debug().Msg("Detected /run/.containerenv marker (Podman/OCI container)")
		return true, "docker"
	}

	// 3. /proc/1/cgroup
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		text := string(data)
		switch {
		case strings.Contains(text, "docker"), strings.Contains(text, "kubepods"), strings.Contains(text, "containerd"):
			log.Debug().Msg("Detected Docker via /proc/1/cgroup")
			return true, "docker"
		case strings.Contains(text, "lxc"), strings.Contains(text, "libcontainer"):
			log.Debug().Msg("Detected LXC via /proc/1/cgroup")
			return true, "lxc"
		}
	} else {
		log.Debug().Err(err).Msg("Unable to read /proc/1/cgroup during container detection")
	}

	// 4. /proc/1/environ
	if data, err := os.ReadFile("/proc/1/environ"); err == nil {
		text := string(data)
		switch {
		case strings.Contains(text, "container=lxc"):
			log.Debug().Msg("Detected LXC via /proc/1/environ")
			return true, "lxc"
		case strings.Contains(text, "container=docker"), strings.Contains(text, "container=podman"):
			log.Debug().Msg("Detected Docker via /proc/1/environ")
			return true, "docker"
		}
	} else {
		log.Debug().Err(err).Msg("Unable to read /proc/1/environ during container detection")
	}

	log.Debug().Msg("No container markers detected; assuming native environment")
	return false, containerType
}

// detectNativeEnvironment builds an EnvironmentProfile for native or VM deployments.
func detectNativeEnvironment(profile *EnvironmentProfile) (*EnvironmentProfile, error) {
	subnets, err := getAllLocalSubnets()
	if err != nil {
		return addFallbackSubnets(profileWithWarning(profile, fmt.Sprintf("Failed to enumerate interfaces: %v", err)))
	}

	if len(subnets) == 0 {
		return addFallbackSubnets(profileWithWarning(profile, "No active IPv4 interfaces found; using fallback subnets"))
	}

	profile.Type = Native
	profile.Confidence = 0.95
	profile.Metadata["detected_mode"] = "native"
	profile.Metadata["interface_count"] = strconv.Itoa(len(subnets))

	profile.Phases = append(profile.Phases, SubnetPhase{
		Name:       "local_networks",
		Subnets:    subnets,
		Confidence: 0.95,
		Priority:   1,
	})

	return profile, nil
}

// detectDockerEnvironment determines whether Docker uses host or bridge networking.
func detectDockerEnvironment(profile *EnvironmentProfile) (*EnvironmentProfile, error) {
	hostMode, hostModeWarnings := isDockerHostMode()
	if len(hostModeWarnings) > 0 {
		profile.Warnings = append(profile.Warnings, hostModeWarnings...)
	}

	if hostMode {
		subnets, err := getAllLocalSubnets()
		if err != nil {
			return addFallbackSubnets(profileWithWarning(profile, fmt.Sprintf("Docker host mode: failed to enumerate subnets: %v", err)))
		}

		if len(subnets) == 0 {
			return addFallbackSubnets(profileWithWarning(profile, "Docker host mode detected but no subnets found; falling back to common subnets"))
		}

		profile.Type = DockerHost
		profile.Confidence = 0.9
		profile.Metadata["docker_mode"] = "host"
		profile.Metadata["interface_count"] = strconv.Itoa(len(subnets))

		profile.Phases = append(profile.Phases, SubnetPhase{
			Name:       "host_networks",
			Subnets:    subnets,
			Confidence: 0.9,
			Priority:   1,
		})

		return profile, nil
	}

	// Bridge/NAT mode.
	profile.Type = DockerBridge
	profile.Confidence = 0.85
	profile.Metadata["docker_mode"] = "bridge"

	containerSubnets, err := getAllLocalSubnets()
	if err != nil {
		profile.Warnings = append(profile.Warnings, fmt.Sprintf("Docker bridge: failed to enumerate container subnets: %v", err))
	} else if len(containerSubnets) > 0 {
		profile.Phases = append(profile.Phases, SubnetPhase{
			Name:       "container_network",
			Subnets:    containerSubnets,
			Confidence: 0.95,
			Priority:   1,
		})
		profile.Metadata["container_subnet_count"] = strconv.Itoa(len(containerSubnets))
	} else {
		profile.Warnings = append(profile.Warnings, "Docker bridge: no container subnets detected")
	}

	if profile.Policy.ScanGateways {
		hostSubnets, confidence, warnings := detectHostNetworkFromContainer()
		if len(warnings) > 0 {
			profile.Warnings = append(profile.Warnings, warnings...)
		}
		if len(hostSubnets) > 0 {
			profile.Phases = append(profile.Phases, SubnetPhase{
				Name:       "inferred_host_network",
				Subnets:    hostSubnets,
				Confidence: confidence,
				Priority:   2,
			})
			profile.Metadata["inferred_host_subnets"] = strconv.Itoa(len(hostSubnets))
		}
	}

	if len(profile.Phases) == 0 {
		return addFallbackSubnets(profileWithWarning(profile, "Docker bridge detection yielded no subnets; adding fallback ranges"))
	}

	return profile, nil
}

// detectLXCEnvironment evaluates privilege level and prepares scanning phases.
func detectLXCEnvironment(profile *EnvironmentProfile) (*EnvironmentProfile, error) {
	privileged, warn := isLXCPrivileged()
	if warn != "" {
		profile.Warnings = append(profile.Warnings, warn)
	}

	containerSubnets, err := getAllLocalSubnets()
	if err != nil {
		profile.Warnings = append(profile.Warnings, fmt.Sprintf("LXC: failed to enumerate container subnets: %v", err))
	}

	if privileged {
		if len(containerSubnets) == 0 {
			return addFallbackSubnets(profileWithWarning(profile, "Privileged LXC detected but no subnets found; using fallback subnets"))
		}

		profile.Type = LXCPrivileged
		profile.Confidence = 0.9
		profile.Metadata["lxc_privileged"] = "true"
		profile.Metadata["interface_count"] = strconv.Itoa(len(containerSubnets))

		profile.Phases = append(profile.Phases, SubnetPhase{
			Name:       "lxc_host_networks",
			Subnets:    containerSubnets,
			Confidence: 0.9,
			Priority:   1,
		})

		return profile, nil
	}

	// Unprivileged container.
	profile.Type = LXCUnprivileged
	profile.Confidence = 0.85
	profile.Metadata["lxc_privileged"] = "false"

	if len(containerSubnets) > 0 {
		profile.Phases = append(profile.Phases, SubnetPhase{
			Name:       "lxc_container_network",
			Subnets:    containerSubnets,
			Confidence: 0.9,
			Priority:   1,
		})
		profile.Metadata["container_subnet_count"] = strconv.Itoa(len(containerSubnets))
	}

	if profile.Policy.ScanGateways {
		hostSubnets, confidence, warnings := detectHostNetworkFromContainer()
		if len(warnings) > 0 {
			profile.Warnings = append(profile.Warnings, warnings...)
		}
		if len(hostSubnets) > 0 {
			profile.Phases = append(profile.Phases, SubnetPhase{
				Name:       "lxc_parent_network",
				Subnets:    hostSubnets,
				Confidence: confidence,
				Priority:   2,
			})
			profile.Metadata["inferred_host_subnets"] = strconv.Itoa(len(hostSubnets))
		}
	}

	if len(profile.Phases) == 0 {
		return addFallbackSubnets(profileWithWarning(profile, "Unprivileged LXC detection yielded no subnets; adding fallback ranges"))
	}

	return profile, nil
}

// isDockerHostMode attempts to determine whether Docker is using host networking.
func isDockerHostMode() (bool, []string) {
	var warnings []string

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to enumerate interfaces while detecting Docker mode")
		warnings = append(warnings, fmt.Sprintf("Unable to enumerate interfaces: %v", err))
		return false, warnings
	}

	routeCount, routeWarn := countKernelRoutes()
	if routeWarn != "" {
		warnings = append(warnings, routeWarn)
	}

	log.Debug().
		Int("interface_count", len(interfaces)).
		Int("route_count", routeCount).
		Msg("Docker networking mode heuristics")

	// Heuristic: host networking tends to expose many interfaces and routes.
	if len(interfaces) > 3 && routeCount > 5 {
		return true, warnings
	}

	return false, warnings
}

// isLXCPrivileged inspects UID mappings to determine privilege level.
func isLXCPrivileged() (bool, string) {
	data, err := os.ReadFile("/proc/self/uid_map")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return false, "Unable to read /proc/self/uid_map (permission denied); assuming unprivileged LXC"
		}
		return false, fmt.Sprintf("Unable to read /proc/self/uid_map; assuming unprivileged LXC: %v", err)
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return false, "Unexpected format in /proc/self/uid_map; assuming unprivileged LXC"
	}

	// Format: id_inside id_outside length
	hostUID, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return false, "Failed to parse uid_map; assuming unprivileged LXC"
	}

	length, err := strconv.ParseUint(fields[2], 10, 32)
	if err != nil {
		return false, "Failed to parse uid_map length; assuming unprivileged LXC"
	}

	if hostUID == 0 && length >= 4294967295 {
		return true, ""
	}

	return false, ""
}

// getAllLocalSubnets enumerates non-loopback, UP IPv4 subnets.
func getAllLocalSubnets() ([]net.IPNet, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	var subnets []net.IPNet
	seen := make(map[string]struct{})

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			log.Debug().Err(err).Str("interface", iface.Name).Msg("Skipping interface due to address enumeration failure")
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet == nil {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}

			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			networkIP := ip.Mask(ipNet.Mask)
			cidr := (&net.IPNet{IP: networkIP, Mask: ipNet.Mask}).String()
			if _, exists := seen[cidr]; exists {
				continue
			}

			seen[cidr] = struct{}{}
			normalized := net.IPNet{IP: networkIP, Mask: ipNet.Mask}
			subnets = append(subnets, normalized)

			log.Debug().
				Str("interface", iface.Name).
				Str("cidr", normalized.String()).
				Msg("Discovered local subnet")
		}
	}

	return subnets, nil
}

// detectHostNetworkFromContainer infers host LAN subnets from container context.
func detectHostNetworkFromContainer() ([]net.IPNet, float64, []string) {
	var warnings []string

	gateway, err := getDefaultGateway()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Could not determine default gateway: %v", err))
		return tryCommonSubnets(), 0.3, warnings
	}

	if gateway == nil || gateway.Equal(net.IPv4zero) {
		warnings = append(warnings, "Default gateway is unspecified; using common subnet fallback")
		return tryCommonSubnets(), 0.3, warnings
	}

	log.Debug().Str("gateway", gateway.String()).Msg("Default gateway detected")

	gateway4 := gateway.To4()
	if gateway4 == nil {
		warnings = append(warnings, fmt.Sprintf("Default gateway %s is not IPv4; using fallback subnets", gateway.String()))
		return tryCommonSubnets(), 0.3, warnings
	}

	confidence := 0.4
	lastOctet := gateway4[3]
	if lastOctet == 1 || lastOctet == 254 {
		confidence = 0.7
	} else {
		warnings = append(warnings, fmt.Sprintf("Gateway %s does not end with .1 or .254; confidence reduced", gateway.String()))
	}

	hostSubnet := net.IPNet{
		IP:   net.IPv4(gateway4[0], gateway4[1], gateway4[2], 0),
		Mask: net.CIDRMask(24, 32),
	}

	log.Debug().Str("host_subnet", hostSubnet.String()).Float64("confidence", confidence).Msg("Inferred host subnet from gateway")

	return []net.IPNet{hostSubnet}, confidence, warnings
}

// getDefaultGateway parses /proc/net/route for the default gateway.
func getDefaultGateway() (net.IP, error) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/net/route: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Iface") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Destination column (fields[1]) equal to 00000000 indicates default route.
		if fields[1] != "00000000" {
			continue
		}

		gatewayHex := fields[2]
		if len(gatewayHex) != 8 {
			continue
		}

		gatewayIP, err := parseHexIP(gatewayHex)
		if err != nil {
			return nil, fmt.Errorf("failed to parse default gateway: %w", err)
		}

		return gatewayIP, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse /proc/net/route: %w", err)
	}

	return nil, fmt.Errorf("default gateway not found")
}

// parseHexIP converts an 8-character little-endian hex string into an IPv4 address.
func parseHexIP(hexIP string) (net.IP, error) {
	if len(hexIP) != 8 {
		return nil, fmt.Errorf("invalid hex IP length %d", len(hexIP))
	}

	var octets [4]byte
	for i := 0; i < 4; i++ {
		part := hexIP[i*2 : i*2+2]
		val, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hex octet %q: %w", part, err)
		}
		octets[3-i] = byte(val) // /proc/net/route stores little-endian.
	}

	return net.IPv4(octets[0], octets[1], octets[2], octets[3]), nil
}

// tryCommonSubnets returns common private IPv4 subnets as a conservative fallback.
func tryCommonSubnets() []net.IPNet {
	commonCIDRs := []string{
		"192.168.1.0/24",
		"192.168.0.0/24",
		"10.0.0.0/24",
		"172.16.0.0/24",
		"192.168.2.0/24",
	}

	var subnets []net.IPNet
	for _, cidr := range commonCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		subnets = append(subnets, *network)
	}

	return subnets
}

// addFallbackSubnets appends fallback subnet phases and updates confidence.
func addFallbackSubnets(profile *EnvironmentProfile) (*EnvironmentProfile, error) {
	fallback := tryCommonSubnets()
	if len(fallback) == 0 {
		return profile, fmt.Errorf("no fallback subnets available")
	}

	profile.Phases = append(profile.Phases, SubnetPhase{
		Name:       "fallback_common_subnets",
		Subnets:    fallback,
		Confidence: 0.3,
		Priority:   10,
	})

	if profile.Confidence == 0.0 {
		profile.Confidence = 0.3
	}

	profile.Warnings = append(profile.Warnings, "Using fallback private subnets; consider manual subnet configuration")
	return profile, nil
}

// countKernelRoutes parses /proc/net/route and returns the number of route entries.
func countKernelRoutes() (int, string) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return 0, fmt.Sprintf("Unable to read /proc/net/route: %v", err)
	}

	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Iface") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Sprintf("Error scanning /proc/net/route: %v", err)
	}

	return count, ""
}

// profileWithWarning appends a warning and returns the same profile for chaining.
func profileWithWarning(profile *EnvironmentProfile, warning string) *EnvironmentProfile {
	if warning != "" {
		profile.Warnings = append(profile.Warnings, warning)
	}
	return profile
}
