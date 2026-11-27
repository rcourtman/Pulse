package monitoring

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestHostActivePeersFromReport(t *testing.T) {
	reported := []agentshost.NetworkPeer{
		// Valid outbound RFC1918
		{LocalIP: "192.168.0.10", RemoteIP: "192.168.0.20", RemotePort: 5432, Protocol: "tcp"},
		// Duplicate outbound should dedupe
		{LocalIP: "192.168.0.10", RemoteIP: "192.168.0.20", RemotePort: 5432, Protocol: "tcp"},
		// Valid Tailscale CGNAT outbound
		{LocalIP: "100.64.0.1", RemoteIP: "100.64.0.2", RemotePort: 445, Protocol: "tcp"},
		// Valid inbound
		{LocalIP: "192.168.0.10", LocalPort: 2049, RemoteIP: "192.168.0.30", Protocol: "tcp", Inbound: true},
		// Public IP must be rejected on server defense-in-depth
		{LocalIP: "192.168.0.10", RemoteIP: "8.8.8.8", RemotePort: 443, Protocol: "tcp"},
		// Loopback must be rejected
		{LocalIP: "127.0.0.1", RemoteIP: "127.0.0.1", RemotePort: 8080, Protocol: "tcp"},
		// Zero port must be rejected
		{LocalIP: "192.168.0.10", RemoteIP: "192.168.0.40", RemotePort: 0, Protocol: "tcp"},
		// Self-IP must be rejected
		{LocalIP: "192.168.0.10", RemoteIP: "192.168.0.10", RemotePort: 8080, Protocol: "tcp"},
	}

	got := hostActivePeersFromReport(reported)
	want := []models.HostNetworkPeer{
		{LocalIP: "100.64.0.1", RemoteIP: "100.64.0.2", RemotePort: 445, Protocol: "tcp"},
		{LocalIP: "192.168.0.10", RemoteIP: "192.168.0.20", RemotePort: 5432, Protocol: "tcp"},
		{LocalIP: "192.168.0.10", LocalPort: 2049, RemoteIP: "192.168.0.30", Protocol: "tcp", Inbound: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hostActivePeersFromReport() =\n%#v\nwant:\n%#v", got, want)
	}
}

func TestDockerAcceleratorsFromReport(t *testing.T) {
	got := dockerAcceleratorsFromReport([]string{" iGPU ", "GPU", "GPU", "bad\nlabel", ""})
	want := []string{"GPU", "iGPU"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dockerAcceleratorsFromReport() = %v, want %v", got, want)
	}
	if got := dockerAcceleratorsFromReport(nil); got != nil {
		t.Fatalf("expected nil for empty input, got %v", got)
	}
}

func TestParseProxmoxVMConfigAccelerators(t *testing.T) {
	cfg := map[string]interface{}{
		"net0":     "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
		"hostpci0": "0000:01:00.0,pcie=1,x-vga=1",
		"hostpci1": "mapping=coral-tpu",
		"hostpci2": "0000:03:00.0,pcie=1",
	}
	got := parseProxmoxVMConfigAccelerators(cfg)
	want := []string{"GPU", "PCIe", "TPU"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseProxmoxVMConfigAccelerators() = %v, want %v", got, want)
	}
}

func TestParseProxmoxLXCConfigAccelerators(t *testing.T) {
	cfg := map[string]interface{}{
		"dev0": "/dev/dri/renderD128,gid=104",
		"dev1": "/dev/apex_0",
		"lxc": []interface{}{
			[]interface{}{"lxc.mount.entry", "/dev/nvidia0 dev/nvidia0 none bind,optional,create=file"},
		},
	}
	got := parseProxmoxLXCConfigAccelerators(cfg)
	want := []string{"GPU", "TPU", "iGPU"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseProxmoxLXCConfigAccelerators() = %v, want %v", got, want)
	}
}
