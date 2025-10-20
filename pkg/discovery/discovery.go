package discovery

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery/envdetect"
)

// DiscoveredServer represents a discovered Proxmox/PBS/PMG server
type DiscoveredServer struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Type     string `json:"type"` // "pve", "pbs", or "pmg"
	Version  string `json:"version"`
	Hostname string `json:"hostname,omitempty"`
	Release  string `json:"release,omitempty"`
}

// DiscoveryResult contains all discovered servers
type DiscoveryResult struct {
	Servers     []DiscoveredServer `json:"servers"`
	Errors      []string           `json:"errors,omitempty"`
	Environment *EnvironmentInfo   `json:"environment,omitempty"`
}

// EnvironmentInfo captures metadata about the environment scan.
type EnvironmentInfo struct {
	Type       string            `json:"type"`
	Confidence float64           `json:"confidence"`
	Phases     []PhaseInfo       `json:"phases"`
	Warnings   []string          `json:"warnings"`
	Metadata   map[string]string `json:"metadata"`
}

// PhaseInfo exposes phase details to clients.
type PhaseInfo struct {
	Name       string   `json:"name"`
	Subnets    []string `json:"subnets"`
	Confidence float64  `json:"confidence"`
}

// Scanner handles network scanning for Proxmox/PBS servers
type Scanner struct {
	policy     envdetect.ScanPolicy
	profile    *envdetect.EnvironmentProfile
	httpClient *http.Client
}

// NewScanner creates a new network scanner
func NewScanner() *Scanner {
	profile, err := envdetect.DetectEnvironment()
	if err != nil {
		log.Warn().Err(err).Msg("Environment detection completed with warnings")
	}

	return NewScannerWithProfile(profile)
}

// NewScannerWithProfile creates a scanner using the supplied environment profile.
func NewScannerWithProfile(profile *envdetect.EnvironmentProfile) *Scanner {
	clonedProfile := cloneProfile(profile)
	policy := ensurePolicyDefaults(clonedProfile.Policy)
	clonedProfile.Policy = policy

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:    100,
		MaxConnsPerHost: max(policy.MaxConcurrent, 10),
	}

	client := &http.Client{
		Timeout:   policy.HTTPTimeout,
		Transport: transport,
	}

	return &Scanner{
		policy:     policy,
		profile:    clonedProfile,
		httpClient: client,
	}
}

// ServerCallback is called when a server is discovered
type ServerCallback func(server DiscoveredServer, phase string)

// DiscoverServers scans the network for Proxmox VE and PBS servers
func (s *Scanner) DiscoverServers(ctx context.Context, subnet string) (*DiscoveryResult, error) {
	return s.DiscoverServersWithCallback(ctx, subnet, nil)
}

// DiscoverServersWithCallback scans and calls callback for each discovered server
func (s *Scanner) DiscoverServersWithCallback(ctx context.Context, subnet string, callback ServerCallback) (*DiscoveryResult, error) {
	activeProfile, err := s.resolveProfile(subnet)
	if err != nil {
		return nil, err
	}

	result := &DiscoveryResult{
		Servers:     []DiscoveredServer{},
		Errors:      []string{},
		Environment: buildEnvironmentInfo(activeProfile),
	}

	seenIPs := make(map[string]struct{})

	// Scan explicit extra targets first, if any.
	extraIPs := s.collectExtraTargets(activeProfile, seenIPs)
	if len(extraIPs) > 0 {
		log.Info().
			Int("count", len(extraIPs)).
			Msg("Starting discovery for explicit extra targets")
		if err := s.runPhase(ctx, "extra_targets", extraIPs, callback, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("extra_targets: %v", err))
			if errors.Is(err, context.Canceled) {
				return result, ctx.Err()
			}
		}
	}

	phases := append([]envdetect.SubnetPhase(nil), activeProfile.Phases...)
	sort.SliceStable(phases, func(i, j int) bool {
		return phases[i].Priority < phases[j].Priority
	})

	for _, phase := range phases {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		if s.shouldSkipPhase(ctx, phase) {
			log.Warn().
				Str("phase", phase.Name).
				Float64("confidence", phase.Confidence).
				Msg("Skipping discovery phase due to low confidence/time budget")
			continue
		}

		phaseIPs, subnetCount := s.expandPhaseIPs(phase, seenIPs)
		if len(phaseIPs) == 0 {
			log.Debug().
				Str("phase", phase.Name).
				Int("subnets", subnetCount).
				Msg("No scan targets generated for phase")
			continue
		}

		log.Info().
			Str("phase", phase.Name).
			Int("subnets", subnetCount).
			Int("targets", len(phaseIPs)).
			Float64("confidence", phase.Confidence).
			Msg("Starting discovery phase")

		if err := s.runPhase(ctx, phase.Name, phaseIPs, callback, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", phase.Name, err))
			if errors.Is(err, context.Canceled) {
				return result, ctx.Err()
			}
		}
	}

	log.Info().
		Int("servers_found", len(result.Servers)).
		Int("errors", len(result.Errors)).
		Msg("Discovery completed")

	return result, nil
}

type discoveredResult struct {
	Phase  string
	Server *DiscoveredServer
}

type phaseError struct {
	Phase   string
	Message string
}

// scanWorker scans IPs from the channel
func (s *Scanner) scanWorker(ctx context.Context, wg *sync.WaitGroup, phase string, ipChan <-chan string, resultChan chan<- discoveredResult, errorChan chan<- phaseError) {
	defer wg.Done()

	for ip := range ipChan {
		select {
		case <-ctx.Done():
			return
		default:
			if server := s.checkPort8006(ctx, ip); server != nil {
				resultChan <- discoveredResult{Phase: phase, Server: server}
			}

			if server := s.checkServer(ctx, ip, 8007, "pbs"); server != nil {
				resultChan <- discoveredResult{Phase: phase, Server: server}
			}
		}
	}
}

func (s *Scanner) runPhase(ctx context.Context, phase string, ips []string, callback ServerCallback, result *DiscoveryResult) error {
	if len(ips) == 0 {
		return nil
	}

	workerCount := s.policy.MaxConcurrent
	if workerCount <= 0 {
		workerCount = 1
	}

	ipChan := make(chan string, len(ips))
	resultChan := make(chan discoveredResult, len(ips))
	errorChan := make(chan phaseError, len(ips))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go s.scanWorker(ctx, &wg, phase, ipChan, resultChan, errorChan)
	}

	for _, ip := range ips {
		ipChan <- ip
	}
	close(ipChan)

	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	for resultChan != nil || errorChan != nil {
		select {
		case res, ok := <-resultChan:
			if !ok {
				resultChan = nil
				continue
			}
			if res.Server == nil {
				continue
			}

			result.Servers = append(result.Servers, *res.Server)

			log.Info().
				Str("phase", res.Phase).
				Str("ip", res.Server.IP).
				Str("type", res.Server.Type).
				Str("hostname", res.Server.Hostname).
				Msg("Discovered server")

			if callback != nil {
				callback(*res.Server, res.Phase)
			}
		case perr, ok := <-errorChan:
			if !ok {
				errorChan = nil
				continue
			}
			if perr.Message == "" {
				continue
			}
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", perr.Phase, perr.Message))
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (s *Scanner) resolveProfile(subnet string) (*envdetect.EnvironmentProfile, error) {
	if strings.EqualFold(strings.TrimSpace(subnet), "auto") || strings.TrimSpace(subnet) == "" {
		return cloneProfile(s.profile), nil
	}

    var (
        subnets        []net.IPNet
        renderedSubnets []string
    )
    for _, token := range strings.Split(subnet, ",") {
        token = strings.TrimSpace(token)
        if token == "" {
            continue
        }
        _, parsedNet, err := net.ParseCIDR(token)
        if err != nil {
            return nil, fmt.Errorf("invalid subnet %q: %w", token, err)
        }
        subnets = append(subnets, *parsedNet)
        renderedSubnets = append(renderedSubnets, parsedNet.String())
    }

    if len(subnets) == 0 {
        return nil, fmt.Errorf("no valid subnets provided")
    }

	manualProfile := &envdetect.EnvironmentProfile{
		Type:       envdetect.Unknown,
		Confidence: 1.0,
		Policy:     s.policy,
		Warnings:   []string{"Manual subnet override applied"},
        Metadata: map[string]string{
            "manual_subnets": strings.Join(renderedSubnets, ","),
        },
		Phases: []envdetect.SubnetPhase{
			{
				Name:       "manual_subnet",
				Subnets:    subnets,
				Confidence: 1.0,
				Priority:   1,
			},
		},
	}

	return manualProfile, nil
}

func (s *Scanner) collectExtraTargets(profile *envdetect.EnvironmentProfile, seen map[string]struct{}) []string {
	if profile == nil {
		return nil
	}

	var targets []string
	for _, ip := range profile.ExtraTargets {
		if ip == nil {
			continue
		}
		ip4 := ip.To4()
		if ip4 == nil {
			continue
		}
		ipStr := ip4.String()
		if _, exists := seen[ipStr]; exists {
			continue
		}
		seen[ipStr] = struct{}{}
		targets = append(targets, ipStr)
	}

	return targets
}

func (s *Scanner) expandPhaseIPs(phase envdetect.SubnetPhase, seen map[string]struct{}) ([]string, int) {
	var targets []string

	for _, subnet := range phase.Subnets {
		if subnet.IP.To4() == nil {
			continue
		}

		copySubnet := subnet // copy to avoid modifying original
		ips := s.generateIPs(&copySubnet)
		for _, ip := range ips {
			if _, exists := seen[ip]; exists {
				continue
			}
			seen[ip] = struct{}{}
			targets = append(targets, ip)
		}
	}

	return targets, len(phase.Subnets)
}

func (s *Scanner) shouldSkipPhase(ctx context.Context, phase envdetect.SubnetPhase) bool {
	if phase.Confidence >= 0.5 {
		return false
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return false
	}

	timeRemaining := time.Until(deadline)
	minBudget := s.policy.DialTimeout * 5
	if minBudget <= 0 {
		minBudget = 5 * time.Second
	}

	return timeRemaining > 0 && timeRemaining < minBudget
}

func buildEnvironmentInfo(profile *envdetect.EnvironmentProfile) *EnvironmentInfo {
	if profile == nil {
		return nil
	}

	info := &EnvironmentInfo{
		Type:       profile.Type.String(),
		Confidence: profile.Confidence,
		Warnings:   append([]string(nil), profile.Warnings...),
		Metadata:   copyMetadata(profile.Metadata),
	}

	for _, phase := range profile.Phases {
		pInfo := PhaseInfo{
			Name:       phase.Name,
			Confidence: phase.Confidence,
			Subnets:    []string{},
		}
		for _, subnet := range phase.Subnets {
			pInfo.Subnets = append(pInfo.Subnets, subnet.String())
		}
		info.Phases = append(info.Phases, pInfo)
	}

	return info
}

func ensurePolicyDefaults(policy envdetect.ScanPolicy) envdetect.ScanPolicy {
	defaults := envdetect.DefaultScanPolicy()

	if policy.MaxConcurrent <= 0 {
		policy.MaxConcurrent = defaults.MaxConcurrent
	}
	if policy.DialTimeout <= 0 {
		policy.DialTimeout = defaults.DialTimeout
	}
	if policy.HTTPTimeout <= 0 {
		policy.HTTPTimeout = defaults.HTTPTimeout
	}
	if policy.MaxHostsPerScan < 0 {
		policy.MaxHostsPerScan = defaults.MaxHostsPerScan
	}

	return policy
}

func cloneProfile(profile *envdetect.EnvironmentProfile) *envdetect.EnvironmentProfile {
	if profile == nil {
		defaults := envdetect.DefaultScanPolicy()
		return &envdetect.EnvironmentProfile{
			Type:       envdetect.Unknown,
			Confidence: 0.3,
			Policy:     defaults,
			Warnings:   []string{"Environment profile unavailable; using defaults"},
			Metadata:   map[string]string{},
		}
	}

	cloned := *profile
	if profile.Metadata != nil {
		cloned.Metadata = copyMetadata(profile.Metadata)
	}
	if profile.Warnings != nil {
		cloned.Warnings = append([]string(nil), profile.Warnings...)
	}
	if profile.ExtraTargets != nil {
		cloned.ExtraTargets = append([]net.IP(nil), profile.ExtraTargets...)
	}
	if profile.Phases != nil {
		cloned.Phases = make([]envdetect.SubnetPhase, len(profile.Phases))
		for i, phase := range profile.Phases {
			cloned.Phases[i] = clonePhase(phase)
		}
	}

	return &cloned
}

func clonePhase(phase envdetect.SubnetPhase) envdetect.SubnetPhase {
	cloned := phase
	if phase.Subnets != nil {
		cloned.Subnets = make([]net.IPNet, len(phase.Subnets))
		for i, subnet := range phase.Subnets {
			cloned.Subnets[i] = subnet
		}
	}
	return cloned
}

func copyMetadata(src map[string]string) map[string]string {
	if src == nil {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// checkPort8006 checks if port 8006 is running PMG or PVE
func (s *Scanner) checkPort8006(ctx context.Context, ip string) *DiscoveredServer {
	address := net.JoinHostPort(ip, "8006")

	// First attempt a TLS handshake so we can inspect certificate metadata.
	var tlsState *tls.ConnectionState
    timeout := s.policy.DialTimeout
    if timeout <= 0 {
        timeout = time.Second
    }
    dialer := &net.Dialer{Timeout: timeout}

    tlsConn, tlsErr := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{InsecureSkipVerify: true})
	if tlsErr != nil {
		// Fallback to a simple TCP dial to confirm the port is open.
        conn, err := dialer.DialContext(ctx, "tcp", address)
        if err != nil {
            return nil // Port not open
        }
        conn.Close()
	} else {
		state := tlsConn.ConnectionState()
		tlsState = &state
		tlsConn.Close()
	}

	serverType := "pve"
	if tlsState != nil {
		if guess := inferTypeFromCertificate(*tlsState); guess != "" {
			serverType = guess
		}
	}

	version := "Unknown"
	var release string

	// Try to get version without auth (some installations allow it)
	versionURL := fmt.Sprintf("https://%s/api2/json/version", address)
	if req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil); err == nil {
		if resp, err := s.httpClient.Do(req); err == nil {
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				var versionResp struct {
					Data struct {
						Version string `json:"version"`
						Release string `json:"release,omitempty"`
					} `json:"data"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&versionResp); err == nil && versionResp.Data.Version != "" {
					version = versionResp.Data.Version
					release = versionResp.Data.Release

					if guess := inferTypeFromMetadata(
						versionResp.Data.Version,
						versionResp.Data.Release,
						resp.Header.Get("Server"),
						resp.Header.Get("Proxmox-Product"),
						resp.Header.Get("WWW-Authenticate"),
						strings.Join(resp.Header.Values("Set-Cookie"), " "),
					); guess != "" {
						serverType = guess
					}

					log.Info().
						Str("ip", ip).
						Int("port", 8006).
						Str("version", version).
						Msg("Got server version without auth")
				}
			case http.StatusUnauthorized, http.StatusForbidden:
				if guess := inferTypeFromMetadata(
					resp.Header.Get("WWW-Authenticate"),
					resp.Header.Get("Server"),
					resp.Header.Get("Proxmox-Product"),
				); guess != "" {
					serverType = guess
				}
			}
		}
	}

	// Fallback: probe PMG-specific endpoints if we still think this is a PVE server.
	if serverType != "pmg" && s.isPMGServer(ctx, address) {
		serverType = "pmg"
	}

	log.Info().
		Str("ip", ip).
		Int("port", 8006).
		Str("type", serverType).
		Msg("Found potential server (port open)")

	server := &DiscoveredServer{
		IP:      ip,
		Port:    8006,
		Type:    serverType,
		Version: version,
		Release: release,
	}

	// Try to resolve hostname via reverse DNS
    if s.policy.EnableReverseDNS {
        names, err := net.DefaultResolver.LookupAddr(ctx, ip)
        if err == nil && len(names) > 0 {
            hostname := strings.TrimSuffix(names[0], ".")
            server.Hostname = hostname
            log.Debug().Str("ip", ip).Str("hostname", hostname).Msg("Resolved hostname via DNS")
        }
    }

    return server
}

// isPMGServer checks if a server is PMG by checking for PMG-specific endpoints
func (s *Scanner) isPMGServer(ctx context.Context, address string) bool {
	endpoints := []string{
		"api2/json/statistics/mail",
		"api2/json/mail/queue",
		"api2/json/mail/quarantine",
	}

	for _, endpoint := range endpoints {
		product := s.detectProductFromEndpoint(ctx, address, endpoint)
		if product == "pmg" {
			log.Debug().
				Str("address", address).
				Str("endpoint", endpoint).
				Msg("PMG-specific endpoint confirmed")
			return true
		}
	}

	return false
}

// detectProductFromEndpoint inspects an HTTP endpoint and tries to infer the product type.
func (s *Scanner) detectProductFromEndpoint(ctx context.Context, address, endpoint string) string {
	url := fmt.Sprintf("https://%s/%s", address, endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	headerProduct := inferTypeFromMetadata(
		resp.Header.Get("Server"),
		resp.Header.Get("Proxmox-Product"),
		resp.Header.Get("WWW-Authenticate"),
		strings.Join(resp.Header.Values("Set-Cookie"), " "),
	)
	if headerProduct != "" {
		return headerProduct
	}

	// If the endpoint responded (not 404) and the path is PMG-specific, treat it as PMG.
	if resp.StatusCode != http.StatusNotFound && strings.Contains(endpoint, "mail") {
		return "pmg"
	}

	return ""
}

// inferTypeFromCertificate tries to determine the product based on TLS certificate metadata.
func inferTypeFromCertificate(state tls.ConnectionState) string {
	if len(state.PeerCertificates) == 0 {
		return ""
	}

	cert := state.PeerCertificates[0]
	parts := []string{cert.Subject.CommonName}
	parts = append(parts, cert.Subject.Organization...)
	parts = append(parts, cert.Subject.OrganizationalUnit...)

	return inferTypeFromMetadata(parts...)
}

// inferTypeFromMetadata inspects textual metadata and returns a best-effort product type.
func inferTypeFromMetadata(parts ...string) string {
	var builder strings.Builder

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(strings.ToLower(part))
	}

	combined := builder.String()
	if combined == "" {
		return ""
	}

	compact := strings.ReplaceAll(combined, " ", "")

	switch {
	case strings.Contains(combined, "pmg"),
		strings.Contains(combined, "mail gateway"),
		strings.Contains(combined, "pmgauth"),
		strings.Contains(combined, "pmgauthcookie"),
		strings.Contains(compact, "mailgateway"),
		strings.Contains(compact, "pmg-api"):
		return "pmg"
	case strings.Contains(combined, "pbs"),
		strings.Contains(combined, "backup server"),
		strings.Contains(combined, "pbsauth"),
		strings.Contains(compact, "pbs-api"):
		return "pbs"
	case strings.Contains(combined, "pve"),
		strings.Contains(combined, "virtual environment"),
		strings.Contains(combined, "pveauth"),
		strings.Contains(compact, "pve-api"):
		return "pve"
	default:
		return ""
	}
}

// checkServer checks if a server is running at the given IP and port
func (s *Scanner) checkServer(ctx context.Context, ip string, port int, serverType string) *DiscoveredServer {
	// First check if port is open
    address := net.JoinHostPort(ip, strconv.Itoa(port))
    timeout := s.policy.DialTimeout
    if timeout <= 0 {
        timeout = time.Second
    }
    dialer := &net.Dialer{Timeout: timeout}

    conn, err := dialer.DialContext(ctx, "tcp", address)
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
    if s.policy.EnableReverseDNS {
        names, err := net.DefaultResolver.LookupAddr(ctx, ip)
        if err == nil && len(names) > 0 {
            hostname := strings.TrimSuffix(names[0], ".")
            server.Hostname = hostname
            log.Debug().Str("ip", ip).Str("hostname", hostname).Msg("Resolved hostname via DNS")
        }
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
    baseIP := ipNet.IP.Mask(ipNet.Mask).To4()
    if baseIP == nil {
        return nil
    }

    ones, bits := ipNet.Mask.Size()
    if bits != 32 {
        return nil
    }

    hostBits := bits - ones
    totalHosts := 1
    if hostBits > 0 {
        totalHosts = 1 << hostBits
    }

    start := 0
    end := totalHosts
    if totalHosts > 2 {
        start = 1
        end = totalHosts - 1
    }

    limit := s.policy.MaxHostsPerScan
    if limit > 0 && limit < (end-start) {
        end = start + limit
        log.Debug().
            Str("subnet", ipNet.String()).
            Int("limit", limit).
            Msg("Applying max hosts per scan limit")
    }

    ips := make([]string, 0, end-start)

    for offset := start; offset < end; offset++ {
        currIP := make(net.IP, len(baseIP))
        copy(currIP, baseIP)

        carry := offset
        for idx := len(currIP) - 1; idx >= 0 && carry > 0; idx-- {
            currIP[idx] += byte(carry & 0xFF)
            carry >>= 8
        }

        ips = append(ips, currIP.String())
    }

    return ips
}
