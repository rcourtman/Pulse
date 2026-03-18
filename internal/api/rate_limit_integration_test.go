package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestRateLimitPersistence_FullRoundTrip tests the complete data flow:
// 1. Existing config has RateLimit=120
// 2. User updates other fields without rateLimit in JSON
// 3. Backend preserves RateLimit=120
// 4. GET returns RateLimit=120
func TestRateLimitPersistence_FullRoundTrip(t *testing.T) {
	// Setup mocks
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)

	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)

	h := NewNotificationHandlers(nil, mockMonitor)

	// Existing config in "database" has RateLimit=120
	existingConfig := notifications.EmailConfig{
		Enabled:   true,
		SMTPHost:  "smtp.example.com",
		Password:  "secret",
		RateLimit: 120,
	}

	// Test 1: GET returns the rateLimit
	t.Run("GET_returns_rateLimit", func(t *testing.T) {
		mockManager.On("GetEmailConfig").Return(existingConfig).Once()

		req := httptest.NewRequest("GET", "/api/notifications/email", nil)
		w := httptest.NewRecorder()
		h.GetEmailConfig(w, req)

		assert.Equal(t, 200, w.Code)

		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		// Verify rateLimit is returned (password should be empty for security)
		assert.Equal(t, float64(120), resp["rateLimit"])
		assert.Equal(t, "", resp["password"]) // Redacted
	})

	// Test 2: PUT without rateLimit preserves existing value
	t.Run("PUT_without_rateLimit_preserves_existing", func(t *testing.T) {
		// Return existing config when handler calls GetEmailConfig
		mockManager.On("GetEmailConfig").Return(existingConfig).Once()

		// Expect SetEmailConfig to be called WITH RateLimit=120 preserved
		mockManager.On("SetEmailConfig", mock.MatchedBy(func(c notifications.EmailConfig) bool {
			t.Logf("SetEmailConfig called with RateLimit=%d", c.RateLimit)
			return c.RateLimit == 120 && c.SMTPHost == "smtp.newhost.com"
		})).Return().Once()

		mockPersistence.On("SaveEmailConfig", mock.MatchedBy(func(c notifications.EmailConfig) bool {
			return c.RateLimit == 120
		})).Return(nil).Once()

		// Request body does NOT include rateLimit
		payload := map[string]interface{}{
			"enabled":  true,
			"server":   "smtp.newhost.com",
			"password": "newpassword",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PUT", "/api/notifications/email", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, 200, w.Code)
		mockManager.AssertExpectations(t)
		mockPersistence.AssertExpectations(t)
	})

	// Test 3: PUT with rateLimit=0 explicitly sets it to 0
	t.Run("PUT_with_explicit_rateLimit_0_sets_to_0", func(t *testing.T) {
		mockManager.On("GetEmailConfig").Return(existingConfig).Once()

		mockManager.On("SetEmailConfig", mock.MatchedBy(func(c notifications.EmailConfig) bool {
			t.Logf("SetEmailConfig called with RateLimit=%d", c.RateLimit)
			return c.RateLimit == 0 // User explicitly set to 0
		})).Return().Once()

		mockPersistence.On("SaveEmailConfig", mock.Anything).Return(nil).Once()

		// Request body INCLUDES rateLimit: 0
		payload := map[string]interface{}{
			"enabled":   true,
			"server":    "smtp.example.com",
			"rateLimit": 0,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PUT", "/api/notifications/email", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, 200, w.Code)
		mockManager.AssertExpectations(t)
	})

	// Test 4: PUT with new rateLimit updates it
	t.Run("PUT_with_new_rateLimit_updates", func(t *testing.T) {
		mockManager.On("GetEmailConfig").Return(existingConfig).Once()

		mockManager.On("SetEmailConfig", mock.MatchedBy(func(c notifications.EmailConfig) bool {
			t.Logf("SetEmailConfig called with RateLimit=%d", c.RateLimit)
			return c.RateLimit == 60 // User changed to 60
		})).Return().Once()

		mockPersistence.On("SaveEmailConfig", mock.Anything).Return(nil).Once()

		payload := map[string]interface{}{
			"enabled":   true,
			"server":    "smtp.example.com",
			"rateLimit": 60,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PUT", "/api/notifications/email", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, 200, w.Code)
		mockManager.AssertExpectations(t)
	})
}
