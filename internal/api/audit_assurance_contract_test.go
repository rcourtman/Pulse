package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/stretchr/testify/assert"
)

func TestAuditAssuranceContractAcrossHandlers(t *testing.T) {
	mac := strings.Repeat("a", 64)
	events := []audit.Event{
		{ID: "strong", Signature: "v3:" + mac, EventType: "login", Success: true, Timestamp: time.Unix(5, 0).UTC()},
		{ID: "compatibility", Signature: "v2:" + mac, EventType: "login", Success: true, Timestamp: time.Unix(4, 0).UTC()},
		{ID: "invalid", Signature: "v3:" + mac, EventType: "login", Timestamp: time.Unix(3, 0).UTC()},
		{ID: "unknown", Signature: "v9:" + mac, EventType: "login", Timestamp: time.Unix(2, 0).UTC()},
		{ID: "unsigned", EventType: "login", Timestamp: time.Unix(1, 0).UTC()},
	}
	results := map[string]audit.SignatureVerification{
		"strong":        {Status: audit.VerificationStatusStrong, Version: audit.SignatureVersionV3, Assurance: audit.SignatureAssuranceStrong, Verified: true},
		"compatibility": {Status: audit.VerificationStatusCompatibility, Version: audit.SignatureVersionV2, Assurance: audit.SignatureAssuranceCompatibility, Verified: true},
		"invalid":       {Status: audit.VerificationStatusInvalid, Version: audit.SignatureVersionV3, Assurance: audit.SignatureAssuranceNone},
		"unknown":       {Status: audit.VerificationStatusUnknown, Version: audit.SignatureVersionUnknown, Assurance: audit.SignatureAssuranceNone},
		"unsigned":      {Status: audit.VerificationStatusUnsigned, Version: audit.SignatureVersionUnsigned, Assurance: audit.SignatureAssuranceNone},
	}
	logger := &classifiedTestAuditLogger{
		testAuditLogger: &testAuditLogger{events: events},
		results:         results,
	}
	setAuditLogger(t, logger)
	handler := NewAuditHandlers()

	t.Run("verify", func(t *testing.T) {
		for _, event := range events {
			event := event
			t.Run(event.ID, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/api/audit/"+event.ID+"/verify", nil)
				req.SetPathValue("id", event.ID)
				rec := httptest.NewRecorder()
				handler.HandleVerifyAuditEvent(rec, req)
				assert.Equal(t, http.StatusOK, rec.Code)

				var response verifyResponse
				assert.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
				want := results[event.ID]
				assert.Equal(t, want.Status, response.Status)
				assert.Equal(t, want.Version, response.Version)
				assert.Equal(t, want.Assurance, response.Assurance)
				assert.Equal(t, want.Verified, response.Verified)
			})
		}
	})

	t.Run("list", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.HandleListAuditEvents(rec, httptest.NewRequest(http.MethodGet, "/api/audit", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
		var response struct {
			Events []audit.EventProjection `json:"events"`
		}
		assert.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Len(t, response.Events, len(events))
		for _, event := range response.Events {
			want := results[event.ID]
			assert.Equal(t, want.Status, event.SignatureStatus)
			assert.Equal(t, want.Version, event.SignatureVersion)
			assert.Equal(t, want.Assurance, event.SignatureAssurance)
		}
	})

	t.Run("export", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.HandleExportAuditEvents(rec, httptest.NewRequest(http.MethodGet, "/api/audit/export?format=json&verify=true", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
		var response struct {
			Events []audit.ExportEvent `json:"events"`
		}
		assert.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Len(t, response.Events, len(events))
		for _, event := range response.Events {
			want := results[event.ID]
			assert.Equal(t, want.Status, event.SignatureStatus)
			assert.Equal(t, want.Version, event.SignatureVersion)
			assert.Equal(t, want.Assurance, event.SignatureAssurance)
		}
	})

	t.Run("summary", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.HandleAuditSummary(rec, httptest.NewRequest(http.MethodGet, "/api/audit/summary", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
		var summary audit.ExportSummary
		assert.NoError(t, json.NewDecoder(rec.Body).Decode(&summary))
		assert.Equal(t, 1, summary.StrongSigCount)
		assert.Equal(t, 1, summary.CompatibilitySigCount)
		assert.Equal(t, 1, summary.InvalidSigCount)
		assert.Equal(t, 1, summary.UnknownSigCount)
		assert.Equal(t, 1, summary.UnsignedSigCount)
	})
}
