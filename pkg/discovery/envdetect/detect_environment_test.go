package envdetect

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string      { return "fake" }
func (fakeFileInfo) Size() int64       { return 0 }
func (fakeFileInfo) Mode() os.FileMode { return 0 }
func (fakeFileInfo) ModTime() (t time.Time) {
	return t
}
func (fakeFileInfo) IsDir() bool      { return false }
func (fakeFileInfo) Sys() interface{} { return nil }

type fakeEnvironmentProbe struct {
	lookPathPresent map[string]bool
	commandOutput   map[string][]byte
	commandErr      map[string]error
	fileData        map[string][]byte
	fileErr         map[string]error
	statPresent     map[string]bool
	interfaces      []ifaceInfo
	interfacesErr   error
}

func (p fakeEnvironmentProbe) LookPath(file string) (string, error) {
	if p.lookPathPresent[file] {
		return "/usr/bin/" + file, nil
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

func (p fakeEnvironmentProbe) CommandCombinedOutput(name string, args ...string) ([]byte, error) {
	key := name + "\x00" + strings.Join(args, "\x00")
	if out, ok := p.commandOutput[key]; ok {
		return out, p.commandErr[key]
	}
	return nil, errors.New("unexpected command invocation")
}

func (p fakeEnvironmentProbe) Stat(name string) (os.FileInfo, error) {
	if p.statPresent[name] {
		return fakeFileInfo{}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

func (p fakeEnvironmentProbe) ReadFile(name string) ([]byte, error) {
	if err, ok := p.fileErr[name]; ok && err != nil {
		return nil, err
	}
	if data, ok := p.fileData[name]; ok {
		return data, nil
	}
	return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
}

func (p fakeEnvironmentProbe) Interfaces() ([]ifaceInfo, error) {
	if p.interfacesErr != nil {
		return nil, p.interfacesErr
	}
	return p.interfaces, nil
}

func mustIPNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("net.ParseCIDR(%q): %v", cidr, err)
	}
	return network
}

func TestDetectEnvironment_Native(t *testing.T) {
	probe := fakeEnvironmentProbe{
		interfaces: []ifaceInfo{
			{
				Name:  "eth0",
				Flags: net.FlagUp,
				Addrs: []net.Addr{
					&net.IPNet{IP: net.IPv4(192, 168, 1, 10), Mask: net.CIDRMask(24, 32)},
				},
			},
		},
	}

	profile, err := detectEnvironment(probe)
	if err != nil {
		t.Fatalf("detectEnvironment returned error: %v", err)
	}
	if profile.Type != Native {
		t.Fatalf("Type = %v, want %v", profile.Type, Native)
	}
	if profile.Metadata["container_detected"] != "false" {
		t.Fatalf("container_detected = %q, want false", profile.Metadata["container_detected"])
	}
	if len(profile.Phases) != 1 || profile.Phases[0].Name != "local_networks" {
		t.Fatalf("Phases = %#v, want single local_networks phase", profile.Phases)
	}
	if got := profile.Phases[0].Subnets[0].String(); got != mustIPNet(t, "192.168.1.0/24").String() {
		t.Fatalf("subnet = %q, want %q", got, "192.168.1.0/24")
	}
}

func TestDetectEnvironment_DockerHostMode(t *testing.T) {
	route := strings.Join([]string{
		"Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT",
		"lo\t00000000\t00000000\t0003\t0\t0\t0\t00000000\t0\t0\t0",
		"eth0\t00000000\t010011AC\t0003\t0\t0\t0\t00000000\t0\t0\t0",
		"eth0\t000011AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0",
		"eth1\t00000000\t010011AC\t0003\t0\t0\t0\t00000000\t0\t0\t0",
		"eth2\t00000000\t010011AC\t0003\t0\t0\t0\t00000000\t0\t0\t0",
		"eth3\t00000000\t010011AC\t0003\t0\t0\t0\t00000000\t0\t0\t0",
	}, "\n")

	probe := fakeEnvironmentProbe{
		statPresent: map[string]bool{"/.dockerenv": true},
		fileData:    map[string][]byte{"/proc/net/route": []byte(route)},
		interfaces: []ifaceInfo{
			{Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
			{
				Name:  "eth0",
				Flags: net.FlagUp,
				Addrs: []net.Addr{
					&net.IPNet{IP: net.IPv4(10, 0, 0, 10), Mask: net.CIDRMask(24, 32)},
				},
			},
			{Name: "eth1", Flags: net.FlagUp},
			{Name: "eth2", Flags: net.FlagUp},
		},
	}

	profile, err := detectEnvironment(probe)
	if err != nil {
		t.Fatalf("detectEnvironment returned error: %v", err)
	}
	if profile.Type != DockerHost {
		t.Fatalf("Type = %v, want %v", profile.Type, DockerHost)
	}
	if profile.Metadata["docker_mode"] != "host" {
		t.Fatalf("docker_mode = %q, want host", profile.Metadata["docker_mode"])
	}
	if len(profile.Phases) != 1 || profile.Phases[0].Name != "host_networks" {
		t.Fatalf("Phases = %#v, want single host_networks phase", profile.Phases)
	}
	if got := profile.Phases[0].Subnets[0].String(); got != mustIPNet(t, "10.0.0.0/24").String() {
		t.Fatalf("subnet = %q, want %q", got, "10.0.0.0/24")
	}
}

func TestDetectEnvironment_DockerBridge_InferredHostNetwork(t *testing.T) {
	route := strings.Join([]string{
		"Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT",
		"eth0\t00000000\t010011AC\t0003\t0\t0\t0\t00000000\t0\t0\t0",
		"eth0\t000011AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0",
	}, "\n")

	probe := fakeEnvironmentProbe{
		statPresent: map[string]bool{"/.dockerenv": true},
		fileData:    map[string][]byte{"/proc/net/route": []byte(route)},
		interfaces: []ifaceInfo{
			{
				Name:  "eth0",
				Flags: net.FlagUp,
				Addrs: []net.Addr{
					&net.IPNet{IP: net.IPv4(172, 17, 0, 2), Mask: net.CIDRMask(16, 32)},
				},
			},
		},
	}

	profile, err := detectEnvironment(probe)
	if err != nil {
		t.Fatalf("detectEnvironment returned error: %v", err)
	}
	if profile.Type != DockerBridge {
		t.Fatalf("Type = %v, want %v", profile.Type, DockerBridge)
	}

	var containerFound, inferredFound bool
	for _, phase := range profile.Phases {
		switch phase.Name {
		case "container_network":
			containerFound = true
			if got := phase.Subnets[0].String(); got != mustIPNet(t, "172.17.0.0/16").String() {
				t.Fatalf("container subnet = %q, want %q", got, "172.17.0.0/16")
			}
		case "inferred_host_network":
			inferredFound = true
			if got := phase.Subnets[0].String(); got != mustIPNet(t, "172.17.0.0/24").String() {
				t.Fatalf("inferred subnet = %q, want %q", got, "172.17.0.0/24")
			}
			if phase.Confidence != 0.7 {
				t.Fatalf("inferred confidence = %v, want 0.7", phase.Confidence)
			}
		}
	}

	if !containerFound || !inferredFound {
		t.Fatalf("expected both container_network and inferred_host_network phases, got %#v", profile.Phases)
	}
}

func TestDetectEnvironment_LXCPrivileged_SystemdDetectVirt(t *testing.T) {
	probe := fakeEnvironmentProbe{
		lookPathPresent: map[string]bool{"systemd-detect-virt": true},
		commandOutput: map[string][]byte{
			"systemd-detect-virt\x00--container": []byte("lxc\n"),
		},
		fileData: map[string][]byte{
			"/proc/self/uid_map": []byte("0 0 4294967295\n"),
		},
		interfaces: []ifaceInfo{
			{
				Name:  "eth0",
				Flags: net.FlagUp,
				Addrs: []net.Addr{
					&net.IPNet{IP: net.IPv4(192, 168, 50, 10), Mask: net.CIDRMask(24, 32)},
				},
			},
		},
	}

	profile, err := detectEnvironment(probe)
	if err != nil {
		t.Fatalf("detectEnvironment returned error: %v", err)
	}
	if profile.Type != LXCPrivileged {
		t.Fatalf("Type = %v, want %v", profile.Type, LXCPrivileged)
	}
	if profile.Metadata["lxc_privileged"] != "true" {
		t.Fatalf("lxc_privileged = %q, want true", profile.Metadata["lxc_privileged"])
	}
	if len(profile.Phases) != 1 || profile.Phases[0].Name != "lxc_host_networks" {
		t.Fatalf("Phases = %#v, want single lxc_host_networks phase", profile.Phases)
	}
}

func TestGetDefaultGateway_DefaultRouteNotFound(t *testing.T) {
	route := strings.Join([]string{
		"Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT",
		"eth0\t000011AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0",
	}, "\n")

	probe := fakeEnvironmentProbe{
		fileData: map[string][]byte{"/proc/net/route": []byte(route)},
	}

	_, err := getDefaultGateway(probe)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "default gateway not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountKernelRoutes_SkipsHeaderAndBlankLines(t *testing.T) {
	route := strings.Join([]string{
		"Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT",
		"",
		"eth0\t00000000\t010011AC\t0003\t0\t0\t0\t00000000\t0\t0\t0",
		"eth0\t000011AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0",
		"   ",
	}, "\n")

	probe := fakeEnvironmentProbe{
		fileData: map[string][]byte{"/proc/net/route": []byte(route)},
	}

	count, warn := countKernelRoutes(probe)
	if warn != "" {
		t.Fatalf("warn = %q, want empty", warn)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}
