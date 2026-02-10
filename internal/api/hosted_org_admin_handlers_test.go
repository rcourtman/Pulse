package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHostedOrganizationsList_HostedModeGate(t *testing.T) {
	router := &Router{
		mux:         http.NewServeMux(),
		config:      &config.Config{DataPath: t.TempDir()},
		multiTenant: config.NewMultiTenantPersistence(t.TempDir()),
		hostedMode:  false,
	}
	router.registerHostedRoutes(nil, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hosted/organizations", nil)
	router.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when hosted mode disabled, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHostedOrganizationsList_ReturnsSummaries(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	createdAt := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	org := &models.Organization{
		ID:          "acme",
		DisplayName: "Acme Co",
		OwnerUserID: "owner@example.com",
		CreatedAt:   createdAt,
		Status:      models.OrgStatusSuspended,
	}
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatalf("SaveOrganization: %v", err)
	}

	router := &Router{
		mux:         http.NewServeMux(),
		config:      &config.Config{DataPath: baseDir},
		multiTenant: mtp,
		hostedMode:  true,
	}
	router.registerHostedRoutes(nil, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hosted/organizations", nil)
	router.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload []struct {
		OrgID       string    `json:"org_id"`
		DisplayName string    `json:"display_name"`
		OwnerUserID string    `json:"owner_user_id"`
		CreatedAt   time.Time `json:"created_at"`
		Suspended   bool      `json:"suspended"`
		SoftDeleted bool      `json:"soft_deleted"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var foundAcme bool
	for _, row := range payload {
		if row.OrgID != "acme" {
			continue
		}
		foundAcme = true
		if row.DisplayName != "Acme Co" {
			t.Fatalf("display_name=%q, want %q", row.DisplayName, "Acme Co")
		}
		if row.OwnerUserID != "owner@example.com" {
			t.Fatalf("owner_user_id=%q, want %q", row.OwnerUserID, "owner@example.com")
		}
		if !row.CreatedAt.Equal(createdAt) {
			t.Fatalf("created_at=%s, want %s", row.CreatedAt.Format(time.RFC3339), createdAt.Format(time.RFC3339))
		}
		if !row.Suspended {
			t.Fatalf("expected suspended=true for acme")
		}
		if row.SoftDeleted {
			t.Fatalf("expected soft_deleted=false for acme")
		}
	}
	if !foundAcme {
		t.Fatalf("expected payload to include org acme, got %+v", payload)
	}
}
