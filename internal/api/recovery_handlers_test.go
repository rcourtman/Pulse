package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	_ "modernc.org/sqlite"
)

func TestParseRecoveryPlatformQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		qs   url.Values
		want recovery.Provider
	}{
		{
			name: "prefers canonical platform query",
			qs: url.Values{
				"platform": []string{" truenas "},
				"provider": []string{"proxmox-pve"},
			},
			want: recovery.Provider("truenas"),
		},
		{
			name: "falls back to legacy provider query",
			qs: url.Values{
				"provider": []string{" proxmox-pbs "},
			},
			want: recovery.Provider("proxmox-pbs"),
		},
		{
			name: "returns empty when neither is present",
			qs:   url.Values{},
			want: recovery.Provider(""),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseRecoveryPlatformQuery(tc.qs); got != tc.want {
				t.Fatalf("parseRecoveryPlatformQuery() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseRecoveryItemResourceIDQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		qs   url.Values
		want string
	}{
		{
			name: "prefers canonical item resource query",
			qs: url.Values{
				"itemResourceId":    []string{" vm-123 "},
				"subjectResourceId": []string{"legacy-vm"},
			},
			want: "vm-123",
		},
		{
			name: "falls back to legacy subject resource query",
			qs: url.Values{
				"subjectResourceId": []string{" vm-404 "},
			},
			want: "vm-404",
		},
		{
			name: "returns empty when neither is present",
			qs:   url.Values{},
			want: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseRecoveryItemResourceIDQuery(tc.qs); got != tc.want {
				t.Fatalf("parseRecoveryItemResourceIDQuery() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHandleListPointsAcceptsCanonicalPlatformQuery(t *testing.T) {
	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() {
		mock.SetEnabled(prevMock)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/recovery/points?platform=truenas&limit=500", nil)
	rec := httptest.NewRecorder()

	NewRecoveryHandlers(nil).HandleListPoints(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleListPoints() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []struct {
			Platform string `json:"platform"`
			Provider string `json:"provider"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected recovery points for platform=truenas, got none")
	}
	for _, point := range resp.Data {
		if point.Platform != "truenas" {
			t.Fatalf("expected only truenas recovery points, got platform %q", point.Platform)
		}
		if point.Provider != "truenas" {
			t.Fatalf("expected compatibility provider field to remain aligned, got %q", point.Provider)
		}
	}
}

func TestBuildRecoveryPointPayloadExposesCanonicalItemResourceIDField(t *testing.T) {
	payload := buildRecoveryPointPayload(recovery.RecoveryPoint{
		ID:                "point-1",
		Provider:          recovery.Provider("truenas"),
		Kind:              recovery.Kind("snapshot"),
		Mode:              recovery.Mode("snapshot"),
		Outcome:           recovery.Outcome("success"),
		SubjectResourceID: "vm-123",
	})

	if payload.ItemResourceID != "vm-123" {
		t.Fatalf("payload.ItemResourceID = %q, want %q", payload.ItemResourceID, "vm-123")
	}
	if payload.SubjectResourceID != "vm-123" {
		t.Fatalf("payload.SubjectResourceID = %q, want %q", payload.SubjectResourceID, "vm-123")
	}
}

func TestBuildRecoveryPointPayloadExposesCanonicalItemRefField(t *testing.T) {
	payload := buildRecoveryPointPayload(recovery.RecoveryPoint{
		ID:       "point-1",
		Provider: recovery.Provider("truenas"),
		Kind:     recovery.Kind("snapshot"),
		Mode:     recovery.Mode("snapshot"),
		Outcome:  recovery.Outcome("success"),
		SubjectRef: &recovery.ExternalRef{
			Type: "truenas-dataset",
			Name: "tank/apps",
		},
	})

	if payload.ItemRef == nil || payload.ItemRef.Name != "tank/apps" {
		t.Fatalf("payload.ItemRef = %#v, want canonical item ref", payload.ItemRef)
	}
	if payload.SubjectRef == nil || payload.SubjectRef.Name != "tank/apps" {
		t.Fatalf("payload.SubjectRef = %#v, want compatibility subject ref", payload.SubjectRef)
	}
}

func TestHandleListRollupsExposeCanonicalPlatformsPayload(t *testing.T) {
	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() {
		mock.SetEnabled(prevMock)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/recovery/rollups?platform=truenas&limit=500", nil)
	rec := httptest.NewRecorder()

	NewRecoveryHandlers(nil).HandleListRollups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleListRollups() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []struct {
			Platforms []string `json:"platforms"`
			Providers []string `json:"providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected recovery rollups for platform=truenas, got none")
	}
	for _, rollup := range resp.Data {
		if len(rollup.Platforms) == 0 || rollup.Platforms[0] != "truenas" {
			t.Fatalf("expected canonical platforms payload to include truenas, got %#v", rollup.Platforms)
		}
		if len(rollup.Providers) == 0 || rollup.Providers[0] != "truenas" {
			t.Fatalf("expected compatibility providers payload to remain aligned, got %#v", rollup.Providers)
		}
	}
}

func TestBuildRecoveryRollupPayloadExposesCanonicalItemResourceIDField(t *testing.T) {
	payload := buildRecoveryRollupPayload(recovery.ProtectionRollup{
		RollupID:          "res:vm-123",
		SubjectResourceID: "vm-123",
		LastOutcome:       recovery.Outcome("success"),
	})

	if payload.ItemResourceID != "vm-123" {
		t.Fatalf("payload.ItemResourceID = %q, want %q", payload.ItemResourceID, "vm-123")
	}
	if payload.SubjectResourceID != "vm-123" {
		t.Fatalf("payload.SubjectResourceID = %q, want %q", payload.SubjectResourceID, "vm-123")
	}
}

func TestBuildRecoveryRollupPayloadExposesCanonicalItemRefField(t *testing.T) {
	payload := buildRecoveryRollupPayload(recovery.ProtectionRollup{
		RollupID: "ext:tank-apps",
		SubjectRef: &recovery.ExternalRef{
			Type: "truenas-dataset",
			Name: "tank/apps",
		},
		LastOutcome: recovery.Outcome("success"),
	})

	if payload.ItemRef == nil || payload.ItemRef.Name != "tank/apps" {
		t.Fatalf("payload.ItemRef = %#v, want canonical item ref", payload.ItemRef)
	}
	if payload.SubjectRef == nil || payload.SubjectRef.Name != "tank/apps" {
		t.Fatalf("payload.SubjectRef = %#v, want compatibility subject ref", payload.SubjectRef)
	}
}

func TestHandleListPointsToleratesMalformedPersistedMetadata(t *testing.T) {
	t.Parallel()

	handler, dbPath := newRecoveryHandlerWithPersistedPoint(t, recovery.RecoveryPoint{
		ID:          "point-bad-json",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtr(time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtr(time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)),
		SubjectRef: &recovery.ExternalRef{
			Type:      "k8s-pvc",
			Namespace: "default",
			Name:      "data",
		},
		RepositoryRef: &recovery.ExternalRef{
			Type: "velero-backup-storage-location",
			Name: "repo-a",
		},
		Details: map[string]any{"foo": "bar"},
	})

	corruptRecoveryRowJSON(t, dbPath, "point-bad-json", true, true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/recovery/points?limit=500", nil)
	rec := httptest.NewRecorder()

	handler.HandleListPoints(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleListPoints() status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			ID            string         `json:"id"`
			ItemRef       map[string]any `json:"itemRef"`
			SubjectRef    map[string]any `json:"subjectRef"`
			RepositoryRef map[string]any `json:"repositoryRef"`
			Details       map[string]any `json:"details"`
			Display       struct {
				SubjectLabel string `json:"subjectLabel"`
				ItemType     string `json:"itemType"`
			} `json:"display"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected exactly 1 point, got %d", len(resp.Data))
	}
	point := resp.Data[0]
	if point.ID != "point-bad-json" {
		t.Fatalf("point.ID = %q, want point-bad-json", point.ID)
	}
	if point.ItemRef != nil || point.SubjectRef != nil || point.RepositoryRef != nil || point.Details != nil {
		t.Fatalf("expected malformed metadata fields to be omitted, got itemRef=%#v subjectRef=%#v repositoryRef=%#v details=%#v", point.ItemRef, point.SubjectRef, point.RepositoryRef, point.Details)
	}
	if point.Display.SubjectLabel != "default/data" {
		t.Fatalf("display.subjectLabel = %q, want %q", point.Display.SubjectLabel, "default/data")
	}
	if point.Display.ItemType != "pvc" {
		t.Fatalf("display.itemType = %q, want %q", point.Display.ItemType, "pvc")
	}
}

func TestHandleListRollupsToleratesMalformedPersistedMetadata(t *testing.T) {
	t.Parallel()

	handler, dbPath := newRecoveryHandlerWithPersistedPoint(t, recovery.RecoveryPoint{
		ID:          "rollup-bad-json",
		Provider:    recovery.ProviderTrueNAS,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtr(time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtr(time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)),
		SubjectRef: &recovery.ExternalRef{
			Type: "truenas-dataset",
			Name: "tank/apps",
			ID:   "tank/apps",
		},
	})

	corruptRecoveryRowJSON(t, dbPath, "rollup-bad-json", true, false, false)

	req := httptest.NewRequest(http.MethodGet, "/api/recovery/rollups?limit=500", nil)
	rec := httptest.NewRecorder()

	handler.HandleListRollups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleListRollups() status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			RollupID    string         `json:"rollupId"`
			ItemRef     map[string]any `json:"itemRef"`
			SubjectRef  map[string]any `json:"subjectRef"`
			LastOutcome string         `json:"lastOutcome"`
			Display     struct {
				SubjectLabel string `json:"subjectLabel"`
				ItemType     string `json:"itemType"`
			} `json:"display"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected exactly 1 rollup, got %d", len(resp.Data))
	}
	rollup := resp.Data[0]
	if rollup.ItemRef != nil || rollup.SubjectRef != nil {
		t.Fatalf("expected malformed item refs to be omitted, got itemRef=%#v subjectRef=%#v", rollup.ItemRef, rollup.SubjectRef)
	}
	if rollup.Display.SubjectLabel != "tank/apps" {
		t.Fatalf("display.subjectLabel = %q, want %q", rollup.Display.SubjectLabel, "tank/apps")
	}
	if rollup.Display.ItemType != "dataset" {
		t.Fatalf("display.itemType = %q, want %q", rollup.Display.ItemType, "dataset")
	}
	if rollup.LastOutcome != "success" {
		t.Fatalf("lastOutcome = %q, want %q", rollup.LastOutcome, "success")
	}
}

func newRecoveryHandlerWithPersistedPoint(t *testing.T, point recovery.RecoveryPoint) (*RecoveryHandlers, string) {
	t.Helper()

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	manager := recoverymanager.New(mtp)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}

	return NewRecoveryHandlers(manager), filepath.Join(persistence.DataDir(), "recovery", "recovery.db")
}

func corruptRecoveryRowJSON(t *testing.T, dbPath string, rowID string, corruptSubject bool, corruptRepository bool, corruptDetails bool) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	assignments := make([]string, 0, 3)
	if corruptSubject {
		assignments = append(assignments, "subject_ref_json = '{'")
	}
	if corruptRepository {
		assignments = append(assignments, "repository_ref_json = '{'")
	}
	if corruptDetails {
		assignments = append(assignments, "details_json = '{'")
	}
	if len(assignments) == 0 {
		t.Fatal("corruptRecoveryRowJSON called without any fields to corrupt")
	}

	query := "UPDATE recovery_points SET " + joinCSV(assignments) + " WHERE id = ?"
	if _, err := db.ExecContext(context.Background(), query, rowID); err != nil {
		t.Fatalf("corrupt recovery row json: %v", err)
	}
}

func joinCSV(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		out := parts[0]
		for _, part := range parts[1:] {
			out += ", " + part
		}
		return out
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
