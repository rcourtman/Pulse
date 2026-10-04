package store

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// Read the SQL used by the real reader, not a duplicate that can drift from it.
func latestProtectionObservationSQL(t testing.TB) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "store_posture.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var query string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "listLatestProtectionProviderObservations" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "QueryContext" {
				return true
			}
			literal, ok := call.Args[1].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			query, err = strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			return false
		})
	}
	if query == "" {
		t.Fatal("could not locate the production observation query")
	}
	return query
}

func observationHistoryStore(t testing.TB, rows int) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	now := time.Now().UTC().Truncate(time.Millisecond)
	latest := []recovery.ProtectionProviderObservation{
		observationForHistoryTest(t, recovery.ProviderProxmoxPVE, "pve-main", now),
		observationForHistoryTest(t, recovery.ProviderProxmoxPBS, "pbs-main", now),
	}
	if err := s.UpsertProtectionProviderObservations(context.Background(), latest); err != nil {
		t.Fatal(err)
	}
	// Synthetic retained rows are deliberately irrelevant to the latest facts.
	// Their 1 KiB payload models the cost of reading wide historical evidence;
	// no reporter database or driver instrumentation is involved.
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO protection_provider_observations (
  id, provider, source, scope, job_state, history_completeness, permissions,
  verification_expected, observed_at_ms, ingested_at_ms, evidence_json,
  created_at_ms, updated_at_ms
 ) VALUES (?, ?, 'history-fixture', ?, 'success', 'complete', 'sufficient',
  0, ?, ?, ?, ?, ?)`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	raw := strings.Repeat(" ", 1024) + "{}"
	for i := 0; i < rows-2; i++ {
		seed := latest[i%2]
		at := now.UnixMilli() - int64(i+1)*1000
		if _, err := stmt.Exec(fmt.Sprintf("history-%09d", i), string(seed.Provider), seed.Scope, at, at, raw, at, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return s
}

func observationForHistoryTest(t testing.TB, provider recovery.Provider, scope string, at time.Time) recovery.ProtectionProviderObservation {
	t.Helper()
	o, err := recovery.NewProtectionProviderObservation(provider, "history-fixture", scope,
		recovery.OutcomeSuccess, recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient, false, at, at, nil)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// Fixed comparisons selected before execution: small history, the reporter's
// row count, and a larger retained history. Timings are evidence, not CI gates.
func BenchmarkLatestProtectionObservationHistory(b *testing.B) {
	for _, n := range []int{1000, 70949, 180000} {
		b.Run(fmt.Sprintf("rows-%d", n), func(b *testing.B) {
			s := observationHistoryStore(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				obs, err := s.listLatestProtectionProviderObservations(context.Background())
				if err != nil || len(obs) != 2 {
					b.Fatalf("latest observations=%d error=%v", len(obs), err)
				}
			}
		})
	}
}
