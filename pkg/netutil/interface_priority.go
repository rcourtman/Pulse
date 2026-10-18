// Package netutil contains display-only network metadata helpers.
package netutil

import "strings"

// IsSecondaryInterfaceName recognises common local-container/overlay names.
// It is only an address-ordering hint: it proves neither a physical NIC nor a
// default route. Do not use it to discard interfaces or change machine identity.
// Management bridges, bonds, VLANs and tunnels are deliberately not excluded.
func IsSecondaryInterfaceName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "lo" {
		return true
	}
	for _, prefix := range []string{"docker", "podman", "veth", "br-", "cni", "flannel", "virbr", "zt"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
