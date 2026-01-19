package hostagent

import "testing"

func TestIsLikelyVirtualInterfaceName(t *testing.T) {
	virtual := []string{"", "lo", "docker0", "veth123", "br-abc", "cni0", "flannel.1", "virbr0", "ztabc"}
	for _, name := range virtual {
		if !isLikelyVirtualInterfaceName(name) {
			t.Fatalf("expected %q to be virtual", name)
		}
	}

	if isLikelyVirtualInterfaceName("eth0") {
		t.Fatal("expected eth0 to be non-virtual")
	}
}
