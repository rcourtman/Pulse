package config

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Keep the previous predicate as a decision/allocation reference, not an
// alternative production authorization path. Its normalization mutates record.
func priorAPITokenHasScope(record *APITokenRecord, scope string) bool {
	if scope == "" {
		return true
	}
	scope = strings.TrimSpace(scope)
	record.ensureScopes()
	for _, candidate := range record.Scopes {
		if candidate == ScopeWildcard || candidate == scope {
			return true
		}
	}
	return false
}

func TestAPITokenScopeReadPreservesDecisions(t *testing.T) {
	atoms := []string{
		"", " ", ScopeWildcard, " * ", ScopeMonitoringRead, ScopeSettingsWrite,
		ScopeAgentReport, ScopeAgentConfigRead, ScopeAgentManage, ScopeAgentEnroll,
		"host-agent:report", "host-agent:config:read", "host-agent:manage", "host-agent:enroll",
		"\t" + ScopeMonitoringRead + "\n", "\u2003host-agent:report\u2003", "unknown:scope", "AGENT:REPORT",
	}
	sets := [][]string{nil, {}}
	for _, first := range atoms {
		sets = append(sets, []string{first})
		for _, second := range atoms {
			// Includes duplicate grants, aliases, unknowns and mixed wildcard sets.
			sets = append(sets, []string{first, second})
		}
	}
	checks := 0
	for _, stored := range sets {
		for _, requested := range atoms {
			prior := APITokenRecord{Scopes: append([]string(nil), stored...)}
			current := APITokenRecord{Scopes: stored}
			want := priorAPITokenHasScope(&prior, requested)
			if got := current.HasScope(requested); got != want {
				t.Fatalf("stored=%q requested=%q: got %t, prior %t", stored, requested, got, want)
			}
			checks++
		}
	}
	t.Logf("preserved %d prior scope decisions", checks)
}

func TestAPITokenScopeReadLeavesRecordUntouched(t *testing.T) {
	for name, scopes := range map[string][]string{
		"absent": nil, "empty": {}, "canonical": {ScopeMonitoringRead},
		"legacy":     {"host-agent:report", " host-agent:config:read "},
		"duplicates": {ScopeMonitoringRead, ScopeMonitoringRead},
		"wildcard":   {" * ", "unknown:scope"},
	} {
		t.Run(name, func(t *testing.T) {
			now := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
			record := APITokenRecord{
				ID: "synthetic-scope-reader", Name: "unchanged", Hash: "synthetic-hash",
				CreatedAt: now, LastUsedAt: &now, ExpiresAt: &now,
				OrgID: "client-a", OrgIDs: []string{"client-a", "client-b"},
				Scopes: scopes, Metadata: map[string]string{"owner_user_id": "synthetic-owner"},
			}
			before := record
			// Clone intentionally canonicalizes, so take an exact slice copy instead.
			if scopes != nil {
				before.Scopes = append([]string{}, scopes...)
			}
			var backing *string
			if len(scopes) > 0 {
				backing = &scopes[0]
			}
			for _, requested := range []string{"", ScopeMonitoringRead, ScopeAgentReport, ScopeAgentConfigRead, ScopeSettingsWrite} {
				record.HasScope(requested)
				if !reflect.DeepEqual(record, before) || (backing != nil && &record.Scopes[0] != backing) {
					t.Fatalf("scope read changed record or replaced its backing slice for %q", requested)
				}
			}
		})
	}
}

func TestAPITokenScopeReadConcurrentContextReaders(t *testing.T) {
	for name, scopes := range map[string][]string{
		"canonical": {ScopeAgentReport, ScopeMonitoringRead},
		"legacy":    {"host-agent:report", " monitoring:read ", "host-agent:report"},
		"default":   nil,
	} {
		t.Run(name, func(t *testing.T) {
			record := &APITokenRecord{Scopes: scopes}
			ctx := auth.WithAPIToken(context.Background(), record)
			start := make(chan struct{})
			var readers sync.WaitGroup
			for range 16 {
				readers.Go(func() {
					<-start
					token := auth.GetAPIToken(ctx)
					for range 1000 {
						if !token.HasScope(ScopeAgentReport) || !token.HasScope(ScopeMonitoringRead) ||
							token.HasScope(ScopeSettingsWrite) != (name == "default") {
							t.Error("shared context scope decision changed")
							return
						}
					}
				})
			}
			close(start)
			readers.Wait()
		})
	}
}

func TestAPITokenScopeReadDoesNotAllocate(t *testing.T) {
	for name, scopes := range map[string][]string{
		"default": nil, "canonical": {ScopeAgentReport, ScopeMonitoringRead},
		"legacy": {"host-agent:report", "host-agent:config:read"},
	} {
		t.Run(name, func(t *testing.T) {
			record := &APITokenRecord{Scopes: scopes}
			if allocations := testing.AllocsPerRun(100, func() {
				if !record.HasScope(ScopeAgentReport) {
					panic("expected scope was denied")
				}
			}); allocations != 0 {
				t.Fatalf("scope read allocated %g times, want 0", allocations)
			}
		})
	}
}

func BenchmarkAPITokenScopeRead(b *testing.B) {
	for _, tc := range []struct {
		name   string
		scopes []string
		want   bool
	}{
		{"default", nil, true},
		{"allowed", []string{ScopeAgentReport, ScopeMonitoringRead}, true},
		{"denied", []string{ScopeMonitoringRead, ScopeSettingsRead, ScopeAuditRead}, false},
		{"legacy", []string{"host-agent:report", "host-agent:config:read"}, true},
	} {
		for _, prior := range []bool{true, false} {
			implementation := "read-only"
			if prior {
				implementation = "prior"
			}
			b.Run(tc.name+"/"+implementation, func(b *testing.B) {
				record := &APITokenRecord{Scopes: append([]string(nil), tc.scopes...)}
				predicate := record.HasScope
				if prior {
					predicate = func(scope string) bool { return priorAPITokenHasScope(record, scope) }
				}
				b.ReportAllocs()
				for b.Loop() {
					if predicate(ScopeAgentReport) != tc.want {
						b.Fatal("scope decision changed")
					}
				}
			})
		}
	}
}
