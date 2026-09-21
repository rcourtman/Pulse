package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAISettingsOllamaPasswordLifecycle(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(&config.Config{DataPath: tmp}, persistence, nil)
	for _, step := range []struct {
		name, payload, username, password string
	}{
		{"set", `{"ollama_username":"monitor","ollama_password":" fixture password "}`, "monitor", " fixture password "},
		{"preserve", `{"ollama_keep_alive":"24h"}`, "monitor", " fixture password "},
		{"replace", `{"ollama_password":"replacement"}`, "monitor", "replacement"},
		{"clear", `{"ollama_username":"","clear_ollama_password":true}`, "", ""},
	} {
		t.Run(step.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.HandleUpdateAISettings(rec, newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewBufferString(step.payload)))
			require.Equal(t, http.StatusOK, rec.Code)
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.NotContains(t, response, "ollama_password")
			require.Equal(t, step.password != "", response["ollama_password_set"])
			stored, err := persistence.LoadAIConfig()
			require.NoError(t, err)
			require.Equal(t, step.username, stored.OllamaUsername)
			require.Equal(t, step.password, stored.OllamaPassword)
		})
	}
}
