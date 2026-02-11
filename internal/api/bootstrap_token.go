package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const (
	bootstrapTokenFilename = ".bootstrap_token"
	bootstrapTokenHeader   = "X-Setup-Token"
)

func generateBootstrapToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func loadOrCreateBootstrapToken(dataPath string) (token string, created bool, fullPath string, err error) {
	if strings.TrimSpace(dataPath) == "" {
		return "", false, "", errors.New("data path required for bootstrap token")
	}

	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		return "", false, "", fmt.Errorf("ensure data path: %w", err)
	}

	fullPath = filepath.Join(dataPath, bootstrapTokenFilename)

	data, readErr := os.ReadFile(fullPath)
	if readErr == nil {
		token = strings.TrimSpace(string(data))
		if token == "" {
			return "", false, fullPath, errors.New("bootstrap token file is empty")
		}
		return token, false, fullPath, nil
	}

	if !errors.Is(readErr, os.ErrNotExist) {
		return "", false, fullPath, fmt.Errorf("read existing bootstrap token: %w", readErr)
	}

	token, err = generateBootstrapToken()
	if err != nil {
		return "", false, fullPath, fmt.Errorf("generate bootstrap token: %w", err)
	}

	if writeErr := os.WriteFile(fullPath, []byte(token+"\n"), 0o600); writeErr != nil {
		return "", false, fullPath, fmt.Errorf("persist bootstrap token: %w", writeErr)
	}

	return token, true, fullPath, nil
}

func (r *Router) initializeBootstrapToken() {
	if r == nil || r.config == nil {
		return
	}

	// If any authentication mechanism is already configured, purge stale bootstrap tokens.
	// In hosted mode, auth is handled by the cloud handoff — no bootstrap needed.
	if r.config.AuthUser != "" || r.config.AuthPass != "" || r.config.HasAPITokens() || r.config.ProxyAuthSecret != "" || r.hostedMode {
		r.clearBootstrapToken()
		return
	}
	if r.config.OIDC != nil && r.config.OIDC.Enabled {
		r.clearBootstrapToken()
		return
	}

	token, created, path, err := loadOrCreateBootstrapToken(r.config.DataPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to prepare bootstrap setup token")
		return
	}

	r.bootstrapTokenHash = internalauth.HashAPIToken(token)
	r.bootstrapTokenPath = path

	if created {
		// Display token prominently for easy discovery
		log.Warn().Msg("╔═══════════════════════════════════════════════════════════════════════╗")
		log.Warn().Msg("║          BOOTSTRAP TOKEN REQUIRED FOR FIRST-TIME SETUP                ║")
		log.Warn().Msg("╠═══════════════════════════════════════════════════════════════════════╣")
		log.Warn().Msgf("║  Token: %-61s ║", token)
		log.Warn().Msgf("║  File:  %-61s ║", path)
		log.Warn().Msg("╠═══════════════════════════════════════════════════════════════════════╣")
		log.Warn().Msg("║  Copy this token and paste it into the unlock screen in your browser. ║")
		log.Warn().Msg("║  This token will be automatically deleted after successful setup.     ║")
		log.Warn().Msg("╚═══════════════════════════════════════════════════════════════════════╝")
	} else {
		log.Info().
			Str("token_path", path).
			Msg("Bootstrap setup token loaded from disk")
	}
}

func (r *Router) bootstrapTokenValid(token string) bool {
	if r == nil || r.bootstrapTokenHash == "" {
		return false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	return internalauth.CompareAPIToken(token, r.bootstrapTokenHash)
}

func (r *Router) clearBootstrapToken() {
	if r == nil {
		return
	}

	if r.bootstrapTokenPath != "" {
		if err := os.Remove(r.bootstrapTokenPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn().
				Err(err).
				Str("token_path", r.bootstrapTokenPath).
				Msg("Failed to remove bootstrap setup token")
		} else if err == nil {
			log.Info().
				Str("token_path", r.bootstrapTokenPath).
				Msg("Bootstrap setup token removed")
		}
	}

	r.bootstrapTokenHash = ""
	r.bootstrapTokenPath = ""
}

func (r *Router) handleValidateBootstrapToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.bootstrapTokenHash == "" {
		http.Error(w, "Bootstrap token unavailable. Reload the page or restart Pulse.", http.StatusConflict)
		return
	}

	token := strings.TrimSpace(req.Header.Get(bootstrapTokenHeader))

	if token == "" {
		var payload struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		token = strings.TrimSpace(payload.Token)
	}

	if token == "" {
		http.Error(w, "Bootstrap token is required", http.StatusBadRequest)
		return
	}

	if !r.bootstrapTokenValid(token) {
		log.Warn().
			Str("ip", GetClientIP(req)).
			Msg("Rejected invalid bootstrap token validation request")
		http.Error(w, "Invalid bootstrap setup token", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
