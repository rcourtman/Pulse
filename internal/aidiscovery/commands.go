package aidiscovery

import (
	"fmt"
	"strings"
)

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
func BuildLXCCommand(vmid string, cmd string) string {
	return fmt.Sprintf("pct exec %s -- sh -c %q", vmid, cmd)
}

// BuildVMCommand wraps a command for execution in a VM via QEMU guest agent.
// Note: This requires the guest agent to be running.
func BuildVMCommand(vmid string, cmd string) string {
	// For VMs, we use qm guest exec which requires the guest agent
	return fmt.Sprintf("qm guest exec %s -- sh -c %q", vmid, cmd)
}

// BuildDockerCommand wraps a command for execution in a Docker container.
func BuildDockerCommand(containerName string, cmd string) string {
	return fmt.Sprintf("docker exec %s sh -c %q", containerName, cmd)
}

// BuildNestedDockerCommand builds a command to run inside Docker on a VM/LXC.
func BuildNestedDockerCommand(vmid string, isLXC bool, containerName string, cmd string) string {
	dockerCmd := BuildDockerCommand(containerName, cmd)
	if isLXC {
		return BuildLXCCommand(vmid, dockerCmd)
	}
	return BuildVMCommand(vmid, dockerCmd)
}

// BuildK8sCommand builds a command to run in a Kubernetes pod.
func BuildK8sCommand(namespace, podName, containerName, cmd string) string {
	if containerName != "" {
		return fmt.Sprintf("kubectl exec -n %s %s -c %s -- sh -c %q", namespace, podName, containerName, cmd)
	}
	return fmt.Sprintf("kubectl exec -n %s %s -- sh -c %q", namespace, podName, cmd)
}

// GetCLIAccessTemplate returns a CLI access template for a resource type.
func GetCLIAccessTemplate(resourceType ResourceType) string {
	switch resourceType {
	case ResourceTypeLXC:
		return "pct exec {vmid} -- {command}"
	case ResourceTypeVM:
		return "qm guest exec {vmid} -- {command}"
	case ResourceTypeDocker:
		return "docker exec {container} {command}"
	case ResourceTypeDockerLXC:
		return "pct exec {vmid} -- docker exec {container} {command}"
	case ResourceTypeDockerVM:
		return "qm guest exec {vmid} -- docker exec {container} {command}"
	case ResourceTypeK8s:
		return "kubectl exec -n {namespace} {pod} -- {command}"
	case ResourceTypeHost:
		return "{command}"
	default:
		return "{command}"
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
