import { describe, expect, it } from 'vitest';
import {
  AI_PROVIDER_DISPLAY_NAMES,
  formatAIModelRouteLabel,
  getAIProviderDisplayName,
  getProviderFromModelId,
  isPulseOwnedLocalModelRoute,
} from '@/utils/aiProviderPresentation';

describe('aiProviderPresentation', () => {
  it('returns canonical provider display names', () => {
    expect(AI_PROVIDER_DISPLAY_NAMES.openai).toBe('OpenAI');
    expect(getAIProviderDisplayName('anthropic')).toBe('Anthropic');
    expect(getAIProviderDisplayName('gemini')).toBe('Google Gemini');
    expect(getAIProviderDisplayName('pulse')).toBe('Pulse');
    expect(getAIProviderDisplayName('custom-provider')).toBe('custom-provider');
  });

  it('detects providers from explicit prefixes and model naming heuristics', () => {
    expect(getProviderFromModelId('openai:gpt-4o')).toBe('openai');
    expect(getProviderFromModelId('anthropic/claude-sonnet-4.5')).toBe('openrouter');
    expect(getProviderFromModelId('claude-3-5-sonnet')).toBe('anthropic');
    expect(getProviderFromModelId('o4-mini')).toBe('openai');
    expect(getProviderFromModelId('deepseek-r1')).toBe('deepseek');
    expect(getProviderFromModelId('gemini-2.5-pro')).toBe('gemini');
    expect(getProviderFromModelId('llama3.1')).toBe('ollama');
  });

  it('keeps OpenRouter-routed model labels distinct from direct provider models', () => {
    expect(
      formatAIModelRouteLabel({
        id: 'openrouter:deepseek/deepseek-v4-pro',
        name: 'DeepSeek: DeepSeek V4 Pro',
        provider: 'openrouter',
      }),
    ).toBe('DeepSeek: DeepSeek V4 Pro via OpenRouter');

    expect(
      formatAIModelRouteLabel({
        id: 'deepseek:deepseek-v4-pro',
        name: 'DeepSeek: DeepSeek V4 Pro',
        provider: 'deepseek',
      }),
    ).toBe('DeepSeek: DeepSeek V4 Pro');

    expect(formatAIModelRouteLabel('deepseek:deepseek-v4-pro')).toBe(
      'DeepSeek: DeepSeek V4 Pro',
    );
  });

  it('does not duplicate an existing OpenRouter route label', () => {
    expect(
      formatAIModelRouteLabel({
        id: 'openrouter:deepseek/deepseek-r1',
        name: 'DeepSeek R1 via OpenRouter',
        provider: 'openrouter',
      }),
    ).toBe('DeepSeek R1 via OpenRouter');
  });

  it('formats OpenRouter route IDs when model catalog names are not hydrated', () => {
    expect(formatAIModelRouteLabel('openrouter:qwen/qwen3.7-plus')).toBe(
      'Qwen: Qwen3.7 Plus via OpenRouter',
    );
    expect(
      formatAIModelRouteLabel({
        id: 'openrouter:anthropic/claude-sonnet-4.5',
        name: 'anthropic/claude-sonnet-4.5',
        provider: 'openrouter',
      }),
    ).toBe('Anthropic: Claude Sonnet 4.5 via OpenRouter');
    expect(formatAIModelRouteLabel('openrouter:~anthropic/claude-sonnet-latest')).toBe(
      'Anthropic: Claude Sonnet Latest via OpenRouter',
    );
  });

  it('labels Pulse-owned local runtime routes without provider wording', () => {
    expect(formatAIModelRouteLabel('pulse:local-inventory')).toBe('Pulse inventory');
    expect(formatAIModelRouteLabel('pulse:mock-assistant')).toBe('Pulse mock Assistant');
    expect(
      formatAIModelRouteLabel({
        id: 'pulse:mock-assistant',
        name: 'Pulse mock Assistant',
        provider: 'pulse',
      }),
    ).toBe('Pulse mock Assistant');
    expect(isPulseOwnedLocalModelRoute('pulse:local-inventory')).toBe(true);
    expect(isPulseOwnedLocalModelRoute('pulse:mock-assistant')).toBe(true);
    expect(isPulseOwnedLocalModelRoute('pulse:unknown')).toBe(false);
  });
});
