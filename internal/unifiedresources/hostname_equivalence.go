package unifiedresources

import "strings"

func normalizeComparableHostname(hostname string) string {
	host := strings.TrimSpace(strings.ToLower(hostname))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return ""
	}
	if NormalizeIP(host) != "" {
		return ""
	}
	return host
}

// HostnamesEquivalent reports whether two hostnames refer to the same host
// when one side may use the short hostname and the other the fully-qualified
// hostname. Distinct fully-qualified hostnames remain distinct even when they
// share the same short name.
func HostnamesEquivalent(a, b string) bool {
	left := normalizeComparableHostname(a)
	right := normalizeComparableHostname(b)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}

	leftShort := NormalizeHostname(left)
	rightShort := NormalizeHostname(right)
	if leftShort == "" || leftShort != rightShort {
		return false
	}

	return left == leftShort || right == rightShort
}
