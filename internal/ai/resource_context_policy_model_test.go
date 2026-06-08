package ai

import (
	"reflect"
	"strings"
	"testing"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildUnifiedResourcePolicyContext(t *testing.T) {
	resources := unifiedresources.RefreshCanonicalMetadataSlice([]unifiedresources.Resource{
		{
			ID:   "public-1",
			Name: "public-vm",
			Type: unifiedresources.ResourceTypeVM,
			Tags: []string{"public"},
		},
		{
			ID:     "sensitive-1",
			Name:   "db-1",
			Type:   unifiedresources.ResourceTypeVM,
			Tags:   []string{"database"},
			Status: unifiedresources.StatusOnline,
			Identity: unifiedresources.ResourceIdentity{
				Hostnames:   []string{"db.internal"},
				IPAddresses: []string{"10.0.0.10"},
			},
			Canonical: &unifiedresources.CanonicalIdentity{
				PlatformID: "db.internal",
				Aliases:    []string{"db-1"},
			},
		},
		{
			ID:     "restricted-1",
			Name:   "mail-gw",
			Type:   unifiedresources.ResourceTypePMG,
			Status: unifiedresources.StatusWarning,
			PMG:    &unifiedresources.PMGData{Hostname: "mail.internal"},
		},
		{
			ID:     "storage-1",
			Name:   "backup-volume",
			Type:   unifiedresources.ResourceTypeStorage,
			Status: unifiedresources.StatusOnline,
			Storage: &unifiedresources.StorageMeta{
				Path: "/mnt/pve/backups",
			},
		},
	})

	context := buildUnifiedResourcePolicyContext(unifiedresources.SummarizePolicyPosture(resources), "", "redacted")

	if !context.hasGovernedResources() {
		t.Fatal("expected governed posture")
	}
	if context.localOnlyCount != 1 {
		t.Fatalf("localOnlyCount = %d, want 1", context.localOnlyCount)
	}

	wantHints := []unifiedresources.ResourceRedactionHint{
		unifiedresources.ResourceRedactionHostname,
		unifiedresources.ResourceRedactionIPAddress,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionAlias,
		unifiedresources.ResourceRedactionPath,
	}
	if !reflect.DeepEqual(context.redactionHints, wantHints) {
		t.Fatalf("redactionHints = %#v, want %#v", context.redactionHints, wantHints)
	}

	joined := strings.Join(context.appendSummarySections(nil), "\n")
	if !strings.Contains(joined, "### Data Governance") {
		t.Fatalf("expected data governance section, got %q", joined)
	}
	if !strings.Contains(joined, "Sensitivity: 1 Public, 0 Internal, 2 Sensitive, 1 Restricted") {
		t.Fatalf("expected canonical sensitivity summary, got %q", joined)
	}
	if !strings.Contains(joined, "Routing: 1 Cloud Summary, 2 Local First, 1 Local Only") {
		t.Fatalf("expected canonical routing summary, got %q", joined)
	}
	if count := strings.Count(joined, "### Policy Redaction Hints"); count != 1 {
		t.Fatalf("policy redaction hints count = %d, want 1 in %q", count, joined)
	}
	if !strings.Contains(joined, "Redactions in use: Hostname, IP Address, Platform ID, Alias, Path") {
		t.Fatalf("expected canonical redaction labels, got %q", joined)
	}
}

func TestBuildUnifiedResourcePolicyContextExternalModel(t *testing.T) {
	localOnly := unifiedresources.Resource{
		ID:     "mail-1",
		Name:   "customer-mail",
		Type:   unifiedresources.ResourceTypePMG,
		Status: unifiedresources.StatusWarning,
		PMG:    &unifiedresources.PMGData{Hostname: "mail.internal"},
	}
	cloudSummary := unifiedresources.Resource{
		ID:     "public-1",
		Name:   "public-node",
		Type:   unifiedresources.ResourceTypeAgent,
		Status: unifiedresources.StatusOnline,
		Tags:   []string{"public"},
	}
	resources := unifiedresources.RefreshCanonicalMetadataSlice([]unifiedresources.Resource{localOnly, cloudSummary})

	context := buildUnifiedResourcePolicyContext(unifiedresources.SummarizePolicyPosture(resources), "openai:gpt-4o", "redacted")

	if !context.externalModel {
		t.Fatal("expected external model handling")
	}
	if context.includeResourceDetails(resources[0]) {
		t.Fatal("expected local-only resource details to be omitted")
	}
	if !context.includeResourceDetails(resources[1]) {
		t.Fatal("expected cloud-summary resource details to remain available")
	}
	if filtered := context.filterDetailedResources(resources); len(filtered) != 1 || filtered[0].ID != "public-1" {
		t.Fatalf("filtered resources = %#v, want only public-1", filtered)
	}

	joined := strings.Join(context.appendSummarySections(nil), "\n")
	if !strings.Contains(joined, "External model handling: 1 local-only resources are represented only in aggregate and omitted from detailed context.") {
		t.Fatalf("expected external handling summary, got %q", joined)
	}

	localContext := buildUnifiedResourcePolicyContext(unifiedresources.SummarizePolicyPosture(resources), "ollama:llama3", "redacted")
	if localContext.externalModel {
		t.Fatal("expected ollama destination to stay local")
	}
	if !localContext.includeResourceDetails(resources[0]) {
		t.Fatal("expected local model context to include local-only resource details")
	}
}

func TestUnifiedResourcePolicyContext_ResourceLabelDialAware(t *testing.T) {
	resources := unifiedresources.RefreshCanonicalMetadataSlice([]unifiedresources.Resource{
		{ // Sensitive -> local-first (shown at full)
			ID: "vm-1", Name: "finance-vm", Type: unifiedresources.ResourceTypeVM,
			Status: unifiedresources.StatusOnline, Tags: []string{"sensitive"},
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"finance.lan"}},
		},
		{ // Restricted -> local-only (hard floor, redacted even at full)
			ID: "agent-1", Name: "vault", Type: unifiedresources.ResourceTypeAgent,
			Status: unifiedresources.StatusOnline, Tags: []string{"secret"},
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"vault.lan"}},
		},
		{ // Internal -> cloud-summary (real name everywhere)
			ID: "vm-2", Name: "web", Type: unifiedresources.ResourceTypeVM,
			Status: unifiedresources.StatusOnline,
		},
	})
	sens, restr, intern := resources[0], resources[1], resources[2]
	posture := unifiedresources.SummarizePolicyPosture(resources)
	label := func(ctx unifiedResourcePolicyContext, r unifiedresources.Resource) string {
		return ctx.resourceLabel(r.Name, r.AISafeSummary, r.Policy)
	}
	// The governed (non-real-name) rendering for a redacted resource — the AI-safe
	// summary when present, else the bare placeholder. The real-resource name must
	// never equal this.
	governed := func(r unifiedresources.Resource) string {
		return unifiedresources.ResourcePolicyLabel(r.Name, r.AISafeSummary, r.Policy)
	}
	if governed(sens) == "finance-vm" || governed(restr) == "vault" {
		t.Fatalf("test fixture invalid: governed labels should not be the real names (sens=%q restr=%q)", governed(sens), governed(restr))
	}

	// Local model: everything shows its real name (local is always full).
	local := buildUnifiedResourcePolicyContext(posture, "ollama:llama3", "redacted")
	if got := label(local, sens); got != "finance-vm" {
		t.Fatalf("local sensitive label = %q, want real name", got)
	}
	if got := label(local, restr); got != "vault" {
		t.Fatalf("local restricted label = %q, want real name", got)
	}

	// Cloud + full: sensitive shows its real name, restricted stays governed
	// (local-only hard floor), internal shows.
	full := buildUnifiedResourcePolicyContext(posture, "openai:gpt-4o", "full")
	if got := label(full, sens); got != "finance-vm" {
		t.Fatalf("cloud-full sensitive label = %q, want real name", got)
	}
	if got := label(full, restr); got != governed(restr) {
		t.Fatalf("cloud-full restricted label = %q, want governed floor %q", got, governed(restr))
	}
	if got := label(full, intern); got != "web" {
		t.Fatalf("cloud-full internal label = %q, want real name", got)
	}

	// Cloud + redacted: both governed resources are redacted; internal still shows.
	red := buildUnifiedResourcePolicyContext(posture, "openai:gpt-4o", "redacted")
	if got := label(red, sens); got != governed(sens) {
		t.Fatalf("cloud-redacted sensitive label = %q, want governed %q", got, governed(sens))
	}
	if got := label(red, restr); got != governed(restr) {
		t.Fatalf("cloud-redacted restricted label = %q, want governed %q", got, governed(restr))
	}
	if got := label(red, intern); got != "web" {
		t.Fatalf("cloud-redacted internal label = %q, want real name", got)
	}
}
