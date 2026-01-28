package servicediscovery

import (
	"fmt"
	"regexp"
	"strings"
)

// safeResourceIDPattern matches valid resource IDs: alphanumeric, dash, underscore, period, colon
// This prevents shell injection via malicious resource names.
var safeResourceIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

// ValidateResourceID checks if a resource ID is safe to use in shell commands.
// Returns an error if the ID contains potentially dangerous characters.
func ValidateResourceID(id string) error {
	if id == "" {
		return fmt.Errorf("resource ID cannot be empty")
	}
	if len(id) > 256 {
		return fmt.Errorf("resource ID too long (max 256 chars)")
	}
	if !safeResourceIDPattern.MatchString(id) {
		return fmt.Errorf("resource ID contains invalid characters: only alphanumeric, dash, underscore, period, and colon allowed")
	}
	return nil
}

// shellQuote safely quotes a string for use as a shell argument.
// Uses single quotes and escapes any embedded single quotes.
func shellQuote(s string) string {
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

// DiscoveryCommand represents a command to run during discovery.
type DiscoveryCommand struct {
	Name        string   // Human-readable name
	Command     string   // The command template
	Description string   // What this discovers
	Categories  []string // What categories of info this provides
	Timeout     int      // Timeout in seconds (0 = default)
	Optional    bool     // If true, don't fail if command fails
}

// CommandSet represents a set of commands for a resource type.
type CommandSet struct {
	ResourceType ResourceType
	Commands     []DiscoveryCommand
}

// GetCommandsForResource returns the commands to run for a given resource type.
func GetCommandsForResource(resourceType ResourceType) []DiscoveryCommand {
	switch resourceType {
	case ResourceTypeLXC:
		return getLXCCommands()
	case ResourceTypeVM:
		return getVMCommands()
	case ResourceTypeDocker:
		return getDockerCommands()
	case ResourceTypeDockerVM, ResourceTypeDockerLXC:
		return getNestedDockerCommands()
	case ResourceTypeK8s:
		return getK8sCommands()
	case ResourceTypeHost:
		return getHostCommands()
	default:
		return []DiscoveryCommand{}
	}
}

// getLXCCommands returns commands for discovering LXC containers.
func getLXCCommands() []DiscoveryCommand {
	return []DiscoveryCommand{
		{
			Name:        "os_release",
			Command:     "cat /etc/os-release",
			Description: "Operating system identification",
			Categories:  []string{"version", "config"},
			Optional:    true,
		},
		{
			Name:        "hostname",
			Command:     "hostname",
			Description: "Container hostname",
			Categories:  []string{"config"},
			Optional:    true,
		},
		{
			Name:        "running_services",
			Command:     "systemctl list-units --type=service --state=running --no-pager 2>/dev/null | head -30 || service --status-all 2>/dev/null | grep '+' | head -30",
			Description: "Running services and daemons",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "listening_ports",
			Command:     "ss -tlnp 2>/dev/null | head -25 || netstat -tlnp 2>/dev/null | head -25",
			Description: "Network ports listening",
			Categories:  []string{"port", "network"},
			Optional:    true,
		},
		{
			Name:        "top_processes",
			Command:     "ps aux --sort=-rss 2>/dev/null | head -15 || ps aux | head -15",
			Description: "Top processes by memory",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "disk_usage",
			Command:     "df -h 2>/dev/null | head -15",
			Description: "Disk usage and mount points",
			Categories:  []string{"storage"},
			Optional:    true,
		},
		{
			Name:        "docker_check",
			Command:     "docker ps --format '{{.Names}}: {{.Image}} ({{.Status}})' 2>/dev/null | head -20 || echo 'no_docker'",
			Description: "Docker containers if running",
			Categories:  []string{"service", "container"},
			Optional:    true,
		},
		{
			Name:        "docker_mounts",
			Command:     `sh -c 'docker ps -q 2>/dev/null | head -15 | while read id; do name=$(docker inspect --format "{{.Name}}" "$id" 2>/dev/null | sed "s|^/||"); echo "CONTAINER:$name"; docker inspect --format "{{range .Mounts}}{{.Source}}|{{.Destination}}|{{.Type}}{{println}}{{end}}" "$id" 2>/dev/null | grep -v "^$" || true; done; echo docker_mounts_done'`,
			Description: "Docker container bind mounts (source -> destination)",
			Categories:  []string{"config", "storage"},
			Optional:    true,
		},
		{
			Name:        "installed_packages",
			Command:     "dpkg -l 2>/dev/null | grep -E '^ii' | awk '{print $2}' | head -50 || rpm -qa 2>/dev/null | head -50 || apk list --installed 2>/dev/null | head -50",
			Description: "Installed packages",
			Categories:  []string{"version", "service"},
			Optional:    true,
		},
		{
			Name:        "config_files",
			Command:     "find /etc -name '*.conf' -o -name '*.yml' -o -name '*.yaml' -o -name '*.json' 2>/dev/null | head -30",
			Description: "Configuration files",
			Categories:  []string{"config"},
			Optional:    true,
		},
		{
			Name:        "cron_jobs",
			Command:     "crontab -l 2>/dev/null | grep -v '^#' | head -10 || ls -la /etc/cron.d/ 2>/dev/null | head -10",
			Description: "Scheduled jobs",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "hardware_info",
			Command:     "lspci 2>/dev/null | head -20 || echo 'no_lspci'",
			Description: "Hardware devices (e.g., Coral TPU)",
			Categories:  []string{"hardware"},
			Optional:    true,
		},
		{
			Name:        "gpu_devices",
			Command:     "ls -la /dev/dri/ 2>/dev/null; ls -la /dev/apex* 2>/dev/null; nvidia-smi -L 2>/dev/null || echo 'no_gpu'",
			Description: "GPU and TPU devices",
			Categories:  []string{"hardware"},
			Optional:    true,
		},
	}
}

// getVMCommands returns commands for discovering VMs (via QEMU guest agent).
func getVMCommands() []DiscoveryCommand {
	return []DiscoveryCommand{
		{
			Name:        "os_release",
			Command:     "cat /etc/os-release",
			Description: "Operating system identification",
			Categories:  []string{"version", "config"},
			Optional:    true,
		},
		{
			Name:        "hostname",
			Command:     "hostname",
			Description: "VM hostname",
			Categories:  []string{"config"},
			Optional:    true,
		},
		{
			Name:        "running_services",
			Command:     "systemctl list-units --type=service --state=running --no-pager 2>/dev/null | head -30",
			Description: "Running services and daemons",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "listening_ports",
			Command:     "ss -tlnp 2>/dev/null | head -25 || netstat -tlnp 2>/dev/null | head -25",
			Description: "Network ports listening",
			Categories:  []string{"port", "network"},
			Optional:    true,
		},
		{
			Name:        "top_processes",
			Command:     "ps aux --sort=-rss 2>/dev/null | head -15",
			Description: "Top processes by memory",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "disk_usage",
			Command:     "df -h 2>/dev/null | head -15",
			Description: "Disk usage and mount points",
			Categories:  []string{"storage"},
			Optional:    true,
		},
		{
			Name:        "docker_check",
			Command:     "docker ps --format '{{.Names}}: {{.Image}} ({{.Status}})' 2>/dev/null | head -20 || echo 'no_docker'",
			Description: "Docker containers if running",
			Categories:  []string{"service", "container"},
			Optional:    true,
		},
		{
			Name:        "docker_mounts",
			Command:     `sh -c 'docker ps -q 2>/dev/null | head -15 | while read id; do name=$(docker inspect --format "{{.Name}}" "$id" 2>/dev/null | sed "s|^/||"); echo "CONTAINER:$name"; docker inspect --format "{{range .Mounts}}{{.Source}}|{{.Destination}}|{{.Type}}{{println}}{{end}}" "$id" 2>/dev/null | grep -v "^$" || true; done; echo docker_mounts_done'`,
			Description: "Docker container bind mounts (source -> destination)",
			Categories:  []string{"config", "storage"},
			Optional:    true,
		},
		{
			Name:        "hardware_info",
			Command:     "lspci 2>/dev/null | head -20",
			Description: "PCI hardware devices",
			Categories:  []string{"hardware"},
			Optional:    true,
		},
		{
			Name:        "gpu_devices",
			Command:     "ls -la /dev/dri/ 2>/dev/null; nvidia-smi -L 2>/dev/null || echo 'no_gpu'",
			Description: "GPU devices",
			Categories:  []string{"hardware"},
			Optional:    true,
		},
	}
}

// getDockerCommands returns commands for discovering Docker containers.
// These are run inside the container via docker exec.
func getDockerCommands() []DiscoveryCommand {
	return []DiscoveryCommand{
		{
			Name:        "os_release",
			Command:     "cat /etc/os-release 2>/dev/null || cat /etc/alpine-release 2>/dev/null || echo 'unknown'",
			Description: "Container OS",
			Categories:  []string{"version"},
			Optional:    true,
		},
		{
			Name:        "processes",
			Command:     "ps aux 2>/dev/null || echo 'no_ps'",
			Description: "Running processes",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "listening_ports",
			Command:     "ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null || echo 'no_ss'",
			Description: "Listening ports inside container",
			Categories:  []string{"port"},
			Optional:    true,
		},
		{
			Name:        "env_vars",
			Command:     "env 2>/dev/null | grep -vE '(PASSWORD|SECRET|KEY|TOKEN|CREDENTIAL)' | head -30",
			Description: "Environment variables (filtered)",
			Categories:  []string{"config"},
			Optional:    true,
		},
		{
			Name:        "config_files",
			Command:     "find /config /data /app /etc -maxdepth 2 -name '*.conf' -o -name '*.yml' -o -name '*.yaml' -o -name '*.json' 2>/dev/null | head -20",
			Description: "Configuration files",
			Categories:  []string{"config"},
			Optional:    true,
		},
	}
}

// getNestedDockerCommands returns commands for Docker inside VMs or LXCs.
func getNestedDockerCommands() []DiscoveryCommand {
	return []DiscoveryCommand{
		{
			Name:        "docker_containers",
			Command:     "docker ps -a --format '{{.Names}}|{{.Image}}|{{.Status}}|{{.Ports}}'",
			Description: "All Docker containers",
			Categories:  []string{"container", "service"},
			Optional:    false,
		},
		{
			Name:        "docker_images",
			Command:     "docker images --format '{{.Repository}}:{{.Tag}}' | head -20",
			Description: "Docker images",
			Categories:  []string{"version"},
			Optional:    true,
		},
		{
			Name:        "docker_compose",
			Command:     "find /opt /home /root -name 'docker-compose*.yml' -o -name 'compose*.yml' 2>/dev/null | head -10",
			Description: "Docker compose files",
			Categories:  []string{"config"},
			Optional:    true,
		},
	}
}

// getK8sCommands returns commands for discovering Kubernetes pods.
func getK8sCommands() []DiscoveryCommand {
	return []DiscoveryCommand{
		{
			Name:        "processes",
			Command:     "ps aux 2>/dev/null || echo 'no_ps'",
			Description: "Running processes in pod",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "listening_ports",
			Command:     "ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null || echo 'no_ss'",
			Description: "Listening ports",
			Categories:  []string{"port"},
			Optional:    true,
		},
		{
			Name:        "env_vars",
			Command:     "env 2>/dev/null | grep -vE '(PASSWORD|SECRET|KEY|TOKEN|CREDENTIAL)' | head -30",
			Description: "Environment variables (filtered)",
			Categories:  []string{"config"},
			Optional:    true,
		},
	}
}

// getHostCommands returns commands for discovering host systems.
func getHostCommands() []DiscoveryCommand {
	return []DiscoveryCommand{
		{
			Name:        "os_release",
			Command:     "cat /etc/os-release",
			Description: "Operating system",
			Categories:  []string{"version", "config"},
			Optional:    true,
		},
		{
			Name:        "hostname",
			Command:     "hostname -f 2>/dev/null || hostname",
			Description: "Full hostname",
			Categories:  []string{"config"},
			Optional:    true,
		},
		{
			Name:        "running_services",
			Command:     "systemctl list-units --type=service --state=running --no-pager 2>/dev/null | head -40",
			Description: "Running services",
			Categories:  []string{"service"},
			Optional:    true,
		},
		{
			Name:        "listening_ports",
			Command:     "ss -tlnp 2>/dev/null | head -30",
			Description: "Listening network ports",
			Categories:  []string{"port", "network"},
			Optional:    true,
		},
		{
			Name:        "docker_containers",
			Command:     "docker ps --format '{{.Names}}: {{.Image}} ({{.Status}})' 2>/dev/null | head -30 || echo 'no_docker'",
			Description: "Docker containers on host",
			Categories:  []string{"container", "service"},
			Optional:    true,
		},
		{
			Name:        "proxmox_version",
			Command:     "pveversion 2>/dev/null || echo 'not_proxmox'",
			Description: "Proxmox version if applicable",
			Categories:  []string{"version"},
			Optional:    true,
		},
		{
			Name:        "zfs_pools",
			Command:     "zpool list 2>/dev/null | head -10 || echo 'no_zfs'",
			Description: "ZFS pools",
			Categories:  []string{"storage"},
			Optional:    true,
		},
		{
			Name:        "disk_usage",
			Command:     "df -h | head -20",
			Description: "Disk usage",
			Categories:  []string{"storage"},
			Optional:    true,
		},
		{
			Name:        "hardware_info",
			Command:     "lscpu | head -20",
			Description: "CPU information",
			Categories:  []string{"hardware"},
			Optional:    true,
		},
		{
			Name:        "memory_info",
			Command:     "free -h",
			Description: "Memory information",
			Categories:  []string{"hardware"},
			Optional:    true,
		},
	}
}

// BuildLXCCommand wraps a command for execution in an LXC container.
// The vmid is validated to prevent command injection.
func BuildLXCCommand(vmid string, cmd string) string {
	if err := ValidateResourceID(vmid); err != nil {
		// Don't include the invalid ID in output to prevent any injection
		return "sh -c 'echo \"Discovery error: invalid LXC container ID\" >&2; exit 1'"
	}
	return fmt.Sprintf("pct exec %s -- sh -c %s", vmid, shellQuote(cmd))
}

// BuildVMCommand wraps a command for execution in a VM via QEMU guest agent.
// Note: This requires the guest agent to be running.
// The vmid is validated to prevent command injection.
func BuildVMCommand(vmid string, cmd string) string {
	if err := ValidateResourceID(vmid); err != nil {
		return "sh -c 'echo \"Discovery error: invalid VM ID\" >&2; exit 1'"
	}
	// For VMs, we use qm guest exec which requires the guest agent
	return fmt.Sprintf("qm guest exec %s -- sh -c %s", vmid, shellQuote(cmd))
}

// BuildDockerCommand wraps a command for execution in a Docker container.
// The containerName is validated to prevent command injection.
// Note: Leading slashes are trimmed as Docker API often returns names with leading /.
func BuildDockerCommand(containerName string, cmd string) string {
	// Docker API returns container names with leading slash, trim it
	containerName = strings.TrimPrefix(containerName, "/")
	if err := ValidateResourceID(containerName); err != nil {
		return "sh -c 'echo \"Discovery error: invalid container name\" >&2; exit 1'"
	}
	return fmt.Sprintf("docker exec %s sh -c %s", shellQuote(containerName), shellQuote(cmd))
}

// BuildNestedDockerCommand builds a command to run inside Docker on a VM/LXC.
// All resource identifiers are validated to prevent command injection.
func BuildNestedDockerCommand(vmid string, isLXC bool, containerName string, cmd string) string {
	if err := ValidateResourceID(vmid); err != nil {
		return "sh -c 'echo \"Discovery error: invalid VM/LXC ID\" >&2; exit 1'"
	}
	// Docker API returns container names with leading slash, trim it
	containerName = strings.TrimPrefix(containerName, "/")
	if err := ValidateResourceID(containerName); err != nil {
		return "sh -c 'echo \"Discovery error: invalid container name\" >&2; exit 1'"
	}
	dockerCmd := BuildDockerCommand(containerName, cmd)
	if isLXC {
		return BuildLXCCommand(vmid, dockerCmd)
	}
	return BuildVMCommand(vmid, dockerCmd)
}

// BuildK8sCommand builds a command to run in a Kubernetes pod.
// All identifiers are validated to prevent command injection.
func BuildK8sCommand(namespace, podName, containerName, cmd string) string {
	if err := ValidateResourceID(namespace); err != nil {
		return "sh -c 'echo \"Discovery error: invalid namespace\" >&2; exit 1'"
	}
	if err := ValidateResourceID(podName); err != nil {
		return "sh -c 'echo \"Discovery error: invalid pod name\" >&2; exit 1'"
	}
	if containerName != "" {
		if err := ValidateResourceID(containerName); err != nil {
			return "sh -c 'echo \"Discovery error: invalid container name\" >&2; exit 1'"
		}
		return fmt.Sprintf("kubectl exec -n %s %s -c %s -- sh -c %s", shellQuote(namespace), shellQuote(podName), shellQuote(containerName), shellQuote(cmd))
	}
	return fmt.Sprintf("kubectl exec -n %s %s -- sh -c %s", shellQuote(namespace), shellQuote(podName), shellQuote(cmd))
}

// GetCLIAccessTemplate returns a CLI access template for a resource type.
// These are instructions for using pulse_control, NOT literal shell commands.
// Commands via pulse_control run directly on the target where the agent is installed.
func GetCLIAccessTemplate(resourceType ResourceType) string {
	switch resourceType {
	case ResourceTypeLXC:
		// Agent runs ON the LXC - commands execute directly inside the container
		return "Use pulse_control with target_host matching this LXC's hostname. Commands run directly inside the container."
	case ResourceTypeVM:
		// Agent runs ON the VM - commands execute directly inside the VM
		return "Use pulse_control with target_host matching this VM's hostname. Commands run directly inside the VM."
	case ResourceTypeDocker:
		// Docker container on a host - need docker exec from the host
		return "Use pulse_control targeting the Docker host with command: docker exec {container} <your-command>"
	case ResourceTypeDockerLXC:
		// Docker inside an LXC - agent on the LXC runs docker exec
		return "Use pulse_control targeting the LXC hostname with command: docker exec {container} <your-command>"
	case ResourceTypeDockerVM:
		// Docker inside a VM - agent on the VM runs docker exec
		return "Use pulse_control targeting the VM hostname with command: docker exec {container} <your-command>"
	case ResourceTypeK8s:
		return "Use kubectl exec -n {namespace} {pod} -- <your-command>"
	case ResourceTypeHost:
		return "Use pulse_control with target_host matching this host. Commands run directly."
	default:
		return "Use pulse_control with target_host matching the resource hostname."
	}
}

// FormatCLIAccess formats a CLI access string with actual values.
func FormatCLIAccess(resourceType ResourceType, vmid, containerName, namespace, podName string) string {
	template := GetCLIAccessTemplate(resourceType)
	result := template

	result = strings.ReplaceAll(result, "{vmid}", vmid)
	result = strings.ReplaceAll(result, "{container}", containerName)
	result = strings.ReplaceAll(result, "{namespace}", namespace)
	result = strings.ReplaceAll(result, "{pod}", podName)

	return result
}
