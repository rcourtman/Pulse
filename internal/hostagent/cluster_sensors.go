package hostagent

import (
	"context"
	"encoding/json"
	"errors"
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
	clusterSensorWarningInterval = 5 * time.Minute
)

// collectClusterSensors discovers Proxmox cluster siblings and collects temperature
// data from each via SSH. Returns nil if not in a cluster or no peers are reachable.
func (a *Agent) collectClusterSensors(ctx context.Context) []agentshost.ClusterNodeSensors {
	if !a.cfg.EnableProxmox || a.cfg.DisableClusterPeerSensors || ctx.Err() != nil {
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

	a.pruneClusterSensorFailures(remotePeers)
	if len(remotePeers) == 0 {
		return nil
	}

	// Offer only the existing supported cluster identities, preferring PVE's
	// usual RSA identity without excluding a working Ed25519 identity.
	sshKeyPaths := a.findClusterSSHKeys()
	if len(sshKeyPaths) == 0 {
		for _, peer := range remotePeers {
			a.logClusterSensorResult(peer, errors.New("no supported cluster SSH key found"), time.Now())
		}
		return nil
	}

	// Collect from all peers in parallel
	type peerResult struct {
		Peer    clusterPeer
		Sensors agentshost.Sensors
		Err     error
	}

	var wg sync.WaitGroup
	results := make([]peerResult, len(remotePeers))

	for i, peer := range remotePeers {
		wg.Add(1)
		go func(idx int, p clusterPeer) {
			defer wg.Done()

			peerCtx, peerCancel := context.WithTimeout(budgetCtx, clusterSensorsPerPeerTimeout)
			defer peerCancel()

			sensorData, err := a.collectPeerSensors(peerCtx, p, sshKeyPaths)
			if err == nil && len(sensorData.TemperatureCelsius) == 0 {
				err = errors.New("peer returned no usable temperature readings")
			}
			results[idx] = peerResult{Peer: p, Sensors: sensorData, Err: err}
		}(i, peer)
	}

	wg.Wait()

	// Build result slice from successful collections
	now := time.Now().UTC().Format(time.RFC3339)
	var clusterSensors []agentshost.ClusterNodeSensors
	for _, r := range results {
		// Shutdown/caller cancellation is not an SSH incident or a recovery.
		if ctx.Err() == nil {
			a.logClusterSensorResult(r.Peer, r.Err, time.Now())
		}
		if r.Err == nil && len(r.Sensors.TemperatureCelsius) > 0 {
			clusterSensors = append(clusterSensors, agentshost.ClusterNodeSensors{
				NodeName:    strings.ToLower(r.Peer.Name),
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

// findClusterSSHKeys returns existing supported identities, not their contents.
// pvecm commonly authorises RSA; an unrelated Ed25519 key must not hide it.
func (a *Agent) findClusterSSHKeys() []string {
	var paths []string
	for _, path := range []string{"/root/.ssh/id_rsa", "/root/.ssh/id_ed25519"} {
		if _, err := a.collector.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func (a *Agent) pruneClusterSensorFailures(peers []clusterPeer) {
	active := make(map[clusterPeer]bool, len(peers))
	for _, peer := range peers {
		active[peer] = true
	}
	a.clusterSensorLogMu.Lock()
	defer a.clusterSensorLogMu.Unlock()
	for peer := range a.clusterSensorFailures {
		if !active[peer] {
			delete(a.clusterSensorFailures, peer)
		}
	}
}

func (a *Agent) logClusterSensorResult(peer clusterPeer, err error, now time.Time) {
	a.clusterSensorLogMu.Lock()
	defer a.clusterSensorLogMu.Unlock()
	last, failed := a.clusterSensorFailures[peer]
	if err == nil {
		if failed {
			delete(a.clusterSensorFailures, peer)
			a.logger.Info().Str("peer", peer.Name).Msg("Cluster peer sensor collection recovered")
		}
		return
	}
	if failed && now.Sub(last) < clusterSensorWarningInterval {
		return
	}
	if a.clusterSensorFailures == nil {
		a.clusterSensorFailures = make(map[clusterPeer]time.Time)
	}
	a.clusterSensorFailures[peer] = now
	// Do not log SSH/sensor output: it can contain private hardware details.
	a.logger.Warn().Err(err).Str("peer", peer.Name).
		Msg("Failed to collect sensors from cluster peer; check root SSH authorisation and sensors -j on that peer. To use only local agent sensors, set --disable-cluster-peer-sensors (warnings limited to once per peer every 5 minutes)")
}

// collectPeerSensors SSHes to a peer node and runs `sensors -j` to collect temperature data.
func (a *Agent) collectPeerSensors(ctx context.Context, peer clusterPeer, sshKeyPaths []string) (agentshost.Sensors, error) {
	// StrictHostKeyChecking=no is intentional here: Proxmox cluster nodes have
	// mutual root SSH trust set up by `pvecm add`, but the local known_hosts
	// may not include all peer IPs (e.g., after adding a new node). Since the
	// agent already runs as root inside the trusted cluster network and only
	// reads temperature data, the MITM risk is acceptable. This differs from
	// the Pulse server SSH path which manages its own known_hosts file.
	args := make([]string, 0, 2*len(sshKeyPaths)+16)
	for _, path := range sshKeyPaths {
		args = append(args, "-i", path)
	}
	args = append(args,
		"-o", "IdentitiesOnly=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"root@"+peer.IP,
		"sensors -j",
	)
	output, err := a.collector.CommandCombinedOutput(ctx, "ssh", args...)
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
