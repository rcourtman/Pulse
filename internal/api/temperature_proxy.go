package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// TemperatureProxyHandlers manages temperature proxy registration
type TemperatureProxyHandlers struct {
	persistence *config.ConfigPersistence
}

// NewTemperatureProxyHandlers constructs a new handler set for temperature proxy
func NewTemperatureProxyHandlers(persistence *config.ConfigPersistence) *TemperatureProxyHandlers {
	return &TemperatureProxyHandlers{persistence: persistence}
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

	if proxyURL == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_proxy_url", "Proxy URL is required", nil)
		return
	}

	// Validate proxy URL format
	if !strings.HasPrefix(proxyURL, "https://") {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_proxy_url", "Proxy URL must use HTTPS", nil)
		return
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

	for i := range nodesConfig.PVEInstances {
		instance := &nodesConfig.PVEInstances[i]

		// Try to match by instance name
		if strings.EqualFold(instance.Name, hostname) {
			matchedInstance = instance
			matchedIndex = i
			break
		}

		// Try to match by hostname in the Host field or cluster endpoints
		if strings.Contains(strings.ToLower(instance.Host), strings.ToLower(hostname)) {
			matchedInstance = instance
			matchedIndex = i
			break
		}

		// Check cluster endpoints
		if instance.IsCluster {
			for _, ep := range instance.ClusterEndpoints {
				if strings.EqualFold(ep.NodeName, hostname) || strings.Contains(strings.ToLower(ep.Host), strings.ToLower(hostname)) {
					matchedInstance = instance
					matchedIndex = i
					break
				}
			}
		}
	}

	if matchedInstance == nil {
		writeErrorResponse(w, http.StatusNotFound, "pve_instance_not_found", fmt.Sprintf("No PVE instance configured for hostname '%s'. Add the instance in Pulse first.", hostname), nil)
		return
	}

	// Generate a secure random token
	token, err := generateSecureToken(32)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate authentication token", map[string]string{"error": err.Error()})
		return
	}

	// Update the instance with proxy configuration
	nodesConfig.PVEInstances[matchedIndex].TemperatureProxyURL = proxyURL
	nodesConfig.PVEInstances[matchedIndex].TemperatureProxyToken = token

	// Save updated configuration
	if err := h.persistence.SaveNodesConfig(nodesConfig.PVEInstances, nodesConfig.PBSInstances, nodesConfig.PMGInstances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "config_save_failed", "Failed to save configuration", map[string]string{"error": err.Error()})
		return
	}

	log.Info().
		Str("hostname", hostname).
		Str("proxy_url", proxyURL).
		Str("pve_instance", matchedInstance.Name).
		Msg("Temperature proxy registered successfully")

	resp := map[string]any{
		"success":      true,
		"token":        token,
		"pve_instance": matchedInstance.Name,
		"message":      fmt.Sprintf("Temperature proxy registered for instance '%s'", matchedInstance.Name),
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize temperature proxy registration response")
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

	for i := range nodesConfig.PVEInstances {
		instance := &nodesConfig.PVEInstances[i]

		if strings.Contains(strings.ToLower(instance.Host), strings.ToLower(hostname)) {
			matchedIndex = i
			matchedName = instance.Name
			break
		}

		if instance.IsCluster {
			for _, ep := range instance.ClusterEndpoints {
				if strings.EqualFold(ep.NodeName, hostname) || strings.Contains(strings.ToLower(ep.Host), strings.ToLower(hostname)) {
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
