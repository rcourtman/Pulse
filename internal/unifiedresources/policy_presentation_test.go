package unifiedresources

import (
	"fmt"
	"testing"
)

func TestResourcePolicyRedactedTextPreservesWhitespace(t *testing.T) {
	resource := Resource{Name: "private-host", Policy: &ResourcePolicy{Routing: ResourceRoutingPolicy{Redact: []ResourceRedactionHint{ResourceRedactionHostname}}}}
	for _, value := range []string{" ", "\n\n", " Summary ", "\n private-host \n"} {
		want := value
		if value == "\n private-host \n" {
			want = "\n " + ResourcePolicyRedactedLabel + " \n"
		}
		if got := ResourcePolicyRedactedText(value, resource); got != want {
			t.Errorf("redact(%q) = %q, want %q", value, got, want)
		}
		if got := ResourcePolicyRedactedText(value, Resource{}); got != value {
			t.Errorf("without policy: %q != %q", got, value)
		}
	}
}

func TestResourcePolicyRedactedTextReplacesWholeIdentifiersOnly(t *testing.T) {
	localOnly := &ResourcePolicy{Routing: ResourceRoutingPolicy{Scope: ResourceRoutingScopeLocalOnly}}
	label := ResourcePolicyRedactedLabel
	for _, tc := range []struct {
		name     string
		resource Resource
		value    string
		want     string
	}{
		{
			name:     "longer identifiers keep a shared prefix",
			resource: Resource{Name: "agent", Policy: localOnly},
			value:    `agent-89b9f14dfbe851b7 runs agent_version 6`,
			want:     `agent-89b9f14dfbe851b7 runs agent_version 6`,
		},
		{
			name:     "longer identifiers keep a shared suffix while dotted hostnames redact",
			resource: Resource{Name: "production", Policy: localOnly},
			value:    `eu-production, pre.production.lan and production-1`,
			want:     `eu-production, pre.` + label + `.lan and production-1`,
		},
		{
			name:     "whole identifiers are replaced next to punctuation",
			resource: Resource{Name: "vault", Policy: localOnly},
			value:    `vault: {"host":"vault"} at https://vault:8200/ui on /srv/vault/data, then vault. --vault- (vault)`,
			want:     label + `: {"host":"` + label + `"} at https://` + label + `:8200/ui on /srv/` + label + `/data, then ` + label + `. --` + label + `- (` + label + `)`,
		},
		{
			name:     "addresses do not match inside longer addresses",
			resource: Resource{Name: "mail", Identity: ResourceIdentity{IPAddresses: []string{"10.0.0.5"}}, Policy: localOnly},
			value:    `10.0.0.5 10.0.0.50 110.0.0.5 10.0.0.5:25 10.0.0.5.`,
			want:     label + ` 10.0.0.50 110.0.0.5 ` + label + `:25 ` + label + `.`,
		},
		{
			name:     "identifiers that start with a delimiter match after any text",
			resource: Resource{Name: "tank", Storage: &StorageMeta{Path: "/mnt/pve/finance"}, Policy: localOnly},
			value:    `nfs1/mnt/pve/finance/images and /mnt/pve/finance2`,
			want:     `nfs1` + label + `/images and /mnt/pve/finance2`,
		},
		{
			name:     "cased non-ASCII letters continue an identifier",
			resource: Resource{Name: "büro", Policy: localOnly},
			value:    `büro, büros, altbüro`,
			want:     label + `, büros, altbüro`,
		},
		{
			name:     "scripts without word spacing match anywhere",
			resource: Resource{Name: "财务库", Policy: localOnly},
			value:    `请检查财务库的状态`,
			want:     `请检查` + label + `的状态`,
		},
		{
			name: "overlapping identifiers share one label",
			resource: Resource{
				Name:     "finance-db",
				Identity: ResourceIdentity{Hostnames: []string{"team-a/finance-db", "finance-db/data"}},
				Policy:   localOnly,
			},
			value: `team-a/finance-db/data is finance-db`,
			want:  label + ` is ` + label,
		},
		{
			name: "an inserted label is not matched again",
			resource: Resource{
				Name:     "redacted",
				Identity: ResourceIdentity{Hostnames: []string{"policy"}},
				Policy:   localOnly,
			},
			value: `policy redacted by policy`,
			want:  label + ` ` + label + ` by ` + label,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResourcePolicyRedactedText(tc.value, tc.resource); got != tc.want {
				t.Fatalf("ResourcePolicyRedactedText() = %q, want %q", got, tc.want)
			}
		})
	}

	withReference := ResourcePolicyRedactedTextWithReferences(
		"agent and agent-89b9f14dfbe851b7", Resource{Name: "unrelated", Policy: localOnly},
		ResourcePolicyReference("agent", ResourceRedactionAlias),
	)
	if want := label + " and agent-89b9f14dfbe851b7"; withReference != want {
		t.Fatalf("ResourcePolicyRedactedTextWithReferences() = %q, want %q", withReference, want)
	}
}

func TestResourcePolicyTextRedactor(t *testing.T) {
	localOnly := &ResourcePolicy{Routing: ResourceRoutingPolicy{Scope: ResourceRoutingScopeLocalOnly}}
	label := ResourcePolicyRedactedLabel
	secret := Resource{
		Name:     "agent-89b9f14dfbe851b7",
		Identity: ResourceIdentity{Hostnames: []string{"agent-89b9f14dfbe851b7", "team-a/agent-89b9f14dfbe851b7"}},
		Policy:   localOnly,
	}

	value := "agent-89b9f14dfbe851b7 has secret team-a/agent-89b9f14dfbe851b7"
	shared := map[string]struct{}{"agent-89b9f14dfbe851b7": {}}
	if got, want := NewResourcePolicyTextRedactor([]Resource{secret}, shared).RedactedText(value), "agent-89b9f14dfbe851b7 has secret "+label; got != want {
		t.Fatalf("RedactedText() with a shared ID = %q, want %q", got, want)
	}
	if got, want := NewResourcePolicyTextRedactor([]Resource{secret}, nil).RedactedText(value), label+" has secret "+label; got != want {
		t.Fatalf("RedactedText() = %q, want %q", got, want)
	}
	if got := NewResourcePolicyTextRedactor([]Resource{{Name: "agent-89b9f14dfbe851b7"}}, nil).RedactedText(value); got != value {
		t.Fatalf("RedactedText() without policy = %q, want %q", got, value)
	}
	var none *ResourcePolicyTextRedactor
	if got := none.RedactedText(" \n"); got != " \n" {
		t.Fatalf("nil redactor changed whitespace: %q", got)
	}

	// The label contains "redacted" and "policy". Resources with those names
	// must each replace their own word once, not the words of the labels that
	// the resources before them inserted.
	var resources []Resource
	for i := 0; i < 24; i++ {
		resources = append(resources, Resource{ID: fmt.Sprintf("pod-%d", i), Name: []string{"redacted", "policy"}[i%2], Policy: localOnly})
	}
	if got, want := NewResourcePolicyTextRedactor(resources, nil).RedactedText("policy redacted pod-7"), label+" "+label+" "+label; got != want {
		t.Fatalf("RedactedText() for many resources = %q, want %q", got, want)
	}

	// JSON escape sequences (\n, \t) and DNS domain suffixes (.lan) must not
	// shield a restricted hostname, while IP octets stay whole.
	payroll := Resource{
		Name:     "payroll",
		Identity: ResourceIdentity{Hostnames: []string{"payroll"}, IPAddresses: []string{"10.0.0.5"}},
		Policy:   localOnly,
	}
	redactor := NewResourcePolicyTextRedactor([]Resource{payroll}, nil)
	if got, want := redactor.RedactedText(`{"log":"line1\npayroll\tpayroll.lan db.payroll.lan 10.0.0.50 192.10.0.0.5 10.0.0.5"}`),
		`{"log":"line1\n`+label+`\t`+label+`.lan db.`+label+`.lan 10.0.0.50 192.10.0.0.5 `+label+`"}`; got != want {
		t.Fatalf("RedactedText() escapes/FQDN = %q, want %q", got, want)
	}
}
