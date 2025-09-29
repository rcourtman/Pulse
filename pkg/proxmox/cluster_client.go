package proxmox

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// ClusterClient wraps multiple Proxmox clients for cluster-aware operations
type ClusterClient struct {
	mu              sync.RWMutex
	name            string
	clients         map[string]*Client   // Key is node name
	endpoints       []string             // All available endpoints
	nodeHealth      map[string]bool      // Track node health
	lastHealthCheck map[string]time.Time // Track last health check time
	lastUsedIndex   int                  // For round-robin
	config          ClientConfig         // Base config (auth info)
}

// NewClusterClient creates a new cluster-aware client
func NewClusterClient(name string, config ClientConfig, endpoints []string) *ClusterClient {
	cc := &ClusterClient{
		name:            name,
		clients:         make(map[string]*Client),
		endpoints:       endpoints,
		nodeHealth:      make(map[string]bool),
		lastHealthCheck: make(map[string]time.Time),
		config:          config,
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
			cfg.Timeout = 5 * time.Second

			testClient, err := NewClient(cfg)
			if err != nil {
				cc.mu.Lock()
				cc.nodeHealth[ep] = false
				cc.lastHealthCheck[ep] = time.Now()
				cc.mu.Unlock()
				log.Info().
					Str("cluster", cc.name).
					Str("endpoint", ep).
					Msg("Cluster endpoint marked unhealthy on initialization")
				return
			}

			// Quick test with slightly longer timeout for initial check
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err = testClient.GetNodes(ctx)
			cancel()

			cc.mu.Lock()

			// Check if error is VM-specific (shouldn't affect health)
			isVMSpecificError := false
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "No QEMU guest agent") ||
					strings.Contains(errStr, "QEMU guest agent is not running") ||
					strings.Contains(errStr, "guest agent") {
					isVMSpecificError = true
				}
			}

			if err == nil || isVMSpecificError {
				// Node is healthy - create a proper client with full timeout for actual use
				fullCfg := cc.config
				fullCfg.Host = ep
				fullClient, clientErr := NewClient(fullCfg)
				if clientErr != nil {
					cc.nodeHealth[ep] = false
					log.Warn().
						Str("cluster", cc.name).
						Str("endpoint", ep).
						Err(clientErr).
						Msg("Failed to create full client after successful health check")
				} else {
					cc.nodeHealth[ep] = true
					cc.clients[ep] = fullClient // Store the full client, not test client
					if isVMSpecificError {
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
				cc.nodeHealth[ep] = false
				log.Info().
					Str("cluster", cc.name).
					Str("endpoint", ep).
					Err(err).
					Msg("Cluster endpoint failed initial health check")
			}
			cc.lastHealthCheck[ep] = time.Now()
			cc.mu.Unlock()
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
	for endpoint, healthy := range cc.nodeHealth {
		if healthy {
			healthyEndpoints = append(healthyEndpoints, endpoint)
		}
	}

	log.Debug().
		Str("cluster", cc.name).
		Int("healthy", len(healthyEndpoints)).
		Int("total", len(cc.nodeHealth)).
		Interface("nodeHealth", cc.nodeHealth).
		Msg("Checking for healthy endpoints")

	if len(healthyEndpoints) == 0 {
		// Try to recover by testing all endpoints
		cc.mu.Unlock()
		cc.recoverUnhealthyNodes(ctx)
		cc.mu.Lock()

		// Check again
		for endpoint, healthy := range cc.nodeHealth {
			if healthy {
				healthyEndpoints = append(healthyEndpoints, endpoint)
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
				return nil, fmt.Errorf("no healthy nodes available in cluster %s", cc.name)
			}
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

		// Quick connectivity test
		testCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		testNodes, testErr := testClient.GetNodes(testCtx)
		cancel()

		if testErr != nil {
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
					Msg("Cluster endpoint failed connectivity test")
				return nil, fmt.Errorf("endpoint %s failed connectivity test: %w", selectedEndpoint, testErr)
			}
		}

		log.Debug().
			Str("cluster", cc.name).
			Str("endpoint", selectedEndpoint).
			Int("nodes", len(testNodes)).
			Msg("Cluster endpoint passed connectivity test")

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

// markUnhealthy marks an endpoint as unhealthy
func (cc *ClusterClient) markUnhealthy(endpoint string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.nodeHealth[endpoint] {
		log.Warn().
			Str("cluster", cc.name).
			Str("endpoint", endpoint).
			Msg("Marking cluster node as unhealthy")
		cc.nodeHealth[endpoint] = false
	}
}

// recoverUnhealthyNodes attempts to recover unhealthy nodes
func (cc *ClusterClient) recoverUnhealthyNodes(ctx context.Context) {
	cc.mu.RLock()
	unhealthyEndpoints := make([]string, 0)
	now := time.Now()
	for endpoint, healthy := range cc.nodeHealth {
		if !healthy {
			// Skip if we checked this endpoint recently (within 10 seconds)
			// Balance between recovery speed and avoiding excessive checks
			if lastCheck, exists := cc.lastHealthCheck[endpoint]; exists {
				if now.Sub(lastCheck) < 10*time.Second {
					continue
				}
			}
			unhealthyEndpoints = append(unhealthyEndpoints, endpoint)
		}
	}
	cc.mu.RUnlock()

	if len(unhealthyEndpoints) == 0 {
		return
	}

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

			// Try to create a client and test connection with shorter timeout
			cfg := cc.config
			cfg.Host = ep
			cfg.Timeout = 2 * time.Second // Use shorter timeout for recovery attempts

			testClient, err := NewClient(cfg)
			if err == nil {
				// Try a simple API call with short timeout
				testCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				_, err = testClient.GetNodes(testCtx)
				cancel()

				// Check if error is VM-specific (shouldn't prevent recovery)
				isVMSpecificError := false
				if err != nil {
					errStr := err.Error()
					if strings.Contains(errStr, "No QEMU guest agent") ||
						strings.Contains(errStr, "QEMU guest agent is not running") ||
						strings.Contains(errStr, "guest agent") {
						isVMSpecificError = true
					}
				}

				if err == nil || isVMSpecificError {
					recoveredEndpoints <- ep

					// Store the client with original timeout
					cfg.Timeout = cc.config.Timeout
					fullClient, _ := NewClient(cfg)

					cc.mu.Lock()
					cc.nodeHealth[ep] = true
					cc.clients[ep] = fullClient
					cc.mu.Unlock()

					if isVMSpecificError {
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
				}
			}
		}(endpoint)
	}

	// Wait for all recovery attempts to complete
	go func() {
		wg.Wait()
		close(recoveredEndpoints)
	}()

	// Process recovered endpoints (just for logging, actual recovery happens above)
	for range recoveredEndpoints {
		// Endpoints are already marked healthy in the goroutine
	}
}

// executeWithFailover executes a function with automatic failover
func (cc *ClusterClient) executeWithFailover(ctx context.Context, fn func(*Client) error) error {
	maxRetries := len(cc.endpoints)

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
			return nil
		}

		// Check error type and content
		errStr := err.Error()

		// Check if it's a node-specific or transient failure that shouldn't mark endpoint unhealthy
		// Error 595 in Proxmox means "no ticket" but in cluster context often means target node unreachable
		// Error 500 with hostname lookup failure means a node reference issue, not endpoint failure
		// Error 403 for storage operations means permission issue, not node health issue
		// Error 500 with "No QEMU guest agent configured" means VM-specific issue, not node failure
		// Error 500 with "QEMU guest agent is not running" means VM-specific issue, not node failure
		// Error 500 with any "guest agent" message means VM-specific issue, not node failure
		// JSON unmarshal errors are data format issues, not connectivity problems
		if strings.Contains(errStr, "595") ||
			(strings.Contains(errStr, "500") && strings.Contains(errStr, "hostname lookup")) ||
			(strings.Contains(errStr, "500") && strings.Contains(errStr, "Name or service not known")) ||
			(strings.Contains(errStr, "500") && strings.Contains(errStr, "No QEMU guest agent configured")) ||
			(strings.Contains(errStr, "500") && strings.Contains(errStr, "QEMU guest agent is not running")) ||
			(strings.Contains(errStr, "500") && strings.Contains(errStr, "guest agent")) ||
			strings.Contains(errStr, "guest agent") ||
			(strings.Contains(errStr, "403") && (strings.Contains(errStr, "storage") || strings.Contains(errStr, "datastore"))) ||
			strings.Contains(errStr, "permission denied") ||
			strings.Contains(errStr, "json: cannot unmarshal") ||
			strings.Contains(errStr, "unexpected response format") {
			// This is likely a node-specific failure, not an endpoint failure
			// Return the error but don't mark the endpoint as unhealthy
			log.Debug().
				Str("cluster", cc.name).
				Str("endpoint", clientEndpoint).
				Err(err).
				Msg("Node-specific or configuration error, not marking endpoint unhealthy")
			return err
		}

		// Check if it's an auth error - don't retry on auth errors
		if IsAuthError(err) {
			return err
		}

		// Mark endpoint as unhealthy and try next
		cc.markUnhealthy(clientEndpoint)

		log.Warn().
			Str("cluster", cc.name).
			Str("endpoint", clientEndpoint).
			Err(err).
			Int("attempt", i+1).
			Msg("Failed on cluster node, trying next")
	}

	return fmt.Errorf("all cluster nodes failed for %s", cc.name)
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

// GetClusterHealthInfo returns detailed health information about the cluster
func (cc *ClusterClient) GetClusterHealthInfo() models.ClusterHealth {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	health := models.ClusterHealth{
		Name:         cc.name,
		TotalNodes:   len(cc.endpoints),
		OnlineNodes:  0,
		OfflineNodes: 0,
		NodeStatuses: make([]models.ClusterNodeStatus, 0, len(cc.endpoints)),
	}

	for endpoint, isHealthy := range cc.nodeHealth {
		status := models.ClusterNodeStatus{
			Endpoint: endpoint,
			Online:   isHealthy,
		}

		if isHealthy {
			health.OnlineNodes++
		} else {
			health.OfflineNodes++
		}

		health.NodeStatuses = append(health.NodeStatuses, status)
	}

	// Calculate overall health percentage
	if health.TotalNodes > 0 {
		health.HealthPercentage = float64(health.OnlineNodes) / float64(health.TotalNodes) * 100
	}

	return health
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

func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403")
}
