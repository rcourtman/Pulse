package agenttokens

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type actionRunnerTestFS struct {
	blockOnce sync.Once
	entered   chan struct{}
	release   chan struct{}
	writeErr  error
}

type actionRunnerFailAfterWritesFS struct {
	mu          sync.Mutex
	writes      int
	allowWrites int
}

func (fs *actionRunnerFailAfterWritesFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}
func (fs *actionRunnerFailAfterWritesFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	fs.mu.Lock()
	fs.writes++
	writes := fs.writes
	fs.mu.Unlock()
	if writes > fs.allowWrites {
		return errors.New("injected rollback persistence failure")
	}
	return os.WriteFile(name, data, perm)
}
func (fs *actionRunnerFailAfterWritesFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
func (fs *actionRunnerFailAfterWritesFS) Remove(name string) error { return os.Remove(name) }
func (fs *actionRunnerFailAfterWritesFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
func (fs *actionRunnerFailAfterWritesFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *actionRunnerTestFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (fs *actionRunnerTestFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	blocked := false
	fs.blockOnce.Do(func() {
		blocked = fs.entered != nil
		if blocked {
			close(fs.entered)
		}
	})
	if blocked {
		<-fs.release
	}
	if fs.writeErr != nil {
		return fs.writeErr
	}
	return os.WriteFile(name, data, perm)
}
func (fs *actionRunnerTestFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
func (fs *actionRunnerTestFS) Remove(name string) error { return os.Remove(name) }
func (fs *actionRunnerTestFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
func (fs *actionRunnerTestFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func TestIssueAndPersistInstallToken(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	raw, record, err := IssueAndPersist(cfg, nil, IssueOptions{
		TokenName:   "host-agent",
		OwnerUserID: "operator",
		Scopes:      HostScopes(true),
		Metadata:    map[string]string{"install_type": "host"},
	})
	if err != nil {
		t.Fatalf("IssueAndPersist: %v", err)
	}
	if raw == "" || record == nil || len(cfg.APITokens) != 1 {
		t.Fatalf("issued token = (%q, %#v), persisted=%d", raw, record, len(cfg.APITokens))
	}
	if OwnerUserID(*record) != "operator" || record.Metadata[IssuedAtMetadataKey] == "" {
		t.Fatalf("issued metadata = %#v", record.Metadata)
	}
	if !record.HasScope(config.ScopeAgentExec) {
		t.Fatalf("commands-enabled host scopes = %v", record.Scopes)
	}
	if got := record.Metadata[RuntimeRoleMetadataKey]; got != CredentialKindLegacyFullTrust {
		t.Fatalf("combined install runtime role = %q, want %q", got, CredentialKindLegacyFullTrust)
	}
}

func TestIssueActionRunnerAndPersistIsHostBoundAndExecOnly(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	raw, record, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: " Node.EXAMPLE. ", OwnerUserID: "operator",
	})
	if err != nil {
		t.Fatalf("IssueActionRunnerAndPersist: %v", err)
	}
	if raw == "" || record == nil {
		t.Fatalf("issued action credential = (%q, %#v)", raw, record)
	}
	if len(record.Scopes) != 1 || record.Scopes[0] != config.ScopeAgentExec {
		t.Fatalf("action credential scopes = %v, want only %q", record.Scopes, config.ScopeAgentExec)
	}
	if record.HasScope(config.ScopeAgentReport) || record.HasScope(config.ScopeAgentConfigRead) || record.HasScope(config.ScopeAgentManage) {
		t.Fatalf("action credential inherited collector authority: %v", record.Scopes)
	}
	if record.OrgID != "org-a" || record.Metadata["bound_agent_id"] != "machine-123" || record.Metadata["bound_hostname"] != "node.example" {
		t.Fatalf("action credential binding = org=%q metadata=%#v", record.OrgID, record.Metadata)
	}
	if record.Metadata[RuntimeRoleMetadataKey] != CredentialKindActionRunner ||
		record.Metadata[ActionCapabilityMetadataKey] != ActionCapabilityTypedV1 ||
		record.Metadata[ActionBindingVersionMetadataKey] != ActionBindingVersion {
		t.Fatalf("action credential authority metadata = %#v", record.Metadata)
	}
}

func TestIssueActionRunnerAndPersistRejectsIncompleteBinding(t *testing.T) {
	for _, tc := range []ActionRunnerIssueOptions{
		{AgentID: "machine", Hostname: "node"},
		{OrgID: "org", Hostname: "node"},
		{OrgID: "org", AgentID: "machine"},
	} {
		if raw, record, err := IssueActionRunnerAndPersist(&config.Config{}, nil, tc); !errors.Is(err, ErrRecord) || raw != "" || record != nil {
			t.Fatalf("incomplete binding %#v = (%q, %#v, %v), want ErrRecord", tc, raw, record, err)
		}
	}
}

func TestIssueActionRunnerAndPersistRejectsAgentIDOutsideRunnerVocabulary(t *testing.T) {
	for _, agentID := range []string{"bad agent", "-agent", strings.Repeat("a", 129)} {
		t.Run(agentID, func(t *testing.T) {
			raw, record, err := IssueActionRunnerAndPersist(&config.Config{}, nil, ActionRunnerIssueOptions{
				OrgID: "org-a", AgentID: agentID, Hostname: "node.example",
			})
			if !errors.Is(err, ErrRecord) || raw != "" || record != nil {
				t.Fatalf("IssueActionRunnerAndPersist = (%q, %#v, %v), want ErrRecord", raw, record, err)
			}
		})
	}
}

func TestIssueActionRunnerAndPersistPreparesRotationWithoutRevokingActiveCredential(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	firstToken, first, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "Node.EXAMPLE",
	})
	if err != nil {
		t.Fatalf("first IssueActionRunnerAndPersist: %v", err)
	}
	if _, _, changed, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), first.ID, "machine-123", "node.example"); err != nil || !changed {
		t.Fatalf("activate first credential = changed %v, error %v", changed, err)
	}
	_, otherHost, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-456", Hostname: "other.example",
	})
	if err != nil {
		t.Fatalf("other-host IssueActionRunnerAndPersist: %v", err)
	}
	_, replacement, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		// Canonical agent identity survives a host rename. The old binding can no
		// longer admit that host and must not remain as an orphaned secret.
		OrgID: "org-a", AgentID: "machine-123", Hostname: "renamed.example",
	})
	if err != nil {
		t.Fatalf("replacement IssueActionRunnerAndPersist: %v", err)
	}
	if replacement.ID == first.ID {
		t.Fatalf("replacement reused token id %q", replacement.ID)
	}
	if len(cfg.APITokens) != 3 {
		t.Fatalf("token inventory = %#v, want pending replacement, active predecessor, and other host", cfg.APITokens)
	}
	foundReplacement, foundOther, foundFirst := false, false, false
	for _, record := range cfg.APITokens {
		foundReplacement = foundReplacement || record.ID == replacement.ID
		foundOther = foundOther || record.ID == otherHost.ID
		foundFirst = foundFirst || record.ID == first.ID
	}
	if !foundReplacement || !foundOther || !foundFirst {
		t.Fatalf("replacement inventory = %#v", cfg.APITokens)
	}
	if _, ok := cfg.ValidateAPIToken(firstToken); !ok {
		t.Fatal("prepare step revoked the active predecessor")
	}
	if replacement.ExpiresAt == nil || replacement.Metadata[ActionRunnerActivationPendingMetadataKey] != "true" || replacement.Metadata[ActionRunnerReplacesTokenIDsMetadataKey] != first.ID {
		t.Fatalf("prepared replacement = %#v", replacement)
	}
}

func TestIssueActionRunnerAndPersistDetailedReturnsOnlyDurablyReplacedRecords(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, prior, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := IssueActionRunnerAndPersistDetailed(cfg, config.NewConfigPersistence(cfg.DataPath), ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "renamed.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.Record == nil || len(result.Replaced) != 1 || result.Replaced[0].ID != prior.ID {
		t.Fatalf("detailed issue result = %#v", result)
	}
	if result.Record.ID == prior.ID || len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != result.Record.ID {
		t.Fatalf("persisted replacement = %#v", cfg.APITokens)
	}
}

func TestActivateActionRunnerAndPersistAtomicallyPromotesAndRevokesPredecessor(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	firstToken, first, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), first.ID, "machine-123", "node.example"); err != nil {
		t.Fatal(err)
	}
	secondToken, second, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	activated, revoked, changed, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), second.ID, "machine-123", "NODE")
	if err != nil || !changed {
		t.Fatalf("activation = (%#v, %#v, %v, %v)", activated, revoked, changed, err)
	}
	if activated.ExpiresAt != nil || activated.Metadata[ActionRunnerActivationPendingMetadataKey] != "" || len(revoked) != 1 || revoked[0].ID != first.ID {
		t.Fatalf("activation result = activated %#v, revoked %#v", activated, revoked)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != second.ID {
		t.Fatalf("activated inventory = %#v", cfg.APITokens)
	}
	if _, ok := cfg.ValidateAPIToken(firstToken); ok {
		t.Fatal("activated rotation left predecessor valid")
	}
	if _, ok := cfg.ValidateAPIToken(secondToken); !ok {
		t.Fatal("activated replacement is not valid")
	}
	if _, revoked, changed, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), second.ID, "machine-123", "node.example"); err != nil || changed || len(revoked) != 0 {
		t.Fatalf("idempotent activation = revoked %#v, changed %v, error %v", revoked, changed, err)
	}
}

func TestActivateActionRunnerAndPersistRequiresDurablePersistenceWithoutMutation(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, pending, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	before := cloneAPITokenRecords(cfg.APITokens)
	if activated, revoked, changed, err := ActivateActionRunnerAndPersist(
		cfg, nil, pending.ID, "machine-123", "node.example",
	); !errors.Is(err, ErrPersist) || activated != nil || revoked != nil || changed {
		t.Fatalf("nil persistence activation = activated %#v revoked %#v changed %v err %v", activated, revoked, changed, err)
	}
	if !reflect.DeepEqual(cfg.APITokens, before) {
		t.Fatalf("nil persistence mutated inventory: before %#v after %#v", before, cfg.APITokens)
	}
}

func TestActivateActionRunnerAndPersistFailureRestoresPendingAndActiveInventory(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, first, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), first.ID, "machine-123", "node.example"); err != nil {
		t.Fatal(err)
	}
	_, second, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "blocked-state")
	persistence := config.NewConfigPersistence(statePath)
	if err := os.RemoveAll(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, changed, err := ActivateActionRunnerAndPersist(cfg, persistence, second.ID, "machine-123", "node.example"); !errors.Is(err, ErrPersist) || changed {
		t.Fatalf("activation = changed %v, error %v", changed, err)
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("restored inventory = %#v", cfg.APITokens)
	}
	for _, record := range cfg.APITokens {
		if record.ID == second.ID && (record.ExpiresAt == nil || record.Metadata[ActionRunnerActivationPendingMetadataKey] != "true" || record.Metadata[ActionRunnerReplacesTokenIDsMetadataKey] != first.ID) {
			t.Fatalf("pending replacement was not fully restored: %#v", record)
		}
	}
}

func TestActivateActionRunnerAndPersistWithPromotionRollsBackWhenExactTransportVanished(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, predecessor, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), predecessor.ID, "machine-123", "node.example"); err != nil {
		t.Fatal(err)
	}
	_, pending, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	persistence := config.NewConfigPersistence(cfg.DataPath)
	promotionCalls := 0
	activated, revoked, changed, err := ActivateActionRunnerAndPersistWithPromotion(cfg, persistence, pending.ID, "machine-123", "node.example", func() bool {
		promotionCalls++
		return false
	})
	if !errors.Is(err, ErrActionRunnerSessionUnavailable) || activated != nil || revoked != nil || changed {
		t.Fatalf("vanished transport activation = activated %#v revoked %#v changed %v err %v", activated, revoked, changed, err)
	}
	if promotionCalls != 1 {
		t.Fatalf("promotion calls = %d, want 1", promotionCalls)
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("rolled-back inventory = %#v", cfg.APITokens)
	}
	var foundPredecessor, foundPending bool
	for _, record := range cfg.APITokens {
		switch record.ID {
		case predecessor.ID:
			foundPredecessor = record.ExpiresAt == nil
		case pending.ID:
			foundPending = record.ExpiresAt != nil && record.Metadata[ActionRunnerActivationPendingMetadataKey] == "true"
		}
	}
	if !foundPredecessor || !foundPending {
		t.Fatalf("rollback did not restore predecessor and pending replacement: %#v", cfg.APITokens)
	}
	persisted, err := config.NewConfigPersistence(cfg.DataPath).LoadAPITokens()
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 2 {
		t.Fatalf("durable rollback inventory = %#v", persisted)
	}
}

func TestActivateActionRunnerPromotionRollbackPersistenceFailureKeepsLastDurableActivation(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, predecessor, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), predecessor.ID, "machine-123", "node.example"); err != nil {
		t.Fatal(err)
	}
	_, pending, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	persistence := config.NewConfigPersistence(cfg.DataPath)
	// The pre-existing durable predecessor inventory causes activation to write
	// one backup plus the new token file. Permit both, then fail the compensating
	// rollback writes.
	persistence.SetFileSystem(&actionRunnerFailAfterWritesFS{allowWrites: 2})
	activated, revoked, changed, err := ActivateActionRunnerAndPersistWithPromotion(cfg, persistence, pending.ID, "machine-123", "node.example", func() bool { return false })
	if !errors.Is(err, ErrActionRunnerActivationIndeterminate) || activated == nil || activated.ID != pending.ID || len(revoked) != 1 || revoked[0].ID != predecessor.ID || !changed {
		t.Fatalf("rollback persistence failure = activated %#v revoked %#v changed %v err %v", activated, revoked, changed, err)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != pending.ID || cfg.APITokens[0].ExpiresAt != nil || cfg.APITokens[0].Metadata[ActionRunnerActivationPendingMetadataKey] != "" {
		t.Fatalf("memory diverged from last durable activation: %#v", cfg.APITokens)
	}
	persisted, err := config.NewConfigPersistence(cfg.DataPath).LoadAPITokens()
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].ID != pending.ID || persisted[0].ExpiresAt != nil || persisted[0].Metadata[ActionRunnerActivationPendingMetadataKey] != "" {
		t.Fatalf("last durable activation inventory = %#v", persisted)
	}
}

func TestCancelPendingActionRunnerAndPersistSerializesWithActivationInBothOrders(t *testing.T) {
	seed := func(t *testing.T) (*config.Config, *config.ConfigPersistence, *config.APITokenRecord, *config.APITokenRecord) {
		t.Helper()
		cfg := &config.Config{DataPath: t.TempDir()}
		_, first, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), first.ID, "machine-123", "node.example"); err != nil {
			t.Fatal(err)
		}
		_, second, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
		if err != nil {
			t.Fatal(err)
		}
		return cfg, config.NewConfigPersistence(cfg.DataPath), first, second
	}

	t.Run("cancel wins", func(t *testing.T) {
		cfg, persistence, first, second := seed(t)
		removed, err := CancelPendingActionRunnerAndPersist(cfg, persistence, second.ID, "org-a", "machine-123", "NODE")
		if err != nil || removed == nil || removed.ID != second.ID {
			t.Fatalf("cancel = (%#v, %v)", removed, err)
		}
		if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != first.ID {
			t.Fatalf("cancelled inventory = %#v", cfg.APITokens)
		}
		if _, _, _, err := ActivateActionRunnerAndPersist(cfg, persistence, second.ID, "machine-123", "node.example"); !errors.Is(err, ErrRecord) {
			t.Fatalf("late activation error = %v, want ErrRecord", err)
		}
	})

	t.Run("activation wins", func(t *testing.T) {
		cfg, persistence, _, second := seed(t)
		if _, _, changed, err := ActivateActionRunnerAndPersist(cfg, persistence, second.ID, "machine-123", "node.example"); err != nil || !changed {
			t.Fatalf("activation = changed %v, error %v", changed, err)
		}
		if removed, err := CancelPendingActionRunnerAndPersist(cfg, persistence, second.ID, "org-a", "machine-123", "node.example"); !errors.Is(err, ErrActionRunnerAlreadyActivated) || removed != nil {
			t.Fatalf("post-commit cancel = (%#v, %v)", removed, err)
		}
		if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != second.ID || cfg.APITokens[0].ExpiresAt != nil {
			t.Fatalf("activated inventory = %#v", cfg.APITokens)
		}
	})
}

func TestCancelPendingActionRunnerAndPersistContendsUnderOneDurableTransaction(t *testing.T) {
	seed := func(t *testing.T) (*config.Config, *config.ConfigPersistence, *config.APITokenRecord, *config.APITokenRecord) {
		t.Helper()
		cfg := &config.Config{DataPath: t.TempDir()}
		_, first, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := ActivateActionRunnerAndPersist(cfg, config.NewConfigPersistence(cfg.DataPath), first.ID, "machine-123", "node.example"); err != nil {
			t.Fatal(err)
		}
		_, pending, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
		if err != nil {
			t.Fatal(err)
		}
		return cfg, config.NewConfigPersistence(cfg.DataPath), first, pending
	}
	waitSignal := func(t *testing.T, signal <-chan struct{}, label string) {
		t.Helper()
		select {
		case <-signal:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s", label)
		}
	}
	waitErr := func(t *testing.T, result <-chan error, label string) error {
		t.Helper()
		select {
		case err := <-result:
			return err
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s", label)
			return nil
		}
	}

	t.Run("cancel owns lock before activation", func(t *testing.T) {
		cfg, persistence, predecessor, pending := seed(t)
		fs := &actionRunnerTestFS{entered: make(chan struct{}), release: make(chan struct{})}
		persistence.SetFileSystem(fs)
		cancelResult := make(chan error, 1)
		go func() {
			_, err := CancelPendingActionRunnerAndPersist(cfg, persistence, pending.ID, "org-a", "machine-123", "node.example")
			cancelResult <- err
		}()
		waitSignal(t, fs.entered, "cancel persistence")
		activateResult := make(chan error, 1)
		go func() {
			_, _, _, err := ActivateActionRunnerAndPersist(cfg, persistence, pending.ID, "machine-123", "node.example")
			activateResult <- err
		}()
		close(fs.release)
		if err := waitErr(t, cancelResult, "cancel result"); err != nil {
			t.Fatalf("cancel error = %v", err)
		}
		if err := waitErr(t, activateResult, "late activation result"); !errors.Is(err, ErrRecord) {
			t.Fatalf("late activation error = %v, want ErrRecord", err)
		}
		if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != predecessor.ID {
			t.Fatalf("cancel-wins inventory = %#v", cfg.APITokens)
		}
	})

	t.Run("activation owns lock before cancel", func(t *testing.T) {
		cfg, persistence, predecessor, pending := seed(t)
		fs := &actionRunnerTestFS{entered: make(chan struct{}), release: make(chan struct{})}
		persistence.SetFileSystem(fs)
		activateResult := make(chan error, 1)
		go func() {
			_, _, _, err := ActivateActionRunnerAndPersist(cfg, persistence, pending.ID, "machine-123", "node.example")
			activateResult <- err
		}()
		waitSignal(t, fs.entered, "activation persistence")
		cancelResult := make(chan error, 1)
		go func() {
			_, err := CancelPendingActionRunnerAndPersist(cfg, persistence, pending.ID, "org-a", "machine-123", "node.example")
			cancelResult <- err
		}()
		close(fs.release)
		if err := waitErr(t, activateResult, "activation result"); err != nil {
			t.Fatalf("activation error = %v", err)
		}
		if err := waitErr(t, cancelResult, "post-commit cancel result"); !errors.Is(err, ErrActionRunnerAlreadyActivated) {
			t.Fatalf("post-commit cancel error = %v, want ErrActionRunnerAlreadyActivated", err)
		}
		if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != pending.ID || cfg.APITokens[0].ExpiresAt != nil || cfg.APITokens[0].Metadata[ActionRunnerActivationPendingMetadataKey] != "" || cfg.APITokens[0].ID == predecessor.ID {
			t.Fatalf("activation-wins inventory = %#v", cfg.APITokens)
		}
	})
}

func TestCancelPendingActionRunnerAndPersistFailureNeverAuthorizesRollback(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, pending, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example"})
	if err != nil {
		t.Fatal(err)
	}
	persistence := config.NewConfigPersistence(t.TempDir())
	persistence.SetFileSystem(&actionRunnerTestFS{writeErr: errors.New("injected persistence failure")})
	if removed, err := CancelPendingActionRunnerAndPersist(cfg, persistence, pending.ID, "org-a", "machine-123", "node.example"); !errors.Is(err, ErrPersist) || removed != nil {
		t.Fatalf("persistence failure = (%#v, %v)", removed, err)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != pending.ID || cfg.APITokens[0].Metadata[ActionRunnerActivationPendingMetadataKey] != "true" {
		t.Fatalf("failed cancel changed inventory = %#v", cfg.APITokens)
	}
	if removed, err := CancelPendingActionRunnerAndPersist(cfg, nil, pending.ID, "org-a", "machine-123", "node.example"); !errors.Is(err, ErrPersist) || removed != nil {
		t.Fatalf("nil persistence = (%#v, %v)", removed, err)
	}
}

func TestIssueActionRunnerAndPersistRestoresReplacedCredentialOnPersistenceFailure(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, prior, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example",
	})
	if err != nil {
		t.Fatalf("seed action credential: %v", err)
	}
	_, unrelated, err := IssueAndPersist(cfg, nil, IssueOptions{TokenName: "collector", OrgID: "org-a"})
	if err != nil {
		t.Fatalf("seed unrelated credential: %v", err)
	}

	statePath := filepath.Join(t.TempDir(), "blocked-state")
	persistence := config.NewConfigPersistence(statePath)
	if err := os.RemoveAll(statePath); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}
	raw, replacement, err := IssueActionRunnerAndPersist(cfg, persistence, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example",
	})
	if !errors.Is(err, ErrPersist) {
		t.Fatalf("replacement error = %v, want ErrPersist", err)
	}
	if raw != "" || replacement != nil {
		t.Fatalf("failed replacement returned credential: raw=%q record=%#v", raw, replacement)
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("restored inventory = %#v", cfg.APITokens)
	}
	foundPrior, foundUnrelated := false, false
	for _, record := range cfg.APITokens {
		foundPrior = foundPrior || record.ID == prior.ID
		foundUnrelated = foundUnrelated || record.ID == unrelated.ID
	}
	if !foundPrior || !foundUnrelated {
		t.Fatalf("full prior inventory was not restored: %#v", cfg.APITokens)
	}
}

func TestIssueAndPersistRejectsMonitoringRoleWithExecAuthority(t *testing.T) {
	_, _, err := IssueAndPersist(&config.Config{}, nil, IssueOptions{
		TokenName: "invalid",
		Scopes:    []string{config.ScopeAgentReport, config.ScopeAgentExec},
		Metadata:  map[string]string{RuntimeRoleMetadataKey: CredentialKindMonitoringCollector},
	})
	if !errors.Is(err, ErrRecord) {
		t.Fatalf("monitoring role with exec authority error = %v, want ErrRecord", err)
	}
}

func TestIssueAndPersistRejectsMonitoringRoleWithUnrelatedAuthority(t *testing.T) {
	for _, scopes := range [][]string{
		{config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeSettingsWrite},
		{config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeActionsExecute},
	} {
		_, _, err := IssueAndPersist(&config.Config{}, nil, IssueOptions{
			TokenName: "invalid",
			Scopes:    scopes,
			Metadata:  map[string]string{RuntimeRoleMetadataKey: CredentialKindMonitoringCollector},
		})
		if !errors.Is(err, ErrRecord) {
			t.Fatalf("monitoring role with scopes %v error = %v, want ErrRecord", scopes, err)
		}
	}
}

func TestIssueAndPersistRejectsInferredMonitoringRoleWithUnrelatedAuthority(t *testing.T) {
	_, _, err := IssueAndPersist(&config.Config{}, nil, IssueOptions{
		TokenName: "invalid inferred collector",
		Scopes:    []string{config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeSettingsWrite},
	})
	if !errors.Is(err, ErrRecord) {
		t.Fatalf("inferred monitoring role with unrelated authority error = %v, want ErrRecord", err)
	}
}

func TestProxmoxScopesRequireExplicitCommandAuthority(t *testing.T) {
	monitoringScopes := ProxmoxScopes(false)
	if (&config.APITokenRecord{Scopes: monitoringScopes}).HasScope(config.ScopeAgentExec) {
		t.Fatalf("monitoring-only Proxmox scopes include %s: %v", config.ScopeAgentExec, monitoringScopes)
	}
	if (&config.APITokenRecord{Scopes: monitoringScopes}).HasScope(config.ScopeAgentManage) {
		t.Fatalf("monitoring-only Proxmox scopes include cross-host management: %v", monitoringScopes)
	}

	commandScopes := ProxmoxScopes(true)
	if !(&config.APITokenRecord{Scopes: commandScopes}).HasScope(config.ScopeAgentExec) {
		t.Fatalf("explicit command profile is missing %s: %v", config.ScopeAgentExec, commandScopes)
	}
	if (&config.APITokenRecord{Scopes: commandScopes}).HasScope(config.ScopeAgentManage) {
		t.Fatalf("collector command profile includes cross-host management: %v", commandScopes)
	}
}

func TestIssueAndPersistDefaultsToMonitoringOnlyScopes(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, record, err := IssueAndPersist(cfg, nil, IssueOptions{TokenName: "default-agent"})
	if err != nil {
		t.Fatalf("IssueAndPersist: %v", err)
	}
	if record.HasScope(config.ScopeAgentExec) {
		t.Fatalf("implicit install scopes include %s: %v", config.ScopeAgentExec, record.Scopes)
	}
}

func TestIssueAndPersistRollsBackCompleteInventoryWhenPersistenceFails(t *testing.T) {
	now := time.Now().UTC()
	tokens := []config.APITokenRecord{
		{ID: "newest", Name: "newest", Hash: "hash-newest", CreatedAt: now, Scopes: []string{config.ScopeWildcard}},
		{ID: "oldest", Name: "oldest", Hash: "hash-oldest", CreatedAt: now.Add(-time.Minute), Scopes: []string{config.ScopeWildcard}},
	}
	cfg := &config.Config{APITokens: append([]config.APITokenRecord(nil), tokens...)}
	cfg.SortAPITokens()

	stateDir := filepath.Join(t.TempDir(), "state")
	persistence := config.NewConfigPersistence(stateDir)
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(stateDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}

	raw, record, err := IssueAndPersist(cfg, persistence, IssueOptions{TokenName: "must-not-survive"})
	if !errors.Is(err, ErrPersist) {
		t.Fatalf("IssueAndPersist error = %v, want ErrPersist", err)
	}
	if raw != "" || record != nil {
		t.Fatalf("failed issue returned credential material: raw=%q record=%#v", raw, record)
	}
	if len(cfg.APITokens) != len(tokens) {
		t.Fatalf("token count = %d, want %d: %#v", len(cfg.APITokens), len(tokens), cfg.APITokens)
	}
	for index, want := range tokens {
		if cfg.APITokens[index].ID != want.ID {
			t.Fatalf("token[%d].ID = %q, want %q", index, cfg.APITokens[index].ID, want.ID)
		}
	}
	if cfg.APIToken != "hash-newest" {
		t.Fatalf("legacy primary token = %q, want rollback to %q", cfg.APIToken, "hash-newest")
	}
	for _, token := range cfg.APITokens {
		if token.Name == "must-not-survive" {
			t.Fatalf("failed issue left generated install token active: %#v", token)
		}
	}
}

func TestIssueAndPersistRejectsReservedOwnerMetadata(t *testing.T) {
	_, _, err := IssueAndPersist(&config.Config{}, nil, IssueOptions{
		TokenName: "invalid",
		Metadata:  map[string]string{OwnerUserIDMetadataKey: "forged"},
	})
	if !errors.Is(err, ErrRecord) {
		t.Fatalf("reserved owner metadata error = %v, want ErrRecord", err)
	}
}

func TestCommandPolicyIntentRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		enabled bool
		valid   bool
	}{
		{name: "enabled", value: CommandPolicyIntent(true), enabled: true, valid: true},
		{name: "disabled", value: CommandPolicyIntent(false), enabled: false, valid: true},
		{name: "missing", value: "", enabled: false, valid: false},
		{name: "invalid", value: "yes", enabled: false, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := &config.APITokenRecord{Metadata: map[string]string{
				CommandPolicyIntentMetadataKey: tc.value,
			}}
			enabled, valid := ParseCommandPolicyIntent(record)
			if enabled != tc.enabled || valid != tc.valid {
				t.Fatalf("ParseCommandPolicyIntent() = (%v, %v), want (%v, %v)", enabled, valid, tc.enabled, tc.valid)
			}
		})
	}

	if _, valid := ParseCommandPolicyIntent(nil); valid {
		t.Fatal("nil record unexpectedly carried command-policy intent")
	}
}
