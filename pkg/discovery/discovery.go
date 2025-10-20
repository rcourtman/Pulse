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
	"sync/atomic"
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

// DiscoveryError represents a structured error during discovery
type DiscoveryError struct {
	IP        string    `json:"ip,omitempty"`
	Port      int       `json:"port,omitempty"`
	Phase     string    `json:"phase"`
	ErrorType string    `json:"error_type"` // "timeout", "connection_refused", "no_identification", "phase_error", etc.
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// DiscoveryResult contains all discovered servers
type DiscoveryResult struct {
	Servers          []DiscoveredServer `json:"servers"`
	Errors           []string           `json:"errors,omitempty"`            // Deprecated: kept for backward compatibility
	StructuredErrors []DiscoveryError   `json:"structured_errors,omitempty"` // New structured error format
	Environment      *EnvironmentInfo   `json:"environment,omitempty"`
}

// AddError adds a structured error to the result (also maintains backward-compatible error list)
func (r *DiscoveryResult) AddError(phase, errorType, message, ip string, port int) {
	structuredErr := DiscoveryError{
		IP:        ip,
		Port:      port,
		Phase:     phase,
		ErrorType: errorType,
		Message:   message,
		Timestamp: time.Now(),
	}
	r.StructuredErrors = append(r.StructuredErrors, structuredErr)

	// Also add to legacy errors for backward compatibility
	if ip != "" && port > 0 {
		r.Errors = append(r.Errors, fmt.Sprintf("%s [%s:%d]: %s", phase, ip, port, message))
	} else if ip != "" {
		r.Errors = append(r.Errors, fmt.Sprintf("%s [%s]: %s", phase, ip, message))
	} else {
		r.Errors = append(r.Errors, fmt.Sprintf("%s: %s", phase, message))
	}
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

// ProgressCallback is called to report scan progress
type ProgressCallback func(progress ScanProgress)

// ScanProgress represents the current state of the scan
type ScanProgress struct {
	CurrentPhase    string  `json:"current_phase"`
	PhaseNumber     int     `json:"phase_number"`
	TotalPhases     int     `json:"total_phases"`
	TargetsInPhase  int     `json:"targets_in_phase"`
	ProcessedInPhase int    `json:"processed_in_phase"`
	TotalTargets    int     `json:"total_targets"`
	TotalProcessed  int     `json:"total_processed"`
	ServersFound    int     `json:"servers_found"`
	Percentage      float64 `json:"percentage"`
}

// DiscoverServers scans the network for Proxmox VE and PBS servers
func (s *Scanner) DiscoverServers(ctx context.Context, subnet string) (*DiscoveryResult, error) {
	return s.DiscoverServersWithCallbacks(ctx, subnet, nil, nil)
}

// DiscoverServersWithCallback scans and calls callback for each discovered server
func (s *Scanner) DiscoverServersWithCallback(ctx context.Context, subnet string, callback ServerCallback) (*DiscoveryResult, error) {
	return s.DiscoverServersWithCallbacks(ctx, subnet, callback, nil)
}

// DiscoverServersWithCallbacks scans and calls callbacks for servers and progress
func (s *Scanner) DiscoverServersWithCallbacks(ctx context.Context, subnet string, serverCallback ServerCallback, progressCallback ProgressCallback) (*DiscoveryResult, error) {
	activeProfile, err := s.resolveProfile(subnet)
	if err != nil {
		return nil, err
	}

	result := &DiscoveryResult{
		Servers:          []DiscoveredServer{},
		Errors:           []string{},
		StructuredErrors: []DiscoveryError{},
		Environment:      buildEnvironmentInfo(activeProfile),
	}

	seenIPs := make(map[string]struct{})

	// Calculate total targets and phases for progress tracking
	// Use a preview map to ensure we count only unique IPs that will actually be scanned
	previewSeen := make(map[string]struct{})
	var totalTargets int
	var validPhases []envdetect.SubnetPhase
	phases := append([]envdetect.SubnetPhase(nil), activeProfile.Phases...)
	sort.SliceStable(phases, func(i, j int) bool {
		return phases[i].Priority < phases[j].Priority
	})

	// Count extra targets first (they scan first)
	extraTargetCount := len(s.collectExtraTargets(activeProfile, previewSeen))
	if extraTargetCount > 0 {
		totalTargets += extraTargetCount
	}

	// Then count phase targets, respecting deduplication
	for _, phase := range phases {
		if !s.shouldSkipPhase(ctx, phase) {
			phaseIPs, _ := s.expandPhaseIPs(phase, previewSeen)
			if len(phaseIPs) > 0 {
				totalTargets += len(phaseIPs)
				validPhases = append(validPhases, phase)
			}
		}
	}

	totalPhases := len(validPhases)
	if extraTargetCount > 0 {
		totalPhases++ // Include extra_targets phase
	}

	var totalProcessed int
	phaseNumber := 0

	// Scan explicit extra targets first, if any.
	extraIPs := s.collectExtraTargets(activeProfile, seenIPs)
	if len(extraIPs) > 0 {
		phaseNumber++
		log.Info().
			Int("count", len(extraIPs)).
			Msg("Starting discovery for explicit extra targets")
		if err := s.runPhaseWithProgress(ctx, "extra_targets", phaseNumber, totalPhases, extraIPs, serverCallback, progressCallback, &totalProcessed, totalTargets, result); err != nil {
			errType := "phase_error"
			if errors.Is(err, context.Canceled) {
				errType = "canceled"
			} else if errors.Is(err, context.DeadlineExceeded) {
				errType = "timeout"
			}
			result.AddError("extra_targets", errType, err.Error(), "", 0)
			if errors.Is(err, context.Canceled) {
				return result, ctx.Err()
			}
		}
	}

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

		phaseNumber++
		log.Info().
			Str("phase", phase.Name).
			Int("subnets", subnetCount).
			Int("targets", len(phaseIPs)).
			Float64("confidence", phase.Confidence).
			Msg("Starting discovery phase")

		if err := s.runPhaseWithProgress(ctx, phase.Name, phaseNumber, totalPhases, phaseIPs, serverCallback, progressCallback, &totalProcessed, totalTargets, result); err != nil {
			errType := "phase_error"
			if errors.Is(err, context.Canceled) {
				errType = "canceled"
			} else if errors.Is(err, context.DeadlineExceeded) {
				errType = "timeout"
			}
			result.AddError(phase.Name, errType, err.Error(), "", 0)
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
// NOTE: This function is kept for backward compatibility but is not actively used.
// New code should use scanWorkerWithProgress which includes progress tracking.
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

// scanWorkerWithProgress scans IPs and reports progress
func (s *Scanner) scanWorkerWithProgress(ctx context.Context, wg *sync.WaitGroup, phase string, ipChan <-chan string, resultChan chan<- discoveredResult, progressChan chan<- int) {
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

			// Signal that this IP has been processed
			progressChan <- 1
		}
	}
}

// runPhase runs a scanning phase without progress tracking
// NOTE: This function is kept for backward compatibility but is not actively used.
// New code should use runPhaseWithProgress which includes progress tracking.
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

// runPhaseWithProgress wraps runPhase with progress tracking and reporting
func (s *Scanner) runPhaseWithProgress(ctx context.Context, phase string, phaseNumber, totalPhases int, ips []string, serverCallback ServerCallback, progressCallback ProgressCallback, totalProcessed *int, totalTargets int, result *DiscoveryResult) error {
	if len(ips) == 0 {
		return nil
	}

	var phaseProcessed atomic.Int32
	targetsInPhase := len(ips)

	// Report initial progress for this phase
	if progressCallback != nil {
		progressCallback(ScanProgress{
			CurrentPhase:     phase,
			PhaseNumber:      phaseNumber,
			TotalPhases:      totalPhases,
			TargetsInPhase:   targetsInPhase,
			ProcessedInPhase: 0,
			TotalTargets:     totalTargets,
			TotalProcessed:   *totalProcessed,
			ServersFound:     len(result.Servers),
			Percentage:       float64(*totalProcessed) / float64(totalTargets) * 100,
		})
	}

	workerCount := s.policy.MaxConcurrent
	if workerCount <= 0 {
		workerCount = 1
	}

	ipChan := make(chan string, len(ips))
	resultChan := make(chan discoveredResult, len(ips))
	progressChan := make(chan int, len(ips)) // Signal when an IP is processed

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go s.scanWorkerWithProgress(ctx, &wg, phase, ipChan, resultChan, progressChan)
	}

	for _, ip := range ips {
		ipChan <- ip
	}
	close(ipChan)

	go func() {
		wg.Wait()
		close(resultChan)
		close(progressChan)
	}()

	// Track how often to report progress (every 10 IPs or 5% of phase, whichever is smaller)
	reportInterval := min(10, max(1, targetsInPhase/20))
	lastReported := 0

	for resultChan != nil || progressChan != nil {
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

			log.Debug().
				Str("phase", res.Phase).
				Str("ip", res.Server.IP).
				Str("type", res.Server.Type).
				Str("hostname", res.Server.Hostname).
				Msg("Discovered server")

			if serverCallback != nil {
				serverCallback(*res.Server, res.Phase)
			}

		case _, ok := <-progressChan:
			if !ok {
				progressChan = nil
				continue
			}

			phaseProcessed.Add(1)
			*totalProcessed++
			processed := int(phaseProcessed.Load())

			// Report progress at intervals
			if progressCallback != nil && (processed-lastReported >= reportInterval || processed == targetsInPhase) {
				lastReported = processed
				percentage := float64(*totalProcessed) / float64(totalTargets) * 100
				progressCallback(ScanProgress{
					CurrentPhase:     phase,
					PhaseNumber:      phaseNumber,
					TotalPhases:      totalPhases,
					TargetsInPhase:   targetsInPhase,
					ProcessedInPhase: processed,
					TotalTargets:     totalTargets,
					TotalProcessed:   *totalProcessed,
					ServersFound:     len(result.Servers),
					Percentage:       percentage,
				})
			}

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

	// First attempt a TLS handshake with proper timeout so we can inspect certificate metadata.
	var tlsState *tls.ConnectionState
	timeout := s.policy.DialTimeout
	if timeout <= 0 {
		timeout = time.Second
	}

	// Use context with timeout for TLS dial to prevent hangs
	tlsCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := &net.Dialer{Timeout: timeout}
	tlsConn, tlsErr := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{InsecureSkipVerify: true})
	if tlsErr != nil {
		// If TLS fails completely, try a context-aware TCP dial
		conn, err := dialer.DialContext(tlsCtx, "tcp", address)
		if err != nil {
			return nil // Port not open or unreachable
		}
		conn.Close()
		// Port is open but TLS failed - continue to HTTP check
	} else {
		state := tlsConn.ConnectionState()
		tlsState = &state
		tlsConn.Close()
	}

	// Track whether we got positive identification
	positiveIdentification := false
	serverType := "pve" // Default assumption
	version := "Unknown"
	var release string

	// Infer from certificate if available
	if tlsState != nil {
		if guess := inferTypeFromCertificate(*tlsState); guess != "" {
			serverType = guess
			positiveIdentification = true // Certificate indicates Proxmox
		}
	}

	// Try to get version or authentication headers
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
					positiveIdentification = true // Got valid version data

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

					log.Debug().
						Str("ip", ip).
						Int("port", 8006).
						Str("version", version).
						Msg("Got server version without auth")
				}
			case http.StatusUnauthorized, http.StatusForbidden:
				// Check for Proxmox-specific auth headers
				if guess := inferTypeFromMetadata(
					resp.Header.Get("WWW-Authenticate"),
					resp.Header.Get("Server"),
					resp.Header.Get("Proxmox-Product"),
				); guess != "" {
					serverType = guess
					positiveIdentification = true // Proxmox auth headers present
				}
			}
		}
	}

	// Fallback: probe PMG-specific endpoints if we still think this is a PVE server.
	if serverType != "pmg" && s.isPMGServer(ctx, address) {
		serverType = "pmg"
		positiveIdentification = true
	}

	// Only report server if we got positive identification
	// (not just an open port)
	if !positiveIdentification {
		log.Debug().
			Str("ip", ip).
			Int("port", 8006).
			Msg("Port 8006 open but no Proxmox identification found")
		return nil
	}

	log.Info().
		Str("ip", ip).
		Int("port", 8006).
		Str("type", serverType).
		Str("version", version).
		Msg("Discovered Proxmox server")

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
	// First check if port is open with context-aware dial
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	timeout := s.policy.DialTimeout
	if timeout <= 0 {
		timeout = time.Second
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil // Port not open
	}
	conn.Close()

	// Port is open - verify it's actually a Proxmox server
	positiveIdentification := false
	version := "Unknown"
	var release string

	// Try to get version or authentication headers
	url := fmt.Sprintf("https://%s/api2/json/version", address)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err == nil {
		resp, err := s.httpClient.Do(req)
		if err == nil {
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
					positiveIdentification = true

					log.Debug().
						Str("ip", ip).
						Int("port", port).
						Str("version", version).
						Msg("Got server version without auth")
				}
			case http.StatusUnauthorized, http.StatusForbidden:
				// Check for Proxmox-specific auth headers
				if inferTypeFromMetadata(
					resp.Header.Get("WWW-Authenticate"),
					resp.Header.Get("Server"),
					resp.Header.Get("Proxmox-Product"),
				) != "" {
					positiveIdentification = true
				}
			}
		}
	}

	// Only report server if we got positive identification
	if !positiveIdentification {
		log.Debug().
			Str("ip", ip).
			Int("port", port).
			Str("expected_type", serverType).
			Msg("Port open but no Proxmox identification found")
		return nil
	}

	log.Info().
		Str("ip", ip).
		Int("port", port).
		Str("type", serverType).
		Str("version", version).
		Msg("Discovered Proxmox server")

	server := &DiscoveredServer{
		IP:      ip,
		Port:    port,
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
