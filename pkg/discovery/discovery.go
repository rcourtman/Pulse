package discovery

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DiscoveredServer represents a discovered Proxmox/PBS server
type DiscoveredServer struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Type     string `json:"type"` // "pve" or "pbs"
	Version  string `json:"version"`
	Hostname string `json:"hostname,omitempty"`
	Release  string `json:"release,omitempty"`
}

// DiscoveryResult contains all discovered servers
type DiscoveryResult struct {
	Servers []DiscoveredServer `json:"servers"`
	Errors  []string           `json:"errors,omitempty"`
}

// Scanner handles network scanning for Proxmox/PBS servers
type Scanner struct {
	timeout    time.Duration
	concurrent int
	httpClient *http.Client
}

// NewScanner creates a new network scanner
func NewScanner() *Scanner {
	return &Scanner{
		timeout:    1 * time.Second, // Reduced timeout for faster scanning
		concurrent: 50,              // Increased concurrent workers for faster scanning
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // Reduced HTTP timeout
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:    100,
				MaxConnsPerHost: 10,
			},
		},
	}
}

// ServerCallback is called when a server is discovered
type ServerCallback func(server DiscoveredServer)

// DiscoverServers scans the network for Proxmox VE and PBS servers
func (s *Scanner) DiscoverServers(ctx context.Context, subnet string) (*DiscoveryResult, error) {
	return s.DiscoverServersWithCallback(ctx, subnet, nil)
}

// DiscoverServersWithCallback scans and calls callback for each discovered server
func (s *Scanner) DiscoverServersWithCallback(ctx context.Context, subnet string, callback ServerCallback) (*DiscoveryResult, error) {
	log.Info().Str("subnet", subnet).Msg("Starting network discovery")

	// Parse subnet
	var ipNets []*net.IPNet

	if subnet == "" || subnet == "auto" {
		// Check if we're in Docker (detected subnet is Docker network)
		autoDetected := s.getLocalSubnet()
		if autoDetected != nil && strings.HasPrefix(autoDetected.String(), "172.17.") {
			log.Info().Msg("Running in Docker - scanning common home/office networks")
			// In Docker, scan common subnets instead
			ipNets = s.getCommonSubnets()
		} else if autoDetected != nil {
			// Use auto-detected subnet
			ipNets = []*net.IPNet{autoDetected}
			log.Info().Str("detected", autoDetected.String()).Msg("Auto-detected local subnet")
		} else {
			// Fallback to common subnets
			log.Info().Msg("Auto-detection failed - scanning common networks")
			ipNets = s.getCommonSubnets()
		}
	} else {
		// Parse provided subnet
		_, parsedNet, err := net.ParseCIDR(subnet)
		if err != nil {
			return nil, fmt.Errorf("invalid subnet: %w", err)
		}
		ipNets = []*net.IPNet{parsedNet}
	}

	// Collect all IPs to scan from all subnets
	var allIPs []string
	for _, ipNet := range ipNets {
		// Check subnet size - limit to /24 or smaller for safety
		ones, bits := ipNet.Mask.Size()
		if ones < 24 && bits == 32 { // IPv4 with more than 256 addresses
			log.Warn().Str("subnet", ipNet.String()).Msg("Subnet too large, limiting to /24")
			// Convert to /24
			ipNet.Mask = net.CIDRMask(24, 32)
		}

		// Generate list of IPs for this subnet
		ips := s.generateIPs(ipNet)
		allIPs = append(allIPs, ips...)
		log.Info().Str("subnet", ipNet.String()).Int("count", len(ips)).Msg("Subnet IPs to scan")
	}
	log.Info().Int("total", len(allIPs)).Msg("Total IPs to scan")

	// Create channels for work distribution
	ipChan := make(chan string, len(allIPs))
	resultChan := make(chan *DiscoveredServer, len(allIPs))
	errorChan := make(chan string, len(allIPs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < s.concurrent; i++ {
		wg.Add(1)
		go s.scanWorker(ctx, &wg, ipChan, resultChan, errorChan)
	}

	// Send IPs to scan
	for _, ip := range allIPs {
		ipChan <- ip
	}
	close(ipChan)

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	result := &DiscoveryResult{
		Servers: []DiscoveredServer{},
		Errors:  []string{},
	}

	done := false
	for !done {
		select {
		case server, ok := <-resultChan:
			if !ok {
				done = true
				break
			}
			if server != nil {
				result.Servers = append(result.Servers, *server)
				// Log immediately when found for real-time feedback
				log.Info().
					Str("ip", server.IP).
					Str("type", server.Type).
					Str("hostname", server.Hostname).
					Msg("🎯 Discovered server - adding to results")

				// Call callback for real-time updates
				if callback != nil {
					callback(*server)
				}
			}
		case errMsg, ok := <-errorChan:
			if ok && errMsg != "" {
				result.Errors = append(result.Errors, errMsg)
			}
		case <-ctx.Done():
			return result, ctx.Err()
		}
	}

	log.Info().
		Int("found", len(result.Servers)).
		Int("errors", len(result.Errors)).
		Msg("Discovery completed")

	return result, nil
}

// scanWorker scans IPs from the channel
func (s *Scanner) scanWorker(ctx context.Context, wg *sync.WaitGroup, ipChan <-chan string, resultChan chan<- *DiscoveredServer, errorChan chan<- string) {
	defer wg.Done()

	for ip := range ipChan {
		select {
		case <-ctx.Done():
			return
		default:
			// Check Proxmox VE (port 8006)
			if server := s.checkServer(ctx, ip, 8006, "pve"); server != nil {
				resultChan <- server
			}

			// Check PBS (port 8007)
			if server := s.checkServer(ctx, ip, 8007, "pbs"); server != nil {
				resultChan <- server
			}
		}
	}
}

// checkServer checks if a server is running at the given IP and port
func (s *Scanner) checkServer(ctx context.Context, ip string, port int, serverType string) *DiscoveredServer {
	// First check if port is open
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, s.timeout)
	if err != nil {
		return nil // Port not open
	}
	conn.Close()

	// Port is open - this is likely a Proxmox/PBS server
	// Since most installations require auth for version endpoint,
	// we'll return it as a discovered server based on the port alone

	log.Info().
		Str("ip", ip).
		Int("port", port).
		Str("type", serverType).
		Msg("Found potential server (port open)")

	server := &DiscoveredServer{
		IP:      ip,
		Port:    port,
		Type:    serverType,
		Version: "Unknown", // Will be determined after auth
	}

	// Try to get version without auth (some installations allow it)
	url := fmt.Sprintf("https://%s/api2/json/version", address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err == nil {
		resp, err := s.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()

			// Only try to parse if we got a successful response
			if resp.StatusCode == 200 {
				var versionResp struct {
					Data struct {
						Version string `json:"version"`
						Release string `json:"release,omitempty"`
					} `json:"data"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&versionResp); err == nil && versionResp.Data.Version != "" {
					server.Version = versionResp.Data.Version
					server.Release = versionResp.Data.Release

					log.Info().
						Str("ip", ip).
						Int("port", port).
						Str("version", server.Version).
						Msg("Got server version without auth")
				}
			}
		}
	}

	// Try to resolve hostname via reverse DNS
	names, err := net.LookupAddr(ip)
	if err == nil && len(names) > 0 {
		// Use the first hostname, remove trailing dot if present
		hostname := strings.TrimSuffix(names[0], ".")
		server.Hostname = hostname
		log.Debug().Str("ip", ip).Str("hostname", hostname).Msg("Resolved hostname via DNS")
	}

	return server
}

// getProxmoxHostname tries to get the hostname of a Proxmox VE server
func (s *Scanner) getProxmoxHostname(ctx context.Context, ip string, port int) string {
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	url := fmt.Sprintf("https://%s/api2/json/nodes", address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var nodesResp struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&nodesResp); err != nil {
		return ""
	}

	if len(nodesResp.Data) > 0 {
		return nodesResp.Data[0].Node
	}

	return ""
}

// getPBSHostname tries to get the hostname of a PBS server
func (s *Scanner) getPBSHostname(ctx context.Context, ip string, port int) string {
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	url := fmt.Sprintf("https://%s/api2/json/nodes", address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var nodesResp struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&nodesResp); err != nil {
		return ""
	}

	if len(nodesResp.Data) > 0 {
		return nodesResp.Data[0].Node
	}

	return ""
}

// generateIPs generates all IPs in a subnet
func (s *Scanner) generateIPs(ipNet *net.IPNet) []string {
	var ips []string

	// Get the starting IP
	ip := ipNet.IP.Mask(ipNet.Mask)

	// Calculate the number of hosts
	ones, bits := ipNet.Mask.Size()
	hostBits := bits - ones
	numHosts := 1 << hostBits

	// Skip network and broadcast addresses for common subnets
	start := 1
	end := numHosts - 1
	if numHosts > 256 {
		// For larger subnets, scan everything
		start = 0
		end = numHosts
	}

	// Limit to maximum 1024 IPs to avoid scanning huge networks
	if end-start > 1024 {
		end = start + 1024
		log.Warn().Int("limited_to", 1024).Msg("Limiting scan to first 1024 IPs")
	}

	for i := start; i < end; i++ {
		// Calculate IP
		currIP := make(net.IP, len(ip))
		copy(currIP, ip)

		// Add offset to IP address
		offset := i
		for j := len(currIP) - 1; j >= 0 && offset > 0; j-- {
			currIP[j] += byte(offset & 0xFF)
			offset >>= 8
		}

		// Skip common non-server IPs
		lastOctet := currIP[len(currIP)-1]
		if lastOctet == 0 || lastOctet == 255 {
			continue // Skip network and broadcast
		}

		ips = append(ips, currIP.String())
	}

	return ips
}

// getLocalSubnet attempts to detect the local subnet
func (s *Scanner) getLocalSubnet() *net.IPNet {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				// Found an IPv4 address
				if !ipNet.IP.IsLoopback() && !ipNet.IP.IsLinkLocalUnicast() {
					// Convert to /24 subnet for auto-detection
					// This ensures we scan a reasonable range
					ip := ipNet.IP.To4()
					if ip != nil {
						// Create a /24 subnet from the IP
						ip[3] = 0 // Set last octet to 0
						return &net.IPNet{
							IP:   ip,
							Mask: net.CIDRMask(24, 32),
						}
					}
				}
			}
		}
	}

	// Default to common subnet if detection fails
	_, defaultNet, _ := net.ParseCIDR("192.168.1.0/24")
	return defaultNet
}

// getCommonSubnets returns a list of common home/office network subnets
func (s *Scanner) getCommonSubnets() []*net.IPNet {
	// Ordered by likelihood - most common first for faster results
	commonSubnets := []string{
		"192.168.1.0/24", // Most common home router default
		"192.168.0.0/24", // Very common alternative
		"10.0.0.0/24",    // Some routers use this
		// Skip less common ones for speed:
		// "192.168.88.0/24", // MikroTik default (uncommon)
		// "172.16.0.0/24",   // Less common but used
	}

	var nets []*net.IPNet
	for _, subnet := range commonSubnets {
		_, ipNet, err := net.ParseCIDR(subnet)
		if err == nil {
			nets = append(nets, ipNet)
		}
	}

	return nets
}
