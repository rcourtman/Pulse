package proxmox

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
	"github.com/rs/zerolog/log"
)

// ClusterClient wraps multiple Proxmox clients for cluster-aware operations
type ClusterClient struct {
	mu                   sync.RWMutex
	name                 string
	clients              map[string]*Client   // Key is node name
	endpoints            []string             // All available endpoints
	endpointFingerprints map[string]string    // Per-endpoint TLS fingerprints (TOFU)
	nodeHealth           map[string]bool      // Track node health
	lastHealthCheck      map[string]time.Time // Track last health check time
	lastError            map[string]string    // Track last error per endpoint
	config               ClientConfig         // Base config (auth info)
	rateLimitUntil       map[string]time.Time // Cooldown window for rate-limited endpoints
}

const (
	rateLimitBaseDelay   = 150 * time.Millisecond
	rateLimitMaxJitter   = 200 * time.Millisecond
	rateLimitRetryBudget = 2
)

var statusCodePattern = regexp.MustCompile(`(?i)(?:api error|status)\s+(\d{3})`)

var transientRateLimitStatusCodes = map[int]struct{}{
	408: {},
	425: {}, // Too Early
	429: {},
	502: {},
	503: {},
	504: {},
}

// isVMSpecificError reports whether an error string is scoped to a single VM/guest agent
// and should not be treated as a node connectivity failure.
func isVMSpecificError(errStr string) bool {
	if errStr == "" {
		return false
	}
	lower := strings.ToLower(errStr)

	if strings.Contains(lower, "no qemu guest agent") ||
		strings.Contains(lower, "qemu guest agent is not running") ||
		strings.Contains(lower, "guest agent") {
		return true
	}

	// QMP guest agent operations can time out or fail per-VM (e.g. guest-get-fsinfo).
	// These aren't node connectivity issues and should not mark endpoints unhealthy.
	if strings.Contains(lower, "qmp command") {
		return true
	}

	if strings.Contains(lower, "guest-get-") {
		return true
	}

	return false
}

// isEndpointConnectivityError reports whether an error indicates the endpoint
// itself is unreachable (TCP, DNS, TLS failures). Application-level errors
// (HTTP responses, parsing errors) mean the endpoint IS reachable.
func isEndpointConnectivityError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())

	// If we received an HTTP response from Proxmox (any status code),
	// the endpoint is reachable.
	if strings.Contains(errStr, "api error") {
		return false
	}

	// TCP/DNS connectivity failures
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "dial:") {
		return true
	}

	// TLS failures
	if strings.Contains(errStr, "tls handshake") ||
		strings.Contains(errStr, "tls:") ||
		strings.Contains(errStr, "certificate") ||
		strings.Contains(errStr, "fingerprint mismatch") {
		return true
	}

	return false
}

// sanitizeEndpointError transforms raw Go errors into user-friendly messages
// for display in the UI. The original error is preserved in logs.
func sanitizeEndpointError(errMsg string) string {
	if errMsg == "" {
		return errMsg
	}

	lower := strings.ToLower(errMsg)

	// Context deadline exceeded - usually means slow API response
	if strings.Contains(lower, "context deadline exceeded") {
		// Check for specific causes
		if strings.Contains(lower, "/storage") {
			return "Request timed out - storage API slow (check for unreachable PBS/NFS/Ceph backends)"
		}
		if strings.Contains(lower, "pbs-") || strings.Contains(lower, ":8007") {
			return "Request timed out - PBS storage backend unreachable"
		}
		return "Request timed out - Proxmox API may be slow or waiting on unreachable backend services"
	}

	// Client timeout - similar to context deadline
	if strings.Contains(lower, "client.timeout exceeded") {
		return "Connection timed out - Proxmox API not responding in time"
	}

	// Connection refused
	if strings.Contains(lower, "connection refused") {
		return "Connection refused - Proxmox API not running or firewall blocking"
	}

	// No route to host
	if strings.Contains(lower, "no route to host") {
		return "Network unreachable - check network connectivity to Proxmox host"
	}

	// TLS/certificate errors
	if strings.Contains(lower, "certificate") || strings.Contains(lower, "x509") {
		return "TLS certificate error - check SSL settings or add fingerprint"
	}

	// Auth errors - keep these specific
	if strings.Contains(lower, "authentication") || strings.Contains(lower, "401") || strings.Contains(lower, "403") {
		return "Authentication failed - check API token or credentials"
	}

	// PBS-specific errors
	if strings.Contains(lower, "can't connect to") && strings.Contains(lower, ":8007") {
		return "PBS storage unreachable - check Proxmox Backup Server connectivity"
	}

	// Return original if no transformation applies
	return errMsg
}

// NewClusterClient creates a new cluster-aware client.
// endpointFingerprints is an optional map of endpoint URL -> TLS fingerprint for per-node certificate verification.
// This enables TOFU (Trust On First Use) for clusters with unique self-signed certs per node.
func NewClusterClient(name string, config ClientConfig, endpoints []string, endpointFingerprints map[string]string) *ClusterClient {
	if endpointFingerprints == nil {
		endpointFingerprints = make(map[string]string)
	}
	cc := &ClusterClient{
		name:                 name,
		clients:              make(map[string]*Client),
		endpoints:            endpoints,
		endpointFingerprints: endpointFingerprints,
		nodeHealth:           make(map[string]bool),
		lastHealthCheck:      make(map[string]time.Time),
		lastError:            make(map[string]string),
		config:               config,
		rateLimitUntil:       make(map[string]time.Time),
	}

	// Initialize all endpoints as unknown (will be tested on first use)
	// Start optimistically - assume healthy until proven otherwise
	// This allows operations to be attempted even if initial health check fails
	for _, endpoint := range endpoints {
		cc.nodeHealth[endpoint] = true // Start optimistic, will be marked unhealthy if operations fail
	}

	// Do a quick parallel health check on initialization (synchronous to avoid race)
	// This will mark unhealthy nodes but won't prevent trying them later
	cc.initialHealthCheck()

	return cc
}

// getEndpointFingerprint returns the TLS fingerprint to use for a specific endpoint.
// It prefers the per-endpoint fingerprint (TOFU) over the base config fingerprint.
func (cc *ClusterClient) getEndpointFingerprint(endpoint string) string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.getEndpointFingerprintLocked(endpoint)
}

func (cc *ClusterClient) getEndpointFingerprintLocked(endpoint string) string {
	if fp, ok := cc.endpointFingerprints[endpoint]; ok && fp != "" {
		return fp
	}
	return cc.config.Fingerprint
}

func isFingerprintMismatchError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "fingerprint mismatch")
}

// refreshTOFUFingerprintAndRetry refreshes a per-endpoint TOFU fingerprint after a mismatch and retries connectivity.
// Returns (client, err, refreshed) where refreshed indicates whether TOFU refresh logic was applied.
func (cc *ClusterClient) refreshTOFUFingerprintAndRetry(ctx context.Context, endpoint string, timeout time.Duration, lastErr error) (*Client, error, bool) {
	if !isFingerprintMismatchError(lastErr) {
		return nil, lastErr, false
	}

	cc.mu.RLock()
	_, hasTOFU := cc.endpointFingerprints[endpoint]
	cc.mu.RUnlock()
	if !hasTOFU {
		return nil, lastErr, false
	}

	newFingerprint, err := tlsutil.FetchFingerprint(endpoint)
	if err != nil {
		log.Warn().
			Str("cluster", cc.name).
			Str("endpoint", endpoint).
			Err(err).
			Msg("Failed to refresh TOFU fingerprint after mismatch")
		return nil, lastErr, false
	}

	cc.mu.Lock()
	cc.endpointFingerprints[endpoint] = newFingerprint
	cc.mu.Unlock()

	log.Warn().
		Str("cluster", cc.name).
		Str("endpoint", endpoint).
		Msg("Detected TLS certificate change; refreshed TOFU fingerprint")

	cfg := cc.config
	cfg.Host = endpoint
	cfg.Fingerprint = newFingerprint
	cfg.Timeout = timeout

	retryClient, err := NewClient(cfg)
	if err != nil {
		return nil, err, true
	}

	retryCtx, cancel := context.WithTimeout(ctx, timeout)
	_, err = retryClient.GetNodes(retryCtx)
	cancel()

	return retryClient, err, true
}

// initialHealthCheck performs a quick parallel health check on all endpoints
func (cc *ClusterClient) initialHealthCheck() {
	// Skip initial health check if there's only one endpoint
	// For single-endpoint clusters (using main host for routing), assume healthy
	if len(cc.endpoints) == 1 {
		log.Info().
			Str("cluster", cc.name).
			Str("endpoint", cc.endpoints[0]).
			Msg("Single endpoint cluster - skipping initial health check")
		return
	}

	// For multi-node clusters, do a very quick check but don't mark unhealthy immediately
	// This prevents nodes from being marked unhealthy due to temporary startup conditions

	var wg sync.WaitGroup
	for _, endpoint := range cc.endpoints {
		wg.Add(1)
		go func(ep string) {
			defer wg.Done()

			// Try a quick connection test with slightly longer timeout for initial check
			cfg := cc.config
			cfg.Host = ep
			cfg.Fingerprint = cc.getEndpointFingerprint(ep)
			cfg.Timeout = 5 * time.Second

			testClient, err := NewClient(cfg)
			if err != nil {
				cc.mu.Lock()
				cc.nodeHealth[ep] = false
				cc.lastError[ep] = sanitizeEndpointError(err.Error())
				cc.lastHealthCheck[ep] = time.Now()
				cc.mu.Unlock()
				log.Info().
					Str("cluster", cc.name).
					Str("endpoint", ep).
					Err(err).
					Msg("Cluster endpoint marked unhealthy on initialization")
				return
			}

			// Quick test with slightly longer timeout for initial check
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err = testClient.GetNodes(ctx)
			cancel()

			if _, retryErr, refreshed := cc.refreshTOFUFingerprintAndRetry(context.Background(), ep, 5*time.Second, err); refreshed {
				err = retryErr
			}

			// Check if error is VM-specific (shouldn't affect health)
			vmSpecificErr := err != nil && isVMSpecificError(err.Error())

			if err == nil || vmSpecificErr {
				// Node is healthy - create a proper client with full timeout for actual use
				fullCfg := cc.config
				fullCfg.Host = ep
				fullCfg.Fingerprint = cc.getEndpointFingerprint(ep)
				fullClient, clientErr := NewClient(fullCfg)

				cc.mu.Lock()
				if clientErr != nil {
					cc.nodeHealth[ep] = false
					cc.lastError[ep] = sanitizeEndpointError(clientErr.Error())
					cc.lastHealthCheck[ep] = time.Now()
					cc.mu.Unlock()
					log.Warn().
						Str("cluster", cc.name).
						Str("endpoint", ep).
						Err(clientErr).
						Msg("Failed to create full client after successful health check")
				} else {
					cc.nodeHealth[ep] = true
					delete(cc.lastError, ep)
					cc.lastHealthCheck[ep] = time.Now()
					cc.clients[ep] = fullClient // Store the full client, not test client
					cc.mu.Unlock()
					if vmSpecificErr {
						log.Debug().
							Str("cluster", cc.name).
							Str("endpoint", ep).
							Msg("Cluster endpoint healthy despite VM-specific errors")
					} else {
						log.Info().
							Str("cluster", cc.name).
							Str("endpoint", ep).
							Msg("Cluster endpoint passed initial health check")
					}
				}
			} else {
				// Real connectivity issue
				cc.mu.Lock()
				cc.nodeHealth[ep] = false
				cc.lastError[ep] = sanitizeEndpointError(err.Error())
				cc.lastHealthCheck[ep] = time.Now()
				cc.mu.Unlock()
				log.Info().
					Str("cluster", cc.name).
					Str("endpoint", ep).
					Err(err).
					Msg("Cluster endpoint failed initial health check")
			}
		}(endpoint)
	}

	// Wait for all checks to complete
	wg.Wait()

	log.Info().
		Str("cluster", cc.name).
		Int("total", len(cc.endpoints)).
		Msg("Initial cluster health check completed")
}

// getHealthyClient returns a healthy client using round-robin selection
func (cc *ClusterClient) getHealthyClient(ctx context.Context) (*Client, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Get list of healthy endpoints
	var healthyEndpoints []string
	var coolingEndpoints []string
	now := time.Now()
	for endpoint, healthy := range cc.nodeHealth {
		if healthy {
			if cooldown, exists := cc.rateLimitUntil[endpoint]; exists {
				if now.Before(cooldown) {
					coolingEndpoints = append(coolingEndpoints, endpoint)
					continue
				}
				delete(cc.rateLimitUntil, endpoint)
			}
			healthyEndpoints = append(healthyEndpoints, endpoint)
		}
	}

	if len(healthyEndpoints) == 0 && len(coolingEndpoints) > 0 {
		// Nothing is immediately available, fall back to endpoints that are in cooldown
		healthyEndpoints = append(healthyEndpoints, coolingEndpoints...)
	}

	// Count unhealthy endpoints for logging and recovery decisions
	unhealthyCount := 0
	for _, healthy := range cc.nodeHealth {
		if !healthy {
			unhealthyCount++
		}
	}

	// Log at warn level if no healthy endpoints to aid troubleshooting
	if len(healthyEndpoints) == 0 && len(coolingEndpoints) == 0 {
		log.Warn().
			Str("cluster", cc.name).
			Int("healthy", len(healthyEndpoints)).
			Int("total", len(cc.nodeHealth)).
			Interface("nodeHealth", cc.nodeHealth).
			Msg("No healthy endpoints available - attempting recovery")
	} else {
		log.Debug().
			Str("cluster", cc.name).
			Int("healthy", len(healthyEndpoints)).
			Int("cooling", len(coolingEndpoints)).
			Int("total", len(cc.nodeHealth)).
			Interface("nodeHealth", cc.nodeHealth).
			Msg("Checking for healthy endpoints")
	}

	// Trigger recovery if we have any unhealthy endpoints
	// This ensures degraded clusters recover individual nodes over time,
	// not just when all nodes are down
	if unhealthyCount > 0 {
		// Use an anonymous function to ensure the lock is re-acquired even if
		// recoverUnhealthyNodes panics, preventing double-unlock from defer
		func() {
			cc.mu.Unlock()
			defer cc.mu.Lock()
			cc.recoverUnhealthyNodes(ctx)
		}()

		// Refresh the healthy/cooling endpoints lists after recovery attempt
		// since cluster state may have changed while lock was released
		healthyEndpoints = nil
		coolingEndpoints = nil
		now = time.Now() // Refresh time for accurate cooldown checks
		for endpoint, healthy := range cc.nodeHealth {
			if healthy {
				if cooldown, exists := cc.rateLimitUntil[endpoint]; exists && now.Before(cooldown) {
					coolingEndpoints = append(coolingEndpoints, endpoint)
					continue
				}
				healthyEndpoints = append(healthyEndpoints, endpoint)
			}
		}

		// Re-apply cooldown fallback if no healthy endpoints but some cooling
		if len(healthyEndpoints) == 0 && len(coolingEndpoints) > 0 {
			healthyEndpoints = append(healthyEndpoints, coolingEndpoints...)
		}
	}

	if len(healthyEndpoints) == 0 {
		// If still no healthy endpoints and we only have one endpoint,
		// try to use it anyway (could be temporarily unreachable)
		if len(cc.endpoints) == 1 {
			log.Warn().
				Str("cluster", cc.name).
				Str("endpoint", cc.endpoints[0]).
				Msg("Single endpoint appears unhealthy but attempting to use it anyway")
			healthyEndpoints = cc.endpoints
			// Mark it as healthy optimistically
			cc.nodeHealth[cc.endpoints[0]] = true
		} else {
			// Provide detailed error with endpoint status
			unhealthyList := make([]string, 0, len(cc.endpoints))
			for _, ep := range cc.endpoints {
				if !cc.nodeHealth[ep] {
					unhealthyList = append(unhealthyList, ep)
				}
			}
			log.Error().
				Str("cluster", cc.name).
				Strs("unhealthyEndpoints", unhealthyList).
				Int("totalEndpoints", len(cc.endpoints)).
				Msg("All cluster endpoints are unhealthy - verify network connectivity and API accessibility from Pulse server")
			return nil, fmt.Errorf("no healthy nodes available in cluster %s (all %d endpoints unreachable: %v)", cc.name, len(cc.endpoints), unhealthyList)
		}
	}

	// Use random selection for better load distribution
	selectedEndpoint := healthyEndpoints[rand.Intn(len(healthyEndpoints))]

	// Get or create client for this endpoint
	client, exists := cc.clients[selectedEndpoint]
	if !exists {
		// Create new client with shorter timeout for initial test
		cfg := cc.config
		cfg.Host = selectedEndpoint
		cfg.Fingerprint = cc.getEndpointFingerprintLocked(selectedEndpoint)

		// First try with a short timeout to quickly detect offline nodes
		testCfg := cfg
		testCfg.Timeout = 3 * time.Second

		testClient, err := NewClient(testCfg)
		if err != nil {
			// Mark as unhealthy
			cc.nodeHealth[selectedEndpoint] = false
			log.Debug().
				Str("cluster", cc.name).
				Str("endpoint", selectedEndpoint).
				Err(err).
				Msg("Failed to create client for cluster endpoint")
			return nil, fmt.Errorf("failed to create client for %s: %w", selectedEndpoint, err)
		}

		// Connectivity test - 5 seconds to allow for TLS handshake (~3s typical)
		testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		testNodes, testErr := testClient.GetNodes(testCtx)
		cancel()

		if testErr != nil {
			// Check if this is a transient rate limit error that shouldn't mark the node unhealthy
			if isRateLimited, _ := isTransientRateLimitError(testErr); isRateLimited {
				log.Debug().
					Str("cluster", cc.name).
					Str("endpoint", selectedEndpoint).
					Err(testErr).
					Msg("Ignoring transient rate limit error during connectivity test")
				// Continue with client creation since the node is accessible, just rate limited
			} else {
				// Check if this is a VM-specific error that shouldn't mark the node unhealthy
				testErrStr := testErr.Error()
				if strings.Contains(testErrStr, "No QEMU guest agent") ||
					strings.Contains(testErrStr, "QEMU guest agent is not running") ||
					strings.Contains(testErrStr, "guest agent") {
					// This is a VM-specific issue, not a connectivity problem
					// The node is actually healthy, so don't mark it unhealthy
					log.Debug().
						Str("cluster", cc.name).
						Str("endpoint", selectedEndpoint).
						Err(testErr).
						Msg("Ignoring VM-specific error during connectivity test")
					// Continue with client creation since the node is actually accessible
				} else {
					// Mark as unhealthy for real connectivity issues
					cc.nodeHealth[selectedEndpoint] = false
					log.Warn().
						Str("cluster", cc.name).
						Str("endpoint", selectedEndpoint).
						Err(testErr).
						Msg("Failed to connect to Proxmox endpoint; endpoint removed from rotation until next refresh")
					return nil, fmt.Errorf("endpoint %s failed connectivity test: %w", selectedEndpoint, testErr)
				}
			}
		}

		log.Debug().
			Str("cluster", cc.name).
			Str("endpoint", selectedEndpoint).
			Int("nodes", len(testNodes)).
			Msg("Cluster endpoint passed connectivity test")

		// Clear any stale error from previous failures now that connectivity succeeded
		delete(cc.lastError, selectedEndpoint)

		// Create the actual client with full timeout
		newClient, err := NewClient(cfg)
		if err != nil {
			// This shouldn't happen since we just tested it
			cc.nodeHealth[selectedEndpoint] = false
			return nil, fmt.Errorf("failed to create client for %s: %w", selectedEndpoint, err)
		}

		cc.clients[selectedEndpoint] = newClient
		client = newClient
	}

	return client, nil
}

// markUnhealthyWithError marks an endpoint as unhealthy and captures the error
func (cc *ClusterClient) markUnhealthyWithError(endpoint string, errMsg string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.nodeHealth[endpoint] {
		log.Warn().
			Str("cluster", cc.name).
			Str("endpoint", endpoint).
			Str("error", errMsg).
			Msg("Marking cluster node as unhealthy")
		cc.nodeHealth[endpoint] = false
	}
	if errMsg != "" {
		cc.lastError[endpoint] = sanitizeEndpointError(errMsg)
	}
	cc.lastHealthCheck[endpoint] = time.Now()
}

// clearEndpointError removes any cached error for an endpoint after successful operations
// and marks the endpoint as healthy since the operation succeeded
func (cc *ClusterClient) clearEndpointError(endpoint string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	delete(cc.lastError, endpoint)
	delete(cc.rateLimitUntil, endpoint)
	// Mark endpoint healthy since operation succeeded - this ensures degraded
	// clusters recover once endpoints start responding again
	cc.nodeHealth[endpoint] = true
}

// recoverUnhealthyNodes attempts to recover unhealthy nodes
func (cc *ClusterClient) recoverUnhealthyNodes(ctx context.Context) {
	cc.mu.RLock()
	unhealthyEndpoints := make([]string, 0)
	throttledEndpoints := make([]string, 0)
	now := time.Now()
	for endpoint, healthy := range cc.nodeHealth {
		if !healthy {
			// Skip if we checked this endpoint recently (within 10 seconds)
			// Balance between recovery speed and avoiding excessive checks
			if lastCheck, exists := cc.lastHealthCheck[endpoint]; exists {
				if now.Sub(lastCheck) < 10*time.Second {
					throttledEndpoints = append(throttledEndpoints, endpoint)
					continue
				}
			}
			unhealthyEndpoints = append(unhealthyEndpoints, endpoint)
		}
	}
	cc.mu.RUnlock()

	if len(unhealthyEndpoints) == 0 {
		if len(throttledEndpoints) > 0 {
			log.Debug().
				Str("cluster", cc.name).
				Strs("throttledEndpoints", throttledEndpoints).
				Msg("Skipping recovery check - endpoints checked recently")
		}
		return
	}

	log.Info().
		Str("cluster", cc.name).
		Strs("unhealthyEndpoints", unhealthyEndpoints).
		Int("count", len(unhealthyEndpoints)).
		Msg("Attempting to recover unhealthy cluster endpoints")

	// Test all unhealthy endpoints concurrently with a short timeout
	var wg sync.WaitGroup
	recoveredEndpoints := make(chan string, len(unhealthyEndpoints))

	for _, endpoint := range unhealthyEndpoints {
		wg.Add(1)
		go func(ep string) {
			defer wg.Done()

			// Update last check time
			cc.mu.Lock()
			cc.lastHealthCheck[ep] = now
			cc.mu.Unlock()

			// Try to create a client and test connection
			// Note: 5-second timeout needed because TLS handshake to Proxmox API
			// typically takes ~3 seconds on local networks
			cfg := cc.config
			cfg.Host = ep
			cfg.Fingerprint = cc.getEndpointFingerprint(ep)
			cfg.Timeout = 5 * time.Second

			testClient, err := NewClient(cfg)
			if err != nil {
				log.Debug().
					Str("cluster", cc.name).
					Str("endpoint", ep).
					Err(err).
					Msg("Failed to create client during recovery attempt")
				return
			}

			// Try a simple API call
			testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err = testClient.GetNodes(testCtx)
			cancel()

			if _, retryErr, refreshed := cc.refreshTOFUFingerprintAndRetry(ctx, ep, 5*time.Second, err); refreshed {
				err = retryErr
				cfg.Fingerprint = cc.getEndpointFingerprint(ep)
			}

			// Check if error is VM-specific (shouldn't prevent recovery)
			vmSpecificErr := err != nil && isVMSpecificError(err.Error())

			if err == nil || vmSpecificErr {
				recoveredEndpoints <- ep

				// Store the client with original timeout
				cfg.Timeout = cc.config.Timeout
				cfg.Fingerprint = cc.getEndpointFingerprint(ep)
				fullClient, _ := NewClient(cfg)

				cc.mu.Lock()
				cc.nodeHealth[ep] = true
				delete(cc.lastError, ep)
				cc.lastHealthCheck[ep] = time.Now()
				cc.clients[ep] = fullClient
				cc.mu.Unlock()

				if vmSpecificErr {
					log.Info().
						Str("cluster", cc.name).
						Str("endpoint", ep).
						Msg("Recovered unhealthy cluster node (ignoring VM-specific errors)")
				} else {
					log.Info().
						Str("cluster", cc.name).
						Str("endpoint", ep).
						Msg("Recovered unhealthy cluster node")
				}
			} else {
				log.Debug().
					Str("cluster", cc.name).
					Str("endpoint", ep).
					Err(err).
					Msg("Recovery attempt failed - endpoint still unhealthy")
			}
		}(endpoint)
	}

	// Wait for all recovery attempts to complete
	go func() {
		wg.Wait()
		close(recoveredEndpoints)
	}()

	// Count recovered endpoints
	recoveredCount := 0
	for range recoveredEndpoints {
		recoveredCount++
	}

	// Log recovery summary
	if recoveredCount > 0 {
		log.Info().
			Str("cluster", cc.name).
			Int("recovered", recoveredCount).
			Int("attempted", len(unhealthyEndpoints)).
			Msg("Cluster endpoint recovery completed")
	} else if len(unhealthyEndpoints) > 0 {
		log.Warn().
			Str("cluster", cc.name).
			Int("attempted", len(unhealthyEndpoints)).
			Strs("failedEndpoints", unhealthyEndpoints).
			Msg("No endpoints recovered - cluster may be unreachable from Pulse server")
	}
}

// executeWithFailover executes a function with automatic failover
func (cc *ClusterClient) executeWithFailover(ctx context.Context, fn func(*Client) error) error {
	baseRetries := len(cc.endpoints)
	maxRetries := baseRetries + rateLimitRetryBudget
	var lastErr error

	log.Debug().
		Str("cluster", cc.name).
		Int("maxRetries", maxRetries).
		Msg("Starting executeWithFailover")

	for i := 0; i < maxRetries; i++ {
		client, err := cc.getHealthyClient(ctx)
		if err != nil {
			log.Debug().
				Str("cluster", cc.name).
				Err(err).
				Int("attempt", i+1).
				Msg("Failed to get healthy client")
			return err
		}

		// Get the endpoint for this client
		var clientEndpoint string
		cc.mu.RLock()
		for endpoint, c := range cc.clients {
			if c == client {
				clientEndpoint = endpoint
				break
			}
		}
		cc.mu.RUnlock()

		// Execute the function
		err = fn(client)
		if err == nil {
			// Clear any stale error for this endpoint on success
			cc.clearEndpointError(clientEndpoint)
			return nil
		}
		lastErr = err

		// Rate limit - retry with backoff (must check first)
		if isRateLimited, statusCode := isTransientRateLimitError(err); isRateLimited {
			backoff := calculateRateLimitBackoff(i)
			cc.applyRateLimitCooldown(clientEndpoint, backoff)

			event := log.Warn().
				Str("cluster", cc.name).
				Str("endpoint", clientEndpoint).
				Err(err).
				Dur("backoff", backoff).
				Int("attempt", i+1)
			if statusCode != 0 {
				event = event.Int("status", statusCode)
			}
			event.Msg("Rate limited by cluster node, retrying with backoff")

			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return fmt.Errorf("context canceled while backing off after rate limit: %w", ctx.Err())
			case <-timer.C:
			}

			continue
		}

		// Auth errors - return immediately without retry
		if isAuthError(err) {
			return err
		}

		// Only mark endpoint unhealthy for actual connectivity failures
		// (connection refused, TLS errors, DNS failures, etc.).
		// Any HTTP response from Proxmox - even a 500 - means the endpoint
		// is reachable and the request just failed at the application level.
		if isEndpointConnectivityError(err) {
			cc.markUnhealthyWithError(clientEndpoint, err.Error())
			log.Warn().
				Str("cluster", cc.name).
				Str("endpoint", clientEndpoint).
				Err(err).
				Int("attempt", i+1).
				Msg("Connectivity error, trying next cluster endpoint")
			continue
		}

		// Endpoint is reachable but the specific request failed
		// (HTTP 4xx/5xx, parsing error, VM-specific error, etc.)
		log.Debug().
			Str("cluster", cc.name).
			Str("endpoint", clientEndpoint).
			Err(err).
			Msg("Request-level error, endpoint reachable - not marking unhealthy")
		return err
	}

	if lastErr != nil {
		return fmt.Errorf("all cluster nodes failed for %s: %w", cc.name, lastErr)
	}

	return fmt.Errorf("all cluster nodes failed for %s", cc.name)
}

func (cc *ClusterClient) applyRateLimitCooldown(endpoint string, backoff time.Duration) {
	if endpoint == "" {
		return
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.rateLimitUntil == nil {
		cc.rateLimitUntil = make(map[string]time.Time)
	}
	cc.rateLimitUntil[endpoint] = time.Now().Add(backoff)
}

func calculateRateLimitBackoff(attempt int) time.Duration {
	// Linear backoff with jitter keeps retries gentle while avoiding thundering herd
	base := rateLimitBaseDelay * time.Duration(attempt+1)
	if rateLimitMaxJitter <= 0 {
		return base
	}

	jitter := time.Duration(rand.Int63n(rateLimitMaxJitter.Nanoseconds()+1)) * time.Nanosecond
	return base + jitter
}

func isTransientRateLimitError(err error) (bool, int) {
	if err == nil {
		return false, 0
	}

	errStr := err.Error()
	statusCode := extractStatusCode(errStr)
	if statusCode != 0 {
		if _, ok := transientRateLimitStatusCodes[statusCode]; ok {
			return true, statusCode
		}
	}

	lowerErr := strings.ToLower(errStr)
	if strings.Contains(lowerErr, "rate limit") || strings.Contains(lowerErr, "too many requests") {
		if statusCode == 0 {
			statusCode = 429
		}
		return true, statusCode
	}

	return false, statusCode
}

func extractStatusCode(errStr string) int {
	matches := statusCodePattern.FindStringSubmatch(errStr)
	if len(matches) != 2 {
		return 0
	}

	code, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return code
}

func isNotImplementedError(errStr string) bool {
	lower := strings.ToLower(errStr)
	if !strings.Contains(lower, "not implemented") {
		return false
	}

	// Common formatting: "status 501", "error 501", "api error 501"
	if strings.Contains(lower, " 501") || strings.Contains(lower, "status 501") || strings.Contains(lower, "error 501") {
		return true
	}

	// Fallback to explicit HTTP status detection
	if extractStatusCode(errStr) == 501 {
		return true
	}

	return false
}

// GetHealthStatus returns the health status of all nodes
func (cc *ClusterClient) GetHealthStatus() map[string]bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	status := make(map[string]bool)
	for endpoint, healthy := range cc.nodeHealth {
		status[endpoint] = healthy
	}
	return status
}

// EndpointHealth contains health information for a single endpoint
type EndpointHealth struct {
	Healthy   bool
	LastCheck time.Time
	LastError string
}

// GetHealthStatusWithErrors returns detailed health status including error messages
func (cc *ClusterClient) GetHealthStatusWithErrors() map[string]EndpointHealth {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	status := make(map[string]EndpointHealth)
	for endpoint, healthy := range cc.nodeHealth {
		status[endpoint] = EndpointHealth{
			Healthy:   healthy,
			LastCheck: cc.lastHealthCheck[endpoint],
			LastError: cc.lastError[endpoint],
		}
	}
	return status
}

// Implement all the Client methods with failover

func (cc *ClusterClient) GetNodes(ctx context.Context) ([]Node, error) {
	log.Debug().
		Str("cluster", cc.name).
		Msg("ClusterClient.GetNodes called")

	var result []Node
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		nodes, err := client.GetNodes(ctx)
		if err != nil {
			return err
		}
		result = nodes
		return nil
	})

	if err != nil {
		log.Warn().
			Str("cluster", cc.name).
			Err(err).
			Msg("ClusterClient.GetNodes failed")
	} else {
		log.Info().
			Str("cluster", cc.name).
			Int("count", len(result)).
			Msg("ClusterClient.GetNodes succeeded")
	}

	return result, err
}

func (cc *ClusterClient) GetNodeStatus(ctx context.Context, node string) (*NodeStatus, error) {
	var result *NodeStatus
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		status, err := client.GetNodeStatus(ctx, node)
		if err != nil {
			return err
		}
		result = status
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetNodeRRDData(ctx context.Context, node, timeframe, cf string, ds []string) ([]NodeRRDPoint, error) {
	var result []NodeRRDPoint
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		points, err := client.GetNodeRRDData(ctx, node, timeframe, cf, ds)
		if err != nil {
			return err
		}
		result = points
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]GuestRRDPoint, error) {
	var result []GuestRRDPoint
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		points, err := client.GetLXCRRDData(ctx, node, vmid, timeframe, cf, ds)
		if err != nil {
			return err
		}
		result = points
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetVMs(ctx context.Context, node string) ([]VM, error) {
	var result []VM
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		vms, err := client.GetVMs(ctx, node)
		if err != nil {
			return err
		}
		result = vms
		return nil
	})

	// Don't return error for transient connectivity issues - preserve UI state
	if err != nil && strings.Contains(err.Error(), "no healthy nodes available") {
		log.Debug().
			Str("cluster", cc.name).
			Str("node", node).
			Err(err).
			Msg("No healthy nodes for GetVMs - returning empty list to preserve UI state")
		return []VM{}, nil
	}

	return result, err
}

func (cc *ClusterClient) GetContainers(ctx context.Context, node string) ([]Container, error) {
	var result []Container
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		containers, err := client.GetContainers(ctx, node)
		if err != nil {
			return err
		}
		result = containers
		return nil
	})

	// Don't return error for transient connectivity issues - preserve UI state
	if err != nil && strings.Contains(err.Error(), "no healthy nodes available") {
		log.Debug().
			Str("cluster", cc.name).
			Str("node", node).
			Err(err).
			Msg("No healthy nodes for GetContainers - returning empty list to preserve UI state")
		return []Container{}, nil
	}

	return result, err
}

func (cc *ClusterClient) GetStorage(ctx context.Context, node string) ([]Storage, error) {
	var result []Storage
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		storage, err := client.GetStorage(ctx, node)
		if err != nil {
			return err
		}
		result = storage
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetAllStorage(ctx context.Context) ([]Storage, error) {
	var result []Storage
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		storage, err := client.GetAllStorage(ctx)
		if err != nil {
			return err
		}
		result = storage
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetBackupTasks(ctx context.Context) ([]Task, error) {
	var result []Task
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		tasks, err := client.GetBackupTasks(ctx)
		if err != nil {
			return err
		}
		result = tasks
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetReplicationStatus(ctx context.Context) ([]ReplicationJob, error) {
	var result []ReplicationJob
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		jobs, err := client.GetReplicationStatus(ctx)
		if err != nil {
			return err
		}
		result = jobs
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetStorageContent(ctx context.Context, node, storage string) ([]StorageContent, error) {
	var result []StorageContent
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		content, err := client.GetStorageContent(ctx, node, storage)
		if err != nil {
			return err
		}
		result = content
		return nil
	})
	return result, err
}

// GetCephStatus returns Ceph cluster status information with failover support.
func (cc *ClusterClient) GetCephStatus(ctx context.Context) (*CephStatus, error) {
	var result *CephStatus
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		status, err := client.GetCephStatus(ctx)
		if err != nil {
			return err
		}
		result = status
		return nil
	})
	return result, err
}

// GetCephDF returns Ceph capacity information with failover support.
func (cc *ClusterClient) GetCephDF(ctx context.Context) (*CephDF, error) {
	var result *CephDF
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		df, err := client.GetCephDF(ctx)
		if err != nil {
			return err
		}
		result = df
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]Snapshot, error) {
	var result []Snapshot
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		snapshots, err := client.GetVMSnapshots(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = snapshots
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]Snapshot, error) {
	var result []Snapshot
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		snapshots, err := client.GetContainerSnapshots(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = snapshots
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*VMStatus, error) {
	var result *VMStatus
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		status, err := client.GetVMStatus(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = status
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		config, err := client.GetVMConfig(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = config
		return nil
	})
	return result, err
}

func (cc *ClusterClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		info, err := client.GetVMAgentInfo(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = info
		return nil
	})
	return result, err
}

// GetVMAgentVersion returns the guest agent version for the VM.
func (cc *ClusterClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	var version string
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		v, err := client.GetVMAgentVersion(ctx, node, vmid)
		if err != nil {
			return err
		}
		version = v
		return nil
	})
	return version, err
}

// GetVMFSInfo returns filesystem information from QEMU guest agent
func (cc *ClusterClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]VMFileSystem, error) {
	var result []VMFileSystem
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		info, err := client.GetVMFSInfo(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = info
		return nil
	})
	return result, err
}

// GetVMNetworkInterfaces returns guest network interfaces from the QEMU agent
func (cc *ClusterClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]VMNetworkInterface, error) {
	var result []VMNetworkInterface
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		interfaces, err := client.GetVMNetworkInterfaces(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = interfaces
		return nil
	})
	return result, err
}

// GetClusterResources returns all resources (VMs, containers) across the cluster in a single call
func (cc *ClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]ClusterResource, error) {
	var result []ClusterResource
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		resources, err := client.GetClusterResources(ctx, resourceType)
		if err != nil {
			return err
		}
		result = resources
		return nil
	})
	return result, err
}

// GetContainerStatus returns the status of a specific container
func (cc *ClusterClient) GetContainerStatus(ctx context.Context, node string, vmid int) (*Container, error) {
	var result *Container
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		status, err := client.GetContainerStatus(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = status
		return nil
	})
	return result, err
}

// GetContainerConfig returns the configuration of a specific container
func (cc *ClusterClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		config, err := client.GetContainerConfig(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = config
		return nil
	})
	return result, err
}

// GetContainerInterfaces returns interface details for a container
func (cc *ClusterClient) GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]ContainerInterface, error) {
	var result []ContainerInterface
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		interfaces, err := client.GetContainerInterfaces(ctx, node, vmid)
		if err != nil {
			return err
		}
		result = interfaces
		return nil
	})
	return result, err
}

// IsClusterMember checks if this node is part of a cluster
func (cc *ClusterClient) IsClusterMember(ctx context.Context) (bool, error) {
	var result bool
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		isMember, err := client.IsClusterMember(ctx)
		if err != nil {
			return err
		}
		result = isMember
		return nil
	})
	return result, err
}

// GetZFSPoolStatus returns ZFS pool status for a node
func (cc *ClusterClient) GetZFSPoolStatus(ctx context.Context, node string) ([]ZFSPoolStatus, error) {
	var result []ZFSPoolStatus
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		pools, err := client.GetZFSPoolStatus(ctx, node)
		if err != nil {
			return err
		}
		result = pools
		return nil
	})
	return result, err
}

// GetZFSPoolsWithDetails returns ZFS pools with full details for a node
func (cc *ClusterClient) GetZFSPoolsWithDetails(ctx context.Context, node string) ([]ZFSPoolInfo, error) {
	var result []ZFSPoolInfo
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		pools, err := client.GetZFSPoolsWithDetails(ctx, node)
		if err != nil {
			return err
		}
		result = pools
		return nil
	})
	return result, err
}

// Helper to check if error is auth-related
func (cc *ClusterClient) GetDisks(ctx context.Context, node string) ([]Disk, error) {
	var result []Disk
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		disks, err := client.GetDisks(ctx, node)
		if err != nil {
			return err
		}
		result = disks
		return nil
	})

	// Don't return error for transient connectivity issues
	if err != nil && strings.Contains(err.Error(), "no healthy nodes available") {
		log.Debug().
			Str("cluster", cc.name).
			Str("node", node).
			Err(err).
			Msg("No healthy nodes for GetDisks - returning empty list")
		return []Disk{}, nil
	}

	return result, err
}

// GetNodePendingUpdates returns pending apt updates for a node with failover support
func (cc *ClusterClient) GetNodePendingUpdates(ctx context.Context, node string) ([]AptPackage, error) {
	var result []AptPackage
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		pkgs, err := client.GetNodePendingUpdates(ctx, node)
		if err != nil {
			return err
		}
		result = pkgs
		return nil
	})

	// Don't return error for transient connectivity issues or permission issues
	if err != nil && (strings.Contains(err.Error(), "no healthy nodes available") ||
		strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "permission")) {
		log.Debug().
			Str("cluster", cc.name).
			Str("node", node).
			Err(err).
			Msg("Could not get pending updates - returning empty list")
		return []AptPackage{}, nil
	}

	return result, err
}

// GetClusterStatus returns the cluster status including all nodes with failover support.
func (cc *ClusterClient) GetClusterStatus(ctx context.Context) ([]ClusterStatus, error) {
	var result []ClusterStatus
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		status, err := client.GetClusterStatus(ctx)
		if err != nil {
			return err
		}
		result = status
		return nil
	})

	return result, err
}

// IsQuorate checks if the cluster has quorum by querying the Proxmox cluster status.
// Returns true if the cluster is quorate (has enough votes for consensus), false otherwise.
// This is the authoritative check for cluster health - a cluster with quorum is healthy
// even if some nodes are intentionally offline (e.g., backup nodes not running).
func (cc *ClusterClient) IsQuorate(ctx context.Context) (bool, error) {
	status, err := cc.GetClusterStatus(ctx)
	if err != nil {
		return false, err
	}

	// Look for the cluster entry which has the quorate field
	for _, s := range status {
		if s.Type == "cluster" {
			return s.Quorate == 1, nil
		}
	}

	// If no cluster entry found, this might be a standalone node - consider it healthy
	return true, nil
}

// isAuthError checks if an error is an authentication error
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403")
}
