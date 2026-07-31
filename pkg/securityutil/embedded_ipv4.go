package securityutil

import "net"

// EmbeddedIPv4Candidates returns every IPv4 destination that an IPv6 transition
// address can encode.
//
// SSRF guards are built out of the net.IP predicates - IsLoopback, IsPrivate,
// IsLinkLocalUnicast and friends - and every one of them inspects the literal
// 16 bytes it is handed. An IPv6 transition address carries an IPv4 destination
// somewhere inside those bytes, so a NAT64 address such as 64:ff9b::a9fe:a9fe
// reaches 169.254.169.254 while reporting itself as an ordinary global unicast
// address. Callers must run each returned candidate through the same policy
// they applied to the outer address.
//
// Plain IPv4 and IPv4-mapped addresses (::ffff:a.b.c.d) return no candidates:
// net.IP.To4 already normalises those, so the stdlib predicates see the real
// destination without help.
//
// Candidates in 0.0.0.0/8 are omitted. They are not routable destinations, and
// including them would make every ::/128 and ::1 look like a transition
// address when the unspecified and loopback checks already cover those.
func EmbeddedIPv4Candidates(ip net.IP) []net.IP {
	v6 := ip.To16()
	if v6 == nil || ip.To4() != nil {
		return nil
	}

	var candidates []net.IP
	add := func(a, b, c, d byte) {
		if a == 0 {
			return
		}
		embedded := net.IPv4(a, b, c, d)
		for _, existing := range candidates {
			if existing.Equal(embedded) {
				return
			}
		}
		candidates = append(candidates, embedded)
	}

	switch {
	case isZeroIPBytes(v6[0:12]):
		// RFC 4291 IPv4-compatible address (::a.b.c.d). Deprecated, still parsed.
		add(v6[12], v6[13], v6[14], v6[15])
	case isZeroIPBytes(v6[0:8]) && v6[8] == 0xff && v6[9] == 0xff && isZeroIPBytes(v6[10:12]):
		// RFC 2765 IPv4-translated address (::ffff:0:a.b.c.d).
		add(v6[12], v6[13], v6[14], v6[15])
	case v6[0] == 0x00 && v6[1] == 0x64 && v6[2] == 0xff && v6[3] == 0x9b:
		if isZeroIPBytes(v6[4:12]) {
			// RFC 6052 Well-Known Prefix 64:ff9b::/96.
			add(v6[12], v6[13], v6[14], v6[15])
		}
		if v6[4] == 0x00 && v6[5] == 0x01 {
			// RFC 8215 Local-Use Prefix 64:ff9b:1::/48. Operators pick the
			// embedding length, so every RFC 6052 layout that fits under a /48
			// is a candidate. Byte 8 is the reserved u octet and is skipped.
			add(v6[6], v6[7], v6[9], v6[10])   // /48
			add(v6[7], v6[9], v6[10], v6[11])  // /56
			add(v6[9], v6[10], v6[11], v6[12]) // /64
			add(v6[12], v6[13], v6[14], v6[15])
		}
	case v6[0] == 0x20 && v6[1] == 0x02:
		// RFC 3056 6to4 (2002::/16) embeds the gateway IPv4 address.
		add(v6[2], v6[3], v6[4], v6[5])
	case v6[0] == 0x20 && v6[1] == 0x01 && v6[2] == 0x00 && v6[3] == 0x00:
		// RFC 4380 Teredo (2001::/32) carries the server IPv4 in bytes 4-7 and
		// the client IPv4 in bytes 12-15, obfuscated as its ones' complement.
		add(v6[4], v6[5], v6[6], v6[7])
		add(^v6[12], ^v6[13], ^v6[14], ^v6[15])
	}

	// RFC 5214 ISATAP interface identifiers sit under an arbitrary /64, so this
	// is checked independently of the prefix cases above.
	if (v6[8] == 0x00 || v6[8] == 0x02) && v6[9] == 0x00 && v6[10] == 0x5e && v6[11] == 0xfe {
		add(v6[12], v6[13], v6[14], v6[15])
	}

	return candidates
}

func isZeroIPBytes(b []byte) bool {
	for _, octet := range b {
		if octet != 0 {
			return false
		}
	}
	return true
}
