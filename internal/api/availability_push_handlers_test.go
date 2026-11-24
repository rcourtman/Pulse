package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAvailabilityPushHandlersCreatePingRotateAndBounds(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		nil,
		nil,
	)

	// Create a push check: server auto-generates a 256-bit token.
	createBody := bytes.NewBufferString(`{
		"name": "Nightly restic backup",
		"protocol": "push",
		"enabled": true,
		"push": {
			"scheduleType": "interval",
			"intervalSeconds": 86400,
			"graceSeconds": 300
		}
	}`)
	createRec := httptest.NewRecorder()
	handler.HandleAdd(createRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", createBody))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	var created availabilityTargetResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Push == nil || len(created.Push.Token) < 32 {
		t.Fatalf("expected server-generated push token, got %+v", created.Push)
	}
	originalToken := created.Push.Token

	// Updating the check's schedule preserves the existing token.
	updateBody := bytes.NewBufferString(`{
		"name": "Nightly restic backup",
		"protocol": "push",
		"enabled": true,
		"push": {
			"scheduleType": "cron",
			"cronExpression": "0 3 * * *",
			"graceSeconds": 600
		}
	}`)
	updateRec := httptest.NewRecorder()
	handler.HandleUpdate(updateRec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/"+created.ID, updateBody))
	if updateRec.Code != http.StatusOK {
		t.Fatalf("HandleUpdate status = %d, body = %s", updateRec.Code, updateRec.Body.String())
	}
	var updated availabilityTargetResponse
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Push == nil || updated.Push.Token != originalToken {
		t.Fatalf("expected PUT update to preserve token %q, got %+v", originalToken, updated.Push)
	}

	// Outbound test endpoint refuses push checks cleanly.
	testRec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(testRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/"+created.ID+"/test", nil))
	if testRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on outbound test of push check, got %d", testRec.Code)
	}

	// Inbound ping with valid token succeeds (GET, /start, /fail, ?status=down, POST body).
	pingRec := httptest.NewRecorder()
	handler.HandlePushPing(pingRec, httptest.NewRequest(http.MethodGet, "/api/push/"+originalToken+"?status=up&msg=ok&ping=42", nil))
	if pingRec.Code != http.StatusOK {
		t.Fatalf("valid GET ping status = %d, body = %s", pingRec.Code, pingRec.Body.String())
	}

	startRec := httptest.NewRecorder()
	handler.HandlePushPing(startRec, httptest.NewRequest(http.MethodPost, "/api/push/"+originalToken+"/start", strings.NewReader("starting")))
	if startRec.Code != http.StatusOK {
		t.Fatalf("valid /start ping status = %d, body = %s", startRec.Code, startRec.Body.String())
	}

	failRec := httptest.NewRecorder()
	handler.HandlePushPing(failRec, httptest.NewRequest(http.MethodPost, "/api/push/"+originalToken+"/fail", strings.NewReader("backup exit 1")))
	if failRec.Code != http.StatusOK {
		t.Fatalf("valid /fail ping status = %d, body = %s", failRec.Code, failRec.Body.String())
	}

	// Oversized POST body (>1024 bytes) is rejected with 413.
	largeRec := httptest.NewRecorder()
	handler.HandlePushPing(largeRec, httptest.NewRequest(http.MethodPost, "/api/push/"+originalToken, strings.NewReader(strings.Repeat("x", 2048))))
	if largeRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized push body, got %d", largeRec.Code)
	}

	// Rotate push token: old token returns 404 immediately, new token returns 200.
	rotateRec := httptest.NewRecorder()
	handler.HandleRotatePushToken(rotateRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/"+created.ID+"/rotate-push-token", nil))
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("HandleRotatePushToken status = %d, body = %s", rotateRec.Code, rotateRec.Body.String())
	}
	var rotated availabilityTargetResponse
	if err := json.Unmarshal(rotateRec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	if rotated.Push == nil || rotated.Push.Token == "" || rotated.Push.Token == originalToken {
		t.Fatalf("expected fresh rotated token distinct from %q, got %+v", originalToken, rotated.Push)
	}

	oldPingRec := httptest.NewRecorder()
	handler.HandlePushPing(oldPingRec, httptest.NewRequest(http.MethodGet, "/api/push/"+originalToken, nil))
	if oldPingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for revoked push token, got %d", oldPingRec.Code)
	}

	newPingRec := httptest.NewRecorder()
	handler.HandlePushPing(newPingRec, httptest.NewRequest(http.MethodGet, "/api/push/"+rotated.Push.Token, nil))
	if newPingRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for new rotated push token, got %d", newPingRec.Code)
	}
}
