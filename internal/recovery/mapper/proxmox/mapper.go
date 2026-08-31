package proxmoxmapper

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/proxmoxidentity"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// GuestInfo is best-effort metadata used to attach display, canonical, and provider identity.
// Callers key it by guestLookupKey().
type GuestInfo struct {
	ResourceID   string
	SourceID     string
	ResourceType unifiedresources.ResourceType
	Name         string
}

type GuestCandidate struct {
	ResourceID    string
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

func subjectResourceID(resourceType unifiedresources.ResourceType, resourceID, sourceID string) string {
	if resourceID = strings.TrimSpace(resourceID); resourceID != "" {
		return resourceID
	}
	if sourceID = strings.TrimSpace(sourceID); sourceID != "" {
		// Guests derive node-independently (instance+VMID, #1669) so a
		// registry miss during a boot window still mints the same subject ID
		// the live resource carries.
		if instance, _, vmid, ok := unifiedresources.ParseProxmoxGuestSourceID(sourceID); ok {
			if id := unifiedresources.ProxmoxGuestCanonicalID(resourceType, instance, vmid); id != "" {
				return id
			}
		}
		return unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceProxmox, sourceID)
	}
	return ""
}

func preferredPBSBackupSubjectName(comment, vmid string) string {
	return proxmoxidentity.PreferredPBSBackupSubjectName(comment, vmid)
}

func selectPBSGuestCandidate(backup models.PBSBackup, candidates []GuestCandidate, learner *proxmoxidentity.PBSSourceLearner) (GuestCandidate, bool) {
	if len(candidates) == 0 {
		return GuestCandidate{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}

	matched := candidates
	if namespace := strings.TrimSpace(backup.Namespace); namespace != "" {
		filtered := make([]GuestCandidate, 0, len(matched))
		for _, candidate := range matched {
			if proxmoxidentity.NamespaceLocationScore(namespace, candidate.InstanceName, candidate.NodeName) > proxmoxidentity.NamespaceNoLocationMatch {
				filtered = append(filtered, candidate)
			}
		}
		if len(filtered) == 1 {
			return filtered[0], true
		}
		if len(filtered) > 1 {
			matched = filtered
		}
	}

	if preferredName := strings.ToLower(strings.TrimSpace(preferredPBSBackupSubjectName(backup.Comment, backup.VMID))); preferredName != "" {
		filtered := make([]GuestCandidate, 0, len(matched))
		for _, candidate := range matched {
			if strings.ToLower(strings.TrimSpace(candidate.DisplayName)) == preferredName {
				filtered = append(filtered, candidate)
			}
		}
		if len(filtered) == 1 {
			return filtered[0], true
		}
		if len(filtered) > 1 {
			matched = filtered
		}
	}

	// Last resort for collision VMIDs with no namespace or name evidence:
	// the learned submission-source mapping (owner token / datastore / PBS
	// instance, learned from this batch's attributable backups) can name the
	// cluster the backup came from (#1639). A decisive attribution to a
	// connection with no candidate here means the backup's own guest is
	// gone; never link it to another cluster's guest.
	if learner != nil {
		if attributed, decisive := learner.Resolve(backup.Instance, backup.Datastore, backup.Owner); decisive {
			filtered := make([]GuestCandidate, 0, len(matched))
			for _, candidate := range matched {
				if candidate.InstanceName == attributed {
					filtered = append(filtered, candidate)
				}
			}
			if len(filtered) == 1 {
				return filtered[0], true
			}
			if len(filtered) == 0 {
				return GuestCandidate{}, false
			}
			matched = filtered
		}
	}

	if len(matched) == 1 {
		return matched[0], true
	}
	return GuestCandidate{}, false
}

func proxmoxSubjectRef(resourceType unifiedresources.ResourceType, info GuestInfo, instanceName, nodeName string, vmid int, sourceID string) *recovery.ExternalRef {
	name := strings.TrimSpace(info.Name)
	if name == "" && vmid > 0 {
		name = fmt.Sprintf("%d", vmid)
	}
	refType := "proxmox-guest"
	if resourceType == unifiedresources.ResourceTypeVM {
		refType = "proxmox-vm"
	} else if resourceType == unifiedresources.ResourceTypeSystemContainer {
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
		return unifiedresources.ResourceTypeSystemContainer
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
				subjectRID = subjectResourceID(resourceType, info.ResourceID, sourceID)
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

func FromPVEStorageBackups(backups []models.StorageBackup, guestInfoByKey map[string]GuestInfo) []recovery.RecoveryPoint {
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
			info := guestInfoByKey[key]
			sourceID := strings.TrimSpace(info.SourceID)
			if sourceID == "" {
				sourceID = guestSourceID(instanceName, nodeName, vmid)
			}
			resourceType := info.ResourceType
			if resourceType == "" {
				resourceType = resourceTypeFromGuestType(b.Type)
			}
			if resourceType != "" {
				subjectRID = subjectResourceID(resourceType, info.ResourceID, sourceID)
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

		// A partial archive that a running vzdump task is still writing is a
		// running recovery point, not a completed one: no completion time,
		// and verification is not applicable yet.
		outcome := recovery.OutcomeSuccess
		completedAt := &t
		if b.InProgress {
			outcome = recovery.OutcomeRunning
			completedAt = nil
			verifiedPtr = nil
		}

		out = append(out, recovery.RecoveryPoint{
			ID:                "pve-backup:" + strings.TrimSpace(b.ID),
			Provider:          recovery.ProviderProxmoxPVE,
			Kind:              recovery.KindBackup,
			Mode:              mode,
			Outcome:           outcome,
			StartedAt:         &t,
			CompletedAt:       completedAt,
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
				"inProgress":   b.InProgress,
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

// FromPVEStorageBackupsWithEvidence maps a complete PVE storage enumeration
// and attaches the point-level evidence consumed by canonical protection
// posture. Collection completeness and permission state remain separate
// ProtectionProviderObservation facts owned by the polling caller.
func FromPVEStorageBackupsWithEvidence(
	backups []models.StorageBackup,
	guestInfoByKey map[string]GuestInfo,
	ingestedAt time.Time,
) ([]recovery.RecoveryPoint, error) {
	return withRecoveryPointEvidence(
		FromPVEStorageBackups(backups, guestInfoByKey),
		"pve-backup-inventory",
		ingestedAt,
	)
}

func FromPVEBackupTasks(tasks []models.BackupTask, guestInfoByKey map[string]GuestInfo) []recovery.RecoveryPoint {
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
			info, ok := guestInfoByKey[key]
			sourceID := strings.TrimSpace(info.SourceID)
			if sourceID == "" {
				sourceID = guestSourceID(instanceName, nodeName, vmid)
			}
			if ok && info.ResourceType != "" {
				subjectRID = subjectResourceID(info.ResourceType, info.ResourceID, sourceID)
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
			ID:       "pve-task:" + strings.TrimSpace(task.ID),
			Provider: recovery.ProviderProxmoxPVE,
			// A task is execution evidence, not proof that a restore artifact is
			// still available in provider inventory. Keep it queryable without
			// letting an OK task mint protected posture on its own.
			Kind:              recovery.KindOther,
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

func FromPVEBackupTasksWithEvidence(
	tasks []models.BackupTask,
	guestInfoByKey map[string]GuestInfo,
	ingestedAt time.Time,
) ([]recovery.RecoveryPoint, error) {
	return withRecoveryPointEvidence(
		FromPVEBackupTasks(tasks, guestInfoByKey),
		"pve-backup-task-inventory",
		ingestedAt,
	)
}

func FromPVEGuestSnapshotsWithEvidence(
	snapshots []models.GuestSnapshot,
	guestInfoByKey map[string]GuestInfo,
	ingestedAt time.Time,
) ([]recovery.RecoveryPoint, error) {
	return withRecoveryPointEvidence(
		FromPVEGuestSnapshots(snapshots, guestInfoByKey),
		"pve-snapshot-inventory",
		ingestedAt,
	)
}

func FromPBSBackups(backups []models.PBSBackup, candidatesByKey map[string][]GuestCandidate) []recovery.RecoveryPoint {
	if len(backups) == 0 {
		return nil
	}

	// First pass: learn each PBS submission source's cluster from the
	// backups that are attributable on their own evidence, so the second
	// pass can resolve collision VMIDs with no evidence of their own (#1639).
	// Every connection owning a candidate guest is registered first, so a
	// connection that contributes no attributable backup keeps the learner
	// inconclusive instead of letting a source it may share resolve to the
	// connections that happened to be attributable.
	learner := proxmoxidentity.NewPBSSourceLearner()
	for _, b := range backups {
		if strings.TrimSpace(b.ID) == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(b.BackupType)) + ":" + strings.TrimSpace(b.VMID)
		for _, candidate := range candidatesByKey[key] {
			learner.RegisterCandidate(b.Instance, candidate.InstanceName)
		}
	}
	for _, b := range backups {
		if strings.TrimSpace(b.ID) == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(b.BackupType)) + ":" + strings.TrimSpace(b.VMID)
		if c, ok := selectPBSGuestCandidate(b, candidatesByKey[key], nil); ok {
			learner.Observe(b.Instance, b.Datastore, b.Owner, c.InstanceName)
		}
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

		// Link to a unified resource when the candidate set is already singular or can be
		// disambiguated by PBS namespace / guest label without guessing across guest collisions.
		if c, ok := selectPBSGuestCandidate(b, candidates, learner); ok {
			subjectRID = subjectResourceID(c.ResourceType, c.ResourceID, c.SourceID)
			subjectRef = proxmoxSubjectRef(c.ResourceType, GuestInfo{Name: c.DisplayName, ResourceType: c.ResourceType, SourceID: c.SourceID}, c.InstanceName, c.NodeName, c.VMID, c.SourceID)
		} else {
			guestType := strings.ToLower(strings.TrimSpace(b.BackupType))
			refType := "proxmox-guest"
			if guestType == "vm" {
				refType = "proxmox-vm"
			} else if guestType == "ct" {
				refType = "proxmox-lxc"
			}
			subjectName := preferredPBSBackupSubjectName(b.Comment, vmidStr)
			if subjectName == "" {
				subjectName = vmidStr
			}
			subjectRef = &recovery.ExternalRef{
				Type:      refType,
				Namespace: strings.TrimSpace(b.Instance),
				Name:      subjectName,
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
			"datastore":             strings.TrimSpace(b.Datastore),
			"namespace":             strings.TrimSpace(b.Namespace),
			"backupType":            strings.TrimSpace(b.BackupType),
			"vmid":                  vmidStr,
			"comment":               strings.TrimSpace(b.Comment),
			"owner":                 strings.TrimSpace(b.Owner),
			"files":                 append([]string(nil), b.Files...),
			"inProgress":            b.InProgress,
			"writeActivityObserved": b.WriteActivityObserved,
			"writeActive":           b.WriteActive,
		}

		// A manifestless snapshot is never a successful recovery point. It is
		// running while a live PBS task accounts for it (or task visibility is
		// unavailable); after an authoritative task read finds no writer, it is
		// a failed/incomplete artifact left by a terminal backup or sync task.
		outcome := recovery.OutcomeSuccess
		completedAt := &t
		if b.InProgress {
			// The snapshot timestamp is its source backup identity, not the
			// terminal task time; no completion time is available here.
			completedAt = nil
			if b.WriteActivityObserved && !b.WriteActive {
				outcome = recovery.OutcomeFailed
			} else {
				outcome = recovery.OutcomeRunning
			}
			verifiedPtr = nil
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
			Outcome:           outcome,
			StartedAt:         &t,
			CompletedAt:       completedAt,
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

// FromPBSBackupsWithEvidence is the operational-trust adapter for direct PBS
// inventory. It preserves the supported RecoveryPoint payload while attaching
// explicit provider scope and typed evidence to every successfully enumerated
// backup. Collection-wide completeness and permission state are recorded
// separately as a ProtectionProviderObservation by the polling owner.
func FromPBSBackupsWithEvidence(
	backups []models.PBSBackup,
	candidatesByKey map[string][]GuestCandidate,
	ingestedAt time.Time,
) ([]recovery.RecoveryPoint, error) {
	return withRecoveryPointEvidence(
		FromPBSBackups(backups, candidatesByKey),
		"pbs-backup-inventory",
		ingestedAt,
	)
}

func withRecoveryPointEvidence(
	points []recovery.RecoveryPoint,
	collector string,
	ingestedAt time.Time,
) ([]recovery.RecoveryPoint, error) {
	if len(points) == 0 {
		return []recovery.RecoveryPoint{}, nil
	}
	if ingestedAt.IsZero() {
		return nil, fmt.Errorf("recovery evidence ingestion time is required")
	}
	for i := range points {
		points[i].ProviderScope = recovery.ProviderScopeForPoint(points[i])
		if strings.TrimSpace(points[i].SubjectResourceID) == "" &&
			strings.TrimSpace(recovery.SubjectKeyForPoint(points[i])) == "" {
			// Provider-native host/config archives have no canonical workload
			// subject. Keep them as recovery artifacts without fabricating a
			// posture identity or failing the subject-linked batch.
			continue
		}
		evidence, err := recovery.NewRecoveryPointEvidence(
			points[i],
			collector,
			ingestedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("map recovery point %q evidence: %w", points[i].ID, err)
		}
		points[i].Evidence = evidence
	}
	return points, nil
}

func sizePtr(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	x := v
	return &x
}
