package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestStoreProtectionPosturePersistsProviderAwareTruth(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Millisecond)
	backupTime := now.Add(-time.Hour)
	verified := true
	point := recovery.RecoveryPoint{
		ID:                   "pbs-backup:vm-100",
		Provider:             recovery.ProviderProxmoxPBS,
		Kind:                 recovery.KindBackup,
		Mode:                 recovery.ModeRemote,
		Outcome:              recovery.OutcomeSuccess,
		CompletedAt:          &backupTime,
		Verified:             &verified,
		SubjectResourceID:    "resource:vm-100",
		RepositoryResourceID: "resource:pbs-store-a",
		ProviderScope:        "pbs-main",
	}
	evidence, err := recovery.NewRecoveryPointEvidence(
		point,
		"pbs-backup-inventory",
		now,
	)
	if err != nil {
		t.Fatalf("NewRecoveryPointEvidence() error = %v", err)
	}
	point.Evidence = evidence

	complete, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		"pbs-main",
		recovery.OutcomeSuccess,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		true,
		now,
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("complete observation error = %v", err)
	}
	if err := store.UpsertProtectionProviderObservations(
		context.Background(),
		[]recovery.ProtectionProviderObservation{complete},
	); err != nil {
		t.Fatalf("UpsertProtectionProviderObservations() error = %v", err)
	}
	if err := store.UpsertPoints(
		context.Background(),
		[]recovery.RecoveryPoint{point},
	); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	got, total, err := store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{"resource:vm-100", "resource:vm-404"},
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures() error = %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("postures total=%d len=%d, want 2/2", total, len(got))
	}
	byID := map[string]recovery.ProtectionPosture{}
	for _, posture := range got {
		byID[posture.SubjectResourceID] = posture
	}
	protected := byID["resource:vm-100"]
	if protected.State != recovery.ProtectionStateProtected {
		t.Fatalf("vm-100 state = %q, want protected; posture=%#v", protected.State, protected)
	}
	if protected.Coverage != recovery.ProtectionCoverageComplete {
		t.Fatalf("vm-100 coverage = %q, want complete", protected.Coverage)
	}
	if protected.Verification != recovery.ProtectionVerificationVerified {
		t.Fatalf("vm-100 verification = %q, want verified", protected.Verification)
	}
	if len(protected.ProviderStates) != 1 {
		t.Fatalf("vm-100 provider states = %d, want 1", len(protected.ProviderStates))
	}
	if byID["resource:vm-404"].State != recovery.ProtectionStateUnknown {
		t.Fatalf(
			"vm-404 state = %q, want unknown",
			byID["resource:vm-404"].State,
		)
	}

	attentionList, total, err := store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			State: recovery.ProtectionStateProtected,
			Page:  1,
			Limit: 10,
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures(protected) error = %v", err)
	}
	if total != 1 || len(attentionList) != 1 {
		t.Fatalf("protected postures total=%d len=%d, want 1/1", total, len(attentionList))
	}

	partialAt := now.Add(time.Minute)
	partial, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		"pbs-main",
		recovery.OutcomeWarning,
		recovery.ProtectionHistoryPartial,
		operationaltrust.EvidencePermissionsPartial,
		true,
		partialAt,
		partialAt,
		&operationaltrust.EvidenceReason{
			Code:    "pbs_partial_enumeration",
			Message: "One PBS datastore could not be enumerated.",
		},
	)
	if err != nil {
		t.Fatalf("partial observation error = %v", err)
	}
	if err := store.UpsertProtectionProviderObservations(
		context.Background(),
		[]recovery.ProtectionProviderObservation{partial},
	); err != nil {
		t.Fatalf("UpsertProtectionProviderObservations(partial) error = %v", err)
	}
	got, _, err = store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{"resource:vm-100"},
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures(after partial) error = %v", err)
	}
	if got[0].State != recovery.ProtectionStateAttention {
		t.Fatalf("state after partial = %q, want attention", got[0].State)
	}
	if got[0].Coverage != recovery.ProtectionCoveragePartial {
		t.Fatalf("coverage after partial = %q, want partial", got[0].Coverage)
	}
}

func TestStoreProtectionPostureMovesWhenPointIdentityIsCorrected(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Millisecond)
	completedAt := now.Add(-time.Hour)
	point := recovery.RecoveryPoint{
		ID:                "pbs-backup:vm-100",
		Provider:          recovery.ProviderProxmoxPBS,
		Kind:              recovery.KindBackup,
		Mode:              recovery.ModeRemote,
		Outcome:           recovery.OutcomeSuccess,
		CompletedAt:       &completedAt,
		SubjectResourceID: "vm-double-hashed",
		ProviderScope:     "pbs-main",
	}
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints(old identity) error = %v", err)
	}

	point.SubjectResourceID = "vm-canonical"
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints(corrected identity) error = %v", err)
	}

	var oldCount int
	if err := store.db.QueryRow(
		`SELECT COUNT(*) FROM protection_postures WHERE subject_resource_id = ?`,
		"vm-double-hashed",
	).Scan(&oldCount); err != nil {
		t.Fatalf("count obsolete posture: %v", err)
	}
	if oldCount != 0 {
		t.Fatalf("obsolete posture count = %d, want 0", oldCount)
	}

	var newCount int
	if err := store.db.QueryRow(
		`SELECT COUNT(*) FROM protection_postures WHERE subject_resource_id = ?`,
		"vm-canonical",
	).Scan(&newCount); err != nil {
		t.Fatalf("count corrected posture: %v", err)
	}
	if newCount != 1 {
		t.Fatalf("corrected posture count = %d, want 1", newCount)
	}
}

func TestStoreProtectionSchemaMigratesLegacyRecoveryDatabase(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "recovery.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	for _, table := range []string{
		"protection_provider_observations",
		"protection_postures",
	} {
		var name string
		if err := store.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&name); err != nil {
			t.Fatalf("lookup table %s: %v", table, err)
		}
		if name != table {
			t.Fatalf("table = %q, want %q", name, table)
		}
	}

	columns := map[string]bool{}
	rows, err := store.db.Query(`PRAGMA table_info(recovery_points)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue any
			primaryKey   int
		)
		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		columns[name] = true
	}
	for _, column := range []string{"provider_scope", "evidence_id", "evidence_json"} {
		if !columns[column] {
			t.Fatalf("missing migrated recovery_points column %q", column)
		}
	}
}

func TestStoreRequestedProtectionPosturesReevaluateAtReadTime(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Millisecond)
	oldBackup := now.Add(-8 * 24 * time.Hour)
	verified := true
	point := recovery.RecoveryPoint{
		ID:                "pbs-backup:stale-vm",
		Provider:          recovery.ProviderProxmoxPBS,
		Kind:              recovery.KindBackup,
		Mode:              recovery.ModeRemote,
		Outcome:           recovery.OutcomeSuccess,
		CompletedAt:       &oldBackup,
		Verified:          &verified,
		SubjectResourceID: "resource:stale-vm",
		ProviderScope:     "pbs-main",
	}
	evidence, err := recovery.NewRecoveryPointEvidence(point, "pbs-backup-inventory", now)
	if err != nil {
		t.Fatalf("NewRecoveryPointEvidence() error = %v", err)
	}
	point.Evidence = evidence
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		"pbs-main",
		recovery.OutcomeSuccess,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		true,
		now,
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation() error = %v", err)
	}
	if err := store.UpsertProtectionProviderObservations(
		context.Background(),
		[]recovery.ProtectionProviderObservation{observation},
	); err != nil {
		t.Fatalf("UpsertProtectionProviderObservations() error = %v", err)
	}
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	// Corrupt the materialized snapshot into a reassuring answer. A requested
	// batch must derive from points + current provider evidence instead of
	// trusting a posture that can age between collection cycles.
	fake := recovery.DeriveProtectionPostureAt(
		point.SubjectResourceID,
		nil,
		recovery.DefaultProtectionPosturePolicy,
		now,
	)
	fake.State = recovery.ProtectionStateProtected
	fake.Freshness = recovery.ProtectionFreshnessCurrent
	fake.Verification = recovery.ProtectionVerificationVerified
	fake.Coverage = recovery.ProtectionCoverageComplete
	fake.Explanation = "incorrect stored posture"
	fakeJSON, err := json.Marshal(fake)
	if err != nil {
		t.Fatalf("json.Marshal(fake) error = %v", err)
	}
	if _, err := store.db.Exec(
		`UPDATE protection_postures SET state = 'protected', posture_json = ?`,
		string(fakeJSON),
	); err != nil {
		t.Fatalf("update materialized posture: %v", err)
	}

	got, _, err := store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{point.SubjectResourceID},
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("postures len = %d, want 1", len(got))
	}
	if got[0].State != recovery.ProtectionStateAttention {
		t.Fatalf("state = %q, want attention; posture=%#v", got[0].State, got[0])
	}
	if got[0].Freshness != recovery.ProtectionFreshnessStale {
		t.Fatalf("freshness = %q, want stale", got[0].Freshness)
	}
	if got[0].Explanation == "incorrect stored posture" {
		t.Fatal("requested posture trusted stale materialized JSON")
	}
}

func TestStoreProtectionPostureBatchUsesIndexedBoundedReads(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	rows, err := store.db.Query(`
		EXPLAIN QUERY PLAN
		SELECT subject_key, subject_resource_id
		FROM protection_postures
		WHERE subject_resource_id IN (?, ?, ?)
	`, "resource:a", "resource:b", "resource:c")
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN error = %v", err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan query plan: %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("query plan rows: %v", err)
	}
	plan := strings.Join(details, "\n")
	if !strings.Contains(plan, "SEARCH protection_postures") ||
		!strings.Contains(plan, "subject_resource_id=?") {
		t.Fatalf("posture batch query is not index-backed:\n%s", plan)
	}
}

func TestStoreProtectionEvidenceRetentionRefreshesMaterializedPosture(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	store.retention = time.Hour
	store.lastPrune = time.Now().UTC()

	now := time.Now().UTC().Truncate(time.Millisecond)
	old := now.Add(-2 * time.Hour)
	point := recovery.RecoveryPoint{
		ID:                "pbs-backup:expired",
		Provider:          recovery.ProviderProxmoxPBS,
		Kind:              recovery.KindBackup,
		Mode:              recovery.ModeRemote,
		Outcome:           recovery.OutcomeSuccess,
		CompletedAt:       &old,
		SubjectResourceID: "resource:expired",
		ProviderScope:     "pbs-main",
	}
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		"pbs-main",
		recovery.OutcomeSuccess,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		false,
		old,
		old,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation() error = %v", err)
	}
	if err := store.UpsertProtectionProviderObservations(
		context.Background(),
		[]recovery.ProtectionProviderObservation{observation},
	); err != nil {
		t.Fatalf("UpsertProtectionProviderObservations() error = %v", err)
	}
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	store.lastPrune = time.Time{}
	store.maybePrune(context.Background())

	for _, table := range []string{
		"recovery_points",
		"protection_provider_observations",
		"protection_postures",
	} {
		var count int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0 after retention prune", table, count)
		}
	}
}

func TestStoreProtectionMetadataBackfillAddsScopeAndTypedEvidence(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	observedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	updatedAt := observedAt.Add(time.Minute)
	if _, err := store.db.Exec(`
		INSERT INTO recovery_points (
			id, provider, kind, mode, outcome,
			completed_at_ms, subject_key, subject_resource_id,
			repository_ref_json, created_at_ms, updated_at_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"pbs-backup:legacy",
		string(recovery.ProviderProxmoxPBS),
		string(recovery.KindBackup),
		string(recovery.ModeRemote),
		string(recovery.OutcomeSuccess),
		observedAt.UnixMilli(),
		"res:resource:legacy",
		"resource:legacy",
		`{"type":"proxmox-pbs-datastore","namespace":"pbs-main","name":"store-a"}`,
		observedAt.UnixMilli(),
		updatedAt.UnixMilli(),
	); err != nil {
		t.Fatalf("insert legacy recovery point: %v", err)
	}

	if err := store.BackfillProtectionMetadata(context.Background()); err != nil {
		t.Fatalf("BackfillProtectionMetadata() error = %v", err)
	}

	var providerScope, evidenceID, evidenceRaw string
	if err := store.db.QueryRow(`
		SELECT provider_scope, evidence_id, evidence_json
		FROM recovery_points
		WHERE id = 'pbs-backup:legacy'
	`).Scan(&providerScope, &evidenceID, &evidenceRaw); err != nil {
		t.Fatalf("read migrated recovery point: %v", err)
	}
	if providerScope != "pbs-main" {
		t.Fatalf("provider_scope = %q, want pbs-main", providerScope)
	}
	var evidence operationaltrust.EvidenceEnvelope
	if err := json.Unmarshal([]byte(evidenceRaw), &evidence); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if evidenceID == "" || evidence.ID != evidenceID {
		t.Fatalf("evidence id mismatch column=%q payload=%q", evidenceID, evidence.ID)
	}
	if evidence.Source.Collector != "recovery-point-migration" {
		t.Fatalf("collector = %q, want recovery-point-migration", evidence.Source.Collector)
	}
	if !evidence.ObservedAt.Equal(observedAt) {
		t.Fatalf("observedAt = %s, want %s", evidence.ObservedAt, observedAt)
	}
	if !evidence.IngestedAt.Equal(updatedAt) {
		t.Fatalf("ingestedAt = %s, want %s", evidence.IngestedAt, updatedAt)
	}
}
