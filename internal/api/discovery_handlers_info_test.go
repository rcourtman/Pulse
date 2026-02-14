package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticAIConfigProvider struct {
	cfg *config.AIConfig
}

func (p staticAIConfigProvider) GetAIConfig() *config.AIConfig {
	return p.cfg
}

func TestDiscoveryHandlersGetAIProviderInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider AIConfigProvider
		wantNil  bool
		want     *servicediscovery.AIProviderInfo
	}{
		{
			name:    "no provider configured",
			wantNil: true,
		},
		{
			name: "provider returns nil config",
			provider: staticAIConfigProvider{
				cfg: nil,
			},
			wantNil: true,
		},
		{
			name: "ai disabled",
			provider: staticAIConfigProvider{
				cfg: &config.AIConfig{
					Enabled:        false,
					DiscoveryModel: "openai:gpt-4o-mini",
				},
			},
			wantNil: true,
		},
		{
			name: "enabled without resolvable model",
			provider: staticAIConfigProvider{
				cfg: &config.AIConfig{
					Enabled: true,
				},
			},
			wantNil: true,
		},
		{
			name: "explicit ollama discovery model",
			provider: staticAIConfigProvider{
				cfg: &config.AIConfig{
					Enabled:        true,
					DiscoveryModel: "ollama:llama3.1",
				},
			},
			want: &servicediscovery.AIProviderInfo{
				Provider: config.AIProviderOllama,
				Model:    "llama3.1",
				IsLocal:  true,
				Label:    "Local (Ollama)",
			},
		},
		{
			name: "discovery model falls back to main model",
			provider: staticAIConfigProvider{
				cfg: &config.AIConfig{
					Enabled: true,
					Model:   "gpt-4o-mini",
				},
			},
			want: &servicediscovery.AIProviderInfo{
				Provider: config.AIProviderOpenAI,
				Model:    "gpt-4o-mini",
				IsLocal:  false,
				Label:    "Cloud (OpenAI)",
			},
		},
		{
			name: "openrouter discovery model",
			provider: staticAIConfigProvider{
				cfg: &config.AIConfig{
					Enabled:        true,
					DiscoveryModel: "openrouter:openai/gpt-4o-mini",
				},
			},
			want: &servicediscovery.AIProviderInfo{
				Provider: config.AIProviderOpenRouter,
				Model:    "openai/gpt-4o-mini",
				IsLocal:  false,
				Label:    "Cloud (OpenRouter)",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &DiscoveryHandlers{}
			h.SetAIConfigProvider(tt.provider)

			got := h.getAIProviderInfo()
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDiscoveryHandlersHandleGetInfoIncludesAIProviderAndCommands(t *testing.T) {
	t.Parallel()

	h := &DiscoveryHandlers{}
	h.SetAIConfigProvider(staticAIConfigProvider{
		cfg: &config.AIConfig{
			Enabled:        true,
			DiscoveryModel: "anthropic:claude-haiku-4-5",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/discovery/info/vm", nil)
	w := httptest.NewRecorder()

	h.HandleGetInfo(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var info servicediscovery.DiscoveryInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&info))
	require.NotNil(t, info.AIProvider)
	assert.Equal(t, config.AIProviderAnthropic, info.AIProvider.Provider)
	assert.Equal(t, "claude-haiku-4-5", info.AIProvider.Model)
	assert.Equal(t, "Cloud (Anthropic)", info.AIProvider.Label)
	assert.NotEmpty(t, info.Commands)
	assert.NotEmpty(t, info.CommandCategories)
}

func TestDiscoveryHandlersHandleGetInfoWithoutProvider(t *testing.T) {
	t.Parallel()

	h := &DiscoveryHandlers{}

	req := httptest.NewRequest(http.MethodGet, "/api/discovery/info/host", nil)
	w := httptest.NewRecorder()

	h.HandleGetInfo(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var info servicediscovery.DiscoveryInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&info))
	assert.Nil(t, info.AIProvider)
	assert.NotEmpty(t, info.Commands)
	assert.NotEmpty(t, info.CommandCategories)
}
