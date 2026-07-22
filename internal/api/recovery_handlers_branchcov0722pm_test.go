package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// mkPosturesBranchcov0722PM builds n deterministic protection postures whose
// only meaningful field for pagination is SubjectResourceID ("p-0".."p-(n-1)").
func mkPosturesBranchcov0722PM(n int) []recovery.ProtectionPosture {
	out := make([]recovery.ProtectionPosture, n)
	for i := 0; i < n; i++ {
		out[i] = recovery.ProtectionPosture{
			SubjectResourceID: fmt.Sprintf("p-%d", i),
		}
	}
	return out
}

func TestBranchcov0722PMProtectionPostureStateRank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state recovery.ProtectionState
		want  int
	}{
		{"attention ranks first", recovery.ProtectionStateAttention, 0},
		{"unprotected ranks second", recovery.ProtectionStateUnprotected, 1},
		{"unknown ranks third", recovery.ProtectionStateUnknown, 2},
		{"protected falls through to default", recovery.ProtectionStateProtected, 3},
		{"empty state falls through to default", recovery.ProtectionState(""), 3},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := protectionPostureStateRank(tc.state); got != tc.want {
				t.Fatalf("protectionPostureStateRank(%q) = %d, want %d", tc.state, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMPaginateProtectionPostures(t *testing.T) {
	t.Parallel()

	t.Run("empty input returns empty", func(t *testing.T) {
		t.Parallel()
		got := paginateProtectionPostures(nil, 1, 10)
		if len(got) != 0 {
			t.Fatalf("expected empty slice, got %d items", len(got))
		}
	})

	t.Run("offset past the end returns empty", func(t *testing.T) {
		t.Parallel()
		got := paginateProtectionPostures(mkPosturesBranchcov0722PM(5), 10, 2)
		if len(got) != 0 {
			t.Fatalf("expected empty slice for page past end, got %d items", len(got))
		}
	})

	t.Run("partial last page returns remaining tail", func(t *testing.T) {
		t.Parallel()
		got := paginateProtectionPostures(mkPosturesBranchcov0722PM(5), 2, 3)
		if len(got) != 2 {
			t.Fatalf("expected 2 items (partial page), got %d", len(got))
		}
		if got[0].SubjectResourceID != "p-3" || got[1].SubjectResourceID != "p-4" {
			t.Fatalf("ids = %q,%q, want p-3,p-4", got[0].SubjectResourceID, got[1].SubjectResourceID)
		}
	})

	t.Run("limit<=0 normalized to default returns all", func(t *testing.T) {
		t.Parallel()
		got := paginateProtectionPostures(mkPosturesBranchcov0722PM(5), 1, 0)
		if len(got) != 5 {
			t.Fatalf("expected all 5 items with limit<=0, got %d", len(got))
		}
		if got[0].SubjectResourceID != "p-0" || got[4].SubjectResourceID != "p-4" {
			t.Fatalf("boundaries = %q..%q, want p-0..p-4", got[0].SubjectResourceID, got[4].SubjectResourceID)
		}
	})

	t.Run("negative page normalized to first page", func(t *testing.T) {
		t.Parallel()
		got := paginateProtectionPostures(mkPosturesBranchcov0722PM(5), -3, 2)
		if len(got) != 2 {
			t.Fatalf("expected first page of 2 items, got %d", len(got))
		}
		if got[0].SubjectResourceID != "p-0" || got[1].SubjectResourceID != "p-1" {
			t.Fatalf("ids = %q,%q, want p-0,p-1", got[0].SubjectResourceID, got[1].SubjectResourceID)
		}
	})

	t.Run("full first page returns exactly limit items", func(t *testing.T) {
		t.Parallel()
		got := paginateProtectionPostures(mkPosturesBranchcov0722PM(5), 1, 2)
		if len(got) != 2 {
			t.Fatalf("expected 2 items, got %d", len(got))
		}
		if got[0].SubjectResourceID != "p-0" || got[1].SubjectResourceID != "p-1" {
			t.Fatalf("ids = %q,%q, want p-0,p-1", got[0].SubjectResourceID, got[1].SubjectResourceID)
		}
	})
}

func TestBranchcov0722PMBuildSeriesFromPoints(t *testing.T) {
	t.Parallel()

	// A fixed two-day window so the result is fully deterministic (time.Now is
	// only consulted when From/To are absent, which we never do for non-empty
	// inputs).
	from := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)

	mkPoint := func(id string, mode recovery.Mode, completedAt time.Time) recovery.RecoveryPoint {
		ca := completedAt.UTC()
		return recovery.RecoveryPoint{ID: id, Mode: mode, CompletedAt: &ca}
	}

	t.Run("empty points returns empty slice", func(t *testing.T) {
		t.Parallel()
		got := buildSeriesFromPoints(nil, recovery.ListPointsOptions{}, 0)
		if len(got) != 0 {
			t.Fatalf("expected empty slice, got %#v", got)
		}
	})

	t.Run("zero tzOffset buckets by UTC day with mode counters", func(t *testing.T) {
		t.Parallel()
		points := []recovery.RecoveryPoint{
			mkPoint("snap", recovery.ModeSnapshot, time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)),
			mkPoint("rem", recovery.ModeRemote, time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)),
			mkPoint("loc", recovery.ModeLocal, time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC)),
		}
		got := buildSeriesFromPoints(points, recovery.ListPointsOptions{From: &from, To: &to}, 0)
		if len(got) != 2 {
			t.Fatalf("expected 2 day buckets, got %d: %#v", len(got), got)
		}
		day15 := got[0]
		if day15.Day != "2026-03-15" {
			t.Fatalf("first bucket Day = %q, want 2026-03-15", day15.Day)
		}
		if day15.Total != 3 || day15.Snapshot != 1 || day15.Remote != 1 || day15.Local != 1 {
			t.Fatalf("2026-03-15 counts = total=%d snap=%d remote=%d local=%d, want 3/1/1/1",
				day15.Total, day15.Snapshot, day15.Remote, day15.Local)
		}
		day16 := got[1]
		if day16.Day != "2026-03-16" || day16.Total != 0 {
			t.Fatalf("second bucket = %#v, want empty 2026-03-16", day16)
		}
	})

	t.Run("positive tzOffset shifts late-UTC point into next day", func(t *testing.T) {
		t.Parallel()
		// 23:30 UTC + 60 min offset => local 00:30 on 2026-03-16.
		points := []recovery.RecoveryPoint{
			mkPoint("late", recovery.ModeSnapshot, time.Date(2026, 3, 15, 23, 30, 0, 0, time.UTC)),
		}
		got := buildSeriesFromPoints(points, recovery.ListPointsOptions{From: &from, To: &to}, 60)
		if len(got) != 2 {
			t.Fatalf("expected 2 day buckets, got %d: %#v", len(got), got)
		}
		if got[0].Day != "2026-03-15" || got[0].Total != 0 {
			t.Fatalf("2026-03-15 should be empty, got %#v", got[0])
		}
		if got[1].Day != "2026-03-16" || got[1].Total != 1 || got[1].Snapshot != 1 {
			t.Fatalf("2026-03-16 should hold the shifted point, got %#v", got[1])
		}
	})

	t.Run("swapped From/After To window is normalised", func(t *testing.T) {
		t.Parallel()
		// From > To triggers the end.Before(start) swap branch.
		swappedFrom := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
		swappedTo := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
		points := []recovery.RecoveryPoint{
			mkPoint("snap", recovery.ModeSnapshot, time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)),
		}
		got := buildSeriesFromPoints(points, recovery.ListPointsOptions{From: &swappedFrom, To: &swappedTo}, 0)
		if len(got) != 2 {
			t.Fatalf("expected 2 day buckets after swap, got %d: %#v", len(got), got)
		}
		if got[0].Day != "2026-03-15" || got[0].Total != 1 {
			t.Fatalf("first bucket = %#v, want 2026-03-15 with the point", got[0])
		}
		if got[1].Day != "2026-03-16" || got[1].Total != 0 {
			t.Fatalf("second bucket = %#v, want empty 2026-03-16", got[1])
		}
	})

	t.Run("points with nil or zero CompletedAt are skipped", func(t *testing.T) {
		t.Parallel()
		zero := time.Time{}
		points := []recovery.RecoveryPoint{
			{ID: "nil-completed", Mode: recovery.ModeSnapshot},
			{ID: "zero-completed", Mode: recovery.ModeRemote, CompletedAt: &zero},
			mkPoint("real", recovery.ModeLocal, time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)),
		}
		got := buildSeriesFromPoints(points, recovery.ListPointsOptions{From: &from, To: &to}, 0)
		if len(got) != 2 {
			t.Fatalf("expected 2 day buckets, got %d", len(got))
		}
		if got[0].Total != 1 || got[0].Local != 1 {
			t.Fatalf("only the real point should be counted, got %#v", got[0])
		}
	})
}

func TestBranchcov0722PMBuildFacetsFromPoints(t *testing.T) {
	t.Parallel()

	t.Run("empty points yields zero-value facets", func(t *testing.T) {
		t.Parallel()
		facets := buildFacetsFromPoints(nil)
		if facets.HasSize || facets.HasVerification || facets.HasEntityID {
			t.Fatalf("flags = size=%v verify=%v entity=%v, want all false",
				facets.HasSize, facets.HasVerification, facets.HasEntityID)
		}
		if len(facets.Clusters) != 0 || len(facets.NodesHosts) != 0 ||
			len(facets.Namespaces) != 0 || len(facets.ItemTypes) != 0 {
			t.Fatalf("expected empty facet slices, got %+v", facets)
		}
	})

	t.Run("explicit Display fields are collected, deduped and sorted", func(t *testing.T) {
		t.Parallel()
		size := int64(1024)
		verified := false
		points := []recovery.RecoveryPoint{
			{
				ID: "a",
				Display: &recovery.RecoveryPointDisplay{
					ClusterLabel:   "beta",
					NodeHostLabel:  "node-2",
					NamespaceLabel: "ns-1",
					ItemType:       "pod",
					EntityIDLabel:  "uid-b",
				},
				SizeBytes: &size,
			},
			{
				ID: "b",
				Display: &recovery.RecoveryPointDisplay{
					ClusterLabel:   "alpha",
					NodeHostLabel:  "node-1",
					NamespaceLabel: "ns-2",
					ItemType:       "pvc",
				},
				Verified: &verified,
			},
		}
		facets := buildFacetsFromPoints(points)
		if got, want := facets.Clusters, []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Clusters = %#v, want %#v", got, want)
		}
		if got, want := facets.NodesHosts, []string{"node-1", "node-2"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NodesHosts = %#v, want %#v", got, want)
		}
		if got, want := facets.Namespaces, []string{"ns-1", "ns-2"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Namespaces = %#v, want %#v", got, want)
		}
		if got, want := facets.ItemTypes, []string{"pod", "pvc"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ItemTypes = %#v, want %#v", got, want)
		}
		if !facets.HasSize {
			t.Fatalf("HasSize = false, want true")
		}
		if !facets.HasVerification {
			t.Fatalf("HasVerification = false, want true")
		}
		if !facets.HasEntityID {
			t.Fatalf("HasEntityID = false, want true")
		}
	})

	t.Run("nil Display is derived from the point", func(t *testing.T) {
		t.Parallel()
		// DeriveIndex maps this k8s pvc point to cluster=prod-cluster,
		// namespace=default, itemType=pvc.
		points := []recovery.RecoveryPoint{
			{
				ID:       "k8s-1",
				Provider: recovery.ProviderKubernetes,
				SubjectRef: &recovery.ExternalRef{
					Type:      "k8s-pvc",
					Namespace: "default",
					Name:      "data",
				},
				Details: map[string]any{"k8sClusterName": "prod-cluster"},
			},
		}
		facets := buildFacetsFromPoints(points)
		if got, want := facets.Clusters, []string{"prod-cluster"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Clusters = %#v, want %#v", got, want)
		}
		if got, want := facets.Namespaces, []string{"default"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Namespaces = %#v, want %#v", got, want)
		}
		if got, want := facets.ItemTypes, []string{"pvc"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ItemTypes = %#v, want %#v", got, want)
		}
	})

	t.Run("zero SizeBytes and nil Verified set no flags", func(t *testing.T) {
		t.Parallel()
		zero := int64(0)
		points := []recovery.RecoveryPoint{
			{ID: "z", SizeBytes: &zero},
		}
		facets := buildFacetsFromPoints(points)
		if facets.HasSize {
			t.Fatalf("HasSize = true, want false for SizeBytes == 0")
		}
		if facets.HasVerification {
			t.Fatalf("HasVerification = true, want false for nil Verified")
		}
	})
}

func TestBranchcov0722PMParseRecoveryListPointsOptions(t *testing.T) {
	t.Parallel()

	t.Run("accepted defaults with empty query", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		opts, ok := parseRecoveryListPointsOptions(rec, url.Values{})
		if !ok {
			t.Fatalf("ok = false, want true; body=%s", rec.Body.String())
		}
		// Success path must not write a response.
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (defaults must not write)", rec.Code)
		}
		if opts.From != nil || opts.To != nil {
			t.Fatalf("From/To = %v/%v, want nil on empty query", opts.From, opts.To)
		}
		if opts.WorkloadOnly {
			t.Fatalf("WorkloadOnly = true, want false by default")
		}
		if opts.Kind != "" || opts.Provider != "" || opts.Verification != "" {
			t.Fatalf("Kind/Provider/Verification = %q/%q/%q, want empty",
				opts.Kind, opts.Provider, opts.Verification)
		}
	})

	t.Run("populated accepted values are parsed", func(t *testing.T) {
		t.Parallel()
		wantFrom := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		wantTo := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
		qs := url.Values{
			"from":         []string{"2026-03-01T00:00:00Z"},
			"to":           []string{"2026-03-31T00:00:00Z"},
			"kind":         []string{"snapshot"},
			"scope":        []string{"workload"},
			"platform":     []string{"kubernetes"},
			"verification": []string{"verified"},
		}
		rec := httptest.NewRecorder()
		opts, ok := parseRecoveryListPointsOptions(rec, qs)
		if !ok {
			t.Fatalf("ok = false, want true; body=%s", rec.Body.String())
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if opts.From == nil || !opts.From.Equal(wantFrom) {
			t.Fatalf("From = %v, want %v", opts.From, wantFrom)
		}
		if opts.To == nil || !opts.To.Equal(wantTo) {
			t.Fatalf("To = %v, want %v", opts.To, wantTo)
		}
		if opts.Kind != recovery.KindSnapshot {
			t.Fatalf("Kind = %q, want snapshot", opts.Kind)
		}
		if opts.Provider != recovery.ProviderKubernetes {
			t.Fatalf("Provider = %q, want kubernetes", opts.Provider)
		}
		if !opts.WorkloadOnly {
			t.Fatalf("WorkloadOnly = false, want true for scope=workload")
		}
		if opts.Verification != "verified" {
			t.Fatalf("Verification = %q, want verified", opts.Verification)
		}
	})

	t.Run("workloadOnly=true query flag also enables WorkloadOnly", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		opts, ok := parseRecoveryListPointsOptions(rec, url.Values{
			"workloadOnly": []string{"true"},
		})
		if !ok {
			t.Fatalf("ok = false, want true; body=%s", rec.Body.String())
		}
		if !opts.WorkloadOnly {
			t.Fatalf("WorkloadOnly = false, want true for workloadOnly=true")
		}
	})

	t.Run("rejection invalid_from writes 400 and returns false", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		opts, ok := parseRecoveryListPointsOptions(rec, url.Values{
			"from": []string{"not-a-time"},
		})
		if ok {
			t.Fatalf("ok = true, want false on invalid from")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if opts.From != nil || opts.To != nil {
			t.Fatalf("opts.From/To = %v/%v, want nil on rejection", opts.From, opts.To)
		}
		if !strings.Contains(rec.Body.String(), "invalid_from") {
			t.Fatalf("body = %q, want it to contain invalid_from", rec.Body.String())
		}
	})

	t.Run("rejection invalid_to writes 400 and returns false", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		opts, ok := parseRecoveryListPointsOptions(rec, url.Values{
			"from": []string{"2026-03-01T00:00:00Z"},
			"to":   []string{"also-not-a-time"},
		})
		if ok {
			t.Fatalf("ok = true, want false on invalid to")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if opts.To != nil {
			t.Fatalf("opts.To = %v, want nil on rejection", opts.To)
		}
		if !strings.Contains(rec.Body.String(), "invalid_to") {
			t.Fatalf("body = %q, want it to contain invalid_to", rec.Body.String())
		}
	})
}
