package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- Agent-side payload types (mirrors agentexec types) ---

type deployPreflightPayload struct {
	RequestID   string                  `json:"request_id"`
	JobID       string                  `json:"job_id"`
	Targets     []deployPreflightTarget `json:"targets"`
	PulseURL    string                  `json:"pulse_url"`
	MaxParallel int                     `json:"max_parallel,omitempty"`
	Timeout     int                     `json:"timeout,omitempty"`
}

type deployPreflightTarget struct {
	TargetID string `json:"target_id"`
	NodeName string `json:"node_name"`
	NodeIP   string `json:"node_ip"`
}

type deployInstallPayload struct {
	RequestID   string                `json:"request_id"`
	JobID       string                `json:"job_id"`
	Targets     []deployInstallTarget `json:"targets"`
	PulseURL    string                `json:"pulse_url"`
	MaxParallel int                   `json:"max_parallel,omitempty"`
	Timeout     int                   `json:"timeout,omitempty"`
}

type deployInstallTarget struct {
	TargetID       string `json:"target_id"`
	NodeName       string `json:"node_name"`
	NodeIP         string `json:"node_ip"`
	Arch           string `json:"arch"`
	BootstrapToken string `json:"bootstrap_token"`
}

type deployCancelPayload struct {
	RequestID string `json:"request_id"`
	JobID     string `json:"job_id"`
}

type deployProgressPayload struct {
	RequestID string `json:"request_id"`
	JobID     string `json:"job_id"`
	TargetID  string `json:"target_id,omitempty"`
	Phase     string `json:"phase"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Data      string `json:"data,omitempty"`
	Final     bool   `json:"final"`
}

// cancelTracker tracks active deploy jobs so they can be canceled.
var (
	activeDeploysMu sync.Mutex
	activeDeploys   = make(map[string]context.CancelFunc) // jobID -> cancel
)

func registerDeployJob(jobID string, cancel context.CancelFunc) {
	activeDeploysMu.Lock()
	activeDeploys[jobID] = cancel
	activeDeploysMu.Unlock()
}

func unregisterDeployJob(jobID string) {
	activeDeploysMu.Lock()
	delete(activeDeploys, jobID)
	activeDeploysMu.Unlock()
}

func (c *CommandClient) handleDeployCancel(payload deployCancelPayload) {
	activeDeploysMu.Lock()
	cancel, ok := activeDeploys[payload.JobID]
	activeDeploysMu.Unlock()

	if ok {
		c.logger.Info().Str("job_id", payload.JobID).Msg("Canceling deploy job")
		cancel()
	} else {
		c.logger.Warn().Str("job_id", payload.JobID).Msg("No active deploy job to cancel")
	}
}

// deployTarget is a unit of work for deployTargetFanOut.
type deployTarget struct {
	ID string // target ID for progress messages
	// Run executes the work and returns true on success.
	Run func(ctx context.Context) bool
}

// deployFanOutParams holds the common parameters for fan-out deploy operations.
type deployFanOutParams struct {
	JobID       string
	RequestID   string
	Operation   string // e.g. "preflight" or "install" — used in log/progress messages
	CancelPhase string // phase name when a target is canceled (e.g. "preflight_complete")
	SuccessMsg  string // final message when all targets succeed
	FailPartial string // format string for partial failures (expects %d, %d)
	FailAllMsg  string // final message when all targets fail
	EmptyMsg    string // message when no targets provided
}

// deployTargetFanOut runs targets concurrently with bounded parallelism,
// tracks failures, and sends a final job_complete progress message.
func (c *CommandClient) deployTargetFanOut(
	ctx context.Context, conn *websocket.Conn,
	targets []deployTarget, maxParallel int, params deployFanOutParams,
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	registerDeployJob(params.JobID, cancel)
	defer unregisterDeployJob(params.JobID)

	c.logger.Info().
		Str("job_id", params.JobID).
		Int("targets", len(targets)).
		Msgf("Starting deploy %s", params.Operation)

	if len(targets) == 0 {
		c.sendDeployProgress(conn, params.RequestID, params.JobID, "",
			"job_complete", "ok", params.EmptyMsg, "", true)
		return
	}

	sem := makeSemaphore(maxParallel)
	var wg sync.WaitGroup
	var failCount int64
	var failMu sync.Mutex

	for _, target := range targets {
		wg.Add(1)
		go func(t deployTarget) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				failMu.Lock()
				failCount++
				failMu.Unlock()
				c.sendDeployProgress(conn, params.RequestID, params.JobID, t.ID,
					params.CancelPhase, "failed", "Canceled", "", false)
				return
			}

			if !t.Run(ctx) {
				failMu.Lock()
				failCount++
				failMu.Unlock()
			}
		}(target)
	}
	wg.Wait()

	totalTargets := int64(len(targets))
	finalStatus := "ok"
	finalMsg := params.SuccessMsg
	if failCount > 0 && failCount < totalTargets {
		finalStatus = "failed"
		finalMsg = fmt.Sprintf(params.FailPartial, failCount, totalTargets)
	} else if failCount == totalTargets {
		finalStatus = "failed"
		finalMsg = params.FailAllMsg
	}
	c.sendDeployProgress(conn, params.RequestID, params.JobID, "",
		"job_complete", finalStatus, finalMsg, "", true)
}

// handleDeployPreflight runs SSH preflight checks against target nodes.
func (c *CommandClient) handleDeployPreflight(ctx context.Context, conn *websocket.Conn, payload deployPreflightPayload) {
	timeout := payload.Timeout
	if timeout <= 0 {
		timeout = 120
	}

	targets := make([]deployTarget, len(payload.Targets))
	for i, t := range payload.Targets {
		t := t
		targets[i] = deployTarget{
			ID: t.TargetID,
			Run: func(ctx context.Context) bool {
				return c.preflightTarget(ctx, conn, payload.RequestID, payload.JobID, t, payload.PulseURL, timeout)
			},
		}
	}

	c.deployTargetFanOut(ctx, conn, targets, payload.MaxParallel, deployFanOutParams{
		JobID:       payload.JobID,
		RequestID:   payload.RequestID,
		Operation:   "preflight checks",
		CancelPhase: "preflight_complete",
		SuccessMsg:  "All preflight checks complete",
		FailPartial: "Preflight completed with %d/%d failures",
		FailAllMsg:  "All preflight checks failed",
		EmptyMsg:    "No targets to check",
	})
}

// handleDeployInstall runs agent installation on target nodes via SSH.
func (c *CommandClient) handleDeployInstall(ctx context.Context, conn *websocket.Conn, payload deployInstallPayload) {
	timeout := payload.Timeout
	if timeout <= 0 {
		timeout = 300
	}

	targets := make([]deployTarget, len(payload.Targets))
	for i, t := range payload.Targets {
		t := t
		targets[i] = deployTarget{
			ID: t.TargetID,
			Run: func(ctx context.Context) bool {
				return c.installTarget(ctx, conn, payload.RequestID, payload.JobID, t, payload.PulseURL, timeout)
			},
		}
	}

	c.deployTargetFanOut(ctx, conn, targets, payload.MaxParallel, deployFanOutParams{
		JobID:       payload.JobID,
		RequestID:   payload.RequestID,
		Operation:   "install",
		CancelPhase: "install_complete",
		SuccessMsg:  "All installations complete",
		FailPartial: "Install completed with %d/%d failures",
		FailAllMsg:  "All installations failed",
		EmptyMsg:    "No targets to install",
	})
}

// preflightTarget runs SSH-based checks against a single peer node.
// Returns true if the target passed all checks, false otherwise.
func (c *CommandClient) preflightTarget(
	ctx context.Context, conn *websocket.Conn,
	requestID, jobID string, target deployPreflightTarget,
	pulseURL string, timeoutSec int,
) bool {
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 0. Validate node IP to prevent injection.
	if err := validateNodeIP(target.NodeIP); err != nil {
		data := marshalPreflightResult(false, false, false, "", err.Error())
		c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
			"preflight_complete", "failed", fmt.Sprintf("Invalid node IP: %s", target.NodeIP), data, false)
		return false
	}

	// 1. SSH reachability check.
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_ssh", "started", "Checking SSH connectivity", "", false)

	sshOK := c.checkSSH(tctx, target.NodeIP)
	if !sshOK {
		data := marshalPreflightResult(false, false, false, "", "SSH connection failed")
		c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
			"preflight_complete", "failed", "SSH check failed", data, false)
		return false
	}
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_ssh", "ok", "SSH reachable", "", false)

	// 2. Architecture detection.
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_arch", "started", "Detecting architecture", "", false)

	arch := c.detectArchSSH(tctx, target.NodeIP)
	if arch == "" {
		data := marshalPreflightResult(true, false, false, "", "Could not detect architecture")
		c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
			"preflight_complete", "failed", "Architecture detection failed", data, false)
		return false
	}
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_arch", "ok", fmt.Sprintf("Architecture: %s", arch), "", false)

	// 3. Check for existing agent.
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_existing_agent", "started", "Checking for existing agent", "", false)

	hasAgent := c.checkExistingAgentSSH(tctx, target.NodeIP)
	agentStatus := "ok"
	agentMsg := "No existing agent"
	if hasAgent {
		agentStatus = "skipped"
		agentMsg = "Agent already installed"
	}
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_existing_agent", agentStatus, agentMsg, "", false)

	// 4. Pulse URL reachability from peer.
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_reachability", "started", "Checking Pulse URL reachability", "", false)

	reachable := c.checkPulseReachabilitySSH(tctx, target.NodeIP, pulseURL)
	reachStatus := "ok"
	reachMsg := "Pulse URL reachable"
	if !reachable {
		reachStatus = "failed"
		reachMsg = "Pulse URL not reachable from target"
	}
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_reachability", reachStatus, reachMsg, "", false)

	// 5. Summary.
	overallStatus := "ok"
	overallMsg := "Ready for deployment"
	if hasAgent {
		overallStatus = "skipped"
		overallMsg = "Skipped: agent already installed"
	} else if !reachable {
		overallStatus = "failed"
		overallMsg = "Preflight failed: Pulse URL not reachable"
	}

	data := marshalPreflightResult(true, reachable, hasAgent, arch, "")
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"preflight_complete", overallStatus, overallMsg, data, false)

	return overallStatus != "failed"
}

// installTarget runs agent installation on a single peer node via SSH.
// Returns true if the installation succeeded, false otherwise.
func (c *CommandClient) installTarget(
	ctx context.Context, conn *websocket.Conn,
	requestID, jobID string, target deployInstallTarget,
	pulseURL string, timeoutSec int,
) bool {
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 0. Validate node IP to prevent injection.
	if err := validateNodeIP(target.NodeIP); err != nil {
		data := marshalInstallResult(-1, err.Error())
		c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
			"install_complete", "failed", fmt.Sprintf("Invalid node IP: %s", target.NodeIP), data, false)
		return false
	}

	// 1. Write bootstrap token to target via SSH stdin.
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"install_transfer", "started", "Transferring bootstrap token", "", false)

	if err := c.writeTokenSSH(tctx, target.NodeIP, target.BootstrapToken); err != nil {
		data := marshalInstallResult(-1, err.Error())
		c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
			"install_complete", "failed", fmt.Sprintf("Token transfer failed: %v", err), data, false)
		return false
	}
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"install_transfer", "ok", "Bootstrap token written", "", false)

	// 2. Run install script.
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"install_execute", "started", "Running install script", "", false)

	exitCode, output, err := c.runInstallSSH(tctx, target.NodeIP, pulseURL)
	if err != nil || exitCode != 0 {
		msg := fmt.Sprintf("Install failed (exit %d)", exitCode)
		if err != nil {
			msg = fmt.Sprintf("Install failed: %v", err)
		}
		data := marshalInstallResult(exitCode, output)
		c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
			"install_complete", "failed", msg, data, false)
		return false
	}

	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"install_execute", "ok", "Install script completed", "", false)

	// 3. Wait for enrollment (the server will update the target status when
	// the new agent enrolls — we just report that install execution is done).
	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"install_enroll_wait", "started", "Waiting for agent enrollment", "", false)

	c.sendDeployProgress(conn, requestID, jobID, target.TargetID,
		"install_complete", "ok", "Installation complete, awaiting enrollment", "", false)

	return true
}

// --- SSH helpers ---

// checkSSH tests basic SSH connectivity to a peer node.
func (c *CommandClient) checkSSH(ctx context.Context, nodeIP string) bool {
	cmd := execCommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", nodeIP),
		"true",
	)
	return cmd.Run() == nil
}

// detectArchSSH detects the architecture of a remote node via SSH.
func (c *CommandClient) detectArchSSH(ctx context.Context, nodeIP string) string {
	cmd := execCommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", nodeIP),
		"uname -m",
	)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	raw := strings.TrimSpace(string(out))
	switch raw {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return raw
	}
}

// checkExistingAgentSSH checks if a Pulse agent is already installed on the target.
func (c *CommandClient) checkExistingAgentSSH(ctx context.Context, nodeIP string) bool {
	cmd := execCommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", nodeIP),
		"systemctl is-active pulse-agent 2>/dev/null || test -f /usr/local/bin/pulse-agent",
	)
	return cmd.Run() == nil
}

// checkPulseReachabilitySSH checks if the peer can reach the Pulse URL.
func (c *CommandClient) checkPulseReachabilitySSH(ctx context.Context, nodeIP, pulseURL string) bool {
	// Use curl with a short timeout to check connectivity.
	cmd := execCommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", nodeIP),
		fmt.Sprintf("curl -sf --connect-timeout 5 -o /dev/null %s/api/health || wget -q --timeout=5 -O /dev/null %s/api/health 2>/dev/null", shellescape(pulseURL), shellescape(pulseURL)),
	)
	return cmd.Run() == nil
}

// writeTokenSSH writes a bootstrap token to a secure path on the target via SSH stdin.
func (c *CommandClient) writeTokenSSH(ctx context.Context, nodeIP, token string) error {
	cmd := execCommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", nodeIP),
		"mkdir -p /run/pulse-agent && cat > /run/pulse-agent/bootstrap.token && chmod 600 /run/pulse-agent/bootstrap.token",
	)
	cmd.Stdin = strings.NewReader(token)
	return cmd.Run()
}

// runInstallSSH runs the Pulse install script on a remote node via SSH.
func (c *CommandClient) runInstallSSH(ctx context.Context, nodeIP, pulseURL string) (int, string, error) {
	// SSH concatenates all command arguments into a single string passed to the
	// remote shell. We use shellescape (single quotes) for the URL, which prevents
	// all shell expansion ($(), backticks, $VAR). We invoke bash explicitly so
	// pipefail works regardless of the remote user's default shell (e.g. dash/ash).
	// The single-quoted URL is embedded in the outer single-quoted bash -c argument
	// using the '\'' escape pattern (end quote, literal quote, resume quote).
	escapedURL := shellescape(pulseURL)
	innerCmd := fmt.Sprintf(
		"set -o pipefail; curl -sfL -- %s/install.sh | bash -s -- --non-interactive --token-file /run/pulse-agent/bootstrap.token --pulse-url %s",
		escapedURL, escapedURL,
	)
	remoteCmd := "bash -c " + shellescape(innerCmd)
	cmd := execCommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", nodeIP),
		remoteCmd,
	)

	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return -1, string(out), err
		}
	}
	return exitCode, string(out), nil
}

// --- helpers ---

func (c *CommandClient) sendDeployProgress(
	conn *websocket.Conn,
	requestID, jobID, targetID, phase, status, message, data string,
	final bool,
) {
	progress := deployProgressPayload{
		RequestID: requestID,
		JobID:     jobID,
		TargetID:  targetID,
		Phase:     phase,
		Status:    status,
		Message:   message,
		Data:      data,
		Final:     final,
	}

	payload, err := json.Marshal(progress)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to marshal deploy progress")
		return
	}

	msg := wsMessage{
		Type:      msgTypeDeployProgress,
		ID:        requestID,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	c.connMu.Lock()
	err = conn.WriteJSON(msg)
	c.connMu.Unlock()

	if err != nil {
		c.logger.Error().Err(err).
			Str("job_id", jobID).
			Str("target_id", targetID).
			Msg("Failed to send deploy progress")
	}
}

func makeSemaphore(maxParallel int) chan struct{} {
	if maxParallel <= 0 {
		maxParallel = 1
	}
	return make(chan struct{}, maxParallel)
}

func marshalPreflightResult(sshOK, pulseReachable, hasAgent bool, arch, errDetail string) string {
	data := map[string]any{
		"ssh_reachable":   sshOK,
		"pulse_reachable": pulseReachable,
		"has_agent":       hasAgent,
		"arch":            arch,
		"error_detail":    errDetail,
	}
	b, _ := json.Marshal(data)
	return string(b)
}

func marshalInstallResult(exitCode int, output string) string {
	// Truncate output to avoid oversized messages.
	if len(output) > 4096 {
		output = output[:4096] + "\n... (truncated)"
	}
	data := map[string]any{
		"exit_code": exitCode,
		"output":    output,
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// shellescape provides basic quoting for use in SSH commands.
// It wraps the string in single quotes and escapes embedded single quotes.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// validateNodeIP checks that a node IP is a valid IP address (not a hostname
// or other string that could be used for command injection).
func validateNodeIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid node IP: %q", ip)
	}
	return nil
}
