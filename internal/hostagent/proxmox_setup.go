package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ProxmoxSetup handles one-time Proxmox API token creation and node registration.
type ProxmoxSetup struct {
	logger             zerolog.Logger
	httpClient         *http.Client
	pulseURL           string
	apiToken           string
	proxmoxType        proxmoxProductType // pve, pbs, or unknown for auto-detect
	hostname           string
	reportIP           string
	insecureSkipVerify bool
	collector          SystemCollector
	retryBackoffs      []time.Duration // overridable for testing; nil uses defaults
}

// ProxmoxSetupResult contains the result of a successful Proxmox setup.
type ProxmoxSetupResult struct {
	ProxmoxType string // "pve" or "pbs"
	TokenID     string // Full token ID (e.g., "pulse-monitor@pam!pulse-1234567890")
	TokenValue  string // The secret token value
	NodeHost    string // The host URL for auto-registration
	Registered  bool   // Whether the node was successfully registered with Pulse
}

type proxmoxProductType string

const (
	proxmoxProductUnknown proxmoxProductType = ""
	proxmoxProductPVE     proxmoxProductType = "pve"
	proxmoxProductPBS     proxmoxProductType = "pbs"
)

type autoRegisterSource string

const autoRegisterSourceAgent autoRegisterSource = "agent"

type autoRegisterRequest struct {
	Type       proxmoxProductType `json:"type"`
	Host       string             `json:"host"`
	ServerName string             `json:"serverName"`
	TokenID    string             `json:"tokenId"`
	TokenValue string             `json:"tokenValue"`
	Source     autoRegisterSource `json:"source"`
}

const (
	proxmoxUser    = "pulse-monitor"
	proxmoxUserPVE = "pulse-monitor@pam"
	proxmoxUserPBS = "pulse-monitor@pbs"
	proxmoxComment = "Pulse monitoring service"
)

const proxmoxMonitorRole = "PulseMonitor"

func privProbeRoleName(priv string) string {
	// Keep role name deterministic (helps tests) and valid for pveum.
	// Replace characters that are likely to cause issues in role names.
	safe := strings.NewReplacer(".", "_", ":", "_", "/", "_", " ", "_", ",", "_").Replace(priv)
	return "PulseTmpPrivCheck_" + safe
}

func (p *ProxmoxSetup) probePVEPrivilege(ctx context.Context, privilege string) bool {
	roleName := privProbeRoleName(privilege)

	// If privilege doesn't exist on this PVE version, pveum will fail with a non-zero exit code.
	if _, err := p.collector.CommandCombinedOutput(ctx, "pveum", "role", "add", roleName, "-privs", privilege); err != nil {
		return false
	}
	if _, err := p.collector.CommandCombinedOutput(ctx, "pveum", "role", "delete", roleName); err != nil {
		p.logger.Debug().Err(err).Str("role", roleName).Msg("Failed to cleanup temporary PVE privilege probe role")
	}
	return true
}

func (p *ProxmoxSetup) configurePVEPermissions(ctx context.Context) {
	// Baseline: read-only access.
	if _, err := p.collector.CommandCombinedOutput(ctx, "pveum", "aclmod", "/", "-user", proxmoxUserPVE, "-role", "PVEAuditor"); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to add PVEAuditor role (may already exist)")
	}

	// Extra privileges are optional, but enable additional features:
	// - Sys.Audit: required for pending apt updates + some cluster/ceph visibility
	// - VM.Monitor (PVE 8) or VM.GuestAgent.Audit (PVE 9+): guest agent data
	// - Datastore.Audit: improved storage visibility
	var extraPrivs []string

	if p.probePVEPrivilege(ctx, "Sys.Audit") {
		extraPrivs = append(extraPrivs, "Sys.Audit")
	}

	if p.probePVEPrivilege(ctx, "VM.Monitor") {
		extraPrivs = append(extraPrivs, "VM.Monitor")
	} else if p.probePVEPrivilege(ctx, "VM.GuestAgent.Audit") {
		extraPrivs = append(extraPrivs, "VM.GuestAgent.Audit")
	}

	if p.probePVEPrivilege(ctx, "Datastore.Audit") {
		extraPrivs = append(extraPrivs, "Datastore.Audit")
	}

	if len(extraPrivs) > 0 {
		privString := strings.Join(extraPrivs, ",")

		// Prefer modify (non-destructive) in case the role already exists.
		if _, err := p.collector.CommandCombinedOutput(ctx, "pveum", "role", "modify", proxmoxMonitorRole, "-privs", privString); err != nil {
			if _, addErr := p.collector.CommandCombinedOutput(ctx, "pveum", "role", "add", proxmoxMonitorRole, "-privs", privString); addErr != nil {
				p.logger.Warn().Err(addErr).Str("privs", privString).Msg("Failed to configure PulseMonitor role")
				return
			}
		}

		if _, err := p.collector.CommandCombinedOutput(ctx, "pveum", "aclmod", "/", "-user", proxmoxUserPVE, "-role", proxmoxMonitorRole); err != nil {
			p.logger.Warn().Err(err).Msg("Failed to apply PulseMonitor role")
		}
	}

	// Add PVEDatastoreAdmin on /storage for backup visibility (issue #1139)
	if _, err := p.collector.CommandCombinedOutput(ctx, "pveum", "aclmod", "/storage", "-user", proxmoxUserPVE, "-role", "PVEDatastoreAdmin"); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to apply PVEDatastoreAdmin role on /storage")
	}
}

var (
	stateFilePath = "/var/lib/pulse-agent/proxmox-registered" // Legacy, kept for backward compat
	stateFileDir  = "/var/lib/pulse-agent"
	// Per-type state files for multi-product support (PVE+PBS on same host)
	stateFilePVE = "/var/lib/pulse-agent/proxmox-pve-registered"
	stateFilePBS = "/var/lib/pulse-agent/proxmox-pbs-registered"
)

const (
	proxmoxStateDirPerm  = 0700
	proxmoxStateFilePerm = 0600
)

func parseProxmoxProductType(rawType string) proxmoxProductType {
	switch strings.ToLower(strings.TrimSpace(rawType)) {
	case string(proxmoxProductPVE):
		return proxmoxProductPVE
	case string(proxmoxProductPBS):
		return proxmoxProductPBS
	default:
		return proxmoxProductUnknown
	}
}

func proxmoxProductTypesToStrings(types []proxmoxProductType) []string {
	if len(types) == 0 {
		return nil
	}

	result := make([]string, 0, len(types))
	for _, t := range types {
		result = append(result, string(t))
	}

	return result
}

// NewProxmoxSetup creates a new ProxmoxSetup instance.
func NewProxmoxSetup(logger zerolog.Logger, httpClient *http.Client, collector SystemCollector, pulseURL, apiToken, proxmoxType, hostname, reportIP string, insecure bool) *ProxmoxSetup {
	return &ProxmoxSetup{
		logger:             logger,
		httpClient:         httpClient,
		collector:          collector,
		pulseURL:           strings.TrimRight(pulseURL, "/"),
		apiToken:           apiToken,
		proxmoxType:        proxmoxProductType(proxmoxType),
		hostname:           hostname,
		reportIP:           reportIP,
		insecureSkipVerify: insecure,
	}
}

// Run executes the Proxmox setup process:
// 1. Detects Proxmox type (if not specified)
// 2. Creates the monitoring user and API token
// 3. Registers the node with Pulse via auto-register
func (p *ProxmoxSetup) Run(ctx context.Context) (*ProxmoxSetupResult, error) {
	if strings.TrimSpace(p.apiToken) == "" {
		return nil, errors.New("api token is required")
	}

	pulseURL, err := normalizePulseURL(p.pulseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid pulse URL: %w", err)
	}
	p.pulseURL = pulseURL

	ptypeStr, err := normalizeProxmoxType(string(p.proxmoxType))
	if err != nil {
		return nil, err
	}
	ptype := proxmoxProductType(ptypeStr)

	// Check if already registered (idempotency)
	if p.isAlreadyRegistered() {
		p.logger.Info().Msg("Proxmox node already registered, skipping setup")
		return nil, nil
	}

	// Detect Proxmox type
	if ptype == proxmoxProductUnknown {
		detected := p.detectProxmoxType()
		if detected == proxmoxProductUnknown {
			return nil, fmt.Errorf("this system does not appear to be a Proxmox VE or PBS node")
		}
		ptype = detected
		p.logger.Info().Str("type", string(ptype)).Msg("Auto-detected Proxmox type")
	}

	// Create monitoring user and token
	tokenID, tokenValue, err := p.setupToken(ctx, ptype)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox API token: %w", err)
	}

	p.logger.Info().Str("token_id", tokenID).Msg("Created Proxmox API token")

	// Get the host URL for registration
	hostURL := p.getHostURL(ctx, ptype)

	// Register with Pulse
	registered := false
	if err := p.registerWithPulse(ctx, ptype, hostURL, tokenID, tokenValue); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to register with Pulse (node may already exist)")
	} else {
		registered = true
		p.markAsRegistered()
		p.markTypeAsRegistered(ptype)
		p.logger.Info().Str("host", hostURL).Msg("Successfully registered Proxmox node with Pulse")
	}

	return &ProxmoxSetupResult{
		ProxmoxType: string(ptype),
		TokenID:     tokenID,
		TokenValue:  tokenValue,
		NodeHost:    hostURL,
		Registered:  registered,
	}, nil
}

// RunAll detects and registers ALL Proxmox products on this system.
// This supports hosts with both PVE and PBS installed (a common and officially
// supported configuration). Each type gets its own registration and state tracking.
// Returns results for all types that were processed (skipping already-registered ones).
func (p *ProxmoxSetup) RunAll(ctx context.Context) ([]*ProxmoxSetupResult, error) {
	if strings.TrimSpace(p.apiToken) == "" {
		return nil, errors.New("api token is required")
	}

	pulseURL, err := normalizePulseURL(p.pulseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid pulse URL: %w", err)
	}
	p.pulseURL = pulseURL

	forcedTypeStr, err := normalizeProxmoxType(string(p.proxmoxType))
	if err != nil {
		return nil, err
	}
	forcedType := proxmoxProductType(forcedTypeStr)

	var results []*ProxmoxSetupResult

	// If a specific type is forced, only run that
	if forcedType != "" {
		result, err := p.runForType(ctx, forcedType)
		if err != nil {
			return nil, fmt.Errorf("setup proxmox type %s: %w", p.proxmoxType, err)
		}
		if result != nil {
			results = append(results, result)
		}
		return results, nil
	}

	// Detect all Proxmox products
	types := p.detectProxmoxTypes()
	if len(types) == 0 {
		return nil, fmt.Errorf("this system does not appear to be a Proxmox VE or PBS node")
	}

	p.logger.Info().Strs("types", proxmoxProductTypesToStrings(types)).Msg("Auto-detected Proxmox products")

	// Register each type
	for _, ptype := range types {
		result, err := p.runForType(ctx, ptype)
		if err != nil {
			p.logger.Error().Err(err).Str("type", string(ptype)).Msg("Failed to setup Proxmox type")
			continue // Don't fail completely, try other types
		}
		if result != nil {
			results = append(results, result)
		}
	}

	return results, nil
}

// runForType executes setup for a specific Proxmox type.
func (p *ProxmoxSetup) runForType(ctx context.Context, ptype proxmoxProductType) (*ProxmoxSetupResult, error) {
	normalizedTypeStr, err := normalizeProxmoxType(string(ptype))
	if err != nil || normalizedTypeStr == "" {
		return nil, fmt.Errorf("invalid proxmox type %q: must be pve or pbs", ptype)
	}
	ptype = proxmoxProductType(normalizedTypeStr)

	// Check if this type is already registered
	if p.isTypeRegistered(ptype) {
		p.logger.Info().Str("type", string(ptype)).Msg("Proxmox type already registered, skipping")
		return nil, nil
	}

	p.logger.Info().Str("type", string(ptype)).Msg("Setting up Proxmox type")

	// Create monitoring user and token
	tokenID, tokenValue, err := p.setupToken(ctx, ptype)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s API token: %w", ptype, err)
	}

	p.logger.Info().Str("type", string(ptype)).Str("token_id", tokenID).Msg("Created Proxmox API token")

	// Get the host URL for registration
	hostURL := p.getHostURL(ctx, ptype)

	// Register with Pulse
	registered := false
	if err := p.registerWithPulse(ctx, ptype, hostURL, tokenID, tokenValue); err != nil {
		p.logger.Warn().Err(err).Str("type", string(ptype)).Msg("Failed to register with Pulse (node may already exist)")
	} else {
		registered = true
		p.markTypeAsRegistered(ptype)
		p.logger.Info().Str("type", string(ptype)).Str("host", hostURL).Msg("Successfully registered Proxmox node with Pulse")
	}

	return &ProxmoxSetupResult{
		ProxmoxType: string(ptype),
		TokenID:     tokenID,
		TokenValue:  tokenValue,
		NodeHost:    hostURL,
		Registered:  registered,
	}, nil
}

// detectProxmoxType checks for pvesh (PVE) or proxmox-backup-manager (PBS).
// For backward compatibility, returns the first detected type.
// Use detectProxmoxTypes() to get all detected types.
func (p *ProxmoxSetup) detectProxmoxType() proxmoxProductType {
	types := p.detectProxmoxTypes()
	if len(types) > 0 {
		return types[0]
	}
	return proxmoxProductUnknown
}

// detectProxmoxTypes checks for ALL Proxmox products on this system.
// Returns a slice of detected types (e.g., ["pve", "pbs"] if both are installed).
// This is common when PBS is installed directly on a PVE host.
// detectProxmoxTypes checks for ALL Proxmox products on this system.
// Returns a slice of detected types (e.g., ["pve", "pbs"] if both are installed).
// This is common when PBS is installed directly on a PVE host.
func (p *ProxmoxSetup) detectProxmoxTypes() []proxmoxProductType {
	var types []proxmoxProductType

	// Check for PVE
	if _, err := p.collector.LookPath("pvesh"); err == nil {
		types = append(types, proxmoxProductPVE)
	}

	// Check for PBS
	if _, err := p.collector.LookPath("proxmox-backup-manager"); err == nil {
		types = append(types, proxmoxProductPBS)
	}

	return types
}

// setupToken creates the monitoring user and API token.
func (p *ProxmoxSetup) setupToken(ctx context.Context, ptype proxmoxProductType) (string, string, error) {
	tokenName := fmt.Sprintf("pulse-%d", time.Now().Unix())

	switch ptype {
	case proxmoxProductPVE:
		return p.setupPVEToken(ctx, tokenName)
	case proxmoxProductPBS:
		return p.setupPBSToken(ctx, tokenName)
	default:
		return "", "", fmt.Errorf("unsupported Proxmox type %q", ptype)
	}
}

// setupPVEToken creates a PVE monitoring user and token.
func (p *ProxmoxSetup) setupPVEToken(ctx context.Context, tokenName string) (string, string, error) {
	// Create user (ignore error if already exists)
	if out, err := p.collector.CommandCombinedOutput(ctx, "pveum", "user", "add", proxmoxUserPVE, "--comment", proxmoxComment); err != nil {
		if isAlreadyExistsOutput(out) {
			p.logger.Debug().Msg("Proxmox PVE monitor user already exists")
		} else {
			p.logger.Warn().Err(err).Str("output", strings.TrimSpace(out)).Msg("Failed to create Proxmox PVE monitor user")
		}
	}

	// Apply baseline + optional enhanced permissions (Sys.Audit, guest agent access).
	p.configurePVEPermissions(ctx)

	// Create token with privilege separation disabled
	output, err := p.collector.CommandCombinedOutput(ctx, "pveum", "user", "token", "add", proxmoxUserPVE, tokenName, "--privsep", "0")
	if err != nil {
		return "", "", fmt.Errorf("failed to create token: %w", err)
	}

	// Parse token value from output
	tokenValue := p.parseTokenValue(output)
	if tokenValue == "" {
		return "", "", fmt.Errorf("failed to parse token value from pveum output")
	}

	tokenID := fmt.Sprintf("%s!%s", proxmoxUserPVE, tokenName)
	return tokenID, tokenValue, nil
}

// setupPBSToken creates a PBS monitoring user and token.
func (p *ProxmoxSetup) setupPBSToken(ctx context.Context, tokenName string) (string, string, error) {
	// Create user (ignore error if already exists)
	if out, err := p.collector.CommandCombinedOutput(ctx, "proxmox-backup-manager", "user", "create", proxmoxUserPBS); err != nil {
		if isAlreadyExistsOutput(out) {
			p.logger.Debug().Msg("Proxmox PBS monitor user already exists")
		} else {
			p.logger.Warn().Err(err).Str("output", strings.TrimSpace(out)).Msg("Failed to create Proxmox PBS monitor user")
		}
	}

	// Add Audit role
	if out, err := p.collector.CommandCombinedOutput(ctx, "proxmox-backup-manager", "acl", "update", "/", "Audit", "--auth-id", proxmoxUserPBS); err != nil {
		p.logger.Warn().Err(err).Str("output", strings.TrimSpace(out)).Msg("Failed to apply PBS Audit role to monitor user")
	}

	// Create token
	output, err := p.collector.CommandCombinedOutput(ctx, "proxmox-backup-manager", "user", "generate-token", proxmoxUserPBS, tokenName)
	if err != nil {
		return "", "", fmt.Errorf("failed to create token: %w", err)
	}

	// Parse token value from JSON output
	tokenValue := p.parsePBSTokenValue(output)
	if tokenValue == "" {
		return "", "", fmt.Errorf("failed to parse token value from PBS output")
	}

	// Add Audit role for the token itself
	tokenID := fmt.Sprintf("%s!%s", proxmoxUserPBS, tokenName)
	if out, err := p.collector.CommandCombinedOutput(ctx, "proxmox-backup-manager", "acl", "update", "/", "Audit", "--auth-id", tokenID); err != nil {
		p.logger.Warn().Err(err).Str("token_id", tokenID).Str("output", strings.TrimSpace(out)).Msg("Failed to apply PBS Audit role to token")
	}

	return tokenID, tokenValue, nil
}

// parseTokenValue extracts the token value from PVE pveum output.
// The output format is typically a table with columns: │ key │ value │
// Example output:
// ┌──────────────┬──────────────────────────────────────┐
// │ key          │ value                                │
// ╞══════════════╪══════════════════════════════════════╡
// │ full-tokenid │ pulse-monitor@pam!pulse-token        │
// ├──────────────┼──────────────────────────────────────┤
// │ info         │ {"privsep":"0"}                      │
// ├──────────────┼──────────────────────────────────────┤
// │ value        │ xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx │
// └──────────────┴──────────────────────────────────────┘
func (p *ProxmoxSetup) parseTokenValue(output string) string {
	// Look for the "value" row in the table output - where "value" is the KEY column
	// We need to find the row where parts[1] (key column) is "value", not any row
	// containing the word "value" (which would match the header row)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Split by the box drawing character │ (U+2502)
		parts := strings.Split(line, "│")
		if len(parts) >= 3 {
			key := strings.TrimSpace(parts[1])
			if key == "value" {
				return strings.TrimSpace(parts[2])
			}
		}
	}

	// Fallback: try regex for UUID-like token
	re := regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
	if match := re.FindString(output); match != "" {
		return match
	}

	return ""
}

// parsePBSTokenValue extracts the token value from PBS JSON output.
func (p *ProxmoxSetup) parsePBSTokenValue(output string) string {
	// PBS outputs JSON like: {"tokenid": "...", "value": "..."}
	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(output), &result); err == nil && result.Value != "" {
		return result.Value
	}

	// Fallback: extract from "value": "..." pattern
	re := regexp.MustCompile(`"value"\s*:\s*"([^"]+)"`)
	if matches := re.FindStringSubmatch(output); len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

// getHostURL constructs the host URL for this Proxmox node.
// Uses the local IP that can reach Pulse, falling back to intelligent IP selection.
func (p *ProxmoxSetup) getHostURL(ctx context.Context, ptype proxmoxProductType) string {
	port := "8006"
	if ptype == proxmoxProductPBS {
		port = "8007"
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// Priority 1: User-specified ReportIP override from configuration.
	// This allows users to manually specify which IP should be used for Proxmox API
	// connections when auto-detection picks the wrong one (e.g., Issue #1061).
	if p.reportIP != "" {
		p.logger.Info().Str("ip", p.reportIP).Msg("Using user-specified ReportIP for Proxmox registration")
		return formatHTTPSURL(p.reportIP, port)
	}

	// Priority 2: Try to determine which local IP is used to connect to Pulse
	// This ensures we pick an IP that can actually communicate with the Pulse server
	if reachableIP := p.getIPThatReachesPulse(); reachableIP != "" {
		p.logger.Debug().Str("ip", reachableIP).Msg("Using IP that can reach Pulse server")
		return formatHTTPSURL(reachableIP, port)
	}

	// Fallback: Get all IPs and select the best one based on heuristics
	hostnameCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if out, err := p.collector.CommandCombinedOutput(hostnameCtx, "hostname", "-I"); err == nil {
		ips := strings.Fields(out)
		if len(ips) > 0 {
			// Get the IP that the system hostname currently resolves to
			hostnameIP := p.getIPForHostname()

			bestIP := selectBestIP(ips, hostnameIP)
			if bestIP != "" {
				return formatHTTPSURL(bestIP, port)
			}
		}
	}

	// Final fallback to hostname if IP detection failed
	hostname := p.hostname
	if hostname == "" {
		hostname = "localhost"
	}
	return formatHTTPSURL(hostname, port)
}

// getIPThatReachesPulse determines which local IP is used to connect to the Pulse server.
// This handles cases where multiple network interfaces exist (e.g., management, Ceph, cluster ring)
// and ensures we pick the one that can actually reach Pulse. Related to #929.
func (p *ProxmoxSetup) getIPThatReachesPulse() string {
	if p.pulseURL == "" {
		return ""
	}

	// Parse the Pulse URL to get host:port
	pulseServerURL, err := url.Parse(p.pulseURL)
	if err != nil {
		return ""
	}

	host := pulseServerURL.Hostname()
	if host == "" {
		return ""
	}

	port := pulseServerURL.Port()
	if port == "" {
		switch pulseServerURL.Scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		default:
			port = "7655"
		}
	}
	target := net.JoinHostPort(host, port)

	// Create a UDP "connection" to determine local address (doesn't actually send data)
	// We use a short timeout as this is just a routing table lookup.
	conn, err := p.collector.DialTimeout("udp", target, 500*time.Millisecond)
	if err != nil {
		p.logger.Debug().Err(err).Str("target", target).Msg("Could not determine local IP for Pulse connection (routing check failed)")
		return ""
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			p.logger.Debug().Err(closeErr).Msg("Failed to close local route probe connection")
		}
	}()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr == nil {
		return ""
	}
	ip := localAddr.IP.String()

	// If we found an IP that can reach Pulse, it's the most reliable one to report.
	// We only skip loopback/link-local which net.Dial UDP shouldn't return anyway.
	if ip != "" && ip != "127.0.0.1" && ip != "::1" && !strings.HasPrefix(ip, "fe80:") {
		return ip
	}
	return ""
}

func formatHTTPSURL(host, port string) string {
	return (&url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, port),
	}).String()
}

// getIPForHostname resolves the system hostname to an IP address.
func (p *ProxmoxSetup) getIPForHostname() string {
	hostname := p.hostname
	if hostname == "" {
		var err error
		hostname, err = p.collector.Hostname()
		if err != nil {
			p.logger.Debug().Err(err).Msg("Could not resolve system hostname")
			return ""
		}
	}
	if hostname == "" {
		return ""
	}

	ips, err := p.collector.LookupIP(hostname)
	if err != nil {
		p.logger.Debug().Err(err).Str("hostname", hostname).Msg("Could not resolve hostname IP")
		return ""
	}

	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	return ""
}

// selectBestIP picks the most likely externally-reachable IP from a list.
// Prefers common LAN subnets (192.168.x.x, 10.x.x.x) and avoids internal
// cluster networks (like corosync's 172.20.x.x) and link-local addresses.
// If hostnameIP is provided and present in the list, it is given the highest priority.
func selectBestIP(ips []string, hostnameIP string) string {
	type scoredIP struct {
		ip    string
		score int
	}

	var candidates []scoredIP

	for _, ip := range ips {
		// Skip loopback and IPv6 link-local
		if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "fe80:") {
			continue
		}

		// Skip IPv6 for simplicity (most Proxmox setups use IPv4)
		if strings.Contains(ip, ":") {
			continue
		}

		score := scoreIPv4(ip)

		// If this IP matches the system hostname, give it a significant bonus.
		// A bonus of +40 ensures that a 172.16.x.x (score 50) match (50+40=90)
		// correctly loses to a 192.168.x.x (score 100) interface if both are present.
		// However, it correctly breaks ties between equal-scored ranges.
		if hostnameIP != "" && ip == hostnameIP {
			score += 40
		}

		if score > 0 {
			candidates = append(candidates, scoredIP{ip: ip, score: score})
		}
	}

	if len(candidates) == 0 {
		// Fallback: return first non-loopback IP if no good candidates
		for _, ip := range ips {
			if ip != "127.0.0.1" && ip != "::1" && !strings.HasPrefix(ip, "fe80:") {
				return ip
			}
		}
		return ""
	}

	// Return highest scored IP
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}
	return best.ip
}

// scoreIPv4 assigns a preference score to an IPv4 address.
// Higher score = more likely to be externally reachable.
func scoreIPv4(ip string) int {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 0
	}

	// Parse first two octets
	first := 0
	second := 0
	if _, err := fmt.Sscanf(parts[0], "%d", &first); err != nil {
		return 0
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &second); err != nil {
		return 0
	}

	// Scoring logic:
	// - 192.168.x.x: Very common home/office LAN, highest priority (score 100)
	// - 10.0.x.x - 10.31.x.x: Common corporate LAN ranges (score 90)
	// - 100.64.x.x: Tailscale / CGNAT (score 85)
	// - 10.x.x.x (other): Less common, but still likely LAN (score 70)
	// - 172.16-31.x.x: Private range, often used for internal clusters (score 50)
	//   Corosync often uses 172.20.x.x or similar for ring0/ring1
	// - 169.254.x.x: Link-local, skip (score 0)
	// - Other private/public: Unknown, low priority (score 30)

	switch {
	case first == 192 && second == 168:
		return 100 // Most common home/office LAN
	case first == 10 && second <= 31:
		return 90 // Common corporate LAN
	case first == 100 && (second >= 64 && second <= 127):
		return 85 // Tailscale / CGNAT
	case first == 10:
		return 70 // Other 10.x.x.x
	case first == 172 && second >= 16 && second <= 31:
		// This is private 172.16-31.x.x range, often used for internal clusters
		// Corosync commonly uses 172.20.x.x for cluster communication
		return 50 // Lower priority - often internal cluster
	case first == 169 && second == 254:
		return 0 // Link-local, skip
	default:
		return 30 // Unknown/public - low priority
	}
}

// clientError represents a non-retryable HTTP client error (4xx).
type clientError struct {
	statusCode int
	body       string
}

func (e *clientError) Error() string {
	return fmt.Sprintf("auto-register returned HTTP %d: %s", e.statusCode, e.body)
}

// permanentError wraps an error that should not be retried (e.g. malformed URL).
type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func (p *ProxmoxSetup) doRegisterRequest(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.pulseURL+"/api/auto-register", bytes.NewReader(body))
	if err != nil {
		return &permanentError{fmt.Errorf("create request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", p.apiToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			p.logger.Debug().Err(closeErr).Msg("Failed to close auto-register response body")
		}
	}()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		bodyStr := strings.TrimSpace(string(bodyBytes))
		if resp.StatusCode < 500 {
			return &clientError{statusCode: resp.StatusCode, body: bodyStr}
		}
		return fmt.Errorf("auto-register returned HTTP %d: %s", resp.StatusCode, bodyStr)
	}
	return nil
}

// registerWithPulse calls the auto-register endpoint to add the node.
func (p *ProxmoxSetup) registerWithPulse(ctx context.Context, ptype proxmoxProductType, hostURL, tokenID, tokenValue string) error {
	payload := autoRegisterRequest{
		Type:       ptype,
		Host:       hostURL,
		ServerName: p.hostname,
		TokenID:    tokenID,
		TokenValue: tokenValue,
		Source:     autoRegisterSourceAgent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	backoffs := p.retryBackoffs
	if backoffs == nil {
		backoffs = []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 60 * time.Second}
	}

	var lastErr error
	for attempt := 0; attempt <= len(backoffs); attempt++ {
		if attempt > 0 {
			p.logger.Info().
				Int("attempt", attempt+1).
				Int("max_attempts", len(backoffs)+1).
				Str("type", string(ptype)).
				Msg("Retrying Proxmox auto-registration with Pulse")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffs[attempt-1]):
			}
		}

		err := p.doRegisterRequest(ctx, body)
		if err == nil {
			if attempt > 0 {
				p.logger.Info().Int("attempt", attempt+1).Str("type", string(ptype)).Msg("Proxmox auto-registration succeeded after retry")
			}
			return nil
		}
		lastErr = err

		// Don't retry client errors (4xx) or permanent errors (e.g. malformed URL).
		var ce *clientError
		var pe *permanentError
		if errors.As(err, &ce) || errors.As(err, &pe) {
			return err
		}

		p.logger.Warn().Err(err).
			Int("attempt", attempt+1).
			Int("max_attempts", len(backoffs)+1).
			Str("type", string(ptype)).
			Msg("Proxmox auto-registration attempt failed")
	}

	return fmt.Errorf("all %d registration attempts failed: %w", len(backoffs)+1, lastErr)
}

func isAlreadyExistsOutput(output string) bool {
	text := strings.ToLower(strings.TrimSpace(output))
	return strings.Contains(text, "already exists")
}

// isAlreadyRegistered checks if we've already done Proxmox setup.
// This uses the legacy single state file for backward compatibility.
func (p *ProxmoxSetup) isAlreadyRegistered() bool {
	_, err := p.collector.Stat(stateFilePath)
	return err == nil
}

// isTypeRegistered checks if a specific Proxmox type has been registered.
func (p *ProxmoxSetup) isTypeRegistered(ptype proxmoxProductType) bool {
	// Check per-type state file first (new behavior)
	stateFile := p.stateFileForType(ptype)
	if _, err := p.collector.Stat(stateFile); err == nil {
		return true
	}

	// Check legacy state file for backward compat
	if _, err := p.collector.Stat(stateFilePath); err == nil {
		// Legacy file exists. The old detection logic was:
		// 1. If pvesh exists → registered "pve"
		// 2. Else if proxmox-backup-manager exists → registered "pbs"
		//
		// So we need to figure out what was likely registered.
		// If PVE is currently installed, treat PVE as registered.
		// If only PBS is installed (no PVE), treat PBS as registered.
		types := p.detectProxmoxTypes()
		if len(types) == 0 {
			return false
		}

		// The old code registered the "first" type it found
		// If PVE is installed, it was always detected first
		if ptype == proxmoxProductPVE {
			for _, t := range types {
				if t == proxmoxProductPVE {
					return true
				}
			}
		}
		// If PBS-only host (no PVE), PBS was what was registered
		if ptype == proxmoxProductPBS {
			hasPVE := false
			hasPBS := false
			for _, t := range types {
				if t == proxmoxProductPVE {
					hasPVE = true
				}
				if t == proxmoxProductPBS {
					hasPBS = true
				}
			}
			if hasPBS && !hasPVE {
				return true
			}
		}
	}

	return false
}

// stateFileForType returns the state file path for a specific Proxmox type.
func (p *ProxmoxSetup) stateFileForType(ptype proxmoxProductType) string {
	switch ptype {
	case proxmoxProductPVE:
		return stateFilePVE
	case proxmoxProductPBS:
		return stateFilePBS
	default:
		return stateFilePath
	}
}

// markAsRegistered creates a state file to indicate setup is complete.
func (p *ProxmoxSetup) markAsRegistered() {
	if err := p.collector.MkdirAll(stateFileDir, proxmoxStateDirPerm); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to create state directory")
		return
	}
	if err := p.collector.Chmod(stateFileDir, proxmoxStateDirPerm); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to enforce state directory permissions")
	}

	if err := p.collector.WriteFile(stateFilePath, []byte(time.Now().Format(time.RFC3339)), proxmoxStateFilePerm); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to write state file")
		return
	}
	if err := p.collector.Chmod(stateFilePath, proxmoxStateFilePerm); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to enforce state file permissions")
	}
}

// markTypeAsRegistered creates a state file for a specific Proxmox type.
func (p *ProxmoxSetup) markTypeAsRegistered(ptype proxmoxProductType) {
	if err := p.collector.MkdirAll(stateFileDir, proxmoxStateDirPerm); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to create state directory")
		return
	}
	if err := p.collector.Chmod(stateFileDir, proxmoxStateDirPerm); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to enforce state directory permissions")
	}

	stateFile := p.stateFileForType(ptype)
	if err := p.collector.WriteFile(stateFile, []byte(time.Now().Format(time.RFC3339)), proxmoxStateFilePerm); err != nil {
		p.logger.Warn().Err(err).Str("type", string(ptype)).Msg("Failed to write type state file")
		return
	}
	if err := p.collector.Chmod(stateFile, proxmoxStateFilePerm); err != nil {
		p.logger.Warn().Err(err).Str("type", string(ptype)).Msg("Failed to enforce type state file permissions")
	}
}
