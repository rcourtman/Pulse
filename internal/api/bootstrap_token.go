package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"weak"

	"github.com/rcourtman/pulse-go-rewrite/internal/bootstrap"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const (
	bootstrapTokenFilename      = bootstrap.TokenFilename
	bootstrapTokenHeader        = "X-Setup-Token"
	defaultBootstrapSetupWindow = 10 * time.Minute
	envRequireBootstrapToken    = "PULSE_REQUIRE_BOOTSTRAP_TOKEN"
)

var (
	bootstrapWindowMu       sync.RWMutex
	bootstrapWindowExpiry   = map[weak.Pointer[Router]]time.Time{}
	bootstrapWindowCleanups = map[weak.Pointer[Router]]runtime.Cleanup{}
	bootstrapSetupWindowNow = time.Now
)

func requireBootstrapTokenEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envRequireBootstrapToken))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (r *Router) openBootstrapSetupWindow() bool {
	if r == nil {
		return false
	}
	if requireBootstrapTokenEnabled() {
		r.closeBootstrapSetupWindow()
		return false
	}
	key := weak.Make(r)
	expiresAt := bootstrapSetupWindowNow().Add(defaultBootstrapSetupWindow)
	bootstrapWindowMu.Lock()
	bootstrapWindowExpiry[key] = expiresAt
	if _, registered := bootstrapWindowCleanups[key]; !registered {
		bootstrapWindowCleanups[key] = runtime.AddCleanup(r, func(k weak.Pointer[Router]) {
			bootstrapWindowMu.Lock()
			delete(bootstrapWindowExpiry, k)
			delete(bootstrapWindowCleanups, k)
			bootstrapWindowMu.Unlock()
		}, key)
	}
	bootstrapWindowMu.Unlock()
	return true
}

func (r *Router) closeBootstrapSetupWindow() {
	if r == nil {
		return
	}
	key := weak.Make(r)
	bootstrapWindowMu.Lock()
	delete(bootstrapWindowExpiry, key)
	if cleanup, ok := bootstrapWindowCleanups[key]; ok {
		cleanup.Stop()
		delete(bootstrapWindowCleanups, key)
	}
	bootstrapWindowMu.Unlock()
}

// IsBootstrapSetupWindowOpen reports whether first-time setup can currently
// proceed directly to admin account creation without an unlock token. The
// window opens for 10 minutes on a fresh boot unless PULSE_REQUIRE_BOOTSTRAP_TOKEN
// is enabled, and closes immediately once authentication is configured or the
// window expires.
func (r *Router) IsBootstrapSetupWindowOpen() bool {
	if r == nil || r.bootstrapTokenHash == "" || requireBootstrapTokenEnabled() {
		return false
	}
	key := weak.Make(r)
	bootstrapWindowMu.RLock()
	expiresAt, ok := bootstrapWindowExpiry[key]
	bootstrapWindowMu.RUnlock()
	if !ok || expiresAt.IsZero() {
		return false
	}
	return bootstrapSetupWindowNow().Before(expiresAt)
}

func generateBootstrapToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func loadOrCreateBootstrapToken(dataPath string) (token string, created bool, fullPath string, err error) {
	token, created, fullPath, _, err = bootstrap.LoadOrCreate(dataPath, generateBootstrapToken)
	return token, created, fullPath, err
}

func (r *Router) initializeBootstrapToken() {
	if r == nil || r.config == nil {
		return
	}

	// If any authentication mechanism is already configured, purge stale bootstrap tokens.
	// In hosted mode, auth is handled by the cloud handoff — no bootstrap needed.
	hasEnabledSSO := false
	if ssoCfg := r.ensureSSOConfig(); ssoCfg != nil {
		hasEnabledSSO = ssoCfg.HasEnabledProviders()
	}
	if r.config.AuthUser != "" || r.config.AuthPass != "" || r.config.HasAPITokens() || r.config.ProxyAuthSecret != "" || r.hostedMode || hasEnabledSSO || r.ssoAuthenticationLoadFailed() {
		r.clearBootstrapToken()
		return
	}

	token, created, path, err := loadOrCreateBootstrapToken(r.config.DataPath)
	logger := r.logEvents()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to prepare bootstrap setup token")
		return
	}

	r.bootstrapTokenHash = internalauth.HashAPIToken(token)
	r.bootstrapTokenPath = path
	windowOpen := r.openBootstrapSetupWindow()

	if windowOpen {
		logger.Info().
			Str("token_path", path).
			Dur("setup_window", defaultBootstrapSetupWindow).
			Msg("First-boot setup window open for 10 minutes; open Pulse in your browser to create your admin account (if the window expires, restart Pulse or run `pulse bootstrap-token`)")
	} else if created {
		logger.Warn().
			Str("token_path", path).
			Msg("Bootstrap setup token created; the file is encrypted, so reveal the token with `pulse bootstrap-token` (Docker: `docker exec <container> /app/pulse bootstrap-token`, or `/app/pulse bootstrap-token` from the container console)")
	} else {
		logger.Info().
			Str("token_path", path).
			Msg("Bootstrap setup token loaded from disk")
	}
}

// HasPendingBootstrapToken reports whether first-time setup is still waiting
// for an operator to unlock the server with the generated bootstrap token.
func (r *Router) HasPendingBootstrapToken() bool {
	return r != nil && r.bootstrapTokenHash != ""
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
		logger := r.logEvents()
		if err := os.Remove(r.bootstrapTokenPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Warn().
				Err(err).
				Str("token_path", r.bootstrapTokenPath).
				Msg("Failed to remove bootstrap setup token")
		} else if err == nil {
			logger.Info().
				Str("token_path", r.bootstrapTokenPath).
				Msg("Bootstrap setup token removed")
		}
	}

	r.bootstrapTokenHash = ""
	r.bootstrapTokenPath = ""
	r.closeBootstrapSetupWindow()
}

func (r *Router) handleValidateBootstrapToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := GetClientIP(req)
	if clientIP == "" {
		clientIP = extractRemoteIP(req.RemoteAddr)
	}
	if clientIP == "" {
		clientIP = req.RemoteAddr
	}

	if limiter := r.bootstrapTokenLimiter(); limiter != nil {
		if allowed, retryAfter := limiter.allowAt(clientIP, time.Now()); !allowed {
			retrySeconds := int(retryAfter.Round(time.Second) / time.Second)
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
			log.Warn().
				Str("ip", clientIP).
				Int("retry_after_seconds", retrySeconds).
				Msg("Rejected bootstrap token validation request due to rate limit")
			http.Error(w, "Too many bootstrap token validation attempts", http.StatusTooManyRequests)
			return
		}
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
		if err := decodeSecurityRequestBody(w, req, &payload); err != nil {
			http.Error(w, "Invalid request payload", securityRequestErrorStatus(err))
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
			Str("ip", clientIP).
			Msg("Rejected invalid bootstrap token validation request")
		http.Error(w, "Invalid bootstrap setup token", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) bootstrapTokenLimiter() *RateLimiter {
	if r == nil {
		return nil
	}
	if r.bootstrapTokenValidationLimiter == nil {
		r.bootstrapTokenValidationLimiter = NewRateLimiter(10, 5*time.Minute)
	}
	return r.bootstrapTokenValidationLimiter
}
