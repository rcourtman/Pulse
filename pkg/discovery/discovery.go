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

// friendlyPhaseName converts technical phase names to user-friendly descriptions
func friendlyPhaseName(phase string) string {
	friendlyNames := map[string]string{
		"lxc_container_network":    "Container network",
		"docker_bridge_network":    "Docker bridge network",
		"docker_container_network": "Docker container network",
		"host_local_network":       "Local network",
		"inferred_gateway_network": "Gateway network",
		"extra_targets":            "Additional targets",
		"proxmox_cluster_network":  "Proxmox cluster network",
	}

	if friendly, ok := friendlyNames[phase]; ok {
		return friendly
	}
	return phase
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

	// Also add to legacy errors for backward compatibility (use friendly phase name for display)
	friendlyPhase := friendlyPhaseName(phase)
	if ip != "" && port > 0 {
		r.Errors = append(r.Errors, fmt.Sprintf("%s [%s:%d]: %s", friendlyPhase, ip, port, message))
	} else if ip != "" {
		r.Errors = append(r.Errors, fmt.Sprintf("%s [%s]: %s", friendlyPhase, ip, message))
	} else {
		r.Errors = append(r.Errors, fmt.Sprintf("%s: %s", friendlyPhase, message))
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
	CurrentPhase     string  `json:"current_phase"`
	PhaseNumber      int     `json:"phase_number"`
	TotalPhases      int     `json:"total_phases"`
	TargetsInPhase   int     `json:"targets_in_phase"`
	ProcessedInPhase int     `json:"processed_in_phase"`
	TotalTargets     int     `json:"total_targets"`
	TotalProcessed   int     `json:"total_processed"`
	ServersFound     int     `json:"servers_found"`
	Percentage       float64 `json:"percentage"`
}

type EndpointProbeFinding struct {
	Endpoint     string
	Status       int
	Headers      http.Header
	ProductGuess string
	Error        error
}

type ProxmoxProbeResult struct {
	IP                string
	Port              int
	Reachable         bool
	TLSState          *tls.ConnectionState
	TLSHandshakeError error

	Version       string
	Release       string
	VersionStatus int
	VersionError  error
	Headers       http.Header

	EndpointFindings map[string]EndpointProbeFinding

	ProductScores   map[string]float64
	ProductEvidence map[string][]string
	PrimaryProduct  string
	PrimaryScore    float64

	Positive        bool
	PositiveReasons []string
	Err             error
}

const (
	productPVE = "pve"
	productPMG = "pmg"
	productPBS = "pbs"

	productPositiveThreshold = 0.7
)

var proxmoxProbePorts = [...]int{8006, 8007}

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

	// Pre-populate seenIPs with blocked IPs to skip them during scanning
	// This prevents probing already-configured Proxmox hosts (reduces PBS auth failure log spam)
	blockedCount := 0
	if activeProfile != nil {
		for _, ip := range activeProfile.IPBlocklist {
			if ip == nil {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				seenIPs[ip4.String()] = struct{}{}
				blockedCount++
			}
		}
		if blockedCount > 0 {
			log.Debug().
				Int("blocked_ips", blockedCount).
				Msg("Pre-populated blocked IPs to skip during discovery")
		}
	}

	// Calculate total targets and phases for progress tracking
	// Use a preview map to ensure we count only unique IPs that will actually be scanned
	// Copy blocked IPs to preview map as well
	previewSeen := make(map[string]struct{})
	for ip := range seenIPs {
		previewSeen[ip] = struct{}{}
	}
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
			errMsg := err.Error()

			if errors.Is(err, context.Canceled) {
				errType = "canceled"
			} else if errors.Is(err, context.DeadlineExceeded) {
				errType = "timeout"
				// Provide user-friendly timeout message
				errMsg = "Scan timed out after 2 minutes - this is normal for large networks. Servers found in earlier phases are still available."
			}
			result.AddError("extra_targets", errType, errMsg, "", 0)
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
			errMsg := err.Error()

			if errors.Is(err, context.Canceled) {
				errType = "canceled"
			} else if errors.Is(err, context.DeadlineExceeded) {
				errType = "timeout"
				// Provide user-friendly timeout message
				errMsg = "Scan timed out after 2 minutes - this is normal for large networks. Servers found in earlier phases are still available."
			}
			result.AddError(phase.Name, errType, errMsg, "", 0)
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

func (s *Scanner) discoverAtPort(ctx context.Context, ip string, port int) *DiscoveredServer {
	probe := s.probeProxmoxService(ctx, ip, port)
	if probe == nil || !probe.Positive {
		if probe != nil && probe.Err != nil && !errors.Is(probe.Err, context.Canceled) {
			log.Debug().
				Str("ip", ip).
				Int("port", port).
				Float64("confidence", probe.PrimaryScore).
				Err(probe.Err).
				Msg("Probe completed without identification")
		}
		return nil
	}

	return s.buildServerFromProbe(ctx, probe)
}

func (s *Scanner) buildServerFromProbe(ctx context.Context, probe *ProxmoxProbeResult) *DiscoveredServer {
	product := strings.TrimSpace(probe.PrimaryProduct)
	if product == "" {
		log.Debug().
			Str("ip", probe.IP).
			Int("port", probe.Port).
			Float64("confidence", probe.PrimaryScore).
			Msg("Probe identified Proxmox server but product type is ambiguous")
		return nil
	}

	version := strings.TrimSpace(probe.Version)
	if version == "" {
		version = "Unknown"
	}

	server := &DiscoveredServer{
		IP:      probe.IP,
		Port:    probe.Port,
		Type:    product,
		Version: version,
		Release: probe.Release,
	}

	s.populateServerHostname(ctx, server)

	log.Info().
		Str("ip", server.IP).
		Int("port", server.Port).
		Str("type", server.Type).
		Str("version", server.Version).
		Float64("confidence", probe.PrimaryScore).
		Msg("Discovered Proxmox server")

	if len(probe.PositiveReasons) > 0 {
		log.Debug().
			Str("ip", server.IP).
			Int("port", server.Port).
			Str("type", server.Type).
			Float64("confidence", probe.PrimaryScore).
			Strs("evidence", probe.PositiveReasons).
			Msg("Probe evidence")
	}

	return server
}

func (s *Scanner) populateServerHostname(ctx context.Context, server *DiscoveredServer) {
	if server == nil {
		return
	}

	if s.policy.EnableReverseDNS {
		names, err := net.DefaultResolver.LookupAddr(ctx, server.IP)
		if err == nil && len(names) > 0 {
			hostname := strings.TrimSuffix(names[0], ".")
			if hostname != "" {
				server.Hostname = hostname
				log.Debug().
					Str("ip", server.IP).
					Int("port", server.Port).
					Str("hostname", hostname).
					Msg("Resolved hostname via reverse DNS")
				return
			}
		}
	}

	switch server.Type {
	case productPVE, productPBS:
		if hostname := s.fetchNodeHostname(ctx, server.IP, server.Port); hostname != "" {
			server.Hostname = hostname
			log.Debug().
				Str("ip", server.IP).
				Int("port", server.Port).
				Str("hostname", hostname).
				Msg("Resolved hostname via API nodes endpoint")
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
			for _, port := range proxmoxProbePorts {
				if server := s.discoverAtPort(ctx, ip, port); server != nil {
					resultChan <- discoveredResult{Phase: phase, Server: server}
				}
			}

			// Signal that this IP has been processed
			progressChan <- 1
		}
	}
}

// runPhaseWithProgress runs a scanning phase with progress tracking and reporting
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
		subnets         []net.IPNet
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
		copy(cloned.Subnets, phase.Subnets)
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

func newProxmoxProbeResult(ip string, port int) *ProxmoxProbeResult {
	return &ProxmoxProbeResult{
		IP:               ip,
		Port:             port,
		ProductScores:    map[string]float64{},
		ProductEvidence:  map[string][]string{},
		EndpointFindings: map[string]EndpointProbeFinding{},
	}
}

func (r *ProxmoxProbeResult) addConfidence(product, reason string, score float64, positive bool) {
	if score <= 0 {
		return
	}

	if product != "" {
		r.ProductScores[product] += score
		if reason != "" {
			r.ProductEvidence[product] = append(r.ProductEvidence[product], reason)
		}
	}

	if positive {
		if reason == "" {
			reason = "positive identification"
		}
		if product != "" {
			r.PositiveReasons = append(r.PositiveReasons, fmt.Sprintf("%s: %s", product, reason))
		} else {
			r.PositiveReasons = append(r.PositiveReasons, reason)
		}
	}
}

func (r *ProxmoxProbeResult) recordEndpoint(f EndpointProbeFinding) {
	if f.Endpoint == "" {
		return
	}
	if r.EndpointFindings == nil {
		r.EndpointFindings = map[string]EndpointProbeFinding{}
	}
	r.EndpointFindings[f.Endpoint] = f
}

func (r *ProxmoxProbeResult) endpointFinding(endpoint string) (EndpointProbeFinding, bool) {
	if r.EndpointFindings == nil {
		return EndpointProbeFinding{}, false
	}
	f, ok := r.EndpointFindings[endpoint]
	return f, ok
}

func (r *ProxmoxProbeResult) finalize() {
	var (
		bestProduct string
		bestScore   float64
	)
	for product, score := range r.ProductScores {
		if score > bestScore {
			bestProduct = product
			bestScore = score
		}
	}

	r.PrimaryProduct = bestProduct
	r.PrimaryScore = bestScore
	r.Positive = len(r.PositiveReasons) > 0
	if r.Positive {
		r.Err = nil
	}
}

func (s *Scanner) probeProxmoxService(ctx context.Context, ip string, port int) *ProxmoxProbeResult {
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	result := newProxmoxProbeResult(ip, port)

	tlsState, reachable, tlsErr := s.performTLSProbe(ctx, address)
	result.TLSState = tlsState
	result.Reachable = reachable
	result.TLSHandshakeError = tlsErr

	if !reachable {
		result.Err = tlsErr
		result.finalize()
		return result
	}

	versionFinding, version, release := s.probeVersionEndpoint(ctx, address)
	result.recordEndpoint(versionFinding)
	result.VersionStatus = versionFinding.Status
	result.VersionError = versionFinding.Error
	result.Version = version
	result.Release = release
	result.Headers = cloneHeader(versionFinding.Headers)

	s.applyProductMatchers(ctx, address, result)

	if strings.TrimSpace(result.Version) == "" {
		result.Version = "Unknown"
	}

	if !result.Positive {
		if tlsErr != nil {
			result.Err = tlsErr
		} else if result.VersionError != nil {
			result.Err = result.VersionError
		} else {
			result.Err = errors.New("no positive identification")
		}
	}

	result.finalize()
	return result
}

func (s *Scanner) applyProductMatchers(ctx context.Context, address string, result *ProxmoxProbeResult) {
	applySharedHeuristics(result)
	applyPVEHeuristics(result)
	s.applyPMGHeuristics(ctx, address, result)
	s.applyPBSHeuristics(ctx, address, result)
}

func applySharedHeuristics(result *ProxmoxProbeResult) {
	switch result.Port {
	case 8006:
		result.addConfidence(productPVE, "port 8006 reachable", 0.1, false)
		result.addConfidence(productPMG, "port 8006 reachable", 0.1, false)
	case 8007:
		result.addConfidence(productPBS, "port 8007 reachable", 0.2, false)
	}

	if result.TLSState != nil {
		if product := inferTypeFromCertificate(*result.TLSState); product != "" {
			result.addConfidence(product, "TLS certificate metadata", 0.6, true)
		}
	}

	versionFinding, ok := result.endpointFinding("api2/json/version")
	if !ok || versionFinding.Error != nil {
		return
	}

	switch versionFinding.Status {
	case http.StatusOK:
		if result.Version != "" {
			result.addConfidence("", "version endpoint JSON responded", 0.35, true)
		} else {
			result.addConfidence("", "version endpoint responded", 0.25, true)
		}

		if versionFinding.ProductGuess != "" {
			result.addConfidence(versionFinding.ProductGuess, "version endpoint headers", 0.45, true)
		}

		if guess := inferTypeFromMetadata(result.Version, result.Release); guess != "" {
			result.addConfidence(guess, "version payload metadata", 0.2, true)
		}

		if versionFinding.ProductGuess == "" {
			for _, product := range defaultProductsForPort(result.Port) {
				result.addConfidence(product, "version endpoint success without explicit product", 0.2, true)
			}
		}
	case http.StatusUnauthorized, http.StatusForbidden:
		if versionFinding.ProductGuess != "" {
			result.addConfidence(versionFinding.ProductGuess, "auth headers indicated product", 0.4, true)
		} else {
			for _, product := range defaultProductsForPort(result.Port) {
				reason := fmt.Sprintf("version endpoint on %s port requires authentication", product)
				result.addConfidence(product, reason, 0.25, true)
			}
		}
	}
}

func applyPVEHeuristics(result *ProxmoxProbeResult) {
	if result.Port != 8006 {
		return
	}

	lowerVersion := strings.ToLower(result.Version)
	lowerRelease := strings.ToLower(result.Release)

	if strings.Contains(lowerVersion, "pve") || strings.Contains(lowerRelease, "pve") {
		result.addConfidence(productPVE, "version metadata references pve", 0.4, true)
	}

	if result.Headers != nil {
		if server := strings.ToLower(result.Headers.Get("Server")); strings.Contains(server, "pve") {
			result.addConfidence(productPVE, "server header references pve", 0.35, true)
		}
	}

	if result.ProductScores[productPMG] >= productPositiveThreshold {
		return
	}

	if result.VersionStatus == http.StatusOK && result.Version != "" && result.ProductScores[productPVE] < 0.5 {
		result.addConfidence(productPVE, "version endpoint success on port 8006", 0.25, true)
	}
}

func (s *Scanner) applyPMGHeuristics(ctx context.Context, address string, result *ProxmoxProbeResult) {
	versionFinding, _ := result.endpointFinding("api2/json/version")
	hasPMGSignal := false

	if versionFinding.ProductGuess == productPMG {
		hasPMGSignal = true
	}

	if result.Headers != nil {
		if guess := inferTypeFromMetadata(
			result.Headers.Get("Proxmox-Product"),
			result.Headers.Get("Server"),
			result.Headers.Get("WWW-Authenticate"),
			strings.Join(result.Headers.Values("Set-Cookie"), " "),
		); guess == productPMG {
			result.addConfidence(productPMG, "version headers reference pmg", 0.45, true)
			hasPMGSignal = true
		}
	}

	if result.Port != 8006 && !hasPMGSignal {
		return
	}

	if result.ProductScores[productPMG] >= productPositiveThreshold {
		return
	}

	pmgEndpoints := []struct {
		Path   string
		Weight float64
	}{
		{"api2/json/statistics/mail", 0.4},
		{"api2/json/mail/queue", 0.35},
		{"api2/json/mail/quarantine", 0.35},
	}

	for _, endpoint := range pmgEndpoints {
		if _, ok := result.EndpointFindings[endpoint.Path]; !ok {
			finding := s.probeAPIEndpoint(ctx, address, endpoint.Path)
			result.recordEndpoint(finding)
		}

		finding, ok := result.endpointFinding(endpoint.Path)
		if !ok || finding.Error != nil {
			continue
		}

		if finding.ProductGuess == productPMG {
			result.addConfidence(productPMG, fmt.Sprintf("endpoint %s headers", endpoint.Path), endpoint.Weight, true)
			continue
		}

		if finding.Status != http.StatusNotFound && finding.Status != 0 {
			result.addConfidence(productPMG, fmt.Sprintf("endpoint %s responded", endpoint.Path), endpoint.Weight-0.05, true)
		}
	}
}

func (s *Scanner) applyPBSHeuristics(ctx context.Context, address string, result *ProxmoxProbeResult) {
	if result.Port != 8007 {
		return
	}

	versionFinding, _ := result.endpointFinding("api2/json/version")

	if result.Headers != nil {
		if guess := inferTypeFromMetadata(
			result.Headers.Get("Proxmox-Product"),
			result.Headers.Get("Server"),
			result.Headers.Get("WWW-Authenticate"),
			strings.Join(result.Headers.Values("Set-Cookie"), " "),
		); guess == productPBS {
			result.addConfidence(productPBS, "version headers reference pbs", 0.4, true)
		}
	}

	switch versionFinding.Status {
	case http.StatusOK:
		if result.Version != "" {
			result.addConfidence(productPBS, "version endpoint returned JSON", 0.45, true)
		} else {
			result.addConfidence(productPBS, "version endpoint responded", 0.35, true)
		}
	case http.StatusUnauthorized, http.StatusForbidden:
		if versionFinding.ProductGuess == productPBS {
			result.addConfidence(productPBS, "auth headers indicated PBS", 0.55, true)
		} else {
			// High confidence: Port 8007 + api2/json/version exists (even if auth required) is a very strong signal.
			// We bump this to ensure we cross the threshold and avoid probing other endpoints (status/datastore)
			// which would generate "authentication failure" logs on the server.
			result.addConfidence(productPBS, "version endpoint on PBS port requires auth", 0.55, true)
		}
	default:
		if result.Reachable && result.ProductScores[productPBS] < 0.25 {
			result.addConfidence(productPBS, "port 8007 reachable", 0.25, true)
		}
	}

	if result.ProductScores[productPBS] >= productPositiveThreshold {
		return
	}

	pbsEndpoints := []struct {
		Path        string
		Weight      float64
		SuccessNote string
	}{
		{"api2/json/status", 0.45, "status endpoint responded"},
		{"api2/json/config/datastore", 0.35, "datastore endpoint reachable"},
	}

	for _, endpoint := range pbsEndpoints {
		if _, ok := result.EndpointFindings[endpoint.Path]; !ok {
			finding := s.probeAPIEndpoint(ctx, address, endpoint.Path)
			result.recordEndpoint(finding)
		}

		finding, ok := result.endpointFinding(endpoint.Path)
		if !ok || finding.Error != nil {
			continue
		}

		if finding.ProductGuess == productPBS {
			result.addConfidence(productPBS, fmt.Sprintf("endpoint %s headers", endpoint.Path), endpoint.Weight, true)
			continue
		}

		if finding.Status != http.StatusNotFound && finding.Status != 0 {
			reason := endpoint.SuccessNote
			if endpoint.Path == "api2/json/config/datastore" && finding.Status == http.StatusUnauthorized {
				reason = "datastore endpoint requires auth"
			}
			result.addConfidence(productPBS, reason, endpoint.Weight-0.05, true)
		}
	}
}

func defaultProductsForPort(port int) []string {
	switch port {
	case 8006:
		return []string{productPVE, productPMG}
	case 8007:
		return []string{productPBS}
	default:
		return nil
	}
}

func (s *Scanner) fetchNodeHostname(ctx context.Context, ip string, port int) string {
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s/api2/json/nodes", address), nil)
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

func (s *Scanner) performTLSProbe(ctx context.Context, address string) (*tls.ConnectionState, bool, error) {
	timeout := s.policy.DialTimeout
	if timeout <= 0 {
		timeout = time.Second
	}

	dialer := &net.Dialer{Timeout: timeout}
	tlsConn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{InsecureSkipVerify: true})
	if err == nil {
		state := tlsConn.ConnectionState()
		tlsConn.Close()
		return &state, true, nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, tcpErr := dialer.DialContext(dialCtx, "tcp", address)
	if tcpErr != nil {
		return nil, false, tcpErr
	}
	conn.Close()

	return nil, true, err
}

func (s *Scanner) probeVersionEndpoint(ctx context.Context, address string) (EndpointProbeFinding, string, string) {
	const endpoint = "api2/json/version"

	finding := EndpointProbeFinding{Endpoint: endpoint}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s/%s", address, endpoint), nil)
	if err != nil {
		finding.Error = err
		return finding, "", ""
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		finding.Error = err
		return finding, "", ""
	}
	defer resp.Body.Close()

	finding.Status = resp.StatusCode
	finding.Headers = cloneHeader(resp.Header)
	finding.ProductGuess = inferTypeFromMetadata(
		resp.Header.Get("Server"),
		resp.Header.Get("Proxmox-Product"),
		resp.Header.Get("WWW-Authenticate"),
		strings.Join(resp.Header.Values("Set-Cookie"), " "),
	)

	if resp.StatusCode != http.StatusOK {
		return finding, "", ""
	}

	var payload struct {
		Data struct {
			Version string `json:"version"`
			Release string `json:"release,omitempty"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		finding.Error = err
		return finding, "", ""
	}

	return finding, payload.Data.Version, payload.Data.Release
}

func (s *Scanner) probeAPIEndpoint(ctx context.Context, address, endpoint string) EndpointProbeFinding {
	finding := EndpointProbeFinding{Endpoint: endpoint}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s/%s", address, endpoint), nil)
	if err != nil {
		finding.Error = err
		return finding
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		finding.Error = err
		return finding
	}
	defer resp.Body.Close()

	finding.Status = resp.StatusCode
	finding.Headers = cloneHeader(resp.Header)
	finding.ProductGuess = inferTypeFromMetadata(
		resp.Header.Get("Server"),
		resp.Header.Get("Proxmox-Product"),
		resp.Header.Get("WWW-Authenticate"),
		strings.Join(resp.Header.Values("Set-Cookie"), " "),
	)

	return finding
}

func cloneHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	c := make(http.Header, len(h))
	for k, values := range h {
		cp := make([]string, len(values))
		copy(cp, values)
		c[k] = cp
	}
	return c
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
