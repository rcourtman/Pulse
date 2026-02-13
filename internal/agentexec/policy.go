package agentexec

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// CommandPolicy defines what commands are allowed, blocked, or require approval
type CommandPolicy struct {
	// AutoApprove patterns - commands matching these are automatically allowed
	AutoApprove []string

	// RequireApproval patterns - commands matching these need user approval
	RequireApproval []string

	// Blocked patterns - commands matching these are never allowed
	Blocked []string

	// compiled regex patterns
	autoApproveRe     []*regexp.Regexp
	requireApprovalRe []*regexp.Regexp
	blockedRe         []*regexp.Regexp
}

// DefaultPolicy returns a sensible default command policy
func DefaultPolicy() *CommandPolicy {
	p := &CommandPolicy{
		AutoApprove: []string{
			// System inspection
			`^ps(\s|$)`,
			`^top\s+-bn`,
			`^df(\s|$)`,
			`^free(\s|$)`,
			`^uptime$`,
			`^hostname$`,
			`^uname(\s|$)`,
			`^cat\s+/proc/`,
			`^cat\s+/etc/os-release`,
			`^lsof(\s|$)`,
			`^netstat(\s|$)`,
			`^ss(\s|$)`,
			`^ip\s+(addr|route|link)`,
			`^ifconfig`,
			`^w$`,
			`^who$`,
			`^last(\s|$)`,

			// Log reading (read-only)
			`^cat\s+/var/log/`,
			`^tail\s+.*(/var/log/|/log/)`,
			`^head\s+.*(/var/log/|/log/)`,
			`^grep\s+.*(/var/log/|/log/)`,
			`^journalctl\s`,

			// Service status (read-only)
			`^systemctl\s+status\s`,
			`^systemctl\s+is-active\s`,
			`^systemctl\s+is-enabled\s`,
			`^systemctl\s+list-units`,
			`^service\s+\S+\s+status`,

			// Docker inspection (read-only)
			`^docker\s+ps`,
			`^docker\s+logs\s`,
			`^docker\s+inspect\s`,
			`^docker\s+stats\s+--no-stream`,
			`^docker\s+top\s`,
			`^docker\s+images`,
			`^docker\s+network\s+ls`,
			`^docker\s+volume\s+ls`,

			// Proxmox inspection (read-only)
			`^pct\s+list`,
			`^pct\s+config\s`,
			`^pct\s+status\s`,
			`^qm\s+list`,
			`^qm\s+config\s`,
			`^qm\s+status\s`,
			`^pvesh\s+get\s`,

			// Disk/storage info
			`^lsblk`,
			`^blkid`,
			`^fdisk\s+-l`,
			`^du(\s|$)`,
			`^ls(\s|$)`,
			`^stat(\s|$)`,
			`^file(\s|$)`,
			`^find\s+/.*-size`,  // Find large files
			`^find\s+/.*-mtime`, // Find by modification time
			`^find\s+/.*-type`,  // Find by type

			// Memory inspection
			`^vmstat`,
			`^sar\s`,
			`^iostat`,
			`^mpstat`,

			// Docker inspection (read-only)
			`^docker\s+system\s+df`,

			// APT inspection (read-only)
			`^apt\s+list`,
			`^apt-cache\s`,
			`^dpkg\s+-l`,
			`^dpkg\s+--list`,
		},

		RequireApproval: []string{
			// Service control
			`^systemctl\s+(restart|stop|start|reload)\s`,
			`^service\s+\S+\s+(restart|stop|start|reload)`,

			// Docker control
			`^docker\s+(restart|stop|start|kill)\s`,
			`^docker\s+exec\s`,
			`^docker\s+rm\s`,

			// Process control
			`^kill\s`,
			`^pkill\s`,
			`^killall\s`,

			// Package management
			`^apt\s`,
			`^apt-get\s`,
			`^yum\s`,
			`^dnf\s`,
			`^pacman\s`,

			// Proxmox control
			`^pct\s+(start|stop|shutdown|reboot|resize|set)\s`,
			`^qm\s+(start|stop|shutdown|reboot|reset|resize|set)\s`,

			// journalctl maintenance (modifies logs / persistent state)
			`^journalctl\s+--vacuum`,
			`^journalctl\s+--rotate`,

			// Temp file cleanup (safe with approval)
			`^rm\s+(-rf?|-fr?)\s+/var/tmp/`,
			`^rm\s+(-rf?|-fr?)\s+/tmp/`,

			// Find commands that can execute actions or delete files
			`^find\s+.+\s-(exec|execdir|ok|okdir|delete)\b`,
		},

		Blocked: []string{
			// Destructive filesystem operations - block root and critical paths
			// Note: /var/tmp and /tmp cleanup is allowed with approval (see RequireApproval)
			`rm\s+-rf\s+/$`,               // rm -rf / (root)
			`rm\s+-rf\s+/\*`,              // rm -rf /* (root wildcard)
			`rm\s+-rf\s+/home($|\s|/)`,    // rm -rf /home
			`rm\s+-rf\s+/etc($|\s|/)`,     // rm -rf /etc
			`rm\s+-rf\s+/usr($|\s|/)`,     // rm -rf /usr
			`rm\s+-rf\s+/var/lib($|\s|/)`, // rm -rf /var/lib
			`rm\s+-rf\s+/boot($|\s|/)`,    // rm -rf /boot
			`rm\s+-rf\s+/root($|\s|/)`,    // rm -rf /root (root home)
			`rm\s+-rf\s+/bin($|\s|/)`,     // rm -rf /bin
			`rm\s+-rf\s+/sbin($|\s|/)`,    // rm -rf /sbin
			`rm\s+-rf\s+/lib($|\s|/)`,     // rm -rf /lib
			`rm\s+-rf\s+/opt($|\s|/)`,     // rm -rf /opt
			`rm\s+--no-preserve-root`,
			`mkfs`,
			`dd\s+.*of=/dev/`,
			`>\s*/dev/sd`,
			`>\s*/dev/nvme`,

			// System destruction (host-level commands only)
			// Note: qm/pct reboot/shutdown are allowed with approval (see RequireApproval)
			`^shutdown(\s|$)`,
			`^reboot(\s|$)`,
			`^init\s+0`,
			`^poweroff(\s|$)`,
			`^halt(\s|$)`,

			// Dangerous permissions
			`chmod\s+777`,
			`chmod\s+-R\s+777`,
			`chown\s+-R\s+.*:.*\s+/`,

			// Remote code execution
			`curl.*\|\s*(ba)?sh`,
			`wget.*\|\s*(ba)?sh`,
			`bash\s+-c\s+.*curl`,
			`bash\s+-c\s+.*wget`,

			// Crypto mining indicators
			`xmrig`,
			`minerd`,
			`cpuminer`,

			// Fork bomb patterns
			`:\(\)\s*{\s*:\s*\|\s*:`,

			// Clear system logs
			`>\s*/var/log/`,
			`truncate.*--size.*0.*/var/log/`,
		},
	}

	p.compile()
	return p
}

func (p *CommandPolicy) compile() {
	p.autoApproveRe = compilePatterns(p.AutoApprove)
	p.requireApprovalRe = compilePatterns(p.RequireApproval)
	p.blockedRe = compilePatterns(p.Blocked)
}

func compilePatterns(patterns []string) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Warn().Err(err).Str("pattern", pattern).Msg("Skipping invalid command policy regex")
			continue
		}
		result = append(result, re)
	}
	return result
}

// PolicyDecision represents the policy decision for a command
type PolicyDecision string

const (
	PolicyAllow           PolicyDecision = "allow"
	PolicyRequireApproval PolicyDecision = "require_approval"
	PolicyBlock           PolicyDecision = "block"
)

// Evaluate checks a command against the policy
func (p *CommandPolicy) Evaluate(command string) PolicyDecision {
	command = strings.TrimSpace(command)
	command = strings.ToLower(command)

	policyCommand := command
	blockedCandidates := []string{command}

	// Normalize simple sudo prefix so policy applies consistently.
	// For sudo invocations with flags, keep policyCommand conservative, but still
	// inspect the extracted underlying command against blocked patterns.
	if strings.HasPrefix(command, "sudo ") {
		if sudoCommand, hasFlags, ok := extractSudoCommand(command); ok {
			blockedCandidates = append(blockedCandidates, sudoCommand)
			if !hasFlags {
				policyCommand = sudoCommand
			}
		}
	}

	// Check blocked first (highest priority)
	for _, candidate := range blockedCandidates {
		for _, re := range p.blockedRe {
			if re.MatchString(candidate) {
				return PolicyBlock
			}
		}
	}

	// Compound shell commands are never auto-approved.
	if containsShellControlOperators(policyCommand) {
		return PolicyRequireApproval
	}

	// Check require approval
	for _, re := range p.requireApprovalRe {
		if re.MatchString(policyCommand) {
			return PolicyRequireApproval
		}
	}

	// Check auto-approve
	for _, re := range p.autoApproveRe {
		if re.MatchString(policyCommand) {
			return PolicyAllow
		}
	}

	// Default: require approval for unknown commands
	return PolicyRequireApproval
}

// IsBlocked returns true if a command is blocked
func (p *CommandPolicy) IsBlocked(command string) bool {
	return p.Evaluate(command) == PolicyBlock
}

// NeedsApproval returns true if a command needs user approval
func (p *CommandPolicy) NeedsApproval(command string) bool {
	return p.Evaluate(command) == PolicyRequireApproval
}

// IsAutoApproved returns true if a command can run without approval
func (p *CommandPolicy) IsAutoApproved(command string) bool {
	return p.Evaluate(command) == PolicyAllow
}

func containsShellControlOperators(command string) bool {
	if strings.Contains(command, "\n") || strings.Contains(command, "\r") {
		return true
	}

	for _, marker := range []string{";", "&&", "||", "|", "`", "$(", ">", "<"} {
		if strings.Contains(command, marker) {
			return true
		}
	}

	return false
}

func extractSudoCommand(command string) (string, bool, bool) {
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "sudo" {
		return "", false, false
	}

	i := 1
	hasFlags := false
	expectValue := false

	for i < len(parts) {
		token := parts[i]
		if expectValue {
			expectValue = false
			i++
			continue
		}

		if token == "--" {
			hasFlags = true
			i++
			break
		}

		if strings.HasPrefix(token, "--") {
			hasFlags = true
			if !strings.Contains(token, "=") && sudoLongOptionNeedsValue(token) {
				expectValue = true
			}
			i++
			continue
		}

		if strings.HasPrefix(token, "-") && token != "-" {
			hasFlags = true
			if sudoShortOptionNeedsValue(token) {
				expectValue = true
			}
			i++
			continue
		}

		break
	}

	if i >= len(parts) {
		return "", hasFlags, false
	}

	return strings.Join(parts[i:], " "), hasFlags, true
}

func sudoLongOptionNeedsValue(token string) bool {
	switch token {
	case "--user", "--group", "--host", "--prompt", "--chdir", "--close-from", "--command-timeout", "--role", "--type":
		return true
	default:
		return false
	}
}

func sudoShortOptionNeedsValue(token string) bool {
	switch token {
	case "-u", "-g", "-h", "-p", "-C", "-T", "-r", "-t", "-R", "-D":
		return true
	}

	if len(token) <= 2 || !strings.HasPrefix(token, "-") || strings.HasPrefix(token, "--") {
		return false
	}

	switch token[1] {
	case 'u', 'g', 'h', 'p', 'C', 'T', 'r', 't', 'R', 'D':
		// Attached value form (e.g. -uroot) already carries its value.
		return false
	default:
		return false
	}
}
