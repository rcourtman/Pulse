package agenttokens

import (
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

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
