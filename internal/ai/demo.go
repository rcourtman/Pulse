package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockmode"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// IsDemoMode returns true if mock/demo mode is enabled.
// Delegates to the build-tag-gated mockmode package (always false in release).
func IsDemoMode() bool {
	return mockmode.IsEnabled()
}

// Demo provider identity reported by readiness and preflight surfaces while
// mock mode simulates Patrol. Matches the model name GenerateDemoAIResponse
// reports for assistant chat.
const (
	DemoPatrolProvider = "demo"
	DemoPatrolModel    = "demo-model"
)

// IsDemoRuntimeIntended reports whether this process is meant to serve demo
// fixtures even when they are not enabled yet. Release demo instances boot
// with mock fixtures off until the license sync validates the demo_fixtures
// entitlement, so boot-time lifecycle decisions (whether to start the patrol
// loop at all) must look at operator intent, not the current gate state.
func IsDemoRuntimeIntended() bool {
	return IsDemoMode() || mockmode.IsRequestedFromEnv()
}

const demoFindingIDPrefix = "demo-"

// isDemoFindingID reports whether a finding was synthesized by the demo
// patrol cycle. Real findings use hex hash IDs, so the prefix cannot collide.
func isDemoFindingID(id string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(id)), demoFindingIDPrefix)
}

// runDemoPatrolCycle executes one simulated patrol pass for demo/mock mode.
// It never contacts a provider: findings are synthesized from the current
// mock infrastructure state so every finding points at a resource the demo
// actually shows, then a matching run record is appended to check history.
// Repeated cycles refresh the same finding IDs (heartbeat semantics) and
// auto-resolve demo findings whose underlying mock condition went away.
// Returns true once a cycle has been recorded; false when it could not run
// yet (mock disabled again, another run in flight, or mock state not
// generated yet).
func (p *PatrolService) runDemoPatrolCycle(trigger TriggerReason) bool {
	return p.runDemoPatrolCycleWithStart(trigger, nil)
}

func (p *PatrolService) runDemoPatrolCycleWithStart(trigger TriggerReason, acceptedStart *patrolRunStart) bool {
	if p == nil || p.findings == nil || p.runHistoryStore == nil {
		return false
	}
	if !IsDemoMode() {
		return false
	}
	runStart := acceptedStart
	if runStart == nil {
		var accepted bool
		runStart, accepted = p.beginRun("full")
		if !accepted {
			return false
		}
	}
	defer p.endRun()

	p.mu.Lock()
	// Mock mode can enable after the boot-time config snapshot was taken
	// (release demo instances authorize fixtures via the license sync).
	// Repair the snapshot so status surfaces report an active patrol instead
	// of the pre-demo disabled/blocked state.
	p.config.Enabled = true
	p.config.RuntimeBlockedReason = ""
	p.config.RuntimeBlockedCause = PatrolFailureCauseNone
	cfg := p.config
	p.mu.Unlock()
	p.clearBlockedReason()

	now := time.Now()
	state := p.currentPatrolRuntimeState()
	findings := synthesizeDemoPatrolFindings(state, now)

	counts := patrolRuntimeCountResources(state)
	nodesChecked, guestsChecked, dockerChecked, storageChecked := 0, 0, 0, 0
	hostsChecked, trueNASChecked, pbsChecked, pmgChecked, kubernetesChecked := 0, 0, 0, 0, 0
	if cfg.AnalyzeNodes {
		nodesChecked = counts.nodes
	}
	if cfg.AnalyzeGuests {
		guestsChecked = counts.guests
	}
	if cfg.AnalyzeDocker {
		dockerChecked = counts.docker
	}
	if cfg.AnalyzeStorage {
		storageChecked = counts.storage
	}
	if cfg.AnalyzeHosts {
		hostsChecked = counts.hosts
		trueNASChecked = counts.truenas
	}
	if cfg.AnalyzePBS {
		pbsChecked = counts.pbs
	}
	if cfg.AnalyzePMG {
		pmgChecked = counts.pmg
	}
	if cfg.AnalyzeKubernetes {
		kubernetesChecked = counts.kubernetes
	}
	resourceCount := nodesChecked + guestsChecked + dockerChecked + storageChecked +
		hostsChecked + trueNASChecked + pbsChecked + pmgChecked + kubernetesChecked

	if resourceCount == 0 && len(findings) == 0 {
		// Mock state has not been generated yet (early boot). The warmup
		// retry or the next scheduled cycle will populate the surface once
		// state exists.
		log.Debug().Msg("demo patrol: no mock state available yet, skipping cycle")
		if acceptedStart != nil {
			p.recordAcceptedRunFailure(runStart, trigger,
				"Patrol demo state unavailable",
				"Pulse accepted the run, but demo infrastructure state was not available.")
			return true
		}
		return false
	}

	// Demo findings are deliberately not persisted, so a restart re-adds
	// them as store-new. Findings already attested by prior demo run records
	// still count as existing: the history says they were open before.
	knownIDs := make(map[string]bool)
	for _, run := range p.runHistoryStore.GetAll() {
		if !isDemoPatrolRunRecord(run) {
			continue
		}
		for _, id := range run.FindingIDs {
			knownIDs[id] = true
		}
	}

	newCount, existingCount := 0, 0
	currentIDs := make(map[string]bool, len(findings))
	findingIDs := make([]string, 0, len(findings))
	for _, f := range findings {
		currentIDs[f.ID] = true
		if p.findings.Add(f) && !knownIDs[f.ID] {
			newCount++
		} else {
			existingCount++
		}
		findingIDs = append(findingIDs, f.ID)
	}

	// Auto-resolve demo findings whose mock condition no longer holds.
	resolvedCount := 0
	for _, active := range p.findings.GetActive(FindingSeverityInfo) {
		if active == nil || !isDemoFindingID(active.ID) || currentIDs[active.ID] {
			continue
		}
		if p.findings.Resolve(active.ID, true) {
			resolvedCount++
		}
	}

	// A simulated pass stands in for a healthy provider-backed run: clear any
	// stale "Provider analysis error" finding left by real runs that failed
	// before mock mode enabled.
	p.resolvePatrolRuntimeFailureFinding("demo_patrol_cycle")

	summary := p.findings.GetSummary()
	findingsSummaryStr := "All healthy"
	status := "healthy"
	if summary.Critical+summary.Warning > 0 {
		parts := []string{}
		if summary.Critical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", summary.Critical))
		}
		if summary.Warning > 0 {
			parts = append(parts, fmt.Sprintf("%d warning", summary.Warning))
		}
		findingsSummaryStr = joinParts(parts)
		if summary.Critical > 0 {
			status = "critical"
		} else {
			status = "issues_found"
		}
	}

	// Presentational duration and token counts: deterministic, scaled to the
	// size of the mock fleet so the history reads like a real analysis pass.
	duration := time.Duration(35+resourceCount%40) * time.Second
	runID := fmt.Sprintf("demo-run-%d", now.UnixNano())
	startedAt := now.Add(-duration)
	if acceptedStart != nil {
		runID = runStart.id
		startedAt = runStart.startedAt
		duration = now.Sub(startedAt)
	}
	record := PatrolRunRecord{
		ID:                runID,
		Source:            PatrolRunSourceDemo,
		StartedAt:         startedAt,
		CompletedAt:       now,
		Duration:          duration,
		DurationMs:        duration.Milliseconds(),
		Type:              "patrol",
		TriggerReason:     string(trigger),
		ResourcesChecked:  resourceCount,
		NodesChecked:      nodesChecked,
		GuestsChecked:     guestsChecked,
		DockerChecked:     dockerChecked,
		StorageChecked:    storageChecked,
		HostsChecked:      hostsChecked,
		TrueNASChecked:    trueNASChecked,
		PBSChecked:        pbsChecked,
		PMGChecked:        pmgChecked,
		KubernetesChecked: kubernetesChecked,
		NewFindings:       newCount,
		ExistingFindings:  existingCount,
		ResolvedFindings:  resolvedCount,
		FindingsSummary:   findingsSummaryStr,
		FindingIDs:        findingIDs,
		Status:            status,
		InputTokens:       1200 + 42*resourceCount,
		OutputTokens:      380 + 60*len(findings),
		AIAnalysis: fmt.Sprintf(
			"Reviewed %d resources across %d nodes, %d guests, %d Docker hosts, and %d storage pools. %s.",
			resourceCount, nodesChecked, guestsChecked, dockerChecked, storageChecked, findingsSummaryStr),
	}

	if p.backfillDemoRunHistory(record, now) {
		// The seeded history attests these findings first appeared earlier,
		// so the current run reports them as still open, not newly found.
		record.ExistingFindings += record.NewFindings
		record.NewFindings = 0
	}
	p.runHistoryStore.Add(record)

	p.mu.Lock()
	p.lastActivity = now
	p.lastFullPatrol = now
	p.lastDuration = duration
	p.resourcesChecked = resourceCount
	p.errorCount = 0
	p.mu.Unlock()

	log.Info().
		Int("resources", resourceCount).
		Int("findings", len(findings)).
		Int("new", newCount).
		Int("resolved", resolvedCount).
		Str("trigger", string(trigger)).
		Msg("demo patrol: completed simulated patrol cycle")
	return true
}

// startDemoPatrolWarmup runs the first simulated patrol cycle once mock state
// is available. Mock fixtures generate concurrently with startup, so an
// immediate cycle can race an empty snapshot; retry briefly instead of
// waiting for the next scheduled tick, which the dev quota guard may never
// start.
func (p *PatrolService) startDemoPatrolWarmup() {
	if p == nil {
		return
	}
	go func() {
		for attempt := 0; attempt < 24; attempt++ {
			if p.runDemoPatrolCycle(TriggerReasonStartup) {
				return
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

// backfillDemoRunHistory seeds plausible past check history the first time a
// demo cycle runs, so the run history panel is not empty on a fresh demo
// instance. No-op once any demo record exists. Returns true when it seeded.
func (p *PatrolService) backfillDemoRunHistory(template PatrolRunRecord, now time.Time) bool {
	for _, run := range p.runHistoryStore.GetAll() {
		if isDemoPatrolRunRecord(run) {
			return false
		}
	}

	// Oldest first: the store prepends, so adding in reverse keeps newest on top.
	for i := 10; i >= 1; i-- {
		start := now.Add(-time.Duration(i*6) * time.Hour)
		duration := time.Duration(38+(i*7)%31) * time.Second
		rec := template
		rec.ID = fmt.Sprintf("demo-run-%d", start.UnixNano())
		rec.StartedAt = start
		rec.CompletedAt = start.Add(duration)
		rec.Duration = duration
		rec.DurationMs = duration.Milliseconds()
		rec.TriggerReason = string(TriggerReasonScheduled)
		rec.NewFindings = 0
		rec.ResolvedFindings = 0
		rec.ExistingFindings = len(template.FindingIDs)
		rec.AIAnalysis = ""
		switch {
		case i >= 8:
			// The oldest runs predate the current issues.
			rec.ExistingFindings = 0
			rec.FindingIDs = nil
			rec.FindingsSummary = "All healthy"
			rec.Status = "healthy"
		case i == 7:
			// The run where the current issues first appeared.
			rec.NewFindings = len(template.FindingIDs)
			rec.ExistingFindings = 0
		}
		p.runHistoryStore.Add(rec)
	}
	return true
}

// synthesizeDemoPatrolFindings derives demo findings from the live mock
// state. Every finding references a resource that exists in the demo dataset
// and quotes its actual observed values, so the patrol surface stays
// consistent with what the rest of the demo UI shows. The demo scenario data
// pins a set of deliberate anomalies (an offline Docker edge host, a degraded
// PBS instance, unhealthy containers), which gives the surface a stable core
// of findings across cycles.
func synthesizeDemoPatrolFindings(snap patrolRuntimeState, now time.Time) []*Finding {
	var findings []*Finding

	// Offline Docker host: the demo scenario keeps one edge host offline.
	for _, host := range snap.DockerHosts {
		if !strings.EqualFold(strings.TrimSpace(host.Status), "offline") {
			continue
		}
		name := demoDockerHostDisplayName(host)
		findings = append(findings, &Finding{
			ID:           demoFindingID("docker-host-offline", host.Hostname),
			Key:          fmt.Sprintf("docker:%s:offline", host.Hostname),
			Severity:     FindingSeverityCritical,
			Category:     FindingCategoryReliability,
			ResourceID:   host.ID,
			ResourceName: name,
			ResourceType: "docker-host",
			Node:         host.Hostname,
			Title:        fmt.Sprintf("Docker host %q is offline", name),
			Description: fmt.Sprintf(
				"The Pulse agent on %s has stopped reporting. %d containers on this host are unmonitored until the agent reconnects.",
				name, len(host.Containers)),
			Impact: "Workloads on this host may be down, and Pulse cannot see them while the agent is offline.",
			Recommendation: "Confirm the machine has power and network connectivity, then check the agent with " +
				"systemctl status pulse-agent and the Docker daemon with systemctl status docker. " +
				"Recent agent logs (journalctl -u pulse-agent) usually show why reporting stopped.",
			Evidence:   fmt.Sprintf("status=offline containers=%d last_seen=%s", len(host.Containers), host.LastSeen.UTC().Format(time.RFC3339)),
			DetectedAt: now.Add(-95 * time.Minute),
			LastSeenAt: now,
			Source:     "patrol",
		})
		break
	}

	// Highest-utilization storage pool, quoting its actual numbers.
	var topStorage *models.Storage
	for i := range snap.Storage {
		s := &snap.Storage[i]
		if s.Total <= 0 || !s.Enabled || strings.EqualFold(strings.TrimSpace(s.Status), "offline") {
			continue
		}
		if topStorage == nil || s.Usage > topStorage.Usage {
			topStorage = s
		}
	}
	if topStorage != nil && topStorage.Usage >= 60 {
		severity := FindingSeverityWarning
		urgency := "Growth at this level is worth reviewing before it becomes urgent."
		if topStorage.Usage >= 85 {
			severity = FindingSeverityCritical
			urgency = "New snapshots, backups, and guest disk growth are at risk once the pool fills."
		}
		location := topStorage.Node
		if topStorage.Shared || location == "" {
			location = "the cluster"
		}
		findings = append(findings, &Finding{
			ID:           demoFindingID("storage-capacity", topStorage.ID),
			Key:          fmt.Sprintf("storage:%s:capacity", topStorage.Name),
			Severity:     severity,
			Category:     FindingCategoryCapacity,
			ResourceID:   topStorage.ID,
			ResourceName: topStorage.Name,
			ResourceType: "storage",
			Node:         topStorage.Node,
			Title:        fmt.Sprintf("Storage %q is %.0f%% full", topStorage.Name, topStorage.Usage),
			Description: fmt.Sprintf(
				"Storage %s on %s has %s free of %s. It is the fullest pool in this environment. %s",
				topStorage.Name, location, formatDemoBytes(topStorage.Free), formatDemoBytes(topStorage.Total), urgency),
			Recommendation: "Review the largest guest disks and stale snapshots on this pool and reclaim what is no longer needed. " +
				"Longer term, plan an expansion or migrate guests to a less utilized pool.",
			Evidence:   fmt.Sprintf("usage=%.1f%% used=%s total=%s", topStorage.Usage, formatDemoBytes(topStorage.Used), formatDemoBytes(topStorage.Total)),
			DetectedAt: now.Add(-26 * time.Hour),
			LastSeenAt: now,
			Source:     "patrol",
		})
	}

	// Degraded backup server: the demo scenario keeps one PBS instance degraded.
	for _, pbs := range snap.PBSInstances {
		statusBad := !strings.EqualFold(strings.TrimSpace(pbs.Status), "online")
		healthBad := pbs.ConnectionHealth != "" && !strings.EqualFold(strings.TrimSpace(pbs.ConnectionHealth), "healthy")
		if !statusBad && !healthBad {
			continue
		}
		findings = append(findings, &Finding{
			ID:           demoFindingID("pbs-degraded", pbs.Name),
			Key:          fmt.Sprintf("pbs:%s:connection", pbs.Name),
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryBackup,
			ResourceID:   pbs.ID,
			ResourceName: pbs.Name,
			ResourceType: "pbs",
			Node:         pbs.Name,
			Title:        fmt.Sprintf("Backup server %q connection is degraded", pbs.Name),
			Description: fmt.Sprintf(
				"Pulse is reaching backup server %s intermittently. New backup and verify jobs cannot be confirmed while the connection is degraded.",
				pbs.Name),
			Impact: "If backups stop landing here, restore points age silently until the connection recovers.",
			Recommendation: "Check the PBS service and datastore status on the server, confirm the network path from the " +
				"Proxmox nodes, and review the most recent backup task logs for timeouts.",
			Evidence:   fmt.Sprintf("status=%s connection=%s", pbs.Status, pbs.ConnectionHealth),
			DetectedAt: now.Add(-7 * time.Hour),
			LastSeenAt: now,
			Source:     "patrol",
		})
		break
	}

	// Unhealthy or restart-looping containers on online Docker hosts.
	containerFindings := 0
	for _, host := range snap.DockerHosts {
		if strings.EqualFold(strings.TrimSpace(host.Status), "offline") {
			continue
		}
		hostName := demoDockerHostDisplayName(host)
		for i := range host.Containers {
			c := host.Containers[i]
			unhealthy := strings.EqualFold(strings.TrimSpace(c.Health), "unhealthy")
			flapping := c.RestartCount >= 3 && strings.EqualFold(strings.TrimSpace(c.State), "running")
			if !unhealthy && !flapping {
				continue
			}
			title := fmt.Sprintf("Container %q is failing its health check", c.Name)
			description := fmt.Sprintf(
				"Container %s on %s reports an unhealthy status from its Docker health check.",
				c.Name, hostName)
			if !unhealthy {
				title = fmt.Sprintf("Container %q is restarting repeatedly", c.Name)
				description = fmt.Sprintf(
					"Container %s on %s has restarted %d times. Frequent restarts usually point at a crash loop or resource limits.",
					c.Name, hostName, c.RestartCount)
			}
			findings = append(findings, &Finding{
				ID:           demoFindingID("docker-container", host.Hostname+"-"+c.Name),
				Key:          fmt.Sprintf("docker:%s/%s:health", host.Hostname, c.Name),
				Severity:     FindingSeverityWarning,
				Category:     FindingCategoryReliability,
				ResourceID:   c.ID,
				ResourceName: c.Name,
				ResourceType: "app-container",
				Node:         host.Hostname,
				Title:        title,
				Description:  description,
				Recommendation: fmt.Sprintf(
					"Check recent logs with docker logs %s --tail 100, inspect the health check definition with docker inspect, "+
						"and watch docker stats for memory pressure.", c.Name),
				Evidence:   fmt.Sprintf("health=%s state=%s restarts=%d image=%s", c.Health, c.State, c.RestartCount, c.Image),
				DetectedAt: now.Add(-4 * time.Hour),
				LastSeenAt: now,
				Source:     "patrol",
			})
			containerFindings++
			if containerFindings >= 2 {
				break
			}
		}
		if containerFindings >= 2 {
			break
		}
	}

	// Degraded mail gateway: the demo scenario keeps one PMG instance degraded.
	for _, pmg := range snap.PMGInstances {
		if strings.EqualFold(strings.TrimSpace(pmg.Status), "online") {
			continue
		}
		findings = append(findings, &Finding{
			ID:           demoFindingID("pmg-degraded", pmg.Name),
			Key:          fmt.Sprintf("pmg:%s:connection", pmg.Name),
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryReliability,
			ResourceID:   pmg.ID,
			ResourceName: pmg.Name,
			ResourceType: "pmg",
			Node:         pmg.Name,
			Title:        fmt.Sprintf("Mail gateway %q is degraded", pmg.Name),
			Description: fmt.Sprintf(
				"Mail gateway %s is responding intermittently. Mail flow statistics and quarantine data may be stale until it recovers.",
				pmg.Name),
			Recommendation: "Confirm the PMG services are running on the host, check connectivity from Pulse to the " +
				"PMG API endpoint, and review the PMG task log for postfix or clamav errors.",
			Evidence:   fmt.Sprintf("status=%s", pmg.Status),
			DetectedAt: now.Add(-3 * time.Hour),
			LastSeenAt: now,
			Source:     "patrol",
		})
		break
	}

	// Highest-memory running guest, only when genuinely high.
	type guestRef struct {
		id, name, node, kind string
		usage                float64
		used, total          int64
	}
	var topGuest *guestRef
	consider := func(id, name, node, kind string, status string, mem models.Memory) {
		if !strings.EqualFold(strings.TrimSpace(status), "running") || mem.Total <= 0 {
			return
		}
		if topGuest == nil || mem.Usage > topGuest.usage {
			topGuest = &guestRef{id: id, name: name, node: node, kind: kind, usage: mem.Usage, used: mem.Used, total: mem.Total}
		}
	}
	for i := range snap.VMs {
		vm := &snap.VMs[i]
		consider(vm.ID, vm.Name, vm.Node, "VM", vm.Status, vm.Memory)
	}
	for i := range snap.Containers {
		ct := &snap.Containers[i]
		consider(ct.ID, ct.Name, ct.Node, "Container", ct.Status, ct.Memory)
	}
	if topGuest != nil && topGuest.usage >= 85 {
		findings = append(findings, &Finding{
			ID:           demoFindingID("guest-memory", topGuest.id),
			Key:          fmt.Sprintf("guest:%s:memory", topGuest.id),
			Severity:     FindingSeverityWarning,
			Category:     FindingCategoryPerformance,
			ResourceID:   topGuest.id,
			ResourceName: topGuest.name,
			ResourceType: strings.ToLower(topGuest.kind),
			Node:         topGuest.node,
			Title:        fmt.Sprintf("%s %q memory usage at %.0f%%", topGuest.kind, topGuest.name, topGuest.usage),
			Description: fmt.Sprintf(
				"%s %s on %s is using %s of its %s allocation. Sustained usage at this level risks swapping and degraded performance under load.",
				topGuest.kind, topGuest.name, topGuest.node, formatDemoBytes(topGuest.used), formatDemoBytes(topGuest.total)),
			Recommendation: "Increase the memory allocation if the host has capacity, or check the workload for a leak " +
				"and review application cache settings to reduce steady-state usage.",
			Evidence:   fmt.Sprintf("memory=%.1f%% used=%s total=%s", topGuest.usage, formatDemoBytes(topGuest.used), formatDemoBytes(topGuest.total)),
			DetectedAt: now.Add(-5 * time.Hour),
			LastSeenAt: now,
			Source:     "patrol",
		})
	}

	return findings
}

func demoFindingID(kind, key string) string {
	return demoFindingIDPrefix + kind + "-" + demoSlug(key)
}

func demoSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func demoDockerHostDisplayName(host models.DockerHost) string {
	if name := strings.TrimSpace(host.CustomDisplayName); name != "" {
		return name
	}
	if name := strings.TrimSpace(host.DisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(host.Hostname)
}

func formatDemoBytes(bytes int64) string {
	const gib = float64(1 << 30)
	value := float64(bytes) / gib
	if value >= 1024 {
		return fmt.Sprintf("%.1f TiB", value/1024)
	}
	if value >= 10 {
		return fmt.Sprintf("%.0f GiB", value)
	}
	return fmt.Sprintf("%.1f GiB", value)
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
KEY: agent:pve2:ssh
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
			"I'm the **Pulse Assistant**, here to help you manage your Proxmox infrastructure.\n\n" +
			"### What I Can Help With\n" +
			"- **Troubleshooting**: Diagnose disk, memory, CPU, and network issues\n" +
			"- **Backups**: Review backup status and configure schedules\n" +
			"- **Optimization**: Identify resource bottlenecks and optimization opportunities\n" +
			"- **Commands**: Execute maintenance commands on your nodes (with your approval)\n\n" +
			"### Try Asking\n" +
			"- \"Why is my disk filling up?\"\n" +
			"- \"Help me fix the backup failure on vm-102\"\n" +
			"- \"Check memory usage on pve1\"\n\n" +
			"*Note: This is a demo instance - command execution is disabled, but you can see how Pulse Assistant analysis works!*"

	default:
		response = "## Analysis\n\n" +
			"I can help you with that! In a production environment with Pulse Assistant configured, I would:\n\n" +
			"1. **Analyze** the current state of your infrastructure\n" +
			"2. **Identify** potential issues or optimization opportunities\n" +
			"3. **Recommend** specific actions with commands you can run\n" +
			"4. **Execute** approved commands directly on your nodes\n\n" +
			"### This Demo Shows\n" +
			"- How Pulse Assistant analysis works in Pulse\n" +
			"- The types of insights and recommendations you'll receive\n" +
			"- Command approvals with manual confirmation\n\n" +
			"To enable full Pulse Assistant capabilities in your own Pulse installation:\n" +
			"1. Go to **Settings → Pulse Assistant**\n" +
			"2. Add your API key (Anthropic, OpenAI, DeepSeek, or Ollama)\n" +
			"3. Enable Pulse Assistant features\n\n" +
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
