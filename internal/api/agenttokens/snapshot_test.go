package agenttokens

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func assertTokenInventorySaved(t *testing.T, cfg *config.Config, persistence *config.ConfigPersistence) {
	t.Helper()
	saved, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, cfg.APITokens) {
		t.Fatal("saved token inventory differs from live authority")
	}
}

func TestIssuedInstallTokenSnapshotsDetached(t *testing.T) {
	for _, organization := range []bool{false, true} {
		for _, durable := range []bool{false, true} {
			name := "local"
			if organization {
				name = "organization"
			}
			if durable {
				name += "/durable"
			} else {
				name += "/memory"
			}
			t.Run(name, func(t *testing.T) {
				cfg := &config.Config{DataPath: t.TempDir()}
				var persistence *config.ConfigPersistence
				if durable {
					persistence = config.NewConfigPersistence(cfg.DataPath)
				}
				issue := IssueAndPersist
				if organization {
					issue = IssueAndPersistForOrganization
				}
				opts := IssueOptions{TokenName: "snapshot", OrgID: "tenant-a", OwnerUserID: "owner-a",
					Metadata: map[string]string{"bound_hostname": "host-a", CommandPolicyIntentMetadataKey: CommandPolicyIntent(false)}}
				raw, result, err := issue(cfg, persistence, opts)
				if err != nil || result == nil {
					t.Fatalf("issue failed: %v", err)
				}
				want := cfg.APITokens[0].Clone()
				if !reflect.DeepEqual(*result, want) {
					t.Fatal("issuance snapshot differs from admitted values")
				}
				result.Metadata["bound_hostname"] = "caller-only-host"
				result.Metadata[RuntimeRoleMetadataKey] = "caller-only-role"
				result.Scopes[0] = config.ScopeSettingsWrite
				if !reflect.DeepEqual(cfg.APITokens[0], want) || !cfg.IsValidAPIToken(raw) {
					t.Error("editing the issuance result changed admitted credential authority")
				}
				if durable {
					assertTokenInventorySaved(t, cfg, persistence)
				}
				callerAfterEdit := result.Clone()
				config.Mu.Lock()
				cfg.APITokens[0].Metadata["bound_agent_id"] = "canonical-host-a"
				config.Mu.Unlock()
				if !reflect.DeepEqual(*result, callerAfterEdit) {
					t.Error("later live binding rewrote the issuance snapshot")
				}
			})
		}
	}
}

func TestIssuedActionRunnerSnapshotSurvivesActivation(t *testing.T) {
	for _, detailed := range []bool{false, true} {
		name := "wrapper"
		if detailed {
			name = "detailed"
		}
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{DataPath: t.TempDir()}
			persistence := config.NewConfigPersistence(cfg.DataPath)
			opts := ActionRunnerIssueOptions{OrgID: "tenant-a", AgentID: "host-a", Hostname: "host-a.example"}
			var result *config.APITokenRecord
			var err error
			if detailed {
				issued, issueErr := IssueActionRunnerAndPersistDetailed(cfg, persistence, opts)
				result, err = issued.Record, issueErr
			} else {
				_, result, err = IssueActionRunnerAndPersist(cfg, persistence, opts)
			}
			if err != nil || result == nil || result.ExpiresAt == nil {
				t.Fatalf("issue failed: %v", err)
			}
			want := result.Clone()
			activated, revoked, changed, err := ActivateActionRunnerAndPersist(cfg, persistence, result.ID, opts.AgentID, opts.Hostname)
			if err != nil || !changed || len(revoked) != 0 || activated == nil {
				t.Fatalf("activation failed: changed=%v revoked=%d error=%v", changed, len(revoked), err)
			}
			if !reflect.DeepEqual(*result, want) {
				t.Error("live activation rewrote the issued pending snapshot")
			}
			if activated.ExpiresAt != nil || activated.Metadata[ActionRunnerActivationPendingMetadataKey] != "" ||
				cfg.APITokens[0].ExpiresAt != nil || cfg.APITokens[0].Metadata[ActionRunnerActivationPendingMetadataKey] != "" {
				t.Error("activation did not commit its separate live authority")
			}
			assertTokenInventorySaved(t, cfg, persistence)
		})
	}
}

func TestIssuedActionRunnerSnapshotCannotRewritePendingAuthority(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	persistence := config.NewConfigPersistence(cfg.DataPath)
	opts := ActionRunnerIssueOptions{OrgID: "tenant-a", AgentID: "host-a", Hostname: "host-a.example"}
	_, result, err := IssueActionRunnerAndPersist(cfg, persistence, opts)
	if err != nil || result == nil || result.ExpiresAt == nil {
		t.Fatalf("issue failed: %v", err)
	}
	want := cfg.APITokens[0].Clone()
	result.Metadata["bound_agent_id"] = "caller-only-host"
	delete(result.Metadata, ActionRunnerActivationPendingMetadataKey)
	*result.ExpiresAt = time.Now().UTC().Add(-time.Hour)
	if !reflect.DeepEqual(cfg.APITokens[0], want) {
		t.Error("caller rewrote the live pending binding or expiry")
	}
	assertTokenInventorySaved(t, cfg, persistence)
	activated, _, changed, err := ActivateActionRunnerAndPersist(cfg, persistence, want.ID, opts.AgentID, opts.Hostname)
	if err != nil || !changed || activated == nil {
		t.Fatalf("caller mutation corrupted valid live activation: changed=%v error=%v", changed, err)
	}
}

func TestIssuedTokenSnapshotConcurrentInventoryWrites(t *testing.T) {
	for _, runner := range []bool{false, true} {
		name := "collector"
		if runner {
			name = "action-runner"
		}
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			var snapshot *config.APITokenRecord
			var err error
			if runner {
				_, snapshot, err = IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
					OrgID: "tenant-a", AgentID: "host-a", Hostname: "host-a.example",
				})
			} else {
				_, snapshot, err = IssueAndPersist(cfg, nil, IssueOptions{OrgID: "tenant-a"})
			}
			if err != nil {
				t.Fatal(err)
			}
			want := snapshot.Clone()
			start := make(chan struct{})
			var callers sync.WaitGroup
			callers.Go(func() {
				<-start
				for range 1000 {
					config.Mu.Lock()
					cfg.APITokens[0].Metadata["bound_agent_id"] = "live-inventory-host"
					if cfg.APITokens[0].ExpiresAt != nil {
						*cfg.APITokens[0].ExpiresAt = cfg.APITokens[0].ExpiresAt.Add(time.Nanosecond)
					}
					config.Mu.Unlock()
				}
			})
			for range 4 {
				callers.Go(func() {
					<-start
					for range 1000 {
						got := snapshot.Clone()
						if !reflect.DeepEqual(got, want) {
							t.Error("live inventory writes changed the detached snapshot")
							return
						}
					}
				})
			}
			close(start)
			callers.Wait()
		})
	}
}
