package dryrun

import (
	"fmt"
	"regexp"
	"strings"
)

// SimulationResult contains the simulated output of a command.
type SimulationResult struct {
	Output      string `json:"output"`
	ExitCode    int    `json:"exitCode"`
	WouldDo     string `json:"wouldDo"`     // Human-readable description
	Reversible  bool   `json:"reversible"`  // Whether this action can be undone
	RollbackCmd string `json:"rollbackCmd"` // Command to undo (if reversible)
	Simulated   bool   `json:"simulated"`   // Always true for dry-run
}

// simulationPattern defines how to simulate a command pattern.
type simulationPattern struct {
	match    *regexp.Regexp
	generate func(cmd string, matches []string) SimulationResult
}

// Simulator provides dry-run simulation for commands.
type Simulator struct {
	patterns []simulationPattern
}

// NewSimulator creates a new command simulator.
func NewSimulator() *Simulator {
	s := &Simulator{}
	s.initPatterns()
	return s
}

// Simulate returns a simulated result for a command without executing it.
func (s *Simulator) Simulate(command string) SimulationResult {
	for _, pattern := range s.patterns {
		if matches := pattern.match.FindStringSubmatch(command); matches != nil {
			result := pattern.generate(command, matches)
			result.Simulated = true
			return result
		}
	}

	// Default simulation for unknown commands
	return SimulationResult{
		Output:     fmt.Sprintf("[SIMULATED] Would execute: %s", command),
		ExitCode:   0,
		WouldDo:    "Execute unknown command",
		Reversible: false,
		Simulated:  true,
	}
}

func (s *Simulator) initPatterns() {
	s.patterns = []simulationPattern{
		// systemctl operations
		{
			match: regexp.MustCompile(`(?i)systemctl\s+(restart|stop|start)\s+(\S+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				action := matches[1]
				service := matches[2]
				return SimulationResult{
					Output: fmt.Sprintf(`[SIMULATED] %s.service - %s Service
   Loaded: loaded (/lib/systemd/system/%s.service; enabled)
   Active: active (running)

Would %s service %s`, service, strings.Title(service), service, action, service),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("%s service %s", strings.Title(action), service),
					Reversible:  true,
					RollbackCmd: fmt.Sprintf("systemctl %s %s", reverseAction(action), service),
				}
			},
		},
		// apt update
		{
			match: regexp.MustCompile(`(?i)apt\s+update`),
			generate: func(cmd string, matches []string) SimulationResult {
				return SimulationResult{
					Output: `[SIMULATED] Hit:1 http://archive.ubuntu.com/ubuntu jammy InRelease
Hit:2 http://archive.ubuntu.com/ubuntu jammy-updates InRelease
Hit:3 http://archive.ubuntu.com/ubuntu jammy-security InRelease
Reading package lists... Done
Building dependency tree... Done
42 packages can be upgraded. Run 'apt list --upgradable' to see them.`,
					ExitCode:    0,
					WouldDo:     "Update package lists from repositories",
					Reversible:  false, // Can't un-update
					RollbackCmd: "",
				}
			},
		},
		// apt upgrade
		{
			match: regexp.MustCompile(`(?i)apt\s+(upgrade|dist-upgrade|full-upgrade)`),
			generate: func(cmd string, matches []string) SimulationResult {
				return SimulationResult{
					Output: `[SIMULATED] Reading package lists... Done
Building dependency tree... Done
Calculating upgrade... Done
The following packages will be upgraded:
  base-files curl libcurl4 nginx openssh-client openssh-server
  openssl libssl3 vim vim-common vim-runtime
11 upgraded, 0 newly installed, 0 to remove.
Need to get 15.2 MB of archives.
After this operation, 128 kB of additional disk space will be used.

Would upgrade 11 packages`,
					ExitCode:    0,
					WouldDo:     "Upgrade 11 packages (base-files, curl, nginx, openssh, etc.)",
					Reversible:  false, // Package upgrades are hard to reverse
					RollbackCmd: "",
				}
			},
		},
		// apt install
		{
			match: regexp.MustCompile(`(?i)apt\s+install\s+(.+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				packages := matches[1]
				return SimulationResult{
					Output: fmt.Sprintf(`[SIMULATED] Reading package lists... Done
Building dependency tree... Done
The following NEW packages will be installed:
  %s
0 upgraded, 1 newly installed, 0 to remove.
Need to get 2.4 MB of archives.

Would install: %s`, packages, packages),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("Install package(s): %s", packages),
					Reversible:  true,
					RollbackCmd: fmt.Sprintf("apt remove %s", packages),
				}
			},
		},
		// apt remove
		{
			match: regexp.MustCompile(`(?i)apt\s+(remove|purge)\s+(.+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				action := matches[1]
				packages := matches[2]
				return SimulationResult{
					Output: fmt.Sprintf(`[SIMULATED] Reading package lists... Done
Building dependency tree... Done
The following packages will be REMOVED:
  %s
0 upgraded, 0 newly installed, 1 to remove.
After this operation, 5.2 MB disk space will be freed.

Would %s: %s`, packages, action, packages),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("Remove package(s): %s", packages),
					Reversible:  true,
					RollbackCmd: fmt.Sprintf("apt install %s", packages),
				}
			},
		},
		// docker restart/stop/start
		{
			match: regexp.MustCompile(`(?i)docker\s+(restart|stop|start)\s+(\S+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				action := matches[1]
				container := matches[2]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] %s\nWould %s container %s", container, action, container),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("%s container %s", strings.Title(action), container),
					Reversible:  true,
					RollbackCmd: fmt.Sprintf("docker %s %s", reverseAction(action), container),
				}
			},
		},
		// docker exec
		{
			match: regexp.MustCompile(`(?i)docker\s+exec\s+(?:-it?\s+)?(\S+)\s+(.+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				container := matches[1]
				execCmd := matches[2]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would execute in container %s: %s", container, execCmd),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("Execute command in container %s", container),
					Reversible:  false,
					RollbackCmd: "",
				}
			},
		},
		// pct operations (Proxmox containers)
		{
			match: regexp.MustCompile(`(?i)pct\s+(start|stop|reboot)\s+(\d+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				action := matches[1]
				vmid := matches[2]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would %s container %s", action, vmid),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("%s LXC container %s", strings.Title(action), vmid),
					Reversible:  true,
					RollbackCmd: fmt.Sprintf("pct %s %s", reverseAction(action), vmid),
				}
			},
		},
		// pct resize
		{
			match: regexp.MustCompile(`(?i)pct\s+resize\s+(\d+)\s+(\S+)\s+([+-]?\d+[KMGT]?)`),
			generate: func(cmd string, matches []string) SimulationResult {
				vmid := matches[1]
				disk := matches[2]
				size := matches[3]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would resize container %s disk %s by %s", vmid, disk, size),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("Resize container %s disk %s by %s", vmid, disk, size),
					Reversible:  strings.HasPrefix(size, "+"), // Only growth is "reversible" (shrink back)
					RollbackCmd: "",                           // Disk resize is complex to reverse
				}
			},
		},
		// qm operations (Proxmox VMs)
		{
			match: regexp.MustCompile(`(?i)qm\s+(start|stop|reboot|shutdown)\s+(\d+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				action := matches[1]
				vmid := matches[2]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would %s VM %s", action, vmid),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("%s VM %s", strings.Title(action), vmid),
					Reversible:  true,
					RollbackCmd: fmt.Sprintf("qm %s %s", reverseAction(action), vmid),
				}
			},
		},
		// kill process
		{
			match: regexp.MustCompile(`(?i)kill\s+(-\d+\s+)?(\d+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				signal := strings.TrimSpace(matches[1])
				pid := matches[2]
				if signal == "" {
					signal = "-15"
				}
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would send signal %s to process %s", signal, pid),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("Send signal %s to process %s", signal, pid),
					Reversible:  false,
					RollbackCmd: "",
				}
			},
		},
		// pkill
		{
			match: regexp.MustCompile(`(?i)pkill\s+(?:-\d+\s+)?(\S+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				process := matches[1]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would kill processes matching: %s", process),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("Kill processes matching '%s'", process),
					Reversible:  false,
					RollbackCmd: "",
				}
			},
		},
		// rm files
		{
			match: regexp.MustCompile(`(?i)rm\s+(-[rf]+\s+)?(.+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				flags := strings.TrimSpace(matches[1])
				path := matches[2]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would remove: %s (flags: %s)", path, flags),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("Remove %s", path),
					Reversible:  false, // File deletion is not reversible
					RollbackCmd: "",
				}
			},
		},
		// chmod
		{
			match: regexp.MustCompile(`(?i)chmod\s+(-R\s+)?(\d+|[ugoa][+-=][rwx]+)\s+(.+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				recursive := matches[1] != ""
				mode := matches[2]
				path := matches[3]
				desc := fmt.Sprintf("Change permissions of %s to %s", path, mode)
				if recursive {
					desc += " (recursive)"
				}
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would %s", strings.ToLower(desc)),
					ExitCode:    0,
					WouldDo:     desc,
					Reversible:  false, // Would need to record old permissions
					RollbackCmd: "",
				}
			},
		},
		// chown
		{
			match: regexp.MustCompile(`(?i)chown\s+(-R\s+)?(\S+)\s+(.+)`),
			generate: func(cmd string, matches []string) SimulationResult {
				recursive := matches[1] != ""
				owner := matches[2]
				path := matches[3]
				desc := fmt.Sprintf("Change owner of %s to %s", path, owner)
				if recursive {
					desc += " (recursive)"
				}
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would %s", strings.ToLower(desc)),
					ExitCode:    0,
					WouldDo:     desc,
					Reversible:  false,
					RollbackCmd: "",
				}
			},
		},
		// nginx reload/restart
		{
			match: regexp.MustCompile(`(?i)nginx\s+(-s\s+)?(reload|restart|stop|start)`),
			generate: func(cmd string, matches []string) SimulationResult {
				action := matches[2]
				return SimulationResult{
					Output:      fmt.Sprintf("[SIMULATED] Would %s nginx", action),
					ExitCode:    0,
					WouldDo:     fmt.Sprintf("%s nginx", strings.Title(action)),
					Reversible:  true,
					RollbackCmd: fmt.Sprintf("nginx -s %s", reverseAction(action)),
				}
			},
		},
		// df (disk free) - diagnostic
		{
			match: regexp.MustCompile(`(?i)^df\b`),
			generate: func(cmd string, matches []string) SimulationResult {
				return SimulationResult{
					Output: `[SIMULATED] Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1        100G   45G   55G  45% /
/dev/sdb1        500G  200G  300G  40% /data
tmpfs            16G   1.2G   15G   8% /dev/shm`,
					ExitCode:    0,
					WouldDo:     "Display disk space usage",
					Reversible:  true, // Read-only
					RollbackCmd: "",
				}
			},
		},
		// free (memory) - diagnostic
		{
			match: regexp.MustCompile(`(?i)^free\b`),
			generate: func(cmd string, matches []string) SimulationResult {
				return SimulationResult{
					Output: `[SIMULATED]                total        used        free      shared  buff/cache   available
Mem:        32768000    12500000     8200000      512000    12068000    19200000
Swap:        8192000      100000     8092000`,
					ExitCode:    0,
					WouldDo:     "Display memory usage",
					Reversible:  true, // Read-only
					RollbackCmd: "",
				}
			},
		},
		// journalctl - diagnostic
		{
			match: regexp.MustCompile(`(?i)^journalctl\b`),
			generate: func(cmd string, matches []string) SimulationResult {
				return SimulationResult{
					Output: `[SIMULATED] -- Logs begin at Mon 2024-01-01 00:00:00 UTC, end at now. --
Jan 11 10:00:00 server systemd[1]: Started Session 1 of user admin.
Jan 11 10:00:01 server nginx[1234]: 10.0.0.1 - - [11/Jan/2024:10:00:01 +0000] "GET / HTTP/1.1" 200
Jan 11 10:00:02 server sshd[5678]: Accepted publickey for admin from 10.0.0.2`,
					ExitCode:    0,
					WouldDo:     "Display system logs",
					Reversible:  true, // Read-only
					RollbackCmd: "",
				}
			},
		},
	}
}

// reverseAction returns the opposite action for rollback.
func reverseAction(action string) string {
	action = strings.ToLower(action)
	switch action {
	case "start":
		return "stop"
	case "stop":
		return "start"
	case "restart":
		return "restart" // Idempotent
	case "reboot":
		return "start"
	case "reload":
		return "reload"
	default:
		return action
	}
}

// Global simulator instance
var defaultSimulator = NewSimulator()

// Simulate uses the default simulator to simulate a command.
func Simulate(command string) SimulationResult {
	return defaultSimulator.Simulate(command)
}
