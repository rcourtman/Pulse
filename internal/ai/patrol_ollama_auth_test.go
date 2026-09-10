package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

// Exercise the runtime Patrol factory, not a directly constructed Ollama client.
func TestPatrolProviderUsesPersistedOllamaCredentials(t *testing.T) {
	var chats atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "monitor" || password != " fixture password " {
			t.Error("missing or changed synthetic Basic Auth credentials")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/chat":
			chats.Add(1)
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = w.Write([]byte("{\"model\":\"llama3\",\"message\":{\"role\":\"assistant\",\"content\":\"ok\"},\"done\":true}\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	persistence := config.NewConfigPersistence(t.TempDir())
	cfg := config.NewDefaultAIConfig()
	cfg.Enabled = true
	cfg.Model = "ollama:llama3"
	cfg.PatrolModel = "ollama:llama3"
	cfg.OllamaBaseURL = server.URL
	cfg.OllamaUsername = "monitor"
	cfg.OllamaPassword = " fixture password "
	require.NoError(t, persistence.SaveAIConfig(*cfg))
	loaded, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	service := NewService(persistence, nil)
	service.cfg = loaded
	provider, err := service.createPatrolProviderForModel(loaded.GetPatrolModel())
	require.NoError(t, err)
	require.NoError(t, provider.ChatStream(context.Background(), providers.ChatRequest{
		Messages: []providers.Message{{Role: "user", Content: "Synthetic authentication probe"}},
	}, func(providers.StreamEvent) {}))
	require.Equal(t, int32(1), chats.Load())
}
