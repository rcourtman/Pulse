package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
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
