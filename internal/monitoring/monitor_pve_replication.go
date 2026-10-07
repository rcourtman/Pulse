package monitoring

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rs/zerolog/log"
)

const pveReplicationPollTimeout = 10 * time.Second

// pveReplicationPoll owns one background observation. The claim is installed
// before dispatch and checked under m.mu together with the registered client at
// publication, so a retired/replaced instance cannot publish a late inventory.
type pveReplicationPoll struct {
	ctx    context.Context
	cancel context.CancelFunc
	client PVEClientInterface
	done   chan struct{}
}

func (m *Monitor) pollReplicationStatusAsync(instanceName string, client PVEClientInterface, vms []models.VM) {
	m.mu.Lock()
	if client == nil || m.pveClients[instanceName] != client {
		m.mu.Unlock()
		return
	}
	parentCtx := m.runtimeCtx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	if parentCtx.Err() != nil {
		m.mu.Unlock()
		return
	}
	if m.pveReplicationPolls == nil {
		m.pveReplicationPolls = make(map[string]*pveReplicationPoll)
	}
	if previous := m.pveReplicationPolls[instanceName]; previous != nil {
		if previous.client == client && previous.ctx.Err() == nil {
			m.mu.Unlock()
			return // A slow read must not stack up once per ordinary cycle.
		}
		previous.cancel()
	}
	ctx, cancel := context.WithTimeout(parentCtx, pveReplicationPollTimeout)
	owner := &pveReplicationPoll{ctx: ctx, cancel: cancel, client: client, done: make(chan struct{})}
	m.pveReplicationPolls[instanceName] = owner
	m.mu.Unlock()

	guestSnapshot := append([]models.VM(nil), vms...)
	if vms == nil {
		// Only replication identity is needed; do not clone guest enrichment or
		// rebuild all prior guest/agent indexes for each scheduled cycle.
		if state := m.GetUnifiedReadStateOrSnapshot(); state != nil {
			for _, vm := range state.VMs() {
				if vm != nil && vm.Instance() == instanceName {
					guestSnapshot = append(guestSnapshot, models.VM{VMID: vm.VMID(), Name: vm.Name(), Type: "qemu", Node: vm.Node()})
				}
			}
		}
	}
	go func() {
		defer func() {
			cancel()
			m.mu.Lock()
			if m.pveReplicationPolls[instanceName] == owner {
				delete(m.pveReplicationPolls, instanceName)
			}
			m.mu.Unlock()
			close(owner.done)
		}()
		defer recoverFromPanic(fmt.Sprintf("pollReplicationStatus-%s", instanceName))
		m.pollReplicationStatusOwned(ctx, instanceName, client, guestSnapshot, owner)
	}()
}

// publishReplicationJobs never treats an interrupted observation as an empty
// inventory, and never lets an old client overwrite its replacement's result.
func (m *Monitor) publishReplicationJobs(ctx context.Context, instanceName string, jobs []models.ReplicationJob, owner *pveReplicationPoll) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	if owner != nil && (m.pveReplicationPolls[instanceName] != owner || m.pveClients[instanceName] != owner.client) {
		return
	}
	m.state.UpdateReplicationJobsForInstance(instanceName, jobs)
}

// pollReplicationStatus polls storage replication jobs for a PVE instance.
func (m *Monitor) pollReplicationStatus(ctx context.Context, instanceName string, client PVEClientInterface, vms []models.VM) {
	m.pollReplicationStatusOwned(ctx, instanceName, client, vms, nil)
}

func (m *Monitor) pollReplicationStatusOwned(ctx context.Context, instanceName string, client PVEClientInterface, vms []models.VM, owner *pveReplicationPoll) {
	if ctx.Err() != nil {
		return
	}
	log.Debug().Str("instance", instanceName).Msg("polling replication status")

	jobs, err := client.GetReplicationStatus(ctx)
	if err != nil {
		errMsg := err.Error()
		lowerMsg := strings.ToLower(errMsg)
		if strings.Contains(errMsg, "501") || strings.Contains(errMsg, "404") || strings.Contains(lowerMsg, "not implemented") || strings.Contains(lowerMsg, "not supported") {
			log.Debug().
				Str("instance", instanceName).
				Msg("Replication API not available on this Proxmox instance")
			// An unavailable endpoint is not a successful empty inventory.
			return
		}

		monErr := errors.WrapAPIError("get_replication_status", instanceName, err, 0)
		log.Warn().
			Err(monErr).
			Str("instance", instanceName).
			Msg("Failed to get replication status")
		return
	}

	if ctx.Err() != nil {
		return
	}
	if len(jobs) == 0 {
		m.publishReplicationJobs(ctx, instanceName, []models.ReplicationJob{}, owner)
		return
	}

	vmByID := make(map[int]models.VM, len(vms))
	for _, vm := range vms {
		vmByID[vm.VMID] = vm
	}

	converted := make([]models.ReplicationJob, 0, len(jobs))
	now := time.Now()

	for idx, job := range jobs {
		guestID := job.GuestID
		if guestID == 0 {
			if parsed, err := strconv.Atoi(strings.TrimSpace(job.Guest)); err == nil {
				guestID = parsed
			}
		}

		guestName := ""
		guestType := ""
		guestNode := ""
		if guestID > 0 {
			if vm, ok := vmByID[guestID]; ok {
				guestName = vm.Name
				guestType = vm.Type
				guestNode = vm.Node
			}
		}
		if guestNode == "" {
			guestNode = strings.TrimSpace(job.Source)
		}

		sourceNode := strings.TrimSpace(job.Source)
		if sourceNode == "" {
			sourceNode = guestNode
		}

		targetNode := strings.TrimSpace(job.Target)

		var lastSyncTime *time.Time
		if job.LastSyncTime != nil && !job.LastSyncTime.IsZero() {
			t := job.LastSyncTime.UTC()
			lastSyncTime = &t
		}

		var nextSyncTime *time.Time
		if job.NextSyncTime != nil && !job.NextSyncTime.IsZero() {
			t := job.NextSyncTime.UTC()
			nextSyncTime = &t
		}

		lastSyncDurationHuman := job.LastSyncDurationHuman
		if lastSyncDurationHuman == "" && job.LastSyncDurationSeconds > 0 {
			lastSyncDurationHuman = formatSeconds(job.LastSyncDurationSeconds)
		}
		durationHuman := job.DurationHuman
		if durationHuman == "" && job.DurationSeconds > 0 {
			durationHuman = formatSeconds(job.DurationSeconds)
		}

		rateLimit := copyFloatPointer(job.RateLimitMbps)

		status := job.Status
		if status == "" {
			status = job.State
		}

		jobID := strings.TrimSpace(job.ID)
		if jobID == "" {
			if job.JobNumber > 0 && guestID > 0 {
				jobID = fmt.Sprintf("%d-%d", guestID, job.JobNumber)
			} else {
				jobID = fmt.Sprintf("job-%s-%d", instanceName, idx)
			}
		}

		uniqueID := fmt.Sprintf("%s-%s", instanceName, jobID)

		converted = append(converted, models.ReplicationJob{
			ID:                      uniqueID,
			Instance:                instanceName,
			JobID:                   jobID,
			JobNumber:               job.JobNumber,
			Guest:                   job.Guest,
			GuestID:                 guestID,
			GuestName:               guestName,
			GuestType:               guestType,
			GuestNode:               guestNode,
			SourceNode:              sourceNode,
			SourceStorage:           job.SourceStorage,
			TargetNode:              targetNode,
			TargetStorage:           job.TargetStorage,
			Schedule:                job.Schedule,
			Type:                    job.Type,
			Enabled:                 job.Enabled,
			State:                   job.State,
			Status:                  status,
			LastSyncStatus:          job.LastSyncStatus,
			LastSyncTime:            lastSyncTime,
			LastSyncUnix:            job.LastSyncUnix,
			LastSyncDurationSeconds: job.LastSyncDurationSeconds,
			LastSyncDurationHuman:   lastSyncDurationHuman,
			NextSyncTime:            nextSyncTime,
			NextSyncUnix:            job.NextSyncUnix,
			DurationSeconds:         job.DurationSeconds,
			DurationHuman:           durationHuman,
			FailCount:               job.FailCount,
			Error:                   job.Error,
			Comment:                 job.Comment,
			RemoveJob:               job.RemoveJob,
			RateLimitMbps:           rateLimit,
			LastPolled:              now,
		})
	}

	m.publishReplicationJobs(ctx, instanceName, converted, owner)
}

func formatSeconds(total int) string {
	if total <= 0 {
		return ""
	}
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
