package envdetect

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"
)

func TestDetectContainer_SystemdDetectVirtDocker(t *testing.T) {
	probe := fakeEnvironmentProbe{
		lookPathPresent: map[string]bool{"systemd-detect-virt": true},
		commandOutput: map[string][]byte{
			"systemd-detect-virt\x00--container": []byte("docker\n"),
		},
	}

	isContainer, containerType := detectContainer(probe)
	if !isContainer {
		t.Fatalf("expected container, got non-container")
	}
	if containerType != "docker" {
		t.Fatalf("containerType = %q, want docker", containerType)
	}
}

func TestDetectContainer_SystemdDetectVirtUnknown(t *testing.T) {
	probe := fakeEnvironmentProbe{
		lookPathPresent: map[string]bool{"systemd-detect-virt": true},
		commandOutput: map[string][]byte{
			"systemd-detect-virt\x00--container": []byte("rkt\n"),
		},
	}

	isContainer, containerType := detectContainer(probe)
	if !isContainer {
		t.Fatalf("expected container, got non-container")
	}
	if containerType != "rkt" {
		t.Fatalf("containerType = %q, want rkt", containerType)
	}
}

func TestDetectContainer_MarkersAndCgroup(t *testing.T) {
	t.Run("marker-containerenv", func(t *testing.T) {
		probe := fakeEnvironmentProbe{
			statPresent: map[string]bool{"/run/.containerenv": true},
		}

		isContainer, containerType := detectContainer(probe)
		if !isContainer || containerType != "docker" {
			t.Fatalf("marker detection = (%v, %q), want (true, docker)", isContainer, containerType)
		}
	})

	t.Run("cgroup-docker", func(t *testing.T) {
		probe := fakeEnvironmentProbe{
			fileData: map[string][]byte{
				"/proc/1/cgroup": []byte("12:cpu:/docker/abc\n"),
			},
		}

		isContainer, containerType := detectContainer(probe)
		if !isContainer || containerType != "docker" {
			t.Fatalf("cgroup detection = (%v, %q), want (true, docker)", isContainer, containerType)
		}
	})

	t.Run("environ-lxc", func(t *testing.T) {
		probe := fakeEnvironmentProbe{
			fileErr: map[string]error{
				"/proc/1/cgroup": errors.New("nope"),
			},
			fileData: map[string][]byte{
				"/proc/1/environ": []byte("container=lxc\x00"),
			},
		}

		isContainer, containerType := detectContainer(probe)
		if !isContainer || containerType != "lxc" {
			t.Fatalf("environ detection = (%v, %q), want (true, lxc)", isContainer, containerType)
		}
	})
}

func TestDetectContainer_NoMarkers(t *testing.T) {
	probe := fakeEnvironmentProbe{
		fileErr: map[string]error{
			"/proc/1/cgroup":  os.ErrNotExist,
			"/proc/1/environ": os.ErrNotExist,
		},
	}

	isContainer, containerType := detectContainer(probe)
	if isContainer {
		t.Fatalf("expected non-container, got container type %q", containerType)
	}
	if containerType != "" {
		t.Fatalf("containerType = %q, want empty", containerType)
	}
}

func TestDetectNativeEnvironment_FallbackOnError(t *testing.T) {
	profile := &EnvironmentProfile{
		Policy:   DefaultScanPolicy(),
		Metadata: map[string]string{},
	}
	probe := fakeEnvironmentProbe{interfacesErr: errors.New("boom")}

	result, err := detectNativeEnvironment(profile, probe)
	if err != nil {
		t.Fatalf("detectNativeEnvironment returned error: %v", err)
	}
	if len(result.Phases) == 0 || result.Phases[0].Name != "fallback_common_subnets" {
		t.Fatalf("expected fallback subnets, got %#v", result.Phases)
	}

	found := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "Failed to enumerate interfaces") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning about interface enumeration, got %#v", result.Warnings)
	}
}

func TestDetectNativeEnvironment_NoSubnetsFallback(t *testing.T) {
	profile := &EnvironmentProfile{
		Policy:   DefaultScanPolicy(),
		Metadata: map[string]string{},
	}
	probe := fakeEnvironmentProbe{
		interfaces: []ifaceInfo{
			{
				Name:  "lo",
				Flags: net.FlagUp | net.FlagLoopback,
				Addrs: []net.Addr{
					&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)},
				},
			},
			{
				Name:  "eth0",
				Flags: net.FlagUp,
				Addrs: []net.Addr{
					&net.IPNet{IP: net.IPv4(169, 254, 1, 2), Mask: net.CIDRMask(16, 32)},
				},
			},
		},
	}

	result, err := detectNativeEnvironment(profile, probe)
	if err != nil {
		t.Fatalf("detectNativeEnvironment returned error: %v", err)
	}
	if len(result.Phases) == 0 || result.Phases[0].Name != "fallback_common_subnets" {
		t.Fatalf("expected fallback subnets, got %#v", result.Phases)
	}

	found := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "No active IPv4 interfaces found") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning about missing interfaces, got %#v", result.Warnings)
	}
}

func TestGetAllLocalSubnets_DedupAndSkip(t *testing.T) {
	probe := fakeEnvironmentProbe{
		interfaces: []ifaceInfo{
			{
				Name:  "lo",
				Flags: net.FlagUp | net.FlagLoopback,
				Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}},
			},
			{
				Name:  "down0",
				Flags: 0,
				Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(10, 0, 0, 2), Mask: net.CIDRMask(24, 32)}},
			},
			{
				Name:     "err0",
				Flags:    net.FlagUp,
				AddrsErr: errors.New("boom"),
			},
			{
				Name:  "v6",
				Flags: net.FlagUp,
				Addrs: []net.Addr{&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}},
			},
			{
				Name:  "eth0",
				Flags: net.FlagUp,
				Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(192, 168, 50, 10), Mask: net.CIDRMask(24, 32)}},
			},
			{
				Name:  "eth1",
				Flags: net.FlagUp,
				Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(192, 168, 50, 20), Mask: net.CIDRMask(24, 32)}},
			},
			{
				Name:  "linklocal",
				Flags: net.FlagUp,
				Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(169, 254, 10, 2), Mask: net.CIDRMask(16, 32)}},
			},
		},
	}

	subnets, err := getAllLocalSubnets(probe)
	if err != nil {
		t.Fatalf("getAllLocalSubnets returned error: %v", err)
	}
	if len(subnets) != 1 {
		t.Fatalf("subnets = %#v, want 1 unique subnet", subnets)
	}
	if got := subnets[0].String(); got != mustIPNet(t, "192.168.50.0/24").String() {
		t.Fatalf("subnet = %q, want 192.168.50.0/24", got)
	}
}

func TestIsDockerHostMode_InterfaceError(t *testing.T) {
	probe := fakeEnvironmentProbe{interfacesErr: errors.New("boom")}

	hostMode, warnings := isDockerHostMode(probe)
	if hostMode {
		t.Fatalf("expected hostMode=false on error")
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "Unable to enumerate interfaces") {
		t.Fatalf("warnings = %#v, want interface enumeration warning", warnings)
	}
}

func TestDetectDockerEnvironment_BridgeFallback(t *testing.T) {
	profile := &EnvironmentProfile{
		Policy:   DefaultScanPolicy(),
		Metadata: map[string]string{},
	}
	profile.Policy.ScanGateways = false

	probe := fakeEnvironmentProbe{
		fileErr: map[string]error{
			"/proc/net/route": errors.New("nope"),
		},
		interfaces: []ifaceInfo{
			{
				Name:  "lo",
				Flags: net.FlagUp | net.FlagLoopback,
				Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}},
			},
		},
	}

	result, err := detectDockerEnvironment(profile, probe)
	if err != nil {
		t.Fatalf("detectDockerEnvironment returned error: %v", err)
	}
	if result.Type != DockerBridge {
		t.Fatalf("Type = %v, want %v", result.Type, DockerBridge)
	}
	if len(result.Phases) == 0 || result.Phases[0].Name != "fallback_common_subnets" {
		t.Fatalf("expected fallback subnets, got %#v", result.Phases)
	}
}

func TestDetectHostNetworkFromContainer_NonStandardGateway(t *testing.T) {
	route := strings.Join([]string{
		"Iface\tDestination\tGateway",
		"eth0\t00000000\t2A00000A",
	}, "\n")

	probe := fakeEnvironmentProbe{
		fileData: map[string][]byte{"/proc/net/route": []byte(route)},
	}

	subnets, confidence, warnings := detectHostNetworkFromContainer(probe)
	if confidence != 0.4 {
		t.Fatalf("confidence = %v, want 0.4", confidence)
	}
	if len(subnets) != 1 || subnets[0].String() != "10.0.0.0/24" {
		t.Fatalf("subnets = %#v, want 10.0.0.0/24", subnets)
	}
	found := false
	for _, warn := range warnings {
		if strings.Contains(warn, "does not end with .1 or .254") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings = %#v, want non-standard gateway warning", warnings)
	}
}

func TestDetectHostNetworkFromContainer_GatewayErrorFallback(t *testing.T) {
	probe := fakeEnvironmentProbe{
		fileErr: map[string]error{"/proc/net/route": errors.New("boom")},
	}

	subnets, confidence, warnings := detectHostNetworkFromContainer(probe)
	if confidence != 0.3 {
		t.Fatalf("confidence = %v, want 0.3", confidence)
	}
	if len(subnets) == 0 {
		t.Fatalf("expected fallback subnets, got none")
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "Could not determine default gateway") {
		t.Fatalf("warnings = %#v, want gateway warning", warnings)
	}
}

func TestGetDefaultGateway_InvalidHex(t *testing.T) {
	route := strings.Join([]string{
		"Iface\tDestination\tGateway",
		"eth0\t00000000\tZZZZZZZZ",
	}, "\n")

	probe := fakeEnvironmentProbe{
		fileData: map[string][]byte{"/proc/net/route": []byte(route)},
	}

	_, err := getDefaultGateway(probe)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse default gateway") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountKernelRoutes_ReadFileError(t *testing.T) {
	probe := fakeEnvironmentProbe{
		fileErr: map[string]error{"/proc/net/route": errors.New("boom")},
	}

	count, warn := countKernelRoutes(probe)
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
	if warn == "" || !strings.Contains(warn, "Unable to read /proc/net/route") {
		t.Fatalf("warn = %q, want read error warning", warn)
	}
}

func TestIsLXCPrivileged_ErrorPaths(t *testing.T) {
	t.Run("permission", func(t *testing.T) {
		probe := fakeEnvironmentProbe{
			fileErr: map[string]error{"/proc/self/uid_map": os.ErrPermission},
		}
		privileged, warn := isLXCPrivileged(probe)
		if privileged {
			t.Fatalf("expected unprivileged on permission error")
		}
		if !strings.Contains(warn, "permission denied") {
			t.Fatalf("warn = %q, want permission warning", warn)
		}
	})

	t.Run("bad-format", func(t *testing.T) {
		probe := fakeEnvironmentProbe{
			fileData: map[string][]byte{"/proc/self/uid_map": []byte("0 0\n")},
		}
		privileged, warn := isLXCPrivileged(probe)
		if privileged {
			t.Fatalf("expected unprivileged on format error")
		}
		if !strings.Contains(warn, "Unexpected format") {
			t.Fatalf("warn = %q, want format warning", warn)
		}
	})

	t.Run("bad-hostuid", func(t *testing.T) {
		probe := fakeEnvironmentProbe{
			fileData: map[string][]byte{"/proc/self/uid_map": []byte("0 x 1\n")},
		}
		privileged, warn := isLXCPrivileged(probe)
		if privileged {
			t.Fatalf("expected unprivileged on parse error")
		}
		if !strings.Contains(warn, "Failed to parse uid_map") {
			t.Fatalf("warn = %q, want parse warning", warn)
		}
	})

	t.Run("bad-length", func(t *testing.T) {
		probe := fakeEnvironmentProbe{
			fileData: map[string][]byte{"/proc/self/uid_map": []byte("0 0 nope\n")},
		}
		privileged, warn := isLXCPrivileged(probe)
		if privileged {
			t.Fatalf("expected unprivileged on length parse error")
		}
		if !strings.Contains(warn, "Failed to parse uid_map length") {
			t.Fatalf("warn = %q, want length warning", warn)
		}
	})
}

func TestDetectEnvironment_UnsupportedContainerType(t *testing.T) {
	probe := fakeEnvironmentProbe{
		lookPathPresent: map[string]bool{"systemd-detect-virt": true},
		commandOutput: map[string][]byte{
			"systemd-detect-virt\x00--container": []byte("rkt\n"),
		},
	}

	profile, err := detectEnvironment(probe)
	if err != nil {
		t.Fatalf("detectEnvironment returned error: %v", err)
	}
	if profile.Type != Unknown {
		t.Fatalf("Type = %v, want %v", profile.Type, Unknown)
	}
	if profile.Metadata["container_type"] != "rkt" {
		t.Fatalf("container_type = %q, want rkt", profile.Metadata["container_type"])
	}
	if len(profile.Phases) == 0 || profile.Phases[0].Name != "fallback_common_subnets" {
		t.Fatalf("expected fallback subnets, got %#v", profile.Phases)
	}
}

func TestDetectLXCEnvironment_UnprivilegedFallback(t *testing.T) {
	profile := &EnvironmentProfile{
		Policy:   DefaultScanPolicy(),
		Metadata: map[string]string{},
	}
	profile.Policy.ScanGateways = false

	probe := fakeEnvironmentProbe{
		fileData: map[string][]byte{"/proc/self/uid_map": []byte("0 100000 65536\n")},
		interfaces: []ifaceInfo{
			{
				Name:  "lo",
				Flags: net.FlagUp | net.FlagLoopback,
				Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}},
			},
		},
	}

	result, err := detectLXCEnvironment(profile, probe)
	if err != nil {
		t.Fatalf("detectLXCEnvironment returned error: %v", err)
	}
	if result.Type != LXCUnprivileged {
		t.Fatalf("Type = %v, want %v", result.Type, LXCUnprivileged)
	}
	if len(result.Phases) == 0 || result.Phases[0].Name != "fallback_common_subnets" {
		t.Fatalf("expected fallback subnets, got %#v", result.Phases)
	}
}
