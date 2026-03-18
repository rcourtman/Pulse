package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

var (
	mockRecoveryPointsOnce sync.Once
	mockRecoveryPoints     []recovery.RecoveryPoint
)

func GetMockRecoveryPoints() []recovery.RecoveryPoint {
	mockRecoveryPointsOnce.Do(func() {
		mockRecoveryPoints = generateMockRecoveryPoints()
	})
	return cloneRecoveryPoints(mockRecoveryPoints)
}

func generateMockRecoveryPoints() []recovery.RecoveryPoint {
	// Anchor timestamps to midnight UTC so results are stable across requests
	// (pagination, sorting) while still staying within the "last 30 days" window.
	anchor := time.Now().UTC().Truncate(24 * time.Hour)

	clusters := []struct {
		id   string
		name string
	}{
		{id: "k8s-mock-cluster-1", name: "dev-cluster"},
		{id: "k8s-mock-cluster-2", name: "prod-cluster"},
	}

	points := make([]recovery.RecoveryPoint, 0, 64)

	boolPtr := func(v bool) *bool { return &v }
	int64Ptr := func(v int64) *int64 { return &v }

	// Kubernetes PVC snapshot subjects: 3 PVCs with multiple points each (success/running/failed).
	k8sPVCSubjects := []struct {
		clusterID   string
		clusterName string
		namespace   string
		pvc         string
		class       string
	}{
		{clusterID: clusters[0].id, clusterName: clusters[0].name, namespace: "default", pvc: "postgres-pvc", class: "csi-ceph-rbd"},
		{clusterID: clusters[0].id, clusterName: clusters[0].name, namespace: "monitoring", pvc: "prometheus-pvc", class: "csi-local-path"},
		{clusterID: clusters[1].id, clusterName: clusters[1].name, namespace: "media", pvc: "nextcloud-pvc", class: "csi-ebs-gp3"},
	}
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
					UID:       rpStableID("k8s", "pvc", s.clusterID, s.namespace, s.pvc),
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

	// Proxmox: a few guest subjects with multiple backup points each.
	// This keeps the Backups page platform-agnostic while still showing familiar PVE/PBS-like artifacts.
	proxmoxSubjects := []struct {
		instance string
		node     string
		vmid     int
		typ      string // "vm" or "lxc"
		name     string
		storage  string
		isPBS    bool
	}{
		{instance: "pve-1", node: "pve-a", vmid: 101, typ: "vm", name: "web-01", storage: "pbs-1", isPBS: true},
		{instance: "pve-1", node: "pve-a", vmid: 102, typ: "vm", name: "db-01", storage: "local-zfs", isPBS: false},
		{instance: "pve-2", node: "pve-b", vmid: 201, typ: "lxc", name: "cache-01", storage: "pbs-1", isPBS: true},
	}
	for si, s := range proxmoxSubjects {
		for i := 0; i < 5; i++ {
			ageDays := 2 + (si*8+i*6)%27
			started := anchor.AddDate(0, 0, -ageDays).Add(time.Duration((6+si*4+i*3)%23) * time.Hour).Add(time.Duration((i%7)*5) * time.Minute)

			outcome := recovery.OutcomeSuccess
			status := "ok"
			errText := ""
			switch i % 5 {
			case 0, 1, 2:
				outcome = recovery.OutcomeSuccess
				status = "ok"
			case 3:
				outcome = recovery.OutcomeWarning
				status = "warning"
			default:
				outcome = recovery.OutcomeFailed
				status = "error"
				errText = "backup failed: I/O error"
			}

			// Keep the newest one sometimes running.
			var completedAt *time.Time
			if i == 0 && si%2 == 0 {
				outcome = recovery.OutcomeRunning
				status = "running"
				completedAt = nil
			} else {
				t := started.Add(time.Duration(7+(i%8)) * time.Minute)
				completedAt = &t
			}

			guestType := "proxmox-guest"
			if s.typ == "vm" {
				guestType = "proxmox-vm"
			} else if s.typ == "lxc" {
				guestType = "proxmox-lxc"
			}

			sourceID := rpStableID("proxmox", "guest", s.instance, s.node, rpItoa(s.vmid))
			backupID := rpStableID("proxmox", "backup", s.instance, s.node, rpItoa(s.vmid), rpTimeKey(completedAt, &started))

			mode := recovery.ModeLocal
			if s.isPBS {
				mode = recovery.ModeRemote
			}

			var verified *bool
			if s.isPBS && completedAt != nil {
				verified = boolPtr(i%3 == 0)
			}
			var immutable *bool
			if completedAt != nil {
				if i%4 == 0 {
					immutable = boolPtr(true)
				} else if i%4 == 1 {
					immutable = boolPtr(false)
				}
			}
			var encrypted *bool
			if s.isPBS {
				if i%4 == 0 {
					encrypted = boolPtr(true)
				} else if i%4 == 1 {
					encrypted = boolPtr(false)
				}
			}

			var sizeBytes *int64
			if completedAt != nil && i%2 == 0 {
				sizeBytes = int64Ptr(15_000_000_000 + int64(si)*4_000_000_000 + int64(i)*1_500_000_000)
			}

			points = append(points, recovery.RecoveryPoint{
				ID:          "pve-backup:" + backupID,
				Provider:    recovery.ProviderProxmoxPVE,
				Kind:        recovery.KindBackup,
				Mode:        mode,
				Outcome:     outcome,
				StartedAt:   rpPtrTime(started),
				CompletedAt: completedAt,
				SizeBytes:   sizeBytes,
				Verified:    verified,
				Encrypted:   encrypted,
				Immutable:   immutable,
				SubjectRef: &recovery.ExternalRef{
					Type:      guestType,
					Namespace: s.instance,
					Name:      s.name,
					ID:        sourceID,
					Class:     s.node,
				},
				RepositoryRef: &recovery.ExternalRef{
					Type:      "proxmox-storage",
					Namespace: s.instance,
					Name:      s.storage,
					Class:     s.node,
				},
				Details: map[string]any{
					"type":      s.typ,
					"instance":  s.instance,
					"node":      s.node,
					"vmid":      s.vmid,
					"storage":   s.storage,
					"isPBS":     s.isPBS,
					"status":    status,
					"lastError": errText,
					"notes":     "scheduled",
				},
			})
		}
	}

	// TrueNAS: 3 dataset subjects with multiple points over time.
	truenasConnID := "truenas-mock-1"
	truenasHost := "truenas.local"
	truenasDatasets := []string{
		"tank/apps/postgres",
		"tank/apps/minio",
		"tank/media/photos",
	}

	// ZFS snapshots (snapshot / snapshot): success points with meaningful details.
	for di, ds := range truenasDatasets {
		for i := 0; i < 4; i++ {
			ageDays := 3 + (di*6+i*5)%25
			completed := anchor.AddDate(0, 0, -ageDays).Add(time.Duration((3+di*4+i*2)%23) * time.Hour).Add(time.Duration((i%7)*7) * time.Minute)

			snapName := "auto-" + completed.Format("20060102") + "-" + rpTwoDigits(i+1)
			full := ds + "@" + snapName

			var sizeBytes *int64
			if i%2 == 1 {
				sizeBytes = int64Ptr(750_000_000 + int64(di)*250_000_000 + int64(i)*125_000_000)
			}

			points = append(points, recovery.RecoveryPoint{
				ID:          rpStableID("mock", "recoverypoint", "truenas", "zfs-snapshot", truenasConnID, full, completed.UTC().Format(time.RFC3339Nano)),
				Provider:    recovery.ProviderTrueNAS,
				Kind:        recovery.KindSnapshot,
				Mode:        recovery.ModeSnapshot,
				Outcome:     recovery.OutcomeSuccess,
				StartedAt:   rpPtrTime(completed),
				CompletedAt: rpPtrTime(completed),
				SizeBytes:   sizeBytes,
				Immutable: func() *bool {
					if i%4 == 0 {
						return boolPtr(true)
					}
					if i%4 == 1 {
						return boolPtr(false)
					}
					return nil
				}(),
				Encrypted: func() *bool {
					if i%3 == 0 {
						return boolPtr(true)
					}
					if i%3 == 1 {
						return boolPtr(false)
					}
					return nil
				}(),
				SubjectRef: &recovery.ExternalRef{
					Type: "truenas-dataset",
					Name: ds,
					ID:   ds,
				},
				Details: map[string]any{
					"connectionId": truenasConnID,
					"hostname":     truenasHost,
					"dataset":      ds,
					"snapshot":     snapName,
					"fullName":     full,
					"policyName":   "hourly-7d",
				},
			})
		}
	}

	// Replication tasks (backup / remote): ensure each dataset has a success plus a warning/failed.
	for di, ds := range truenasDatasets {
		taskID := "rep-task-" + rpTwoDigits(di+1)
		taskName := "replicate-" + strings.ReplaceAll(ds, "/", "-")
		target := "backup-tank/replicated/" + strings.ReplaceAll(ds, "/", "_")

		for i := 0; i < 4; i++ {
			ageDays := 1 + (di*5+i*6)%27
			started := anchor.AddDate(0, 0, -ageDays).Add(time.Duration((8+di*3+i*4)%23) * time.Hour).Add(time.Duration((i%7)*11) * time.Minute)

			state := "SUCCESS"
			errText := ""
			outcome := recovery.OutcomeSuccess
			switch i % 4 {
			case 0:
				state = "SUCCESS"
				outcome = recovery.OutcomeSuccess
			case 1:
				state = "WARNING"
				outcome = recovery.OutcomeWarning
			case 2:
				state = "FAILED"
				errText = "network timeout"
				outcome = recovery.OutcomeFailed
			default:
				state = "RUNNING"
				outcome = recovery.OutcomeRunning
			}

			var completedAt *time.Time
			if outcome == recovery.OutcomeRunning {
				completedAt = nil
			} else {
				t := started.Add(time.Duration(14+(i%8)) * time.Minute)
				completedAt = &t
			}

			lastSnapshot := ds + "@auto-" + anchor.AddDate(0, 0, -((i+2)%14)).Format("20060102") + "-01"

			points = append(points, recovery.RecoveryPoint{
				ID:          rpStableID("mock", "recoverypoint", "truenas", "replication-task", truenasConnID, taskID, rpTimeKey(completedAt, &started)),
				Provider:    recovery.ProviderTrueNAS,
				Kind:        recovery.KindBackup,
				Mode:        recovery.ModeRemote,
				Outcome:     outcome,
				StartedAt:   rpPtrTime(started),
				CompletedAt: completedAt,
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
				SubjectRef:    &recovery.ExternalRef{Type: "truenas-dataset", Name: ds, ID: ds},
				RepositoryRef: &recovery.ExternalRef{Type: "truenas-dataset", Name: target, ID: target},
				Details: map[string]any{
					"connectionId":   truenasConnID,
					"hostname":       truenasHost,
					"taskId":         taskID,
					"taskName":       taskName,
					"sourceDatasets": []string{ds},
					"targetDataset":  target,
					"lastState":      state,
					"lastError":      errText,
					"lastSnapshot":   lastSnapshot,
					"policyName":     "daily-replication",
				},
			})
		}
	}

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

	// Hard cap within requested range, while keeping at least 30 points.
	if len(points) > 80 {
		points = points[:80]
	}

	return points
}

func cloneRecoveryPoints(src []recovery.RecoveryPoint) []recovery.RecoveryPoint {
	if len(src) == 0 {
		return nil
	}
	dst := make([]recovery.RecoveryPoint, 0, len(src))
	for _, p := range src {
		dst = append(dst, cloneRecoveryPoint(p))
	}
	return dst
}

func cloneRecoveryPoint(p recovery.RecoveryPoint) recovery.RecoveryPoint {
	out := p

	out.StartedAt = cloneTimePtr(p.StartedAt)
	out.CompletedAt = cloneTimePtr(p.CompletedAt)
	out.SizeBytes = cloneInt64Ptr(p.SizeBytes)
	out.Verified = cloneBoolPtr(p.Verified)
	out.Encrypted = cloneBoolPtr(p.Encrypted)
	out.Immutable = cloneBoolPtr(p.Immutable)

	if p.SubjectRef != nil {
		ref := *p.SubjectRef
		if p.SubjectRef.Extra != nil {
			ref.Extra = cloneStringMap(p.SubjectRef.Extra)
		}
		out.SubjectRef = &ref
	}
	if p.RepositoryRef != nil {
		ref := *p.RepositoryRef
		if p.RepositoryRef.Extra != nil {
			ref.Extra = cloneStringMap(p.RepositoryRef.Extra)
		}
		out.RepositoryRef = &ref
	}
	if p.Details != nil {
		out.Details = cloneAnyMap(p.Details)
	}
	return out
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

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		// Values are primitives/slices in our mock payloads; shallow copy is sufficient.
		if s, ok := v.([]string); ok {
			dst[k] = append([]string(nil), s...)
			continue
		}
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
