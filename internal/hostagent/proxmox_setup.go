package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
	proxmoxType        string // "pve", "pbs", or "" for auto-detect
	hostname           string
	insecureSkipVerify bool
}

// ProxmoxSetupResult contains the result of a successful Proxmox setup.
type ProxmoxSetupResult struct {
	ProxmoxType string // "pve" or "pbs"
	TokenID     string // Full token ID (e.g., "pulse-monitor@pam!pulse-1234567890")
	TokenValue  string // The secret token value
	NodeHost    string // The host URL for auto-registration
	Registered  bool   // Whether the node was successfully registered with Pulse
}

const (
	proxmoxUser    = "pulse-monitor"
	proxmoxUserPVE = "pulse-monitor@pam"
	proxmoxUserPBS = "pulse-monitor@pbs"
	proxmoxComment = "Pulse monitoring service"
	stateFilePath  = "/var/lib/pulse-agent/proxmox-registered"
	stateFileDir   = "/var/lib/pulse-agent"
)

// NewProxmoxSetup creates a new ProxmoxSetup instance.
func NewProxmoxSetup(logger zerolog.Logger, httpClient *http.Client, pulseURL, apiToken, proxmoxType, hostname string, insecure bool) *ProxmoxSetup {
	return &ProxmoxSetup{
		logger:             logger,
		httpClient:         httpClient,
		pulseURL:           strings.TrimRight(pulseURL, "/"),
		apiToken:           apiToken,
		proxmoxType:        proxmoxType,
		hostname:           hostname,
		insecureSkipVerify: insecure,
	}
}

// Run executes the Proxmox setup process:
// 1. Detects Proxmox type (if not specified)
// 2. Creates the monitoring user and API token
// 3. Registers the node with Pulse via auto-register
func (p *ProxmoxSetup) Run(ctx context.Context) (*ProxmoxSetupResult, error) {
	// Check if already registered (idempotency)
	if p.isAlreadyRegistered() {
		p.logger.Info().Msg("Proxmox node already registered, skipping setup")
		return nil, nil
	}

	// Detect Proxmox type
	ptype := p.proxmoxType
	if ptype == "" {
		detected := p.detectProxmoxType()
		if detected == "" {
			return nil, fmt.Errorf("this system does not appear to be a Proxmox VE or PBS node")
		}
		ptype = detected
		p.logger.Info().Str("type", ptype).Msg("Auto-detected Proxmox type")
	}

	// Create monitoring user and token
	tokenID, tokenValue, err := p.setupToken(ctx, ptype)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox API token: %w", err)
	}

	p.logger.Info().Str("token_id", tokenID).Msg("Created Proxmox API token")

	// Get the host URL for registration
	hostURL := p.getHostURL(ptype)

	// Register with Pulse
	registered := false
	if err := p.registerWithPulse(ctx, ptype, hostURL, tokenID, tokenValue); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to register with Pulse (node may already exist)")
	} else {
		registered = true
		p.markAsRegistered()
		p.logger.Info().Str("host", hostURL).Msg("Successfully registered Proxmox node with Pulse")
	}

	return &ProxmoxSetupResult{
		ProxmoxType: ptype,
		TokenID:     tokenID,
		TokenValue:  tokenValue,
		NodeHost:    hostURL,
		Registered:  registered,
	}, nil
}

// detectProxmoxType checks for pvesh (PVE) or proxmox-backup-manager (PBS).
func (p *ProxmoxSetup) detectProxmoxType() string {
	// Check for PVE first
	if _, err := exec.LookPath("pvesh"); err == nil {
		return "pve"
	}
	// Check for PBS
	if _, err := exec.LookPath("proxmox-backup-manager"); err == nil {
		return "pbs"
	}
	return ""
}

// setupToken creates the monitoring user and API token.
func (p *ProxmoxSetup) setupToken(ctx context.Context, ptype string) (string, string, error) {
	tokenName := fmt.Sprintf("pulse-%d", time.Now().Unix())

	if ptype == "pve" {
		return p.setupPVEToken(ctx, tokenName)
	}
	return p.setupPBSToken(ctx, tokenName)
}

// setupPVEToken creates a PVE monitoring user and token.
func (p *ProxmoxSetup) setupPVEToken(ctx context.Context, tokenName string) (string, string, error) {
	// Create user (ignore error if already exists)
	_ = runCommand(ctx, "pveum", "user", "add", proxmoxUserPVE, "--comment", proxmoxComment)

	// Add PVEAuditor role
	if err := runCommand(ctx, "pveum", "aclmod", "/", "-user", proxmoxUserPVE, "-role", "PVEAuditor"); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to add PVEAuditor role (may already exist)")
	}

	// Try to create PulseMonitor role with additional privileges
	_ = runCommand(ctx, "pveum", "role", "add", "PulseMonitor", "-privs", "Sys.Audit,VM.Monitor,Datastore.Audit")
	_ = runCommand(ctx, "pveum", "aclmod", "/", "-user", proxmoxUserPVE, "-role", "PulseMonitor")

	// Create token with privilege separation disabled
	output, err := runCommandOutput(ctx, "pveum", "user", "token", "add", proxmoxUserPVE, tokenName, "--privsep", "0")
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
	_ = runCommand(ctx, "proxmox-backup-manager", "user", "create", proxmoxUserPBS)

	// Add Audit role
	_ = runCommand(ctx, "proxmox-backup-manager", "acl", "update", "/", "Audit", "--auth-id", proxmoxUserPBS)

	// Create token
	output, err := runCommandOutput(ctx, "proxmox-backup-manager", "user", "generate-token", proxmoxUserPBS, tokenName)
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
	_ = runCommand(ctx, "proxmox-backup-manager", "acl", "update", "/", "Audit", "--auth-id", tokenID)

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
// Uses intelligent IP selection to prefer LAN addresses over internal cluster networks.
func (p *ProxmoxSetup) getHostURL(ptype string) string {
	port := "8006"
	if ptype == "pbs" {
		port = "8007"
	}

	// Get all IPs and select the best one for external access
	if out, err := exec.Command("hostname", "-I").Output(); err == nil {
		ips := strings.Fields(string(out))
		if len(ips) > 0 {
			bestIP := selectBestIP(ips)
			if bestIP != "" {
				return fmt.Sprintf("https://%s:%s", bestIP, port)
			}
		}
	}

	// Fallback to hostname if IP detection failed
	hostname := p.hostname
	if hostname == "" {
		hostname = "localhost"
	}
	return fmt.Sprintf("https://%s:%s", hostname, port)
}

// selectBestIP picks the most likely externally-reachable IP from a list.
// Prefers common LAN subnets (192.168.x.x, 10.x.x.x) and avoids internal
// cluster networks (like corosync's 172.20.x.x) and link-local addresses.
func selectBestIP(ips []string) string {
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
	fmt.Sscanf(parts[0], "%d", &first)
	fmt.Sscanf(parts[1], "%d", &second)

	// Scoring logic:
	// - 192.168.x.x: Very common home/office LAN, high priority (score 100)
	// - 10.0.x.x - 10.31.x.x: Common corporate LAN ranges (score 90)
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

// registerWithPulse calls the auto-register endpoint to add the node.
func (p *ProxmoxSetup) registerWithPulse(ctx context.Context, ptype, hostURL, tokenID, tokenValue string) error {
	payload := map[string]interface{}{
		"type":       ptype,
		"host":       hostURL,
		"serverName": p.hostname,
		"tokenId":    tokenID,
		"tokenValue": tokenValue,
		"source":     "agent", // Indicates this was registered via agent
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.pulseURL+"/api/auto-register", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", p.apiToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("auto-register returned %d", resp.StatusCode)
	}

	return nil
}

// isAlreadyRegistered checks if we've already done Proxmox setup.
func (p *ProxmoxSetup) isAlreadyRegistered() bool {
	_, err := os.Stat(stateFilePath)
	return err == nil
}

// markAsRegistered creates a state file to indicate setup is complete.
func (p *ProxmoxSetup) markAsRegistered() {
	if err := os.MkdirAll(stateFileDir, 0755); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to create state directory")
		return
	}

	if err := os.WriteFile(stateFilePath, []byte(time.Now().Format(time.RFC3339)), 0644); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to write state file")
	}
}

// runCommand executes a command and returns any error.
func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

// runCommandOutput executes a command and returns the output.
func runCommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
