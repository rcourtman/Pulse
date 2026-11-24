package securityutil

import "net"

// IsSpecialUseInternalIP reports whether ip lies in an IPv4 special-use range
// that net.IP.IsPrivate does not cover but that is never a public destination.
// IsPrivate answers for RFC 1918 and fc00::/7 only, and IsGlobalUnicast reports
// every range below as ordinary unicast, yet shared address space carries
// overlay networks such as Tailscale, carrier NAT and a cloud metadata service
// (100.100.100.200), and the IETF protocol block carries another (192.0.0.192).
// SSRF guards must hold these ranges to the policy they apply to private
// addresses.
func IsSpecialUseInternalIP(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}

	switch {
	case v4[0] == 0: // 0.0.0.0/8 "this" network (RFC 1122)
	case isCarrierGradeNATIPv4(v4): // 100.64.0.0/10 shared address space (RFC 6598)
	case v4[0] == 192 && v4[1] == 0 && v4[2] == 0: // 192.0.0.0/24 IETF protocol assignments (RFC 6890)
	case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19): // 198.18.0.0/15 benchmarking (RFC 2544)
	case v4[0] >= 240: // 240.0.0.0/4 reserved (RFC 1112)
	default:
		return false
	}
	return true
}
