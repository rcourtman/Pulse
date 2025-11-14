package main

import "strings"

// Capability represents a permission bit granted to a peer.
type Capability uint32

const (
	CapabilityRead Capability = 1 << iota
	CapabilityWrite
	CapabilityAdmin
	capabilityLegacyAll = CapabilityRead | CapabilityWrite | CapabilityAdmin
)

func (c Capability) Has(flag Capability) bool {
	return c&flag == flag
}

func parseCapabilityList(values []string) Capability {
	if len(values) == 0 {
		return CapabilityRead
	}
	var caps Capability
	for _, raw := range values {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "read":
			caps |= CapabilityRead
		case "write":
			caps |= CapabilityWrite
		case "admin":
			caps |= CapabilityAdmin
		}
	}
	return caps
}

func capabilityNames(c Capability) []string {
	names := make([]string, 0, 3)
	if c.Has(CapabilityRead) {
		names = append(names, "read")
	}
	if c.Has(CapabilityWrite) {
		names = append(names, "write")
	}
	if c.Has(CapabilityAdmin) {
		names = append(names, "admin")
	}
	return names
}
