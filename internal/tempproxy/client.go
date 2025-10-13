package tempproxy

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultSocketPath = "/run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
	defaultTimeout    = 10 * time.Second
)

// Client communicates with pulse-sensor-proxy via unix socket
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new proxy client
func NewClient() *Client {
	socketPath := os.Getenv("PULSE_SENSOR_PROXY_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	return &Client{
		socketPath: socketPath,
		timeout:    defaultTimeout,
	}
}

// IsAvailable checks if the proxy is running and accessible
func (c *Client) IsAvailable() bool {
	_, err := os.Stat(c.socketPath)
	return err == nil
}

// RPCRequest represents a request to the proxy
type RPCRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// RPCResponse represents a response from the proxy
type RPCResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// call sends an RPC request and returns the response
func (c *Client) call(method string, params map[string]interface{}) (*RPCResponse, error) {
	// Connect to unix socket
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}
	defer conn.Close()

	// Set deadline
	conn.SetDeadline(time.Now().Add(c.timeout))

	// Send request
	req := RPCRequest{
		Method: method,
		Params: params,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Read response
	var resp RPCResponse
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

// GetStatus returns proxy status
func (c *Client) GetStatus() (map[string]interface{}, error) {
	resp, err := c.call("get_status", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("proxy error: %s", resp.Error)
	}

	return resp.Data, nil
}

// EnsureClusterKeys discovers cluster nodes and pushes SSH keys
func (c *Client) EnsureClusterKeys() (map[string]interface{}, error) {
	log.Info().Msg("Requesting proxy to configure cluster SSH keys")

	resp, err := c.call("ensure_cluster_keys", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("proxy error: %s", resp.Error)
	}

	return resp.Data, nil
}

// RegisterNodes returns list of discovered nodes with SSH status
func (c *Client) RegisterNodes() ([]map[string]interface{}, error) {
	resp, err := c.call("register_nodes", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("proxy error: %s", resp.Error)
	}

	// Extract nodes array from data
	nodesRaw, ok := resp.Data["nodes"]
	if !ok {
		return nil, fmt.Errorf("no nodes in response")
	}

	// Type assertion to []interface{} first, then convert
	nodesArray, ok := nodesRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("nodes is not an array")
	}

	nodes := make([]map[string]interface{}, len(nodesArray))
	for i, nodeRaw := range nodesArray {
		node, ok := nodeRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("node %d is not a map", i)
		}
		nodes[i] = node
	}

	return nodes, nil
}

// GetTemperature fetches temperature data from a specific node
func (c *Client) GetTemperature(nodeHost string) (string, error) {
	params := map[string]interface{}{
		"node": nodeHost,
	}

	resp, err := c.call("get_temperature", params)
	if err != nil {
		return "", err
	}

	if !resp.Success {
		return "", fmt.Errorf("proxy error: %s", resp.Error)
	}

	// Extract temperature JSON string
	tempRaw, ok := resp.Data["temperature"]
	if !ok {
		return "", fmt.Errorf("no temperature data in response")
	}

	tempStr, ok := tempRaw.(string)
	if !ok {
		return "", fmt.Errorf("temperature is not a string")
	}

	return tempStr, nil
}
