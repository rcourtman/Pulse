import { describe, expect, it } from 'vitest';
import type { AIProvider, AISettings } from '@/types/ai';
import { PROVIDER_DESCRIPTIONS } from '@/types/ai';
import {
  AI_PROVIDERS,
  AI_PROVIDER_CONFIGS,
  AI_SETUP_PROVIDER_OPTIONS,
  createInitialProviderHealth,
  getAIProviderConfig,
  isAIProviderConfigured,
  isModelProviderConfigured,
} from '../aiSettingsModel';

const ALL_PROVIDERS: AIProvider[] = [
  'anthropic',
  'openai',
  'openrouter',
  'deepseek',
  'gemini',
  'zai',
  'groq',
  'mistral',
  'cerebras',
  'together',
  'fireworks',
  'ollama',
];

function makeSettings(overrides: Partial<AISettings> = {}): AISettings {
  return {
    enabled: false,
    model: '',
    configured: false,
    custom_context: '',
    auth_method: 'api_key',
    oauth_connected: false,
    anthropic_configured: false,
    openai_configured: false,
    openrouter_configured: false,
    deepseek_configured: false,
    gemini_configured: false,
    ollama_configured: false,
    ollama_base_url: '',
    ollama_keep_alive: '',
    configured_providers: [],
    ...overrides,
  };
}

describe('aiSettingsModel - AI_SETUP_PROVIDER_OPTIONS contract', () => {
  it('uses each provider value at most once', () => {
    const values = AI_SETUP_PROVIDER_OPTIONS.map((option) => option.value);
    expect(new Set(values).size).toBe(values.length);
  });

  it('lists provider values in the same order as AI_PROVIDERS', () => {
    expect(AI_SETUP_PROVIDER_OPTIONS.map((option) => option.value)).toEqual(AI_PROVIDERS);
  });

  it('gives every option a non-empty title and a present description', () => {
    for (const option of AI_SETUP_PROVIDER_OPTIONS) {
      expect(option.title.length).toBeGreaterThan(0);
      expect(option.description).toBeDefined();
      expect((option.description ?? '').length).toBeGreaterThan(0);
    }
  });

  it('keeps option values in sync with PROVIDER_DESCRIPTIONS in both directions', () => {
    const optionValues = new Set(AI_SETUP_PROVIDER_OPTIONS.map((option) => option.value));
    const descriptionKeys = new Set(Object.keys(PROVIDER_DESCRIPTIONS));

    for (const value of optionValues) {
      expect(descriptionKeys.has(value)).toBe(true);
    }
    for (const key of descriptionKeys) {
      expect(optionValues.has(key as AIProvider)).toBe(true);
    }
  });
});

describe('aiSettingsModel - createInitialProviderHealth', () => {
  it('seeds exactly the known providers, each not_configured with an empty message', () => {
    const health = createInitialProviderHealth();

    expect(Object.keys(health).sort()).toEqual([...ALL_PROVIDERS].sort());
    for (const provider of ALL_PROVIDERS) {
      expect(health[provider]).toEqual({ status: 'not_configured', message: '' });
    }
  });
});

describe('aiSettingsModel - getAIProviderConfig', () => {
  it('returns the config whose provider matches for every known provider', () => {
    for (const provider of ALL_PROVIDERS) {
      expect(getAIProviderConfig(provider).provider).toBe(provider);
    }
  });

  it('distinguishes the url-based ollama config from password-based providers', () => {
    expect(getAIProviderConfig('ollama').inputType).toBe('url');
    expect(getAIProviderConfig('ollama').inputField).toBe('ollamaBaseUrl');
    expect(getAIProviderConfig('anthropic').inputType).toBe('password');
    expect(getAIProviderConfig('anthropic').inputField).toBe('anthropicApiKey');
  });

  it('throws and names the provider when it is unknown', () => {
    const unknown = 'pulse' as unknown as AIProvider;
    expect(() => getAIProviderConfig(unknown)).toThrowError(/pulse/);
  });

  it('exposes a config entry for every option value', () => {
    const configuredProviders = new Set(AI_PROVIDER_CONFIGS.map((config) => config.provider));
    for (const option of AI_SETUP_PROVIDER_OPTIONS) {
      expect(configuredProviders.has(option.value)).toBe(true);
    }
  });
});

describe('aiSettingsModel - isAIProviderConfigured', () => {
  it('returns false when settings are null regardless of provider', () => {
    expect(isAIProviderConfigured('anthropic', null)).toBe(false);
    expect(isAIProviderConfigured('ollama', null)).toBe(false);
  });

  it('routes each provider to its own configured field', () => {
    const fieldByProvider: Record<AIProvider, keyof AISettings> = {
      anthropic: 'anthropic_configured',
      openai: 'openai_configured',
      openrouter: 'openrouter_configured',
      deepseek: 'deepseek_configured',
      gemini: 'gemini_configured',
      zai: 'zai_configured',
      groq: 'groq_configured',
      mistral: 'mistral_configured',
      cerebras: 'cerebras_configured',
      together: 'together_configured',
      fireworks: 'fireworks_configured',
      ollama: 'ollama_configured',
    };

    for (const provider of ALL_PROVIDERS) {
      const onlyThis = makeSettings({
        [fieldByProvider[provider]]: true,
      } as Partial<AISettings>);

      expect(isAIProviderConfigured(provider, onlyThis)).toBe(true);
      const other = provider === 'anthropic' ? 'openai' : 'anthropic';
      expect(isAIProviderConfigured(other, onlyThis)).toBe(false);
    }
  });

  it('treats missing optional provider fields as not configured', () => {
    const settings = makeSettings();
    expect(settings.zai_configured).toBeUndefined();
    expect(isAIProviderConfigured('zai', settings)).toBe(false);
    expect(isAIProviderConfigured('zai', makeSettings({ zai_configured: true }))).toBe(true);
  });

  it('returns false for an unknown provider', () => {
    expect(isAIProviderConfigured('pulse', makeSettings())).toBe(false);
  });
});

describe('aiSettingsModel - isModelProviderConfigured', () => {
  it('returns false when settings are null', () => {
    expect(isModelProviderConfigured('anthropic:claude-opus-4', null)).toBe(false);
  });

  it('delegates to the provider resolved from the model id', () => {
    const anthropicConfigured = makeSettings({ anthropic_configured: true });
    expect(isModelProviderConfigured('claude-opus-4', anthropicConfigured)).toBe(true);

    const noneConfigured = makeSettings();
    expect(isModelProviderConfigured('gpt-4o', noneConfigured)).toBe(false);
  });
});
