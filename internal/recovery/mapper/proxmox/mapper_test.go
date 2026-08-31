package proxmoxmapper

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestSubjectResourceIDPrefersCanonicalIdentityAndFallsBackToSourceIdentity(t *testing.T) {
	t.Parallel()

	resourceType := unifiedresources.ResourceTypeVM
	sourceID := "pve-main:pve-a:100"
	// A node-scoped source triple falls back to the node-independent guest
	// derivation so registry-miss rows key identically to the live resource.
	fallback := unifiedresources.ProxmoxGuestCanonicalID(resourceType, "pve-main", 100)

	tests := []struct {
		name       string
		resourceID string
		sourceID   string
		want       string
	}{
		{
			name:       "canonical resource identity",
			resourceID: " vm-canonical ",
			sourceID:   sourceID,
			want:       "vm-canonical",
		},
		{
			name:     "provider source fallback",
			sourceID: " " + sourceID + " ",
			want:     fallback,
		},
		{
			name: "missing identity",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := subjectResourceID(resourceType, test.resourceID, test.sourceID); got != test.want {
				t.Fatalf("subjectResourceID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFromPVEGuestSnapshots_Empty(t *testing.T) {
	result := FromPVEGuestSnapshots(nil, nil)
	if result != nil {
		t.Errorf("FromPVEGuestSnapshots(nil, nil) = %v, want nil", result)
	}

	result = FromPVEGuestSnapshots([]models.GuestSnapshot{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPVEGuestSnapshots([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPVEGuestSnapshots_Single(t *testing.T) {
	snapshots := []models.GuestSnapshot{
		{
			ID:        "snapshot-1",
			VMID:      100,
			Node:      "pve1",
			Instance:  "pve-cluster",
			Name:      "web-server",
			Type:      "qemu",
			Time:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			SizeBytes: 1024,
		},
	}

	result := FromPVEGuestSnapshots(snapshots, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderProxmoxPVE {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderProxmoxPVE)
	}
	if p.Kind != recovery.KindSnapshot {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindSnapshot)
	}
	if p.Mode != recovery.ModeSnapshot {
		t.Errorf("Mode = %v, want %v", p.Mode, recovery.ModeSnapshot)
	}
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
	if p.Details == nil {
		t.Error("expected Details to be set")
	}
}

func TestFromPVEGuestSnapshots_WithGuestInfo(t *testing.T) {
	snapshots := []models.GuestSnapshot{
		{
			ID:       "snapshot-1",
			VMID:     100,
			Node:     "pve1",
			Instance: "pve-cluster",
			Name:     "web-server",
			Type:     "qemu",
			Time:     time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
	}

	guestInfoByKey := map[string]GuestInfo{
		"pve-cluster|pve1|100": {
			ResourceID:   "vm-unified-resource-1",
			SourceID:     "pve-cluster:pve1:100",
			ResourceType: unifiedresources.ResourceTypeVM,
			Name:         "web-server",
		},
	}

	result := FromPVEGuestSnapshots(snapshots, guestInfoByKey)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if got, want := p.SubjectResourceID, "vm-unified-resource-1"; got != want {
		t.Fatalf("SubjectResourceID = %q, want canonical resource ID %q", got, want)
	}
	if p.SubjectRef == nil || p.SubjectRef.ID != "pve-cluster:pve1:100" {
		t.Fatalf("SubjectRef = %#v, want provider-native source ID", p.SubjectRef)
	}
}

func TestFromPVEStorageBackups_Empty(t *testing.T) {
	result := FromPVEStorageBackups(nil, nil)
	if result != nil {
		t.Errorf("FromPVEStorageBackups(nil, nil) = %v, want nil", result)
	}

	result = FromPVEStorageBackups([]models.StorageBackup{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPVEStorageBackups([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPVEStorageBackups_Single(t *testing.T) {
	backups := []models.StorageBackup{
		{
			ID:       "backup-1",
			VMID:     100,
			Node:     "pve1",
			Instance: "pve-cluster",
			Storage:  "local",
			Type:     "qemu",
			Time:     time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Size:     2048,
		},
	}

	result := FromPVEStorageBackups(backups, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderProxmoxPVE {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderProxmoxPVE)
	}
	if p.Kind != recovery.KindBackup {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindBackup)
	}
	if p.SizeBytes == nil || *p.SizeBytes != 2048 {
		t.Errorf("SizeBytes = %v, want 2048", p.SizeBytes)
	}
}

func TestFromPVEStorageBackupsWithEvidenceAttachesCanonicalProviderScope(t *testing.T) {
	t.Parallel()

	ingestedAt := time.Date(2026, 8, 4, 8, 5, 0, 0, time.UTC)
	points, err := FromPVEStorageBackupsWithEvidence(
		[]models.StorageBackup{{
			ID:       "backup-1",
			VMID:     100,
			Node:     "pve1",
			Instance: "pve-cluster",
			Storage:  "local",
			Type:     "qemu",
			Time:     time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC),
		}},
		nil,
		ingestedAt,
	)
	if err != nil {
		t.Fatalf("FromPVEStorageBackupsWithEvidence() error = %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("points = %d, want 1", len(points))
	}
	point := points[0]
	if point.ProviderScope != "pve-cluster" {
		t.Fatalf("ProviderScope = %q, want pve-cluster", point.ProviderScope)
	}
	if point.Evidence == nil || point.Evidence.Source.Collector != "pve-backup-inventory" {
		t.Fatalf("Evidence = %#v, want PVE backup inventory evidence", point.Evidence)
	}
	if point.SubjectResourceID != unifiedresources.ProxmoxGuestCanonicalID(
		unifiedresources.ResourceTypeVM,
		"pve-cluster",
		100,
	) {
		t.Fatalf("SubjectResourceID = %q, want canonical guest ID", point.SubjectResourceID)
	}
}

func TestFromPVEBackupTasks_Empty(t *testing.T) {
	result := FromPVEBackupTasks(nil, nil)
	if result != nil {
		t.Errorf("FromPVEBackupTasks(nil, nil) = %v, want nil", result)
	}

	result = FromPVEBackupTasks([]models.BackupTask{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPVEBackupTasks([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPVEBackupTasks_Success(t *testing.T) {
	tasks := []models.BackupTask{
		{
			ID:        "task-1",
			VMID:      100,
			Node:      "pve1",
			Instance:  "pve-cluster",
			Type:      "vzdump",
			Status:    "ok",
			StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC),
			Size:      4096,
		},
	}

	result := FromPVEBackupTasks(tasks, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
	if p.Kind != recovery.KindOther {
		t.Errorf("Kind = %v, want task evidence kind %v", p.Kind, recovery.KindOther)
	}
}

func TestFromPVEBackupTasks_Failed(t *testing.T) {
	tasks := []models.BackupTask{
		{
			ID:        "task-1",
			VMID:      100,
			Node:      "pve1",
			Instance:  "pve-cluster",
			Type:      "vzdump",
			Status:    "error",
			StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC),
		},
	}

	result := FromPVEBackupTasks(tasks, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Outcome != recovery.OutcomeFailed {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeFailed)
	}
}

func TestFromPBSBackups_Empty(t *testing.T) {
	result := FromPBSBackups(nil, nil)
	if result != nil {
		t.Errorf("FromPBSBackups(nil, nil) = %v, want nil", result)
	}

	result = FromPBSBackups([]models.PBSBackup{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPBSBackups([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPBSBackups_Single(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-1",
			VMID:       "100",
			Instance:   "pbs1",
			Datastore:  "backup-store",
			BackupType: "vm",
			BackupTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Size:       8192,
			Verified:   true,
			Protected:  true,
		},
	}

	result := FromPBSBackups(backups, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderProxmoxPBS {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderProxmoxPBS)
	}
	if p.Kind != recovery.KindBackup {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindBackup)
	}
	if p.Mode != recovery.ModeRemote {
		t.Errorf("Mode = %v, want %v", p.Mode, recovery.ModeRemote)
	}
	if p.SizeBytes == nil || *p.SizeBytes != 8192 {
		t.Errorf("SizeBytes = %v, want 8192", p.SizeBytes)
	}
}

func TestFromPBSBackups_PrefersCommentNameWhenGuestIsUnresolved(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-legacy",
			VMID:       "140",
			Instance:   "pbs-docker",
			Namespace:  "pimox",
			Datastore:  "main",
			BackupType: "ct",
			BackupTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Comment:    "pulse-v4-prod, pi, 140",
		},
	}

	result := FromPBSBackups(backups, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}
	if result[0].SubjectRef == nil {
		t.Fatal("expected SubjectRef to be set")
	}
	if got := result[0].SubjectRef.Name; got != "pulse-v4-prod" {
		t.Fatalf("SubjectRef.Name = %q, want %q", got, "pulse-v4-prod")
	}
	if got := result[0].SubjectRef.ID; got != "140" {
		t.Fatalf("SubjectRef.ID = %q, want %q", got, "140")
	}
}

func TestFromPBSBackups_IgnoresNumericOnlyCommentName(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-numeric-comment",
			VMID:       "112",
			Instance:   "pbs-docker",
			Namespace:  "minipc",
			Datastore:  "main",
			BackupType: "ct",
			BackupTime: time.Date(2026, 3, 29, 3, 3, 31, 0, time.UTC),
			Comment:    "112",
		},
	}

	result := FromPBSBackups(backups, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}
	if result[0].SubjectRef == nil {
		t.Fatal("expected SubjectRef to be set")
	}
	if got := result[0].SubjectRef.Name; got != "112" {
		t.Fatalf("SubjectRef.Name = %q, want VMID fallback", got)
	}
	if got := result[0].Details["comment"]; got != "112" {
		t.Fatalf("Details[comment] = %#v, want raw numeric comment preserved", got)
	}
}

func TestFromPBSBackups_WithCandidates(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-1",
			VMID:       "100",
			Instance:   "pbs1",
			Datastore:  "backup-store",
			BackupType: "vm",
			BackupTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Size:       8192,
		},
	}

	candidatesByKey := map[string][]GuestCandidate{
		"vm:100": {
			{
				ResourceID:    "vm-unified-resource-1",
				SourceID:      "pve-cluster:pve1:100",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "web-server",
				InstanceName:  "pbs1",
				NodeName:      "pve1",
				VMID:          100,
				BackupTypeKey: "vm",
			},
		},
	}

	result := FromPBSBackups(backups, candidatesByKey)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if got, want := p.SubjectResourceID, "vm-unified-resource-1"; got != want {
		t.Fatalf("SubjectResourceID = %q, want canonical resource ID %q", got, want)
	}
}

func TestFromPBSBackups_DisambiguatesCandidatesByNamespace(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-minipc-ct-112",
			VMID:       "112",
			Instance:   "pbs-docker",
			Namespace:  "minipc",
			Datastore:  "main",
			BackupType: "ct",
			BackupTime: time.Date(2026, 3, 29, 3, 3, 31, 0, time.UTC),
			Comment:    "debian-go",
		},
	}

	candidatesByKey := map[string][]GuestCandidate{
		"ct:112": {
			{
				ResourceID:    "system-container-fb42a70d89bd20a6",
				SourceID:      "delly:minipc:112",
				ResourceType:  unifiedresources.ResourceTypeSystemContainer,
				DisplayName:   "debian-go",
				InstanceName:  "delly",
				NodeName:      "minipc",
				VMID:          112,
				BackupTypeKey: "ct",
			},
			{
				ResourceID:    "system-container-deadbeefdeadbeef",
				SourceID:      "other:pve-b:112",
				ResourceType:  unifiedresources.ResourceTypeSystemContainer,
				DisplayName:   "other-guest",
				InstanceName:  "other",
				NodeName:      "pve-b",
				VMID:          112,
				BackupTypeKey: "ct",
			},
		},
	}

	result := FromPBSBackups(backups, candidatesByKey)
	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	expectedRID := "system-container-fb42a70d89bd20a6"

	if got := result[0].SubjectResourceID; got != expectedRID {
		t.Fatalf("SubjectResourceID = %q, want %q", got, expectedRID)
	}
	if result[0].SubjectRef == nil || result[0].SubjectRef.Name != "debian-go" {
		t.Fatalf("SubjectRef = %#v, want linked debian-go guest", result[0].SubjectRef)
	}
}

func TestFromPBSBackupsWithEvidenceAddsProviderScopeAndCorrelation(t *testing.T) {
	t.Parallel()

	backupTime := time.Date(2026, 7, 19, 6, 0, 0, 0, time.UTC)
	ingestedAt := backupTime.Add(2 * time.Minute)
	backups := []models.PBSBackup{
		{
			ID:         "pbs-main:store-a:vm/100/2026-07-19T06:00:00Z",
			VMID:       "100",
			Instance:   "pbs-main",
			Datastore:  "store-a",
			BackupType: "vm",
			BackupTime: backupTime,
			Verified:   true,
		},
	}
	candidates := map[string][]GuestCandidate{
		"vm:100": {
			{
				ResourceID:   "vm-100",
				SourceID:     "pve-main:pve-a:100",
				ResourceType: unifiedresources.ResourceTypeVM,
				DisplayName:  "database",
				InstanceName: "pve-main",
				NodeName:     "pve-a",
				VMID:         100,
			},
		},
	}

	points, err := FromPBSBackupsWithEvidence(backups, candidates, ingestedAt)
	if err != nil {
		t.Fatalf("FromPBSBackupsWithEvidence() error = %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("points = %d, want 1", len(points))
	}
	point := points[0]
	if point.ProviderScope != "pbs-main" {
		t.Fatalf("provider scope = %q, want pbs-main", point.ProviderScope)
	}
	if point.Evidence == nil {
		t.Fatal("expected typed PBS evidence")
	}
	if point.Evidence.ObservedAt != backupTime {
		t.Fatalf("observedAt = %v, want %v", point.Evidence.ObservedAt, backupTime)
	}
	if point.Evidence.Correlation == nil {
		t.Fatal("expected auditable canonical guest correlation")
	}
	if point.Evidence.Correlation.CandidateCount != 1 {
		t.Fatalf(
			"candidate count = %d, want 1",
			point.Evidence.Correlation.CandidateCount,
		)
	}
	if err := point.Evidence.Validate(); err != nil {
		t.Fatalf("evidence Validate() error = %v", err)
	}
}

// Issue #1639: a root-namespace comment-less snapshot whose VMID exists on
// two clusters must link to the cluster its submission source (owner token,
// datastore, PBS instance) belongs to, learned from the batch's
// attributable backups — and must stay unlinked when the source is shared.
func TestFromPBSBackups_DisambiguatesCollisionVMIDBySubmissionSource(t *testing.T) {
	candidatesByKey := map[string][]GuestCandidate{
		"vm:173": {
			{
				ResourceID:    "vm-aaaaaaaaaaaaaaaa",
				SourceID:      "cluster-a:pve-a1:173",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "web-a",
				InstanceName:  "cluster-a",
				NodeName:      "pve-a1",
				VMID:          173,
				BackupTypeKey: "vm",
			},
			{
				ResourceID:    "vm-bbbbbbbbbbbbbbbb",
				SourceID:      "cluster-b:pve-b1:173",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "web-b",
				InstanceName:  "cluster-b",
				NodeName:      "pve-b1",
				VMID:          173,
				BackupTypeKey: "vm",
			},
		},
		"vm:100": {
			{
				ResourceID:    "vm-cccccccccccccccc",
				SourceID:      "cluster-a:pve-a1:100",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "db-a",
				InstanceName:  "cluster-a",
				NodeName:      "pve-a1",
				VMID:          100,
				BackupTypeKey: "vm",
			},
		},
		"vm:150": {
			{
				ResourceID:    "vm-dddddddddddddddd",
				SourceID:      "cluster-b:pve-b1:150",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "db-b",
				InstanceName:  "cluster-b",
				NodeName:      "pve-b1",
				VMID:          150,
				BackupTypeKey: "vm",
			},
		},
	}

	backups := []models.PBSBackup{
		// Teaching snapshots: unique VMIDs attribute each cluster's owner
		// token, which is also what makes both clusters visible — a cluster
		// with no attributable backup would keep the learner inconclusive.
		{
			ID:         "pbs-a-100",
			VMID:       "100",
			Instance:   "pbs-main",
			Datastore:  "store-a",
			Owner:      "cluster-a@pbs!token",
			BackupType: "vm",
			BackupTime: time.Date(2026, 7, 26, 1, 0, 0, 0, time.UTC),
		},
		{
			ID:         "pbs-b-150",
			VMID:       "150",
			Instance:   "pbs-main",
			Datastore:  "store-b",
			Owner:      "cluster-b@pbs!token",
			BackupType: "vm",
			BackupTime: time.Date(2026, 7, 26, 2, 0, 0, 0, time.UTC),
		},
		// Collision snapshot: no namespace, no comment, cluster-a's source.
		{
			ID:         "pbs-a-173",
			VMID:       "173",
			Instance:   "pbs-main",
			Datastore:  "store-a",
			Owner:      "cluster-a@pbs!token",
			BackupType: "vm",
			BackupTime: time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC),
		},
		// Collision snapshot from a source never attributed to any cluster:
		// must stay unlinked rather than guess.
		{
			ID:         "pbs-x-173",
			VMID:       "173",
			Instance:   "pbs-main",
			Datastore:  "store-x",
			Owner:      "mystery@pbs!token",
			BackupType: "vm",
			BackupTime: time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC),
		},
	}

	result := FromPBSBackups(backups, candidatesByKey)
	if len(result) != 4 {
		t.Fatalf("expected 4 points, got %d", len(result))
	}

	byID := map[string]int{}
	for i, p := range result {
		byID[p.ID] = i
	}

	linked := result[byID["pbs-backup:pbs-a-173"]]
	if linked.SubjectResourceID != "vm-aaaaaaaaaaaaaaaa" {
		t.Fatalf("collision snapshot SubjectResourceID = %q, want cluster-a guest", linked.SubjectResourceID)
	}

	unlinked := result[byID["pbs-backup:pbs-x-173"]]
	if unlinked.SubjectResourceID != "" {
		t.Fatalf("unattributable snapshot linked to %q, want unlinked", unlinked.SubjectResourceID)
	}
}

// Issue #1639 hardening: the learner only ever sees clusters that already had
// a backup attributed to them, so a cluster with no attributable backup is
// invisible and a source it shares looks exclusive to the cluster that was
// visible. The collision snapshot must stay unlinked rather than be handed to
// the visible cluster.
func TestFromPBSBackups_InvisibleClusterKeepsCollisionSnapshotUnlinked(t *testing.T) {
	candidatesByKey := map[string][]GuestCandidate{
		"vm:173": {
			{
				ResourceID:    "vm-aaaaaaaaaaaaaaaa",
				SourceID:      "cluster-a:pve-a1:173",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "web-a",
				InstanceName:  "cluster-a",
				NodeName:      "pve-a1",
				VMID:          173,
				BackupTypeKey: "vm",
			},
			{
				ResourceID:    "vm-bbbbbbbbbbbbbbbb",
				SourceID:      "cluster-b:pve-b1:173",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "web-b",
				InstanceName:  "cluster-b",
				NodeName:      "pve-b1",
				VMID:          173,
				BackupTypeKey: "vm",
			},
		},
		"vm:100": {
			{
				ResourceID:    "vm-cccccccccccccccc",
				SourceID:      "cluster-a:pve-a1:100",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "db-a",
				InstanceName:  "cluster-a",
				NodeName:      "pve-a1",
				VMID:          100,
				BackupTypeKey: "vm",
			},
		},
	}

	backups := []models.PBSBackup{
		{
			ID:         "pbs-a-100",
			VMID:       "100",
			Instance:   "pbs-main",
			Datastore:  "backups",
			Owner:      "shared@pbs!token",
			BackupType: "vm",
			BackupTime: time.Date(2026, 7, 26, 1, 0, 0, 0, time.UTC),
		},
		// Authored by cluster-b, which owns no attributable guest here.
		{
			ID:         "pbs-b-173",
			VMID:       "173",
			Instance:   "pbs-main",
			Datastore:  "backups",
			Owner:      "shared@pbs!token",
			BackupType: "vm",
			BackupTime: time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC),
		},
	}

	result := FromPBSBackups(backups, candidatesByKey)
	if len(result) != 2 {
		t.Fatalf("expected 2 points, got %d", len(result))
	}
	for _, point := range result {
		if point.ID != "pbs-backup:pbs-b-173" {
			continue
		}
		if point.SubjectResourceID != "" {
			t.Fatalf("collision snapshot linked to %q, want unlinked while cluster-b is invisible", point.SubjectResourceID)
		}
	}
}

// An in-flight PBS snapshot (no manifest yet) must map to a running recovery
// point: no completion time, no verification verdict, and never
// OutcomeSuccess - otherwise posture freshness treats a backup that merely
// STARTED as one that finished.
func TestFromPBSBackups_InProgressMapsToRunning(t *testing.T) {
	started := time.Date(2026, 8, 20, 9, 39, 1, 0, time.UTC)
	backups := []models.PBSBackup{
		{
			ID:         "pbs-verdeclose-main-pve-pc-vm-117-1787221141",
			VMID:       "117",
			Instance:   "verdeclose",
			Datastore:  "main",
			Namespace:  "pve-pc",
			BackupType: "vm",
			BackupTime: started,
			Files:      []string{"qemu-server.conf.blob"},
			InProgress: true,
		},
	}

	result := FromPBSBackups(backups, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Outcome != recovery.OutcomeRunning {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeRunning)
	}
	if p.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil for a running backup", p.CompletedAt)
	}
	if p.StartedAt == nil || !p.StartedAt.Equal(started) {
		t.Errorf("StartedAt = %v, want %v", p.StartedAt, started)
	}
	if p.Verified != nil {
		t.Errorf("Verified = %v, want nil: verification is not applicable yet", *p.Verified)
	}
	if inProgress, _ := p.Details["inProgress"].(bool); !inProgress {
		t.Error("Details[inProgress] = false, want true")
	}
}

func TestFromPBSBackups_TerminalIncompleteMapsToFailed(t *testing.T) {
	started := time.Date(2026, 8, 30, 23, 15, 0, 0, time.UTC)
	result := FromPBSBackups([]models.PBSBackup{{
		ID:                    "pbs-offsite-main-vm-117-terminal",
		VMID:                  "117",
		Instance:              "offsite",
		Datastore:             "main",
		BackupType:            "vm",
		BackupTime:            started,
		Files:                 []string{"qemu-server.conf.blob"},
		InProgress:            true,
		WriteActivityObserved: true,
		WriteActive:           false,
	}}, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}
	p := result[0]
	if p.Outcome != recovery.OutcomeFailed {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeFailed)
	}
	if p.Verified != nil {
		t.Errorf("Verified = %v, want nil for an incomplete artifact", *p.Verified)
	}
	if p.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil because PBS did not report a terminal time for the artifact", p.CompletedAt)
	}
}

func TestFromPVEStorageBackups_InProgressMapsToRunning(t *testing.T) {
	started := time.Date(2026, 8, 20, 9, 39, 1, 0, time.UTC)
	backups := []models.StorageBackup{
		{
			ID:       "pve-pc-local:backup/vzdump-qemu-117.vma.zst",
			Instance: "pve-pc",
			Node:     "pve-pc",
			Type:     "qemu",
			VMID:     117,
			Time:     started,
			Size:     7 << 30,
			Storage:  "local",
			// Partial archive a running vzdump task is still writing.
			InProgress: true,
		},
	}

	result := FromPVEStorageBackups(backups, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Outcome != recovery.OutcomeRunning {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeRunning)
	}
	if p.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil for a running backup", p.CompletedAt)
	}
	if p.Verified != nil {
		t.Errorf("Verified = %v, want nil for a running backup", *p.Verified)
	}
}
