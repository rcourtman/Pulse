package tlsutil

import (
	"context"
	"net"
	"net/netip"
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

// LookupHostCached resolves host through the same process-global cached
// resolver that DialContextWithCache uses, returning parsed addresses.
//
// Callers that reason about which addresses a connection will actually reach
// (endpoint policy checks, for example) must go through this rather than
// net.LookupIP, so the decision and the dial share one DNS view. The resolver
// caches both answers and lookup errors until the next refresh tick, so repeat
// callers cost a map lookup rather than a query (#1638).
func LookupHostCached(ctx context.Context, host string) ([]net.IP, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	addrs, err := GetDNSResolver().LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil {
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return nil, &net.DNSError{
			Err:  "no IP addresses found",
			Name: host,
		}
	}
	return ips, nil
}

// DialContextWithCache is a DialContext function that uses the DNS cache.
// On macOS, connections to RFC 1918 addresses are routed through a subprocess
// (nc) to bypass VPN/NECP routing captures that affect the host process.
func DialContextWithCache(ctx context.Context, network, address string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	// Look up the IP address using the cached resolver
	ips, err := GetDNSResolver().LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	return dialCachedAddresses(ctx, network, host, port, ips, dialResolvedAddress)
}

// Retry only socket establishment, using this one cached DNS answer. A failed
// first address must not conceal a reachable alternative or start another DNS
// lookup. Keep the resolver's order and divide the remaining dial budget so a
// stalled address cannot consume the time reserved for later candidates.
func dialCachedAddresses(ctx context.Context, network, host, port string, ips []string, dial func(context.Context, string, string, time.Duration) (net.Conn, error)) (net.Conn, error) {
	deadline := time.Now().Add(10 * time.Second)
	if callerDeadline, ok := ctx.Deadline(); ok && callerDeadline.Before(deadline) {
		deadline = callerDeadline
	}
	addresses := make([]string, 0, len(ips))
	seen := make(map[netip.Addr]bool, len(ips))
	for _, raw := range ips {
		ip, err := netip.ParseAddr(raw)
		if err != nil {
			continue
		}
		ip = ip.Unmap()
		if ((network == "tcp4" || network == "udp4") && !ip.Is4()) ||
			((network == "tcp6" || network == "udp6") && !ip.Is6()) || seen[ip] {
			continue
		}
		seen[ip] = true
		addresses = append(addresses, net.JoinHostPort(ip.String(), port))
	}
	if len(addresses) == 0 {
		return nil, &net.DNSError{Err: "no suitable IP addresses found", Name: host}
	}

	var firstErr error
	for i, address := range addresses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, &net.OpError{Op: "dial", Net: network, Err: context.DeadlineExceeded}
		}
		timeout := max(time.Nanosecond, remaining/time.Duration(len(addresses)-i))
		conn, err := dial(ctx, network, address, timeout)
		if err == nil {
			if contextErr := ctx.Err(); contextErr != nil {
				conn.Close()
				return nil, contextErr
			}
			return conn, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, firstErr
}

func dialResolvedAddress(ctx context.Context, network, resolvedAddr string, timeout time.Duration) (net.Conn, error) {
	host, _, err := net.SplitHostPort(resolvedAddr)
	if err != nil {
		return nil, err
	}
	resolvedIP := net.ParseIP(host)

	// On macOS, route local-network connections through a subprocess to bypass
	// Tailscale NECP routing captures that prevent the host process from
	// reaching RFC 1918 addresses directly.
	if runtime.GOOS == "darwin" && isPrivateNonVPN(resolvedIP) {
		return dialViaSubprocess(ctx, network, resolvedAddr)
	}

	// Create a dialer with the resolved IP
	dialer := &net.Dialer{
		Timeout:   timeout,
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
