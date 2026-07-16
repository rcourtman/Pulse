package modelresolution

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// These branch-coverage tests target the model-resolution entry points and the
// selectedModelProviderError helper. They deliberately avoid the live
// provider-catalog paths (providers.NewForProvider + ListModels), which have no
// test seam in this package and would require real network access; every
// deterministic input-validation, preferred-model short-circuit, delegation,
// and error-propagation branch is exercised instead. See GLM_REPORT.md for the
// small set of branches that are not sensibly coverable in isolation.

// TestBranchCovResolveConfiguredModel covers every deterministic branch of
// ResolveConfiguredModel: nil config, explicit usable model (including the
// heuristic no-prefix resolution and the non-chat path that accepts
// specialized models), explicit model on an unconfigured provider (error
// propagated from selectedModelProvider), no configured providers, and
// delegation to ResolveConfiguredProviderModel where a provider-scoped
// preferred model wins without touching the network.
func TestBranchCovResolveConfiguredModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *config.AIConfig
		want    string
		wantErr string // non-empty substring expected in the error
	}{
		{
			name:    "nil config returns config-is-nil error",
			cfg:     nil,
			wantErr: "config is nil",
		},
		{
			name: "explicit usable model on configured provider is returned verbatim",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "openai:gpt-4o",
			},
			want: "openai:gpt-4o",
		},
		{
			name: "explicit model with no provider prefix resolved via heuristic to configured openai",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "gpt-4o",
			},
			want: "gpt-4o",
		},
		{
			name: "explicit specialized model on configured provider is usable on the non-chat path",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "openai:text-embedding-3",
			},
			want: "openai:text-embedding-3",
		},
		{
			name: "explicit model on unconfigured provider surfaces provider-not-configured error",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "deepseek:deepseek-v4",
			},
			wantErr: "deepseek provider is not configured",
		},
		{
			name: "explicit model defaulting to unconfigured ollama via heuristic surfaces ollama error",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "totally-unknown-local-model",
			},
			wantErr: "ollama provider is not configured",
		},
		{
			name:    "no explicit model and no configured providers returns no-provider-configured error",
			cfg:     &config.AIConfig{},
			wantErr: "no provider configured",
		},
		{
			name: "no explicit model delegates to first configured provider and resolves preferred without network",
			cfg: &config.AIConfig{
				// Only Ollama is configured, so configuredProviders[0] == "ollama".
				// Model is empty so ResolveConfiguredModel delegates; the
				// provider-scoped ChatModel is picked as Ollama's preferred model
				// and short-circuits before any catalog call.
				OllamaBaseURL: "http://localhost:11434",
				ChatModel:     "ollama:qwen3:8b",
			},
			want: "ollama:qwen3:8b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveConfiguredModel(context.Background(), tt.cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ResolveConfiguredModel() error = nil, want containing %q (got %q)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveConfiguredModel() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveConfiguredModel() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveConfiguredModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBranchCovResolveConfiguredChatModel covers every deterministic branch of
// ResolveConfiguredChatModel: nil config, explicit chat-suitable model,
// explicit specialized model on a configured provider (which reaches the
// chat-variant "not usable for Assistant chat" fallthrough because the provider
// IS configured but the model is not chat-suitable), explicit model on an
// unconfigured provider, no configured providers, and delegation to
// ResolveConfiguredChatProviderModel via a provider-scoped PatrolModel.
func TestBranchCovResolveConfiguredChatModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *config.AIConfig
		want    string
		wantErr string
	}{
		{
			name:    "nil config returns config-is-nil error",
			cfg:     nil,
			wantErr: "config is nil",
		},
		{
			name: "explicit chat-suitable model returned verbatim",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				ChatModel:    "openai:gpt-4o",
			},
			want: "openai:gpt-4o",
		},
		{
			name: "explicit specialized model on configured provider is rejected as not-usable-for-chat",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				ChatModel:    "openai:text-embedding-3",
			},
			wantErr: "not usable for Assistant chat",
		},
		{
			name: "explicit realtime model on configured provider rejected as not-usable-for-chat",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				ChatModel:    "openai:gpt-4o-realtime",
			},
			wantErr: "not usable for Assistant chat",
		},
		{
			name: "explicit model on unconfigured provider surfaces provider-not-configured",
			cfg: &config.AIConfig{
				OpenRouterAPIKey: "sk-or-test",
				ChatModel:        "deepseek:deepseek-v4",
			},
			wantErr: "deepseek provider is not configured",
		},
		{
			name:    "no explicit model and no configured providers returns no-provider-configured error",
			cfg:     &config.AIConfig{},
			wantErr: "no provider configured",
		},
		{
			name: "no explicit model delegates and resolves provider-scoped preferred chat model",
			cfg: &config.AIConfig{
				// Neither ChatModel nor Model is set, so GetChatModel() == "" and
				// ResolveConfiguredChatModel delegates to the first configured
				// provider. PatrolModel is provider-scoped to OpenAI and is
				// chat-suitable, so it wins without a catalog call.
				OpenAIAPIKey: "sk-test",
				PatrolModel:  "openai:gpt-4o",
			},
			want: "openai:gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveConfiguredChatModel(context.Background(), tt.cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ResolveConfiguredChatModel() error = nil, want containing %q (got %q)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveConfiguredChatModel() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveConfiguredChatModel() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveConfiguredChatModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBranchCovResolvePreferredModelForProvider covers: nil config, the
// preferred-model short-circuit win, and the fall-through-to-delegate path in
// several flavours (preferred present but on an unconfigured provider, and an
// empty preferred routing into the delegate's provider validation branches).
func TestBranchCovResolvePreferredModelForProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.AIConfig
		provider string
		want     string
		wantErr  string
	}{
		{
			name:     "nil config returns config-is-nil error",
			cfg:      nil,
			provider: "openai",
			wantErr:  "config is nil",
		},
		{
			name: "provider-scoped preferred usable model wins",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "openai:gpt-4o",
			},
			provider: "openai",
			want:     "openai:gpt-4o",
		},
		{
			name: "preferred present but provider unconfigured falls through to delegate provider-not-configured",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "deepseek:deepseek-v4",
			},
			provider: "deepseek",
			wantErr:  "deepseek provider is not configured",
		},
		{
			name:     "empty preferred and empty provider delegate to provider-required error",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "",
			wantErr:  "provider is required",
		},
		{
			name:     "empty preferred and quickstart provider delegate to retired error",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "quickstart",
			wantErr:  "quickstart provider is retired",
		},
		{
			name:     "empty preferred and unconfigured provider delegate to provider-not-configured",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "anthropic",
			wantErr:  "anthropic provider is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolvePreferredModelForProvider(context.Background(), tt.cfg, tt.provider)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ResolvePreferredModelForProvider() error = nil, want containing %q (got %q)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolvePreferredModelForProvider() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePreferredModelForProvider() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("ResolvePreferredModelForProvider() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBranchCovResolveConfiguredProviderModel covers the deterministic branches
// of ResolveConfiguredProviderModel (chatOnly=false): nil config, empty and
// whitespace provider, retired quickstart, unconfigured provider, and the
// preferred-model short-circuit (including whitespace trimming of the provider
// argument and the non-chat acceptance of a specialized preferred model).
func TestBranchCovResolveConfiguredProviderModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.AIConfig
		provider string
		want     string
		wantErr  string
	}{
		{
			name:     "nil config returns config-is-nil error",
			cfg:      nil,
			provider: "openai",
			wantErr:  "config is nil",
		},
		{
			name:     "empty provider returns provider-required error",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "",
			wantErr:  "provider is required",
		},
		{
			name:     "whitespace-only provider is trimmed to empty and returns provider-required",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "   ",
			wantErr:  "provider is required",
		},
		{
			name:     "quickstart provider returns retired error",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "quickstart",
			wantErr:  "quickstart provider is retired",
		},
		{
			name:     "unconfigured provider returns provider-not-configured",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "anthropic",
			wantErr:  "anthropic provider is not configured",
		},
		{
			name: "whitespace-padded configured provider trimmed and preferred model wins",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				PatrolModel:  "openai:gpt-4o",
			},
			provider: "  openai  ",
			want:     "openai:gpt-4o",
		},
		{
			name: "specialized preferred model wins on the non-chat path",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				Model:        "openai:text-embedding-3",
			},
			provider: "openai",
			want:     "openai:text-embedding-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveConfiguredProviderModel(context.Background(), tt.cfg, tt.provider)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ResolveConfiguredProviderModel() error = nil, want containing %q (got %q)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveConfiguredProviderModel() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveConfiguredProviderModel() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveConfiguredProviderModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBranchCovResolveConfiguredChatProviderModel covers the deterministic
// branches of ResolveConfiguredChatProviderModel (chatOnly=true): nil config,
// empty/whitespace provider, retired quickstart, unconfigured provider, and the
// chat-suitable preferred-model short-circuit.
func TestBranchCovResolveConfiguredChatProviderModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.AIConfig
		provider string
		want     string
		wantErr  string
	}{
		{
			name:     "nil config returns config-is-nil error",
			cfg:      nil,
			provider: "openai",
			wantErr:  "config is nil",
		},
		{
			name:     "empty provider returns provider-required error",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "",
			wantErr:  "provider is required",
		},
		{
			name:     "whitespace-only provider is trimmed to empty and returns provider-required",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "\t",
			wantErr:  "provider is required",
		},
		{
			name:     "quickstart provider returns retired error",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "quickstart",
			wantErr:  "quickstart provider is retired",
		},
		{
			name:     "unconfigured provider returns provider-not-configured",
			cfg:      &config.AIConfig{OpenAIAPIKey: "sk-test"},
			provider: "gemini",
			wantErr:  "gemini provider is not configured",
		},
		{
			name: "chat-suitable provider-scoped preferred model wins",
			cfg: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				PatrolModel:  "openai:gpt-4o",
			},
			provider: "openai",
			want:     "openai:gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveConfiguredChatProviderModel(context.Background(), tt.cfg, tt.provider)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ResolveConfiguredChatProviderModel() error = nil, want containing %q (got %q)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveConfiguredChatProviderModel() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveConfiguredChatProviderModel() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveConfiguredChatProviderModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBranchCovSelectedModelProviderError exercises selectedModelProviderError
// directly. It covers every selectedModelProvider propagation arm (empty model,
// quickstart provider, nil config, unconfigured provider reached via the
// default-Ollama heuristic) AND the fallthrough "not usable with the current
// Pulse Assistant config" arm. The fallthrough arm is only reachable by a
// direct call: when ResolveConfiguredModel invokes selectedModelProviderError
// it has already established IsModelUsableWithConfig==false, whose provider
// checks are equivalent to selectedModelProvider's, so selectedModelProvider
// always returns an error there and the fallthrough never runs in production
// (see GLM_REPORT.md).
func TestBranchCovSelectedModelProviderError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *config.AIConfig
		model   string
		wantErr string
	}{
		{
			name:    "empty model propagates selected-model-route-is-empty",
			cfg:     &config.AIConfig{OpenAIAPIKey: "sk-test"},
			model:   "",
			wantErr: "selected model route is empty",
		},
		{
			name:    "whitespace model propagates selected-model-route-is-empty",
			cfg:     &config.AIConfig{OpenAIAPIKey: "sk-test"},
			model:   "   ",
			wantErr: "selected model route is empty",
		},
		{
			name:    "quickstart provider propagates retired error",
			cfg:     &config.AIConfig{},
			model:   "quickstart:gpt-4o",
			wantErr: "quickstart provider is retired",
		},
		{
			name:    "nil config propagates provider-not-configured-for-route",
			cfg:     nil,
			model:   "openai:gpt-4o",
			wantErr: "openai provider is not configured for selected model route",
		},
		{
			name:    "default-ollama heuristic on configured-openai-only propagates ollama-not-configured-for-route",
			cfg:     &config.AIConfig{OpenAIAPIKey: "sk-test"},
			model:   "totally-unknown-local-model",
			wantErr: "ollama provider is not configured for selected model route",
		},
		{
			name:    "configured provider fallthrough returns not-usable-with-current-config",
			cfg:     &config.AIConfig{OpenAIAPIKey: "sk-test"},
			model:   "openai:gpt-4o",
			wantErr: "not usable with the current Pulse Assistant config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := selectedModelProviderError(tt.cfg, tt.model)
			if err == nil {
				t.Fatalf("selectedModelProviderError(%q) = nil, want error containing %q", tt.model, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("selectedModelProviderError(%q) = %q, want containing %q", tt.model, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestBranchCovResolveConfiguredModel_NeverReachesNotUsableFallthrough pins the
// dead-code observation described above: for every explicit-model input that
// drives ResolveConfiguredModel into its error branch, the surfaced error is
// always selectedModelProvider's propagation, never the
// "not usable with the current Pulse Assistant config" fallthrough message.
func TestBranchCovResolveConfiguredModel_NeverReachesNotUsableFallthrough(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		cfg   *config.AIConfig
		model string
	}{
		{name: "unconfigured provider", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "deepseek:deepseek-v4"},
		{name: "default ollama heuristic unconfigured", cfg: &config.AIConfig{OpenAIAPIKey: "sk"}, model: "weird-local-name"},
		// A quickstart Model is intentionally omitted here: GetModel() runs it
		// through NormalizeQuickstartModelString, which collapses it to "" so
		// ResolveConfiguredModel treats it as "no explicit model" and never
		// reaches selectedModelProviderError at all.
	}
	for _, tc := range cases {
		tc.cfg.Model = tc.model
		_, err := ResolveConfiguredModel(context.Background(), tc.cfg)
		if err == nil {
			t.Fatalf("%s: expected an error, got nil", tc.name)
		}
		if strings.Contains(err.Error(), "not usable with the current Pulse Assistant config") {
			t.Fatalf("%s: error %q reached the supposedly-unreachable fallthrough", tc.name, err.Error())
		}
	}
}
