package ai

import (
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// IsDemoMode returns true if mock/demo mode is enabled
// This checks the same MOCK_ENABLED env var used by the mock data system
func IsDemoMode() bool {
	return os.Getenv("MOCK_ENABLED") == "true" || os.Getenv("MOCK_ENABLED") == "1"
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

	// Create a few historical patrol runs
	demoRuns := []PatrolRunRecord{
		{
			ID:               "demo-run-1",
			StartedAt:        now.Add(-15 * time.Minute),
			CompletedAt:      now.Add(-14*time.Minute - 30*time.Second),
			Duration:         90 * time.Second,
			Type:             "patrol",
			ResourcesChecked: 47,
			NodesChecked:     5,
			GuestsChecked:    32,
			DockerChecked:    8,
			StorageChecked:   6,
			NewFindings:      1,
			ExistingFindings: 4,
			ResolvedFindings: 0,
			FindingsSummary:  "1 critical, 3 warnings",
			FindingIDs:       []string{"demo-storage-critical", "demo-memory-warning", "demo-backup-warning", "demo-cpu-warning"},
			Status:           "issues_found",
			InputTokens:      4250,
			OutputTokens:     890,
		},
		{
			ID:               "demo-run-2",
			StartedAt:        now.Add(-30 * time.Minute),
			CompletedAt:      now.Add(-29*time.Minute - 15*time.Second),
			Duration:         75 * time.Second,
			Type:             "patrol",
			ResourcesChecked: 47,
			NodesChecked:     5,
			GuestsChecked:    32,
			DockerChecked:    8,
			StorageChecked:   6,
			NewFindings:      2,
			ExistingFindings: 2,
			ResolvedFindings: 1,
			FindingsSummary:  "1 critical, 2 warnings",
			FindingIDs:       []string{"demo-storage-critical", "demo-memory-warning", "demo-backup-warning"},
			Status:           "issues_found",
			InputTokens:      4180,
			OutputTokens:     720,
		},
		{
			ID:               "demo-run-3",
			StartedAt:        now.Add(-45 * time.Minute),
			CompletedAt:      now.Add(-44*time.Minute - 45*time.Second),
			Duration:         45 * time.Second,
			Type:             "patrol",
			ResourcesChecked: 47,
			NodesChecked:     5,
			GuestsChecked:    32,
			DockerChecked:    8,
			StorageChecked:   6,
			NewFindings:      0,
			ExistingFindings: 2,
			ResolvedFindings: 0,
			FindingsSummary:  "2 warnings",
			FindingIDs:       []string{"demo-memory-warning", "demo-cpu-warning"},
			Status:           "issues_found",
			InputTokens:      4100,
			OutputTokens:     450,
		},
	}

	for _, run := range demoRuns {
		p.runHistoryStore.Add(run)
	}
}
