package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
	"github.com/rs/zerolog/log"
)

const (
	tempWrapperPath   = "/usr/local/libexec/pulse-sensor-proxy/temp-wrapper.sh"
	tempWrapperScript = `#!/bin/sh
set -eu

if command -v sensors >/dev/null 2>&1; then
    OUTPUT="$(sensors -j 2>/dev/null || true)"
    if [ -n "$OUTPUT" ]; then
        printf '%s\n' "$OUTPUT"
        exit 0
    fi
fi

if [ -r /sys/class/thermal/thermal_zone0/temp ]; then
    RAW="$(cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null || true)"
    if [ -n "$RAW" ]; then
        TEMP="$(awk -v raw="$RAW" 'BEGIN { if (raw == "") exit 1; printf "%.2f", raw / 1000.0 }' 2>/dev/null || true)"
        if [ -n "$TEMP" ]; then
            printf '{"rpitemp-virtual":{"temp1":{"temp1_input":%s}}}\n' "$TEMP"
            exit 0
        fi
    fi
fi

exit 1
`
)

const proxmoxClusterKnownHostsPath = "/etc/pve/priv/known_hosts"

// execCommand executes a shell command and returns output
func execCommand(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(out), err
}

func execCommandWithLimits(cmd string, stdoutLimit, stderrLimit int64) (string, string, bool, bool, error) {
	command := exec.Command("sh", "-c", cmd)

	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return "", "", false, false, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return "", "", false, false, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return "", "", false, false, err
	}

	type pipeResult struct {
		data     []byte
		exceeded bool
		err      error
	}

	var wg sync.WaitGroup
	wg.Add(2)

	stdoutCh := make(chan pipeResult, 1)
	stderrCh := make(chan pipeResult, 1)

	go func() {
		defer wg.Done()
		data, exceeded, readErr := readAllWithLimit(stdoutPipe, stdoutLimit)
		stdoutCh <- pipeResult{data: data, exceeded: exceeded, err: readErr}
	}()

	go func() {
		defer wg.Done()
		data, exceeded, readErr := readAllWithLimit(stderrPipe, stderrLimit)
		stderrCh <- pipeResult{data: data, exceeded: exceeded, err: readErr}
	}()

	var stdoutRes, stderrRes pipeResult
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	<-wgDone
	stdoutRes = <-stdoutCh
	stderrRes = <-stderrCh

	waitErr := command.Wait()

	if stdoutRes.err != nil {
		return "", "", stdoutRes.exceeded, stderrRes.exceeded, fmt.Errorf("stdout read: %w", stdoutRes.err)
	}
	if stderrRes.err != nil {
		return "", "", stdoutRes.exceeded, stderrRes.exceeded, fmt.Errorf("stderr read: %w", stderrRes.err)
	}

	if waitErr != nil {
		return string(stdoutRes.data), string(stderrRes.data), stdoutRes.exceeded, stderrRes.exceeded, waitErr
	}

	return string(stdoutRes.data), string(stderrRes.data), stdoutRes.exceeded, stderrRes.exceeded, nil
}

func readAllWithLimit(r io.Reader, limit int64) ([]byte, bool, error) {
	if limit <= 0 {
		data, err := io.ReadAll(r)
		return data, false, err
	}

	const chunkSize = 32 * 1024
	buf := make([]byte, chunkSize)
	var out bytes.Buffer
	var total int64
	exceeded := false

	for {
		n, err := r.Read(buf)
		if n > 0 {
			if total < limit {
				remaining := limit - total
				toWrite := n
				if int64(n) > remaining {
					toWrite = int(remaining)
					exceeded = true
				}
				if toWrite > 0 {
					if _, writeErr := out.Write(buf[:toWrite]); writeErr != nil {
						return nil, exceeded, writeErr
					}
				}
				if int64(n) > remaining {
					exceeded = true
				}
			} else {
				exceeded = true
			}
			total += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, exceeded, err
		}
	}

	return out.Bytes(), exceeded, nil
}

func (p *Proxy) ensureHostKeyFromProxmox(ctx context.Context, node string) error {
	if !isProxmoxHost() {
		return fmt.Errorf("not running on Proxmox host")
	}

	entries, err := loadProxmoxHostKeys(node)
	if err != nil {
		return err
	}

	if err := p.knownHosts.EnsureWithEntries(ctx, node, 22, entries); err != nil {
		return p.handleHostKeyEnsureError(node, err)
	}

	log.Debug().
		Str("node", node).
		Msg("Loaded host key from Proxmox cluster store")
	return nil
}

func (p *Proxy) handleHostKeyEnsureError(node string, err error) error {
	var changeErr *knownhosts.HostKeyChangeError
	if errors.As(err, &changeErr) {
		log.Error().
			Str("node", node).
			Str("host_spec", changeErr.Host).
			Msg("Detected SSH host key change")
		if p.metrics != nil {
			p.metrics.recordHostKeyChange(node)
		}
	}
	return err
}

func loadProxmoxHostKeys(host string) ([][]byte, error) {
	file, err := os.Open(proxmoxClusterKnownHostsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries [][]byte
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if !knownhosts.HostFieldMatches(host, fields[0]) {
			continue
		}

		comment := ""
		if len(fields) > 3 {
			comment = strings.Join(fields[3:], " ")
		}
		var entry string
		if comment != "" {
			entry = fmt.Sprintf("%s %s %s %s", host, fields[1], fields[2], comment)
		} else {
			entry = fmt.Sprintf("%s %s %s", host, fields[1], fields[2])
		}
		entries = append(entries, []byte(entry))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no Proxmox host keys found for %s in %s", host, proxmoxClusterKnownHostsPath)
	}
	return entries, nil
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
	forced := fmt.Sprintf(`command="%s",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty`, tempWrapperPath)

	// Format: from="...",command="...",no-* ssh-rsa AAAA... pulse-sensor-proxy
	return fmt.Sprintf(`%s,%s %s %s`, fromClause, forced, pubKey, comment), nil
}

func (p *Proxy) ensureHostKey(node string) error {
	if p.knownHosts == nil {
		return fmt.Errorf("host key manager not configured")
	}

	ctx := context.Background()
	if isProxmoxHost() {
		if err := p.ensureHostKeyFromProxmox(ctx, node); err == nil {
			return nil
		} else {
			if p.config.RequireProxmoxHostkeys {
				return err
			}
			log.Warn().
				Str("node", node).
				Err(err).
				Msg("Failed to load host key from Proxmox; falling back to ssh-keyscan")
		}
	} else if p.config.RequireProxmoxHostkeys {
		return fmt.Errorf("require_proxmox_hostkeys enabled but not running on Proxmox host")
	}

	if err := p.knownHosts.Ensure(ctx, node); err != nil {
		return p.handleHostKeyEnsureError(node, err)
	}
	return nil
}

func (p *Proxy) sshCommonOptions() string {
	if p.knownHosts == nil {
		return "-o StrictHostKeyChecking=yes -o BatchMode=yes"
	}
	return fmt.Sprintf("-o StrictHostKeyChecking=yes -o BatchMode=yes -o UserKnownHostsFile=%s -o GlobalKnownHostsFile=/dev/null",
		shellQuote(p.knownHosts.Path()))
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.Contains(arg, "'") {
		return "'" + arg + "'"
	}
	return strconv.Quote(arg)
}

func (p *Proxy) ensureTempWrapper(nodeHost, commonOpts string) error {
	dir := filepath.Dir(tempWrapperPath)
	mkdirCmd := fmt.Sprintf(
		`ssh %s -o ConnectTimeout=10 root@%s "mkdir -p %s && chmod 755 %s"`,
		commonOpts,
		nodeHost,
		dir,
		dir,
	)

	if _, err := execCommand(mkdirCmd); err != nil {
		return fmt.Errorf("failed to prepare temperature wrapper directory on %s: %w", nodeHost, err)
	}

	uploadCmd := fmt.Sprintf(
		`ssh %s -o ConnectTimeout=10 root@%s "cat > %s <<'EOF'
%s
EOF
chmod 755 %s"`,
		commonOpts,
		nodeHost,
		tempWrapperPath,
		tempWrapperScript,
		tempWrapperPath,
	)

	if _, err := execCommand(uploadCmd); err != nil {
		return fmt.Errorf("failed to install temperature wrapper on %s: %w", nodeHost, err)
	}

	return nil
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

	if err := p.ensureHostKey(nodeHost); err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to ensure host key for %s: %w", nodeHost, err)
	}

	commonOpts := p.sshCommonOptions()
	if err := p.ensureTempWrapper(nodeHost, commonOpts); err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to stage temperature wrapper on %s: %w", nodeHost, err)
	}

	// Check if the exact restricted entry already exists
	checkCmd := fmt.Sprintf(
		`ssh %s -o ConnectTimeout=10 root@%s "grep -F '%s' /root/.ssh/authorized_keys 2>/dev/null"`,
		commonOpts,
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
		`ssh %s -o ConnectTimeout=10 root@%s "mkdir -p /root/.ssh && chmod 700 /root/.ssh && grep -v -e 'pulse-temp-proxy$' -e 'pulse-sensor-proxy$' /root/.ssh/authorized_keys > /root/.ssh/authorized_keys.tmp 2>/dev/null || touch /root/.ssh/authorized_keys.tmp"`,
		commonOpts,
		nodeHost,
	)

	if _, err := execCommand(removeOldCmd); err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to prepare authorized_keys on %s: %w", nodeHost, err)
	}

	// Add the new restricted key and atomically replace the file
	addCmd := fmt.Sprintf(
		`ssh %s -o ConnectTimeout=10 root@%s "echo '%s' >> /root/.ssh/authorized_keys.tmp && mv /root/.ssh/authorized_keys.tmp /root/.ssh/authorized_keys && chmod 600 /root/.ssh/authorized_keys"`,
		commonOpts,
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
	if err := p.ensureHostKey(nodeHost); err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to ensure host key for %s: %w", nodeHost, err)
	}

	commonOpts := p.sshCommonOptions()
	cmd := fmt.Sprintf(
		`ssh %s -i %s -T -n -o LogLevel=ERROR -o ConnectTimeout=5 root@%s ""`,
		commonOpts,
		shellQuote(privKeyPath),
		nodeHost,
	)

	_, stderr, stdoutExceeded, stderrExceeded, err := execCommandWithLimits(cmd, p.maxSSHOutputBytes, p.maxSSHOutputBytes)
	if stdoutExceeded {
		log.Warn().Str("node", nodeHost).Int64("limit_bytes", p.maxSSHOutputBytes).Msg("SSH test output exceeded limit")
		if p.metrics != nil {
			p.metrics.recordSSHOutputOversized(nodeHost)
		}
		return fmt.Errorf("ssh test output exceeded %d bytes", p.maxSSHOutputBytes)
	}

	if stderrExceeded {
		log.Warn().Str("node", nodeHost).Int64("limit_bytes", p.maxSSHOutputBytes).Msg("SSH test stderr exceeded limit")
	}

	if err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		stderrMsg := strings.TrimSpace(stderr)
		if stderrMsg != "" {
			return fmt.Errorf("SSH test failed: %w (stderr: %s)", err, stderrMsg)
		}
		return fmt.Errorf("SSH test failed: %w", err)
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
	if err := p.ensureHostKey(nodeHost); err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return "", fmt.Errorf("failed to ensure host key for %s: %w", nodeHost, err)
	}

	commonOpts := p.sshCommonOptions()

	// Since we use a forced wrapper command, any SSH connection runs the wrapper
	// We don't need to specify the command
	cmd := fmt.Sprintf(
		`ssh %s -i %s -T -n -o LogLevel=ERROR -o ConnectTimeout=5 root@%s ""`,
		commonOpts,
		shellQuote(privKeyPath),
		nodeHost,
	)

	stdout, stderr, stdoutExceeded, stderrExceeded, err := execCommandWithLimits(cmd, p.maxSSHOutputBytes, p.maxSSHOutputBytes)
	if stdoutExceeded {
		log.Warn().Str("node", nodeHost).Int64("limit_bytes", p.maxSSHOutputBytes).Msg("SSH temperature output exceeded limit")
		if p.metrics != nil {
			p.metrics.recordSSHOutputOversized(nodeHost)
		}
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		return "", fmt.Errorf("ssh output exceeded %d bytes", p.maxSSHOutputBytes)
	}

	if stderrExceeded {
		log.Warn().Str("node", nodeHost).Int64("limit_bytes", p.maxSSHOutputBytes).Msg("SSH temperature stderr exceeded limit")
	}

	if err != nil {
		p.metrics.sshRequests.WithLabelValues(nodeLabel, "error").Inc()
		p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
		stderrMsg := strings.TrimSpace(stderr)
		if stderrMsg != "" {
			return "", fmt.Errorf("failed to fetch temperatures: %w (stderr: %s)", err, stderrMsg)
		}
		return "", fmt.Errorf("failed to fetch temperatures: %w", err)
	}

	p.metrics.sshRequests.WithLabelValues(nodeLabel, "success").Inc()
	p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
	return stdout, nil
}

// discoverClusterNodes discovers all nodes in the Proxmox cluster
// Returns IP addresses of cluster nodes
// For standalone nodes (no cluster), returns the host's own addresses
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
	err := cmd.Run()

	// pvecm status exits with code 2 or 255 on standalone nodes (not in a cluster)
	// Also handle LXC containers where pvecm can't access corosync IPC
	// Treat these as valid cases and discover local host addresses
	if err != nil {
		stderrStr := stderr.String()
		stdoutStr := out.String()
		combinedOutput := stderrStr + stdoutStr

		// Broadly detect standalone/container scenarios by checking for known patterns
		// This list comes from real user reports and should handle localization/version differences
		//
		// Common patterns that indicate standalone/container operation:
		// - Configuration missing: "does not exist", "not found", "no such file"
		// - Cluster state: "not part of a cluster", "no cluster", "standalone"
		// - IPC failures: "ipcc_send_rec", "IPC", "communication failed"
		// - Permission/access: "Unknown error -1", "Unable to load", "access denied", "permission denied"
		//
		// Strategy: Be permissive - if pvecm fails with any of these common patterns,
		// assume standalone and fall back to localhost. This is safer than false negatives.
		standaloneIndicators := []string{
			// Configuration issues
			"does not exist", "not found", "no such file",
			// Cluster state
			"not part of a cluster", "no cluster", "standalone",
			// IPC/communication failures (common in LXC)
			"ipcc_send_rec", "IPC", "communication failed", "connection refused",
			// Permission/access issues
			"Unknown error -1", "Unable to load", "access denied", "permission denied",
			"access control list",
		}

		isStandalone := false
		for _, indicator := range standaloneIndicators {
			if strings.Contains(strings.ToLower(combinedOutput), strings.ToLower(indicator)) {
				isStandalone = true
				break
			}
		}

		if isStandalone {
			// Log at INFO level since this is expected for standalone/container scenarios
			log.Info().
				Str("exit_code", fmt.Sprintf("%v", err)).
				Msg("Standalone Proxmox node or LXC container detected - using localhost for temperature collection")
			return discoverLocalHostAddresses()
		}

		// For truly unexpected errors (rare), fail with full context for debugging
		log.Warn().
			Err(err).
			Str("stderr", stderrStr).
			Str("stdout", stdoutStr).
			Msg("pvecm status failed with unexpected error - please report this if temperature monitoring doesn't work")
		return nil, fmt.Errorf("failed to get cluster status: %w (stderr: %s, stdout: %s)", err, stderrStr, stdoutStr)
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

// discoverLocalHostAddresses discovers all addresses of the local host
// Used for standalone nodes that aren't part of a cluster
func discoverLocalHostAddresses() ([]string, error) {
	addresses := make(map[string]struct{})

	// Get hostname and FQDN
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		addresses[strings.ToLower(hostname)] = struct{}{}

		// Try to get FQDN
		cmd := exec.Command("hostname", "-f")
		if out, err := cmd.Output(); err == nil {
			fqdn := strings.TrimSpace(string(out))
			if fqdn != "" && fqdn != hostname {
				addresses[strings.ToLower(fqdn)] = struct{}{}
			}
		}
	}

	// Get all non-loopback IP addresses using ip command
	cmd := exec.Command("ip", "-o", "addr", "show")
	out, err := cmd.Output()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get IP addresses via 'ip addr'")
	} else {
		// Parse output lines like:
		// 2: eth0    inet 192.168.0.100/24 brd 192.168.0.255 scope global eth0
		// 2: eth0    inet6 fe80::a00:27ff:fe4e:66a1/64 scope link
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			// Field 2 is interface, field 3 is inet/inet6, field 4 is addr/prefix
			if fields[2] != "inet" && fields[2] != "inet6" {
				continue
			}

			// Split addr/prefix (e.g., "192.168.0.100/24")
			addrWithPrefix := fields[3]
			addr := strings.Split(addrWithPrefix, "/")[0]

			// Skip loopback addresses
			if strings.HasPrefix(addr, "127.") || addr == "::1" {
				continue
			}

			// Skip link-local IPv6
			if strings.HasPrefix(addr, "fe80:") {
				continue
			}

			addresses[addr] = struct{}{}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(addresses))
	for addr := range addresses {
		result = append(result, addr)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no local host addresses found")
	}

	log.Info().
		Strs("addresses", result).
		Msg("Discovered local host addresses for standalone node validation")

	return result, nil
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
