package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
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
	VMDiskCheck  *VMDiskCheckResult `json:"vmDiskCheck,omitempty"`
}

// NodeDetails contains node-specific details
type NodeDetails struct {
	NodeCount int    `json:"node_count,omitempty"`
	Version   string `json:"version,omitempty"`
}

// VMDiskCheckResult contains VM disk monitoring diagnostic results
type VMDiskCheckResult struct {
	VMsFound         int      `json:"vmsFound"`
	VMsWithAgent     int      `json:"vmsWithAgent"`
	VMsWithDiskData  int      `json:"vmsWithDiskData"`
	TestVMID         int      `json:"testVMID,omitempty"`
	TestVMName       string   `json:"testVMName,omitempty"`
	TestResult       string   `json:"testResult,omitempty"`
	Permissions      []string `json:"permissions,omitempty"`
	Recommendations  []string `json:"recommendations,omitempty"`
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
						
						// Get version from first node
						if len(nodes) > 0 {
							if status, err := client.GetNodeStatus(ctx, nodes[0].Node); err == nil && status != nil {
								if status.PVEVersion != "" {
									nodeDiag.Details.Version = status.PVEVersion
								}
							}
						}
					}
					
					// Run VM disk monitoring check
					nodeDiag.VMDiskCheck = r.checkVMDiskMonitoring(ctx, client, node.Name)
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

// checkVMDiskMonitoring performs diagnostic checks for VM disk monitoring
func (r *Router) checkVMDiskMonitoring(ctx context.Context, client *proxmox.Client, nodeName string) *VMDiskCheckResult {
	result := &VMDiskCheckResult{
		Recommendations: []string{},
		Permissions: []string{},
	}
	
	// Get all nodes to check
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		result.TestResult = "Failed to get nodes: " + err.Error()
		return result
	}
	
	if len(nodes) == 0 {
		result.TestResult = "No nodes found"
		return result
	}
	
	// Check all nodes for VMs
	var allVMs []proxmox.VM
	for _, node := range nodes {
		vms, err := client.GetVMs(ctx, node.Node)
		if err != nil {
			log.Debug().Err(err).Str("node", node.Node).Msg("Failed to get VMs from node")
			continue
		}
		allVMs = append(allVMs, vms...)
	}
	
	result.VMsFound = len(allVMs)
	vms := allVMs
	
	if len(vms) == 0 {
		result.TestResult = "No VMs found to test"
		result.Recommendations = append(result.Recommendations, "Create a test VM to verify disk monitoring")
		return result
	}
	
	// Check VMs for agent and disk data
	var testVM *proxmox.VM
	var testVMNode string
	for _, vm := range vms {
		if vm.Template == 0 && vm.Status == "running" {
			// Find which node this VM is on
			vmNode := ""
			for _, node := range nodes {
				nodeVMs, _ := client.GetVMs(ctx, node.Node)
				for _, nvm := range nodeVMs {
					if nvm.VMID == vm.VMID {
						vmNode = node.Node
						break
					}
				}
				if vmNode != "" {
					break
				}
			}
			
			if vmNode == "" {
				continue
			}
			
			// Check if agent is configured
			vmStatus, err := client.GetVMStatus(ctx, vmNode, vm.VMID)
			if err == nil && vmStatus != nil && vmStatus.Agent > 0 {
				result.VMsWithAgent++
				
				// Try to get filesystem info
				fsInfo, err := client.GetVMFSInfo(ctx, vmNode, vm.VMID)
				if err == nil && len(fsInfo) > 0 {
					result.VMsWithDiskData++
					if testVM == nil {
						testVM = &vm
						testVMNode = vmNode
					}
				} else if testVM == nil {
					// Keep this as a test candidate if we haven't found a working one
					testVM = &vm
					testVMNode = vmNode
				}
			}
		}
	}
	
	// Perform detailed test on one VM
	if testVM != nil {
		result.TestVMID = testVM.VMID
		result.TestVMName = testVM.Name
		
		// Check VM status for agent
		vmStatus, err := client.GetVMStatus(ctx, testVMNode, testVM.VMID)
		if err != nil {
			result.TestResult = "Failed to get VM status: " + err.Error()
			result.Recommendations = append(result.Recommendations, "Check API token has PVEAuditor role")
		} else if vmStatus == nil || vmStatus.Agent == 0 {
			result.TestResult = "Guest agent not enabled in VM configuration"
			result.Recommendations = append(result.Recommendations, 
				"Enable QEMU Guest Agent in VM Options",
				"Install qemu-guest-agent package in the VM")
		} else {
			// Try to get filesystem info
			fsInfo, err := client.GetVMFSInfo(ctx, testVMNode, testVM.VMID)
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "500") || strings.Contains(errStr, "not running") {
					result.TestResult = "Guest agent not running inside VM"
					result.Recommendations = append(result.Recommendations,
						"SSH into VM and run: systemctl status qemu-guest-agent",
						"If not installed: apt install qemu-guest-agent",
						"If installed but not running: systemctl start qemu-guest-agent")
				} else if strings.Contains(errStr, "403") || strings.Contains(errStr, "401") {
					result.TestResult = "Permission denied accessing guest agent"
					result.Recommendations = append(result.Recommendations,
						"Ensure API token has PVEAuditor role",
						"For PVE 9: PVEAuditor includes VM.GuestAgent.Audit",
						"For PVE 8: May need additional VM.Monitor permission")
				} else {
					result.TestResult = "Failed to get guest agent data: " + errStr
				}
			} else if len(fsInfo) == 0 {
				result.TestResult = "Guest agent returned no filesystem info"
				result.Recommendations = append(result.Recommendations,
					"Guest agent may need restart inside VM",
					"Check VM has mounted filesystems")
			} else {
				// Calculate disk usage from filesystem info
				var totalBytes, usedBytes uint64
				for _, fs := range fsInfo {
					if fs.Type != "tmpfs" && fs.Type != "devtmpfs" && 
					   !strings.HasPrefix(fs.Mountpoint, "/dev") &&
					   !strings.HasPrefix(fs.Mountpoint, "/proc") &&
					   !strings.HasPrefix(fs.Mountpoint, "/sys") {
						totalBytes += fs.TotalBytes
						usedBytes += fs.UsedBytes
					}
				}
				
				if totalBytes > 0 {
					percent := float64(usedBytes) / float64(totalBytes) * 100
					result.TestResult = fmt.Sprintf("SUCCESS: Guest agent working! Disk usage: %.1f%% (%d/%d bytes)", 
						percent, usedBytes, totalBytes)
				} else {
					result.TestResult = "Guest agent returned filesystems but no usable disk data"
				}
			}
		}
	} else {
		result.TestResult = "No running VMs found to test"
		result.Recommendations = append(result.Recommendations, "Start a VM to test disk monitoring")
	}
	
	// Add general recommendations based on results
	if result.VMsWithAgent > 0 && result.VMsWithDiskData == 0 {
		result.Recommendations = append(result.Recommendations,
			"Guest agent is configured but not providing disk data",
			"Check guest agent is running inside VMs",
			"Verify API token permissions")
	}
	
	return result
}