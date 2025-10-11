package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// DiagnosticsInfo contains comprehensive diagnostic information
type DiagnosticsInfo struct {
	Version string           `json:"version"`
	Runtime string           `json:"runtime"`
	Uptime  float64          `json:"uptime"`
	Nodes   []NodeDiagnostic `json:"nodes"`
	PBS     []PBSDiagnostic  `json:"pbs"`
	System  SystemDiagnostic `json:"system"`
	Errors  []string         `json:"errors"`
}

// NodeDiagnostic contains diagnostic info for a Proxmox node
type NodeDiagnostic struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Host          string             `json:"host"`
	Type          string             `json:"type"`
	AuthMethod    string             `json:"authMethod"`
	Connected     bool               `json:"connected"`
	Error         string             `json:"error,omitempty"`
	Details       *NodeDetails       `json:"details,omitempty"`
	LastPoll      string             `json:"lastPoll,omitempty"`
	ClusterInfo   *ClusterInfo       `json:"clusterInfo,omitempty"`
	VMDiskCheck   *VMDiskCheckResult `json:"vmDiskCheck,omitempty"`
	PhysicalDisks *PhysicalDiskCheck `json:"physicalDisks,omitempty"`
}

// NodeDetails contains node-specific details
type NodeDetails struct {
	NodeCount int    `json:"node_count,omitempty"`
	Version   string `json:"version,omitempty"`
}

// VMDiskCheckResult contains VM disk monitoring diagnostic results
type VMDiskCheckResult struct {
	VMsFound         int                `json:"vmsFound"`
	VMsWithAgent     int                `json:"vmsWithAgent"`
	VMsWithDiskData  int                `json:"vmsWithDiskData"`
	TestVMID         int                `json:"testVMID,omitempty"`
	TestVMName       string             `json:"testVMName,omitempty"`
	TestResult       string             `json:"testResult,omitempty"`
	Permissions      []string           `json:"permissions,omitempty"`
	Recommendations  []string           `json:"recommendations,omitempty"`
	ProblematicVMs   []VMDiskIssue      `json:"problematicVMs,omitempty"`
	FilesystemsFound []FilesystemDetail `json:"filesystemsFound,omitempty"`
}

type VMDiskIssue struct {
	VMID   int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Issue  string `json:"issue"`
}

type FilesystemDetail struct {
	Mountpoint string `json:"mountpoint"`
	Type       string `json:"type"`
	Total      uint64 `json:"total"`
	Used       uint64 `json:"used"`
	Filtered   bool   `json:"filtered"`
	Reason     string `json:"reason,omitempty"`
}

// PhysicalDiskCheck contains diagnostic results for physical disk detection
type PhysicalDiskCheck struct {
	NodesChecked    int              `json:"nodesChecked"`
	NodesWithDisks  int              `json:"nodesWithDisks"`
	TotalDisks      int              `json:"totalDisks"`
	NodeResults     []NodeDiskResult `json:"nodeResults"`
	TestResult      string           `json:"testResult,omitempty"`
	Recommendations []string         `json:"recommendations,omitempty"`
}

type NodeDiskResult struct {
	NodeName    string   `json:"nodeName"`
	DiskCount   int      `json:"diskCount"`
	Error       string   `json:"error,omitempty"`
	DiskDevices []string `json:"diskDevices,omitempty"`
	APIResponse string   `json:"apiResponse,omitempty"`
}

// ClusterInfo contains cluster information
type ClusterInfo struct {
	Nodes int `json:"nodes"`
}

// PBSDiagnostic contains diagnostic info for a PBS instance
type PBSDiagnostic struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Host      string      `json:"host"`
	Connected bool        `json:"connected"`
	Error     string      `json:"error,omitempty"`
	Details   *PBSDetails `json:"details,omitempty"`
}

// PBSDetails contains PBS-specific details
type PBSDetails struct {
	Version string `json:"version,omitempty"`
}

// SystemDiagnostic contains system-level diagnostic info
type SystemDiagnostic struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	GoVersion    string `json:"goVersion"`
	NumCPU       int    `json:"numCPU"`
	NumGoroutine int    `json:"numGoroutine"`
	MemoryMB     uint64 `json:"memoryMB"`
}

// handleDiagnostics returns comprehensive diagnostic information
func (r *Router) handleDiagnostics(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
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
			Host:       node.Host,
			User:       node.User,
			Password:   node.Password,
			TokenName:  node.TokenName,
			TokenValue: node.TokenValue,
			VerifySSL:  node.VerifySSL,
		}

		client, err := proxmox.NewClient(testCfg)
		if err != nil {
			nodeDiag.Connected = false
			nodeDiag.Error = err.Error()
		} else {
			// Try to get nodes first (this should work for both clustered and standalone)
			nodes, err := client.GetNodes(ctx)
			if err != nil {
				nodeDiag.Connected = false
				nodeDiag.Error = "Failed to connect to Proxmox API: " + err.Error()
			} else {
				nodeDiag.Connected = true

				// Set node details
				if len(nodes) > 0 {
					nodeDiag.Details = &NodeDetails{
						NodeCount: len(nodes),
					}

					// Get version from first node
					if status, err := client.GetNodeStatus(ctx, nodes[0].Node); err == nil && status != nil {
						if status.PVEVersion != "" {
							nodeDiag.Details.Version = status.PVEVersion
						}
					}
				}

				// Try to get cluster status (this may fail for standalone nodes, which is OK)
				if clusterStatus, err := client.GetClusterStatus(ctx); err == nil {
					nodeDiag.ClusterInfo = &ClusterInfo{
						Nodes: len(clusterStatus),
					}
				} else {
					// Standalone node or cluster status not available
					// This is not an error - standalone nodes don't have cluster status
					log.Debug().Str("node", node.Name).Msg("Cluster status not available (likely standalone node)")
					nodeDiag.ClusterInfo = &ClusterInfo{
						Nodes: 1, // Standalone node
					}
				}

				// Run VM disk monitoring check
				nodeDiag.VMDiskCheck = r.checkVMDiskMonitoring(ctx, client, node.Name)

				// Run physical disk check
				nodeDiag.PhysicalDisks = r.checkPhysicalDisks(ctx, client, node.Name)
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
			Host:        pbsNode.Host,
			User:        pbsNode.User,
			Password:    pbsNode.Password,
			TokenName:   pbsNode.TokenName,
			TokenValue:  pbsNode.TokenValue,
			Fingerprint: pbsNode.Fingerprint,
			VerifySSL:   pbsNode.VerifySSL,
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
		Permissions:     []string{},
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

	// Fetch VMs once per node and keep lookup map
	nodeVMMap := make(map[string][]proxmox.VM)
	var allVMs []proxmox.VM
	for _, node := range nodes {
		vmCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		vms, err := client.GetVMs(vmCtx, node.Node)
		cancel()
		if err != nil {
			log.Debug().Err(err).Str("node", node.Node).Msg("Failed to get VMs from node")
			continue
		}
		nodeVMMap[node.Node] = vms
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
	result.ProblematicVMs = []VMDiskIssue{}
	for _, vm := range vms {
		if vm.Template == 0 && vm.Status == "running" {
			vmNode := strings.TrimSpace(vm.Node)
			if vmNode == "" {
				continue
			}

			// Check if agent is configured
			statusCtx, statusCancel := context.WithTimeout(ctx, 10*time.Second)
			vmStatus, err := client.GetVMStatus(statusCtx, vmNode, vm.VMID)
			statusCancel()
			if err != nil {
				errStr := err.Error()
				result.ProblematicVMs = append(result.ProblematicVMs, VMDiskIssue{
					VMID:   vm.VMID,
					Name:   vm.Name,
					Status: vm.Status,
					Issue:  "Failed to get VM status: " + errStr,
				})
			} else if vmStatus != nil && vmStatus.Agent > 0 {
				result.VMsWithAgent++

				// Try to get filesystem info
				fsCtx, fsCancel := context.WithTimeout(ctx, 10*time.Second)
				fsInfo, err := client.GetVMFSInfo(fsCtx, vmNode, vm.VMID)
				fsCancel()
				if err != nil {
					result.ProblematicVMs = append(result.ProblematicVMs, VMDiskIssue{
						VMID:   vm.VMID,
						Name:   vm.Name,
						Status: vm.Status,
						Issue:  "Agent enabled but failed to get filesystem info: " + err.Error(),
					})
					if testVM == nil {
						testVM = &vm
						testVMNode = vmNode
					}
				} else if len(fsInfo) == 0 {
					result.ProblematicVMs = append(result.ProblematicVMs, VMDiskIssue{
						VMID:   vm.VMID,
						Name:   vm.Name,
						Status: vm.Status,
						Issue:  "Agent returned no filesystem info",
					})
					if testVM == nil {
						testVM = &vm
						testVMNode = vmNode
					}
				} else {
					// Check if we get usable disk data
					hasUsableFS := false
					for _, fs := range fsInfo {
						if fs.Type != "tmpfs" && fs.Type != "devtmpfs" &&
							!strings.HasPrefix(fs.Mountpoint, "/dev") &&
							!strings.HasPrefix(fs.Mountpoint, "/proc") &&
							!strings.HasPrefix(fs.Mountpoint, "/sys") &&
							fs.TotalBytes > 0 {
							hasUsableFS = true
							break
						}
					}

					if hasUsableFS {
						result.VMsWithDiskData++
					} else {
						result.ProblematicVMs = append(result.ProblematicVMs, VMDiskIssue{
							VMID:   vm.VMID,
							Name:   vm.Name,
							Status: vm.Status,
							Issue:  fmt.Sprintf("Agent returned %d filesystems but none are usable for disk metrics", len(fsInfo)),
						})
					}

					if testVM == nil {
						testVM = &vm
						testVMNode = vmNode
					}
				}
			} else if vmStatus != nil {
				// Agent not enabled
				result.ProblematicVMs = append(result.ProblematicVMs, VMDiskIssue{
					VMID:   vm.VMID,
					Name:   vm.Name,
					Status: vm.Status,
					Issue:  "Guest agent not enabled in VM configuration",
				})
			}
		}
	}

	// Perform detailed test on one VM
	if testVM != nil {
		result.TestVMID = testVM.VMID
		result.TestVMName = testVM.Name

		// Check VM status for agent
		statusCtx, statusCancel := context.WithTimeout(ctx, 10*time.Second)
		vmStatus, err := client.GetVMStatus(statusCtx, testVMNode, testVM.VMID)
		statusCancel()
		if err != nil {
			errStr := err.Error()
			result.TestResult = "Failed to get VM status: " + errStr
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errStr, "context deadline exceeded") {
				result.Recommendations = append(result.Recommendations,
					"VM status request timed out; check network connectivity to the node",
					"If this persists, increase the diagnostics timeout or reduce VM load during checks",
				)
			} else if strings.Contains(errStr, "403") || strings.Contains(errStr, "401") {
				result.Recommendations = append(result.Recommendations,
					"Ensure API token has PVEAuditor role",
					"For PVE 9: PVEAuditor includes VM.GuestAgent.Audit",
					"For PVE 8: May need additional VM.Monitor permission",
				)
			} else {
				result.Recommendations = append(result.Recommendations,
					"Verify the node is reachable and API token is valid",
				)
			}
		} else if vmStatus == nil || vmStatus.Agent == 0 {
			result.TestResult = "Guest agent not enabled in VM configuration"
			result.Recommendations = append(result.Recommendations,
				"Enable QEMU Guest Agent in VM Options",
				"Install qemu-guest-agent package in the VM")
		} else {
			// Try to get filesystem info
			fsCtx, fsCancel := context.WithTimeout(ctx, 10*time.Second)
			fsInfo, err := client.GetVMFSInfo(fsCtx, testVMNode, testVM.VMID)
			fsCancel()
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
				} else if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errStr, "context deadline exceeded") {
					result.TestResult = "Guest agent request timed out"
					result.Recommendations = append(result.Recommendations,
						"Ensure the VM responds to guest agent queries promptly",
						"Consider increasing the diagnostics timeout if the environment is large",
					)
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
				result.FilesystemsFound = []FilesystemDetail{}

				for _, fs := range fsInfo {
					fsDetail := FilesystemDetail{
						Mountpoint: fs.Mountpoint,
						Type:       fs.Type,
						Total:      fs.TotalBytes,
						Used:       fs.UsedBytes,
					}

					// Check if this filesystem should be filtered
					if fs.Type == "tmpfs" || fs.Type == "devtmpfs" {
						fsDetail.Filtered = true
						fsDetail.Reason = "Special filesystem type"
					} else if strings.HasPrefix(fs.Mountpoint, "/dev") ||
						strings.HasPrefix(fs.Mountpoint, "/proc") ||
						strings.HasPrefix(fs.Mountpoint, "/sys") ||
						strings.HasPrefix(fs.Mountpoint, "/run") ||
						fs.Mountpoint == "/boot/efi" {
						fsDetail.Filtered = true
						fsDetail.Reason = "System mount point"
					} else if fs.TotalBytes == 0 {
						fsDetail.Filtered = true
						fsDetail.Reason = "Zero total bytes"
					} else {
						// This filesystem counts toward disk usage
						totalBytes += fs.TotalBytes
						usedBytes += fs.UsedBytes
					}

					result.FilesystemsFound = append(result.FilesystemsFound, fsDetail)
				}

				if totalBytes > 0 {
					percent := float64(usedBytes) / float64(totalBytes) * 100
					result.TestResult = fmt.Sprintf("SUCCESS: Guest agent working! Disk usage: %.1f%% (%d/%d bytes)",
						percent, usedBytes, totalBytes)
				} else {
					result.TestResult = fmt.Sprintf("Guest agent returned %d filesystems but no usable disk data (all filtered out)", len(fsInfo))
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

// checkPhysicalDisks performs diagnostic checks for physical disk detection
func (r *Router) checkPhysicalDisks(ctx context.Context, client *proxmox.Client, instanceName string) *PhysicalDiskCheck {
	result := &PhysicalDiskCheck{
		Recommendations: []string{},
		NodeResults:     []NodeDiskResult{},
	}

	// Get all nodes
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		result.TestResult = "Failed to get nodes: " + err.Error()
		return result
	}

	result.NodesChecked = len(nodes)

	// Check each node for physical disks
	for _, node := range nodes {
		nodeResult := NodeDiskResult{
			NodeName: node.Node,
		}

		// Skip offline nodes
		if node.Status != "online" {
			nodeResult.Error = "Node is offline"
			result.NodeResults = append(result.NodeResults, nodeResult)
			continue
		}

		// Try to get disk list
		diskCtx, diskCancel := context.WithTimeout(ctx, 10*time.Second)
		disks, err := client.GetDisks(diskCtx, node.Node)
		diskCancel()
		if err != nil {
			errStr := err.Error()
			nodeResult.Error = errStr

			// Provide specific recommendations based on error
			if strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
				nodeResult.APIResponse = "Permission denied"
				if !contains(result.Recommendations, "Check API token has sufficient permissions for disk monitoring") {
					result.Recommendations = append(result.Recommendations,
						"Check API token has sufficient permissions for disk monitoring",
						"Token needs at least PVEAuditor role on the node")
				}
			} else if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errStr, "context deadline exceeded") {
				nodeResult.APIResponse = "Timeout"
				if !contains(result.Recommendations, "Disk query timed out; verify node connectivity and load") {
					result.Recommendations = append(result.Recommendations,
						"Disk query timed out; verify node connectivity and load",
						"Increase diagnostics timeout if nodes are slow to respond")
				}
			} else if strings.Contains(errStr, "404") || strings.Contains(errStr, "501") {
				nodeResult.APIResponse = "Endpoint not available"
				if !contains(result.Recommendations, "Node may be running older Proxmox version without disk API support") {
					result.Recommendations = append(result.Recommendations,
						"Node may be running older Proxmox version without disk API support",
						"Check if node is running on non-standard hardware (Raspberry Pi, etc)")
				}
			} else {
				nodeResult.APIResponse = "API error"
			}
		} else {
			nodeResult.DiskCount = len(disks)
			if len(disks) > 0 {
				result.NodesWithDisks++
				result.TotalDisks += len(disks)

				// List disk devices
				for _, disk := range disks {
					nodeResult.DiskDevices = append(nodeResult.DiskDevices, disk.DevPath)
				}
			} else {
				nodeResult.APIResponse = "Empty response (no traditional disks found)"
				// This could be normal for SD card/USB based systems
				if !contains(result.Recommendations, "Some nodes returned no disks - may be using SD cards or USB storage") {
					result.Recommendations = append(result.Recommendations,
						"Some nodes returned no disks - may be using SD cards or USB storage",
						"Proxmox disk API only returns SATA/NVMe/SAS disks, not SD cards")
				}
			}
		}

		result.NodeResults = append(result.NodeResults, nodeResult)
	}

	// Generate summary
	if result.NodesChecked == 0 {
		result.TestResult = "No nodes found to check"
	} else if result.NodesWithDisks == 0 {
		result.TestResult = fmt.Sprintf("Checked %d nodes, none returned physical disks", result.NodesChecked)
	} else {
		result.TestResult = fmt.Sprintf("Found %d disks across %d of %d nodes",
			result.TotalDisks, result.NodesWithDisks, result.NodesChecked)
	}

	return result
}

// Helper function to check if slice contains string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
