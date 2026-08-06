package vmware

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func newTagTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{
		Host:               serverURL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestClientCollectInventoryAttachesVCenterTags(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{})
	defer server.Close()

	client := newTagTestClient(t, server.URL)
	snapshot, err := client.CollectInventory(context.Background())
	if err != nil {
		t.Fatalf("CollectInventory: %v", err)
	}
	// The one tag whose metadata cannot be read is reported and skipped; it
	// must not blank out the tags that did resolve.
	var tagIssues []InventoryEnrichmentIssue
	for _, issue := range snapshot.EnrichmentIssues {
		if issue.Stage == "tags" {
			tagIssues = append(tagIssues, issue)
		}
	}
	if len(tagIssues) != 1 || tagIssues[0].EntityID != "urn:tag:orphan" || tagIssues[0].Category != "not_found" {
		t.Fatalf("expected one not_found issue for the unresolvable tag, got %+v", tagIssues)
	}

	host := snapshot.Hosts[0]
	if len(host.Tags) != 1 {
		t.Fatalf("expected one host tag, got %+v", host.Tags)
	}
	if host.Tags[0].Name != "R1" || host.Tags[0].Category != "Rack" {
		t.Fatalf("unexpected host tag: %+v", host.Tags[0])
	}

	vm := snapshot.VMs[0]
	// The server answers with the tags out of order and includes one id it
	// will not resolve; the client sorts by category and drops the orphan.
	if len(vm.Tags) != 2 {
		t.Fatalf("expected two resolvable VM tags, got %+v", vm.Tags)
	}
	if vm.Tags[0].Category != "Environment" || vm.Tags[0].Name != "Production" {
		t.Fatalf("unexpected first VM tag: %+v", vm.Tags[0])
	}
	if vm.Tags[1].Category != "Owner" || vm.Tags[1].Name != "Platform" {
		t.Fatalf("unexpected second VM tag: %+v", vm.Tags[1])
	}
}

func TestClientTagCatalogReusesResolvedNamesAcrossCollections(t *testing.T) {
	counts := map[string]int{}
	var mu sync.Mutex
	server := newVMwareTestServer(t, vmwareTestServerConfig{
		tagRequestCounts:   counts,
		tagRequestCountsMu: &mu,
	})
	defer server.Close()

	client := newTagTestClient(t, server.URL)
	for i := 0; i < 3; i++ {
		if _, err := client.CollectInventory(context.Background()); err != nil {
			t.Fatalf("CollectInventory %d: %v", i, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	// Associations follow the estate, so they are re-read every refresh.
	if counts["tag-association"] != 3 {
		t.Fatalf("expected 3 association reads, got %d", counts["tag-association"])
	}
	// Tag and category names are estate-wide and stable, so they resolve once
	// per TTL rather than once per refresh.
	for _, tagID := range []string{"urn:tag:env-production", "urn:tag:owner-platform", "urn:tag:rack-r1"} {
		if got := counts["tag:"+tagID]; got != 1 {
			t.Fatalf("expected 1 metadata read for %s, got %d", tagID, got)
		}
	}
	for _, categoryID := range []string{"urn:category:environment", "urn:category:owner", "urn:category:rack"} {
		if got := counts["category:"+categoryID]; got != 1 {
			t.Fatalf("expected 1 metadata read for %s, got %d", categoryID, got)
		}
	}
	// The unresolvable id is retried because it never enters the catalog, but
	// it must not be cached as a real tag either.
	if got := counts["tag:urn:tag:orphan"]; got != 3 {
		t.Fatalf("expected the unresolvable tag to be retried each refresh, got %d", got)
	}
}

func TestEnrichInventoryTagsSkipsCollectionWithoutObjects(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{})
	defer server.Close()

	client := newTagTestClient(t, server.URL)
	issues, err := client.enrichInventoryTags(context.Background(), "automation-session", &InventorySnapshot{})
	if err != nil {
		t.Fatalf("enrichInventoryTags: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues for an empty snapshot, got %+v", issues)
	}
	if issues, err := client.enrichInventoryTags(context.Background(), "automation-session", nil); err != nil || issues != nil {
		t.Fatalf("expected a nil snapshot to be a no-op, got issues=%+v err=%v", issues, err)
	}
}

func TestInventoryTagLabel(t *testing.T) {
	cases := []struct {
		name string
		tag  InventoryTag
		want string
	}{
		{"category and name", InventoryTag{Category: "Environment", Name: "Production"}, "Environment:Production"},
		{"name only", InventoryTag{Name: "Production"}, "Production"},
		{"blank name is not a label", InventoryTag{Category: "Environment", Name: "  "}, ""},
		{"whitespace is trimmed", InventoryTag{Category: " Owner ", Name: " Platform "}, "Owner:Platform"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := InventoryTagLabel(tc.tag); got != tc.want {
				t.Fatalf("InventoryTagLabel(%+v) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestFixtureRecordsCarryVCenterTagsOnBothSurfaces(t *testing.T) {
	records := FixtureRecords(DefaultFixtures())

	var vmRecord *unifiedresources.IngestRecord
	var hostRecord *unifiedresources.IngestRecord
	labelSets := make(map[string]struct{})
	for i := range records {
		resource := records[i].Resource
		if resource.VMware == nil {
			continue
		}
		switch resource.VMware.EntityType {
		case "vm":
			labelSets[strings.Join(vmwareTagLabels(resource.VMware.Tags), "|")] = struct{}{}
			if resource.Name == "postgres-ha-01" {
				vmRecord = &records[i]
			}
		case "host":
			if resource.Name == "esxi-01.lab.local" {
				hostRecord = &records[i]
			}
		}
	}

	if vmRecord == nil || hostRecord == nil {
		t.Fatal("expected the default fixtures to contain postgres-ha-01 and esxi-01.lab.local")
	}
	// The whole point of reading real tags is that rows stop looking alike.
	// One label set across the fixture estate would mean the Tags column is
	// still decoration.
	if len(labelSets) < 5 {
		t.Fatalf("expected the fixture VMs to carry varied tag sets, got %d distinct sets", len(labelSets))
	}

	wantVMLabels := []string{"Backup:Nightly", "Compliance:PCI", "Environment:Production", "Owner:Data"}
	if got := vmwareTagLabels(vmRecord.Resource.VMware.Tags); strings.Join(got, "|") != strings.Join(wantVMLabels, "|") {
		t.Fatalf("postgres-ha-01 VMware.Tags = %v, want %v", got, wantVMLabels)
	}
	wantHostLabels := []string{"Environment:Production", "Rack:R1"}
	if got := vmwareTagLabels(hostRecord.Resource.VMware.Tags); strings.Join(got, "|") != strings.Join(wantHostLabels, "|") {
		t.Fatalf("esxi-01 VMware.Tags = %v, want %v", got, wantHostLabels)
	}

	// The flat keyword set keeps provenance so resource search, the `?tags=`
	// filter, and saved report schedules keep matching, and gains the real
	// labels alongside it.
	flat := vmRecord.Resource.Tags
	for _, want := range []string{"vmware", "vsphere", "vm", "source:vcenter", "Environment:Production", "Compliance:PCI"} {
		if !containsString(flat, want) {
			t.Fatalf("expected flat resource tags to contain %q, got %v", want, flat)
		}
	}
}

func vmwareTagLabels(tags []unifiedresources.VMwareTagData) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, tag.Label)
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func TestVMwareResourceTagsKeepsProvenanceAndAppendsRealTags(t *testing.T) {
	got := vmwareResourceTags(
		[]string{"vmware", "vsphere", "vm", "source:vcenter", "connection:lab vcenter", "power:powered_on"},
		[]InventoryTag{
			{Category: "Environment", Name: "Production"},
			{Name: "Uncategorized"},
			{Name: "   "},
		},
	)
	want := []string{
		"vmware",
		"vsphere",
		"vm",
		"source:vcenter",
		"connection:lab vcenter",
		"power:powered_on",
		"Environment:Production",
		"Uncategorized",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("vmwareResourceTags = %v, want %v", got, want)
	}
}
