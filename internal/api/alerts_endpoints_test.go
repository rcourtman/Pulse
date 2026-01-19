package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestAlertsEndpoints(t *testing.T) {
	srv := newIntegrationServer(t)

	// 1. Get initial alert config
	t.Run("GetAlertConfig", func(t *testing.T) {
		res, err := http.Get(srv.server.URL + "/api/alerts/config")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		var config alerts.AlertConfig
		if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
	})

	// 2. Update alert config
	t.Run("UpdateAlertConfig", func(t *testing.T) {
		newConfig := alerts.AlertConfig{
			Schedule: alerts.ScheduleConfig{
				Cooldown: 300,
			},
		}
		body, _ := json.Marshal(newConfig)
		// HandleAlerts expects PUT for config updates
		req, err := http.NewRequest(http.MethodPut, srv.server.URL+"/api/alerts/config", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Verify update persistence
		resVerify, err := http.Get(srv.server.URL + "/api/alerts/config")
		if err != nil {
			t.Fatalf("verify request failed: %v", err)
		}
		defer resVerify.Body.Close()

		var updatedConfig alerts.AlertConfig
		if err := json.NewDecoder(resVerify.Body).Decode(&updatedConfig); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		if updatedConfig.Schedule.Cooldown != 300 {
			t.Errorf("expected cooldown 300, got %d", updatedConfig.Schedule.Cooldown)
		}
	})

	// 3. Activate alerts
	t.Run("ActivateAlerts", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/activate", nil)
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Activate again (should be idempotent)
		res2, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("retry request failed: %v", err)
		}
		defer res2.Body.Close()
		if res2.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res2.StatusCode, http.StatusOK)
		}
	})

	// 4. Get active alerts
	t.Run("GetActiveAlerts", func(t *testing.T) {
		res, err := http.Get(srv.server.URL + "/api/alerts/active")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 5. Get alert history
	t.Run("GetAlertHistory", func(t *testing.T) {
		res, err := http.Get(srv.server.URL + "/api/alerts/history")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Test filters
		resFilter, err := http.Get(srv.server.URL + "/api/alerts/history?limit=10&severity=critical")
		if err != nil {
			t.Fatalf("filter request failed: %v", err)
		}
		defer resFilter.Body.Close()
		if resFilter.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", resFilter.StatusCode, http.StatusOK)
		}
	})

	// 6. Clear alert history
	t.Run("ClearAlertHistory", func(t *testing.T) {
		// HandleAlerts expects DELETE on /history
		req, err := http.NewRequest(http.MethodDelete, srv.server.URL+"/api/alerts/history", nil)
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 7. Acknowledge Alert (Single)
	t.Run("AcknowledgeAlert", func(t *testing.T) {
		body := map[string]string{"id": "test-alert-id"}
		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/acknowledge", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		// Should be 404 because alert doesn't exist, but that proves the handler code ran
		if res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want 404 or 200", res.StatusCode)
		}
	})

	// 8. Bulk Acknowledge
	t.Run("BulkAcknowledge", func(t *testing.T) {
		body := map[string]interface{}{
			"alertIds": []string{"alert-1", "alert-2"},
		}
		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/bulk/acknowledge", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 9. Bulk Clear
	t.Run("BulkClear", func(t *testing.T) {
		body := map[string]interface{}{
			"alertIds": []string{"alert-1", "alert-2"},
		}
		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/bulk/clear", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 10. Incident Timeline
	t.Run("GetIncidentTimeline", func(t *testing.T) {
		// Test timeline list by resource
		res, err := http.Get(srv.server.URL + "/api/alerts/incidents?resource_id=test-node")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Test specific alert timeline
		res2, err := http.Get(srv.server.URL + "/api/alerts/incidents?alert_id=test-alert")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res2.Body.Close()

		// 200 OK (empty/null) or 404 depending on impl. Implementation returns null/empty usually if not found but status 200.
		if res2.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res2.StatusCode, http.StatusOK)
		}
	})
}
