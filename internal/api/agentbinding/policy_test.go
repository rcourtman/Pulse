package agentbinding

import (
	"testing"

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
