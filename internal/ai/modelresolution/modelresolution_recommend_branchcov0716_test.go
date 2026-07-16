package modelresolution

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestBranchCovSelectRecommendedProviderModel exercises every branch of
// SelectRecommendedProviderModel: the empty/unusable-catalog returns, the
// "first usable becomes best" seeding, and each tie-break arm inside
// recommendedModelBetter (blessed -> suitability rank -> Notable -> CreatedAt
// presence -> CreatedAt value -> lexical sort key on Name -> sort key on ID ->
// stable index tie-break).
func TestBranchCovSelectRecommendedProviderModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		models []providers.ModelInfo
		wantID string
		wantOK bool
		// wantExtra lets a case assert a non-compared field on the winner (used
		// to prove the stable index tie-break returns the earlier entry).
		wantExtra func(*testing.T, providers.ModelInfo)
	}{
		{
			name:   "nil slice returns zero value and false",
			models: nil,
			wantOK: false,
		},
		{
			name:   "empty slice returns zero value and false",
			models: []providers.ModelInfo{},
			wantOK: false,
		},
		{
			name: "only blank IDs return false",
			models: []providers.ModelInfo{
				{ID: ""},
				{ID: "   "},
				{ID: "\t"},
			},
			wantOK: false,
		},
		{
			name: "only specialized entries return false",
			models: []providers.ModelInfo{
				{ID: "text-embedding-3"},
				{ID: "tts-1"},
				{ID: "omni-moderation"},
				{Name: "whisper transcription"},
			},
			wantOK: false,
		},
		{
			name: "specialized marker in name filters entry out",
			models: []providers.ModelInfo{
				{ID: "acme-1", Name: "Acme Embedding v2"},
				{ID: "gpt-4o"},
			},
			wantID: "gpt-4o",
			wantOK: true,
		},
		{
			name: "blank and specialized entries skipped, later valid wins",
			models: []providers.ModelInfo{
				{ID: "   "},
				{ID: ""},
				{ID: "text-embedding-3"},
				{ID: "gpt-4o"},
			},
			wantID: "gpt-4o",
			wantOK: true,
		},
		{
			name: "single usable chat model wins",
			models: []providers.ModelInfo{
				{ID: "gpt-4o"},
			},
			wantID: "gpt-4o",
			wantOK: true,
		},
		{
			name: "blessed model beats non-blessed regardless of order",
			models: []providers.ModelInfo{
				{ID: "gpt-4o"},
				{ID: "qwen3:8b"},
			},
			wantID: "qwen3:8b",
			wantOK: true,
		},
		{
			name: "blessed model beats non-blessed when blessed is first",
			models: []providers.ModelInfo{
				{ID: "qwen3:8b"},
				{ID: "gpt-4o"},
			},
			wantID: "qwen3:8b",
			wantOK: true,
		},
		{
			name: "blessed equivalent id qwen3:latest also wins",
			models: []providers.ModelInfo{
				{ID: "gpt-4o"},
				{ID: "qwen3:latest"},
			},
			wantID: "qwen3:latest",
			wantOK: true,
		},
		{
			name: "blessed id matched case-insensitively and returned verbatim",
			models: []providers.ModelInfo{
				{ID: "gpt-4o"},
				{ID: "  QWEN3:8B  "},
			},
			wantID: "  QWEN3:8B  ",
			wantOK: true,
		},
		{
			name: "chat rank beats unknown rank",
			models: []providers.ModelInfo{
				{ID: "gpt-4o"},
				{ID: "research-only-alpha"},
			},
			wantID: "gpt-4o",
			wantOK: true,
		},
		{
			name: "chat rank beats unknown rank when unknown is first",
			models: []providers.ModelInfo{
				{ID: "research-only-alpha"},
				{ID: "gpt-4o"},
			},
			wantID: "gpt-4o",
			wantOK: true,
		},
		{
			name: "notable flag wins among equal rank",
			models: []providers.ModelInfo{
				{ID: "gpt-4o", Notable: false},
				{ID: "gpt-4.1", Notable: true},
			},
			wantID: "gpt-4.1",
			wantOK: true,
		},
		{
			name: "notable flag wins when notable is first",
			models: []providers.ModelInfo{
				{ID: "gpt-4.1", Notable: true},
				{ID: "gpt-4o", Notable: false},
			},
			wantID: "gpt-4.1",
			wantOK: true,
		},
		{
			name: "presence of created timestamp beats absence",
			models: []providers.ModelInfo{
				{ID: "gpt-4o", Notable: true, CreatedAt: 0},
				{ID: "gpt-4.1", Notable: true, CreatedAt: 1700000000},
			},
			wantID: "gpt-4.1",
			wantOK: true,
		},
		{
			name: "newer created timestamp wins",
			models: []providers.ModelInfo{
				{ID: "gpt-4o", Notable: true, CreatedAt: 1700000000},
				{ID: "gpt-4.1", Notable: true, CreatedAt: 1800000000},
			},
			wantID: "gpt-4.1",
			wantOK: true,
		},
		{
			name: "newer created timestamp wins when newer is first",
			models: []providers.ModelInfo{
				{ID: "gpt-4.1", Notable: true, CreatedAt: 1800000000},
				{ID: "gpt-4o", Notable: true, CreatedAt: 1700000000},
			},
			wantID: "gpt-4.1",
			wantOK: true,
		},
		{
			name: "lexical sort key by name breaks tie",
			models: []providers.ModelInfo{
				{ID: "gpt-z", Notable: true, Name: "Zeta"},
				{ID: "gpt-a", Notable: true, Name: "Alpha"},
			},
			wantID: "gpt-a",
			wantOK: true,
		},
		{
			name: "sort key falls back to id when name is empty",
			models: []providers.ModelInfo{
				{ID: "zzz-unknown"},
				{ID: "aaa-unknown"},
			},
			wantID: "aaa-unknown",
			wantOK: true,
		},
		{
			name: "stable index tie-break returns earlier entry",
			models: []providers.ModelInfo{
				{ID: "gpt-4o", Name: "Same", Description: "first"},
				{ID: "gpt-4o", Name: "Same", Description: "second"},
			},
			wantID: "gpt-4o",
			wantOK: true,
			wantExtra: func(t *testing.T, got providers.ModelInfo) {
				t.Helper()
				if got.Description != "first" {
					t.Fatalf("index tie-break returned Description %q, want %q (earlier index)", got.Description, "first")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := SelectRecommendedProviderModel(tt.models)
			if ok != tt.wantOK {
				t.Fatalf("SelectRecommendedProviderModel ok = %v, want %v (got=%#v)", ok, tt.wantOK, got)
			}
			if !tt.wantOK {
				return
			}
			if got.ID != tt.wantID {
				t.Fatalf("SelectRecommendedProviderModel ID = %q, want %q", got.ID, tt.wantID)
			}
			if tt.wantExtra != nil {
				tt.wantExtra(t, got)
			}
		})
	}
}

// TestBranchCovIsModelUsableWithConfig exercises every branch of
// IsModelUsableWithConfig (chatOnly=false): empty/whitespace model, retired
// quickstart provider, nil config, configured-but-unconfigured provider, and
// the configured-provider true path. It also pins the notable behaviour that
// the non-chat variant does NOT perform chat-suitability filtering, so a
// specialized model on a configured provider still reports usable.
func TestBranchCovIsModelUsableWithConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   *config.AIConfig
		model string
		want  bool
	}{
		{name: "nil config returns false", cfg: nil, model: "openai:gpt-4o", want: false},
		{name: "empty model returns false", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "", want: false},
		{name: "whitespace-only model returns false", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "   ", want: false},
		{name: "quickstart provider returns false", cfg: &config.AIConfig{}, model: "quickstart:foo", want: false},
		{name: "quickstart provider returns false even with credentials present", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "quickstart:gpt-4o", want: false},
		{name: "explicit configured openai returns true", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "openai:gpt-4o", want: true},
		{name: "explicit configured anthropic returns true", cfg: &config.AIConfig{AnthropicAPIKey: "ak"}, model: "anthropic:claude-sonnet-4", want: true},
		{name: "explicit configured deepseek returns true", cfg: &config.AIConfig{DeepSeekAPIKey: "ds"}, model: "deepseek:deepseek-v4", want: true},
		{name: "explicit configured gemini returns true", cfg: &config.AIConfig{GeminiAPIKey: "gm"}, model: "gemini:gemini-3-flash", want: true},
		{name: "explicit configured openrouter returns true", cfg: &config.AIConfig{OpenRouterAPIKey: "or"}, model: "openrouter:openai/gpt-4o", want: true},
		{name: "ollama configured via base url returns true", cfg: &config.AIConfig{OllamaBaseURL: "http://localhost:11434"}, model: "ollama:llama3.2", want: true},
		{name: "whitespace-padded model is trimmed then accepted", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "  openai:gpt-4o  ", want: true},
		{name: "heuristic gpt prefix resolves to openai configured", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "gpt-4o", want: true},
		{name: "heuristic claude prefix resolves to anthropic configured", cfg: &config.AIConfig{AnthropicAPIKey: "ak"}, model: "claude-sonnet-4", want: true},
		{name: "unconfigured anthropic returns false", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "anthropic:claude-sonnet-4", want: false},
		{name: "unrecognized local name defaults to ollama unconfigured", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "totally-unknown-local-model", want: false},
		{name: "specialized model still usable on configured provider (chatOnly=false skips suitability)", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "openai:text-embedding-3", want: true},
		{name: "specialized realtime model still usable on configured provider", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "openai:gpt-4o-realtime", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsModelUsableWithConfig(tt.cfg, tt.model); got != tt.want {
				t.Fatalf("IsModelUsableWithConfig(cfg, %q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// TestBranchCovIsModelUsableWithConfig_NonNilZeroConfig pins the nil-vs-zero
// distinction: a non-nil config with no credentials set must report false
// (driven by HasProvider, not by the nil guard), distinguishing the
// cfg==nil short-circuit from the unconfigured-provider branch.
func TestBranchCovIsModelUsableWithConfig_NonNilZeroConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{}
	if IsModelUsableWithConfig(cfg, "openai:gpt-4o") {
		t.Fatalf("IsModelUsableWithConfig on zero-value non-nil config = true, want false")
	}
}
