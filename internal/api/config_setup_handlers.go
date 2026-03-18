package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// HandleSetupScript serves the setup script for Proxmox/PBS nodes
func (h *ConfigHandlers) handleSetupScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	query := r.URL.Query()
	serverType := strings.TrimSpace(query.Get("type")) // "pve" or "pbs"
	serverHost := strings.TrimSpace(query.Get("host"))
	pulseURL := strings.TrimSpace(query.Get("pulse_url"))     // URL of the Pulse server for auto-registration
	backupPerms := query.Get("backup_perms") == "true"        // Whether to add backup management permissions
	setupToken := strings.TrimSpace(query.Get("setup_token")) // Temporary setup token for auto-registration

	// Validate required parameters
	if serverType == "" {
		http.Error(w, "Missing required parameter: type (must be 'pve' or 'pbs')", http.StatusBadRequest)
		return
	}
	if !isCanonicalAutoRegisterType(serverType) {
		http.Error(w, "type must be 'pve' or 'pbs'", http.StatusBadRequest)
		return
	}
	if serverHost == "" {
		http.Error(w, "Missing required parameter: host", http.StatusBadRequest)
		return
	}
	if safeHost, err := sanitizeInstallerURL(serverHost); err != nil {
		http.Error(w, fmt.Sprintf("Invalid host parameter: %v", err), http.StatusBadRequest)
		return
	} else {
		serverHost = safeHost
	}
	if normalizedHost, err := normalizeNodeHost(serverHost, serverType); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	} else {
		serverHost = normalizedHost
	}
	if pulseURL == "" {
		http.Error(w, "Missing required parameter: pulse_url", http.StatusBadRequest)
		return
	}
	if safeURL, err := sanitizeInstallerURL(pulseURL); err != nil {
		http.Error(w, fmt.Sprintf("Invalid pulse_url parameter: %v", err), http.StatusBadRequest)
		return
	} else {
		pulseURL = safeURL
	}
	if backupPerms && serverType != "pve" {
		http.Error(w, "backup_perms is only supported for type 'pve'", http.StatusBadRequest)
		return
	}
	if sanitizedToken, err := sanitizeSetupAuthToken(setupToken); err != nil {
		http.Error(w, fmt.Sprintf("Invalid setup_token parameter: %v", err), http.StatusBadRequest)
		return
	} else {
		setupToken = sanitizedToken
	}
	// Ensure validated pulseURL stays normalized before it reaches generated script state.
	if safeURL, err := sanitizeInstallerURL(pulseURL); err == nil {
		pulseURL = safeURL
	}

	log.Info().
		Str("type", serverType).
		Str("host", serverHost).
		Bool("has_auth", h.getConfig(r.Context()).AuthUser != "" || h.getConfig(r.Context()).AuthPass != "" || h.getConfig(r.Context()).HasAPITokens()).
		Msg("HandleSetupScript called")

	// The setup script is now public; authentication happens via setup token.
	// No need to check auth here since the script will prompt for a code

	serverName := deriveSetupScriptServerName(serverHost)

	pulseTokenScope := pulseTokenSuffix(pulseURL)
	tokenName := buildPulseMonitorTokenName(pulseURL)
	tokenMatchPrefix := tokenName

	// Log the token name for debugging
	log.Info().
		Str("pulseURL", pulseURL).
		Str("pulseTokenScope", pulseTokenScope).
		Str("tokenName", tokenName).
		Msg("Generated deterministic token name for setup script")

	artifact := buildSetupScriptInstallArtifact(
		pulseURL,
		serverType,
		serverHost,
		pulseURL,
		backupPerms,
		setupToken,
		0,
	)

	// Get or generate SSH public key for temperature monitoring
	sshKeys := h.getOrGenerateSSHKeys()

	storagePerms := ""
	if backupPerms {
		storagePerms = "\npveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin"
	}

	script := renderSetupScript(serverType, setupScriptRenderContext{
		ServerName:       serverName,
		PulseURL:         pulseURL,
		ServerHost:       serverHost,
		SetupToken:       setupToken,
		TokenName:        tokenName,
		TokenMatchPrefix: tokenMatchPrefix,
		StoragePerms:     storagePerms,
		SensorsPublicKey: sshKeys.SensorsPublicKey,
		Artifact:         artifact,
	})

	// Serve setup scripts as canonical shell-script downloads instead of generic text.
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", buildSetupScriptFileName(serverType)))
	w.Write([]byte(script))
}

type setupScriptURLRequest struct {
	Type        string `json:"type"`
	Host        string `json:"host"`
	BackupPerms bool   `json:"backupPerms"`
}

// generateSetupTokenRecord generates a secure hex token that satisfies sanitizeSetupAuthToken.
func (h *ConfigHandlers) generateSetupTokenRecord() string {
	// 16 bytes => 32 hex characters which matches the sanitizer's lower bound.
	const tokenBytes = 16
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	// rand.Read should never fail, but if it does fall back to timestamp-based token.
	log.Warn().Msg("fallback setup token generator used due to entropy failure")
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// HandleSetupScriptURL generates a one-time setup token and URL for the setup script.
func (h *ConfigHandlers) handleSetupScriptURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	// Parse request
	var req setupScriptURLRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if decoder.More() {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Host = strings.TrimSpace(req.Host)
	if !isCanonicalAutoRegisterType(req.Type) {
		http.Error(w, "type must be 'pve' or 'pbs'", http.StatusBadRequest)
		return
	}
	if req.BackupPerms && req.Type != "pve" {
		http.Error(w, "backupPerms is only supported for type 'pve'", http.StatusBadRequest)
		return
	}
	normalizedHost, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Host = normalizedHost

	// Generate a temporary setup token for setup-script bootstrap transport.
	token := h.generateSetupTokenRecord()
	tokenHash := internalauth.HashAPIToken(token)

	// Store the token with expiry (5 minutes)
	expiry := time.Now().Add(5 * time.Minute)
	h.codeMutex.Lock()
	h.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: expiry,
		Used:      false,
		NodeType:  req.Type,
		Host:      req.Host,
		OrgID:     GetOrgID(r.Context()),
	}
	h.codeMutex.Unlock()

	log.Info().
		Str("token_hash", safePrefixForLog(tokenHash, 8)+"...").
		Time("expiry", expiry).
		Str("type", req.Type).
		Msg("Generated temporary setup token")

	// Build the canonical bootstrap artifact for setup-script transport.
	pulseURL := resolveLoopbackAwarePublicBaseURL(r, h.getConfig(r.Context()))
	artifact := buildSetupScriptInstallArtifact(
		pulseURL,
		req.Type,
		req.Host,
		pulseURL,
		req.BackupPerms,
		token,
		expiry.Unix(),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(artifact)
}

// AutoRegisterRequest represents a request from the setup script or agent to auto-register a node
type AutoRegisterRequest struct {
	Type       string `json:"type"`                 // "pve" or "pbs"
	Host       string `json:"host"`                 // The host URL
	TokenID    string `json:"tokenId"`              // Full token ID like pulse-monitor@pve!pulse-token
	TokenValue string `json:"tokenValue,omitempty"` // The token value for the node
	ServerName string `json:"serverName,omitempty"` // Hostname or IP
	AuthToken  string `json:"authToken,omitempty"`  // One-time setup token from setup/install flows
	Source     string `json:"source,omitempty"`     // "agent" or "script" - indicates how the node was registered
}

// AutoRegisterResponse is the canonical success shape for /api/auto-register.
type AutoRegisterResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	Action     string `json:"action"`
	Type       string `json:"type"`
	Source     string `json:"source"`
	Host       string `json:"host"`
	NodeID     string `json:"nodeId"`
	NodeName   string `json:"nodeName"`
	TokenID    string `json:"tokenId"`
	TokenValue string `json:"tokenValue,omitempty"`
}

func isCanonicalAutoRegisterType(nodeType string) bool {
	switch strings.TrimSpace(nodeType) {
	case "pve", "pbs":
		return true
	default:
		return false
	}
}

func isCanonicalAutoRegisterTokenID(nodeType string, tokenID string) bool {
	trimmedType := strings.TrimSpace(nodeType)
	trimmedTokenID := strings.TrimSpace(tokenID)
	if !isCanonicalAutoRegisterType(trimmedType) || trimmedTokenID == "" {
		return false
	}
	prefix := "pulse-monitor@" + trimmedType + "!"
	if !strings.HasPrefix(trimmedTokenID, prefix) {
		return false
	}
	suffix := strings.TrimSpace(strings.TrimPrefix(trimmedTokenID, prefix))
	return strings.HasPrefix(suffix, "pulse-") && suffix != "pulse-"
}

func isCanonicalAutoRegisterSource(source string) bool {
	switch strings.TrimSpace(source) {
	case "agent", "script":
		return true
	default:
		return false
	}
}

func canonicalAutoRegisterMatchMessage(reason string) string {
	return "Canonical auto-register matched existing node by " + reason
}

func canonicalAutoRegisterCompletionPayloadMessage() string {
	return "Incomplete canonical auto-register token completion payload"
}

func canonicalAutoRegisterMissingFieldsMessage(typeValue string, host string, hasTokenID bool, serverName string) string {
	missing := make([]string, 0, 4)
	if strings.TrimSpace(typeValue) == "" {
		missing = append(missing, "type")
	}
	if strings.TrimSpace(host) == "" {
		missing = append(missing, "host")
	}
	if !hasTokenID {
		missing = append(missing, "tokenId/tokenValue")
	}
	if strings.TrimSpace(serverName) == "" {
		missing = append(missing, "serverName")
	}
	if len(missing) == 0 {
		return "Missing required canonical auto-register fields"
	}
	return "Missing required canonical auto-register fields: " + strings.Join(missing, ", ")
}

func canonicalAutoRegisterNodeIdentity(req *AutoRegisterRequest, actualName string, host string) string {
	eventName := strings.TrimSpace(actualName)
	if eventName == "" {
		eventName = strings.TrimSpace(req.ServerName)
	}
	if eventName == "" {
		eventName = strings.TrimSpace(host)
	}
	return eventName
}

func canonicalAutoRegisterSuccessMessage(nodeName string, host string) string {
	trimmedNodeName := strings.TrimSpace(nodeName)
	trimmedHost := strings.TrimSpace(host)
	if trimmedNodeName == "" {
		return fmt.Sprintf("Node registered successfully at %s", trimmedHost)
	}
	if trimmedHost == "" {
		return fmt.Sprintf("Node %s registered successfully", trimmedNodeName)
	}
	return fmt.Sprintf("Node %s registered successfully at %s", trimmedNodeName, trimmedHost)
}

func buildCanonicalAutoRegisterResponse(req *AutoRegisterRequest, host string, actualName string, tokenID string, tokenValue string) AutoRegisterResponse {
	nodeName := canonicalAutoRegisterNodeIdentity(req, actualName, host)
	return AutoRegisterResponse{
		Status:     "success",
		Message:    canonicalAutoRegisterSuccessMessage(nodeName, host),
		Action:     "use_token",
		Type:       req.Type,
		Source:     req.Source,
		Host:       host,
		NodeID:     nodeName,
		NodeName:   nodeName,
		TokenID:    tokenID,
		TokenValue: strings.TrimSpace(tokenValue),
	}
}

func buildAutoRegisterEventData(req *AutoRegisterRequest, host string, actualName string, tokenID string) map[string]interface{} {
	eventName := canonicalAutoRegisterNodeIdentity(req, actualName, host)

	return map[string]interface{}{
		"type":      req.Type,
		"host":      host,
		"name":      eventName,
		"nodeId":    eventName,
		"nodeName":  eventName,
		"tokenId":   tokenID,
		"hasToken":  true,
		"verifySSL": true,
		"status":    "connected",
	}
}

func (h *ConfigHandlers) notifyAutoRegistrationSuccess(ctx context.Context, req *AutoRegisterRequest, host string, actualName string, tokenID string) {
	if h.getMonitor(ctx) != nil && h.getMonitor(ctx).GetDiscoveryService() != nil {
		log.Info().Msg("Triggering discovery refresh after auto-registration")
		h.getMonitor(ctx).GetDiscoveryService().ForceRefresh()
	}

	if h.wsHub == nil {
		log.Warn().Msg("WebSocket hub is nil, cannot broadcast auto-registration")
		return
	}

	nodeInfo := buildAutoRegisterEventData(req, host, actualName, tokenID)
	h.wsHub.BroadcastMessage(websocket.Message{
		Type:      "node_auto_registered",
		Data:      nodeInfo,
		Timestamp: time.Now().Format(time.RFC3339),
	})

	if h.getMonitor(ctx) != nil && h.getMonitor(ctx).GetDiscoveryService() != nil {
		result, _ := h.getMonitor(ctx).GetDiscoveryService().GetCachedResult()
		if result != nil {
			h.wsHub.BroadcastMessage(websocket.Message{
				Type: "discovery_update",
				Data: map[string]interface{}{
					"servers":           result.Servers,
					"errors":            result.LegacyErrors(),
					"structured_errors": result.StructuredErrors,
					"timestamp":         time.Now().Unix(),
				},
				Timestamp: time.Now().Format(time.RFC3339),
			})
			log.Info().Msg("Broadcasted discovery update after auto-registration")
		}
	}

	log.Info().
		Str("host", host).
		Str("name", actualName).
		Str("type", "node_auto_registered").
		Msg("Broadcasted auto-registration success via WebSocket")
}

// HandleAutoRegister receives token details from the setup script and auto-configures the node
func (h *ConfigHandlers) handleAutoRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body first so the setup token can be validated early.
	var req AutoRegisterRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse auto-register request")
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Check authentication through the one-time setup token carried in authToken.
	authenticated := false
	setupToken := strings.TrimSpace(req.AuthToken)

	log.Debug().
		Bool("hasAuthToken", setupToken != "").
		Bool("hasConfigToken", h.getConfig(r.Context()).HasAPITokens()).
		Msg("Checking authentication for auto-register")

	// First check for the one-time setup token in the request.
	if setupToken != "" {
		tokenHash := internalauth.HashAPIToken(setupToken)
		log.Debug().
			Bool("hasSetupToken", true).
			Str("tokenHash", safePrefixForLog(tokenHash, 8)+"...").
			Msg("Checking auth token as one-time setup token")
		h.codeMutex.Lock()
		setupTokenRecord, exists := h.setupTokens[tokenHash]
		log.Debug().
			Bool("exists", exists).
			Int("totalTokens", len(h.setupTokens)).
			Msg("Setup token lookup result")
		if exists && !setupTokenRecord.Used && time.Now().Before(setupTokenRecord.ExpiresAt) {
			// Validate that the token matches the node type.
			// Note: We don't validate the host anymore as it may differ between
			// what's entered in the UI and what's provided in the setup script URL
			if setupTokenRecord.NodeType == req.Type {
				setupTokenRecord.Used = true // Mark as used immediately.

				// Inject OrgID from the setup token into context for subsequent processing.
				if setupTokenRecord.OrgID != "" {
					ctx := context.WithValue(r.Context(), OrgIDContextKey, setupTokenRecord.OrgID)
					r = r.WithContext(ctx)
				}
				// Allow a short grace period for follow-up actions without keeping tokens alive too long.
				graceExpiry := time.Now().Add(1 * time.Minute)
				if setupTokenRecord.ExpiresAt.Before(graceExpiry) {
					graceExpiry = setupTokenRecord.ExpiresAt
				}
				h.recentSetupTokens[tokenHash] = graceExpiry
				authenticated = true
				log.Info().
					Str("type", req.Type).
					Str("host", req.Host).
					Bool("via_authToken", req.AuthToken != "").
					Msg("Auto-register authenticated via setup token")
			} else {
				log.Warn().
					Str("expected_type", setupTokenRecord.NodeType).
					Str("got_type", req.Type).
					Msg("Setup token validation failed - type mismatch")
			}
		} else if exists && setupTokenRecord.Used {
			log.Warn().Msg("Setup token already used")
		} else if exists {
			log.Warn().Msg("Setup token expired")
		} else {
			log.Warn().Msg("Invalid setup token - not in setup token registry")
		}
		h.codeMutex.Unlock()
	}

	// Abort when no authentication succeeded. This applies even when API tokens
	// are not configured to ensure one-time setup tokens are always required.
	if !authenticated {
		log.Warn().
			Str("ip", r.RemoteAddr).
			Bool("has_setup_token", setupToken != "").
			Msg("Unauthorized auto-register attempt rejected")

		if setupToken == "" {
			http.Error(w, "Pulse setup token required", http.StatusUnauthorized)
		} else {
			http.Error(w, "Invalid or expired setup token", http.StatusUnauthorized)
		}
		return
	}

	// Log source IP for security auditing
	clientIP := r.RemoteAddr
	// Only trust X-Forwarded-For if request comes from a trusted proxy
	peerIP := extractRemoteIP(clientIP)
	if isTrustedProxyIP(peerIP) {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}
	}
	log.Info().Str("clientIP", clientIP).Msg("Auto-register request from")

	log.Info().
		Str("type", req.Type).
		Str("host", req.Host).
		Str("tokenId", req.TokenID).
		Bool("hasTokenValue", req.TokenValue != "").
		Str("serverName", req.ServerName).
		Msg("Processing auto-register request")

	h.handleCanonicalAutoRegister(w, r, &req, clientIP)
}

// handleCanonicalAutoRegister handles the canonical /api/auto-register flow.
func (h *ConfigHandlers) handleCanonicalAutoRegister(w http.ResponseWriter, r *http.Request, req *AutoRegisterRequest, clientIP string) {
	log.Info().
		Str("type", req.Type).
		Str("host", req.Host).
		Msg("Processing canonical auto-register request")

	typeValue := strings.TrimSpace(req.Type)
	hostValue := strings.TrimSpace(req.Host)
	tokenID := strings.TrimSpace(req.TokenID)
	tokenValue := strings.TrimSpace(req.TokenValue)
	serverName := strings.TrimSpace(req.ServerName)
	registrationSource := strings.TrimSpace(req.Source)
	hasTokenID := tokenID != ""
	hasTokenValue := tokenValue != ""

	if hasTokenID != hasTokenValue {
		log.Error().
			Bool("hasTokenID", hasTokenID).
			Bool("hasTokenValue", hasTokenValue).
			Msg(canonicalAutoRegisterCompletionPayloadMessage())
		http.Error(w, "tokenId and tokenValue must be provided together", http.StatusBadRequest)
		return
	}
	if typeValue == "" || hostValue == "" || !hasTokenID || serverName == "" {
		missingMessage := canonicalAutoRegisterMissingFieldsMessage(typeValue, hostValue, hasTokenID, serverName)
		log.Error().
			Str("type", req.Type).
			Str("host", req.Host).
			Str("serverName", req.ServerName).
			Bool("hasTokenID", hasTokenID).
			Bool("hasTokenValue", hasTokenValue).
			Msg(missingMessage)
		http.Error(w, missingMessage, http.StatusBadRequest)
		return
	}
	if registrationSource == "" {
		http.Error(w, "source is required", http.StatusBadRequest)
		return
	}
	if !isCanonicalAutoRegisterSource(registrationSource) {
		http.Error(w, "source must be 'agent' or 'script'", http.StatusBadRequest)
		return
	}
	if !isCanonicalAutoRegisterType(typeValue) {
		http.Error(w, "type must be 'pve' or 'pbs'", http.StatusBadRequest)
		return
	}
	if !isCanonicalAutoRegisterTokenID(typeValue, tokenID) {
		http.Error(w, "tokenId must be a canonical Pulse-managed token id", http.StatusBadRequest)
		return
	}

	req.Type = typeValue
	req.Host = hostValue
	req.TokenID = tokenID
	req.TokenValue = tokenValue
	req.ServerName = serverName
	req.Source = registrationSource

	host, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fingerprint := ""
	if fp, err := fetchTLSFingerprint(host); err != nil {
		log.Warn().Err(err).Str("host", host).Msg("Failed to fetch TLS fingerprint for auto-register")
	} else {
		fingerprint = fp
	}
	verifySSL := true

	fullTokenID := req.TokenID
	tokenValue = req.TokenValue
	log.Info().
		Str("host", host).
		Str("tokenID", fullTokenID).
		Str("source", registrationSource).
		Msg("Using caller-supplied token for canonical /api/auto-register completion")
	// Add the node to configuration
	if req.Type == "pve" {
		pveDisplayName := h.disambiguateNodeName(r.Context(), serverName, host, "pve")
		pveNode := config.PVEInstance{
			Name:              pveDisplayName,
			Host:              host,
			TokenName:         fullTokenID,
			TokenValue:        tokenValue,
			Fingerprint:       fingerprint,
			VerifySSL:         verifySSL,
			MonitorVMs:        true,
			MonitorContainers: true,
			MonitorStorage:    true,
			MonitorBackups:    true,
			Source:            registrationSource,
		}
		// Deduplicate by host to keep canonical auto-registration idempotent on reruns.
		existingIndex := -1
		preserveHost := false
		for i, node := range h.getConfig(r.Context()).PVEInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
			if hostsShareResolvedIdentity(node.Host, host) {
				existingIndex = i
				preserveHost = true
				log.Info().
					Str("existingHost", node.Host).
					Str("newHost", host).
					Str("type", "pve").
					Msg(canonicalAutoRegisterMatchMessage("resolved host identity"))
				break
			}
			if serverName != "" &&
				strings.EqualFold(strings.TrimSpace(node.Name), serverName) &&
				node.TokenName == pveNode.TokenName {
				existingIndex = i
				log.Info().
					Str("existingHost", node.Host).
					Str("newHost", host).
					Str("type", "pve").
					Str("node", serverName).
					Msg(canonicalAutoRegisterMatchMessage("DHCP continuity token identity"))
				break
			}
		}
		if existingIndex >= 0 {
			instance := &h.getConfig(r.Context()).PVEInstances[existingIndex]
			if !preserveHost {
				instance.Host = host
			}
			instance.User = ""
			instance.Password = ""
			instance.TokenName = pveNode.TokenName
			instance.TokenValue = pveNode.TokenValue
			if pveNode.Source != "" {
				instance.Source = pveNode.Source
			}
			// Update TLS fingerprint only when one was captured; a failed
			// FetchFingerprint must not erase a previously valid pin.
			if pveNode.Fingerprint != "" {
				instance.Fingerprint = pveNode.Fingerprint
			}
			instance.VerifySSL = pveNode.VerifySSL
			log.Info().Str("host", host).Str("type", "pve").Msg(canonicalAutoRegisterMatchMessage("host; updated token in-place"))
		} else {
			if enforceMonitoredSystemLimitForConfigRegistration(w, r.Context(), h.getConfig(r.Context()), h.getMonitor(r.Context()), unifiedresources.MonitoredSystemCandidate{
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     serverName,
				Hostname: serverName,
				HostURL:  host,
			}) {
				return
			}
			h.getConfig(r.Context()).PVEInstances = append(h.getConfig(r.Context()).PVEInstances, pveNode)
			h.normalizePVEConfigState(r.Context())
		}
	} else if req.Type == "pbs" {
		pbsDisplayName := h.disambiguateNodeName(r.Context(), serverName, host, "pbs")
		pbsNode := config.PBSInstance{
			Name:              pbsDisplayName,
			Host:              host,
			TokenName:         fullTokenID,
			TokenValue:        tokenValue,
			Fingerprint:       fingerprint,
			VerifySSL:         verifySSL,
			MonitorBackups:    true,
			MonitorDatastores: true,
			MonitorSyncJobs:   true,
			MonitorVerifyJobs: true,
			MonitorPruneJobs:  true,
		}
		// Deduplicate by host to keep canonical auto-registration idempotent on reruns.
		existingIndex := -1
		preserveHost := false
		for i, node := range h.getConfig(r.Context()).PBSInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
			if hostsShareResolvedIdentity(node.Host, host) {
				existingIndex = i
				preserveHost = true
				log.Info().
					Str("existingHost", node.Host).
					Str("newHost", host).
					Str("type", "pbs").
					Msg(canonicalAutoRegisterMatchMessage("resolved host identity"))
				break
			}
			if serverName != "" &&
				strings.EqualFold(strings.TrimSpace(node.Name), serverName) &&
				node.TokenName == pbsNode.TokenName {
				existingIndex = i
				log.Info().
					Str("existingHost", node.Host).
					Str("newHost", host).
					Str("type", "pbs").
					Str("node", serverName).
					Msg(canonicalAutoRegisterMatchMessage("DHCP continuity token identity"))
				break
			}
		}
		if existingIndex >= 0 {
			instance := &h.getConfig(r.Context()).PBSInstances[existingIndex]
			if !preserveHost {
				instance.Host = host
			}
			instance.User = ""
			instance.Password = ""
			instance.TokenName = pbsNode.TokenName
			instance.TokenValue = pbsNode.TokenValue
			// Update TLS fingerprint only when one was captured; a failed
			// FetchFingerprint must not erase a previously valid pin.
			if pbsNode.Fingerprint != "" {
				instance.Fingerprint = pbsNode.Fingerprint
			}
			instance.VerifySSL = pbsNode.VerifySSL
			log.Info().Str("host", host).Str("type", "pbs").Msg(canonicalAutoRegisterMatchMessage("host; updated token in-place"))
		} else {
			if enforceMonitoredSystemLimitForConfigRegistration(w, r.Context(), h.getConfig(r.Context()), h.getMonitor(r.Context()), unifiedresources.MonitoredSystemCandidate{
				Type:     unifiedresources.ResourceTypePBS,
				Name:     serverName,
				Hostname: serverName,
				HostURL:  host,
			}) {
				return
			}
			h.getConfig(r.Context()).PBSInstances = append(h.getConfig(r.Context()).PBSInstances, pbsNode)
		}
	}

	// Save configuration
	h.normalizePVEConfigState(r.Context())
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	actualName := h.findInstanceNameByHost(r.Context(), req.Type, host)
	if actualName == "" {
		actualName = serverName
	}
	h.markAutoRegistered(req.Type, actualName)

	// Reload monitor
	if h.reloadFunc != nil {
		go func() {
			if err := h.reloadFunc(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor after auto-registration")
			}
		}()
	}

	h.notifyAutoRegistrationSuccess(r.Context(), req, host, actualName, fullTokenID)

	response := buildCanonicalAutoRegisterResponse(req, host, actualName, fullTokenID, tokenValue)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

}

// SSHKeyPair holds the sensors SSH public key for temperature monitoring.
type SSHKeyPair struct {
	SensorsPublicKey string
}

// getOrGenerateSSHKeys returns the SSH public key for temperature monitoring
// If keys don't exist, they are generated automatically
// SECURITY: Blocks key generation when running in containers unless dev mode override is enabled
func (h *ConfigHandlers) getOrGenerateSSHKeys() SSHKeyPair {
	// CRITICAL SECURITY CHECK: Never generate SSH keys in containers (unless dev mode)
	// Container compromise = SSH key compromise = root access to Proxmox
	devModeAllowSSH := os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true"
	isContainer := os.Getenv("PULSE_DOCKER") == "true" || system.InContainer()

	if isContainer && !devModeAllowSSH {
		log.Error().Msg("SECURITY BLOCK: SSH key generation disabled in containerized deployments")
		log.Error().Msg("Temperature monitoring via SSH is disabled in containerized deployments")
		log.Error().Msg("See: https://github.com/rcourtman/Pulse/blob/main/SECURITY.md#critical-security-notice-for-container-deployments")
		log.Error().Msg("To test SSH keys in dev/lab only: PULSE_DEV_ALLOW_CONTAINER_SSH=true (NEVER in production!)")
		return SSHKeyPair{}
	}

	if devModeAllowSSH && isContainer {
		log.Warn().Msg("⚠️  DEV MODE: SSH key generation ENABLED in container - FOR TESTING ONLY")
		log.Warn().Msg("⚠️  This grants root SSH access from container - NEVER use in production!")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Warn().Err(err).Msg("Could not determine home directory for SSH keys")
		return SSHKeyPair{}
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Generate/load sensors key (for temperature collection)
	sensorsPrivPath := filepath.Join(sshDir, "id_ed25519_sensors")
	sensorsPubPath := filepath.Join(sshDir, "id_ed25519_sensors.pub")
	sensorsKey := h.generateOrLoadSSHKey(sshDir, sensorsPrivPath, sensorsPubPath, "sensors")

	return SSHKeyPair{
		SensorsPublicKey: sensorsKey,
	}
}

// generateOrLoadSSHKey generates or loads a single SSH keypair
func (h *ConfigHandlers) generateOrLoadSSHKey(sshDir, privateKeyPath, publicKeyPath, keyType string) string {
	// Check if public key already exists
	if pubKeyBytes, err := os.ReadFile(publicKeyPath); err == nil {
		publicKey := strings.TrimSpace(string(pubKeyBytes))
		log.Info().Str("keyPath", publicKeyPath).Str("type", keyType).Msg("Using existing SSH public key")
		return publicKey
	}

	// Key doesn't exist - generate one
	log.Info().Str("sshDir", sshDir).Str("type", keyType).Msg("Generating new SSH keypair for temperature monitoring")

	// Create .ssh directory if it doesn't exist
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		log.Error().Err(err).Str("sshDir", sshDir).Msg("Failed to create .ssh directory")
		return ""
	}

	// Generate Ed25519 key pair (more secure and faster than RSA)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate Ed25519 key")
		return ""
	}

	// Save private key in OpenSSH format
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Error().Err(err).Str("path", privateKeyPath).Msg("Failed to create private key file")
		return ""
	}
	defer privateKeyFile.Close()

	// Marshal Ed25519 private key to OpenSSH format
	privKeyBytes, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal private key")
		return ""
	}
	if err := pem.Encode(privateKeyFile, privKeyBytes); err != nil {
		log.Error().Err(err).Msg("Failed to write private key")
		return ""
	}

	// Generate public key in OpenSSH format
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate public key")
		return ""
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	publicKeyString := strings.TrimSpace(string(publicKeyBytes))

	// Save public key
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0644); err != nil {
		log.Error().Err(err).Str("path", publicKeyPath).Msg("Failed to write public key")
		return ""
	}

	log.Info().
		Str("privateKey", privateKeyPath).
		Str("publicKey", publicKeyPath).
		Msg("Successfully generated SSH keypair")

	return publicKeyString
}

// AgentInstallCommandRequest represents a request for an agent install command
type AgentInstallCommandRequest struct {
	Type string `json:"type"` // "pve" or "pbs"
}

// AgentInstallCommandResponse contains the generated install command
type AgentInstallCommandResponse struct {
	Command string `json:"command"`
	Token   string `json:"token"`
}

// HandleAgentInstallCommand generates an API token and install command for agent-based Proxmox setup
func (h *ConfigHandlers) handleAgentInstallCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentInstallCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	installType, err := normalizeProxmoxInstallType(req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := h.getConfig(r.Context())
	persistence := h.getPersistence(r.Context())

	rawToken := ""
	if authConfiguredForAgentLifecycle(cfg) {
		tokenName := fmt.Sprintf("proxmox-agent-%s-%d", installType, time.Now().Unix())
		rawToken, _, err = issueAndPersistAgentInstallToken(cfg, persistence, issueAgentInstallTokenOptions{
			TokenName: tokenName,
			Metadata: map[string]string{
				"install_type": installType,
				"issued_via":   "config_agent_install_command",
			},
		})
		if err != nil {
			switch {
			case errors.Is(err, errAgentInstallTokenGeneration):
				log.Error().Err(err).Msg("Failed to generate API token for agent install")
				http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
			case errors.Is(err, errAgentInstallTokenRecord):
				log.Error().Err(err).Str("token_name", tokenName).Msg("Failed to construct API token record")
				http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			case errors.Is(err, errAgentInstallTokenPersist):
				log.Error().Err(err).Msg("Failed to persist API tokens after creation")
				http.Error(w, "Failed to save token", http.StatusInternalServerError)
			default:
				log.Error().Err(err).Msg("Failed to create API token for agent install")
				http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
			}
			return
		}
	}

	baseURL := resolveConfigAgentInstallBaseURL(r, cfg)
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            baseURL,
		Token:              rawToken,
		InstallType:        installType,
		IncludeInstallType: true,
	})

	log.Info().
		Str("type", installType).
		Bool("token_issued", rawToken != "").
		Msg("Generated agent install command")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentInstallCommandResponse{
		Command: command,
		Token:   rawToken,
	})
}
