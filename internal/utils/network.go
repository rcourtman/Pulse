package utils

import (
	"net"
	"strings"
)

// IsPrivateIP checks if an IP address is in private/local ranges (RFC1918)
func IsPrivateIP(ip string) bool {
	// Extract IP without port
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	// Remove brackets from IPv6
	ip = strings.Trim(ip, "[]")

	// Parse the IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check if it's loopback
	if parsedIP.IsLoopback() {
		return true
	}

	// Check if it's link-local
	if parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast() {
		return true
	}

	// Define private IP ranges (RFC1918)
	privateRanges := []string{
		"10.0.0.0/8",     // Class A private
		"172.16.0.0/12",  // Class B private
		"192.168.0.0/16", // Class C private
		"127.0.0.0/8",    // Loopback
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// GetClientIP extracts the real client IP from request headers
func GetClientIP(remoteAddr string, xForwardedFor string, xRealIP string) string {
	// Check X-Forwarded-For first (for proxies)
	if xForwardedFor != "" {
		// Take the first IP if there are multiple
		if idx := strings.Index(xForwardedFor, ","); idx != -1 {
			return strings.TrimSpace(xForwardedFor[:idx])
		}
		return strings.TrimSpace(xForwardedFor)
	}

	// Check X-Real-IP
	if xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// Fall back to RemoteAddr
	return remoteAddr
}

// IsTrustedNetwork checks if an IP is within trusted network ranges
func IsTrustedNetwork(ip string, trustedNetworks []string) bool {
	// If no trusted networks defined, consider all private IPs as trusted
	if len(trustedNetworks) == 0 {
		return IsPrivateIP(ip)
	}

	// Extract IP without port
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	ip = strings.Trim(ip, "[]")

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check against trusted networks
	for _, cidr := range trustedNetworks {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}
