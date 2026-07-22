package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestBranchcov0722PMResolveRunCommandApprovalTarget drives every branch of
// resolveRunCommandApprovalTarget: runOnHost true/false, empty vs populated
// targetHost, each ExecuteRequest shape that changes the resolved type/ID/name,
// and the node/hostname/host_name context fallbacks (string vs non-string vs
// empty). All three return values are asserted for every case.
func TestBranchcov0722PMResolveRunCommandApprovalTarget(t *testing.T) {
	tests := []struct {
		name       string
		req        ExecuteRequest
		runOnHost  bool
		targetHost string
		wantType   string
		wantID     string
		wantName   string
	}{
		{
			name:       "runOnHost true with populated targetHost overrides type and binds normalized host",
			req:        ExecuteRequest{},
			runOnHost:  true,
			targetHost: "PVE-01",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "pve-01",
			wantName:   "pve-01",
		},
		{
			name:       "runOnHost true overrides explicit non-agent target type keeping the explicit id",
			req:        ExecuteRequest{TargetType: "vm", TargetID: "vm-100"},
			runOnHost:  true,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "vm-100",
			wantName:   "vm-100",
		},
		{
			name:       "runOnHost false normalizes and preserves non-agent target type, name falls back to id",
			req:        ExecuteRequest{TargetType: " VM ", TargetID: "vm-100"},
			runOnHost:  false,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeVM,
			wantID:     "vm-100",
			wantName:   "vm-100",
		},
		{
			name:       "runOnHost false empty type defaults to agent and resolves node context lowercased",
			req:        ExecuteRequest{Context: map[string]interface{}{"node": "Node-1"}},
			runOnHost:  false,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "node-1",
			wantName:   "node-1",
		},
		{
			name:       "hostname context fallback resolved lowercased when node absent",
			req:        ExecuteRequest{Context: map[string]interface{}{"hostname": "Host-A"}},
			runOnHost:  false,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "host-a",
			wantName:   "host-a",
		},
		{
			name:       "host_name context fallback resolved lowercased when node and hostname absent",
			req:        ExecuteRequest{Context: map[string]interface{}{"host_name": "Host-B"}},
			runOnHost:  false,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "host-b",
			wantName:   "host-b",
		},
		{
			name:       "populated targetHost supplies name via original-case trim without populating id when not runOnHost",
			req:        ExecuteRequest{},
			runOnHost:  false,
			targetHost: "  PVE-02  ",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "",
			wantName:   "PVE-02",
		},
		{
			name:       "non-string node context value is ignored leaving id empty",
			req:        ExecuteRequest{Context: map[string]interface{}{"node": 123}},
			runOnHost:  false,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "",
			wantName:   "",
		},
		{
			name:       "empty-string node context is ignored",
			req:        ExecuteRequest{Context: map[string]interface{}{"node": ""}},
			runOnHost:  false,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "",
			wantName:   "",
		},
		{
			name:       "runOnHost true with whitespace-only targetHost keeps explicit id and name falls back to id",
			req:        ExecuteRequest{TargetType: "system-container", TargetID: "ct-200"},
			runOnHost:  true,
			targetHost: "   ",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "ct-200",
			wantName:   "ct-200",
		},
		{
			name:       "node context takes priority over hostname and host_name",
			req:        ExecuteRequest{Context: map[string]interface{}{"node": "N1", "hostname": "H1", "host_name": "HN1"}},
			runOnHost:  false,
			targetHost: "",
			wantType:   agentcapabilities.ActionTargetTypeAgent,
			wantID:     "n1",
			wantName:   "n1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotID, gotName := resolveRunCommandApprovalTarget(tt.req, tt.runOnHost, tt.targetHost)
			if gotType != tt.wantType {
				t.Errorf("targetType = %q, want %q", gotType, tt.wantType)
			}
			if gotID != tt.wantID {
				t.Errorf("targetID = %q, want %q", gotID, tt.wantID)
			}
			if gotName != tt.wantName {
				t.Errorf("targetName = %q, want %q", gotName, tt.wantName)
			}
		})
	}
}

// withGlobalApprovalStore swaps the package-global approval store for the
// duration of a subtest and restores the previous value on cleanup. The store
// is process-global state, so every subtest must restore it to avoid leaking
// into siblings.
func withGlobalApprovalStore(t *testing.T, store *approval.Store) {
	t.Helper()
	previous := approval.GetStore()
	approval.SetStore(store)
	t.Cleanup(func() { approval.SetStore(previous) })
}

// TestBranchcov0722PMCreateRunCommandApprovalRecord covers the three return
// paths of createRunCommandApprovalRecord: nil store (""), CreateApproval error
// (""), and success (record ID). The success path asserts the deterministic
// persisted fields and that two distinct inputs never collide. The ID embeds a
// random UUID (see store.CreateApproval), so only shape/non-emptiness and
// non-collision are asserted, not the literal value.
func TestBranchcov0722PMCreateRunCommandApprovalRecord(t *testing.T) {
	t.Run("nil store returns empty string", func(t *testing.T) {
		withGlobalApprovalStore(t, nil)

		got := createRunCommandApprovalRecord("default", "uptime", "tool-1", ExecuteRequest{}, true, "pve-01", "reason")
		if got != "" {
			t.Fatalf("expected empty string when store is nil, got %q", got)
		}
	})

	t.Run("success persists deterministic record fields retrievable by id", func(t *testing.T) {
		store, err := approval.NewStore(approval.StoreConfig{
			DataDir:            t.TempDir(),
			DisablePersistence: true,
			DefaultTimeout:     1 * time.Minute,
			MaxApprovals:       100,
		})
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		withGlobalApprovalStore(t, store)

		id := createRunCommandApprovalRecord(
			"default",
			"pct exec 100 -- uptime",
			"tool-call-7",
			ExecuteRequest{},
			true,
			"pve-01",
			"investigating load",
		)
		if id == "" {
			t.Fatal("expected non-empty approval id on success")
		}

		rec, ok := store.GetApproval(id)
		if !ok {
			t.Fatalf("approval %q not retrievable from store", id)
		}
		if rec.Command != "pct exec 100 -- uptime" {
			t.Errorf("Command = %q, want %q", rec.Command, "pct exec 100 -- uptime")
		}
		if rec.ToolID != "tool-call-7" {
			t.Errorf("ToolID = %q, want %q", rec.ToolID, "tool-call-7")
		}
		if rec.OrgID != "default" {
			t.Errorf("OrgID = %q, want %q", rec.OrgID, "default")
		}
		// resolveRunCommandApprovalTarget with runOnHost=true, targetHost="pve-01"
		// yields type=agent, id=pve-01, name=pve-01.
		if rec.TargetType != agentcapabilities.ActionTargetTypeAgent {
			t.Errorf("TargetType = %q, want %q", rec.TargetType, agentcapabilities.ActionTargetTypeAgent)
		}
		if rec.TargetID != "pve-01" {
			t.Errorf("TargetID = %q, want %q", rec.TargetID, "pve-01")
		}
		if rec.TargetName != "pve-01" {
			t.Errorf("TargetName = %q, want %q", rec.TargetName, "pve-01")
		}
		if rec.Context != "investigating load" {
			t.Errorf("Context = %q, want %q", rec.Context, "investigating load")
		}
		if rec.Status != approval.StatusPending {
			t.Errorf("Status = %q, want %q", rec.Status, approval.StatusPending)
		}
	})

	t.Run("two distinct inputs produce non-colliding ids", func(t *testing.T) {
		store, err := approval.NewStore(approval.StoreConfig{
			DataDir:            t.TempDir(),
			DisablePersistence: true,
			MaxApprovals:       100,
		})
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		withGlobalApprovalStore(t, store)

		idA := createRunCommandApprovalRecord("default", "uptime", "tool-1", ExecuteRequest{}, false, "host-a", "ra")
		idB := createRunCommandApprovalRecord("default", "df -h", "tool-2", ExecuteRequest{}, false, "host-b", "rb")
		if idA == "" || idB == "" {
			t.Fatalf("expected both ids non-empty, got %q and %q", idA, idB)
		}
		if idA == idB {
			t.Fatalf("expected distinct ids for distinct inputs, both = %q", idA)
		}
	})

	t.Run("CreateApproval error returns empty string", func(t *testing.T) {
		store, err := approval.NewStore(approval.StoreConfig{
			DataDir:            t.TempDir(),
			DisablePersistence: true,
			MaxApprovals:       1,
		})
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		withGlobalApprovalStore(t, store)

		// Fill the single pending slot so the next creation hits the capacity
		// cap and CreateApproval returns an error.
		if err := store.CreateApproval(&approval.ApprovalRequest{
			Command:    "filler",
			TargetType: agentcapabilities.ActionTargetTypeAgent,
		}); err != nil {
			t.Fatalf("seed approval: %v", err)
		}

		got := createRunCommandApprovalRecord("default", "uptime", "tool-1", ExecuteRequest{}, true, "pve-01", "reason")
		if got != "" {
			t.Fatalf("expected empty string when CreateApproval fails, got %q", got)
		}
	})
}

// TestBranchcov0722PMMergeProviderCatalogFallbackModels covers the unknown
// provider (unchanged), de-duplication when the catalog already contains a
// fallback, empty catalogs, empty-id skipping, and case-insensitive dedup.
// Ordering and de-duplication are asserted explicitly.
func TestBranchcov0722PMMergeProviderCatalogFallbackModels(t *testing.T) {
	// deepseek is registered with four fallback models in this exact order.
	deepseekFallbackOrder := []string{
		config.DeepSeekModelV4Flash,
		config.DeepSeekModelV4Pro,
		config.DeepSeekModelLegacyChat,
		config.DeepSeekModelLegacyReasoner,
	}

	t.Run("unknown provider returns models unchanged", func(t *testing.T) {
		in := []providers.ModelInfo{{ID: "m1"}, {ID: "m2"}}
		got := mergeProviderCatalogFallbackModels("totally-unknown-prov", in)
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].ID != "m1" || got[1].ID != "m2" {
			t.Errorf("order changed: got %q, %q, want m1, m2", got[0].ID, got[1].ID)
		}
	})

	t.Run("unknown provider returns nil for nil input", func(t *testing.T) {
		got := mergeProviderCatalogFallbackModels("totally-unknown-prov", nil)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty catalog returns fallbacks only in defined order", func(t *testing.T) {
		got := mergeProviderCatalogFallbackModels(config.AIProviderDeepSeek, nil)
		if len(got) != len(deepseekFallbackOrder) {
			t.Fatalf("len = %d, want %d", len(got), len(deepseekFallbackOrder))
		}
		for i, want := range deepseekFallbackOrder {
			if got[i].ID != want {
				t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, want)
			}
		}
	})

	t.Run("catalog entries precede fallbacks with duplicate removed", func(t *testing.T) {
		in := []providers.ModelInfo{
			{ID: config.DeepSeekModelV4Flash},
			{ID: "extra-model"},
		}
		got := mergeProviderCatalogFallbackModels(config.AIProviderDeepSeek, in)

		wantOrder := []string{
			config.DeepSeekModelV4Flash,
			"extra-model",
			config.DeepSeekModelV4Pro,
			config.DeepSeekModelLegacyChat,
			config.DeepSeekModelLegacyReasoner,
		}
		if len(got) != len(wantOrder) {
			t.Fatalf("len = %d, want %d (%+v)", len(got), len(wantOrder), got)
		}
		for i, want := range wantOrder {
			if got[i].ID != want {
				t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, want)
			}
		}
		// Explicitly assert no duplicate of the shared fallback id.
		count := 0
		for _, m := range got {
			if m.ID == config.DeepSeekModelV4Flash {
				count++
			}
		}
		if count != 1 {
			t.Errorf("duplicate id %q appeared %d times, want 1", config.DeepSeekModelV4Flash, count)
		}
	})

	t.Run("empty-id catalog entries are skipped", func(t *testing.T) {
		in := []providers.ModelInfo{
			{ID: ""},
			{ID: "   "},
			{ID: config.DeepSeekModelV4Flash},
			{ID: "new-model"},
		}
		got := mergeProviderCatalogFallbackModels(config.AIProviderDeepSeek, in)

		wantOrder := []string{
			config.DeepSeekModelV4Flash,
			"new-model",
			config.DeepSeekModelV4Pro,
			config.DeepSeekModelLegacyChat,
			config.DeepSeekModelLegacyReasoner,
		}
		if len(got) != len(wantOrder) {
			t.Fatalf("len = %d, want %d (%+v)", len(got), len(wantOrder), got)
		}
		for i, want := range wantOrder {
			if got[i].ID != want {
				t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, want)
			}
		}
	})

	t.Run("dedup is case-insensitive keeping catalog casing", func(t *testing.T) {
		in := []providers.ModelInfo{{ID: "DeepSeek-V4-Flash"}}
		got := mergeProviderCatalogFallbackModels(config.AIProviderDeepSeek, in)

		if len(got) != len(deepseekFallbackOrder) {
			t.Fatalf("len = %d, want %d (dup not removed case-insensitively)", len(got), len(deepseekFallbackOrder))
		}
		if got[0].ID != "DeepSeek-V4-Flash" {
			t.Errorf("catalog entry casing not preserved: got[0].ID = %q, want %q", got[0].ID, "DeepSeek-V4-Flash")
		}
		// Remaining entries must be the non-duplicate fallbacks in order.
		wantTail := []string{
			config.DeepSeekModelV4Pro,
			config.DeepSeekModelLegacyChat,
			config.DeepSeekModelLegacyReasoner,
		}
		for i, want := range wantTail {
			if got[1+i].ID != want {
				t.Errorf("got[%d].ID = %q, want %q", 1+i, got[1+i].ID, want)
			}
		}
	})
}
