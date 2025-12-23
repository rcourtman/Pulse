package ai

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// IsDemoMode returns true if mock/demo mode is enabled
// This checks the PULSE_MOCK_MODE env var used by the mock data system
func IsDemoMode() bool {
	return strings.EqualFold(os.Getenv("PULSE_MOCK_MODE"), "true")
}

// mockResourcePatterns contains name patterns that indicate mock/demo resources
var mockResourcePatterns = []string{
	"pve1", "pve2", "pve3", "pve4", "pve5", "pve6", "pve7", // mock PVE nodes
	"mock-cluster", "mock-",                                 // generic mock prefixes
	"Ceres", "Atlas", "Nova", "Orion", "Vega", "Rigel",     // mock host agent names
	"docker-host-", "k8s-cluster-",                          // mock Docker/K8s names
	"demo-",                                                 // demo prefixes
}

// IsMockResource returns true if the resource name/ID appears to be mock data
// This is used to filter out mock resources from heuristic analysis when not in demo mode
func IsMockResource(resourceID, resourceName, node string) bool {
	// If we're in demo mode, don't filter anything - we want mock resources
	if IsDemoMode() {
		return false
	}
	
	// Check against mock patterns
	toCheck := []string{resourceID, resourceName, node}
	for _, value := range toCheck {
		if value == "" {
			continue
		}
		for _, pattern := range mockResourcePatterns {
			if strings.Contains(value, pattern) {
				return true
			}
		}
	}
	return false
}


// InjectDemoFindings populates the patrol service with realistic mock findings
// This is used for demo instances to showcase AI features without actual AI API calls
func (p *PatrolService) InjectDemoFindings() {
	if p == nil || p.findings == nil {
		return
	}

	log.Info().Msg("Demo mode: Injecting mock AI patrol findings")

	now := time.Now()

	// Create realistic demo findings
	demoFindings := []*Finding{
		{
			ID:           "demo-storage-critical",
			Key:          "storage:local-zfs:capacity",
			Severity:     FindingSeverityCritical,
			Category:     FindingCategoryCapacity,
			Title:        "ZFS pool 'local-zfs' is 94% full",
			Description:  "Storage pool local-zfs on pve1 has only 47GB remaining out of 750GB. At current growth rate, pool will be full in approximately 5 days.",
			ResourceID:   "storage/pve1/local-zfs",
			ResourceName: "local-zfs",
			ResourceType: "storage",
			Node:         "pve1",
			Recommendation: `**Immediate actions:**
1. Identify large files: ` + "`zfs list -o name,used,refer -t all | sort -k2 -h | tail -20`" + `
2. Check for orphaned VM disks: ` + "`pvesm list local-zfs | grep -v 'vm-'`" + `
3. Remove old snapshots: ` + "`zfs list -t snapshot -o name,used | sort -k2 -h`" + `

**Long-term:**
- Add additional storage or migrate VMs to other pools
- Enable ZFS compression if not already enabled`,
			DetectedAt: now.Add(-2 * time.Hour),
			LastSeenAt:  now.Add(-5 * time.Minute),
			TimesRaised: 3,
			Source:      "patrol",
		},
		{
			ID:           "demo-memory-warning",
			Key:          "guest:vm-102:memory",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryPerformance,
			Title:        "VM 'jellyfin' memory usage at 91%",
			Description:  "jellyfin (VM 102) on pve2 is consistently using 14.5GB of its 16GB allocated memory. Swapping may occur under load, degrading performance.",
			ResourceID:   "qemu/102",
			ResourceName: "jellyfin",
			ResourceType: "qemu",
			Node:         "pve2",
			Recommendation: `**Options to consider:**
1. Increase VM memory allocation to 20GB if host has capacity
2. Check for memory leaks in Jellyfin: restart the service
3. Limit transcoding to reduce memory pressure
4. Review Jellyfin cache settings in the dashboard`,
			DetectedAt: now.Add(-6 * time.Hour),
			LastSeenAt:  now.Add(-10 * time.Minute),
			TimesRaised: 5,
			Source:      "patrol",
		},
		{
			ID:           "demo-backup-warning",
			Key:          "guest:vm-105:backup",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryBackup,
			Title:        "Container 'postgres' hasn't been backed up in 8 days",
			Description:  "The postgres container (CT 105) last successful backup was 8 days ago. Your backup schedule targets daily backups.",
			ResourceID:   "lxc/105",
			ResourceName: "postgres",
			ResourceType: "lxc",
			Node:         "pve1",
			Recommendation: `**Investigate:**
1. Check PBS backup job status: ` + "`pvesh get /nodes/pve1/tasks --typefilter vzdump`" + `
2. Verify PBS datastore connectivity
3. Check for backup job errors in the Proxmox datacenter backup view

**Manual backup:**
` + "`vzdump 105 --storage pbs --mode snapshot`",
			DetectedAt: now.Add(-24 * time.Hour),
			LastSeenAt:  now.Add(-15 * time.Minute),
			TimesRaised: 2,
			Source:      "patrol",
		},
		{
			ID:           "demo-cpu-warning",
			Key:          "node:pve3:cpu",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryPerformance,
			Title:        "Node 'pve3' sustained high CPU (87%)",
			Description:  "Node pve3 has maintained CPU usage above 85% for the past 2 hours. This may indicate over-provisioning or a runaway process.",
			ResourceID:   "node/pve3",
			ResourceName: "pve3",
			ResourceType: "node",
			Node:         "pve3",
			Recommendation: `**Diagnose:**
1. Check top processes: ` + "`ssh pve3 'top -bn1 | head -20'`" + `
2. Identify VM CPU usage: ` + "`pvesh get /nodes/pve3/qemu --output-format json | jq '.[] | {name, cpu}'`" + `

**Consider:**
- Live-migrate a VM to another node: ` + "`qm migrate <vmid> pve1 --online`" + `
- Set CPU limits on high-usage VMs`,
			DetectedAt: now.Add(-2 * time.Hour),
			LastSeenAt:  now.Add(-8 * time.Minute),
			TimesRaised: 4,
			Source:      "patrol",
		},
		{
			ID:           "demo-docker-warning",
			Key:          "docker:portainer:container-restart",
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryReliability,
			Title:        "Docker container 'uptime-kuma' restarting frequently",
			Description:  "The uptime-kuma container on docker-host-1 has restarted 7 times in the past 24 hours. This may indicate configuration issues or resource constraints.",
			ResourceID:   "docker/docker-host-1/uptime-kuma",
			ResourceName: "uptime-kuma",
			ResourceType: "docker_container",
			Node:         "docker-host-1",
			Recommendation: `**Check logs:**
` + "`docker logs uptime-kuma --tail 100`" + `

**Common causes:**
- OOM kills: check ` + "`docker stats uptime-kuma`" + `
- Configuration errors in environment variables
- Database corruption (check data volume)`,
			DetectedAt: now.Add(-12 * time.Hour),
			LastSeenAt:  now.Add(-20 * time.Minute),
			TimesRaised: 3,
			Source:      "patrol",
		},
	}

	// Add findings to the store
	for _, f := range demoFindings {
		p.findings.Add(f)
	}

	// Also add some demo patrol run history
	p.injectDemoRunHistory()

	log.Info().Int("findings_count", len(demoFindings)).Msg("Demo mode: Mock findings injected")
}

// injectDemoRunHistory adds realistic patrol run history for the demo
func (p *PatrolService) injectDemoRunHistory() {
	if p.runHistoryStore == nil {
		return
	}

	now := time.Now()

	// Clear existing history first to avoid duplicates on restart
	// (Assuming we can't easily clear, we'll just generate new IDs based on time to be idempotent-ish)

	// Create a realistic schedule: every 6 hours for the last 3 days
	var demoRuns []PatrolRunRecord

	// 1. Most recent run (just happened)
	demoRuns = append(demoRuns, PatrolRunRecord{
		ID:               fmt.Sprintf("demo-run-%d", now.Unix()),
		StartedAt:        now.Add(-15 * time.Minute),
		CompletedAt:      now.Add(-14*time.Minute + 15*time.Second),
		Duration:         75 * time.Second,
		Type:             "patrol",
		ResourcesChecked: 47,
		NodesChecked:     5,
		GuestsChecked:    32,
		DockerChecked:    8,
		StorageChecked:   6,
		NewFindings:      0,
		ExistingFindings: 5,
		ResolvedFindings: 0,
		FindingsSummary:  "2 critical, 3 warnings",
		FindingIDs:       []string{"demo-storage-critical", "9e1eb083b7109506", "demo-memory-warning", "demo-backup-warning", "demo-cpu-warning"},
		Status:           "issues_found",
		InputTokens:      4250,
		OutputTokens:     890,
	})

	// 2. Scheduled runs (every 6 hours)
	for i := 1; i <= 12; i++ {
		offset := time.Duration(i*6) * time.Hour
		startTime := now.Add(-offset)
		
		// Vary the duration slightly
		duration := time.Duration(40 + (i % 30)) * time.Second
		
		// Outcomes vary over time
		var summary string
		var status string
		var newFindings, existingFindings, resolvedFindings int
		var findingIDs []string

		if i <= 4 { // Last 24h - steady state of issues
			summary = "2 critical, 3 warnings"
			status = "issues_found"
			existingFindings = 5
			findingIDs = []string{"demo-storage-critical", "9e1eb083b7109506", "demo-memory-warning", "demo-backup-warning", "demo-cpu-warning"}
		} else if i == 5 { // 30h ago - one issue appeared
			summary = "1 new critical, 3 warnings"
			status = "issues_found"
			newFindings = 1
			existingFindings = 3
			findingIDs = []string{"demo-storage-critical", "demo-memory-warning", "demo-backup-warning", "demo-cpu-warning"}
		} else if i <= 10 { // 2-3 days ago - fewer issues
			summary = "3 warnings"
			status = "issues_found"
			existingFindings = 3
			findingIDs = []string{"demo-memory-warning", "demo-backup-warning", "demo-cpu-warning"}
		} else { // > 3 days ago - clean state
			summary = "No issues found"
			status = "healthy"
			resolvedFindings = 1
		}

		demoRuns = append(demoRuns, PatrolRunRecord{
			ID:               fmt.Sprintf("demo-run-%d", startTime.Unix()),
			StartedAt:        startTime,
			CompletedAt:      startTime.Add(duration),
			Duration:         duration,
			Type:             "patrol",
			ResourcesChecked: 47,
			NodesChecked:     5,
			GuestsChecked:    32,
			DockerChecked:    8,
			StorageChecked:   6,
			NewFindings:      newFindings,
			ExistingFindings: existingFindings,
			ResolvedFindings: resolvedFindings,
			FindingsSummary:  summary,
			FindingIDs:       findingIDs,
			Status:           status,
			InputTokens:      4000 + (i * 10),
			OutputTokens:     500 + (i * 5),
		})
	}

	for _, run := range demoRuns {
		p.runHistoryStore.Add(run)
	}
}

// GenerateDemoAIResponse returns a realistic mock AI response for demo mode
// This allows the demo server to showcase the AI Assistant without a real API key
func GenerateDemoAIResponse(prompt string) *ExecuteResponse {
	promptLower := strings.ToLower(prompt)

	// Determine response based on prompt content
	var response string

	switch {
	// Detect Patrol Analysis Request
	case strings.Contains(promptLower, "analyze") && strings.Contains(promptLower, "infrastructure"):
		response = `Based on the infrastructure data provided, I have identified the following issues:

[FINDING]
SEVERITY: critical
CATEGORY: capacity
KEY: storage:local-zfs:capacity
RESOURCE_ID: storage/pve1/local-zfs
RESOURCE_NAME: local-zfs
RESOURCE_TYPE: storage
NODE: pve1
TITLE: ZFS pool 'local-zfs' is 94% full
DESCRIPTION: Storage pool local-zfs on pve1 has critical capacity usage. This endangers VM stability and snapshot creation.
RECOMMENDATION: **Immediate actions:**
1. Identify large files: ` + "`zfs list -o name,used,refer -t all | sort -k2 -h`" + `
2. Check for orphaned VM disks
3. Remove old snapshots
EVIDENCE: Used: 94% (703GB/750GB)
[/FINDING]

[FINDING]
SEVERITY: warning
CATEGORY: performance
KEY: vm:vm-102:memory
RESOURCE_ID: qemu/102
RESOURCE_NAME: vm-database
RESOURCE_TYPE: vm
NODE: pve1
TITLE: High memory pressure on Database VM
DESCRIPTION: VM 'vm-database' is consistently using >90% RAM with significant swap activity.
RECOMMENDATION: Increase memory allocation to 16GB or enable ballooning.
EVIDENCE: Memory: 92% (7.4GB/8GB), Swap: 30%
[/FINDING]

[FINDING]
SEVERITY: warning
CATEGORY: security
KEY: host:pve2:ssh
RESOURCE_ID: node/pve2
RESOURCE_NAME: pve2
RESOURCE_TYPE: node
NODE: pve2
TITLE: Root SSH login enabled
DESCRIPTION: Root SSH login is enabled on node pve2, which is a security risk.
RECOMMENDATION: Disable root login in /etc/ssh/sshd_config and use key-based authentication.
EVIDENCE: PermitRootLogin yes found in config
[/FINDING]
`

	case strings.Contains(promptLower, "disk") || strings.Contains(promptLower, "storage") || strings.Contains(promptLower, "full"):
		response = "## Disk Usage Analysis\n\n" +
			"Based on the current metrics, I can see elevated disk usage. Here are my recommendations:\n\n" +
			"### Immediate Actions\n" +
			"1. **Check large files**: Run `du -sh /* | sort -rh | head -20` to find the largest directories\n" +
			"2. **Review logs**: Old logs often consume significant space. Check `/var/log` and consider log rotation\n" +
			"3. **Docker cleanup**: If using Docker, run `docker system prune -a` to remove unused images\n\n" +
			"### Long-term Solutions\n" +
			"- Set up automated log rotation with logrotate\n" +
			"- Configure alerts at 80% to catch issues before they become critical\n" +
			"- Consider expanding storage if usage is consistently high\n\n" +
			"Would you like me to help investigate any specific directory?"

	case strings.Contains(promptLower, "memory") || strings.Contains(promptLower, "ram") || strings.Contains(promptLower, "oom"):
		response = "## Memory Analysis\n\n" +
			"I can help analyze memory usage patterns. Here's what I recommend:\n\n" +
			"### Quick Diagnostics\n" +
			"1. **Current usage**: Check top consumers with `ps aux --sort=-%mem | head -15`\n" +
			"2. **Memory pressure**: Review `/proc/meminfo` for swap usage\n" +
			"3. **OOM events**: Check `dmesg | grep -i oom` for recent kills\n\n" +
			"### Optimization Tips\n" +
			"- Consider increasing VM memory allocation if the host has capacity\n" +
			"- Review application memory limits (especially for Java apps with -Xmx)\n" +
			"- Enable memory ballooning for better cluster-wide memory utilization\n\n" +
			"This is a **demo instance** - in production, I can run these commands directly on your nodes."

	case strings.Contains(promptLower, "backup") || strings.Contains(promptLower, "pbs"):
		response = "## Backup Status Review\n\n" +
			"Backups are critical for data protection. Here's my analysis:\n\n" +
			"### Recommended Checks\n" +
			"1. **PBS connectivity**: Verify the Proxmox Backup Server is reachable\n" +
			"2. **Job schedules**: Review backup job configurations in Datacenter → Backup\n" +
			"3. **Storage capacity**: Ensure PBS datastore has sufficient space for new backups\n\n" +
			"### Best Practices\n" +
			"- Schedule daily backups during low-usage periods\n" +
			"- Keep at least 7 daily + 4 weekly retention\n" +
			"- Test restores periodically to verify backup integrity\n\n" +
			"Would you like me to help configure backup schedules for specific VMs?"

	case strings.Contains(promptLower, "cpu") || strings.Contains(promptLower, "load") || strings.Contains(promptLower, "slow"):
		response = "## CPU/Performance Analysis\n\n" +
			"High CPU can indicate various issues. Let me help diagnose:\n\n" +
			"### Diagnostic Steps\n" +
			"1. **Top processes**: `top -bn1 | head -20` shows current CPU consumers\n" +
			"2. **Load average**: Check if load > number of CPU cores\n" +
			"3. **Per-VM usage**: Review individual guest CPU allocation\n\n" +
			"### Common Causes\n" +
			"- Overprovisioned guests (total vCPUs > physical cores)\n" +
			"- Runaway processes within VMs\n" +
			"- Background tasks like backups or replication\n\n" +
			"### Quick Wins\n" +
			"- Consider live-migrating busy VMs to less loaded nodes\n" +
			"- Set CPU limits on non-critical guests\n" +
			"- Schedule heavy tasks during off-peak hours"

	case strings.Contains(promptLower, "hello") || strings.Contains(promptLower, "hi") || strings.Contains(promptLower, "help"):
		response = "## Hello! 👋\n\n" +
			"I'm the **Pulse AI Assistant**, here to help you manage your Proxmox infrastructure.\n\n" +
			"### What I Can Help With\n" +
			"- **Troubleshooting**: Diagnose disk, memory, CPU, and network issues\n" +
			"- **Backups**: Review backup status and configure schedules\n" +
			"- **Optimization**: Identify resource bottlenecks and optimization opportunities\n" +
			"- **Commands**: Execute maintenance commands on your nodes (with your approval)\n\n" +
			"### Try Asking\n" +
			"- \"Why is my disk filling up?\"\n" +
			"- \"Help me fix the backup failure on vm-102\"\n" +
			"- \"Check memory usage on pve1\"\n\n" +
			"*Note: This is a demo instance - command execution is disabled, but you can see how the AI analysis works!*"

	default:
		response = "## Analysis\n\n" +
			"I can help you with that! In a production environment with AI configured, I would:\n\n" +
			"1. **Analyze** the current state of your infrastructure\n" +
			"2. **Identify** potential issues or optimization opportunities\n" +
			"3. **Recommend** specific actions with commands you can run\n" +
			"4. **Execute** approved commands directly on your nodes\n\n" +
			"### This Demo Shows\n" +
			"- How AI-powered analysis works in Pulse\n" +
			"- The types of insights and recommendations you'll receive\n" +
			"- Command suggestions with approval workflow\n\n" +
			"To enable full AI capabilities in your own Pulse installation:\n" +
			"1. Go to **Settings → AI Settings**\n" +
			"2. Add your API key (Anthropic, OpenAI, DeepSeek, or Ollama)\n" +
			"3. Enable AI features\n\n" +
			"*Visit [pulserelay.pro](https://pulserelay.pro) to get Pulse Pro!*"
	}

	return &ExecuteResponse{
		Content:      response,
		Model:        "demo-model",
		InputTokens:  150,
		OutputTokens: 400,
	}
}

// GenerateDemoAIStream acts like GenerateDemoAIResponse but streams content via callback
func GenerateDemoAIStream(prompt string, callback StreamCallback) (*ExecuteResponse, error) {
	resp := GenerateDemoAIResponse(prompt)
	
	// Simulate streaming by sending chunks
	chunkSize := 10
	content := resp.Content
	
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}
		
		callback(StreamEvent{
			Type: "content",
			Data: content[i:end],
		})
		
		// Tiny sleep to simulate generation speed
		time.Sleep(10 * time.Millisecond)
	}
	
	callback(StreamEvent{
		Type: "done",
	})
	
	return resp, nil
}
