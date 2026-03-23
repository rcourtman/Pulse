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

	context := buildUnifiedResourcePolicyContext(resources)

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
