import { describe, expect, it } from 'vitest';
import {
  AI_PROVIDER_DISPLAY_NAMES,
  getAIProviderDisplayName,
  getProviderFromModelId,
} from '@/utils/aiProviderPresentation';

describe('aiProviderPresentation', () => {
  it('returns canonical provider display names', () => {
    expect(AI_PROVIDER_DISPLAY_NAMES.openai).toBe('OpenAI');
    expect(getAIProviderDisplayName('anthropic')).toBe('Anthropic');
    expect(getAIProviderDisplayName('gemini')).toBe('Google Gemini');
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
});
