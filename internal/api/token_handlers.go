package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

type TokenGenerateRequest struct {
	ValidityMinutes int      `json:"validityMinutes"`
	MaxUses         int      `json:"maxUses"`
	AllowedTypes    []string `json:"allowedTypes"`
	Description     string   `json:"description"`
}

type TokenResponse struct {
	Token       string   `json:"token"`
	Expires     string   `json:"expires"`
	MaxUses     int      `json:"maxUses"`
	UsedCount   int      `json:"usedCount"`
	Description string   `json:"description,omitempty"`
}

func (r *Router) handleGenerateToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var tokenReq TokenGenerateRequest
	if err := json.NewDecoder(req.Body).Decode(&tokenReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if tokenReq.ValidityMinutes <= 0 {
		tokenReq.ValidityMinutes = 15
	}
	if tokenReq.ValidityMinutes > 1440 {
		tokenReq.ValidityMinutes = 1440
	}

	if tokenReq.MaxUses < 0 {
		tokenReq.MaxUses = 1
	}
	if tokenReq.MaxUses > 100 {
		tokenReq.MaxUses = 100
	}

	clientIP := req.RemoteAddr
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.Split(forwarded, ",")[0]
	}

	token, err := r.tokenManager.GenerateToken(
		tokenReq.ValidityMinutes,
		tokenReq.MaxUses,
		tokenReq.AllowedTypes,
		clientIP,
		tokenReq.Description,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	response := TokenResponse{
		Token:       token.Token,
		Expires:     token.Expires.Format("2006-01-02T15:04:05Z07:00"),
		MaxUses:     token.MaxUses,
		UsedCount:   token.UsedCount,
		Description: token.Description,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (r *Router) handleListTokens(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenList := r.tokenManager.ListTokens()
	
	responses := make([]TokenResponse, len(tokenList))
	for i, token := range tokenList {
		responses[i] = TokenResponse{
			Token:       token.Token,
			Expires:     token.Expires.Format("2006-01-02T15:04:05Z07:00"),
			MaxUses:     token.MaxUses,
			UsedCount:   token.UsedCount,
			Description: token.Description,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (r *Router) handleRevokeToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenStr := req.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Token parameter required", http.StatusBadRequest)
		return
	}

	if err := r.tokenManager.RevokeToken(tokenStr); err != nil {
		if err.Error() == "token not found" {
			http.Error(w, "Token not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Msg("Failed to revoke token")
		http.Error(w, "Failed to revoke token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}