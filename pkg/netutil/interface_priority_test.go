package netutil

import "testing"

func TestSecondaryInterfaceNamesAreOnlyDisplayHints(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "lo", "docker0", "podman0", " PODMAN1 ", "veth123", "br-abc", "cni0", "flannel.1", "virbr0", "ztabc"} {
		t.Run("secondary:"+name, func(t *testing.T) {
			if !IsSecondaryInterfaceName(name) {
				t.Fatalf("expected secondary name %q", name)
			}
		})
	}
	for _, name := range []string{"eth0", "ens18", "Ethernet", "wlp2s0", "br0", "vmbr0", "bond0", "vlan100", "eth0.100", "tun0", "wg0", "tailscale0"} {
		t.Run("management:"+name, func(t *testing.T) {
			if IsSecondaryInterfaceName(name) {
				t.Fatalf("management interface %q must not be classified as local-container/overlay", name)
			}
		})
	}
}
