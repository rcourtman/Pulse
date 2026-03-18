package hostagent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// clusterPeer represents a discovered Proxmox cluster node.
type clusterPeer struct {
	Name   string // Proxmox node name
	IP     string // Node IP address from cluster status
	Online bool   // Whether the node is online
	Local  bool   // Whether this is the local node
}

// pveshClusterStatus is the JSON shape returned by `pvesh get /cluster/status`.
type pveshClusterStatus struct {
	Type   string `json:"type"` // "cluster" or "node"
	Name   string `json:"name"`
	IP     string `json:"ip,omitempty"`
	Local  int    `json:"local,omitempty"` // 1 if this is the local node
	Online int    `json:"online,omitempty"`
	// Other fields exist but we only need these
}

const (
	clusterSensorsPerPeerTimeout = 8 * time.Second
	clusterSensorsTotalBudget    = 15 * time.Second
)

// collectClusterSensors discovers Proxmox cluster siblings and collects temperature
// data from each via SSH. Returns nil if not in a cluster or no peers are reachable.
func (a *Agent) collectClusterSensors(ctx context.Context) []agentshost.ClusterNodeSensors {
	if !a.cfg.EnableProxmox {
		return nil
	}

	// Only collect on Linux (Proxmox is Linux-only)
	if a.collector.GOOS() != "linux" {
		return nil
	}

	budgetCtx, budgetCancel := context.WithTimeout(ctx, clusterSensorsTotalBudget)
	defer budgetCancel()

	// Discover cluster peers
	peers, err := a.discoverClusterPeers(budgetCtx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to discover cluster peers for sensor collection")
		return nil
	}

	// Filter to online, remote peers only
	var remotePeers []clusterPeer
	for _, p := range peers {
		if !p.Local && p.Online && p.IP != "" {
			remotePeers = append(remotePeers, p)
		}
	}

	if len(remotePeers) == 0 {
		return nil
	}

	// Find SSH key for inter-node communication
	sshKeyPath := a.findClusterSSHKey()
	if sshKeyPath == "" {
		a.logger.Debug().Msg("No Proxmox cluster SSH key found, skipping cluster sensor collection")
		return nil
	}

	// Collect from all peers in parallel
	type peerResult struct {
		NodeName string
		Sensors  agentshost.Sensors
		OK       bool
	}

	var wg sync.WaitGroup
	results := make([]peerResult, len(remotePeers))

	for i, peer := range remotePeers {
		wg.Add(1)
		go func(idx int, p clusterPeer) {
			defer wg.Done()

			peerCtx, peerCancel := context.WithTimeout(budgetCtx, clusterSensorsPerPeerTimeout)
			defer peerCancel()

			sensors, err := a.collectPeerSensors(peerCtx, p, sshKeyPath)
			if err != nil {
				a.logger.Debug().
					Err(err).
					Str("peer", p.Name).
					Str("ip", p.IP).
					Msg("Failed to collect sensors from cluster peer")
				return
			}

			results[idx] = peerResult{
				NodeName: strings.ToLower(p.Name),
				Sensors:  sensors,
				OK:       true,
			}
		}(i, peer)
	}

	wg.Wait()

	// Build result slice from successful collections
	now := time.Now().UTC().Format(time.RFC3339)
	var clusterSensors []agentshost.ClusterNodeSensors
	for _, r := range results {
		if r.OK && len(r.Sensors.TemperatureCelsius) > 0 {
			clusterSensors = append(clusterSensors, agentshost.ClusterNodeSensors{
				NodeName:    r.NodeName,
				Sensors:     r.Sensors,
				CollectedAt: now,
			})
		}
	}

	if len(clusterSensors) > 0 {
		a.logger.Debug().
			Int("peerCount", len(remotePeers)).
			Int("successCount", len(clusterSensors)).
			Msg("Collected cluster peer sensor data")
	}

	return clusterSensors
}

// discoverClusterPeers runs pvesh to get the cluster node list.
func (a *Agent) discoverClusterPeers(ctx context.Context) ([]clusterPeer, error) {
	output, err := a.collector.CommandCombinedOutput(ctx, "pvesh", "get", "/cluster/status", "--output-format", "json")
	if err != nil {
		return nil, err
	}

	return parseClusterStatus(output)
}

// parseClusterStatus parses the JSON output of `pvesh get /cluster/status`.
func parseClusterStatus(jsonOutput string) ([]clusterPeer, error) {
	var entries []pveshClusterStatus
	if err := json.Unmarshal([]byte(jsonOutput), &entries); err != nil {
		return nil, err
	}

	var peers []clusterPeer
	for _, entry := range entries {
		if entry.Type != "node" {
			continue
		}
		peers = append(peers, clusterPeer{
			Name:   entry.Name,
			IP:     entry.IP,
			Online: entry.Online == 1,
			Local:  entry.Local == 1,
		})
	}

	return peers, nil
}

// findClusterSSHKey returns the path to the Proxmox inter-node SSH key.
// Proxmox clusters set up mutual root SSH trust during `pvecm add`.
func (a *Agent) findClusterSSHKey() string {
	// Check keys in priority order: ed25519 first (modern), then RSA (legacy)
	candidates := []string{
		"/root/.ssh/id_ed25519",
		"/root/.ssh/id_rsa",
	}

	for _, path := range candidates {
		if _, err := a.collector.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// collectPeerSensors SSHes to a peer node and runs `sensors -j` to collect temperature data.
func (a *Agent) collectPeerSensors(ctx context.Context, peer clusterPeer, sshKeyPath string) (agentshost.Sensors, error) {
	// StrictHostKeyChecking=no is intentional here: Proxmox cluster nodes have
	// mutual root SSH trust set up by `pvecm add`, but the local known_hosts
	// may not include all peer IPs (e.g., after adding a new node). Since the
	// agent already runs as root inside the trusted cluster network and only
	// reads temperature data, the MITM risk is acceptable. This differs from
	// the Pulse server SSH path which manages its own known_hosts file.
	output, err := a.collector.CommandCombinedOutput(ctx, "ssh",
		"-i", sshKeyPath,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"root@"+peer.IP,
		"sensors -j 2>/dev/null || true",
	)
	if err != nil {
		return agentshost.Sensors{}, err
	}

	output = strings.TrimSpace(output)
	if output == "" || output == "{}" {
		return agentshost.Sensors{}, nil
	}

	// Parse using the shared sensors parser
	tempData, err := a.collector.SensorsParse(output)
	if err != nil {
		return agentshost.Sensors{}, err
	}

	if !tempData.Available {
		return agentshost.Sensors{}, nil
	}

	// Convert to flat sensor format using the shared helper
	return convertTemperatureDataToSensors(tempData), nil
}
