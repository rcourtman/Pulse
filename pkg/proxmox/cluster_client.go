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
	clients         map[string]*Client // Key is node name
	endpoints       []string           // All available endpoints
	nodeHealth      map[string]bool    // Track node health
	lastUsedIndex   int                // For round-robin
	config          ClientConfig       // Base config (auth info)
}

// NewClusterClient creates a new cluster-aware client
func NewClusterClient(name string, config ClientConfig, endpoints []string) *ClusterClient {
	cc := &ClusterClient{
		name:       name,
		clients:    make(map[string]*Client),
		endpoints:  endpoints,
		nodeHealth: make(map[string]bool),
		config:     config,
	}
	
	// Initialize all endpoints as healthy
	for _, endpoint := range endpoints {
		cc.nodeHealth[endpoint] = true
	}
	
	return cc
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
			return nil, fmt.Errorf("no healthy nodes available in cluster %s", cc.name)
		}
	}
	
	// Use random selection for better load distribution
	selectedEndpoint := healthyEndpoints[rand.Intn(len(healthyEndpoints))]
	
	// Get or create client for this endpoint
	client, exists := cc.clients[selectedEndpoint]
	if !exists {
		// Create new client
		cfg := cc.config
		cfg.Host = selectedEndpoint
		
		newClient, err := NewClient(cfg)
		if err != nil {
			// Mark as unhealthy
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
	for endpoint, healthy := range cc.nodeHealth {
		if !healthy {
			unhealthyEndpoints = append(unhealthyEndpoints, endpoint)
		}
	}
	cc.mu.RUnlock()
	
	for _, endpoint := range unhealthyEndpoints {
		// Try to create a client and test connection
		cfg := cc.config
		cfg.Host = endpoint
		
		testClient, err := NewClient(cfg)
		if err == nil {
			// Try a simple API call
			testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err = testClient.GetNodes(testCtx)
			cancel()
			
			if err == nil {
				cc.mu.Lock()
				cc.nodeHealth[endpoint] = true
				cc.clients[endpoint] = testClient
				cc.mu.Unlock()
				
				log.Info().
					Str("cluster", cc.name).
					Str("endpoint", endpoint).
					Msg("Recovered unhealthy cluster node")
			}
		}
	}
}

// executeWithFailover executes a function with automatic failover
func (cc *ClusterClient) executeWithFailover(ctx context.Context, fn func(*Client) error) error {
	maxRetries := len(cc.endpoints)
	
	for i := 0; i < maxRetries; i++ {
		client, err := cc.getHealthyClient(ctx)
		if err != nil {
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
		
		// Check if it's an auth error - don't retry on auth errors
		if IsAuthError(err) {
			return err
		}
		
		// Mark endpoint as unhealthy and try next
		cc.markUnhealthy(clientEndpoint)
		
		log.Debug().
			Str("cluster", cc.name).
			Str("endpoint", clientEndpoint).
			Err(err).
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
	var result []Node
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		nodes, err := client.GetNodes(ctx)
		if err != nil {
			return err
		}
		result = nodes
		return nil
	})
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

// GetClusterHealthInfo returns detailed health information about the cluster
func (cc *ClusterClient) GetClusterHealthInfo() models.ClusterHealth {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	health := models.ClusterHealth{
		Name:           cc.name,
		TotalNodes:     len(cc.endpoints),
		OnlineNodes:    0,
		OfflineNodes:   0,
		NodeStatuses:   make([]models.ClusterNodeStatus, 0, len(cc.endpoints)),
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
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "authentication") || 
		strings.Contains(errStr, "401") || 
		strings.Contains(errStr, "403")
}