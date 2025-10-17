package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// execCommand executes a shell command and returns output
func execCommand(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(out), err
}

// getPublicKey reads the SSH public key from the default directory
func (p *Proxy) getPublicKey() (string, error) {
	return p.getPublicKeyFrom(p.sshKeyPath)
}

// getPublicKeyFrom reads the SSH public key from a specific directory
func (p *Proxy) getPublicKeyFrom(keyDir string) (string, error) {
	pubKeyPath := filepath.Join(keyDir, "id_ed25519.pub")
	data, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// buildAuthorizedKey constructs an authorized_keys entry with from= IP restrictions
func (p *Proxy) buildAuthorizedKey(pubKey string) (string, error) {
	subnets := p.config.AllowedSourceSubnets
	if len(subnets) == 0 {
		return "", fmt.Errorf("no allowed source subnets configured or detected")
	}

	// Build from= clause with all allowed subnets
	fromClause := fmt.Sprintf(`from="%s"`, strings.Join(subnets, ","))

	// Comment helps identify and upgrade this key later
	const comment = "pulse-sensor-proxy"

	// Forced command with all restrictions
	const forced = `command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty`

	// Format: from="...",command="...",no-* ssh-rsa AAAA... pulse-sensor-proxy
	return fmt.Sprintf(`%s,%s %s %s`, fromClause, forced, pubKey, comment), nil
}

// pushSSHKeyFrom pushes a public key from a specific directory to a node
func (p *Proxy) pushSSHKeyFrom(nodeHost, keyDir string) error {
	startTime := time.Now()
	nodeLabel := sanitizeNodeLabel(nodeHost)

	pubKey, err := p.getPublicKeyFrom(keyDir)
	if err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to get public key from %s: %w", keyDir, err)
	}

	// Build the restricted authorized_keys entry
	entry, err := p.buildAuthorizedKey(pubKey)
	if err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to build authorized key: %w", err)
	}

	// Check if the exact restricted entry already exists
	checkCmd := fmt.Sprintf(
		`ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 root@%s "grep -F '%s' /root/.ssh/authorized_keys 2>/dev/null"`,
		nodeHost,
		entry,
	)

	if output, _ := execCommand(checkCmd); strings.Contains(output, entry) {
		log.Debug().Str("node", nodeHost).Msg("SSH key already present with from= restrictions")
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "success").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return nil
	}

	// Remove old pulse-temp-proxy and pulse-sensor-proxy entries (for upgrade path)
	removeOldCmd := fmt.Sprintf(
		`ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 root@%s "mkdir -p /root/.ssh && chmod 700 /root/.ssh && grep -v -e 'pulse-temp-proxy$' -e 'pulse-sensor-proxy$' /root/.ssh/authorized_keys > /root/.ssh/authorized_keys.tmp 2>/dev/null || touch /root/.ssh/authorized_keys.tmp"`,
		nodeHost,
	)

	if _, err := execCommand(removeOldCmd); err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to prepare authorized_keys on %s: %w", nodeHost, err)
	}

	// Add the new restricted key and atomically replace the file
	addCmd := fmt.Sprintf(
		`ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 root@%s "echo '%s' >> /root/.ssh/authorized_keys.tmp && mv /root/.ssh/authorized_keys.tmp /root/.ssh/authorized_keys && chmod 600 /root/.ssh/authorized_keys"`,
		nodeHost,
		entry,
	)

	if _, err := execCommand(addCmd); err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to add SSH key to %s: %w", nodeHost, err)
	}

	log.Info().
		Str("node", nodeHost).
		Str("key_dir", keyDir).
		Strs("allowed_subnets", p.config.AllowedSourceSubnets).
		Msg("SSH key installed with from= IP restrictions")

	p.metrics.sshRequests.WithLabelValues(nodeLabel, "success").Inc()
	p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
	return nil
}

// pushSSHKey adds the proxy's public key to a node's authorized_keys with IP restrictions
// Automatically upgrades old keys without from= restrictions
func (p *Proxy) pushSSHKey(nodeHost string) error {
	return p.pushSSHKeyFrom(nodeHost, p.sshKeyPath)
}

// testSSHConnection verifies SSH connectivity to a node
func (p *Proxy) testSSHConnection(nodeHost string) error {
	startTime := time.Now()
	nodeLabel := sanitizeNodeLabel(nodeHost)

	privKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519")
	cmd := fmt.Sprintf(
		`ssh -i %[1]s -T -n -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5 root@%[2]s "echo test"`,
		privKeyPath,
		nodeHost,
	)

	output, err := execCommand(cmd)
	if err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("SSH test failed: %w (output: %s)", err, output)
	}

	// The forced command will run "sensors -j" instead of "echo test"
	// So we should get JSON output, not "test"
	// For now, just check that connection succeeded
	p.metrics.sshRequests.WithLabelValues(nodeLabel, "success").Inc()
	p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
	return nil
}

// getTemperatureViaSSH fetches temperature data from a node
func (p *Proxy) getTemperatureViaSSH(nodeHost string) (string, error) {
	startTime := time.Now()
	nodeLabel := sanitizeNodeLabel(nodeHost)

	privKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519")

	// Since we use ForceCommand="sensors -j", any SSH command will run sensors
	// We don't need to specify the command
	cmd := fmt.Sprintf(
		`ssh -i %[1]s -T -n -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5 root@%[2]s ""`,
		privKeyPath,
		nodeHost,
	)

	output, err := execCommand(cmd)
	if err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return "", fmt.Errorf("failed to fetch temperatures: %w", err)
	}

	p.metrics.sshRequests.WithLabelValues(nodeLabel, "success").Inc()
	p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
	return output, nil
}

// discoverClusterNodes discovers all nodes in the Proxmox cluster
// Returns IP addresses of cluster nodes
func discoverClusterNodes() ([]string, error) {
	// Check if pvecm is available (only on Proxmox hosts)
	if _, err := exec.LookPath("pvecm"); err != nil {
		return nil, fmt.Errorf("pvecm not found - not running on Proxmox host")
	}

	// Get cluster status with IP addresses
	cmd := exec.Command("pvecm", "status")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Warn().Str("stderr", stderr.String()).Msg("pvecm status failed")
		return nil, fmt.Errorf("failed to get cluster status: %w (stderr: %s)", err, stderr.String())
	}

	// Parse output to extract IP addresses
	// Format example:
	// 0x00000001          1 192.168.0.134
	// 0x00000003          1 192.168.0.5 (local)

	var nodes []string
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		// Look for lines with hex ID and IP address
		if !strings.Contains(line, "0x") {
			continue
		}

		fields := strings.Fields(line)
		// Need at least 3 fields: hex_id votes ip [optional:(local)]
		if len(fields) < 3 {
			continue
		}

		// Third field should be the IP address
		ip := fields[2]
		// Basic validation that it looks like an IP
		if strings.Contains(ip, ".") {
			nodes = append(nodes, ip)
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no cluster nodes found with IP addresses")
	}

	return nodes, nil
}

// isProxmoxHost checks if we're running on a Proxmox host
func isProxmoxHost() bool {
	// Check for pvecm command
	if _, err := exec.LookPath("pvecm"); err == nil {
		return true
	}
	// Check for /etc/pve directory
	if info, err := os.Stat("/etc/pve"); err == nil && info.IsDir() {
		return true
	}
	return false
}
