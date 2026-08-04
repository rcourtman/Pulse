package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	truenasmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func (g FixtureGraph) RecoveryPoints() []recovery.RecoveryPoint {
	resources, _ := g.UnifiedResourceSnapshot()
	return generateMockRecoveryPoints(g.State, g.PlatformFixtures, resources)
}

func generateMockRecoveryPoints(
	snapshot models.StateSnapshot,
	fixtures PlatformFixtures,
	resources []unifiedresources.Resource,
) []recovery.RecoveryPoint {
	// Anchor timestamps to midnight UTC so results are stable across requests
	// (pagination, sorting) while still staying within the "last 30 days" window.
	anchor := time.Now().UTC().Truncate(24 * time.Hour)

	clusters := recoveryKubernetesClusters(snapshot)

	points := make([]recovery.RecoveryPoint, 0, 192)

	boolPtr := func(v bool) *bool { return &v }
	int64Ptr := func(v int64) *int64 { return &v }

	// Kubernetes PVC snapshot subjects: 3 PVCs with multiple points each (success/running/failed).
	k8sPVCSubjects := recoveryKubernetesPVCSubjects(clusters)
	for si, s := range k8sPVCSubjects {
		for i := 0; i < 6; i++ {
			ageDays := 2 + (si*7+i*4)%28
			started := anchor.AddDate(0, 0, -ageDays).Add(time.Duration((si*5+i*3)%23) * time.Hour).Add(time.Duration((i%7)*6) * time.Minute)

			outcome := recovery.OutcomeSuccess
			var completedAt *time.Time
			switch i % 4 {
			case 0, 1:
				outcome = recovery.OutcomeSuccess
				t := started.Add(time.Duration(2+(i%4)) * time.Minute)
				completedAt = &t
			case 2:
				outcome = recovery.OutcomeRunning
				completedAt = nil
			default:
				outcome = recovery.OutcomeFailed
				t := started.Add(time.Duration(1+(i%3)) * time.Minute)
				completedAt = &t
			}

			snapName := "vs-" + s.namespace + "-" + s.pvc + "-" + rpTwoDigits(i+1)
			snapUID := rpStableID("k8s", "volumesnapshot", s.clusterID, s.namespace, snapName)

			details := map[string]any{
				"k8sClusterId":   s.clusterID,
				"k8sClusterName": s.clusterName,
				"snapshotUid":    snapUID,
				"snapshotName":   snapName,
				"snapshotNs":     s.namespace,
			}
			if i%3 == 0 {
				details["snapshotContentName"] = "snapcontent-" + strings.ToLower(rpTwoDigits(i+1))
			}

			var sizeBytes *int64
			if i%2 == 0 && completedAt != nil {
				sizeBytes = int64Ptr(2_000_000_000 + int64(si+1)*750_000_000 + int64(i)*125_000_000)
			}

			// Mix optional flags across points: true/false/nil.
			var verified *bool
			if completedAt != nil {
				if i%3 == 0 {
					verified = boolPtr(true)
				} else if i%3 == 1 {
					verified = boolPtr(false)
				}
			}
			var encrypted *bool
			if i%4 == 0 {
				encrypted = boolPtr(true)
			} else if i%4 == 1 {
				encrypted = boolPtr(false)
			}
			var immutable *bool
			if i%5 == 0 {
				immutable = boolPtr(true)
			} else if i%5 == 1 {
				immutable = boolPtr(false)
			}

			points = append(points, recovery.RecoveryPoint{
				ID:          rpStableID("mock", "recoverypoint", "k8s", "snapshot", s.clusterID, s.namespace, s.pvc, rpTimeKey(completedAt, &started)),
				Provider:    recovery.ProviderKubernetes,
				Kind:        recovery.KindSnapshot,
				Mode:        recovery.ModeSnapshot,
				Outcome:     outcome,
				StartedAt:   rpPtrTime(started),
				CompletedAt: completedAt,
				SizeBytes:   sizeBytes,
				Verified:    verified,
				Encrypted:   encrypted,
				Immutable:   immutable,
				SubjectRef: &recovery.ExternalRef{
					Type:      "k8s-pvc",
					Namespace: s.namespace,
					Name:      s.pvc,
					UID:       mockRecoveryPVCUID(s.clusterName, s.namespace, s.pvc),
				},
				RepositoryRef: &recovery.ExternalRef{
					Type:  "k8s-volume-snapshot-class",
					Name:  s.class,
					Class: s.class,
				},
				Details: details,
			})
		}
	}

	// Kubernetes Velero backups: 2 clusters with multiple points each (success/warning/failed/running).
	veleroLocations := []string{"minio", "s3-primary"}
	for ci, c := range clusters {
		for i := 0; i < 6; i++ {
			ageDays := 1 + (ci*9+i*3)%26
			started := anchor.AddDate(0, 0, -ageDays).Add(time.Duration((12+i*2+ci)%23) * time.Hour).Add(time.Duration((i%7)*9) * time.Minute)

			outcome := recovery.OutcomeSuccess
			phase := "Completed"
			switch i % 5 {
			case 0, 1:
				outcome = recovery.OutcomeSuccess
				phase = "Completed"
			case 2:
				outcome = recovery.OutcomeWarning
				phase = "PartiallyFailed"
			case 3:
				outcome = recovery.OutcomeFailed
				phase = "Failed"
			default:
				outcome = recovery.OutcomeRunning
				phase = "InProgress"
			}

			var completedAt *time.Time
			if outcome == recovery.OutcomeRunning {
				completedAt = nil
			} else {
				t := started.Add(time.Duration(8+(i%12)) * time.Minute)
				completedAt = &t
			}

			veleroNs := "velero"
			backupName := "backup-" + c.name + "-" + rpTwoDigits(i+1)
			veleroUID := rpStableID("k8s", "velero-backup", c.id, veleroNs, backupName)
			location := veleroLocations[(ci+i)%len(veleroLocations)]

			var sizeBytes *int64
			if completedAt != nil && i%2 == 0 {
				sizeBytes = int64Ptr(20_000_000_000 + int64(ci+1)*5_000_000_000 + int64(i)*1_250_000_000)
			}

			points = append(points, recovery.RecoveryPoint{
				ID:          rpStableID("mock", "recoverypoint", "k8s", "velero", c.id, veleroNs, backupName, rpTimeKey(completedAt, &started)),
				Provider:    recovery.ProviderKubernetes,
				Kind:        recovery.KindBackup,
				Mode:        recovery.ModeRemote,
				Outcome:     outcome,
				StartedAt:   rpPtrTime(started),
				CompletedAt: completedAt,
				SizeBytes:   sizeBytes,
				Verified: func() *bool {
					if completedAt == nil {
						return nil
					}
					if i%3 == 0 {
						return boolPtr(true)
					}
					if i%3 == 1 {
						return boolPtr(false)
					}
					return nil
				}(),
				Encrypted: func() *bool {
					if i%4 == 0 {
						return boolPtr(true)
					}
					if i%4 == 1 {
						return boolPtr(false)
					}
					return nil
				}(),
				Immutable: func() *bool {
					if i%5 == 0 {
						return boolPtr(true)
					}
					if i%5 == 1 {
						return boolPtr(false)
					}
					return nil
				}(),
				SubjectRef: &recovery.ExternalRef{
					Type: "k8s-cluster",
					ID:   c.id,
					Name: c.name,
				},
				RepositoryRef: &recovery.ExternalRef{
					Type: "velero-backup-storage-location",
					Name: location,
				},
				Details: map[string]any{
					"k8sClusterId":    c.id,
					"k8sClusterName":  c.name,
					"veleroUid":       veleroUID,
					"veleroName":      backupName,
					"veleroNs":        veleroNs,
					"phase":           phase,
					"storageLocation": location,
					"policyName":      "daily-30d",
				},
			})
		}
	}

	// Proxmox recovery is projected from the same canonical fixture inventories
	// consumed by the Backups page. Independent sample points drifted away from
	// live workload IDs and made every server-owned posture look unknown.
	points = append(points, mockProxmoxRecoveryPoints(
		snapshot,
		resources,
		mockRecoveryIngestedAt(snapshot),
	)...)

	// TrueNAS: reuse the provider-native recovery artifact model so demo-mode
	// recovery mirrors the same contract as live TrueNAS reads.
	truenasConnection := defaultTrueNASConnectionFixture(fixtures)
	rebasedTrueNAS := rebaseTrueNASRecoverySnapshot(fixtures.TrueNAS, time.Now().UTC().Add(-90*time.Minute).Truncate(time.Minute))
	points = append(points, truenasmapper.FromTrueNASSnapshot(truenasConnection.ID, &rebasedTrueNAS)...)

	// Ensure newest-first ordering like the store (completedAt desc with NULLS last),
	// with stable ID tie-breakers for deterministic pagination.
	sort.SliceStable(points, func(i, j int) bool {
		a := points[i]
		b := points[j]

		aHasCompleted := a.CompletedAt != nil && !a.CompletedAt.IsZero()
		bHasCompleted := b.CompletedAt != nil && !b.CompletedAt.IsZero()
		if aHasCompleted != bHasCompleted {
			return aHasCompleted
		}
		if aHasCompleted && bHasCompleted {
			if !a.CompletedAt.Equal(*b.CompletedAt) {
				return a.CompletedAt.After(*b.CompletedAt)
			}
		} else {
			aStart := time.Time{}
			bStart := time.Time{}
			if a.StartedAt != nil {
				aStart = a.StartedAt.UTC()
			}
			if b.StartedAt != nil {
				bStart = b.StartedAt.UTC()
			}
			if !aStart.Equal(bStart) {
				return aStart.After(bStart)
			}
		}
		return a.ID > b.ID
	})

	return points
}

func mockProxmoxRecoveryPoints(
	snapshot models.StateSnapshot,
	resources []unifiedresources.Resource,
	ingestedAt time.Time,
) []recovery.RecoveryPoint {
	guestInfo, candidates := mockProxmoxRecoverySubjects(snapshot, resources)
	out := make([]recovery.RecoveryPoint, 0,
		len(snapshot.PVEBackups.StorageBackups)+
			len(snapshot.PVEBackups.GuestSnapshots)+
			len(snapshot.PVEBackups.BackupTasks)+
			len(snapshot.PBSBackups),
	)

	appendMapped := func(points []recovery.RecoveryPoint, err error) {
		if err == nil {
			out = append(out, points...)
		}
	}
	appendMapped(proxmoxmapper.FromPVEStorageBackupsWithEvidence(
		snapshot.PVEBackups.StorageBackups,
		guestInfo,
		ingestedAt,
	))
	appendMapped(proxmoxmapper.FromPVEGuestSnapshotsWithEvidence(
		snapshot.PVEBackups.GuestSnapshots,
		guestInfo,
		ingestedAt,
	))
	appendMapped(proxmoxmapper.FromPVEBackupTasksWithEvidence(
		snapshot.PVEBackups.BackupTasks,
		guestInfo,
		ingestedAt,
	))
	appendMapped(proxmoxmapper.FromPBSBackupsWithEvidence(
		snapshot.PBSBackups,
		candidates,
		ingestedAt,
	))
	return out
}

func mockRecoveryIngestedAt(snapshot models.StateSnapshot) time.Time {
	var latest time.Time
	consider := func(candidate time.Time) {
		if candidate.After(latest) {
			latest = candidate.UTC()
		}
	}
	for _, backup := range snapshot.PVEBackups.StorageBackups {
		consider(backup.Time)
	}
	for _, point := range snapshot.PVEBackups.GuestSnapshots {
		consider(point.Time)
	}
	for _, task := range snapshot.PVEBackups.BackupTasks {
		consider(task.StartTime)
		consider(task.EndTime)
	}
	for _, backup := range snapshot.PBSBackups {
		consider(backup.BackupTime)
	}
	if latest.IsZero() {
		return time.Now().UTC()
	}
	return latest.Add(time.Second)
}

func mockProxmoxRecoverySubjects(
	snapshot models.StateSnapshot,
	resources []unifiedresources.Resource,
) (
	map[string]proxmoxmapper.GuestInfo,
	map[string][]proxmoxmapper.GuestCandidate,
) {
	guestInfo := make(map[string]proxmoxmapper.GuestInfo, len(snapshot.VMs)+len(snapshot.Containers))
	candidates := make(map[string][]proxmoxmapper.GuestCandidate, len(snapshot.VMs)+len(snapshot.Containers))
	type guestCoordinate struct {
		node string
		vmid int
	}
	byCoordinate := make(map[guestCoordinate][]proxmoxmapper.GuestInfo)
	seenResourceIDs := make(map[string]struct{}, len(snapshot.VMs)+len(snapshot.Containers))

	appendGuest := func(
		resourceID string,
		resourceType unifiedresources.ResourceType,
		backupType, instance, node, name, sourceID string,
		vmid int,
	) {
		instance = strings.TrimSpace(instance)
		node = strings.TrimSpace(node)
		if instance == "" || node == "" || vmid <= 0 {
			return
		}
		resourceID = strings.TrimSpace(resourceID)
		if resourceID == "" {
			resourceID = unifiedresources.ProxmoxGuestCanonicalID(resourceType, instance, vmid)
		}
		if sourceID = strings.TrimSpace(sourceID); sourceID == "" {
			sourceID = fmt.Sprintf("%s:%s:%d", instance, node, vmid)
		}
		info := proxmoxmapper.GuestInfo{
			ResourceID:   resourceID,
			SourceID:     sourceID,
			ResourceType: resourceType,
			Name:         strings.TrimSpace(name),
		}
		guestInfo[fmt.Sprintf("%s|%s|%d", instance, node, vmid)] = info
		coordinate := guestCoordinate{node: node, vmid: vmid}
		byCoordinate[coordinate] = append(byCoordinate[coordinate], info)
		if _, seen := seenResourceIDs[resourceID]; seen {
			return
		}
		seenResourceIDs[resourceID] = struct{}{}
		key := backupType + ":" + rpItoa(vmid)
		candidates[key] = append(candidates[key], proxmoxmapper.GuestCandidate{
			ResourceID:    resourceID,
			SourceID:      sourceID,
			ResourceType:  resourceType,
			DisplayName:   strings.TrimSpace(name),
			InstanceName:  instance,
			NodeName:      node,
			VMID:          vmid,
			BackupTypeKey: backupType,
		})
	}

	for _, resource := range resources {
		if resource.Proxmox == nil || resource.Proxmox.Template {
			continue
		}
		backupType := ""
		switch resource.Type {
		case unifiedresources.ResourceTypeVM:
			backupType = "vm"
		case unifiedresources.ResourceTypeSystemContainer:
			backupType = "ct"
		default:
			continue
		}
		appendGuest(
			resource.ID,
			resource.Type,
			backupType,
			resource.Proxmox.Instance,
			resource.Proxmox.NodeName,
			resource.Name,
			resource.Proxmox.SourceID,
			resource.Proxmox.VMID,
		)
	}

	// Preserve a defensive fallback for a malformed fixture graph, but prefer
	// the registry-owned IDs above. They include identity successions and merge
	// decisions that cannot be reconstructed safely from a backup record.
	for _, vm := range snapshot.VMs {
		coordinate := guestCoordinate{node: strings.TrimSpace(vm.Node), vmid: vm.VMID}
		if !vm.Template && len(byCoordinate[coordinate]) == 0 {
			appendGuest("", unifiedresources.ResourceTypeVM, "vm", vm.Instance, vm.Node, vm.Name, vm.ID, vm.VMID)
		}
	}
	for _, container := range snapshot.Containers {
		coordinate := guestCoordinate{node: strings.TrimSpace(container.Node), vmid: container.VMID}
		if !container.Template && len(byCoordinate[coordinate]) == 0 {
			appendGuest("", unifiedresources.ResourceTypeSystemContainer, "ct", container.Instance, container.Node, container.Name, container.ID, container.VMID)
		}
	}

	bindProviderAlias := func(instance, node string, vmid int) {
		instance = strings.TrimSpace(instance)
		node = strings.TrimSpace(node)
		if instance == "" || node == "" || vmid <= 0 {
			return
		}
		matches := byCoordinate[guestCoordinate{node: node, vmid: vmid}]
		if len(matches) == 1 {
			guestInfo[fmt.Sprintf("%s|%s|%d", instance, node, vmid)] = matches[0]
		}
	}
	for _, backup := range snapshot.PVEBackups.StorageBackups {
		bindProviderAlias(backup.Instance, backup.Node, backup.VMID)
	}
	for _, point := range snapshot.PVEBackups.GuestSnapshots {
		bindProviderAlias(point.Instance, point.Node, point.VMID)
	}
	for _, task := range snapshot.PVEBackups.BackupTasks {
		bindProviderAlias(task.Instance, task.Node, task.VMID)
	}
	return guestInfo, candidates
}

func rebaseTrueNASRecoverySnapshot(snapshot truenas.FixtureSnapshot, targetLatest time.Time) truenas.FixtureSnapshot {
	latest := latestTrueNASRecoveryTime(snapshot)
	if latest.IsZero() {
		latest = targetLatest
	}
	if targetLatest.IsZero() {
		targetLatest = latest
	}

	shift := targetLatest.Sub(latest)
	rebased := snapshot
	if !rebased.CollectedAt.IsZero() {
		rebased.CollectedAt = rebased.CollectedAt.Add(shift)
	}

	if len(snapshot.ZFSSnapshots) > 0 {
		rebased.ZFSSnapshots = make([]truenas.ZFSSnapshot, len(snapshot.ZFSSnapshots))
		for i, snap := range snapshot.ZFSSnapshots {
			rebased.ZFSSnapshots[i] = snap
			if snap.CreatedAt != nil {
				shifted := snap.CreatedAt.Add(shift)
				rebased.ZFSSnapshots[i].CreatedAt = &shifted
			}
		}
	}

	if len(snapshot.ReplicationTasks) > 0 {
		rebased.ReplicationTasks = make([]truenas.ReplicationTask, len(snapshot.ReplicationTasks))
		for i, task := range snapshot.ReplicationTasks {
			rebased.ReplicationTasks[i] = task
			if task.LastRun != nil {
				shifted := task.LastRun.Add(shift)
				rebased.ReplicationTasks[i].LastRun = &shifted
			}
		}
	}

	return rebased
}

func latestTrueNASRecoveryTime(snapshot truenas.FixtureSnapshot) time.Time {
	latest := snapshot.CollectedAt

	for _, snap := range snapshot.ZFSSnapshots {
		if snap.CreatedAt != nil && snap.CreatedAt.After(latest) {
			latest = *snap.CreatedAt
		}
	}

	for _, task := range snapshot.ReplicationTasks {
		if task.LastRun != nil && task.LastRun.After(latest) {
			latest = *task.LastRun
		}
	}

	return latest
}

type mockRecoveryCluster struct {
	id   string
	name string
}

type mockRecoveryPVCSubject struct {
	clusterID   string
	clusterName string
	namespace   string
	pvc         string
	class       string
}

func recoveryKubernetesClusters(snapshot models.StateSnapshot) []mockRecoveryCluster {
	clusters := make([]mockRecoveryCluster, 0, len(snapshot.KubernetesClusters))
	seen := make(map[string]struct{}, len(snapshot.KubernetesClusters))

	for _, cluster := range snapshot.KubernetesClusters {
		id := strings.TrimSpace(cluster.ID)
		if id == "" {
			id = strings.TrimSpace(cluster.AgentID)
		}
		name := strings.TrimSpace(cluster.DisplayName)
		if name == "" {
			name = strings.TrimSpace(cluster.CustomDisplayName)
		}
		if name == "" {
			name = strings.TrimSpace(cluster.Name)
		}
		if name == "" {
			name = strings.TrimSpace(cluster.Context)
		}
		if name == "" {
			name = strings.TrimSpace(cluster.Server)
		}
		if id == "" && name == "" {
			continue
		}
		if id == "" {
			id = rpStableID("k8s", "cluster", name)
		}
		if name == "" {
			name = id
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clusters = append(clusters, mockRecoveryCluster{id: id, name: name})
	}

	if len(clusters) > 0 {
		return clusters
	}

	return []mockRecoveryCluster{
		{id: "k8s-mock-cluster-1", name: "dev-cluster"},
		{id: "k8s-mock-cluster-2", name: "prod-cluster"},
	}
}

func recoveryKubernetesPVCSubjects(clusters []mockRecoveryCluster) []mockRecoveryPVCSubject {
	if len(clusters) == 0 {
		return nil
	}

	templates := []struct {
		namespace string
		pvc       string
		class     string
	}{
		{namespace: "default", pvc: "postgres-pvc", class: "csi-ceph-rbd"},
		{namespace: "monitoring", pvc: "prometheus-pvc", class: "csi-local-path"},
		{namespace: "media", pvc: "nextcloud-pvc", class: "csi-ebs-gp3"},
	}

	subjects := make([]mockRecoveryPVCSubject, 0, len(templates))
	for i := 0; i < len(templates); i++ {
		cluster := clusters[i%len(clusters)]
		template := templates[i]
		subjects = append(subjects, mockRecoveryPVCSubject{
			clusterID:   cluster.id,
			clusterName: cluster.name,
			namespace:   template.namespace,
			pvc:         template.pvc,
			class:       template.class,
		})
	}

	return subjects
}

func mockRecoveryPVCUID(clusterName, namespace, pvc string) string {
	clusterName = strings.TrimSpace(strings.ToLower(clusterName))
	clusterName = strings.ReplaceAll(clusterName, " ", "-")
	namespace = strings.TrimSpace(strings.ToLower(namespace))
	pvc = strings.TrimSpace(strings.ToLower(pvc))
	return firstNonEmptyTrimmed(
		fmt.Sprintf("%s/%s/%s", clusterName, namespace, pvc),
		fmt.Sprintf("%s/%s", namespace, pvc),
		pvc,
	)
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	tt := t.UTC()
	return &tt
}

func cloneBoolPtr(b *bool) *bool {
	if b == nil {
		return nil
	}
	v := *b
	return &v
}

func cloneInt64Ptr(n *int64) *int64 {
	if n == nil {
		return nil
	}
	v := *n
	return &v
}

func rpPtrTime(t time.Time) *time.Time {
	tt := t.UTC()
	return &tt
}

func rpTimeKey(primary, fallback *time.Time) string {
	if primary != nil && !primary.IsZero() {
		return primary.UTC().Format(time.RFC3339Nano)
	}
	if fallback != nil && !fallback.IsZero() {
		return fallback.UTC().Format(time.RFC3339Nano)
	}
	return ""
}

func rpStableID(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(part)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func rpTwoDigits(n int) string {
	if n < 0 {
		n = -n
	}
	if n < 10 {
		return "0" + rpItoa(n)
	}
	return rpItoa(n)
}

func rpItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(buf[i:])
}
