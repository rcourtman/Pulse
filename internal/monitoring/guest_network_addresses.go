package monitoring

import (
	"net/netip"
	"sort"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/netutil"
)

// guestIPAddressesByInterface keeps interface/address association when choosing
// the first displayed guest address. The existing name-ordered interface view
// remains unchanged; local-container/overlay addresses follow other interfaces.
// Scalar addresses with no known interface remain useful, ahead of secondary
// addresses. No address is dropped and no route or reachability is inferred.
func guestIPAddressesByInterface(addresses []string, interfaces []models.GuestNetworkInterface) []string {
	if len(addresses) < 2 {
		return addresses
	}
	remaining := make(map[string]bool, len(addresses))
	for _, address := range addresses {
		remaining[address] = true
	}
	associated := make(map[string]bool)
	for _, iface := range interfaces {
		for _, address := range iface.Addresses {
			associated[address] = true
		}
	}
	ordered := make([]string, 0, len(addresses))
	appendAddress := func(address string) {
		if remaining[address] {
			ordered = append(ordered, address)
			delete(remaining, address)
		}
	}
	// Callers have sorted interface names for stable presentation. Sort only
	// within each interface, never the flattened list (#2757).
	for pass := 0; pass < 2; pass++ {
		for _, iface := range interfaces {
			if (pass == 0) == netutil.IsSecondaryInterfaceName(iface.Name) {
				continue
			}
			local := cloneStringSlice(iface.Addresses)
			sortGuestAddresses(local)
			for _, address := range local {
				appendAddress(address)
			}
		}
		if pass == 0 {
			unassociated := cloneStringSlice(addresses)
			sortGuestAddresses(unassociated)
			for _, address := range unassociated {
				if !associated[address] {
					appendAddress(address)
				}
			}
		}
	}
	return ordered
}

// sortGuestAddresses orders valid IPs numerically (IPv4 before IPv6). Existing
// unparsed metadata is retained with a deterministic text fallback, not silently
// turned into a valid or reachable address.
func sortGuestAddresses(addresses []string) {
	sort.SliceStable(addresses, func(i, j int) bool {
		left, leftErr := netip.ParseAddr(addresses[i])
		right, rightErr := netip.ParseAddr(addresses[j])
		if leftErr == nil && rightErr == nil {
			if comparison := left.Compare(right); comparison != 0 {
				return comparison < 0
			}
		} else if (leftErr == nil) != (rightErr == nil) {
			return leftErr == nil
		}
		return addresses[i] < addresses[j]
	})
}
