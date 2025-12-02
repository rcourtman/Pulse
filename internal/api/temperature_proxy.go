package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// TemperatureProxyHandlers manages temperature proxy registration
type TemperatureProxyHandlers struct {
	config      *config.Config
	persistence *config.ConfigPersistence
	reloadFunc  func() error
	syncMu      sync.RWMutex
	syncStatus  map[string]proxySyncState
}

type proxySyncState struct {
	Instance        string
	LastPull        time.Time
	RefreshInterval int
}

const defaultProxyAllowlistRefreshSeconds = 60

type authorizedNode struct {
	Name string `json:"name,omitempty"`
	IP   string `json:"ip,omitempty"`
}

// NewTemperatureProxyHandlers constructs a new handler set for temperature proxy
func NewTemperatureProxyHandlers(cfg *config.Config, persistence *config.ConfigPersistence, reloadFunc func() error) *TemperatureProxyHandlers {
	return &TemperatureProxyHandlers{
		config:      cfg,
		persistence: persistence,
		reloadFunc:  reloadFunc,
		syncStatus:  make(map[string]proxySyncState),
	}
}

// SetConfig updates the configuration reference used by the handler.
func (h *TemperatureProxyHandlers) SetConfig(cfg *config.Config) {
	if h != nil {
		h.config = cfg
	}
}

func (h *TemperatureProxyHandlers) recordSync(instance string, refreshSeconds int) {
	if h == nil {
		return
	}

	instance = strings.TrimSpace(instance)
	if instance == "" {
		return
	}

	if refreshSeconds <= 0 {
		refreshSeconds = defaultProxyAllowlistRefreshSeconds
	}

	h.syncMu.Lock()
	defer h.syncMu.Unlock()

	key := strings.ToLower(instance)
	h.syncStatus[key] = proxySyncState{
		Instance:        instance,
		LastPull:        time.Now(),
		RefreshInterval: refreshSeconds,
	}
}

func (h *TemperatureProxyHandlers) SnapshotSyncStatus() map[string]proxySyncState {
	if h == nil {
		return nil
	}

	h.syncMu.RLock()
	defer h.syncMu.RUnlock()

	if len(h.syncStatus) == 0 {
		return nil
	}

	copy := make(map[string]proxySyncState, len(h.syncStatus))
	for key, state := range h.syncStatus {
		copy[key] = state
	}
	return copy
}

func extractHostPart(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}

	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		if parsed, err := url.Parse(host); err == nil {
			return parsed.Hostname()
		}
	}

	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return strings.TrimSpace(host)
}

func buildAuthorizedNodeList(instances []config.PVEInstance) []authorizedNode {
	nodes := make([]authorizedNode, 0)
	seen := make(map[string]struct{})
	add := func(name, ip string) {
		name = strings.TrimSpace(name)
		ip = strings.TrimSpace(ip)
		if name == "" && ip == "" {
			return
		}
		key := name + "|" + ip
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		nodes = append(nodes, authorizedNode{Name: name, IP: ip})
	}

	for i := range instances {
		instance := &instances[i]
		add(instance.Name, extractHostPart(instance.Host))
		add(instance.Name, extractHostPart(instance.GuestURL))

		if instance.ClusterEndpoints != nil {
			for _, ep := range instance.ClusterEndpoints {
				name := ep.NodeName
				ip := ep.IP
				if ip == "" {
					ip = extractHostPart(ep.Host)
				}
				if name == "" {
					name = ep.Host
				}
				add(name, ip)
			}
		}
	}

	return nodes
}

// HandleRegister handles temperature proxy registration from the installer
//
// POST /api/temperature-proxy/register
// Body: {"hostname": "pve1", "proxy_url": "https://pve1.lan:8443"}
// Response: {"success": true, "token": "...", "pve_instance": "pve1"}
func (h *TemperatureProxyHandlers) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	defer r.Body.Close()

	var req struct {
		Hostname string `json:"hostname"`
		ProxyURL string `json:"proxy_url"`
		Mode     string `json:"mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	// Validate inputs
	hostname := strings.TrimSpace(req.Hostname)
	proxyURL := strings.TrimSpace(req.ProxyURL)

	if hostname == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_hostname", "Hostname is required", nil)
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		if proxyURL != "" {
			mode = "http"
		} else {
			mode = "socket"
		}
	}

	isHTTPMode := mode == "http"
	if isHTTPMode {
		if proxyURL == "" {
			writeErrorResponse(w, http.StatusBadRequest, "missing_proxy_url", "Proxy URL is required for HTTP mode", nil)
			return
		}
		if !strings.HasPrefix(strings.ToLower(proxyURL), "https://") {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_proxy_url", "Proxy URL must use HTTPS", nil)
			return
		}
	} else {
		proxyURL = ""
	}

	// Load current config
	nodesConfig, err := h.persistence.LoadNodesConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "config_load_failed", "Failed to load configuration", map[string]string{"error": err.Error()})
		return
	}

	// Find matching PVE instance by hostname
	var matchedInstance *config.PVEInstance
	var matchedIndex int
	var matchedEndpointIndex = -1 // For cluster nodes, track which endpoint matched
	hostnameLower := strings.ToLower(hostname)

	for i := range nodesConfig.PVEInstances {
		instance := &nodesConfig.PVEInstances[i]

		// Try to match by instance name
		if strings.EqualFold(instance.Name, hostname) {
			matchedInstance = instance
			matchedIndex = i
			break
		}

		// Try to match by hostname in the Host field or cluster endpoints
		if strings.Contains(strings.ToLower(instance.Host), hostnameLower) {
			matchedInstance = instance
			matchedIndex = i
			break
		}

		// Check cluster endpoints
		if instance.IsCluster {
			for j, ep := range instance.ClusterEndpoints {
				if strings.EqualFold(ep.NodeName, hostname) || strings.Contains(strings.ToLower(ep.Host), hostnameLower) {
					matchedInstance = instance
					matchedIndex = i
					matchedEndpointIndex = j
					break
				}
			}
		}
	}

	if matchedInstance == nil {
		writeErrorResponse(w, http.StatusNotFound, "pve_instance_not_found", fmt.Sprintf("No PVE instance configured for hostname '%s'. Add the instance in Pulse first.", hostname), nil)
		return
	}

	// Generate tokens
	authToken := ""
	if isHTTPMode {
		authToken, err = generateSecureToken(32)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate authentication token", map[string]string{"error": err.Error()})
			return
		}
	}

	ctrlToken, err := generateSecureToken(32)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate authentication token", map[string]string{"error": err.Error()})
		return
	}

	// Update the instance with proxy configuration
	// For clusters, store per-node tokens on the ClusterEndpoint; instance-level token is for non-cluster or fallback
	nodesConfig.PVEInstances[matchedIndex].TemperatureProxyURL = proxyURL
	if isHTTPMode {
		nodesConfig.PVEInstances[matchedIndex].TemperatureProxyToken = authToken
	}

	// For cluster nodes, store token on the specific endpoint so each node has its own token
	if matchedEndpointIndex >= 0 && matchedInstance.IsCluster {
		nodesConfig.PVEInstances[matchedIndex].ClusterEndpoints[matchedEndpointIndex].TemperatureProxyControlToken = ctrlToken
		log.Debug().
			Str("hostname", hostname).
			Int("endpoint_index", matchedEndpointIndex).
			Msg("Storing control token on cluster endpoint")
	} else {
		// Non-cluster instance or matched by instance name directly
		nodesConfig.PVEInstances[matchedIndex].TemperatureProxyControlToken = ctrlToken
	}

	// Save updated configuration
	log.Debug().
		Int("matchedIndex", matchedIndex).
		Int("matchedEndpointIndex", matchedEndpointIndex).
		Str("saving_url", nodesConfig.PVEInstances[matchedIndex].TemperatureProxyURL).
		Bool("saving_has_token", nodesConfig.PVEInstances[matchedIndex].TemperatureProxyToken != "").
		Str("instance_name", nodesConfig.PVEInstances[matchedIndex].Name).
		Msg("About to save nodes config with proxy registration")
	if err := h.persistence.SaveNodesConfig(nodesConfig.PVEInstances, nodesConfig.PBSInstances, nodesConfig.PMGInstances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "config_save_failed", "Failed to save configuration", map[string]string{"error": err.Error()})
		return
	}

	// Reload the entire config to ensure all components (router, monitor, handlers) get the fresh config
	// This prevents the monitor from later overwriting nodes.enc with stale data
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload config after temperature proxy registration")
			// Don't fail the request - the save succeeded, reload is best-effort
		} else {
			log.Info().Str("instance", matchedInstance.Name).Msg("Config reloaded after temperature proxy registration")
		}
	}

	log.Info().
		Str("hostname", hostname).
		Str("proxy_url", proxyURL).
		Str("mode", mode).
		Str("pve_instance", matchedInstance.Name).
		Msg("Temperature proxy registered successfully")

	allowed := buildAuthorizedNodeList(nodesConfig.PVEInstances)
	resp := map[string]any{
		"success":          true,
		"token":            authToken,
		"control_token":    ctrlToken,
		"pve_instance":     matchedInstance.Name,
		"allowed_nodes":    allowed,
		"refresh_interval": defaultProxyAllowlistRefreshSeconds,
		"message":          fmt.Sprintf("Temperature proxy registered for instance '%s'", matchedInstance.Name),
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize temperature proxy registration response")
	}
}

// HandleAuthorizedNodes returns the list of nodes Pulse has authorized for a proxy.
//
// GET /api/temperature-proxy/authorized-nodes
// Headers:
//
//	X-Proxy-Token: <control-plane token>
func (h *TemperatureProxyHandlers) HandleAuthorizedNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	token := strings.TrimSpace(r.Header.Get("X-Proxy-Token"))
	if token == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token = strings.TrimSpace(authHeader[7:])
		}
	}

	if token == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "missing_token", "X-Proxy-Token header required", nil)
		return
	}

	nodesConfig, err := h.persistence.LoadNodesConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "config_load_failed", "Failed to load configuration", map[string]string{"error": err.Error()})
		return
	}

	var matched *config.PVEInstance
	for i := range nodesConfig.PVEInstances {
		inst := &nodesConfig.PVEInstances[i]

		// Check instance-level token first
		switch {
		case strings.TrimSpace(inst.TemperatureProxyControlToken) == token:
			matched = inst
		case inst.TemperatureProxyControlToken == "" && strings.TrimSpace(inst.TemperatureProxyToken) == token:
			// Legacy HTTP-mode proxies reuse TemperatureProxyToken
			matched = inst
		}
		if matched != nil {
			break
		}

		// For clusters, also check per-node tokens on ClusterEndpoints
		if inst.IsCluster {
			for j := range inst.ClusterEndpoints {
				ep := &inst.ClusterEndpoints[j]
				if strings.TrimSpace(ep.TemperatureProxyControlToken) == token {
					matched = inst
					break
				}
			}
			if matched != nil {
				break
			}
		}
	}

	refreshFromProxy := 0
	if hdr := strings.TrimSpace(r.Header.Get("X-Proxy-Refresh")); hdr != "" {
		if val, err := strconv.Atoi(hdr); err == nil && val > 0 {
			refreshFromProxy = val
		}
	}

	if matched == nil {
		writeErrorResponse(w, http.StatusUnauthorized, "invalid_token", "Proxy token not recognized", nil)
		return
	}

	refreshInterval := defaultProxyAllowlistRefreshSeconds
	if refreshFromProxy > 0 {
		refreshInterval = refreshFromProxy
	}

	nodes := buildAuthorizedNodeList(nodesConfig.PVEInstances)
	if len(nodes) == 0 && matched != nil {
		nodes = append(nodes, authorizedNode{Name: matched.Name, IP: extractHostPart(matched.Host)})
	}

	hashMaterial := make([]string, 0, len(nodes))
	for _, node := range nodes {
		hashMaterial = append(hashMaterial, fmt.Sprintf("%s|%s", node.Name, node.IP))
	}
	sort.Strings(hashMaterial)
	hashBytes := sha256.Sum256([]byte(strings.Join(hashMaterial, "\n")))

	resp := map[string]interface{}{
		"instance":         matched.Name,
		"nodes":            nodes,
		"hash":             hex.EncodeToString(hashBytes[:]),
		"refresh_interval": refreshInterval,
		"generated_at":     time.Now().UTC(),
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to write authorized-nodes response")
	} else {
		h.recordSync(matched.Name, refreshInterval)
	}
}

// HandleUnregister removes temperature proxy configuration from a PVE instance
//
// DELETE /api/temperature-proxy/unregister?hostname=pve1
func (h *TemperatureProxyHandlers) HandleUnregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only DELETE is allowed", nil)
		return
	}

	hostname := strings.TrimSpace(r.URL.Query().Get("hostname"))
	if hostname == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_hostname", "Hostname query parameter is required", nil)
		return
	}

	// Load current config
	nodesConfig, err := h.persistence.LoadNodesConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "config_load_failed", "Failed to load configuration", map[string]string{"error": err.Error()})
		return
	}

	// Find matching PVE instance
	var matchedIndex = -1
	var matchedName string
	hostnameLower := strings.ToLower(hostname)

	for i := range nodesConfig.PVEInstances {
		instance := &nodesConfig.PVEInstances[i]

		if strings.Contains(strings.ToLower(instance.Host), hostnameLower) {
			matchedIndex = i
			matchedName = instance.Name
			break
		}

		if instance.IsCluster {
			for _, ep := range instance.ClusterEndpoints {
				if strings.EqualFold(ep.NodeName, hostname) || strings.Contains(strings.ToLower(ep.Host), hostnameLower) {
					matchedIndex = i
					matchedName = instance.Name
					break
				}
			}
		}
	}

	if matchedIndex == -1 {
		writeErrorResponse(w, http.StatusNotFound, "pve_instance_not_found", fmt.Sprintf("No PVE instance found for hostname '%s'", hostname), nil)
		return
	}

	// Clear proxy configuration
	nodesConfig.PVEInstances[matchedIndex].TemperatureProxyURL = ""
	nodesConfig.PVEInstances[matchedIndex].TemperatureProxyToken = ""
	nodesConfig.PVEInstances[matchedIndex].TemperatureProxyControlToken = ""

	// Save updated configuration
	if err := h.persistence.SaveNodesConfig(nodesConfig.PVEInstances, nodesConfig.PBSInstances, nodesConfig.PMGInstances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "config_save_failed", "Failed to save configuration", map[string]string{"error": err.Error()})
		return
	}

	log.Info().
		Str("hostname", hostname).
		Str("pve_instance", matchedName).
		Msg("Temperature proxy unregistered")

	resp := map[string]any{
		"success":      true,
		"pve_instance": matchedName,
		"message":      "Temperature proxy configuration removed",
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize temperature proxy unregistration response")
	}
}

// generateSecureToken creates a cryptographically secure random token
func generateSecureToken(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
