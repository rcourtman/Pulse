package api

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rs/zerolog/log"
)

// DiagnosticsInfo contains comprehensive diagnostic information
type DiagnosticsInfo struct {
	Version     string                    `json:"version"`
	Runtime     string                    `json:"runtime"`
	Uptime      float64                  `json:"uptime"`
	Nodes       []NodeDiagnostic         `json:"nodes"`
	PBS         []PBSDiagnostic          `json:"pbs"`
	System      SystemDiagnostic         `json:"system"`
	Errors      []string                 `json:"errors"`
}

// NodeDiagnostic contains diagnostic info for a Proxmox node
type NodeDiagnostic struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	Type         string            `json:"type"`
	AuthMethod   string            `json:"authMethod"`
	Connected    bool              `json:"connected"`
	Error        string            `json:"error,omitempty"`
	Details      *NodeDetails      `json:"details,omitempty"`
	LastPoll     string            `json:"lastPoll,omitempty"`
	ClusterInfo  *ClusterInfo      `json:"clusterInfo,omitempty"`
}

// NodeDetails contains node-specific details
type NodeDetails struct {
	NodeCount int `json:"node_count,omitempty"`
}

// ClusterInfo contains cluster information
type ClusterInfo struct {
	Nodes int `json:"nodes"`
}

// PBSDiagnostic contains diagnostic info for a PBS instance
type PBSDiagnostic struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Host       string        `json:"host"`
	Connected  bool          `json:"connected"`
	Error      string        `json:"error,omitempty"`
	Details    *PBSDetails   `json:"details,omitempty"`
}

// PBSDetails contains PBS-specific details
type PBSDetails struct {
	Version string `json:"version,omitempty"`
}

// SystemDiagnostic contains system-level diagnostic info
type SystemDiagnostic struct {
	OS           string  `json:"os"`
	Arch         string  `json:"arch"`
	GoVersion    string  `json:"goVersion"`
	NumCPU       int     `json:"numCPU"`
	NumGoroutine int     `json:"numGoroutine"`
	MemoryMB     uint64  `json:"memoryMB"`
}

// handleDiagnostics returns comprehensive diagnostic information
func (r *Router) handleDiagnostics(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	diag := DiagnosticsInfo{
		Errors: []string{},
	}

	// Version info
	if versionInfo, err := updates.GetCurrentVersion(); err == nil {
		diag.Version = versionInfo.Version
		diag.Runtime = versionInfo.Runtime
	} else {
		diag.Version = "unknown"
		diag.Runtime = "go"
	}

	// Uptime
	diag.Uptime = time.Since(r.monitor.GetStartTime()).Seconds()

	// System info
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	diag.System = SystemDiagnostic{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		MemoryMB:     memStats.Alloc / 1024 / 1024,
	}

	// Test each configured node
	for _, node := range r.config.PVEInstances {
		nodeDiag := NodeDiagnostic{
			ID:   node.Name,
			Name: node.Name,
			Host: node.Host,
			Type: "pve",
		}

		// Determine auth method (sanitized - don't expose actual values)
		if node.TokenName != "" && node.TokenValue != "" {
			nodeDiag.AuthMethod = "api_token"
		} else if node.User != "" && node.Password != "" {
			nodeDiag.AuthMethod = "username_password"
		} else {
			nodeDiag.AuthMethod = "none"
			nodeDiag.Error = "No authentication configured"
		}

		// Test connection
		testCfg := proxmox.ClientConfig{
			Host:         node.Host,
			User:         node.User,
			Password:     node.Password,
			TokenName:    node.TokenName,
			TokenValue:   node.TokenValue,
			VerifySSL:    node.VerifySSL,
		}

			client, err := proxmox.NewClient(testCfg)
			if err != nil {
				nodeDiag.Connected = false
				nodeDiag.Error = err.Error()
			} else {
				// Try to get cluster status
				if clusterStatus, err := client.GetClusterStatus(ctx); err != nil {
					nodeDiag.Connected = false
					nodeDiag.Error = "Connection established but cluster status failed: " + err.Error()
				} else {
					nodeDiag.Connected = true
					nodeDiag.ClusterInfo = &ClusterInfo{
						Nodes: len(clusterStatus),
					}
					
					// Get node details
					if nodes, err := client.GetNodes(ctx); err == nil && len(nodes) > 0 {
						nodeDiag.Details = &NodeDetails{
							NodeCount: len(nodes),
						}
					}
				}
			}

		diag.Nodes = append(diag.Nodes, nodeDiag)
	}

	// Test PBS instances
	for _, pbsNode := range r.config.PBSInstances {
		pbsDiag := PBSDiagnostic{
			ID:   pbsNode.Name,
			Name: pbsNode.Name,
			Host: pbsNode.Host,
		}

		// Test connection
		testCfg := pbs.ClientConfig{
			Host:         pbsNode.Host,
			User:         pbsNode.User,
			Password:     pbsNode.Password,
			TokenName:    pbsNode.TokenName,
			TokenValue:   pbsNode.TokenValue,
			Fingerprint:  pbsNode.Fingerprint,
			VerifySSL:    pbsNode.VerifySSL,
		}

		client, err := pbs.NewClient(testCfg)
		if err != nil {
			pbsDiag.Connected = false
			pbsDiag.Error = err.Error()
		} else {
			// Try to get version
			if version, err := client.GetVersion(ctx); err != nil {
				pbsDiag.Connected = false
				pbsDiag.Error = "Connection established but version check failed: " + err.Error()
			} else {
				pbsDiag.Connected = true
				pbsDiag.Details = &PBSDetails{
					Version: version.Version,
				}
			}
		}

		diag.PBS = append(diag.PBS, pbsDiag)
	}

	// Add any recent errors from logs (this would need a log collector)
	// For now, just check basic connectivity

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(diag); err != nil {
		log.Error().Err(err).Msg("Failed to encode diagnostics")
		http.Error(w, "Failed to generate diagnostics", http.StatusInternalServerError)
	}
}