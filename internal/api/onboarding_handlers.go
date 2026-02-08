package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
)

const onboardingSchemaVersion = "pulse-mobile-onboarding-v1"

type onboardingRelayDetails struct {
	Enabled             bool   `json:"enabled"`
	URL                 string `json:"url"`
	IdentityFingerprint string `json:"identity_fingerprint,omitempty"`
	IdentityPublicKey   string `json:"identity_public_key,omitempty"`
}

type onboardingQRResponse struct {
	Schema      string                 `json:"schema"`
	InstanceURL string                 `json:"instance_url"`
	InstanceID  string                 `json:"instance_id,omitempty"`
	Relay       onboardingRelayDetails `json:"relay"`
	AuthToken   string                 `json:"auth_token"`
	DeepLink    string                 `json:"deep_link"`
	Diagnostics []onboardingDiagnostic `json:"diagnostics,omitempty"`
}

type onboardingDeepLinkResponse struct {
	URL         string                 `json:"url"`
	Diagnostics []onboardingDiagnostic `json:"diagnostics,omitempty"`
}

type onboardingValidationRequest struct {
	InstanceID string `json:"instance_id"`
	RelayURL   string `json:"relay_url"`
	AuthToken  string `json:"auth_token"`
}

type onboardingValidationResponse struct {
	Success     bool                   `json:"success"`
	Diagnostics []onboardingDiagnostic `json:"diagnostics,omitempty"`
}

type onboardingDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Received string `json:"received,omitempty"`
}

func (r *Router) handleGetOnboardingQR(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relayCfg, err := r.loadRelayConfigForOnboarding()
	if err != nil {
		http.Error(w, "failed to load relay config", http.StatusInternalServerError)
		return
	}

	payload, diagnostics := r.buildOnboardingPayload(req, relayCfg, onboardingAuthTokenFromRequest(req))
	if len(diagnostics) > 0 {
		payload.Diagnostics = diagnostics
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (r *Router) handleValidateOnboardingConnection(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload onboardingValidationRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	payload.InstanceID = strings.TrimSpace(payload.InstanceID)
	payload.RelayURL = strings.TrimSpace(payload.RelayURL)
	payload.AuthToken = strings.TrimSpace(payload.AuthToken)

	relayCfg, err := r.loadRelayConfigForOnboarding()
	if err != nil {
		http.Error(w, "failed to load relay config", http.StatusInternalServerError)
		return
	}

	diagnostics := make([]onboardingDiagnostic, 0, 8)

	if !relayCfg.Enabled {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "relay_disabled",
			Severity: "error",
			Field:    "relay_url",
			Message:  "Relay is disabled on this instance.",
		})
	}

	if payload.InstanceID == "" {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "instance_id_missing",
			Severity: "error",
			Field:    "instance_id",
			Message:  "instance_id is required.",
		})
	} else {
		expectedInstanceID := r.currentRelayInstanceID()
		if expectedInstanceID == "" {
			diagnostics = append(diagnostics, onboardingDiagnostic{
				Code:     "instance_id_unverifiable",
				Severity: "warning",
				Field:    "instance_id",
				Message:  "Relay is not currently connected; instance_id cannot be verified.",
			})
		} else if payload.InstanceID != expectedInstanceID {
			diagnostics = append(diagnostics, onboardingDiagnostic{
				Code:     "instance_id_mismatch",
				Severity: "error",
				Field:    "instance_id",
				Expected: expectedInstanceID,
				Received: payload.InstanceID,
				Message:  fmt.Sprintf("instance_id does not match the connected relay instance (%s).", expectedInstanceID),
			})
		}
	}

	if payload.RelayURL == "" {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "relay_url_missing",
			Severity: "error",
			Field:    "relay_url",
			Message:  "relay_url is required.",
		})
	} else {
		providedRelayURL, providedOK := normalizeRelayURL(payload.RelayURL)
		if !providedOK {
			diagnostics = append(diagnostics, onboardingDiagnostic{
				Code:     "relay_url_invalid",
				Severity: "error",
				Field:    "relay_url",
				Received: payload.RelayURL,
				Message:  "relay_url must be a valid ws:// or wss:// URL.",
			})
		} else {
			expectedRelayURL, expectedOK := normalizeRelayURL(relayCfg.ServerURL)
			if expectedOK && expectedRelayURL != providedRelayURL {
				diagnostics = append(diagnostics, onboardingDiagnostic{
					Code:     "relay_url_mismatch",
					Severity: "error",
					Field:    "relay_url",
					Expected: expectedRelayURL,
					Received: providedRelayURL,
					Message:  "relay_url does not match the configured relay server URL.",
				})
			}
		}
	}

	if payload.AuthToken == "" {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "auth_token_missing",
			Severity: "error",
			Field:    "auth_token",
			Message:  "auth_token is required.",
		})
	} else if !r.validateOnboardingAuthToken(payload.AuthToken) {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "auth_token_invalid",
			Severity: "error",
			Field:    "auth_token",
			Message:  "auth_token is not valid for this Pulse instance.",
		})
	}

	response := onboardingValidationResponse{
		Success:     !hasOnboardingError(diagnostics),
		Diagnostics: diagnostics,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (r *Router) handleGetOnboardingDeepLink(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relayCfg, err := r.loadRelayConfigForOnboarding()
	if err != nil {
		http.Error(w, "failed to load relay config", http.StatusInternalServerError)
		return
	}

	payload, diagnostics := r.buildOnboardingPayload(req, relayCfg, onboardingAuthTokenFromRequest(req))
	response := onboardingDeepLinkResponse{
		URL:         payload.DeepLink,
		Diagnostics: diagnostics,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (r *Router) buildOnboardingPayload(req *http.Request, relayCfg *relay.Config, authToken string) (onboardingQRResponse, []onboardingDiagnostic) {
	if relayCfg == nil {
		relayCfg = relay.DefaultConfig()
	}

	relayURL := strings.TrimSpace(relayCfg.ServerURL)
	if relayURL == "" {
		relayURL = relay.DefaultServerURL
	}

	payload := onboardingQRResponse{
		Schema:      onboardingSchemaVersion,
		InstanceURL: strings.TrimSpace(r.resolvePublicURL(req)),
		InstanceID:  r.currentRelayInstanceID(),
		Relay: onboardingRelayDetails{
			Enabled:             relayCfg.Enabled,
			URL:                 relayURL,
			IdentityFingerprint: strings.TrimSpace(relayCfg.IdentityFingerprint),
			IdentityPublicKey:   strings.TrimSpace(relayCfg.IdentityPublicKey),
		},
		AuthToken: authToken,
	}
	payload.DeepLink = buildOnboardingDeepLink(payload)

	diagnostics := make([]onboardingDiagnostic, 0, 4)
	if !payload.Relay.Enabled {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "relay_disabled",
			Severity: "warning",
			Field:    "relay.enabled",
			Message:  "Relay is disabled. Hosted mobile connection will not work until relay is enabled.",
		})
	}
	if payload.InstanceID == "" {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "instance_id_unavailable",
			Severity: "warning",
			Field:    "instance_id",
			Message:  "Relay instance_id is not available until relay registration succeeds.",
		})
	}
	if payload.AuthToken == "" {
		diagnostics = append(diagnostics, onboardingDiagnostic{
			Code:     "auth_token_missing",
			Severity: "warning",
			Field:    "auth_token",
			Message:  "No API token was provided in the request. Include X-API-Token or Authorization: Bearer for QR bootstrap payloads.",
		})
	}
	return payload, diagnostics
}

func (r *Router) loadRelayConfigForOnboarding() (*relay.Config, error) {
	if r != nil && r.persistence != nil {
		cfg, err := r.persistence.LoadRelayConfig()
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return relay.DefaultConfig(), nil
}

func (r *Router) currentRelayInstanceID() string {
	if r == nil {
		return ""
	}
	r.relayMu.RLock()
	client := r.relayClient
	r.relayMu.RUnlock()
	if client == nil {
		return ""
	}
	return strings.TrimSpace(client.Status().InstanceID)
}

func (r *Router) validateOnboardingAuthToken(token string) bool {
	if r == nil || r.config == nil {
		return false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	return r.config.IsValidAPIToken(token)
}

func onboardingAuthTokenFromRequest(req *http.Request) string {
	if req == nil {
		return ""
	}
	if token := strings.TrimSpace(req.Header.Get("X-API-Token")); token != "" {
		return token
	}
	if token := extractBearerToken(req.Header.Get("Authorization")); token != "" {
		return token
	}
	if token := strings.TrimSpace(req.URL.Query().Get("auth_token")); token != "" {
		return token
	}
	return ""
}

func normalizeRelayURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	parsed.Scheme = strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return "", false
	}

	parsed.Host = strings.ToLower(strings.TrimSpace(parsed.Host))
	if parsed.Host == "" {
		return "", false
	}

	parsed.RawFragment = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	}

	return parsed.String(), true
}

func hasOnboardingError(diagnostics []onboardingDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}

func buildOnboardingDeepLink(payload onboardingQRResponse) string {
	query := url.Values{}
	query.Set("schema", payload.Schema)
	query.Set("instance_url", payload.InstanceURL)
	if payload.InstanceID != "" {
		query.Set("instance_id", payload.InstanceID)
	}
	if payload.Relay.URL != "" {
		query.Set("relay_url", payload.Relay.URL)
	}
	if payload.AuthToken != "" {
		query.Set("auth_token", payload.AuthToken)
	}
	if payload.Relay.IdentityFingerprint != "" {
		query.Set("identity_fingerprint", payload.Relay.IdentityFingerprint)
	}
	if payload.Relay.IdentityPublicKey != "" {
		query.Set("identity_public_key", payload.Relay.IdentityPublicKey)
	}
	return (&url.URL{
		Scheme:   "pulse",
		Host:     "connect",
		RawQuery: query.Encode(),
	}).String()
}
