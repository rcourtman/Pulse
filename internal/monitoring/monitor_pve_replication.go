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

// pollReplicationStatus polls storage replication jobs for a PVE instance.
func (m *Monitor) pollReplicationStatus(ctx context.Context, instanceName string, client PVEClientInterface, vms []models.VM) {
	log.Debug().Str("instance", instanceName).Msg("Polling replication status")

	jobs, err := client.GetReplicationStatus(ctx)
	if err != nil {
		errMsg := err.Error()
		lowerMsg := strings.ToLower(errMsg)
		if strings.Contains(errMsg, "501") || strings.Contains(errMsg, "404") || strings.Contains(lowerMsg, "not implemented") || strings.Contains(lowerMsg, "not supported") {
			log.Debug().
				Str("instance", instanceName).
				Msg("Replication API not available on this Proxmox instance")
			m.state.UpdateReplicationJobsForInstance(instanceName, []models.ReplicationJob{})
			return
		}

		monErr := errors.WrapAPIError("get_replication_status", instanceName, err, 0)
		log.Warn().
			Err(monErr).
			Str("instance", instanceName).
			Msg("Failed to get replication status")
		return
	}

	if len(jobs) == 0 {
		m.state.UpdateReplicationJobsForInstance(instanceName, []models.ReplicationJob{})
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

	m.state.UpdateReplicationJobsForInstance(instanceName, converted)
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
