import { describe, expect, it } from 'vitest';
import {
  getNextAssistantRecentModelRoute,
  isAssistantExplicitModelRoute,
  normalizeAssistantRecentModelRoutes,
} from '../assistantModelRoutes';

describe('assistantModelRoutes', () => {
  it('accepts provider-qualified model routes only', () => {
    expect(isAssistantExplicitModelRoute('openrouter:qwen/qwen3.7-plus')).toBe(true);
    expect(isAssistantExplicitModelRoute('deepseek:deepseek-chat')).toBe(true);
    expect(isAssistantExplicitModelRoute('plain-model-name')).toBe(false);
    expect(isAssistantExplicitModelRoute('openrouter:')).toBe(false);
    expect(isAssistantExplicitModelRoute(':qwen/qwen3.7-plus')).toBe(false);
    expect(isAssistantExplicitModelRoute('https://openrouter.ai/models/qwen')).toBe(false);
    expect(isAssistantExplicitModelRoute('openrouter:/qwen/qwen3.7-plus')).toBe(false);
  });

  it('normalizes recent routes by trimming, filtering, deduping, and capping', () => {
    expect(
      normalizeAssistantRecentModelRoutes(
        [
          ' openrouter:qwen/qwen3.7-plus ',
          'plain-model-name',
          'deepseek:deepseek-chat',
          'openrouter:qwen/qwen3.7-plus',
          'gemini:gemini-3.1-flash-lite',
        ],
        2,
      ),
    ).toEqual(['openrouter:qwen/qwen3.7-plus', 'deepseek:deepseek-chat']);
  });

  it('cycles to the next recent model without reordering the list', () => {
    const recentModelIds = [
      'openrouter:qwen/qwen3.7-plus',
      'openrouter:deepseek/deepseek-v4-pro',
      'gemini:gemini-3.1-flash-lite',
    ];

    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openrouter:qwen/qwen3.7-plus',
        recentModelIds,
      }),
    ).toBe('openrouter:deepseek/deepseek-v4-pro');
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'gemini:gemini-3.1-flash-lite',
        recentModelIds,
      }),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });

  it('selects the first recent route when the current model is outside recents', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openai:gpt-4o',
        recentModelIds: ['openrouter:qwen/qwen3.7-plus'],
      }),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });

  it('returns null when the only recent route is already active', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openrouter:qwen/qwen3.7-plus',
        recentModelIds: ['openrouter:qwen/qwen3.7-plus'],
      }),
    ).toBeNull();
  });
});
