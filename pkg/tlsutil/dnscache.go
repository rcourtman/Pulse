package tlsutil

import (
	"context"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/rs/dnscache"
	"github.com/rs/zerolog/log"
)

var (
	// Global DNS resolver with caching
	globalResolver     *dnscache.Resolver
	globalResolverOnce sync.Once
	resolverMutex      sync.RWMutex
	resolverRefreshTTL time.Duration = 5 * time.Minute // Default TTL
)

// GetDNSResolver returns the global DNS resolver instance with caching
func GetDNSResolver() *dnscache.Resolver {
	globalResolverOnce.Do(func() {
		initDNSResolver(resolverRefreshTTL)
	})
	return globalResolver
}

// initDNSResolver initializes the DNS resolver with the specified TTL
func initDNSResolver(ttl time.Duration) {
	log.Info().
		Dur("ttl", ttl).
		Msg("Initializing DNS resolver cache to reduce DNS query load")

	globalResolver = &dnscache.Resolver{}

	// Start a goroutine to periodically refresh the DNS cache
	// This prevents stale DNS entries while still providing caching benefits
	go func() {
		ticker := time.NewTicker(ttl)
		defer ticker.Stop()

		for range ticker.C {
			globalResolver.Refresh(true)
			log.Debug().
				Dur("ttl", ttl).
				Msg("DNS cache refreshed")
		}
	}()
}

// SetDNSCacheTTL updates the DNS cache TTL
// This function should be called before any HTTP clients are created
func SetDNSCacheTTL(ttl time.Duration) {
	resolverMutex.Lock()
	defer resolverMutex.Unlock()

	if ttl <= 0 {
		ttl = 5 * time.Minute // Default
	}

	resolverRefreshTTL = ttl

	log.Info().
		Dur("ttl", ttl).
		Msg("DNS cache TTL configured")
}

// DialContextWithCache is a DialContext function that uses the DNS cache.
// On macOS, connections to RFC 1918 addresses are routed through a subprocess
// (nc) to bypass VPN/NECP routing captures that affect the host process.
func DialContextWithCache(ctx context.Context, network, address string) (net.Conn, error) {
	resolver := GetDNSResolver()

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	// Look up the IP address using the cached resolver
	ips, err := resolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	// Use the first IP address
	if len(ips) == 0 {
		return nil, &net.DNSError{
			Err:  "no IP addresses found",
			Name: host,
		}
	}

	resolvedIP := net.ParseIP(ips[0])
	resolvedAddr := net.JoinHostPort(ips[0], port)

	// On macOS, route local-network connections through a subprocess to bypass
	// Tailscale NECP routing captures that prevent the host process from
	// reaching RFC 1918 addresses directly.
	if runtime.GOOS == "darwin" && isPrivateNonVPN(resolvedIP) {
		return dialViaSubprocess(ctx, network, resolvedAddr)
	}

	// Create a dialer with the resolved IP
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Dial with the resolved IP address
	return dialer.DialContext(ctx, network, resolvedAddr)
}

// isPrivateNonVPN returns true for RFC 1918 private IPv4 addresses, excluding
// the Tailscale CGNAT range 100.64.0.0/10 so those connections still route
// through the Tailscale tunnel.
func isPrivateNonVPN(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return false
	}
	return ip4[0] == 10 ||
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
		(ip4[0] == 192 && ip4[1] == 168)
}
