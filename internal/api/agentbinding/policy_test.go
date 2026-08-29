package agentbinding

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestEvaluateInstallTokenBinding(t *testing.T) {
	record := &config.APITokenRecord{Metadata: map[string]string{
		"install_type": "host",
		"issued_via":   IssuedViaConfig,
	}}
	decision := Evaluate(record, "machine-id", "node.example")
	if !decision.Admit || !decision.FirstBind || decision.LegacyMigrate {
		t.Fatalf("fresh install-token decision = %+v", decision)
	}

	record.Metadata["bound_hostname"] = "node"
	decision = Evaluate(record, "machine-id", "node.example")
	if !decision.Admit || !decision.FirstBind || decision.LegacyMigrate {
		t.Fatalf("auto-registered install-token decision = %+v", decision)
	}

	record.Metadata[VersionKey] = Version
	if decision := Evaluate(record, "machine-id", "other.example"); decision.Admit {
		t.Fatalf("versioned mismatched binding admitted: %+v", decision)
	}
}

func TestCanBindInstallTokenRejectsUnsupportedIssuer(t *testing.T) {
	record := &config.APITokenRecord{Metadata: map[string]string{
		"install_type": "host",
		"issued_via":   "untrusted",
	}}
	if CanBindInstallToken(record, "machine-id", "node") {
		t.Fatal("unsupported issuer admitted")
	}
}

func TestEvaluateActionRunnerRequiresExactTypedPrebinding(t *testing.T) {
	record := &config.APITokenRecord{
		OrgID:  "org-a",
		Scopes: []string{config.ScopeAgentExec},
		Metadata: map[string]string{
			agenttokens.RuntimeRoleMetadataKey:          agenttokens.CredentialKindActionRunner,
			agenttokens.ActionCapabilityMetadataKey:     agenttokens.ActionCapabilityTypedV1,
			agenttokens.ActionBindingVersionMetadataKey: agenttokens.ActionBindingVersion,
			"bound_agent_id":                            "machine-a",
			"bound_hostname":                            "node.example",
		},
	}
	if decision := EvaluateActionRunner(record, "machine-a", "NODE"); !decision.Admit || decision.FirstBind || decision.LegacyMigrate {
		t.Fatalf("typed action runner decision = %+v", decision)
	}
	for _, mutate := range []func(map[string]string){
		func(metadata map[string]string) {
			metadata[agenttokens.RuntimeRoleMetadataKey] = agenttokens.CredentialKindMonitoringCollector
		},
		func(metadata map[string]string) { metadata[agenttokens.ActionCapabilityMetadataKey] = "shell.v1" },
		func(metadata map[string]string) { metadata[agenttokens.ActionBindingVersionMetadataKey] = "2" },
		func(metadata map[string]string) { metadata["bound_agent_id"] = "other" },
		func(metadata map[string]string) { metadata["bound_hostname"] = "other.example" },
	} {
		clone := record.Clone()
		clone.Metadata = make(map[string]string, len(record.Metadata))
		for key, value := range record.Metadata {
			clone.Metadata[key] = value
		}
		mutate(clone.Metadata)
		if decision := EvaluateActionRunner(&clone, "machine-a", "node.example"); decision.Admit {
			t.Fatalf("mismatched action runner admitted: metadata=%#v decision=%+v", clone.Metadata, decision)
		}
	}
}
