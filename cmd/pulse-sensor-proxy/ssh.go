package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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

// Variable for testing to mock net.Interfaces
var netInterfaces = net.Interfaces

// Variable for testing to mock exec.LookPath
var execLookPath = exec.LookPath

// Variable for testing to mock os.Hostname
var osHostname = os.Hostname

// Variable for testing to mock exec.Command (for simple output)
var execCommandFunc = exec.Command

// Variable for testing to mock exec.CommandContext
var execCommandContextFunc = exec.CommandContext

const (
	tempWrapperPath   = "/usr/local/libexec/pulse-sensor-proxy/temp-wrapper.sh"
	tempWrapperScript = `#!/bin/sh
set -eu

WRAPPER_PATH="${PULSE_SENSOR_WRAPPER:-/opt/pulse/sensor-proxy/bin/pulse-sensor-wrapper.sh}"

# Prefer the full wrapper when available so SMART disk temperatures are included
if [ -x "$WRAPPER_PATH" ]; then
    exec "$WRAPPER_PATH"
fi

# Legacy locations (for older installations)
if [ -x /usr/local/bin/pulse-sensor-wrapper.sh ]; then
    exec /usr/local/bin/pulse-sensor-wrapper.sh
fi

# Fallback to basic lm-sensors output (no SMART data)
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

var proxmoxClusterKnownHostsPath = "/etc/pve/priv/known_hosts"

// execCommand executes a shell command and returns output
func execCommand(cmd string) (string, error) {
	out, err := execCommandFunc("sh", "-c", cmd).CombinedOutput()
	return string(out), err
}

// execCommandWithLimitsContext runs a shell command with output limits and context cancellation
func execCommandWithLimitsContext(ctx context.Context, cmd string, stdoutLimit, stderrLimit int64) (string, string, bool, bool, error) {
	command := execCommandContextFunc(ctx, "sh", "-c", cmd)

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

func execCommandWithLimits(cmd string, stdoutLimit, stderrLimit int64) (string, string, bool, bool, error) {
	command := execCommandFunc("sh", "-c", cmd)

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
func (p *Proxy) getTemperatureViaSSH(ctx context.Context, nodeHost string) (string, error) {
	startTime := time.Now()
	nodeLabel := sanitizeNodeLabel(nodeHost)
	localNode := isLocalNode(nodeHost)

	if localNode {
		log.Debug().Str("node", nodeHost).Msg("Self-monitoring detected, using SSH wrapper for SMART temperatures")
	}

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

	stdout, stderr, stdoutExceeded, stderrExceeded, err := execCommandWithLimitsContext(ctx, cmd, p.maxSSHOutputBytes, p.maxSSHOutputBytes)
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
		if localNode {
			log.Warn().
				Str("node", nodeHost).
				Err(err).
				Msg("SSH temperature collection failed on local node, falling back to direct sensors")

			if fallback, localErr := p.getTemperatureLocal(ctx); localErr == nil && strings.TrimSpace(fallback) != "" {
				p.metrics.sshRequests.WithLabelValues(nodeLabel, "success").Inc()
				p.metrics.sshLatency.WithLabelValues(nodeLabel).Observe(time.Since(startTime).Seconds())
				return fallback, nil
			}
		}

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
	cmd := execCommandFunc("pvecm", "status")
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

		// First check for IPC/permission errors - these indicate a cluster exists but we can't access it
		// These should NOT be treated as standalone mode
		ipcErrorIndicators := []string{
			"ipcc_send_rec",
			"Unable to load access control list",
			"access control list",
		}

		for _, indicator := range ipcErrorIndicators {
			if strings.Contains(combinedOutput, indicator) {
				log.Warn().
					Str("stderr", stderrStr).
					Msg("Cannot access Proxmox cluster IPC - cluster validation disabled. Add nodes to allowed_nodes in config if needed.")
				// Return error to disable cluster validation rather than falling back to incorrect standalone mode
				return nil, fmt.Errorf("pvecm cluster IPC access denied (check systemd restrictions or run outside container): %s", stderrStr)
			}
		}

		// Now check for true standalone/no-cluster patterns
		// These indicate the node genuinely isn't part of a cluster
		standaloneIndicators := []string{
			// Configuration missing
			"does not exist", "not found", "no such file",
			// Cluster state
			"not part of a cluster", "no cluster", "standalone",
		}

		isStandalone := false
		for _, indicator := range standaloneIndicators {
			if strings.Contains(strings.ToLower(combinedOutput), strings.ToLower(indicator)) {
				isStandalone = true
				break
			}
		}

		if isStandalone {
			// Log at INFO level since this is expected for standalone scenarios
			log.Info().
				Str("exit_code", fmt.Sprintf("%v", err)).
				Msg("Standalone Proxmox node detected - discovering local host addresses for validation")
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
	return parseClusterNodes(out.String())
}

// parseClusterNodes parses pvecm status output to extract IP addresses
func parseClusterNodes(output string) ([]string, error) {
	var nodes []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "qdevice") {
			continue
		}
		fields := strings.Fields(line)
		// Need at least 2 fields to be a valid node line (id + ip/name)
		if len(fields) < 2 {
			continue
		}

		// Iterate through fields to find the IP address
		for _, field := range fields {
			// Check if it's a valid IP
			if ip := net.ParseIP(field); ip != nil {
				nodes = append(nodes, field)
				break
			}
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
		cmd := execCommandFunc("hostname", "-f")
		if out, err := cmd.Output(); err == nil {
			fqdn := strings.TrimSpace(string(out))
			if fqdn != "" && fqdn != hostname {
				addresses[strings.ToLower(fqdn)] = struct{}{}
			}
		}
	}

	// This is more reliable than shelling out to 'ip addr' and works even with strict systemd restrictions
	ipCount := 0
	interfaces, err := netInterfaces()
	if err != nil {
		// Check if this is an AF_NETLINK restriction error from systemd
		if strings.Contains(err.Error(), "netlinkrib") || strings.Contains(err.Error(), "address family not supported") {
			log.Warn().
				Err(err).
				Msg("AF_NETLINK restricted by systemd - falling back to 'ip addr' command. Update systemd unit to add AF_NETLINK to RestrictAddressFamilies.")
			return discoverLocalHostAddressesFallback()
		}
		log.Warn().
			Err(err).
			Msg("Failed to enumerate network interfaces - temperature monitoring may require manual allowed_nodes configuration")
	} else {
		for _, iface := range interfaces {
			// Skip loopback interfaces
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			// Skip interfaces that are down
			if iface.Flags&net.FlagUp == 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				log.Debug().
					Err(err).
					Str("interface", iface.Name).
					Msg("Failed to get addresses for interface")
				continue
			}

			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				default:
					continue
				}

				// Skip loopback addresses
				if ip.IsLoopback() {
					continue
				}

				// Skip link-local IPv6 addresses
				if ip.IsLinkLocalUnicast() {
					continue
				}

				// Skip unspecified addresses
				if ip.IsUnspecified() {
					continue
				}

				addresses[ip.String()] = struct{}{}
				ipCount++
			}
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

	// Log helpful info about discovered addresses
	logger := log.Info().
		Strs("addresses", result).
		Int("ip_count", ipCount).
		Int("hostname_count", len(result)-ipCount)

	if ipCount == 0 {
		logger.Msg("WARNING: No IP addresses discovered for standalone node - only hostnames available. If temperature monitoring fails with 'node_not_cluster_member' errors, add the node's IP to allowed_nodes in /etc/pulse-sensor-proxy/config.yaml")
	} else {
		logger.Msg("Discovered local host addresses for standalone node validation")
	}

	return result, nil
}

// discoverLocalHostAddressesFallback uses 'ip addr' command when AF_NETLINK is restricted
func discoverLocalHostAddressesFallback() ([]string, error) {
	addresses := make(map[string]struct{})

	// Get hostname and FQDN (same as native version)
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		addresses[strings.ToLower(hostname)] = struct{}{}
		cmd := execCommandFunc("hostname", "-f")
		if out, err := cmd.Output(); err == nil {
			fqdn := strings.TrimSpace(string(out))
			if fqdn != "" && fqdn != hostname {
				addresses[strings.ToLower(fqdn)] = struct{}{}
			}
		}
	}

	// Use 'ip addr' to get IP addresses
	cmd := execCommandFunc("ip", "addr", "show")
	out, err := cmd.Output()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to run 'ip addr' command")
		// Return at least the hostname
		result := make([]string, 0, len(addresses))
		for addr := range addresses {
			result = append(result, addr)
		}
		return result, nil
	}

	// Parse 'ip addr' output for inet/inet6 lines
	// Example: "    inet 192.168.0.5/24 brd 192.168.0.255 scope global eno1"
	ipCount := 0
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "inet ") && !strings.HasPrefix(line, "inet6 ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Second field is the IP/CIDR (e.g., "192.168.0.5/24")
		ipCIDR := fields[1]
		ipStr, _, _ := strings.Cut(ipCIDR, "/")

		ip := net.ParseIP(ipStr)
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			continue
		}

		addresses[ip.String()] = struct{}{}
		ipCount++
	}

	result := make([]string, 0, len(addresses))
	for addr := range addresses {
		result = append(result, addr)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no local host addresses found")
	}

	// Log helpful info about discovered addresses
	logger := log.Info().
		Strs("addresses", result).
		Int("ip_count", ipCount).
		Int("hostname_count", len(result)-ipCount)

	if ipCount == 0 {
		logger.Msg("WARNING: No IP addresses discovered via 'ip addr' fallback - only hostnames available. Temperature monitoring may require manual allowed_nodes configuration.")
	} else {
		logger.Msg("Discovered local host addresses via 'ip addr' fallback (systemd unit needs AF_NETLINK update)")
	}

	return result, nil
}

// isProxmoxHost checks if we're running on a Proxmox host
func isProxmoxHost() bool {
	// Check for pvecm command
	if _, err := execLookPath("pvecm"); err == nil {
		return true
	}
	// Check for /etc/pve directory
	if info, err := os.Stat("/etc/pve"); err == nil && info.IsDir() {
		return true
	}
	return false
}

// isLocalNode checks if the requested node is the local machine
func isLocalNode(nodeHost string) bool {
	// Get local hostname (short)
	hostname, err := osHostname()
	if err == nil {
		// Match short hostname
		if strings.EqualFold(nodeHost, hostname) {
			return true
		}
		// Match FQDN if nodeHost contains dots
		if strings.Contains(nodeHost, ".") {
			cmd := execCommandFunc("hostname", "-f")
			if output, err := cmd.Output(); err == nil {
				fqdn := strings.TrimSpace(string(output))
				if strings.EqualFold(nodeHost, fqdn) {
					return true
				}
			}
		}
	}

	// Check if nodeHost is a local IP address
	ifaces, err := netInterfaces()
	if err != nil {
		return false
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.String() == nodeHost {
				return true
			}
		}
	}

	// Check special cases
	if nodeHost == "localhost" || nodeHost == "127.0.0.1" || nodeHost == "::1" {
		return true
	}

	return false
}

// getTemperatureLocal collects temperature data from the local machine
func (p *Proxy) getTemperatureLocal(ctx context.Context) (string, error) {
	// Run the same command that the wrapper script runs with context timeout
	cmd := execCommandContextFunc(ctx, "sensors", "-j")
	output, err := cmd.Output()
	if err != nil {
		// Try without -j flag as fallback
		cmd = execCommandContextFunc(ctx, "sensors")
		if _, err = cmd.Output(); err != nil {
			return "", fmt.Errorf("failed to run sensors: %w", err)
		}
		// Return empty JSON object for non-JSON output
		return "{}", nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "{}", nil
	}

	return result, nil
}
