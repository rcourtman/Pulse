package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func matchesDatastoreExclude(datastoreName string, excludePatterns []string) bool {
	if len(excludePatterns) == 0 {
		return false
	}

	for _, pattern := range excludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Contains pattern: *substring*
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && len(pattern) > 2 {
			substring := strings.ToLower(pattern[1 : len(pattern)-1])
			if strings.Contains(strings.ToLower(datastoreName), substring) {
				return true
			}
			continue
		}

		// Suffix pattern: *suffix
		if strings.HasPrefix(pattern, "*") && len(pattern) > 1 {
			suffix := strings.ToLower(pattern[1:])
			if strings.HasSuffix(strings.ToLower(datastoreName), suffix) {
				return true
			}
			continue
		}

		// Prefix pattern: prefix*
		if strings.HasSuffix(pattern, "*") && len(pattern) > 1 {
			prefix := strings.ToLower(pattern[:len(pattern)-1])
			if strings.HasPrefix(strings.ToLower(datastoreName), prefix) {
				return true
			}
			continue
		}

		// Exact match (case-insensitive)
		if strings.EqualFold(pattern, datastoreName) {
			return true
		}
	}

	return false
}

func pbsDatastoreStatus(ds pbs.Datastore) string {
	status := strings.TrimSpace(ds.Status)
	if status != "" {
		return status
	}
	if strings.TrimSpace(ds.Error) != "" {
		return "unavailable"
	}
	return "available"
}

func pbsJobHealthEvidenceFromFacts(facts []pbs.JobHealthEvidence, observedAt time.Time) []models.PBSJobHealthEvidence {
	if len(facts) == 0 {
		return []models.PBSJobHealthEvidence{}
	}
	out := make([]models.PBSJobHealthEvidence, 0, len(facts))
	for _, fact := range facts {
		evidence := models.PBSJobHealthEvidence{
			ID:             fact.ID,
			Family:         fact.Family,
			Store:          fact.Store,
			Remote:         fact.Remote,
			Namespace:      fact.Namespace,
			Schedule:       fact.Schedule,
			Comment:        fact.Comment,
			Enabled:        fact.Enabled,
			LastRunState:   fact.LastRunState,
			LastRunUPID:    fact.LastRunUPID,
			LastRunEndtime: fact.LastRunEndtime,
			NextRun:        fact.NextRun,
			UPID:           fact.UPID,
			WorkerType:     fact.WorkerType,
			WorkerID:       fact.WorkerID,
			TaskStatus:     fact.TaskStatus,
			TaskStartTime:  fact.TaskStartTime,
			TaskEndTime:    fact.TaskEndTime,
			Confidence:     fact.Confidence,
			EvidenceSource: fact.EvidenceSource,
			EvidenceScope:  fact.EvidenceScope,
			Error:          fact.Error,
		}
		evidence.Freshness = pbsJobHealthFreshness(fact, observedAt)
		evidence.Posture, evidence.PostureReason = pbsJobHealthPosture(fact, evidence.Freshness)
		out = append(out, evidence)
	}
	return out
}

func pbsJobHealthFreshness(fact pbs.JobHealthEvidence, observedAt time.Time) models.PBSJobHealthFreshness {
	freshness := models.PBSJobHealthFreshness{
		ObservedAt: observedAt.UTC(),
		State:      "unknown",
	}
	if fact.LastRunEndtime > 0 {
		freshness.LastRunEndTime = time.Unix(fact.LastRunEndtime, 0).UTC()
	} else if fact.TaskEndTime > 0 {
		freshness.LastRunEndTime = time.Unix(fact.TaskEndTime, 0).UTC()
	}
	if fact.NextRun > 0 {
		freshness.NextRun = time.Unix(fact.NextRun, 0).UTC()
	}
	if !freshness.LastRunEndTime.IsZero() {
		freshness.AgeSeconds = int64(observedAt.Sub(freshness.LastRunEndTime).Seconds())
		if freshness.AgeSeconds < 0 {
			freshness.AgeSeconds = 0
		}
		freshness.State = "observed"
	}
	if !freshness.NextRun.IsZero() {
		if observedAt.After(freshness.NextRun) && (freshness.LastRunEndTime.IsZero() || freshness.LastRunEndTime.Before(freshness.NextRun)) {
			freshness.State = "overdue"
		} else if freshness.State == "unknown" {
			freshness.State = "scheduled"
		}
	}
	return freshness
}

func pbsJobHealthPosture(fact pbs.JobHealthEvidence, freshness models.PBSJobHealthFreshness) (string, string) {
	status := strings.ToLower(strings.TrimSpace(firstNonEmptyString(fact.TaskStatus, fact.LastRunState)))
	if fact.Error != "" {
		return "unknown", strings.TrimSpace(fact.Confidence)
	}
	if freshness.State == "overdue" {
		return "warning", "job-overdue"
	}
	switch {
	case status == "":
		return "unknown", "no-run-observed"
	case strings.Contains(status, "ok") || strings.Contains(status, "success"):
		return "healthy", "last-run-success"
	case strings.Contains(status, "running"):
		return "healthy", "running"
	case strings.Contains(status, "warn"):
		return "warning", "last-run-warning"
	case strings.Contains(status, "fail") || strings.Contains(status, "error"):
		return "critical", "last-run-failed"
	default:
		return "unknown", "last-run-state-" + strings.ReplaceAll(status, " ", "-")
	}
}

// pollPBSInstance polls a single PBS instance
func (m *Monitor) pollPBSInstance(ctx context.Context, instanceName string, client *pbs.Client) {
	start := time.Now()
	debugEnabled := logging.IsLevelEnabled(zerolog.DebugLevel)
	var pollErr error
	var pbsInst models.PBSInstance
	publishResult := false
	if m.pollMetrics != nil {
		m.pollMetrics.IncInFlight("pbs")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			pollErr = fmt.Errorf("panic while polling PBS instance %q: %v", instanceName, recovered)
			log.Error().
				Str("goroutine", fmt.Sprintf("pollPBSInstance-%s", instanceName)).
				Interface("panic", recovered).
				Stack().
				Msg("Recovered from panic in monitoring goroutine")
		}

		m.recordTaskResult(InstanceTypePBS, instanceName, pollErr)
		if m.stalenessTracker != nil {
			if pollErr == nil {
				m.stalenessTracker.UpdateSuccess(InstanceTypePBS, instanceName, nil)
			} else {
				m.stalenessTracker.UpdateError(InstanceTypePBS, instanceName)
			}
		}
		if publishResult {
			m.publishPBSConnectionOutcome(pbsInst, pollErr)
		}
		if m.pollMetrics != nil {
			m.pollMetrics.DecInFlight("pbs")
			m.pollMetrics.RecordResult(PollResult{
				InstanceName: instanceName,
				InstanceType: "pbs",
				Success:      pollErr == nil,
				Error:        pollErr,
				StartTime:    start,
				EndTime:      time.Now(),
			})
		}
	}()

	// Get instance config
	var instanceCfg *config.PBSInstance
	for _, cfg := range m.config.PBSInstances {
		if cfg.Name == instanceName {
			instanceCfg = &cfg
			if debugEnabled {
				log.Debug().
					Str("instance", instanceName).
					Bool("monitorDatastores", cfg.MonitorDatastores).
					Msg("Found PBS instance config")
			}
			break
		}
	}
	if instanceCfg == nil {
		pollErr = fmt.Errorf("PBS instance config %q not found", instanceName)
		log.Error().Str("instance", instanceName).Msg("PBS instance config not found")
		return
	}
	if instanceCfg.Disabled {
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("Skipping PBS poll: instance is paused")
		}
		return
	}

	// Initialize PBS instance with default values
	pbsInst = models.PBSInstance{
		ID:               "pbs-" + instanceName,
		Name:             instanceName,
		Host:             instanceCfg.Host,
		GuestURL:         instanceCfg.GuestURL,
		Status:           "offline",
		Version:          "unknown",
		ConnectionHealth: "unhealthy",
		LastSeen:         time.Now(),
	}
	publishResult = true

	// Check if context is cancelled after the configured instance projection is
	// available, so cancellation still publishes the same failed poll outcome.
	select {
	case <-ctx.Done():
		pollErr = ctx.Err()
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("polling cancelled")
		}
		return
	default:
	}

	if debugEnabled {
		log.Debug().Str("instance", instanceName).Msg("polling PBS instance")
	}

	// Try to get version first
	version, versionErr := client.GetVersion(ctx)
	if versionErr == nil {
		pbsInst.Status = "online"
		pbsInst.Version = version.Version
		pbsInst.ConnectionHealth = "healthy"
		m.resetAuthFailures(instanceName, "pbs")

		if debugEnabled {
			log.Debug().
				Str("instance", instanceName).
				Str("version", version.Version).
				Bool("monitorDatastores", instanceCfg.MonitorDatastores).
				Msg("PBS version retrieved successfully")
		}
	} else {
		if debugEnabled {
			log.Debug().Err(versionErr).Str("instance", instanceName).Msg("failed to get PBS version, trying fallback")
		}

		// Use parent context for proper cancellation chain
		ctx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
		defer cancel2()
		_, datastoreErr := client.GetDatastores(ctx2)
		if datastoreErr == nil {
			pbsInst.Status = "online"
			pbsInst.Version = "connected"
			pbsInst.ConnectionHealth = "healthy"
			m.resetAuthFailures(instanceName, "pbs")

			log.Info().
				Str("instance", instanceName).
				Msg("PBS connected (version unavailable but datastores accessible)")
		} else {
			pbsInst.Status = "offline"
			pbsInst.ConnectionHealth = "error"
			connectionErr := versionErr
			if errors.IsAuthError(datastoreErr) {
				connectionErr = datastoreErr
			}
			monErr := errors.WrapConnectionError("get_pbs_version", instanceName, connectionErr)
			pollErr = monErr
			log.Error().Err(monErr).Str("instance", instanceName).Msg("failed to connect to PBS")

			if errors.IsAuthError(versionErr) || errors.IsAuthError(datastoreErr) {
				m.recordAuthFailure(instanceName, "pbs")
			}
			return
		}
	}

	// Get node status (CPU, memory, etc.)
	nodeStatus, err := client.GetNodeStatus(ctx)
	if err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("could not get PBS node status (may need Sys.Audit permission)")
		}
	} else if nodeStatus != nil {
		pbsInst.CPU = nodeStatus.CPU
		if nodeStatus.Memory.Total > 0 {
			pbsInst.Memory = float64(nodeStatus.Memory.Used) / float64(nodeStatus.Memory.Total) * 100
			pbsInst.MemoryUsed = nodeStatus.Memory.Used
			pbsInst.MemoryTotal = nodeStatus.Memory.Total
		}
		pbsInst.Uptime = nodeStatus.Uptime

		log.Debug().
			Str("instance", instanceName).
			Float64("cpu", pbsInst.CPU).
			Float64("memory", pbsInst.Memory).
			Int64("uptime", pbsInst.Uptime).
			Msg("PBS node status retrieved")
	}

	// Poll datastores if enabled
	if instanceCfg.MonitorDatastores {
		datastores, err := client.GetDatastores(ctx)
		if err != nil {
			monErr := errors.WrapAPIError("get_datastores", instanceName, err, 0)
			log.Error().Err(monErr).Str("instance", instanceName).Msg("failed to get datastores")
		} else {
			log.Info().
				Str("instance", instanceName).
				Int("count", len(datastores)).
				Msg("Got PBS datastores")

			for _, ds := range datastores {
				// Skip excluded datastores (for removable/unmounted datastores)
				if matchesDatastoreExclude(ds.Store, instanceCfg.ExcludeDatastores) {
					log.Debug().
						Str("instance", instanceName).
						Str("datastore", ds.Store).
						Msg("Skipping excluded datastore")
					continue
				}
				total := ds.Total
				if total == 0 && ds.TotalSpace > 0 {
					total = ds.TotalSpace
				}
				used := ds.Used
				if used == 0 && ds.UsedSpace > 0 {
					used = ds.UsedSpace
				}
				avail := ds.Avail
				if avail == 0 && ds.AvailSpace > 0 {
					avail = ds.AvailSpace
				}
				if total == 0 && used > 0 && avail > 0 {
					total = used + avail
				}

				log.Debug().
					Str("store", ds.Store).
					Int64("total", total).
					Int64("used", used).
					Int64("avail", avail).
					Int64("orig_total", ds.Total).
					Int64("orig_total_space", ds.TotalSpace).
					Msg("PBS datastore details")

				modelDS := models.PBSDatastore{
					Name:                ds.Store,
					Total:               total,
					Used:                used,
					Free:                avail,
					Usage:               safePercentage(float64(used), float64(total)),
					Status:              pbsDatastoreStatus(ds),
					Error:               ds.Error,
					DeduplicationFactor: ds.DeduplicationFactor,
				}

				namespaces, err := client.ListNamespaces(ctx, ds.Store, "", 0)
				if err != nil {
					log.Warn().Err(err).
						Str("instance", instanceName).
						Str("datastore", ds.Store).
						Msg("Failed to list namespaces")
				} else {
					for _, ns := range namespaces {
						nsPath := ns.NS
						if nsPath == "" {
							nsPath = ns.Path
						}
						if nsPath == "" {
							nsPath = ns.Name
						}

						modelNS := models.PBSNamespace{
							Path:   nsPath,
							Parent: ns.Parent,
							Depth:  strings.Count(nsPath, "/"),
						}
						modelDS.Namespaces = append(modelDS.Namespaces, modelNS)
					}

					hasRoot := false
					for _, ns := range modelDS.Namespaces {
						if ns.Path == "" {
							hasRoot = true
							break
						}
					}
					if !hasRoot {
						modelDS.Namespaces = append([]models.PBSNamespace{{Path: "", Depth: 0}}, modelDS.Namespaces...)
					}
				}

				pbsInst.Datastores = append(pbsInst.Datastores, modelDS)
			}
		}
	}

	if instanceCfg.MonitorBackups || instanceCfg.MonitorSyncJobs || instanceCfg.MonitorVerifyJobs || instanceCfg.MonitorPruneJobs || instanceCfg.MonitorGarbageJobs {
		datastoreNames := make([]string, 0, len(pbsInst.Datastores))
		for _, ds := range pbsInst.Datastores {
			datastoreNames = append(datastoreNames, ds.Name)
		}
		jobFacts, err := client.GetJobHealthEvidence(ctx, datastoreNames, pbs.JobHealthOptions{
			MonitorBackups:     instanceCfg.MonitorBackups,
			MonitorSyncJobs:    instanceCfg.MonitorSyncJobs,
			MonitorVerifyJobs:  instanceCfg.MonitorVerifyJobs,
			MonitorPruneJobs:   instanceCfg.MonitorPruneJobs,
			MonitorGarbageJobs: instanceCfg.MonitorGarbageJobs,
		})
		if err != nil {
			log.Warn().Err(err).
				Str("instance", instanceName).
				Msg("Failed to collect PBS job health evidence")
		}
		pbsInst.JobHealthEvidence = pbsJobHealthEvidenceFromFacts(jobFacts, time.Now())
	}

	// Convert PBS datastores to Storage entries for unified storage view
	if len(pbsInst.Datastores) > 0 && instanceCfg.MonitorDatastores {
		var pbsStorages []models.Storage
		for _, ds := range pbsInst.Datastores {
			// Create a storage entry for this PBS datastore
			storageID := fmt.Sprintf("pbs-%s-%s", instanceName, ds.Name)
			pbsStorage := models.Storage{
				ID: storageID,
				// The unified registry and thresholds UI key this datastore by
				// its canonical "<instance-id>/<name>" ID; carrying it as an
				// alias lets alert override lookups accept either format (#1591).
				AliasIDs: []string{pbsInst.ID + "/" + ds.Name},
				Name:     ds.Name,
				Node:     instanceName, // Use PBS instance name as "node"
				Instance: "pbs-" + instanceName,
				Type:     "pbs",
				Status:   ds.Status,
				Total:    ds.Total,
				Used:     ds.Used,
				Free:     ds.Free,
				Usage:    ds.Usage,
				Content:  "backup", // PBS datastores are for backups
				Shared:   true,     // PBS datastores are typically shared/network storage
				Enabled:  true,
				Active:   pbsInst.Status == "online",
				LastSeen: pbsInst.LastSeen,
			}
			pbsStorages = append(pbsStorages, pbsStorage)
		}
		m.state.UpdateStorageForInstance("pbs-"+instanceName, pbsStorages)
		log.Debug().
			Str("instance", instanceName).
			Int("storageEntries", len(pbsStorages)).
			Msg("Added PBS datastores to unified storage view")
	}

	// Poll backups if enabled
	if instanceCfg.MonitorBackups {
		if len(pbsInst.Datastores) == 0 {
			log.Debug().
				Str("instance", instanceName).
				Msg("No PBS datastores available for backup polling")
		} else if !m.config.EnableBackupPolling {
			log.Debug().
				Str("instance", instanceName).
				Msg("Skipping PBS backup polling - globally disabled")
		} else {
			now := time.Now()

			m.mu.Lock()
			lastPoll := m.lastPBSBackupPoll[instanceName]
			if m.pbsBackupPollers == nil {
				m.pbsBackupPollers = make(map[string]bool)
			}
			inProgress := m.pbsBackupPollers[instanceName]
			m.mu.Unlock()

			shouldPoll, reason, newLast := m.shouldRunBackupPoll(lastPoll, now)
			if !shouldPoll {
				if reason != "" {
					log.Debug().
						Str("instance", instanceName).
						Str("reason", reason).
						Msg("Skipping PBS backup polling this cycle")
				}
			} else if inProgress {
				log.Debug().
					Str("instance", instanceName).
					Msg("PBS backup polling already in progress")
			} else {
				datastoreSnapshot := make([]models.PBSDatastore, len(pbsInst.Datastores))
				copy(datastoreSnapshot, pbsInst.Datastores)

				// Atomically check and set poller flag
				m.mu.Lock()
				if m.pbsBackupPollers[instanceName] {
					// Race: another goroutine started between our check and lock
					m.mu.Unlock()
					log.Debug().
						Str("instance", instanceName).
						Msg("PBS backup polling started by another goroutine")
				} else {
					m.pbsBackupPollers[instanceName] = true
					m.lastPBSBackupPoll[instanceName] = newLast
					m.mu.Unlock()

					go func(ds []models.PBSDatastore, inst string, start time.Time, pbsClient *pbs.Client) {
						defer recoverFromPanic(fmt.Sprintf("pollPBSBackups-%s", inst))
						defer func() {
							m.mu.Lock()
							delete(m.pbsBackupPollers, inst)
							m.lastPBSBackupPoll[inst] = time.Now()
							m.mu.Unlock()
						}()

						log.Info().
							Str("instance", inst).
							Int("datastores", len(ds)).
							Msg("Starting background PBS backup polling")

						// The per-cycle ctx is canceled as soon as the main polling loop finishes,
						// so derive the backup poll context from the long-lived runtime context instead.
						parentCtx := m.getRuntimeContext()
						if parentCtx == nil {
							parentCtx = context.Background()
						}
						backupCtx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
						defer cancel()

						m.pollPBSBackups(backupCtx, inst, pbsClient, ds)

						log.Info().
							Str("instance", inst).
							Dur("duration", time.Since(start)).
							Msg("Completed background PBS backup polling")
					}(datastoreSnapshot, instanceName, now, client)
				}
			}
		}
	} else {
		log.Debug().
			Str("instance", instanceName).
			Msg("PBS backup monitoring disabled")
	}
}

// publishPBSConnectionOutcome projects the final connectivity result into every
// legacy PBS health consumer. Inventory collection may be partial, but the
// connection map, dashboard resource, and legacy alert evaluator must all use
// the same poll error that is recorded in the scheduler ledger.
func (m *Monitor) publishPBSConnectionOutcome(instance models.PBSInstance, pollErr error) {
	connected := pollErr == nil
	if connected {
		instance.Status = "online"
		instance.ConnectionHealth = "healthy"
	} else {
		instance.Status = "offline"
		instance.ConnectionHealth = "error"
	}

	m.setProviderConnectionHealth(InstanceTypePBS, instance.Name, connected)
	m.state.UpdatePBSInstance(instance)
	log.Info().
		Str("instance", instance.Name).
		Str("id", instance.ID).
		Bool("connected", connected).
		Int("datastores", len(instance.Datastores)).
		Msg("PBS instance updated in state")

	if m.alertManager != nil {
		m.alertManager.CheckPBS(instance)
	}
}

// pollPMGInstance polls a single Proxmox Mail Gateway instance
func (m *Monitor) pollPMGInstance(ctx context.Context, instanceName string, client *pmg.Client) {
	defer recoverFromPanic(fmt.Sprintf("pollPMGInstance-%s", instanceName))

	start := time.Now()
	debugEnabled := logging.IsLevelEnabled(zerolog.DebugLevel)
	var pollErr error
	if m.pollMetrics != nil {
		m.pollMetrics.IncInFlight("pmg")
		defer m.pollMetrics.DecInFlight("pmg")
		defer func() {
			m.pollMetrics.RecordResult(PollResult{
				InstanceName: instanceName,
				InstanceType: "pmg",
				Success:      pollErr == nil,
				Error:        pollErr,
				StartTime:    start,
				EndTime:      time.Now(),
			})
		}()
	}
	if m.stalenessTracker != nil {
		defer func() {
			if pollErr == nil {
				m.stalenessTracker.UpdateSuccess(InstanceTypePMG, instanceName, nil)
			} else {
				m.stalenessTracker.UpdateError(InstanceTypePMG, instanceName)
			}
		}()
	}
	defer func() {
		m.recordTaskResult(InstanceTypePMG, instanceName, pollErr)
	}()

	select {
	case <-ctx.Done():
		pollErr = ctx.Err()
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("PMG polling cancelled by context")
		}
		return
	default:
	}

	if debugEnabled {
		log.Debug().Str("instance", instanceName).Msg("polling PMG instance")
	}

	var instanceCfg *config.PMGInstance
	for idx := range m.config.PMGInstances {
		if m.config.PMGInstances[idx].Name == instanceName {
			instanceCfg = &m.config.PMGInstances[idx]
			break
		}
	}

	if instanceCfg == nil {
		log.Error().Str("instance", instanceName).Msg("PMG instance config not found")
		pollErr = fmt.Errorf("pmg instance config not found for %s", instanceName)
		return
	}
	if instanceCfg.Disabled {
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("Skipping PMG poll: instance is paused")
		}
		return
	}

	now := time.Now()
	pmgInst := models.PMGInstance{
		ID:               "pmg-" + instanceName,
		Name:             instanceName,
		Host:             instanceCfg.Host,
		GuestURL:         instanceCfg.GuestURL,
		Status:           "offline",
		ConnectionHealth: "unhealthy",
		LastSeen:         now,
		LastUpdated:      now,
	}

	version, err := client.GetVersion(ctx)
	if err != nil {
		monErr := errors.WrapConnectionError("pmg_get_version", instanceName, err)
		pollErr = monErr
		log.Error().Err(monErr).Str("instance", instanceName).Msg("failed to connect to PMG instance")
		m.setProviderConnectionHealth(InstanceTypePMG, instanceName, false)
		m.state.UpdatePMGInstance(pmgInst)

		// Check PMG offline status against alert thresholds
		if m.alertManager != nil {
			m.alertManager.CheckPMG(pmgInst)
		}

		if errors.IsAuthError(err) {
			m.recordAuthFailure(instanceName, "pmg")
		}
		return
	}

	pmgInst.Status = "online"
	pmgInst.ConnectionHealth = "healthy"
	if version != nil {
		pmgInst.Version = strings.TrimSpace(version.Version)
	}
	m.setProviderConnectionHealth(InstanceTypePMG, instanceName, true)
	m.resetAuthFailures(instanceName, "pmg")

	cluster, err := client.GetClusterStatus(ctx, true)
	if err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("failed to retrieve PMG cluster status")
		}
	}

	backupNodes := make(map[string]struct{})

	if len(cluster) > 0 {
		nodes := make([]models.PMGNodeStatus, 0, len(cluster))
		for _, entry := range cluster {
			status := strings.ToLower(strings.TrimSpace(entry.Type))
			if status == "" {
				status = "online"
			}
			node := models.PMGNodeStatus{
				Name:   entry.Name,
				Status: status,
				Role:   entry.Type,
			}

			backupNodes[entry.Name] = struct{}{}

			// Fetch queue status for this node
			if queueData, qErr := client.GetQueueStatus(ctx, entry.Name); qErr != nil {
				if debugEnabled {
					log.Debug().Err(qErr).
						Str("instance", instanceName).
						Str("node", entry.Name).
						Msg("Failed to fetch PMG queue status")
				}
			} else if queueData != nil {
				total := queueData.Active.Int64() + queueData.Deferred.Int64() + queueData.Hold.Int64() + queueData.Incoming.Int64()
				node.QueueStatus = &models.PMGQueueStatus{
					Active:    queueData.Active.Int(),
					Deferred:  queueData.Deferred.Int(),
					Hold:      queueData.Hold.Int(),
					Incoming:  queueData.Incoming.Int(),
					Total:     int(total),
					OldestAge: queueData.OldestAge.Int64(),
					UpdatedAt: time.Now(),
				}
			}

			nodes = append(nodes, node)
		}
		pmgInst.Nodes = nodes
	}

	if len(backupNodes) == 0 {
		trimmed := strings.TrimSpace(instanceName)
		if trimmed != "" {
			backupNodes[trimmed] = struct{}{}
		}
	}

	pmgBackups := make([]models.PMGBackup, 0)
	seenBackupIDs := make(map[string]struct{})

	for nodeName := range backupNodes {
		if ctx.Err() != nil {
			break
		}

		backups, backupErr := client.ListBackups(ctx, nodeName)
		if backupErr != nil {
			if debugEnabled {
				log.Debug().Err(backupErr).
					Str("instance", instanceName).
					Str("node", nodeName).
					Msg("Failed to list PMG configuration backups")
			}
			continue
		}

		for _, b := range backups {
			timestamp := b.Timestamp.Int64()
			backupTime := time.Unix(timestamp, 0)
			backupID := fmt.Sprintf("pmg-%s-%s-%d", instanceName, nodeName, timestamp)
			if _, exists := seenBackupIDs[backupID]; exists {
				continue
			}
			seenBackupIDs[backupID] = struct{}{}
			pmgBackups = append(pmgBackups, models.PMGBackup{
				ID:         backupID,
				Instance:   instanceName,
				Node:       nodeName,
				Filename:   b.Filename,
				BackupTime: backupTime,
				Size:       b.Size.Int64(),
			})
		}
	}

	if debugEnabled {
		log.Debug().
			Str("instance", instanceName).
			Int("backupCount", len(pmgBackups)).
			Msg("PMG backups polled")
	}

	if stats, err := client.GetMailStatistics(ctx, ""); err != nil {
		log.Warn().Err(err).Str("instance", instanceName).Msg("failed to fetch PMG mail statistics")
	} else if stats != nil {
		pmgInst.MailStats = &models.PMGMailStats{
			Timeframe:            "day",
			CountTotal:           stats.Count.Float64(),
			CountIn:              stats.CountIn.Float64(),
			CountOut:             stats.CountOut.Float64(),
			SpamIn:               stats.SpamIn.Float64(),
			SpamOut:              stats.SpamOut.Float64(),
			VirusIn:              stats.VirusIn.Float64(),
			VirusOut:             stats.VirusOut.Float64(),
			BouncesIn:            stats.BouncesIn.Float64(),
			BouncesOut:           stats.BouncesOut.Float64(),
			BytesIn:              stats.BytesIn.Float64(),
			BytesOut:             stats.BytesOut.Float64(),
			GreylistCount:        stats.GreylistCount.Float64(),
			JunkIn:               stats.JunkIn.Float64(),
			AverageProcessTimeMs: stats.AvgProcessSec.Float64() * 1000,
			RBLRejects:           stats.RBLRejects.Float64(),
			PregreetRejects:      stats.Pregreet.Float64(),
			UpdatedAt:            time.Now(),
		}
	}

	if counts, err := client.GetMailCount(ctx, 86400); err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("failed to fetch PMG mail count data")
		}
	} else if len(counts) > 0 {
		points := make([]models.PMGMailCountPoint, 0, len(counts))
		for _, entry := range counts {
			ts := time.Unix(entry.Time.Int64(), 0)
			points = append(points, models.PMGMailCountPoint{
				Timestamp:   ts,
				Count:       entry.Count.Float64(),
				CountIn:     entry.CountIn.Float64(),
				CountOut:    entry.CountOut.Float64(),
				SpamIn:      entry.SpamIn.Float64(),
				SpamOut:     entry.SpamOut.Float64(),
				VirusIn:     entry.VirusIn.Float64(),
				VirusOut:    entry.VirusOut.Float64(),
				RBLRejects:  entry.RBLRejects.Float64(),
				Pregreet:    entry.PregreetReject.Float64(),
				BouncesIn:   entry.BouncesIn.Float64(),
				BouncesOut:  entry.BouncesOut.Float64(),
				Greylist:    entry.GreylistCount.Float64(),
				Index:       entry.Index.Int(),
				Timeframe:   "hour",
				WindowStart: ts,
			})
		}
		pmgInst.MailCount = points
	}

	if scores, err := client.GetSpamScores(ctx); err != nil {
		if debugEnabled {
			log.Debug().Err(err).Str("instance", instanceName).Msg("failed to fetch PMG spam score distribution")
		}
	} else if len(scores) > 0 {
		buckets := make([]models.PMGSpamBucket, 0, len(scores))
		for _, bucket := range scores {
			buckets = append(buckets, models.PMGSpamBucket{
				Score: bucket.Level,
				Count: float64(bucket.Count.Int()),
			})
		}
		pmgInst.SpamDistribution = buckets
	}

	quarantine := models.PMGQuarantineTotals{}
	if spamStatus, err := client.GetQuarantineStatus(ctx, "spam"); err == nil && spamStatus != nil {
		quarantine.Spam = int(spamStatus.Count.Int64())
	}
	if virusStatus, err := client.GetQuarantineStatus(ctx, "virus"); err == nil && virusStatus != nil {
		quarantine.Virus = int(virusStatus.Count.Int64())
	}
	pmgInst.Quarantine = &quarantine

	if instanceCfg.MonitorDomainStats {
		if domains, err := client.ListRelayDomains(ctx); err != nil {
			if debugEnabled {
				log.Debug().Err(err).Str("instance", instanceName).Msg("failed to fetch PMG relay domains")
			}
		} else if len(domains) > 0 {
			relayDomains := make([]models.PMGRelayDomain, 0, len(domains))
			for _, domain := range domains {
				relayDomains = append(relayDomains, models.PMGRelayDomain{
					Domain:  strings.TrimSpace(domain.Domain),
					Comment: strings.TrimSpace(domain.Comment),
				})
			}
			pmgInst.RelayDomains = relayDomains
		}

		end := time.Now()
		start := end.Add(-24 * time.Hour)
		if stats, err := client.GetDomainStatistics(ctx, start.Unix(), end.Unix()); err != nil {
			if debugEnabled {
				log.Debug().Err(err).Str("instance", instanceName).Msg("failed to fetch PMG domain statistics")
			}
		} else if len(stats) > 0 {
			domainStats := make([]models.PMGDomainStat, 0, len(stats))
			for _, entry := range stats {
				d := strings.TrimSpace(entry.Domain)
				if d == "" {
					continue
				}
				domainStats = append(domainStats, models.PMGDomainStat{
					Domain:     d,
					MailCount:  entry.Count.Float64(),
					SpamCount:  entry.SpamCount.Float64(),
					VirusCount: entry.VirusCount.Float64(),
					Bytes:      entry.Bytes.Float64(),
				})
			}
			pmgInst.DomainStats = domainStats
			pmgInst.DomainStatsAsOf = end
		}
	}

	m.state.UpdatePMGBackups(instanceName, pmgBackups)
	m.state.UpdatePMGInstance(pmgInst)
	log.Info().
		Str("instance", instanceName).
		Str("status", pmgInst.Status).
		Int("nodes", len(pmgInst.Nodes)).
		Msg("PMG instance updated in state")

	// Check PMG metrics against alert thresholds
	if m.alertManager != nil {
		m.alertManager.CheckPMG(pmgInst)
	}
}

// GetState returns the current state
