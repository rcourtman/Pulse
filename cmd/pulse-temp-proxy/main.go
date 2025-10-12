package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// Version information (set at build time with -ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

const (
	defaultSocketPath = "/var/run/pulse-temp-proxy.sock"
	defaultSSHKeyPath = "/var/lib/pulse-temp-proxy/ssh"
)

var rootCmd = &cobra.Command{
	Use:     "pulse-temp-proxy",
	Short:   "Pulse Temperature Proxy - Secure SSH bridge for containerized Pulse",
	Long:    `Temperature monitoring proxy that keeps SSH keys on the host and exposes temperature data via unix socket`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		runProxy()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("pulse-temp-proxy %s\n", Version)
		if BuildTime != "unknown" {
			fmt.Printf("Built: %s\n", BuildTime)
		}
		if GitCommit != "unknown" {
			fmt.Printf("Commit: %s\n", GitCommit)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// Proxy manages the temperature monitoring proxy
type Proxy struct {
	socketPath string
	sshKeyPath string
	listener   net.Listener
}

// RPC request types
const (
	RPCEnsureClusterKeys = "ensure_cluster_keys"
	RPCRegisterNodes     = "register_nodes"
	RPCGetTemperature    = "get_temperature"
	RPCGetStatus         = "get_status"
)

// RPCRequest represents a request from Pulse
type RPCRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// RPCResponse represents a response to Pulse
type RPCResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func runProxy() {
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	socketPath := os.Getenv("PULSE_TEMP_PROXY_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	sshKeyPath := os.Getenv("PULSE_TEMP_PROXY_SSH_DIR")
	if sshKeyPath == "" {
		sshKeyPath = defaultSSHKeyPath
	}

	log.Info().
		Str("socket", socketPath).
		Str("ssh_key_dir", sshKeyPath).
		Msg("Starting pulse-temp-proxy")

	proxy := &Proxy{
		socketPath: socketPath,
		sshKeyPath: sshKeyPath,
	}

	if err := proxy.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start proxy")
	}

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Info().Msg("Shutting down proxy...")
	proxy.Stop()
	log.Info().Msg("Proxy stopped")
}

// Start initializes and starts the proxy
func (p *Proxy) Start() error {
	// Create SSH key directory if it doesn't exist
	if err := os.MkdirAll(p.sshKeyPath, 0700); err != nil {
		return fmt.Errorf("failed to create SSH key directory: %w", err)
	}

	// Ensure SSH keypair exists
	if err := p.ensureSSHKeypair(); err != nil {
		return fmt.Errorf("failed to ensure SSH keypair: %w", err)
	}

	// Remove existing socket if it exists
	if err := os.RemoveAll(p.socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create socket directory if needed
	socketDir := filepath.Dir(p.socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Create unix socket listener
	listener, err := net.Listen("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create unix socket: %w", err)
	}
	p.listener = listener

	// Set socket permissions so Pulse container can access it
	if err := os.Chmod(p.socketPath, 0666); err != nil {
		log.Warn().Err(err).Msg("Failed to set socket permissions")
	}

	log.Info().Str("socket", p.socketPath).Msg("Unix socket ready")

	// Start accepting connections
	go p.acceptConnections()

	return nil
}

// Stop shuts down the proxy
func (p *Proxy) Stop() {
	if p.listener != nil {
		p.listener.Close()
		os.Remove(p.socketPath)
	}
}

// acceptConnections handles incoming socket connections
func (p *Proxy) acceptConnections() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			// Check if listener was closed
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return
			}
			log.Error().Err(err).Msg("Failed to accept connection")
			continue
		}

		go p.handleConnection(conn)
	}
}

// handleConnection processes a single RPC request
func (p *Proxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Decode request
	var req RPCRequest
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode RPC request")
		p.sendError(conn, "invalid request format")
		return
	}

	log.Debug().Str("method", req.Method).Msg("Received RPC request")

	// Route to handler
	var resp RPCResponse
	switch req.Method {
	case RPCGetStatus:
		resp = p.handleGetStatus(req)
	case RPCEnsureClusterKeys:
		resp = p.handleEnsureClusterKeys(req)
	case RPCRegisterNodes:
		resp = p.handleRegisterNodes(req)
	case RPCGetTemperature:
		resp = p.handleGetTemperature(req)
	default:
		resp = RPCResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown method: %s", req.Method),
		}
	}

	// Send response
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(resp); err != nil {
		log.Error().Err(err).Msg("Failed to encode RPC response")
	}
}

// sendError sends an error response
func (p *Proxy) sendError(conn net.Conn, message string) {
	resp := RPCResponse{
		Success: false,
		Error:   message,
	}
	encoder := json.NewEncoder(conn)
	encoder.Encode(resp)
}

// handleGetStatus returns proxy status
func (p *Proxy) handleGetStatus(req RPCRequest) RPCResponse {
	pubKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519.pub")
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return RPCResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read public key: %v", err),
		}
	}

	return RPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"version":    Version,
			"public_key": string(pubKey),
			"ssh_dir":    p.sshKeyPath,
		},
	}
}

// ensureSSHKeypair generates SSH keypair if it doesn't exist
func (p *Proxy) ensureSSHKeypair() error {
	privKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519")
	pubKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519.pub")

	// Check if keypair already exists
	if _, err := os.Stat(privKeyPath); err == nil {
		if _, err := os.Stat(pubKeyPath); err == nil {
			log.Info().Msg("SSH keypair already exists")
			return nil
		}
	}

	log.Info().Msg("Generating new SSH keypair")

	// Generate ed25519 keypair using ssh-keygen
	cmd := fmt.Sprintf("ssh-keygen -t ed25519 -f %s -N '' -C 'pulse-temp-proxy'", privKeyPath)
	if output, err := execCommand(cmd); err != nil {
		return fmt.Errorf("failed to generate SSH keypair: %w (output: %s)", err, output)
	}

	log.Info().Str("path", privKeyPath).Msg("SSH keypair generated")
	return nil
}

// handleEnsureClusterKeys discovers cluster nodes and pushes SSH keys
func (p *Proxy) handleEnsureClusterKeys(req RPCRequest) RPCResponse {
	// Check if we're on a Proxmox host
	if !isProxmoxHost() {
		return RPCResponse{
			Success: false,
			Error:   "not running on Proxmox host - cannot discover cluster",
		}
	}

	// Discover cluster nodes
	nodes, err := discoverClusterNodes()
	if err != nil {
		return RPCResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to discover cluster: %v", err),
		}
	}

	log.Info().Strs("nodes", nodes).Msg("Discovered cluster nodes")

	// Push SSH key to each node
	results := make(map[string]interface{})
	successCount := 0
	for _, node := range nodes {
		log.Info().Str("node", node).Msg("Pushing SSH key to node")
		if err := p.pushSSHKey(node); err != nil {
			log.Error().Err(err).Str("node", node).Msg("Failed to push SSH key")
			results[node] = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
		} else {
			log.Info().Str("node", node).Msg("SSH key pushed successfully")
			results[node] = map[string]interface{}{
				"success": true,
			}
			successCount++
		}
	}

	return RPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"nodes":         nodes,
			"results":       results,
			"success_count": successCount,
			"total_count":   len(nodes),
		},
	}
}

// handleRegisterNodes returns discovered nodes
func (p *Proxy) handleRegisterNodes(req RPCRequest) RPCResponse {
	// Check if we're on a Proxmox host
	if !isProxmoxHost() {
		return RPCResponse{
			Success: false,
			Error:   "not running on Proxmox host",
		}
	}

	// Discover cluster nodes
	nodes, err := discoverClusterNodes()
	if err != nil {
		return RPCResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to discover nodes: %v", err),
		}
	}

	// Test SSH connectivity to each node
	nodeStatus := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		status := map[string]interface{}{
			"name": node,
		}

		if err := p.testSSHConnection(node); err != nil {
			status["ssh_ready"] = false
			status["error"] = err.Error()
		} else {
			status["ssh_ready"] = true
		}

		nodeStatus = append(nodeStatus, status)
	}

	return RPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"nodes": nodeStatus,
		},
	}
}

// handleGetTemperature fetches temperature data from a node via SSH
func (p *Proxy) handleGetTemperature(req RPCRequest) RPCResponse {
	// Extract node parameter
	nodeParam, ok := req.Params["node"]
	if !ok {
		return RPCResponse{
			Success: false,
			Error:   "missing 'node' parameter",
		}
	}

	node, ok := nodeParam.(string)
	if !ok {
		return RPCResponse{
			Success: false,
			Error:   "'node' parameter must be a string",
		}
	}

	// Fetch temperature data
	tempData, err := p.getTemperatureViaSSH(node)
	if err != nil {
		return RPCResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get temperatures: %v", err),
		}
	}

	return RPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"node":        node,
			"temperature": tempData,
		},
	}
}
