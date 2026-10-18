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

func TestEvaluateRepairsDeployPlaceholderBindingOnce(t *testing.T) {
	historical := func() *config.APITokenRecord {
		return &config.APITokenRecord{Metadata: map[string]string{
			"bound_agent_id": "agent-delly2",
			"bound_hostname": "delly2",
			"deploy_job_id":  "dep_1",
			VersionKey:       Version,
		}}
	}

	decision := Evaluate(historical(), "machine-id", "delly2")
	if !decision.Admit || !decision.LegacyMigrate || !decision.RepairDeployIdentity {
		t.Fatalf("historical deploy placeholder decision = %+v, want a one-time identity repair", decision)
	}
	if decision := Evaluate(historical(), "machine-id", "DELLY2"); !decision.RepairDeployIdentity {
		t.Fatalf("case-only hostname difference blocked the repair: %+v", decision)
	}
	for _, host := range []string{"other-node", "delly2.lan", "delly2.site-b"} {
		if decision := Evaluate(historical(), "machine-id", host); decision.Admit {
			t.Fatalf("placeholder repaired for hostname %q: %+v", host, decision)
		}
	}
	if decision := Evaluate(historical(), "agent-delly2", "delly2"); !decision.Admit || decision.RepairDeployIdentity {
		t.Fatalf("placeholder identity itself = %+v, want an ordinary admit", decision)
	}

	for _, marker := range []string{DeployIdentityAgent, DeployIdentityRepaired} {
		record := historical()
		record.Metadata[DeployIdentityKey] = marker
		if decision := Evaluate(record, "machine-id", "delly2"); decision.Admit {
			t.Fatalf("token marked %q moved to a new identity: %+v", marker, decision)
		}
	}

	notDeployed := historical()
	delete(notDeployed.Metadata, "deploy_job_id")
	if decision := Evaluate(notDeployed, "machine-id", "delly2"); decision.Admit {
		t.Fatalf("a non-deploy token bound to agent-<hostname> moved: %+v", decision)
	}

	realIdentity := historical()
	realIdentity.Metadata["bound_agent_id"] = "machine-id"
	if decision := Evaluate(realIdentity, "another-machine", "delly2"); decision.Admit {
		t.Fatalf("a deploy token bound to a real identity moved: %+v", decision)
	}
}

// Deploy tokens issued before binding versions were stamped at enrollment, and
// never connected since, carry no version. They must still move only through
// the marked repair: the generic legacy branch would accept an equivalent
// hostname and leave no marker behind.
func TestEvaluateVersionlessDeployPlaceholderOnlyMovesThroughTheRepair(t *testing.T) {
	versionless := func() *config.APITokenRecord {
		return &config.APITokenRecord{Metadata: map[string]string{
			"bound_agent_id": "agent-a",
			"bound_hostname": "a",
			"deploy_job_id":  "dep_1",
		}}
	}
	if decision := Evaluate(versionless(), "agent-b", "a.example"); decision.Admit {
		t.Fatalf("versionless placeholder moved on an equivalent hostname: %+v", decision)
	}
	if decision := Evaluate(versionless(), "machine-id", "a"); !decision.Admit || !decision.RepairDeployIdentity {
		t.Fatalf("versionless placeholder decision = %+v, want the marked repair", decision)
	}
	if decision := Evaluate(versionless(), "agent-a", "a"); !decision.Admit || decision.RepairDeployIdentity {
		t.Fatalf("versionless placeholder identity itself = %+v, want admitted without a move", decision)
	}
}

func TestEvaluateCurrentDeployTokenBackfillsTheAgentIdentity(t *testing.T) {
	record := &config.APITokenRecord{Metadata: map[string]string{
		"bound_hostname":  "delly2",
		"deploy_job_id":   "dep_1",
		VersionKey:        Version,
		DeployIdentityKey: DeployIdentityAgent,
	}}
	decision := Evaluate(record, "machine-id", "delly2")
	if !decision.Admit || !decision.BackfillID || decision.LegacyMigrate || decision.RepairDeployIdentity {
		t.Fatalf("current deploy token decision = %+v, want an ordinary identity backfill", decision)
	}
	if decision := Evaluate(record, "machine-id", "other-node"); decision.Admit {
		t.Fatalf("current deploy token admitted another host: %+v", decision)
	}
}

// A report may name the agent on a token bound only to its hostname, but only
// with the identity the command channel would bind for that same agent, and
// never by moving, repairing, or rehosting a binding.
func TestEvaluateReportedIdentityRecordsOnlyWhatCommandRegistrationWouldBind(t *testing.T) {
	deployToken := func() *config.APITokenRecord {
		return &config.APITokenRecord{OrgID: "default", Metadata: map[string]string{
			"bound_hostname":  "delly2",
			"deploy_job_id":   "dep_1",
			VersionKey:        Version,
			DeployIdentityKey: DeployIdentityAgent,
		}}
	}
	autoRegistered := func() *config.APITokenRecord {
		return &config.APITokenRecord{OrgID: "default", Metadata: map[string]string{
			"install_type":                     "pve",
			"issued_via":                       IssuedViaConfig,
			"bound_hostname":                   "pve1",
			agenttokens.RuntimeRoleMetadataKey: agenttokens.CredentialKindMonitoringCollector,
		}}
	}
	legacyHostnameOnly := func() *config.APITokenRecord {
		return &config.APITokenRecord{Metadata: map[string]string{"bound_hostname": "node.example"}}
	}

	for _, test := range []struct {
		name   string
		record *config.APITokenRecord
		host   string
	}{
		{name: "deploy runtime token", record: deployToken(), host: "delly2"},
		{name: "deploy runtime token on an equivalent hostname", record: deployToken(), host: "delly2.lan"},
		{name: "Proxmox auto-registered collector", record: autoRegistered(), host: "pve1"},
		{name: "pre-v6.1.1 hostname-only token", record: legacyHostnameOnly(), host: "NODE.example"},
	} {
		if !EvaluateReportedIdentity(test.record, "machine-id", "machine-id", test.host) {
			t.Fatalf("%s: report identity not recorded", test.name)
		}
		if EvaluateReportedIdentity(test.record, "configured-id", "machine-id", test.host) {
			t.Fatalf("%s: recorded the resolved identity although the agent presented another", test.name)
		}
		if EvaluateReportedIdentity(test.record, "", "", test.host) {
			t.Fatalf("%s: recorded an empty identity", test.name)
		}
	}

	with := func(record *config.APITokenRecord, mutate func(*config.APITokenRecord)) *config.APITokenRecord {
		mutate(record)
		return record
	}
	for _, test := range []struct {
		name   string
		record *config.APITokenRecord
		host   string
	}{
		{name: "another hostname", record: deployToken(), host: "other-node"},
		{name: "an already bound ID", host: "delly2", record: with(deployToken(), func(r *config.APITokenRecord) {
			r.Metadata["bound_agent_id"] = "other-machine"
		})},
		{name: "an ID already recorded", host: "delly2", record: with(deployToken(), func(r *config.APITokenRecord) {
			r.Metadata["bound_agent_id"] = "machine-id"
		})},
		// Evaluate would legacy-migrate this one on command registration; the
		// report path never moves an ID.
		{name: "a versionless token already bound to an ID", host: "node.example", record: with(legacyHostnameOnly(), func(r *config.APITokenRecord) {
			r.Metadata["bound_agent_id"] = "other-machine"
		})},
		{name: "a historical deploy placeholder", host: "delly2", record: with(deployToken(), func(r *config.APITokenRecord) {
			r.Metadata["bound_agent_id"] = "agent-delly2"
			delete(r.Metadata, DeployIdentityKey)
		})},
		{name: "a token bound to no hostname", host: "node.example", record: &config.APITokenRecord{OrgID: "default", Metadata: map[string]string{
			"install_type": "host",
			"issued_via":   IssuedViaConfig,
		}}},
		{name: "a token bound to several organizations", host: "delly2", record: with(deployToken(), func(r *config.APITokenRecord) {
			r.OrgID = ""
			r.OrgIDs = []string{"org-a", "org-b"}
		})},
		{name: "an action-runner credential", host: "delly2", record: with(deployToken(), func(r *config.APITokenRecord) {
			r.Metadata[agenttokens.RuntimeRoleMetadataKey] = agenttokens.CredentialKindActionRunner
		})},
		{name: "an unsupported runtime role", host: "delly2", record: with(deployToken(), func(r *config.APITokenRecord) {
			r.Metadata[agenttokens.RuntimeRoleMetadataKey] = "future-role"
		})},
	} {
		if EvaluateReportedIdentity(test.record, "machine-id", "machine-id", test.host) {
			t.Fatalf("%s: report identity recorded", test.name)
		}
	}
	if EvaluateReportedIdentity(nil, "machine-id", "machine-id", "delly2") {
		t.Fatal("nil token recorded an identity")
	}
}
