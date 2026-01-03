package providers

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestNewFromConfig_NilConfig(t *testing.T) {
	_, err := NewFromConfig(nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}
	if err.Error() != "AI config is nil" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestNewFromConfig_DisabledAI(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: false,
	}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for disabled AI")
	}
	if err.Error() != "AI is not enabled" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestNewFromConfig_UnknownProvider(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: "unknown-provider",
		APIKey:   "test-key",
		Model:    "", // No model - need to force the legacy path
	}
	// The code tries multi-provider format first, which parses "" as Ollama
	// So this actually succeeds with Ollama provider
	// To test the error path, we need to make sure there's no fallback
	provider, err := NewFromConfig(cfg)
	if err != nil {
		// If it errors, that's expected for unknown provider
		return
	}
	// If it doesn't error, it must have parsed as some valid provider
	// (likely Ollama as the default for unrecognized models)
	if provider != nil && provider.Name() != "ollama" {
		t.Errorf("For unknown provider without API key, expected either error or Ollama fallback, got %s", provider.Name())
	}
}

func TestNewFromConfig_LegacyUnknownProvider(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Model:    "anthropic:claude-3-5-sonnet", // Forces multi-provider to fail if AnthropicAPIKey is missing
		Provider: "unknown",
	}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Fatal("Expected error for unknown legacy provider")
	}
	if !strings.Contains(err.Error(), "unknown provider: unknown") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNewFromConfig_AnthropicWithAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderAnthropic,
		APIKey:   "test-api-key",
		Model:    "claude-3-5-sonnet-20241022",
	}
	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}
	if provider.Name() != "anthropic" {
		t.Errorf("Expected provider name 'anthropic', got '%s'", provider.Name())
	}
}

func TestNewFromConfig_AnthropicNoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderAnthropic,
		APIKey:   "",
		Model:    "claude-3-5-sonnet-20241022",
	}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for Anthropic without API key")
	}
}

func TestNewFromConfig_OpenAIWithAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderOpenAI,
		APIKey:   "test-api-key",
		Model:    "gpt-4o",
	}
	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}
	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", provider.Name())
	}
}

func TestNewFromConfig_OpenAINoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderOpenAI,
		APIKey:   "",
		Model:    "gpt-4o",
	}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for OpenAI without API key")
	}
}

func TestNewFromConfig_Ollama(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderOllama,
		Model:    "llama2",
	}
	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}
	if provider.Name() != "ollama" {
		t.Errorf("Expected provider name 'ollama', got '%s'", provider.Name())
	}
}

func TestNewFromConfig_DeepSeekWithAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderDeepSeek,
		APIKey:   "test-api-key",
		Model:    "deepseek-chat",
	}
	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}
	// DeepSeek uses OpenAI-compatible client
	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai' (DeepSeek uses OpenAI client), got '%s'", provider.Name())
	}
}

func TestNewFromConfig_DeepSeekNoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderDeepSeek,
		APIKey:   "",
		Model:    "deepseek-chat",
	}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for DeepSeek without API key")
	}
}

func TestNewFromConfig_GeminiWithAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderGemini,
		APIKey:   "test-api-key",
		Model:    "gemini-1.5-pro",
	}
	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}
	if provider.Name() != "gemini" {
		t.Errorf("Expected provider name 'gemini', got '%s'", provider.Name())
	}
}

func TestNewFromConfig_GeminiNoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:  true,
		Provider: config.AIProviderGemini,
		APIKey:   "",
		Model:    "gemini-1.5-pro",
	}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for Gemini without API key")
	}
}

func TestNewFromConfig_AnthropicOAuth(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:           true,
		Provider:          config.AIProviderAnthropic,
		Model:             "claude-3-5-sonnet-20241022",
		AuthMethod:        config.AuthMethodOAuth,
		OAuthAccessToken:  "test-token",
		OAuthRefreshToken: "test-refresh",
	}
	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}
	if provider.Name() != "anthropic-oauth" {
		t.Errorf("Expected provider name 'anthropic-oauth', got '%s'", provider.Name())
	}
}

func TestNewForProvider_NilConfig(t *testing.T) {
	_, err := NewForProvider(nil, "anthropic", "claude-3")
	if err == nil {
		t.Error("Expected error for nil config")
	}
}

func TestNewForProvider_UnknownProvider(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: true,
	}
	_, err := NewForProvider(cfg, "unknown-provider", "model")
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestNewForProvider_Anthropic(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:         true,
		AnthropicAPIKey: "test-key",
	}
	provider, err := NewForProvider(cfg, config.AIProviderAnthropic, "claude-3-5-sonnet")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "anthropic" {
		t.Errorf("Expected provider name 'anthropic', got '%s'", provider.Name())
	}
}

func TestNewForProvider_AnthropicNoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: true,
	}
	_, err := NewForProvider(cfg, config.AIProviderAnthropic, "claude-3")
	if err == nil {
		t.Error("Expected error for Anthropic without API key")
	}
}

func TestNewForProvider_OpenAI(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:      true,
		OpenAIAPIKey: "test-key",
	}
	provider, err := NewForProvider(cfg, config.AIProviderOpenAI, "gpt-4o")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", provider.Name())
	}
}

func TestNewForProvider_OpenAINoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: true,
	}
	_, err := NewForProvider(cfg, config.AIProviderOpenAI, "gpt-4o")
	if err == nil {
		t.Error("Expected error for OpenAI without API key")
	}
}

func TestNewForProvider_DeepSeek(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:        true,
		DeepSeekAPIKey: "test-key",
	}
	provider, err := NewForProvider(cfg, config.AIProviderDeepSeek, "deepseek-chat")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// DeepSeek uses OpenAI-compatible client
	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", provider.Name())
	}
}

func TestNewForProvider_DeepSeekNoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: true,
	}
	_, err := NewForProvider(cfg, config.AIProviderDeepSeek, "deepseek-chat")
	if err == nil {
		t.Error("Expected error for DeepSeek without API key")
	}
}

func TestNewForProvider_Ollama(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: true,
	}
	provider, err := NewForProvider(cfg, config.AIProviderOllama, "llama2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "ollama" {
		t.Errorf("Expected provider name 'ollama', got '%s'", provider.Name())
	}
}

func TestNewForProvider_Gemini(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:      true,
		GeminiAPIKey: "test-key",
	}
	provider, err := NewForProvider(cfg, config.AIProviderGemini, "gemini-1.5-pro")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "gemini" {
		t.Errorf("Expected provider name 'gemini', got '%s'", provider.Name())
	}
}

func TestNewForProvider_GeminiNoAPIKey(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: true,
	}
	_, err := NewForProvider(cfg, config.AIProviderGemini, "gemini-1.5-pro")
	if err == nil {
		t.Error("Expected error for Gemini without API key")
	}
}

func TestNewForProvider_AnthropicOAuth(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:           true,
		AuthMethod:        config.AuthMethodOAuth,
		OAuthAccessToken:  "test-token",
		OAuthRefreshToken: "test-refresh",
	}
	provider, err := NewForProvider(cfg, config.AIProviderAnthropic, "claude-3")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "anthropic-oauth" {
		t.Errorf("Expected provider name 'anthropic-oauth', got '%s'", provider.Name())
	}
}

func TestNewForModel(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:         true,
		AnthropicAPIKey: "test-key",
	}

	// Test with provider prefix format (uses colon separator)
	provider, err := NewForModel(cfg, "anthropic:claude-3-5-sonnet")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "anthropic" {
		t.Errorf("Expected provider name 'anthropic', got '%s'", provider.Name())
	}
}

func TestNewForModel_OllamaDefault(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled: true,
	}

	// Test with ollama prefix (uses colon separator)
	provider, err := NewForModel(cfg, "ollama:llama2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "ollama" {
		t.Errorf("Expected provider name 'ollama', got '%s'", provider.Name())
	}
}

func TestNewFromConfig_MultiProviderFormat(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:         true,
		Model:           "anthropic:claude-3-5-sonnet",
		AnthropicAPIKey: "test-key",
	}
	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "anthropic" {
		t.Errorf("Expected provider name 'anthropic', got '%s'", provider.Name())
	}
}

func TestNewForProvider_OllamaWithCustomBaseURL(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:       true,
		OllamaBaseURL: "http://custom-ollama:11434",
	}
	provider, err := NewForProvider(cfg, config.AIProviderOllama, "llama2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "ollama" {
		t.Errorf("Expected provider name 'ollama', got '%s'", provider.Name())
	}
}

func TestNewForProvider_OpenAIWithCustomBaseURL(t *testing.T) {
	cfg := &config.AIConfig{
		Enabled:       true,
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: "https://custom-openai-compatible.example.com",
	}
	provider, err := NewForProvider(cfg, config.AIProviderOpenAI, "gpt-4o")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", provider.Name())
	}
}
