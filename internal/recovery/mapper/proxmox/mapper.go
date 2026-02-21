package proxmoxmapper

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// GuestInfo is best-effort metadata used to attach display identity and unified resource IDs.
// Keyed by guest source ID (see guestSourceID()).
type GuestInfo struct {
	SourceID     string
	ResourceType unifiedresources.ResourceType
	Name         string
}

type GuestCandidate struct {
	SourceID      string
	ResourceType  unifiedresources.ResourceType
	DisplayName   string
	InstanceName  string
	NodeName      string
	VMID          int
	BackupTypeKey string // "vm" or "ct" (PBS nomenclature)
}

func guestSourceID(instanceName, nodeName string, vmid int) string {
	return fmt.Sprintf("%s:%s:%d", strings.TrimSpace(instanceName), strings.TrimSpace(nodeName), vmid)
}

func guestLookupKey(instanceName, nodeName string, vmid int) string {
	return fmt.Sprintf("%s|%s|%d", strings.TrimSpace(instanceName), strings.TrimSpace(nodeName), vmid)
}

func proxmoxSubjectRef(resourceType unifiedresources.ResourceType, info GuestInfo, instanceName, nodeName string, vmid int, sourceID string) *recovery.ExternalRef {
	name := strings.TrimSpace(info.Name)
	if name == "" && vmid > 0 {
		name = fmt.Sprintf("%d", vmid)
	}
	refType := "proxmox-guest"
	if resourceType == unifiedresources.ResourceTypeVM {
		refType = "proxmox-vm"
	} else if resourceType == unifiedresources.ResourceTypeLXC {
		refType = "proxmox-lxc"
	}
	return &recovery.ExternalRef{
		Type:      refType,
		Namespace: strings.TrimSpace(instanceName),
		Name:      name,
		ID:        strings.TrimSpace(sourceID),
		Class:     strings.TrimSpace(nodeName),
	}
}

func outcomeFromTaskStatus(status string) recovery.Outcome {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "ok", "success", "completed":
		return recovery.OutcomeSuccess
	case "warning":
		return recovery.OutcomeWarning
	case "running", "active":
		return recovery.OutcomeRunning
	case "":
		return recovery.OutcomeUnknown
	default:
		if strings.Contains(s, "fail") || strings.Contains(s, "error") {
			return recovery.OutcomeFailed
		}
		return recovery.OutcomeUnknown
	}
}

func resourceTypeFromGuestType(guestType string) unifiedresources.ResourceType {
	switch strings.ToLower(strings.TrimSpace(guestType)) {
	case "qemu", "vm":
		return unifiedresources.ResourceTypeVM
	case "lxc", "ct":
		return unifiedresources.ResourceTypeLXC
	default:
		return ""
	}
}

func FromPVEGuestSnapshots(snapshots []models.GuestSnapshot, guestInfoByKey map[string]GuestInfo) []recovery.RecoveryPoint {
	if len(snapshots) == 0 {
		return nil
	}

	out := make([]recovery.RecoveryPoint, 0, len(snapshots))
	for _, snap := range snapshots {
		if strings.TrimSpace(snap.ID) == "" {
			continue
		}

		instanceName := strings.TrimSpace(snap.Instance)
		nodeName := strings.TrimSpace(snap.Node)
		vmid := snap.VMID

		var subjectRID string
		var subjectRef *recovery.ExternalRef

		if vmid > 0 && instanceName != "" && nodeName != "" {
			key := guestLookupKey(instanceName, nodeName, vmid)
			info := guestInfoByKey[key]
			sourceID := strings.TrimSpace(info.SourceID)
			if sourceID == "" {
				sourceID = guestSourceID(instanceName, nodeName, vmid)
			}
			resourceType := info.ResourceType
			if resourceType == "" {
				resourceType = resourceTypeFromGuestType(snap.Type)
			}
			if resourceType != "" {
				subjectRID = unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceProxmox, sourceID)
				subjectRef = proxmoxSubjectRef(resourceType, info, instanceName, nodeName, vmid, sourceID)
			}
		}

		t := snap.Time.UTC()

		out = append(out, recovery.RecoveryPoint{
			ID:                "pve-snapshot:" + strings.TrimSpace(snap.ID),
			Provider:          recovery.ProviderProxmoxPVE,
			Kind:              recovery.KindSnapshot,
			Mode:              recovery.ModeSnapshot,
			Outcome:           recovery.OutcomeSuccess,
			StartedAt:         &t,
			CompletedAt:       &t,
			SizeBytes:         sizePtr(snap.SizeBytes),
			SubjectResourceID: subjectRID,
			SubjectRef:        subjectRef,
			Details: map[string]any{
				"snapshotName": strings.TrimSpace(snap.Name),
				"description":  strings.TrimSpace(snap.Description),
				"parent":       strings.TrimSpace(snap.Parent),
				"vmState":      snap.VMState,
				"type":         strings.TrimSpace(snap.Type),
				"instance":     instanceName,
				"node":         nodeName,
				"vmid":         vmid,
			},
		})
	}

	return out
}

func FromPVEStorageBackups(backups []models.StorageBackup, guestInfoBySourceID map[string]GuestInfo) []recovery.RecoveryPoint {
	if len(backups) == 0 {
		return nil
	}

	out := make([]recovery.RecoveryPoint, 0, len(backups))
	for _, b := range backups {
		if strings.TrimSpace(b.ID) == "" {
			continue
		}

		instanceName := strings.TrimSpace(b.Instance)
		nodeName := strings.TrimSpace(b.Node)
		vmid := b.VMID

		var subjectRID string
		var subjectRef *recovery.ExternalRef

		if vmid > 0 && instanceName != "" && nodeName != "" {
			key := guestLookupKey(instanceName, nodeName, vmid)
			info := guestInfoBySourceID[key]
			sourceID := strings.TrimSpace(info.SourceID)
			if sourceID == "" {
				sourceID = guestSourceID(instanceName, nodeName, vmid)
			}
			resourceType := info.ResourceType
			if resourceType == "" {
				resourceType = resourceTypeFromGuestType(b.Type)
			}
			if resourceType != "" {
				subjectRID = unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceProxmox, sourceID)
				subjectRef = proxmoxSubjectRef(resourceType, info, instanceName, nodeName, vmid, sourceID)
			}
		}

		mode := recovery.ModeLocal
		if b.IsPBS {
			mode = recovery.ModeRemote
		}

		t := b.Time.UTC()
		protected := b.Protected
		immutable := &protected

		var verifiedPtr *bool
		if b.IsPBS {
			verified := b.Verified
			verifiedPtr = &verified
		}

		out = append(out, recovery.RecoveryPoint{
			ID:                "pve-backup:" + strings.TrimSpace(b.ID),
			Provider:          recovery.ProviderProxmoxPVE,
			Kind:              recovery.KindBackup,
			Mode:              mode,
			Outcome:           recovery.OutcomeSuccess,
			StartedAt:         &t,
			CompletedAt:       &t,
			SizeBytes:         sizePtr(b.Size),
			Verified:          verifiedPtr,
			Immutable:         immutable,
			SubjectResourceID: subjectRID,
			SubjectRef:        subjectRef,
			RepositoryRef: &recovery.ExternalRef{
				Type:      "proxmox-storage",
				Namespace: instanceName,
				Name:      strings.TrimSpace(b.Storage),
				Class:     nodeName,
			},
			Details: map[string]any{
				"storage":      strings.TrimSpace(b.Storage),
				"format":       strings.TrimSpace(b.Format),
				"notes":        strings.TrimSpace(b.Notes),
				"volid":        strings.TrimSpace(b.Volid),
				"isPBS":        b.IsPBS,
				"verification": strings.TrimSpace(b.Verification),
				"type":         strings.TrimSpace(b.Type),
				"instance":     instanceName,
				"node":         nodeName,
				"vmid":         vmid,
			},
		})
	}

	return out
}

func FromPVEBackupTasks(tasks []models.BackupTask, guestInfoBySourceID map[string]GuestInfo) []recovery.RecoveryPoint {
	if len(tasks) == 0 {
		return nil
	}

	out := make([]recovery.RecoveryPoint, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == "" {
			continue
		}

		instanceName := strings.TrimSpace(task.Instance)
		nodeName := strings.TrimSpace(task.Node)
		vmid := task.VMID

		var subjectRID string
		var subjectRef *recovery.ExternalRef

		if vmid > 0 && instanceName != "" && nodeName != "" {
			key := guestLookupKey(instanceName, nodeName, vmid)
			info, ok := guestInfoBySourceID[key]
			sourceID := strings.TrimSpace(info.SourceID)
			if sourceID == "" {
				sourceID = guestSourceID(instanceName, nodeName, vmid)
			}
			if ok && info.ResourceType != "" {
				subjectRID = unifiedresources.SourceSpecificID(info.ResourceType, unifiedresources.SourceProxmox, sourceID)
				subjectRef = proxmoxSubjectRef(info.ResourceType, info, instanceName, nodeName, vmid, sourceID)
			} else {
				// At least show something in UIs even when type is unknown.
				subjectRef = &recovery.ExternalRef{
					Type:      "proxmox-guest",
					Namespace: instanceName,
					Name:      fmt.Sprintf("%d", vmid),
					ID:        strings.TrimSpace(sourceID),
					Class:     nodeName,
				}
			}
		}

		started := task.StartTime.UTC()
		var completed *time.Time
		if !task.EndTime.IsZero() {
			t := task.EndTime.UTC()
			completed = &t
		}

		outcome := outcomeFromTaskStatus(task.Status)

		out = append(out, recovery.RecoveryPoint{
			ID:                "pve-task:" + strings.TrimSpace(task.ID),
			Provider:          recovery.ProviderProxmoxPVE,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeLocal,
			Outcome:           outcome,
			StartedAt:         &started,
			CompletedAt:       completed,
			SizeBytes:         sizePtr(task.Size),
			SubjectResourceID: subjectRID,
			SubjectRef:        subjectRef,
			Details: map[string]any{
				"type":      strings.TrimSpace(task.Type),
				"status":    strings.TrimSpace(task.Status),
				"error":     strings.TrimSpace(task.Error),
				"instance":  instanceName,
				"node":      nodeName,
				"vmid":      vmid,
				"taskID":    strings.TrimSpace(task.ID),
				"completed": completed != nil,
			},
		})
	}

	return out
}

func FromPBSBackups(backups []models.PBSBackup, candidatesByKey map[string][]GuestCandidate) []recovery.RecoveryPoint {
	if len(backups) == 0 {
		return nil
	}

	out := make([]recovery.RecoveryPoint, 0, len(backups))
	for _, b := range backups {
		if strings.TrimSpace(b.ID) == "" {
			continue
		}

		vmidStr := strings.TrimSpace(b.VMID)
		key := strings.ToLower(strings.TrimSpace(b.BackupType)) + ":" + vmidStr
		candidates := candidatesByKey[key]

		var subjectRID string
		var subjectRef *recovery.ExternalRef

		// Link to a unified resource only when we have a single unambiguous guest match.
		if len(candidates) == 1 {
			c := candidates[0]
			subjectRID = unifiedresources.SourceSpecificID(c.ResourceType, unifiedresources.SourceProxmox, c.SourceID)
			subjectRef = proxmoxSubjectRef(c.ResourceType, GuestInfo{Name: c.DisplayName, ResourceType: c.ResourceType, SourceID: c.SourceID}, c.InstanceName, c.NodeName, c.VMID, c.SourceID)
		} else {
			guestType := strings.ToLower(strings.TrimSpace(b.BackupType))
			refType := "proxmox-guest"
			if guestType == "vm" {
				refType = "proxmox-vm"
			} else if guestType == "ct" {
				refType = "proxmox-lxc"
			}
			subjectRef = &recovery.ExternalRef{
				Type:      refType,
				Namespace: strings.TrimSpace(b.Instance),
				Name:      vmidStr,
				ID:        vmidStr,
				Class:     "",
			}
		}

		t := b.BackupTime.UTC()
		protected := b.Protected
		immutable := &protected
		verified := b.Verified
		verifiedPtr := &verified
		details := map[string]any{
			"datastore":  strings.TrimSpace(b.Datastore),
			"namespace":  strings.TrimSpace(b.Namespace),
			"backupType": strings.TrimSpace(b.BackupType),
			"vmid":       vmidStr,
			"comment":    strings.TrimSpace(b.Comment),
			"owner":      strings.TrimSpace(b.Owner),
			"files":      append([]string(nil), b.Files...),
		}

		// Extract verification detail for frontend stability.
		switch v := b.VerificationRaw.(type) {
		case string:
			details["verificationState"] = v
		case map[string]interface{}:
			if state, ok := v["state"].(string); ok {
				details["verificationState"] = state
			}
			if upid, ok := v["upid"].(string); ok {
				details["verificationUpid"] = upid
			}
		}

		out = append(out, recovery.RecoveryPoint{
			ID:                "pbs-backup:" + strings.TrimSpace(b.ID),
			Provider:          recovery.ProviderProxmoxPBS,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeSuccess,
			StartedAt:         &t,
			CompletedAt:       &t,
			SizeBytes:         sizePtr(b.Size),
			Verified:          verifiedPtr,
			Immutable:         immutable,
			SubjectResourceID: subjectRID,
			SubjectRef:        subjectRef,
			RepositoryRef: &recovery.ExternalRef{
				Type:      "proxmox-pbs-datastore",
				Namespace: strings.TrimSpace(b.Instance),
				Name:      strings.TrimSpace(b.Datastore),
				Class:     strings.TrimSpace(b.Namespace),
			},
			Details: details,
		})
	}

	return out
}

func sizePtr(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	x := v
	return &x
}
