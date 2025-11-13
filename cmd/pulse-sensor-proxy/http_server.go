package main

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// HTTPServer provides HTTP/HTTPS access to temperature data
type HTTPServer struct {
	proxy  *Proxy
	server *http.Server
	config *Config
}

// NewHTTPServer creates a new HTTP server for the proxy
func NewHTTPServer(proxy *Proxy, config *Config) *HTTPServer {
	return &HTTPServer{
		proxy:  proxy,
		config: config,
	}
}

// Start starts the HTTP server with TLS
func (h *HTTPServer) Start() error {
	if !h.config.HTTPEnabled {
		return nil
	}

	// Validate TLS certificate and key exist
	if h.config.HTTPTLSCertFile == "" || h.config.HTTPTLSKeyFile == "" {
		return fmt.Errorf("TLS cert and key required for HTTP mode")
	}

	mux := http.NewServeMux()

	// Register endpoints
	mux.HandleFunc("/temps", h.handleTemperature)
	mux.HandleFunc("/health", h.handleHealth)

	// Create TLS config with modern security settings
	tlsConfig := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	h.server = &http.Server{
		Addr:           h.config.HTTPListenAddr,
		Handler:        h.sourceIPMiddleware(h.rateLimitMiddleware(h.authMiddleware(mux))),
		TLSConfig:      tlsConfig,
		ReadTimeout:    h.config.ReadTimeout,
		WriteTimeout:   h.config.WriteTimeout,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	log.Info().
		Str("addr", h.config.HTTPListenAddr).
		Str("cert", h.config.HTTPTLSCertFile).
		Msg("Starting HTTPS server")

	go func() {
		if err := h.server.ListenAndServeTLS(h.config.HTTPTLSCertFile, h.config.HTTPTLSKeyFile); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTPS server failed")
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (h *HTTPServer) Stop(ctx context.Context) error {
	if h.server == nil {
		return nil
	}
	log.Info().Msg("Shutting down HTTPS server")
	return h.server.Shutdown(ctx)
}

// authMiddleware validates Bearer token authentication
func (h *HTTPServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.sendJSONError(w, http.StatusUnauthorized, "missing authorization header")
			if h.proxy.audit != nil {
				h.proxy.audit.LogHTTPRequest(r.RemoteAddr, r.Method, r.URL.Path, http.StatusUnauthorized, "missing_auth_header")
			}
			return
		}

		// Check Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			h.sendJSONError(w, http.StatusUnauthorized, "invalid authorization format")
			if h.proxy.audit != nil {
				h.proxy.audit.LogHTTPRequest(r.RemoteAddr, r.Method, r.URL.Path, http.StatusUnauthorized, "invalid_auth_format")
			}
			return
		}

		// Constant-time token comparison to prevent timing attacks
		providedToken := parts[1]
		if subtle.ConstantTimeCompare([]byte(providedToken), []byte(h.config.HTTPAuthToken)) != 1 {
			h.sendJSONError(w, http.StatusUnauthorized, "invalid token")
			if h.proxy.audit != nil {
				h.proxy.audit.LogHTTPRequest(r.RemoteAddr, r.Method, r.URL.Path, http.StatusUnauthorized, "invalid_token")
			}
			return
		}

		// Token valid, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// sourceIPMiddleware enforces allowed_source_subnets restrictions
func (h *HTTPServer) sourceIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP
		clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			clientIP = r.RemoteAddr
		}

		// Parse client IP
		ip := net.ParseIP(clientIP)
		if ip == nil {
			log.Warn().Str("remote_addr", r.RemoteAddr).Msg("Failed to parse client IP")
			h.sendJSONError(w, http.StatusForbidden, "invalid source IP")
			if h.proxy.audit != nil {
				h.proxy.audit.LogHTTPRequest(r.RemoteAddr, r.Method, r.URL.Path, http.StatusForbidden, "invalid_source_ip")
			}
			return
		}

		// Check if IP is in allowed subnets
		allowed := false
		for _, subnetStr := range h.config.AllowedSourceSubnets {
			_, subnet, err := net.ParseCIDR(subnetStr)
			if err != nil {
				continue
			}
			if subnet.Contains(ip) {
				allowed = true
				break
			}
		}

		if !allowed {
			log.Warn().
				Str("client_ip", clientIP).
				Str("path", r.URL.Path).
				Msg("HTTP request from unauthorized source IP")
			h.sendJSONError(w, http.StatusForbidden, "source IP not allowed")
			if h.proxy.audit != nil {
				h.proxy.audit.LogHTTPRequest(r.RemoteAddr, r.Method, r.URL.Path, http.StatusForbidden, "source_ip_not_allowed")
			}
			return
		}

		// IP is allowed, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware applies rate limiting per client IP
func (h *HTTPServer) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP
		clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			clientIP = r.RemoteAddr
		}

		// Create synthetic peer credentials for rate limiting
		// Use IP hash as UID for HTTP clients
		peerCred := &peerCredentials{
			uid: hashIPToUID(clientIP),
			gid: 0,
			pid: 0,
		}

		if h.proxy.rateLimiter == nil {
			h.sendJSONError(w, http.StatusServiceUnavailable, "rate limiter not available")
			return
		}

		// Check rate limit
		peer := h.proxy.rateLimiter.identifyPeer(peerCred)
		peerLabel := peer.String()
		releaseLimiter, limitReason, allowed := h.proxy.rateLimiter.allow(peer)
		if !allowed {
			log.Warn().
				Str("client_ip", clientIP).
				Str("reason", limitReason).
				Msg("HTTP rate limit exceeded")
			if h.proxy.audit != nil {
				h.proxy.audit.LogHTTPRequest(r.RemoteAddr, r.Method, r.URL.Path, http.StatusTooManyRequests, "rate_limit_"+limitReason)
			}
			h.sendJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		defer func() {
			if releaseLimiter != nil {
				releaseLimiter()
			}
		}()

		// Apply penalty if handler returns error
		releaseFn := releaseLimiter
		applyPenalty := func(reason string) {
			if releaseFn != nil {
				releaseFn()
				releaseFn = nil
			}
			h.proxy.rateLimiter.penalize(peerLabel, reason)
		}

		// Wrap response writer to detect errors
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrappedWriter, r)

		// Apply penalty for errors
		if wrappedWriter.statusCode >= 400 && wrappedWriter.statusCode != http.StatusTooManyRequests {
			applyPenalty("http_error")
		}
	})
}

// handleTemperature handles GET /temps?node=<nodename>
func (h *HTTPServer) handleTemperature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract node parameter
	nodeName := r.URL.Query().Get("node")
	if nodeName == "" {
		h.sendJSONError(w, http.StatusBadRequest, "missing 'node' query parameter")
		return
	}

	// Validate node name
	nodeName = strings.TrimSpace(nodeName)
	if err := validateNodeName(nodeName); err != nil {
		h.sendJSONError(w, http.StatusBadRequest, "invalid node name format")
		return
	}

	// Validate node against allowlist
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if h.proxy.nodeValidator != nil {
		if err := h.proxy.nodeValidator.Validate(ctx, nodeName); err != nil {
			log.Warn().Err(err).Str("node", nodeName).Msg("Node validation failed")
			h.sendJSONError(w, http.StatusForbidden, "node not allowed")
			return
		}
	}

	// Acquire per-node concurrency lock
	releaseNode := h.proxy.nodeGate.acquire(nodeName)
	defer releaseNode()

	// Fetch temperature data via SSH
	log.Debug().Str("node", nodeName).Msg("Fetching temperature via SSH (HTTP request)")
	tempData, err := h.proxy.getTemperatureViaSSH(nodeName)
	if err != nil {
		log.Warn().Err(err).Str("node", nodeName).Msg("Failed to get temperatures via SSH")
		h.sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get temperatures: %v", err))
		return
	}

	// Return temperature data as JSON
	response := map[string]interface{}{
		"node":        nodeName,
		"temperature": tempData,
	}

	log.Info().Str("node", nodeName).Msg("Temperature data fetched successfully via HTTP")
	h.sendJSON(w, http.StatusOK, response)

	if h.proxy.audit != nil {
		h.proxy.audit.LogHTTPRequest(r.RemoteAddr, r.Method, r.URL.Path, http.StatusOK, "temperature_success")
	}
}

// handleHealth handles GET /health
func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"version": Version,
	}

	h.sendJSON(w, http.StatusOK, response)
}

// sendJSON sends a JSON response
func (h *HTTPServer) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

// sendJSONError sends a JSON error response
func (h *HTTPServer) sendJSONError(w http.ResponseWriter, statusCode int, message string) {
	h.sendJSON(w, statusCode, map[string]interface{}{
		"error": message,
	})
}

// hashIPToUID creates a deterministic UID from an IP address for rate limiting
func hashIPToUID(ip string) uint32 {
	// Simple hash function: sum of byte values
	var hash uint32
	for i := 0; i < len(ip); i++ {
		hash = hash*31 + uint32(ip[i])
	}
	// Ensure it's in a reasonable range for UID
	return 100000 + (hash % 900000)
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
