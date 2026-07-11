package unifiedresources

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func atomicLifecycleTestRecord(id string, state ActionState) ActionAuditRecord {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	actor := ActionActor{SubjectID: "agent:test", Kind: ActionActorService, CredentialID: "service:test", OrgID: "default"}
	return ActionAuditRecord{
		ID: id, CreatedAt: now, UpdatedAt: now, State: state,
		Request: ActionRequest{RequestID: "req-" + id, ResourceID: "vm:42", CapabilityName: "restart", Reason: "atomic lifecycle proof", RequestedBy: "agent:test", Actor: actor},
		Plan:    ActionPlan{ActionID: id, RequestID: "req-" + id, Allowed: true, RequiresApproval: state == ActionStatePending, ApprovalPolicy: ApprovalAdmin, ApprovalRequirement: ApprovalRequirementForFloor(ApprovalAdmin), PlannedAt: now, ExpiresAt: now.Add(time.Hour), ResourceVersion: "resource:sha256:test", PolicyVersion: "policy:sha256:test", PlanHash: "sha256:" + id},
		Origin:  &ActionOrigin{Surface: "patrol", FindingID: "finding-1", InvestigationID: "inv-1", ProposalID: "proposal-1"},
	}
}

func atomicLifecycleInitialEvents(record ActionAuditRecord) []ActionLifecycleEvent {
	events := []ActionLifecycleEvent{{ActionID: record.ID, Timestamp: record.CreatedAt, State: ActionStatePlanned, Actor: record.Request.RequestedBy, Message: "Action plan created."}}
	if record.State == ActionStatePending {
		events = append(events, ActionLifecycleEvent{ActionID: record.ID, Timestamp: record.CreatedAt, State: ActionStatePending, Actor: record.Request.RequestedBy, Message: "Action is waiting for approval before execution."})
	}
	return events
}

func TestMemoryStoreCreateActionAuditConcurrentReturnsCurrent(t *testing.T) {
	store := NewMemoryStore()
	record := atomicLifecycleTestRecord("act_memory_create", ActionStatePending)
	start := make(chan struct{})
	type result struct {
		current ActionAuditRecord
		created bool
		err     error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			current, created, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record))
			results <- result{current: current, created: created, err: err}
		}()
	}
	close(start)
	createdCount := 0
	for range 2 {
		got := <-results
		if got.err != nil {
			t.Fatalf("CreateActionAudit: %v", got.err)
		}
		if got.created {
			createdCount++
		}
		if got.current.State != ActionStatePending {
			t.Fatalf("current state = %q", got.current.State)
		}
	}
	if createdCount != 1 {
		t.Fatalf("created count = %d, want 1", createdCount)
	}
	events, err := store.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
	if err != nil || len(events) != 2 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
}

func TestMemoryStoreActionTransitionsAreMonotonic(t *testing.T) {
	store := NewMemoryStore()
	record := atomicLifecycleTestRecord("act_memory_terminal", ActionStatePending)
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	approved, event, err := ApplyActionDecision(record, testBoundActionApproval(record, "operator", MethodSession, OutcomeApproved, "", record.CreatedAt.Add(time.Minute)), record.CreatedAt.Add(time.Minute))
	if err != nil || store.RecordActionDecision(approved, event) != nil {
		t.Fatalf("approve: %v", err)
	}
	started, startEvent, err := BeginActionExecution(approved, "operator", record.CreatedAt.Add(2*time.Minute))
	if err != nil || store.RecordActionExecutionStart(started, startEvent) != nil {
		t.Fatalf("start: %v", err)
	}
	completed, doneEvent, err := CompleteActionExecution(started, &ExecutionResult{Success: true}, "operator", record.CreatedAt.Add(3*time.Minute))
	if err != nil || store.RecordActionExecutionResult(completed, doneEvent) != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := store.RecordActionDecision(approved, event); !errors.Is(err, ErrActionDecisionRevisionConflict) {
		t.Fatalf("terminal rewind error=%v", err)
	}
}

func TestMemoryStorePolicyExecutionStartRequiresAuthorizationLease(t *testing.T) {
	store := NewMemoryStore()
	record := atomicLifecycleTestRecord("act_memory_policy_lease", ActionStatePlanned)
	record.Plan.RequiresApproval = false
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	record.State = ActionStateExecuting
	record.Approvals = []ActionApprovalRecord{{Actor: "pulse_patrol_policy", Method: MethodPolicy, Outcome: OutcomeApproved}}
	approval := ActionLifecycleEvent{ActionID: record.ID, Timestamp: record.CreatedAt.Add(time.Minute), State: ActionStateApproved, Actor: "pulse_patrol_policy", Message: "Policy authorization approved action."}
	executing := ActionLifecycleEvent{ActionID: record.ID, Timestamp: record.CreatedAt.Add(time.Minute), State: ActionStateExecuting, Actor: "pulse_patrol_policy", Message: "Action execution started."}
	if err := store.RecordActionPolicyExecutionStart(record, approval, executing); !errors.Is(err, ErrActionPolicyAuthorizationInvalid) {
		t.Fatalf("error = %v, want ErrActionPolicyAuthorizationInvalid", err)
	}
	current, found, err := store.GetActionAudit(record.ID)
	if err != nil || !found {
		t.Fatalf("GetActionAudit found=%v err=%v", found, err)
	}
	if current.State != ActionStatePlanned || len(current.Approvals) != 0 {
		t.Fatalf("failed admission mutated audit: state=%q approvals=%d", current.State, len(current.Approvals))
	}
}

func TestMemoryStoreConcurrentExecutionStartHasOneCASWinner(t *testing.T) {
	store := NewMemoryStore()
	record := atomicLifecycleTestRecord("act_memory_execute", ActionStatePlanned)
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	started, event, err := BeginActionExecution(record, "operator", record.CreatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() { <-start; errs <- store.RecordActionExecutionStart(started, event) }()
	}
	close(start)
	successes := 0
	for range 2 {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("execution CAS winners = %d, want 1", successes)
	}
}

func TestSQLiteStoreCreateActionAuditConcurrentAcrossTwoInstances(t *testing.T) {
	dir := t.TempDir()
	first, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	record := atomicLifecycleTestRecord("act_sqlite_create", ActionStatePending)
	start := make(chan struct{})
	created := make(chan bool, 2)
	errs := make(chan error, 2)
	for _, store := range []*SQLiteResourceStore{first, second} {
		go func(s *SQLiteResourceStore) {
			<-start
			_, ok, err := s.CreateActionAudit(record, atomicLifecycleInitialEvents(record))
			created <- ok
			errs <- err
		}(store)
	}
	close(start)
	createdCount := 0
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		if <-created {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created count=%d, want 1", createdCount)
	}
	events, err := first.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
	if err != nil || len(events) != 2 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
}

func TestSQLiteStoreActionTransitionCASAcrossTwoInstances(t *testing.T) {
	dir := t.TempDir()
	first, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	record := atomicLifecycleTestRecord("act_sqlite_decide", ActionStatePending)
	if _, _, err := first.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	approved, approvedEvent, _ := ApplyActionDecision(record, testBoundActionApproval(record, "one", MethodSession, OutcomeApproved, "", record.CreatedAt.Add(time.Minute)), record.CreatedAt.Add(time.Minute))
	rejected, rejectedEvent, _ := ApplyActionDecision(record, testBoundActionApproval(record, "two", MethodSession, OutcomeRejected, "", record.CreatedAt.Add(time.Minute)), record.CreatedAt.Add(time.Minute))
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() { <-start; errs <- first.RecordActionDecision(approved, approvedEvent) }()
	go func() { <-start; errs <- second.RecordActionDecision(rejected, rejectedEvent) }()
	close(start)
	successes := 0
	for range 2 {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("decision CAS winners=%d, want 1", successes)
	}
}

func TestSQLiteStoreConcurrentExecutionStartAcrossTwoInstancesHasOneWinner(t *testing.T) {
	dir := t.TempDir()
	first, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	record := atomicLifecycleTestRecord("act_sqlite_execute", ActionStatePlanned)
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	if _, _, err := first.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	started, event, _ := BeginActionExecution(record, "operator", record.CreatedAt.Add(time.Minute))
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, store := range []*SQLiteResourceStore{first, second} {
		go func(s *SQLiteResourceStore) { <-start; errs <- s.RecordActionExecutionStart(started, event) }(store)
	}
	close(start)
	successes := 0
	for range 2 {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("execution CAS winners=%d, want 1", successes)
	}
}

func TestSQLiteStoreCreateActionAuditRollsBackWhenInitialEventInsertFails(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.db.Exec(`CREATE TRIGGER fail_initial_event BEFORE INSERT ON action_lifecycle_events BEGIN SELECT RAISE(ABORT, 'forced event failure'); END`); err != nil {
		t.Fatal(err)
	}
	record := atomicLifecycleTestRecord("act_create_rollback", ActionStatePending)
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err == nil {
		t.Fatal("expected creation failure")
	}
	if _, found, err := store.GetActionAudit(record.ID); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestSQLiteStoreActionDecisionRollsBackWhenDecisionEventInsertFails(t *testing.T) {
	store := newTestStore(t)
	record := atomicLifecycleTestRecord("act_transition_rollback", ActionStatePending)
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_decision_event BEFORE INSERT ON action_lifecycle_events WHEN NEW.kind = 'decision' BEGIN SELECT RAISE(ABORT, 'forced decision event failure'); END`); err != nil {
		t.Fatal(err)
	}
	approved, event, _ := ApplyActionDecision(record, testBoundActionApproval(record, "operator", MethodSession, OutcomeApproved, "", record.CreatedAt.Add(time.Minute)), record.CreatedAt.Add(time.Minute))
	if err := store.RecordActionDecision(approved, event); err == nil {
		t.Fatal("expected transition failure")
	}
	current, found, err := store.GetActionAudit(record.ID)
	if err != nil || !found || current.State != ActionStatePending || current.DecisionRevision != 0 || len(current.Approvals) != 0 {
		t.Fatalf("current=%#v found=%v err=%v", current, found, err)
	}
	events, _ := store.GetActionLifecycleEvents(record.ID, time.Time{}, 20)
	if len(events) != 2 {
		t.Fatalf("decision-event failure leaked events: %#v", events)
	}
}

func TestSQLiteStoreActionDecisionRollsBackWhenResultingTransitionInsertFails(t *testing.T) {
	store := newTestStore(t)
	record := atomicLifecycleTestRecord("act_transition_rollback", ActionStatePending)
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_resulting_transition BEFORE INSERT ON action_lifecycle_events WHEN NEW.kind = 'transition' AND NEW.state = 'approved' BEGIN SELECT RAISE(ABORT, 'forced transition event failure'); END`); err != nil {
		t.Fatal(err)
	}
	approved, event, _ := ApplyActionDecision(record, testBoundActionApproval(record, "operator", MethodSession, OutcomeApproved, "", record.CreatedAt.Add(time.Minute)), record.CreatedAt.Add(time.Minute))
	if err := store.RecordActionDecision(approved, event); err == nil {
		t.Fatal("expected resulting transition failure")
	}
	current, found, err := store.GetActionAudit(record.ID)
	if err != nil || !found || current.State != ActionStatePending || current.DecisionRevision != 0 || len(current.Approvals) != 0 {
		t.Fatalf("current=%#v found=%v err=%v", current, found, err)
	}
	events, _ := store.GetActionLifecycleEvents(record.ID, time.Time{}, 20)
	if len(events) != 2 {
		t.Fatalf("transition-event failure leaked events: %#v", events)
	}
}

func TestSQLiteStoreLifecycleRestartPreservesMonotonicState(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	record := atomicLifecycleTestRecord("act_restart_terminal", ActionStatePlanned)
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	started, startEvent, _ := BeginActionExecution(record, "operator", record.CreatedAt.Add(time.Minute))
	if err := store.RecordActionExecutionStart(started, startEvent); err != nil {
		t.Fatal(err)
	}
	completed, doneEvent, _ := CompleteActionExecution(started, &ExecutionResult{Success: true}, "operator", record.CreatedAt.Add(2*time.Minute))
	if err := store.RecordActionExecutionResult(completed, doneEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	current, created, err := reopened.CreateActionAudit(record, atomicLifecycleInitialEvents(record))
	if err != nil || created || current.State != ActionStateCompleted {
		t.Fatalf("current=%#v created=%v err=%v", current, created, err)
	}
}

func TestSQLiteStoreRestartDoesNotReadmitExecutingAction(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	record := atomicLifecycleTestRecord("act_restart_executing", ActionStatePlanned)
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	started, event, _ := BeginActionExecution(record, "operator", record.CreatedAt.Add(time.Minute))
	if err := store.RecordActionExecutionStart(started, event); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.RecordActionExecutionStart(started, event); !errors.Is(err, ErrActionAlreadyExecuting) {
		t.Fatalf("error=%v", err)
	}
}

func TestSQLiteActionLifecycleMigrationRetainsHistoricalDuplicatesAndRestoresTransitionUniqueness(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resources", "unified_resources.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE action_lifecycle_events (id INTEGER PRIMARY KEY AUTOINCREMENT, action_id TEXT NOT NULL, timestamp DATETIME NOT NULL, state TEXT NOT NULL, actor TEXT, message TEXT)`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.Exec(`INSERT INTO action_lifecycle_events (action_id, timestamp, state, actor, message) VALUES (?, ?, ?, '', '')`, "act_event_dedupe", time.Now().UTC(), string(ActionStatePlanned)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	events, err := store.GetActionLifecycleEvents("act_event_dedupe", time.Time{}, 10)
	if err != nil || len(events) != 2 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	kinds := map[ActionLifecycleEventKind]int{}
	for _, event := range events {
		kinds[event.Kind]++
	}
	if kinds[ActionLifecycleEventTransition] != 1 || kinds[ActionLifecycleEventLegacy] != 1 {
		t.Fatalf("migrated historical event kinds=%v, want one transition and one retained legacy fact", kinds)
	}
	duplicateTransition := ActionLifecycleEvent{ActionID: "act_event_dedupe", Timestamp: time.Now().UTC(), State: ActionStatePlanned}
	if err := store.RecordActionLifecycleEvent(duplicateTransition); err == nil {
		t.Fatal("duplicate planned transition should remain rejected after migration")
	}
}

func TestSanitizeOrgID_AllowsSafeChars(t *testing.T) {
	in := "Acme_Org-123"
	if got := sanitizeOrgID(in); got != in {
		t.Fatalf("sanitizeOrgID(%q) = %q, want %q", in, got, in)
	}
}

func TestSanitizeOrgID_StripsUnsafeCharsAndBoundsLength(t *testing.T) {
	in := "../../../../tenant?mode=memory&_pragma=trusted_schema(OFF)#frag"
	got := sanitizeOrgID(in)

	if got == "" {
		t.Fatal("expected non-empty sanitized org ID")
	}
	if len(got) > maxOrgIDLength {
		t.Fatalf("sanitizeOrgID length = %d, want <= %d", len(got), maxOrgIDLength)
	}
	if strings.ContainsAny(got, "/\\?&=#. \t\r\n") {
		t.Fatalf("sanitizeOrgID produced unsafe characters: %q", got)
	}
}

func TestSanitizeOrgID_AllUnsafeInputReturnsEmpty(t *testing.T) {
	if got := sanitizeOrgID("../??//..  "); got != "" {
		t.Fatalf("sanitizeOrgID returned %q, want empty string", got)
	}
}

func TestNewSQLiteResourceStore_DefaultOrgUsesSharedResourcesPath(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	wantPath := filepath.Join(dataDir, "resources", "unified_resources.db")
	if store.dbPath != wantPath {
		t.Fatalf("db path = %q, want %q", store.dbPath, wantPath)
	}
}

func TestNewSQLiteResourceStore_NonDefaultOrgUsesTenantScopedPath(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "org-a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	wantPath := filepath.Join(dataDir, "orgs", "org-a", "resources", "unified_resources.db")
	if store.dbPath != wantPath {
		t.Fatalf("db path = %q, want %q", store.dbPath, wantPath)
	}
}

func TestNewSQLiteResourceStore_OrgDotAndUnderscoreDoNotCollide(t *testing.T) {
	dataDir := t.TempDir()
	dotStore, err := NewSQLiteResourceStore(dataDir, "org.a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore(org.a) returned error: %v", err)
	}
	defer dotStore.Close()

	underscoreStore, err := NewSQLiteResourceStore(dataDir, "org_a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore(org_a) returned error: %v", err)
	}
	defer underscoreStore.Close()

	if dotStore.dbPath == underscoreStore.dbPath {
		t.Fatalf("db path collision: org.a and org_a both mapped to %q", dotStore.dbPath)
	}
}

func TestNewSQLiteResourceStore_RejectsInvalidOrgID(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := NewSQLiteResourceStore(dataDir, "../bad-org"); err == nil {
		t.Fatal("expected invalid org ID error, got nil")
	}
}

func TestNewSQLiteResourceStore_RecoversFromCorruptedDB(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("initial store: %v", err)
	}
	store.Close()

	dbPath := filepath.Join(dataDir, "resources", "unified_resources.db")
	if err := os.WriteFile(dbPath, []byte("this is not a valid sqlite database"), 0o600); err != nil {
		t.Fatalf("corrupt database: %v", err)
	}

	recovered, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("expected recovery from corrupted db, got error: %v", err)
	}
	defer recovered.Close()

	matches, err := filepath.Glob(dbPath + ".corrupted.*")
	if err != nil || len(matches) == 0 {
		t.Fatal("expected corrupted database to be backed up")
	}
}

func TestNewSQLiteResourceStore_MigratesLegacyStore(t *testing.T) {
	dataDir := t.TempDir()
	orgID := "org.a"
	legacyPath := filepath.Join(dataDir, "resources", legacyResourceStoreFileName(orgID))
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(legacyPath), err)
	}
	seedLegacyLinksTable(t, legacyPath)

	store, err := NewSQLiteResourceStore(dataDir, orgID)
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	links, err := store.GetLinks()
	if err != nil {
		t.Fatalf("GetLinks returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("GetLinks length = %d, want 1", len(links))
	}
	if links[0].ResourceA != "legacy-a" || links[0].ResourceB != "legacy-b" {
		t.Fatalf("unexpected migrated link: %+v", links[0])
	}

	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy db should remain for compatibility, stat(%q) failed: %v", legacyPath, err)
	}
	if store.dbPath == legacyPath {
		t.Fatalf("expected migrated store path to differ from legacy path: %q", store.dbPath)
	}
}

func TestNewSQLiteResourceStore_MigratesLegacyResourceChangesTable(t *testing.T) {
	dataDir := t.TempDir()
	legacyPath := filepath.Join(dataDir, "resources", resourceDBFileName)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(legacyPath), err)
	}

	db, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("sql.Open(%q) failed: %v", legacyPath, err)
	}
	if _, err := db.Exec(`
		CREATE TABLE resource_changes (
			id TEXT PRIMARY KEY,
			canonical_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			kind TEXT NOT NULL,
			from_state TEXT,
			to_state TEXT,
			source TEXT,
			confidence TEXT NOT NULL,
			reason TEXT
		)
	`); err != nil {
		_ = db.Close()
		t.Fatalf("create legacy resource_changes table failed: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO resource_changes (
			id, canonical_id, timestamp, kind, from_state, to_state, source, confidence, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "chg-legacy", "vm:legacy", time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC), string(ChangeStateTransition), "offline", "online", "proxmox", string(ConfidenceHigh), "legacy row"); err != nil {
		_ = db.Close()
		t.Fatalf("insert legacy resource change failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db failed: %v", err)
	}

	store, err := NewSQLiteResourceStore(dataDir, defaultOrgID)
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	var rawObservedAt sql.NullString
	if err := store.db.QueryRow(`SELECT observed_at FROM resource_changes WHERE id = ?`, "chg-legacy").Scan(&rawObservedAt); err != nil {
		t.Fatalf("query raw migrated observed_at: %v", err)
	}
	if rawObservedAt.Valid && strings.TrimSpace(rawObservedAt.String) != "" {
		t.Fatalf("legacy observed_at was physically backfilled during startup: %q", rawObservedAt.String)
	}

	results, err := store.GetRecentChanges("vm:legacy", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges on migrated legacy table returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("GetRecentChanges on migrated legacy table returned %d rows, want 1", len(results))
	}
	if results[0].ID != "chg-legacy" {
		t.Fatalf("unexpected legacy row after migration: %+v", results[0])
	}
	if results[0].SourceType != SourcePulseDiff {
		t.Fatalf("legacy source type = %q, want %q", results[0].SourceType, SourcePulseDiff)
	}
	if results[0].SourceAdapter != ChangeSourceAdapter("proxmox") {
		t.Fatalf("legacy source adapter = %q, want proxmox", results[0].SourceAdapter)
	}
	if !results[0].ObservedAt.Equal(time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("legacy observed_at = %v, want 2026-03-18T12:00:00Z", results[0].ObservedAt)
	}
	if results[0].OccurredAt != nil {
		t.Fatalf("legacy occurred_at = %v, want nil", results[0].OccurredAt)
	}

	count, err := store.CountRecentChanges("vm:legacy", time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CountRecentChanges on migrated legacy table returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountRecentChanges on migrated legacy table returned %d, want 1", count)
	}

	if err := store.RecordChange(ResourceChange{
		ID:            "chg-new",
		ResourceID:    "vm:legacy",
		ObservedAt:    time.Date(2026, 3, 18, 13, 0, 0, 0, time.UTC),
		Kind:          ChangeRestart,
		SourceType:    SourcePlatformEvent,
		SourceAdapter: AdapterProxmox,
		Confidence:    ConfidenceHigh,
		Reason:        "post-migration write",
	}); err != nil {
		t.Fatalf("RecordChange after migration failed: %v", err)
	}
	results, err = store.GetRecentChanges("vm:legacy", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges after migration write returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("GetRecentChanges after migration write returned %d rows, want 2", len(results))
	}

	columns, err := resourceChangeColumns(store.db)
	if err != nil {
		t.Fatalf("resourceChangeColumns: %v", err)
	}
	for _, want := range []string{"observed_at", "occurred_at", "source_type", "source_adapter", "actor", "related_resources", "metadata_json"} {
		if _, ok := columns[want]; !ok {
			t.Fatalf("expected migrated resource_changes column %q, got %#v", want, columns)
		}
	}

	indexes, err := resourceChangesIndexes(store.db)
	if err != nil {
		t.Fatalf("resourceChangesIndexes: %v", err)
	}
	for _, want := range []string{
		"idx_resource_changes_canonical_time",
		"idx_resource_changes_kind_time",
		"idx_resource_changes_source_type_time",
		"idx_resource_changes_source_adapter_time",
	} {
		if _, ok := indexes[want]; !ok {
			t.Fatalf("expected migrated resource_changes index %q, got %#v", want, indexes)
		}
	}
}

func TestNormalizeResourceChangeRowsSkipsAlreadyCanonicalRows(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewSQLiteResourceStore(dataDir, defaultOrgID)
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	if err := store.RecordChange(ResourceChange{
		ID:            "chg-canonical",
		ResourceID:    "vm:canonical",
		ObservedAt:    time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		Kind:          ChangeRestart,
		SourceType:    SourcePlatformEvent,
		SourceAdapter: AdapterProxmox,
		Confidence:    ConfidenceHigh,
		Reason:        "canonical row",
	}); err != nil {
		t.Fatalf("RecordChange failed: %v", err)
	}

	columns, err := resourceChangeColumns(store.db)
	if err != nil {
		t.Fatalf("resourceChangeColumns: %v", err)
	}
	var before int
	if err := store.db.QueryRow(`SELECT total_changes()`).Scan(&before); err != nil {
		t.Fatalf("SELECT total_changes() before normalize: %v", err)
	}
	if err := store.normalizeResourceChangeRows(columns); err != nil {
		t.Fatalf("normalizeResourceChangeRows returned error: %v", err)
	}
	var after int
	if err := store.db.QueryRow(`SELECT total_changes()`).Scan(&after); err != nil {
		t.Fatalf("SELECT total_changes() after normalize: %v", err)
	}
	if after != before {
		t.Fatalf("normalizeResourceChangeRows changed %d row(s), want 0", after-before)
	}
}

func TestNewSQLiteResourceStore_InitializesCanonicalResourceChangesSchema(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewSQLiteResourceStore(dataDir, defaultOrgID)
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	columns, err := resourceChangeColumns(store.db)
	if err != nil {
		t.Fatalf("resourceChangeColumns: %v", err)
	}
	if _, ok := columns["timestamp"]; ok {
		t.Fatalf("fresh resource_changes schema unexpectedly contains legacy timestamp column: %#v", columns)
	}
	for _, want := range []string{"observed_at", "occurred_at", "source_type", "source_adapter", "actor", "related_resources", "metadata_json"} {
		if _, ok := columns[want]; !ok {
			t.Fatalf("expected canonical resource_changes column %q, got %#v", want, columns)
		}
	}

	change := ResourceChange{
		ID:            "chg-fresh",
		ResourceID:    "vm:fresh",
		ObservedAt:    time.Date(2026, 3, 18, 14, 0, 0, 0, time.UTC),
		Kind:          ChangeRestart,
		SourceType:    SourcePlatformEvent,
		SourceAdapter: AdapterProxmox,
		Confidence:    ConfidenceHigh,
		Reason:        "fresh schema write",
	}
	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange on fresh schema failed: %v", err)
	}
	results, err := store.GetRecentChanges("vm:fresh", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges on fresh schema returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("GetRecentChanges on fresh schema returned %d rows, want 1", len(results))
	}
	if results[0].ID != change.ID {
		t.Fatalf("unexpected fresh row after write: %+v", results[0])
	}
	if !results[0].ObservedAt.Equal(change.ObservedAt) {
		t.Fatalf("fresh observed_at = %v, want %v", results[0].ObservedAt, change.ObservedAt)
	}
}

func TestNewSQLiteResourceStore_InitializesCanonicalAuditSchemas(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewSQLiteResourceStore(dataDir, defaultOrgID)
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	auditTables := []struct {
		name    string
		columns []string
		indexes []string
	}{
		{
			name: "action_audits",
			columns: []string{
				"id", "action_id", "canonical_id", "request_id", "created_at", "updated_at",
				"state", "request_json", "plan_json", "approvals_json", "result_json",
			},
			indexes: []string{
				"idx_action_audits_canonical_created",
				"idx_action_audits_action_id",
				"idx_action_audits_origin_investigation_updated_v2",
				"idx_action_audits_state_updated",
			},
		},
		{
			name:    "action_lifecycle_events",
			columns: []string{"id", "action_id", "timestamp", "state", "actor", "message"},
			indexes: []string{"idx_action_lifecycle_events_action"},
		},
		{
			name: "action_dispatch_attempts",
			columns: []string{
				"attempt_id", "action_id", "state", "created_at", "updated_at",
				"lease_owner", "lease_expires_at", "dispatch_count",
			},
			indexes: []string{"idx_action_dispatch_attempts_state_updated"},
		},
		{
			name:    "action_dispatch_outbox",
			columns: []string{"attempt_id", "action_id", "available_at"},
		},
		{
			name:    "action_dispatch_receipts",
			columns: []string{"attempt_id", "action_id", "transport_request_id", "received_at"},
		},
		{
			name:    "export_audits",
			columns: []string{"id", "timestamp", "actor", "envelope_hash", "decision", "destination", "redactions_json"},
			indexes: []string{"idx_export_audits_timestamp"},
		},
	}

	for _, tt := range auditTables {
		columns, err := tableColumns(store.db, tt.name)
		if err != nil {
			t.Fatalf("tableColumns(%q): %v", tt.name, err)
		}
		for _, want := range tt.columns {
			if _, ok := columns[want]; !ok {
				t.Fatalf("expected %s column %q, got %#v", tt.name, want, columns)
			}
		}

		indexes, err := tableIndexes(store.db, tt.name)
		if err != nil {
			t.Fatalf("tableIndexes(%q): %v", tt.name, err)
		}
		for _, want := range tt.indexes {
			if _, ok := indexes[want]; !ok {
				t.Fatalf("expected %s index %q, got %#v", tt.name, want, indexes)
			}
		}
	}
}

func resourceChangeColumns(db *sql.DB) (map[string]struct{}, error) {
	return tableColumns(db, "resource_changes")
}

func tableColumns(db *sql.DB, tableName string) (map[string]struct{}, error) {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]struct{})
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

func resourceChangesIndexes(db *sql.DB) (map[string]struct{}, error) {
	return tableIndexes(db, "resource_changes")
}

func tableIndexes(db *sql.DB, tableName string) (map[string]struct{}, error) {
	rows, err := db.Query(`PRAGMA index_list(` + tableName + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]struct{})
	for rows.Next() {
		var (
			seq    int
			name   string
			uniq   int
			origin string
			part   int
		)
		if err := rows.Scan(&seq, &name, &uniq, &origin, &part); err != nil {
			return nil, err
		}
		indexes[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return indexes, nil
}

func newTestStore(t *testing.T) *SQLiteResourceStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "testorg")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func insertRawActionAuditForTest(t *testing.T, store *SQLiteResourceStore, record ActionAuditRecord) {
	t.Helper()
	requestJSON, err := json.Marshal(record.Request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	planJSON, err := json.Marshal(record.Plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	approvalsJSON, err := json.Marshal(record.Approvals)
	if err != nil {
		t.Fatalf("marshal approvals: %v", err)
	}
	resultJSON, err := json.Marshal(record.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	verificationOutcomeJSON, err := json.Marshal(record.VerificationOutcome)
	if err != nil {
		t.Fatalf("marshal verification outcome: %v", err)
	}
	_, err = store.db.Exec(`
		INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.ID, CanonicalResourceID(record.Request.ResourceID), record.Request.RequestID, record.CreatedAt, record.UpdatedAt, string(record.State), string(requestJSON), string(planJSON), string(approvalsJSON), string(resultJSON), string(verificationOutcomeJSON))
	if err != nil {
		t.Fatalf("insert raw action audit: %v", err)
	}
}

func rawActionAuditResultJSONForTest(t *testing.T, store *SQLiteResourceStore, id string) string {
	t.Helper()
	var resultJSON string
	if err := store.db.QueryRow(`SELECT result_json FROM action_audits WHERE id = ?`, id).Scan(&resultJSON); err != nil {
		t.Fatalf("query raw action audit result_json: %v", err)
	}
	return resultJSON
}

func assertActionAuditVerificationDetailsRedactedForTest(t *testing.T, record ActionAuditRecord, rawDetails ...string) {
	t.Helper()
	verification := CanonicalActionVerification(record)
	if verification == nil {
		t.Fatalf("expected canonical verification to remain present: %+v", record)
	}
	if verification.Command != auditVerificationCommandRedacted {
		t.Fatalf("verification command was not redacted: %+v", verification)
	}
	if verification.Output != auditVerificationOutputRedacted {
		t.Fatalf("verification output was not redacted: %+v", verification)
	}
	if verification.Note != auditVerificationNoteRedacted {
		t.Fatalf("verification note was not redacted: %+v", verification)
	}
	if record.Result == nil || record.Result.Verification == nil {
		t.Fatalf("expected result verification to remain aligned: %+v", record.Result)
	}
	if record.Result.Verification.Command != verification.Command || record.Result.Verification.Output != verification.Output || record.Result.Verification.Note != verification.Note {
		t.Fatalf("result verification was not aligned with canonical verification: result=%+v canonical=%+v", record.Result.Verification, verification)
	}

	wire, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal action audit record for leak check: %v", err)
	}
	for _, rawDetail := range rawDetails {
		if strings.Contains(string(wire), rawDetail) {
			t.Fatalf("raw verification detail %q leaked through action audit read: %s", rawDetail, string(wire))
		}
	}
}

func TestRecordChange_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	occurredAt := now.Add(-30 * time.Second)

	change := ResourceChange{
		ID:               "chg-1",
		ResourceID:       "vm:100",
		ObservedAt:       now,
		OccurredAt:       &occurredAt,
		Kind:             ChangeStateTransition,
		From:             "offline",
		To:               "online",
		SourceType:       SourcePlatformEvent,
		SourceAdapter:    AdapterProxmox,
		Confidence:       ConfidenceHigh,
		Actor:            "agent:ops-helper",
		RelatedResources: []string{"node:1", "storage:2"},
		Reason:           "vm started",
		Metadata: map[string]any{
			"source": "snapshot",
			"retry":  1,
		},
	}

	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	results, err := store.GetRecentChanges("vm:100", now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change, got %d", len(results))
	}
	got := results[0]
	if got.ID != change.ID {
		t.Errorf("ID: got %q, want %q", got.ID, change.ID)
	}
	if got.Kind != change.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, change.Kind)
	}
	if got.From != change.From || got.To != change.To {
		t.Errorf("From/To: got %q/%q, want %q/%q", got.From, got.To, change.From, change.To)
	}
	if got.Confidence != change.Confidence {
		t.Errorf("Confidence: got %q, want %q", got.Confidence, change.Confidence)
	}
	if got.SourceType != change.SourceType {
		t.Errorf("SourceType: got %q, want %q", got.SourceType, change.SourceType)
	}
	if got.SourceAdapter != change.SourceAdapter {
		t.Errorf("SourceAdapter: got %q, want %q", got.SourceAdapter, change.SourceAdapter)
	}
	if got.Actor != change.Actor {
		t.Errorf("Actor: got %q, want %q", got.Actor, change.Actor)
	}
	if got.Reason != change.Reason {
		t.Errorf("Reason: got %q, want %q", got.Reason, change.Reason)
	}
	if len(got.RelatedResources) != len(change.RelatedResources) {
		t.Fatalf("RelatedResources length: got %d, want %d", len(got.RelatedResources), len(change.RelatedResources))
	}
	for i := range change.RelatedResources {
		if got.RelatedResources[i] != change.RelatedResources[i] {
			t.Fatalf("RelatedResources[%d]: got %q, want %q", i, got.RelatedResources[i], change.RelatedResources[i])
		}
	}
	if got.OccurredAt == nil || !got.OccurredAt.Equal(occurredAt) {
		t.Fatalf("OccurredAt: got %v, want %v", got.OccurredAt, occurredAt)
	}
	if got.Metadata["source"] != change.Metadata["source"] {
		t.Fatalf("Metadata source: got %v, want %v", got.Metadata["source"], change.Metadata["source"])
	}
	if fmt.Sprint(got.Metadata["retry"]) != "1" {
		t.Fatalf("Metadata retry: got %v, want 1", got.Metadata["retry"])
	}
}

func TestResourceChangeFiltersIncludeRelatedResources(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 4, 25, 21, 10, 0, 0, time.UTC)
	changes := []ResourceChange{
		{
			ID:               "chg-related-node",
			ResourceID:       "vm:100",
			ObservedAt:       now,
			Kind:             ChangeRestart,
			SourceType:       SourcePlatformEvent,
			SourceAdapter:    AdapterProxmox,
			Confidence:       ConfidenceHigh,
			RelatedResources: []string{" node:1 "},
			Reason:           "vm restarted on node",
		},
		{
			ID:            "chg-direct-node",
			ResourceID:    "node:1",
			ObservedAt:    now.Add(-time.Minute),
			Kind:          ChangeStateTransition,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceMedium,
			Reason:        "node status refreshed",
		},
		{
			ID:               "chg-related-storage",
			ResourceID:       "storage:1",
			ObservedAt:       now.Add(-2 * time.Minute),
			Kind:             ChangeAnomaly,
			SourceType:       SourcePulseDiff,
			SourceAdapter:    AdapterTrueNAS,
			Confidence:       ConfidenceMedium,
			RelatedResources: []string{" node:1 "},
			Reason:           "storage issue affects node",
		},
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	directOnly, err := store.GetRecentChangesFiltered("node:1", now.Add(-time.Hour), 10, ResourceChangeFilters{})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered direct: %v", err)
	}
	if len(directOnly) != 1 || directOnly[0].ID != "chg-direct-node" {
		t.Fatalf("direct timeline = %#v, want only chg-direct-node", directOnly)
	}

	timeline, err := store.GetRecentChangesFiltered("node:1", now.Add(-time.Hour), 10, ResourceChangeFilters{IncludeRelated: true})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered include related: %v", err)
	}
	if got := changeIDs(timeline); !sameStringSet(got, []string{"chg-related-node", "chg-direct-node", "chg-related-storage"}) {
		t.Fatalf("relationship-aware timeline IDs = %#v, want direct plus related changes", got)
	}
	if timeline[0].ID != "chg-related-node" || timeline[1].ID != "chg-direct-node" || timeline[2].ID != "chg-related-storage" {
		t.Fatalf("timeline order = %#v, want observed_at desc across direct and related changes", changeIDs(timeline))
	}

	filtered, err := store.GetRecentChangesFiltered("node:1", now.Add(-time.Hour), 10, ResourceChangeFilters{
		IncludeRelated: true,
		Kinds:          []ChangeKind{ChangeAnomaly},
	})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered related kind: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "chg-related-storage" {
		t.Fatalf("filtered related timeline = %#v, want chg-related-storage", filtered)
	}

	count, err := store.CountRecentChangesFiltered("node:1", now.Add(-time.Hour), ResourceChangeFilters{IncludeRelated: true})
	if err != nil {
		t.Fatalf("CountRecentChangesFiltered include related: %v", err)
	}
	if count != 3 {
		t.Fatalf("relationship-aware count = %d, want 3", count)
	}

	kindCounts, err := store.CountRecentChangesByKindFiltered("node:1", now.Add(-time.Hour), ResourceChangeFilters{IncludeRelated: true})
	if err != nil {
		t.Fatalf("CountRecentChangesByKindFiltered include related: %v", err)
	}
	if got := kindCounts[ChangeRestart]; got != 1 {
		t.Fatalf("restart count = %d, want 1", got)
	}
	if got := kindCounts[ChangeStateTransition]; got != 1 {
		t.Fatalf("state transition count = %d, want 1", got)
	}
	if got := kindCounts[ChangeAnomaly]; got != 1 {
		t.Fatalf("anomaly count = %d, want 1", got)
	}

	sourceCounts, err := store.CountRecentChangesBySourceAdapterFiltered("node:1", now.Add(-time.Hour), ResourceChangeFilters{IncludeRelated: true})
	if err != nil {
		t.Fatalf("CountRecentChangesBySourceAdapterFiltered include related: %v", err)
	}
	if got := sourceCounts[AdapterProxmox]; got != 2 {
		t.Fatalf("proxmox count = %d, want 2", got)
	}
	if got := sourceCounts[AdapterTrueNAS]; got != 1 {
		t.Fatalf("truenas count = %d, want 1", got)
	}
}

func TestMemoryStoreResourceChangeFiltersIncludeRelatedResources(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 4, 25, 21, 15, 0, 0, time.UTC)
	for _, change := range []ResourceChange{
		{
			ID:               "mem-related",
			ResourceID:       "vm:100",
			ObservedAt:       now,
			Kind:             ChangeRestart,
			SourceType:       SourcePlatformEvent,
			SourceAdapter:    AdapterProxmox,
			Confidence:       ConfidenceHigh,
			RelatedResources: []string{"node:1"},
		},
		{
			ID:            "mem-direct",
			ResourceID:    "node:1",
			ObservedAt:    now.Add(-time.Minute),
			Kind:          ChangeStateTransition,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceMedium,
		},
	} {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	directOnly, err := store.GetRecentChangesFiltered("node:1", now.Add(-time.Hour), 10, ResourceChangeFilters{})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered direct: %v", err)
	}
	if len(directOnly) != 1 || directOnly[0].ID != "mem-direct" {
		t.Fatalf("direct memory timeline = %#v, want only mem-direct", directOnly)
	}

	timeline, err := store.GetRecentChangesFiltered("node:1", now.Add(-time.Hour), 10, ResourceChangeFilters{IncludeRelated: true})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered include related: %v", err)
	}
	if len(timeline) != 2 || timeline[0].ID != "mem-direct" || timeline[1].ID != "mem-related" {
		t.Fatalf("relationship-aware memory timeline = %#v, want reverse insertion order direct plus related", timeline)
	}

	count, err := store.CountRecentChangesFiltered("node:1", now.Add(-time.Hour), ResourceChangeFilters{IncludeRelated: true})
	if err != nil {
		t.Fatalf("CountRecentChangesFiltered include related: %v", err)
	}
	if count != 2 {
		t.Fatalf("relationship-aware memory count = %d, want 2", count)
	}
}

func changeIDs(changes []ResourceChange) []string {
	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		ids = append(ids, change.ID)
	}
	return ids
}

func TestRecordChange_IgnoresDuplicateIDs(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 30, 18, 20, 0, 0, time.UTC)

	initial := ResourceChange{
		ID:            "chg-dup-1",
		ResourceID:    "vm:dup",
		ObservedAt:    now,
		Kind:          ChangeActivity,
		SourceType:    SourcePlatformEvent,
		SourceAdapter: AdapterVMware,
		Confidence:    ConfidenceHigh,
		Reason:        "Create snapshot (success)",
	}
	if err := store.RecordChange(initial); err != nil {
		t.Fatalf("RecordChange initial: %v", err)
	}

	duplicate := initial
	duplicate.Reason = "should be ignored"
	duplicate.Metadata = map[string]any{"ignored": true}
	if err := store.RecordChange(duplicate); err != nil {
		t.Fatalf("RecordChange duplicate: %v", err)
	}

	results, err := store.GetRecentChanges("vm:dup", now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change after duplicate insert, got %d", len(results))
	}
	if results[0].Reason != initial.Reason {
		t.Fatalf("Reason = %q, want original %q", results[0].Reason, initial.Reason)
	}
	if len(results[0].Metadata) != 0 {
		t.Fatalf("Metadata = %#v, want original empty metadata", results[0].Metadata)
	}
}

func TestMemoryStore_RecordChangeIgnoresDuplicateIDs(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 30, 18, 25, 0, 0, time.UTC)

	change := ResourceChange{
		ID:            "chg-mem-dup-1",
		ResourceID:    "vm:memdup",
		ObservedAt:    now,
		Kind:          ChangeActivity,
		SourceType:    SourcePlatformEvent,
		SourceAdapter: AdapterVMware,
		Confidence:    ConfidenceHigh,
		Reason:        "Host entered maintenance evaluation",
	}
	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange initial: %v", err)
	}
	change.Reason = "should be ignored"
	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange duplicate: %v", err)
	}

	results, err := store.GetRecentChanges("vm:memdup", now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change after duplicate insert, got %d", len(results))
	}
	if results[0].Reason != "Host entered maintenance evaluation" {
		t.Fatalf("Reason = %q, want original reason", results[0].Reason)
	}
}

func TestRecordChange_PreservesTimelineMetadata(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	occurredAt := now.Add(-5 * time.Minute)

	change := ResourceChange{
		ID:               "chg-rich-1",
		ResourceID:       "vm:200",
		ObservedAt:       now,
		OccurredAt:       &occurredAt,
		Kind:             ChangeRelationship,
		SourceType:       SourcePulseDiff,
		SourceAdapter:    AdapterDocker,
		Confidence:       ConfidenceMedium,
		Actor:            "pulse:differ",
		RelatedResources: []string{"node:20", "service:api"},
		Reason:           "relationship updated",
		Metadata: map[string]any{
			"edgeType": "runs_on",
			"active":   true,
		},
	}

	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	results, err := store.GetRecentChanges("vm:200", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change, got %d", len(results))
	}

	got := results[0]
	if got.Kind != change.Kind || got.SourceType != change.SourceType || got.SourceAdapter != change.SourceAdapter {
		t.Fatalf("unexpected change headers: %+v", got)
	}
	if got.OccurredAt == nil || !got.OccurredAt.Equal(occurredAt) {
		t.Fatalf("OccurredAt: got %v, want %v", got.OccurredAt, occurredAt)
	}
	if len(got.RelatedResources) != 2 || got.RelatedResources[0] != "node:20" || got.RelatedResources[1] != "service:api" {
		t.Fatalf("RelatedResources round-trip failed: %+v", got.RelatedResources)
	}
	if got.Metadata["edgeType"] != "runs_on" {
		t.Fatalf("Metadata edgeType: got %v, want %v", got.Metadata["edgeType"], "runs_on")
	}
	if got.Metadata["active"] != true {
		t.Fatalf("Metadata active: got %v, want true", got.Metadata["active"])
	}
}

func TestCountRecentChanges_RespectsFilters(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	changes := []ResourceChange{
		{
			ID:            "chg-count-1",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-30 * time.Minute),
			Kind:          ChangeStateTransition,
			SourceType:    SourcePlatformEvent,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceHigh,
		},
		{
			ID:            "chg-count-2",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-20 * time.Minute),
			Kind:          ChangeAnomaly,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceMedium,
		},
		{
			ID:            "chg-count-3",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-10 * time.Minute),
			Kind:          ChangeRelationship,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceLow,
		},
		{
			ID:            "chg-count-4",
			ResourceID:    "vm:2",
			ObservedAt:    base.Add(-5 * time.Minute),
			Kind:          ChangeCapability,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceLow,
		},
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	count, err := store.CountRecentChanges("vm:1", base.Add(-25*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChanges vm:1: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountRecentChanges vm:1 = %d, want 2", count)
	}

	allCount, err := store.CountRecentChanges("", base.Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChanges all: %v", err)
	}
	if allCount != 2 {
		t.Fatalf("CountRecentChanges all = %d, want 2", allCount)
	}

	filteredCount, err := store.CountRecentChangesFiltered("vm:1", base.Add(-35*time.Minute), ResourceChangeFilters{
		Kinds: []ChangeKind{ChangeAnomaly, ChangeRelationship},
	})
	if err != nil {
		t.Fatalf("CountRecentChangesFiltered kinds: %v", err)
	}
	if filteredCount != 2 {
		t.Fatalf("CountRecentChangesFiltered kinds = %d, want 2", filteredCount)
	}

	sourceFilteredCount, err := store.CountRecentChangesFiltered("", base.Add(-25*time.Minute), ResourceChangeFilters{
		SourceTypes: []ChangeSourceType{SourcePulseDiff},
	})
	if err != nil {
		t.Fatalf("CountRecentChangesFiltered source types: %v", err)
	}
	if sourceFilteredCount != 3 {
		t.Fatalf("CountRecentChangesFiltered source types = %d, want 3", sourceFilteredCount)
	}

	adapterFilteredCount, err := store.CountRecentChangesFiltered("vm:1", base.Add(-35*time.Minute), ResourceChangeFilters{
		SourceAdapters: []ChangeSourceAdapter{AdapterProxmox},
	})
	if err != nil {
		t.Fatalf("CountRecentChangesFiltered source adapters: %v", err)
	}
	if adapterFilteredCount != 2 {
		t.Fatalf("CountRecentChangesFiltered source adapters = %d, want 2", adapterFilteredCount)
	}
}

func TestCountRecentChangesByKind_RespectsFilters(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	changes := []ResourceChange{
		{
			ID:            "chg-kind-1",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-30 * time.Minute),
			Kind:          ChangeStateTransition,
			SourceType:    SourcePlatformEvent,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceHigh,
		},
		{
			ID:            "chg-kind-2",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-20 * time.Minute),
			Kind:          ChangeAnomaly,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceMedium,
		},
		{
			ID:            "chg-kind-3",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-10 * time.Minute),
			Kind:          ChangeAnomaly,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceLow,
		},
		{
			ID:            "chg-kind-4",
			ResourceID:    "vm:2",
			ObservedAt:    base.Add(-5 * time.Minute),
			Kind:          ChangeCapability,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceLow,
		},
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	counts, err := store.CountRecentChangesByKind("vm:1", base.Add(-35*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChangesByKind vm:1: %v", err)
	}
	wantCounts := map[ChangeKind]int{
		ChangeStateTransition: 1,
		ChangeAnomaly:         2,
	}
	if !reflect.DeepEqual(counts, wantCounts) {
		t.Fatalf("CountRecentChangesByKind vm:1 = %#v, want %#v", counts, wantCounts)
	}

	filteredCounts, err := store.CountRecentChangesByKindFiltered("vm:1", base.Add(-35*time.Minute), ResourceChangeFilters{
		SourceTypes: []ChangeSourceType{SourcePulseDiff},
	})
	if err != nil {
		t.Fatalf("CountRecentChangesByKindFiltered source types: %v", err)
	}
	if !reflect.DeepEqual(filteredCounts, map[ChangeKind]int{ChangeAnomaly: 2}) {
		t.Fatalf("CountRecentChangesByKindFiltered source types = %#v, want %#v", filteredCounts, map[ChangeKind]int{ChangeAnomaly: 2})
	}

	adapterCounts, err := store.CountRecentChangesByKindFiltered("vm:1", base.Add(-35*time.Minute), ResourceChangeFilters{
		SourceAdapters: []ChangeSourceAdapter{AdapterProxmox},
	})
	if err != nil {
		t.Fatalf("CountRecentChangesByKindFiltered source adapters: %v", err)
	}
	if !reflect.DeepEqual(adapterCounts, map[ChangeKind]int{ChangeStateTransition: 1, ChangeAnomaly: 1}) {
		t.Fatalf("CountRecentChangesByKindFiltered source adapters = %#v, want %#v", adapterCounts, map[ChangeKind]int{ChangeStateTransition: 1, ChangeAnomaly: 1})
	}
}

func TestStorePersistsCanonicalIncidentTimelineKinds(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	changes := []ResourceChange{
		{
			ID:         "incident-kind-1",
			ResourceID: "vm:incident",
			ObservedAt: base.Add(-3 * time.Minute),
			Kind:       ChangeAlertFired,
			SourceType: SourceHeuristic,
			Confidence: ConfidenceHigh,
		},
		{
			ID:         "incident-kind-2",
			ResourceID: "vm:incident",
			ObservedAt: base.Add(-2 * time.Minute),
			Kind:       ChangeCommandExecuted,
			SourceType: SourceAgentAction,
			Confidence: ConfidenceHigh,
		},
		{
			ID:         "incident-kind-3",
			ResourceID: "vm:incident",
			ObservedAt: base.Add(-time.Minute),
			Kind:       ChangeRunbookExecuted,
			SourceType: SourceAgentAction,
			Confidence: ConfidenceHigh,
		},
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	results, err := store.GetRecentChangesFiltered("vm:incident", base.Add(-10*time.Minute), 10, ResourceChangeFilters{
		Kinds: []ChangeKind{ChangeAlertFired, ChangeCommandExecuted, ChangeRunbookExecuted},
	})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 incident timeline changes, got %d", len(results))
	}

	counts, err := store.CountRecentChangesByKind("vm:incident", base.Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChangesByKind: %v", err)
	}
	for _, kind := range []ChangeKind{ChangeAlertFired, ChangeCommandExecuted, ChangeRunbookExecuted} {
		if counts[kind] != 1 {
			t.Fatalf("CountRecentChangesByKind[%q] = %d, want 1", kind, counts[kind])
		}
	}

	if got := results[0].Kind; got != ChangeRunbookExecuted {
		t.Fatalf("latest kind = %q, want %q", got, ChangeRunbookExecuted)
	}
}

func TestCountRecentChangesBySourceType_RespectsFilters(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	changes := []ResourceChange{
		{
			ID:            "chg-source-1",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-30 * time.Minute),
			Kind:          ChangeStateTransition,
			SourceType:    SourcePlatformEvent,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceHigh,
		},
		{
			ID:            "chg-source-2",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-20 * time.Minute),
			Kind:          ChangeAnomaly,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceMedium,
		},
		{
			ID:            "chg-source-3",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-10 * time.Minute),
			Kind:          ChangeAnomaly,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceLow,
		},
		{
			ID:            "chg-source-4",
			ResourceID:    "vm:2",
			ObservedAt:    base.Add(-5 * time.Minute),
			Kind:          ChangeCapability,
			SourceType:    SourceAgentAction,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceLow,
		},
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	counts, err := store.CountRecentChangesBySourceType("vm:1", base.Add(-35*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChangesBySourceType vm:1: %v", err)
	}
	wantCounts := map[ChangeSourceType]int{
		SourcePlatformEvent: 1,
		SourcePulseDiff:     2,
	}
	if !reflect.DeepEqual(counts, wantCounts) {
		t.Fatalf("CountRecentChangesBySourceType vm:1 = %#v, want %#v", counts, wantCounts)
	}

	filteredCounts, err := store.CountRecentChangesBySourceTypeFiltered("vm:1", base.Add(-35*time.Minute), ResourceChangeFilters{
		SourceAdapters: []ChangeSourceAdapter{AdapterProxmox},
	})
	if err != nil {
		t.Fatalf("CountRecentChangesBySourceTypeFiltered source adapters: %v", err)
	}
	if !reflect.DeepEqual(filteredCounts, map[ChangeSourceType]int{SourcePlatformEvent: 1, SourcePulseDiff: 1}) {
		t.Fatalf("CountRecentChangesBySourceTypeFiltered source adapters = %#v, want %#v", filteredCounts, map[ChangeSourceType]int{SourcePlatformEvent: 1, SourcePulseDiff: 1})
	}

	adapterCounts, err := store.CountRecentChangesBySourceAdapter("vm:1", base.Add(-35*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChangesBySourceAdapter vm:1: %v", err)
	}
	wantAdapterCounts := map[ChangeSourceAdapter]int{
		AdapterDocker:  1,
		AdapterProxmox: 2,
	}
	if !reflect.DeepEqual(adapterCounts, wantAdapterCounts) {
		t.Fatalf("CountRecentChangesBySourceAdapter vm:1 = %#v, want %#v", adapterCounts, wantAdapterCounts)
	}

	filteredAdapterCounts, err := store.CountRecentChangesBySourceAdapterFiltered("vm:1", base.Add(-35*time.Minute), ResourceChangeFilters{
		SourceTypes: []ChangeSourceType{SourcePulseDiff},
	})
	if err != nil {
		t.Fatalf("CountRecentChangesBySourceAdapterFiltered source types: %v", err)
	}
	if !reflect.DeepEqual(filteredAdapterCounts, map[ChangeSourceAdapter]int{AdapterDocker: 1, AdapterProxmox: 1}) {
		t.Fatalf("CountRecentChangesBySourceAdapterFiltered source types = %#v, want %#v", filteredAdapterCounts, map[ChangeSourceAdapter]int{AdapterDocker: 1, AdapterProxmox: 1})
	}
}

func TestGetRecentChanges_RespectsFilters(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	changes := []ResourceChange{
		{
			ID:            "chg-1",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-30 * time.Minute),
			Kind:          ChangeStateTransition,
			SourceType:    SourcePlatformEvent,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceHigh,
		},
		{
			ID:            "chg-2",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-20 * time.Minute),
			Kind:          ChangeAnomaly,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceMedium,
		},
		{
			ID:            "chg-3",
			ResourceID:    "vm:1",
			ObservedAt:    base.Add(-10 * time.Minute),
			Kind:          ChangeRelationship,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceLow,
		},
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	results, err := store.GetRecentChangesFiltered("vm:1", base.Add(-35*time.Minute), 10, ResourceChangeFilters{
		Kinds: []ChangeKind{ChangeRelationship},
	})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered kinds: %v", err)
	}
	if len(results) != 1 || results[0].ID != "chg-3" {
		t.Fatalf("GetRecentChangesFiltered kinds = %#v, want chg-3", results)
	}

	sourceResults, err := store.GetRecentChangesFiltered("vm:1", base.Add(-25*time.Minute), 10, ResourceChangeFilters{
		SourceTypes: []ChangeSourceType{SourcePulseDiff},
	})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered source types: %v", err)
	}
	if len(sourceResults) != 2 || sourceResults[0].ID != "chg-3" || sourceResults[1].ID != "chg-2" {
		t.Fatalf("GetRecentChangesFiltered source types = %#v, want chg-3 then chg-2", sourceResults)
	}

	adapterResults, err := store.GetRecentChangesFiltered("vm:1", base.Add(-35*time.Minute), 10, ResourceChangeFilters{
		SourceAdapters: []ChangeSourceAdapter{AdapterProxmox},
	})
	if err != nil {
		t.Fatalf("GetRecentChangesFiltered source adapters: %v", err)
	}
	if len(adapterResults) != 2 || adapterResults[0].ID != "chg-3" || adapterResults[1].ID != "chg-1" {
		t.Fatalf("GetRecentChangesFiltered source adapters = %#v, want chg-3 then chg-1", adapterResults)
	}
}

func TestActionAuditRecord_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 13, 0, 0, 0, time.UTC)
	expires := now.Add(15 * time.Minute)
	approvedAt := now.Add(2 * time.Minute)
	result := &ExecutionResult{
		Success: true,
		Output:  "completed token=result-output-secret",
		Verification: &ActionVerificationResult{
			Ran:     true,
			Command: "systemctl is-active 'nginx' token=verify-command-secret",
			Output:  "active token=verify-output-secret",
			Success: true,
			RanAt:   now.Add(4 * time.Minute),
			Note:    "verified via password=verify-note-secret",
		},
	}

	record := ActionAuditRecord{
		ID:        "action-1",
		CreatedAt: now,
		UpdatedAt: now.Add(5 * time.Minute),
		State:     ActionStateCompleted,
		Request: ActionRequest{
			RequestID:      "req-1",
			ResourceID:     "vm:300",
			CapabilityName: "restart",
			Params: map[string]any{
				"force": true,
			},
			Reason:      "restart for maintenance",
			RequestedBy: "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:             "action-1",
			RequestID:            "req-1",
			Allowed:              true,
			RequiresApproval:     true,
			ApprovalPolicy:       ApprovalAdmin,
			PredictedBlastRadius: []string{"node:1", "storage:1"},
			RollbackAvailable:    true,
			Message:              "allowed",
			PlannedAt:            now,
			ExpiresAt:            expires,
			ResourceVersion:      "rv-1",
			PolicyVersion:        "pv-1",
			PlanHash:             "plan-hash-1",
		},
		Approvals: []ActionApprovalRecord{
			{
				Actor:     "admin@example.com",
				Method:    MethodUI,
				Timestamp: approvedAt,
				Outcome:   OutcomeApproved,
				Reason:    "approved for maintenance window",
			},
		},
		Result: result,
	}

	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	results, err := store.GetActionAudits("vm:300", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 action audit, got %d", len(results))
	}

	got := results[0]
	if got.ID != record.ID || got.State != record.State {
		t.Fatalf("unexpected audit headers: %+v", got)
	}
	if got.Request.RequestID != record.Request.RequestID || got.Request.RequestedBy != record.Request.RequestedBy {
		t.Fatalf("request round-trip failed: %+v", got.Request)
	}
	if got.Plan.ResourceVersion != record.Plan.ResourceVersion || got.Plan.PolicyVersion != record.Plan.PolicyVersion || got.Plan.PlanHash != record.Plan.PlanHash {
		t.Fatalf("plan round-trip failed: %+v", got.Plan)
	}
	if len(got.Approvals) != 1 || got.Approvals[0].Actor != record.Approvals[0].Actor || got.Approvals[0].Outcome != record.Approvals[0].Outcome {
		t.Fatalf("approvals round-trip failed: %+v", got.Approvals)
	}
	if got.Result == nil || !got.Result.Success {
		t.Fatalf("result round-trip failed: %+v", got.Result)
	}
	if strings.Contains(got.Result.Output, "result-output-secret") || !strings.Contains(got.Result.Output, "completed") {
		t.Fatalf("result output redaction failed: %+v", got.Result)
	}
	verification := CanonicalActionVerification(got)
	if verification == nil || !verification.Ran {
		t.Fatalf("canonical verification round-trip failed: %+v", verification)
	}
	if verification.Command != auditVerificationCommandRedacted {
		t.Fatalf("canonical verification command redaction failed: %+v", verification)
	}
	if verification.Output != auditVerificationOutputRedacted {
		t.Fatalf("canonical verification output redaction failed: %+v", verification)
	}
	if verification.Note != auditVerificationNoteRedacted {
		t.Fatalf("canonical verification note redaction failed: %+v", verification)
	}
	if got.Verification == nil || got.Verification.Command != verification.Command {
		t.Fatalf("top-level verification was not restored from sqlite row: top-level=%+v canonical=%+v", got.Verification, verification)
	}
	if got.Result.Verification == nil || got.Result.Verification.Command != verification.Command {
		t.Fatalf("result verification did not stay aligned with canonical verification: result=%+v canonical=%+v", got.Result.Verification, verification)
	}
	if got.Result.Verification.Note != verification.Note {
		t.Fatalf("result verification note did not stay aligned with canonical verification: result=%+v canonical=%+v", got.Result.Verification, verification)
	}
}

func TestActionAuditRecord_RoundTripLegacyResultVerificationRedactsSQLiteReads(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 14, 0, 0, 0, time.UTC)
	rawCommand := "systemctl is-active nginx --token legacy-command-secret"
	rawOutput := "active legacy-output-secret"
	rawNote := "legacy note password=legacy-note-secret"

	insertRawActionAuditForTest(t, store, ActionAuditRecord{
		ID:        "action-legacy-verification",
		CreatedAt: now,
		UpdatedAt: now.Add(time.Minute),
		State:     ActionStateCompleted,
		Request: ActionRequest{
			RequestID:      "req-legacy-verification",
			ResourceID:     "vm:legacy-verification",
			CapabilityName: "restart",
			RequestedBy:    "agent:test",
			Actor:          ActionActor{SubjectID: "agent:test", Kind: ActionActorService, CredentialID: "service:test", OrgID: "default"},
		},
		Plan: ActionPlan{
			ActionID:  "action-legacy-verification",
			RequestID: "req-legacy-verification",
			Allowed:   true,
		},
		Result: &ExecutionResult{
			Success: true,
			Verification: &ActionVerificationResult{
				Ran:     true,
				Success: true,
				Command: rawCommand,
				Output:  rawOutput,
				RanAt:   now.Add(30 * time.Second),
				Note:    rawNote,
			},
		},
	})

	got, ok, err := store.GetActionAudit("action-legacy-verification")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok {
		t.Fatal("expected raw legacy action audit row")
	}
	if got.Verification == nil || !got.Verification.Ran || !got.Verification.Success || !got.Verification.RanAt.Equal(now.Add(30*time.Second)) {
		t.Fatalf("legacy verification semantics were not preserved: %+v", got.Verification)
	}
	assertActionAuditVerificationDetailsRedactedForTest(t, got, rawCommand, rawOutput, rawNote)

	results, err := store.GetActionAudits("vm:legacy-verification", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 raw legacy action audit row, got %d", len(results))
	}
	assertActionAuditVerificationDetailsRedactedForTest(t, results[0], rawCommand, rawOutput, rawNote)
}

func TestActionAuditRecord_LegacyResultVerificationMigrationRedactsSQLiteAtRest(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	rawCommand := "systemctl is-active nginx --token legacy-command-at-rest"
	rawOutput := "active legacy-output-at-rest"
	rawNote := "legacy note password=legacy-note-at-rest"

	store, err := NewSQLiteResourceStore(dir, "testorg")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	insertRawActionAuditForTest(t, store, ActionAuditRecord{
		ID:        "action-legacy-at-rest",
		CreatedAt: now,
		UpdatedAt: now.Add(time.Minute),
		State:     ActionStateCompleted,
		Request: ActionRequest{
			RequestID:      "req-legacy-at-rest",
			ResourceID:     "vm:legacy-at-rest",
			CapabilityName: "restart",
			RequestedBy:    "agent:test",
		},
		Plan: ActionPlan{
			ActionID:  "action-legacy-at-rest",
			RequestID: "req-legacy-at-rest",
			Allowed:   true,
		},
		Result: &ExecutionResult{
			Success: true,
			Verification: &ActionVerificationResult{
				Ran:     true,
				Success: true,
				Command: rawCommand,
				Output:  rawOutput,
				RanAt:   now.Add(30 * time.Second),
				Note:    rawNote,
			},
		},
	})
	before := rawActionAuditResultJSONForTest(t, store, "action-legacy-at-rest")
	for _, rawDetail := range []string{rawCommand, rawOutput, rawNote} {
		if !strings.Contains(before, rawDetail) {
			t.Fatalf("raw setup did not persist %q before migration: %s", rawDetail, before)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store before migration reopen: %v", err)
	}

	store, err = NewSQLiteResourceStore(dir, "testorg")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore after raw legacy insert: %v", err)
	}
	defer store.Close()

	after := rawActionAuditResultJSONForTest(t, store, "action-legacy-at-rest")
	for _, rawDetail := range []string{rawCommand, rawOutput, rawNote} {
		if strings.Contains(after, rawDetail) {
			t.Fatalf("raw verification detail %q survived SQLite migration: %s", rawDetail, after)
		}
	}
	for _, marker := range []string{auditVerificationCommandRedacted, auditVerificationOutputRedacted, auditVerificationNoteRedacted} {
		if !strings.Contains(after, marker) {
			t.Fatalf("expected migration marker %q in result_json, got %s", marker, after)
		}
	}
}

func TestActionAuditRecord_RoundTripMalformedUnrunVerificationScrubsSQLiteRead(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 14, 30, 0, 0, time.UTC)

	insertRawActionAuditForTest(t, store, ActionAuditRecord{
		ID:        "action-malformed-unrun-verification",
		CreatedAt: now,
		UpdatedAt: now.Add(time.Minute),
		State:     ActionStateCompleted,
		Request: ActionRequest{
			RequestID:      "req-malformed-unrun-verification",
			ResourceID:     "vm:malformed-unrun-verification",
			CapabilityName: "restart",
		},
		Plan: ActionPlan{
			ActionID:  "action-malformed-unrun-verification",
			RequestID: "req-malformed-unrun-verification",
			Allowed:   true,
		},
		Result: &ExecutionResult{
			Success: true,
			Verification: &ActionVerificationResult{
				Ran:     false,
				Success: true,
				Command: "should not leak",
				Output:  "sensitive output",
				Note:    "sensitive note",
				RanAt:   now.Add(30 * time.Second),
			},
		},
	})

	results, err := store.GetActionAudits("vm:malformed-unrun-verification", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 raw malformed action audit row, got %d", len(results))
	}
	got := results[0]
	if got.Verification == nil || got.Verification.Ran {
		t.Fatalf("expected sanitized ran=false verification, got %+v", got.Verification)
	}
	if got.Verification.Command != "" || got.Verification.Output != "" || got.Verification.Note != "" || !got.Verification.RanAt.IsZero() || got.Verification.Success {
		t.Fatalf("top-level ran=false verification leaked details: %+v", got.Verification)
	}
	if got.Result == nil || got.Result.Verification == nil {
		t.Fatalf("expected result verification to remain present and aligned: %+v", got.Result)
	}
	if got.Result.Verification.Command != "" || got.Result.Verification.Output != "" || got.Result.Verification.Note != "" || !got.Result.Verification.RanAt.IsZero() || got.Result.Verification.Success {
		t.Fatalf("result ran=false verification leaked details: %+v", got.Result.Verification)
	}
}

func TestMemoryStore_RecordActionAudit_IsInsertOnly(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 18, 13, 30, 0, 0, time.UTC)

	first := ActionAuditRecord{
		ID:        "action-2",
		CreatedAt: now,
		UpdatedAt: now,
		State:     ActionStatePlanned,
		Request: ActionRequest{
			RequestID:      "req-2",
			ResourceID:     "vm:301",
			CapabilityName: "restart",
			RequestedBy:    "agent:test",
		},
	}
	second := first
	second.UpdatedAt = now.Add(2 * time.Minute)
	second.State = ActionStateCompleted
	second.Result = &ExecutionResult{Success: true, Output: "done"}

	if err := store.RecordActionAudit(first); err != nil {
		t.Fatalf("RecordActionAudit(first): %v", err)
	}
	if err := store.RecordActionAudit(second); !errors.Is(err, ErrActionAuditAlreadyExists) {
		t.Fatalf("RecordActionAudit(second) error = %v, want ErrActionAuditAlreadyExists", err)
	}

	results, err := store.GetActionAudits("vm:301", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 action audit after upsert, got %d", len(results))
	}
	if results[0].State != ActionStatePlanned {
		t.Fatalf("insert-only audit state = %q, want planned", results[0].State)
	}
	if results[0].Result != nil {
		t.Fatalf("duplicate insert rewrote result: %+v", results[0].Result)
	}
}

func TestActionAudit_GetActionAuditByID(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 5, 4, 13, 0, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_lookup",
		CreatedAt: now,
		UpdatedAt: now,
		State:     ActionStatePending,
		Request: ActionRequest{
			RequestID:      "req-lookup",
			ResourceID:     "vm:302",
			CapabilityName: "restart",
			Reason:         "lookup proof",
			RequestedBy:    "agent:test",
		},
		Plan: ActionPlan{
			ActionID:            "act_lookup",
			RequestID:           "req-lookup",
			ExpiresAt:           now.Add(5 * time.Minute),
			ResourceVersion:     "resource:sha256:test",
			PolicyVersion:       "policy:sha256:test",
			PlanHash:            "sha256:test",
			ApprovalPolicy:      ApprovalAdmin,
			ApprovalRequirement: ApprovalRequirementForFloor(ApprovalAdmin),
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	got, ok, err := store.GetActionAudit("act_lookup")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || got.ID != "act_lookup" || got.Request.ResourceID != "vm:302" {
		t.Fatalf("GetActionAudit = %#v, %v", got, ok)
	}
	_, ok, err = store.GetActionAudit("missing")
	if err != nil {
		t.Fatalf("GetActionAudit missing: %v", err)
	}
	if ok {
		t.Fatal("missing action unexpectedly found")
	}
}

func TestRecordActionDecision_UpdatesAuditAndAppendsLifecycle(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 5, 4, 13, 30, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_decision",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		State:     ActionStatePending,
		Request: ActionRequest{
			RequestID:      "req-decision",
			ResourceID:     "vm:303",
			CapabilityName: "restart",
			Reason:         "decision proof",
			RequestedBy:    "agent:test",
			Actor:          ActionActor{SubjectID: "agent:test", Kind: ActionActorService, CredentialID: "service:test", OrgID: "default"},
		},
		Plan: ActionPlan{
			ActionID:            "act_decision",
			RequestID:           "req-decision",
			ExpiresAt:           now.Add(5 * time.Minute),
			ResourceVersion:     "resource:sha256:test",
			PolicyVersion:       "policy:sha256:test",
			PlanHash:            "sha256:test",
			ApprovalPolicy:      ApprovalAdmin,
			ApprovalRequirement: ApprovalRequirementForFloor(ApprovalAdmin),
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}
	approvalFor := func(subject string, outcome ApprovalOutcome, reason string, at time.Time) ActionApprovalRecord {
		actor := ActionActor{SubjectID: subject, Kind: ActionActorUser, CredentialID: "session:" + subject, OrgID: "default"}
		evidence := ApprovalEvidence{Version: 1, Method: MethodSession, Actor: actor, OrgID: "default", ActionID: record.ID, PlanHash: record.Plan.PlanHash, Outcome: outcome, IssuedAt: at}
		return ActionApprovalRecord{Actor: subject, ActorBinding: actor, Method: MethodSession, Timestamp: at, Outcome: outcome, Reason: reason, Evidence: &evidence}
	}
	updated, event, err := ApplyActionDecision(record, approvalFor("operator@example.com", OutcomeApproved, "approved for proof", now), now)
	if err != nil {
		t.Fatalf("ApplyActionDecision: %v", err)
	}
	if err := store.RecordActionDecision(updated, event); err != nil {
		t.Fatalf("RecordActionDecision: %v", err)
	}

	got, ok, err := store.GetActionAudit("act_decision")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || got.State != ActionStateApproved || len(got.Approvals) != 1 {
		t.Fatalf("decision audit = %#v, %v", got, ok)
	}
	events, err := store.GetActionLifecycleEvents("act_decision", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 2 || events[0].State != ActionStateApproved || events[0].Actor != "operator@example.com" ||
		events[0].Kind != ActionLifecycleEventDecision || events[1].Kind != ActionLifecycleEventTransition {
		t.Fatalf("decision events = %#v", events)
	}

	staleUpdate, staleEvent, err := ApplyActionDecision(record, approvalFor("second-operator@example.com", OutcomeRejected, "late rejection", now.Add(time.Second)), now.Add(time.Second))
	if err != nil {
		t.Fatalf("ApplyActionDecision stale: %v", err)
	}
	if err := store.RecordActionDecision(staleUpdate, staleEvent); !errors.Is(err, ErrActionDecisionRevisionConflict) {
		t.Fatalf("stale RecordActionDecision error = %v, want %v", err, ErrActionDecisionRevisionConflict)
	}
}

func TestRecordActionExecutionStartAndResult_UpdatesAuditAndAppendsLifecycle(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 5, 4, 14, 30, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_execution",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-30 * time.Second),
		State:     ActionStateApproved,
		Request: ActionRequest{
			RequestID:      "req-execution",
			ResourceID:     "vm:304",
			CapabilityName: "restart",
			Reason:         "execution proof",
			RequestedBy:    "agent:test",
		},
		Plan: ActionPlan{
			ActionID:         "act_execution",
			RequestID:        "req-execution",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   ApprovalAdmin,
			PlannedAt:        now.Add(-time.Minute),
			ExpiresAt:        now.Add(5 * time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
		Approvals: []ActionApprovalRecord{
			{
				Actor:     "operator@example.com",
				Method:    MethodAPI,
				Timestamp: now.Add(-30 * time.Second),
				Outcome:   OutcomeApproved,
				Reason:    "approved for proof",
			},
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	started, startEvent, err := BeginActionExecution(record, "operator@example.com", now)
	if err != nil {
		t.Fatalf("BeginActionExecution: %v", err)
	}
	if err := store.RecordActionExecutionStart(started, startEvent); err != nil {
		t.Fatalf("RecordActionExecutionStart: %v", err)
	}
	got, ok, err := store.GetActionAudit("act_execution")
	if err != nil {
		t.Fatalf("GetActionAudit after start: %v", err)
	}
	if !ok || got.State != ActionStateExecuting || got.Result != nil {
		t.Fatalf("started audit = %#v, %v", got, ok)
	}
	if err := store.RecordActionExecutionStart(started, startEvent); !errors.Is(err, ErrActionAlreadyExecuting) {
		t.Fatalf("stale RecordActionExecutionStart error = %v, want %v", err, ErrActionAlreadyExecuting)
	}

	completed, doneEvent, err := CompleteActionExecution(started, &ExecutionResult{Success: true, Output: "done"}, "operator@example.com", now.Add(time.Second))
	if err != nil {
		t.Fatalf("CompleteActionExecution: %v", err)
	}
	if err := store.RecordActionExecutionResult(completed, doneEvent); err != nil {
		t.Fatalf("RecordActionExecutionResult: %v", err)
	}
	got, ok, err = store.GetActionAudit("act_execution")
	if err != nil {
		t.Fatalf("GetActionAudit after result: %v", err)
	}
	if !ok || got.State != ActionStateCompleted || got.Result == nil || got.Result.Output != "done" {
		t.Fatalf("completed audit = %#v, %v", got, ok)
	}
	if err := store.RecordActionExecutionResult(completed, doneEvent); !errors.Is(err, ErrActionExecutionFinal) {
		t.Fatalf("stale RecordActionExecutionResult error = %v, want %v", err, ErrActionExecutionFinal)
	}

	events, err := store.GetActionLifecycleEvents("act_execution", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[ActionState]bool{}
	for _, event := range events {
		seen[event.State] = true
	}
	if len(events) != 2 || !seen[ActionStateExecuting] || !seen[ActionStateCompleted] {
		t.Fatalf("execution events = %#v", events)
	}
}

func TestRecordActionExecutionStartRejectsDryRunOnlyPlan(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 5, 4, 15, 0, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_dry_run_only",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		State:     ActionStatePlanned,
		Request: ActionRequest{
			RequestID:      "req-dry-run",
			ResourceID:     "vm:404",
			CapabilityName: "restart",
			Reason:         "dry-run validation",
			RequestedBy:    "agent:test",
		},
		Plan: ActionPlan{
			ActionID:        "act_dry_run_only",
			RequestID:       "req-dry-run",
			Allowed:         true,
			ApprovalPolicy:  ApprovalDryRun,
			PlannedAt:       now.Add(-time.Minute),
			ExpiresAt:       now.Add(5 * time.Minute),
			ResourceVersion: "resource:sha256:test",
			PolicyVersion:   "policy:sha256:test",
			PlanHash:        "sha256:test",
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	forcedExecuting := record
	forcedExecuting.State = ActionStateExecuting
	forcedExecuting.UpdatedAt = now
	err := store.RecordActionExecutionStart(forcedExecuting, ActionLifecycleEvent{
		ActionID:  record.ID,
		Timestamp: now,
		State:     ActionStateExecuting,
		Actor:     "agent:test",
		Message:   "should not execute",
	})
	if !errors.Is(err, ErrActionDryRunOnly) {
		t.Fatalf("RecordActionExecutionStart error = %v, want %v", err, ErrActionDryRunOnly)
	}

	got, ok, err := store.GetActionAudit(record.ID)
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || got.State != ActionStatePlanned || got.Result != nil {
		t.Fatalf("dry-run-only audit changed = %#v, ok=%v", got, ok)
	}
	events, err := store.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("dry-run-only execution should not append events: %#v", events)
	}
}

func TestRecordActionAudit_NormalizesGovernedPlan(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 4, 25, 22, 40, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "action-governed-1",
		CreatedAt: now,
		UpdatedAt: now,
		State:     ActionStateExecuting,
		Request: ActionRequest{
			ResourceID:     " vm:500 ",
			CapabilityName: "pulse_control",
			Reason:         "restart vm",
			RequestedBy:    "pulse_assistant",
		},
	}

	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}
	results, err := store.GetActionAudits("vm:500", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 normalized action audit, got %d", len(results))
	}
	got := results[0]
	if got.Request.RequestID != "action-governed-1" || got.Plan.ActionID != "action-governed-1" || got.Plan.RequestID != "action-governed-1" {
		t.Fatalf("normalized action identity = %#v", got)
	}
	if got.Request.ResourceID != "vm:500" {
		t.Fatalf("normalized resource id = %q, want vm:500", got.Request.ResourceID)
	}
	if got.Plan.ApprovalPolicy != ApprovalNone {
		t.Fatalf("approval policy = %q, want %q", got.Plan.ApprovalPolicy, ApprovalNone)
	}
	if got.Plan.Preflight == nil || got.Plan.Preflight.Target != "vm:500" || got.Plan.Preflight.DryRunAvailable {
		t.Fatalf("normalized preflight = %#v", got.Plan.Preflight)
	}
}

func TestSQLiteStore_GetActionAudits_AllWhenResourceIDBlank(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 13, 45, 0, 0, time.UTC)

	records := []ActionAuditRecord{
		{
			ID:        "action-3",
			CreatedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
			State:     ActionStatePlanned,
			Request: ActionRequest{
				RequestID:      "req-3",
				ResourceID:     "vm:400",
				CapabilityName: "restart",
				RequestedBy:    "agent:test",
			},
		},
		{
			ID:        "action-4",
			CreatedAt: now.Add(-time.Minute),
			UpdatedAt: now.Add(-time.Minute),
			State:     ActionStateCompleted,
			Request: ActionRequest{
				RequestID:      "req-4",
				ResourceID:     "vm:401",
				CapabilityName: "restart",
				RequestedBy:    "agent:test",
			},
		},
	}

	for _, record := range records {
		if err := store.RecordActionAudit(record); err != nil {
			t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
		}
	}

	results, err := store.GetActionAudits("", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 action audits without resource filter, got %d", len(results))
	}
	if results[0].ID != "action-4" || results[1].ID != "action-3" {
		t.Fatalf("unexpected action audit order: %+v", results)
	}
}

func TestSQLiteStore_GetRecentChanges_AllWhenResourceIDBlank(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 13, 50, 0, 0, time.UTC)

	changes := []ResourceChange{
		{
			ID:            "change-1",
			ResourceID:    "vm:500",
			ObservedAt:    now.Add(-2 * time.Minute),
			Kind:          ChangeRestart,
			SourceType:    SourcePlatformEvent,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceHigh,
			Reason:        "restart detected",
		},
		{
			ID:            "change-2",
			ResourceID:    "vm:501",
			ObservedAt:    now.Add(-time.Minute),
			Kind:          ChangeConfigUpdate,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceMedium,
			Reason:        "configuration drift",
		},
	}

	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	results, err := store.GetRecentChanges("", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 recent changes without resource filter, got %d", len(results))
	}
	if results[0].ID != "change-2" || results[1].ID != "change-1" {
		t.Fatalf("unexpected recent change order: %+v", results)
	}
	if results[0].ResourceID != "vm:501" || results[1].ResourceID != "vm:500" {
		t.Fatalf("expected canonical resource IDs to round-trip, got %+v", results)
	}
}

func TestActionLifecycleEvent_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 14, 0, 0, 0, time.UTC)

	events := []ActionLifecycleEvent{
		{
			ActionID:  "action-2",
			Timestamp: now,
			State:     ActionStatePlanned,
			Actor:     "system",
			Message:   "planned",
		},
		{
			ActionID:  "action-2",
			Timestamp: now.Add(1 * time.Minute),
			State:     ActionStateApproved,
			Actor:     "admin@example.com",
			Message:   "approved",
		},
	}

	for _, event := range events {
		if err := store.RecordActionLifecycleEvent(event); err != nil {
			t.Fatalf("RecordActionLifecycleEvent: %v", err)
		}
	}

	results, err := store.GetActionLifecycleEvents("action-2", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 lifecycle events, got %d", len(results))
	}
	if results[0].State != ActionStateApproved || results[1].State != ActionStatePlanned {
		t.Fatalf("unexpected lifecycle ordering: %+v", results)
	}
}

func TestExportAuditRecord_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)

	record := ExportAuditRecord{
		ID:           "export-1",
		Timestamp:    now,
		Actor:        "agent:context-router",
		EnvelopeHash: "sha256:deadbeef",
		Decision:     ExportRedacted,
		Destination:  "local-llama",
		Redactions:   []string{"metadata.hostname", "identity.ipAddresses"},
	}

	if err := store.RecordExportAudit(record); err != nil {
		t.Fatalf("RecordExportAudit: %v", err)
	}

	results, err := store.GetExportAudits(now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetExportAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 export audit, got %d", len(results))
	}

	got := results[0]
	if got.ID != record.ID || got.Decision != record.Decision || got.Destination != record.Destination {
		t.Fatalf("unexpected export audit round-trip: %+v", got)
	}
	if len(got.Redactions) != len(record.Redactions) || got.Redactions[0] != record.Redactions[0] || got.Redactions[1] != record.Redactions[1] {
		t.Fatalf("redactions round-trip failed: %+v", got.Redactions)
	}
}

func TestGetRecentChanges_RespectsTimeFilter(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Second)

	old := ResourceChange{ID: "chg-old", ResourceID: "vm:1", ObservedAt: base.Add(-2 * time.Hour), Kind: ChangeStateTransition, SourceType: "proxmox", Confidence: ConfidenceHigh}
	recent := ResourceChange{ID: "chg-new", ResourceID: "vm:1", ObservedAt: base, Kind: ChangeStateTransition, SourceType: "proxmox", Confidence: ConfidenceHigh}

	for _, c := range []ResourceChange{old, recent} {
		if err := store.RecordChange(c); err != nil {
			t.Fatalf("RecordChange: %v", err)
		}
	}

	results, err := store.GetRecentChanges("vm:1", base.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (recent only), got %d", len(results))
	}
	if results[0].ID != "chg-new" {
		t.Errorf("expected chg-new, got %q", results[0].ID)
	}
}

func TestGetRecentChanges_RespectsLimit(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		c := ResourceChange{
			ID:         strings.Repeat("x", 3) + string(rune('0'+i)),
			ResourceID: "vm:2",
			ObservedAt: base.Add(time.Duration(i) * time.Second),
			Kind:       ChangeStateTransition,
			SourceType: "proxmox",
			Confidence: ConfidenceHigh,
		}
		if err := store.RecordChange(c); err != nil {
			t.Fatalf("RecordChange: %v", err)
		}
	}

	results, err := store.GetRecentChanges("vm:2", base.Add(-time.Minute), 3)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (limit), got %d", len(results))
	}
}

func seedLegacyLinksTable(t *testing.T, legacyPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("sql.Open(%q) failed: %v", legacyPath, err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS resource_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_a TEXT NOT NULL,
			resource_b TEXT NOT NULL,
			primary_id TEXT NOT NULL,
			reason TEXT,
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(resource_a, resource_b)
		);
	`); err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO resource_links (resource_a, resource_b, primary_id, reason, created_by, created_at)
		VALUES ('legacy-a', 'legacy-b', 'legacy-a', 'legacy migration', 'tester', CURRENT_TIMESTAMP);
	`); err != nil {
		t.Fatalf("failed to seed legacy link row: %v", err)
	}
}

func TestSQLiteResourceStore_ResourceOperatorState_RoundTrips(t *testing.T) {
	// Smoke test the SQLite implementation of ResourceOperatorState
	// upsert + get + clear. The Memory store has a deeper test suite in
	// resource_operator_state_test.go; this run pins that the SQLite
	// schema migration succeeds and the column round-trips work end-to-end.
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, defaultOrgID)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if _, found, err := store.GetResourceOperatorState("vm:101"); err != nil {
		t.Fatalf("initial get: %v", err)
	} else if found {
		t.Fatal("fresh store must not return an entry")
	}

	start := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	want := ResourceOperatorState{
		CanonicalID:          "vm:101",
		IntentionallyOffline: true,
		NeverAutoRemediate:   true,
		MaintenanceStartAt:   &start,
		MaintenanceEndAt:     &end,
		MaintenanceReason:    "Q3 storage upgrade",
		Criticality:          CriticalityHigh,
		Note:                 "do not auto-fix",
		SetAt:                time.Date(2026, 5, 9, 11, 59, 0, 0, time.UTC),
		SetBy:                "operator:richard",
	}
	if err := store.SetResourceOperatorState(want); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, found, err := store.GetResourceOperatorState("vm:101")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !found {
		t.Fatal("expected entry after set")
	}
	if !got.IntentionallyOffline || !got.NeverAutoRemediate {
		t.Errorf("boolean flags must round-trip: %+v", got)
	}
	if got.Criticality != CriticalityHigh {
		t.Errorf("criticality: got %q want %q", got.Criticality, CriticalityHigh)
	}
	if got.MaintenanceReason != "Q3 storage upgrade" || got.Note != "do not auto-fix" {
		t.Errorf("string columns must round-trip: %+v", got)
	}
	if got.MaintenanceStartAt == nil || !got.MaintenanceStartAt.Equal(start) {
		t.Errorf("maintenance_start_at: got %v want %v", got.MaintenanceStartAt, start)
	}
	if got.MaintenanceEndAt == nil || !got.MaintenanceEndAt.Equal(end) {
		t.Errorf("maintenance_end_at: got %v want %v", got.MaintenanceEndAt, end)
	}

	// Upsert: Set again with different values should overwrite, not error.
	want.IntentionallyOffline = false
	want.Criticality = CriticalityLow
	if err := store.SetResourceOperatorState(want); err != nil {
		t.Fatalf("re-set: %v", err)
	}
	got, _, _ = store.GetResourceOperatorState("vm:101")
	if got.IntentionallyOffline {
		t.Error("upsert must overwrite IntentionallyOffline")
	}
	if got.Criticality != CriticalityLow {
		t.Errorf("upsert must overwrite criticality; got %q", got.Criticality)
	}

	// Clear: idempotent removal.
	if err := store.ClearResourceOperatorState("vm:101"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, found, _ := store.GetResourceOperatorState("vm:101"); found {
		t.Error("entry must be gone after clear")
	}
	if err := store.ClearResourceOperatorState("vm:101"); err != nil {
		t.Errorf("clear must be idempotent; got %v", err)
	}
}

func TestResourceChangeReadsMergeCanonicalIDEras(t *testing.T) {
	const machineID = "7d465a78-test-machine-id"
	steadyID := buildHashID(ResourceTypeAgent, "machine:"+machineID)
	bootEraID := buildHashID(ResourceTypeAgent, "cluster:homelab:delly")

	sqliteStore, err := NewSQLiteResourceStore(t.TempDir(), "")
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer sqliteStore.Close()

	stores := map[string]ResourceStore{
		"memory": NewMemoryStore(),
		"sqlite": sqliteStore,
	}

	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			if err := store.UpsertResourceIdentityPins([]ResourceIdentityPin{{
				CanonicalID:  steadyID,
				ResourceType: ResourceTypeAgent,
				MachineID:    machineID,
				ClusterName:  "homelab",
				Hostname:     "delly",
			}}); err != nil {
				t.Fatalf("upsert pin: %v", err)
			}

			base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
			record := func(id, resourceID string, at time.Time) {
				t.Helper()
				if err := store.RecordChange(ResourceChange{
					ID:         id,
					ResourceID: resourceID,
					ObservedAt: at,
					Kind:       ChangeStateTransition,
					SourceType: SourcePulseDiff,
					Confidence: ConfidenceHigh,
				}); err != nil {
					t.Fatalf("record change %s: %v", id, err)
				}
			}
			record("change-boot-era", bootEraID, base)
			record("change-steady-era", steadyID, base.Add(time.Hour))
			record("change-unrelated", "agent-unrelated", base.Add(2*time.Hour))

			changes, err := store.GetRecentChanges(steadyID, time.Time{}, 0)
			if err != nil {
				t.Fatalf("get changes by steady ID: %v", err)
			}
			if len(changes) != 2 {
				t.Fatalf("expected both journal eras under the steady ID, got %d changes", len(changes))
			}

			// Reads keyed by the old era ID resolve through the pin too, so
			// callers holding a stale canonical ID see the full timeline.
			changes, err = store.GetRecentChanges(bootEraID, time.Time{}, 0)
			if err != nil {
				t.Fatalf("get changes by boot-era ID: %v", err)
			}
			if len(changes) != 2 {
				t.Fatalf("expected both journal eras under the boot-era ID, got %d changes", len(changes))
			}

			count, err := store.CountRecentChanges(steadyID, time.Time{})
			if err != nil {
				t.Fatalf("count changes: %v", err)
			}
			if count != 2 {
				t.Fatalf("expected era-merged count 2, got %d", count)
			}

			unrelated, err := store.GetRecentChanges("agent-unrelated", time.Time{}, 0)
			if err != nil {
				t.Fatalf("get unrelated changes: %v", err)
			}
			if len(unrelated) != 1 {
				t.Fatalf("expected unrelated resource to keep its own timeline, got %d changes", len(unrelated))
			}
		})
	}
}

func TestPruneOldRecords_DeletesExpiredChangesAndAudits(t *testing.T) {
	store := newTestStore(t)

	old := time.Now().Add(-120 * 24 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)

	changes := []ResourceChange{
		{
			ID:         "change-old",
			ObservedAt: old,
			ResourceID: "vm:100",
			Kind:       ChangeStateTransition,
			SourceType: SourcePulseDiff,
			Confidence: ConfidenceHigh,
		},
		{
			ID:         "change-recent",
			ObservedAt: recent,
			ResourceID: "vm:100",
			Kind:       ChangeStateTransition,
			SourceType: SourcePulseDiff,
			Confidence: ConfidenceHigh,
		},
	}
	for _, ch := range changes {
		if err := store.RecordChange(ch); err != nil {
			t.Fatalf("RecordChange(%s): %v", ch.ID, err)
		}
	}

	audits := []ActionAuditRecord{
		{
			ID:        "audit-old",
			CreatedAt: old,
			UpdatedAt: old,
			State:     ActionStateCompleted,
			Request: ActionRequest{
				RequestID:      "req-old",
				ResourceID:     "vm:100",
				CapabilityName: "restart",
				RequestedBy:    "agent:test",
			},
		},
		{
			ID:        "audit-recent",
			CreatedAt: recent,
			UpdatedAt: recent,
			State:     ActionStateCompleted,
			Request: ActionRequest{
				RequestID:      "req-recent",
				ResourceID:     "vm:100",
				CapabilityName: "restart",
				RequestedBy:    "agent:test",
			},
		},
	}
	for _, a := range audits {
		if err := store.RecordActionAudit(a); err != nil {
			t.Fatalf("RecordActionAudit(%s): %v", a.ID, err)
		}
	}

	store.pruneOldRecords()

	remaining, err := store.GetRecentChanges("vm:100", time.Time{}, 0)
	if err != nil {
		t.Fatalf("GetRecentChanges after prune: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "change-recent" {
		t.Fatalf("expected only change-recent to survive, got %d: %+v", len(remaining), remaining)
	}

	auditResults, err := store.GetActionAudits("", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits after prune: %v", err)
	}
	if len(auditResults) != 1 || auditResults[0].ID != "audit-recent" {
		t.Fatalf("expected only audit-recent to survive, got %d: %+v", len(auditResults), auditResults)
	}
}

func TestCapResourceChanges_KeepsNewestRows(t *testing.T) {
	store := newTestStore(t)

	base := time.Now().Add(-10 * time.Hour)
	for i := 0; i < 10; i++ {
		change := ResourceChange{
			ID:         fmt.Sprintf("change-%d", i),
			ObservedAt: base.Add(time.Duration(i) * time.Hour),
			ResourceID: "vm:100",
			Kind:       ChangeStateTransition,
			SourceType: SourcePulseDiff,
			Confidence: ConfidenceHigh,
		}
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	deleted, err := store.capResourceChanges(3)
	if err != nil {
		t.Fatalf("capResourceChanges: %v", err)
	}
	if deleted != 7 {
		t.Fatalf("deleted = %d, want 7", deleted)
	}

	remaining, err := store.GetRecentChanges("vm:100", time.Time{}, 0)
	if err != nil {
		t.Fatalf("GetRecentChanges after cap: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("expected 3 surviving rows, got %d", len(remaining))
	}
	for _, change := range remaining {
		switch change.ID {
		case "change-7", "change-8", "change-9":
		default:
			t.Fatalf("unexpected survivor %s; want the newest three", change.ID)
		}
	}
}

func TestSQLiteStoreActionAuditOriginRoundTrip(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	record := ActionAuditRecord{
		ID:    "act_origin_roundtrip",
		State: ActionStatePending,
		Request: ActionRequest{
			RequestID:      "prop-1",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			RequestedBy:    "pulse_patrol",
		},
		Plan: ActionPlan{
			ActionID:         "act_origin_roundtrip",
			RequestID:        "prop-1",
			Allowed:          true,
			RequiresApproval: true,
		},
		Origin: &ActionOrigin{
			Surface:         " patrol ",
			FindingID:       "finding-1",
			InvestigationID: "inv-1",
			ProposalID:      "prop-1",
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	got, ok, err := store.GetActionAudit("act_origin_roundtrip")
	if err != nil || !ok {
		t.Fatalf("GetActionAudit: ok=%v err=%v", ok, err)
	}
	if got.Origin == nil {
		t.Fatal("origin was not persisted")
	}
	if got.Origin.Surface != "patrol" || got.Origin.FindingID != "finding-1" || got.Origin.InvestigationID != "inv-1" || got.Origin.ProposalID != "prop-1" {
		t.Fatalf("origin round-trip = %#v", got.Origin)
	}

	// An all-empty origin must normalize to nil, never persist as an
	// empty object.
	record.ID = "act_origin_empty"
	record.Plan.ActionID = record.ID
	record.Origin = &ActionOrigin{Surface: "  "}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit(empty origin): %v", err)
	}
	got, ok, err = store.GetActionAudit("act_origin_empty")
	if err != nil || !ok {
		t.Fatalf("GetActionAudit(empty origin): ok=%v err=%v", ok, err)
	}
	if got.Origin != nil {
		t.Fatalf("empty origin should read back nil, got %#v", got.Origin)
	}
}

func TestActionAuditOriginReaderReturnsLatestTransition(t *testing.T) {
	constructors := []struct {
		name string
		new  func(t *testing.T) ResourceStore
	}{
		{
			name: "sqlite",
			new: func(t *testing.T) ResourceStore {
				store, err := NewSQLiteResourceStore(t.TempDir(), "default")
				if err != nil {
					t.Fatalf("NewSQLiteResourceStore: %v", err)
				}
				t.Cleanup(func() { _ = store.Close() })
				return store
			},
		},
		{name: "memory", new: func(_ *testing.T) ResourceStore { return NewMemoryStore() }},
	}
	for _, tc := range constructors {
		t.Run(tc.name, func(t *testing.T) {
			store := tc.new(t)
			reader, ok := store.(ActionAuditOriginReader)
			if !ok {
				t.Fatalf("%T does not implement ActionAuditOriginReader", store)
			}
			now := time.Now().UTC()
			for _, record := range []ActionAuditRecord{
				{
					ID: "act-old", CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute), State: ActionStatePending,
					Request: ActionRequest{RequestID: "prop-old", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "pulse_patrol"},
					Plan:    ActionPlan{ActionID: "act-old", RequestID: "prop-old", Allowed: true},
					Origin:  &ActionOrigin{Surface: "patrol", FindingID: "finding-1", InvestigationID: "inv-1", ProposalID: "prop-old"},
				},
				{
					ID: "act-new", CreatedAt: now, UpdatedAt: now, State: ActionStateCompleted,
					Request:             ActionRequest{RequestID: "prop-new", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "pulse_patrol"},
					Plan:                ActionPlan{ActionID: "act-new", RequestID: "prop-new", Allowed: true},
					Origin:              &ActionOrigin{Surface: "patrol", FindingID: "finding-1", InvestigationID: "inv-1", ProposalID: "prop-new"},
					VerificationOutcome: VerificationOutcome{Status: VerificationVerified},
				},
			} {
				if err := store.RecordActionAudit(record); err != nil {
					t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
				}
			}
			got, found, err := reader.GetLatestActionAuditByOrigin("patrol", "inv-1")
			if err != nil || !found {
				t.Fatalf("GetLatestActionAuditByOrigin: found=%v err=%v", found, err)
			}
			if got.ID != "act-new" || got.State != ActionStateCompleted {
				t.Fatalf("latest audit = %#v, want act-new completed", got)
			}
		})
	}
}

func TestPendingActionAuditReaderReturnsOldestPendingFirst(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	for _, record := range []ActionAuditRecord{
		{ID: "act-new", CreatedAt: now, UpdatedAt: now, State: ActionStatePending, Request: ActionRequest{RequestID: "new", ResourceID: "vm:2", CapabilityName: "restart", RequestedBy: "pulse_patrol"}, Plan: ActionPlan{ActionID: "act-new", RequestID: "new", Allowed: true}},
		{ID: "act-old", CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute), State: ActionStatePending, Request: ActionRequest{RequestID: "old", ResourceID: "vm:1", CapabilityName: "restart", RequestedBy: "pulse_patrol"}, Plan: ActionPlan{ActionID: "act-old", RequestID: "old", Allowed: true}},
		{ID: "act-done", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), State: ActionStateCompleted, Request: ActionRequest{RequestID: "done", ResourceID: "vm:3", CapabilityName: "restart", RequestedBy: "pulse_patrol"}, Plan: ActionPlan{ActionID: "act-done", RequestID: "done", Allowed: true}},
	} {
		if err := store.RecordActionAudit(record); err != nil {
			t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
		}
	}
	got, err := store.GetPendingActionAudits(100)
	if err != nil {
		t.Fatalf("GetPendingActionAudits: %v", err)
	}
	if len(got) != 2 || got[0].ID != "act-old" || got[1].ID != "act-new" {
		t.Fatalf("pending actions = %#v, want oldest pending first", got)
	}
}
