import { describe, expect, it } from 'vitest';
import {
  getNextAssistantRecentModelRoute,
  normalizeAssistantModelRouteArgument,
  normalizeAssistantRecentModelRoutes,
} from '../assistantModelRoutes';

// Branch-coverage companion to assistantModelRoutes.test.ts. Each test below
// drives a specific arm (early return, ternary branch, `??` default, optional
// chain, guard, modular wrap) of the three named functions and asserts against
// concrete outputs rather than truthiness.

const RECENTS: string[] = [
  'openrouter:qwen/qwen3.7-plus',
  'openrouter:deepseek/deepseek-v4-pro',
  'gemini:gemini-3.1-flash-lite',
];

// ---------------------------------------------------------------------------
// normalizeAssistantModelRouteArgument
// ---------------------------------------------------------------------------

describe('normalizeAssistantModelRouteArgument branch coverage', () => {
  it('passes an explicit route through after trimming surrounding whitespace', () => {
    expect(
      normalizeAssistantModelRouteArgument('  openrouter:qwen/qwen3.7-plus  ', ['openrouter']),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });

  it('returns null for an empty or whitespace-only argument (falsy candidate)', () => {
    expect(normalizeAssistantModelRouteArgument('', ['openrouter'])).toBeNull();
    expect(normalizeAssistantModelRouteArgument('   ', ['openrouter'])).toBeNull();
  });

  it('returns null when the candidate contains internal whitespace', () => {
    expect(normalizeAssistantModelRouteArgument('open router/model', ['openrouter'])).toBeNull();
  });

  it('returns null for url-like arguments containing ://', () => {
    expect(normalizeAssistantModelRouteArgument('http://example.com/m', ['http'])).toBeNull();
  });

  it('returns null when no slash separator is present', () => {
    expect(normalizeAssistantModelRouteArgument('plainname', ['plainname'])).toBeNull();
  });

  it('returns null when the slash is the first character', () => {
    expect(normalizeAssistantModelRouteArgument('/model', ['openrouter'])).toBeNull();
  });

  it('returns null when the slash is the last character', () => {
    expect(normalizeAssistantModelRouteArgument('openrouter/', ['openrouter'])).toBeNull();
  });

  it('returns null when the provider segment fails the provider regex', () => {
    expect(normalizeAssistantModelRouteArgument('123/model', ['123'])).toBeNull();
  });

  it('returns null when the model segment begins with a slash', () => {
    expect(normalizeAssistantModelRouteArgument('openrouter//qwen', ['openrouter'])).toBeNull();
  });

  it('matches known providers case-insensitively while preserving candidate provider case', () => {
    expect(normalizeAssistantModelRouteArgument('OpenRouter/model', ['openrouter'])).toBe(
      'OpenRouter:model',
    );
  });

  it('trims, lowercases, and drops blank entries in knownProviders before matching', () => {
    expect(
      normalizeAssistantModelRouteArgument('openrouter/model', [' OpenRouter ', '', '  ']),
    ).toBe('openrouter:model');
  });

  it('returns null when knownProviders is empty (default argument)', () => {
    expect(normalizeAssistantModelRouteArgument('openrouter/model')).toBeNull();
  });

  it('returns null when the provider is absent from knownProviders', () => {
    expect(normalizeAssistantModelRouteArgument('foo/model', ['openrouter'])).toBeNull();
  });

  it('builds the canonical route from a slash-style argument', () => {
    expect(
      normalizeAssistantModelRouteArgument('openrouter/qwen/qwen3.7-plus', ['openrouter']),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });
});

// ---------------------------------------------------------------------------
// normalizeAssistantRecentModelRoutes
// ---------------------------------------------------------------------------

describe('normalizeAssistantRecentModelRoutes branch coverage', () => {
  it('skips non-string entries via the typeof ternary', () => {
    expect(
      normalizeAssistantRecentModelRoutes(
        [
          123,
          { model: 'openrouter:qwen/qwen3.7-plus' },
          null,
          undefined,
          true,
          'openrouter:qwen/qwen3.7-plus',
        ],
        10,
      ),
    ).toEqual(['openrouter:qwen/qwen3.7-plus']);
  });

  it('skips blank and non-explicit strings and trims valid ones', () => {
    expect(
      normalizeAssistantRecentModelRoutes(
        ['', '   ', 'plain-model-name', ' openrouter:qwen/qwen3.7-plus '],
        10,
      ),
    ).toEqual(['openrouter:qwen/qwen3.7-plus']);
  });

  it('deduplicates explicit routes by their trimmed form', () => {
    expect(
      normalizeAssistantRecentModelRoutes(
        ['openrouter:qwen/qwen3.7-plus', 'openrouter:qwen/qwen3.7-plus', 'deepseek:deepseek-chat'],
        10,
      ),
    ).toEqual(['openrouter:qwen/qwen3.7-plus', 'deepseek:deepseek-chat']);
  });

  it('breaks as soon as the limit is reached, ignoring later valid routes', () => {
    expect(
      normalizeAssistantRecentModelRoutes(
        ['openrouter:qwen/qwen3.7-plus', 'deepseek:deepseek-chat', 'gemini:gemini-3.1-flash-lite'],
        2,
      ),
    ).toEqual(['openrouter:qwen/qwen3.7-plus', 'deepseek:deepseek-chat']);
  });

  it('returns every deduped route when the limit exceeds the input length', () => {
    expect(
      normalizeAssistantRecentModelRoutes(
        ['openrouter:qwen/qwen3.7-plus', 'deepseek:deepseek-chat'],
        99,
      ),
    ).toEqual(['openrouter:qwen/qwen3.7-plus', 'deepseek:deepseek-chat']);
  });

  it('returns an empty array for an empty input', () => {
    expect(normalizeAssistantRecentModelRoutes([], 5)).toEqual([]);
  });

  it('still yields the first entry when limit is 0 (cap evaluated after push)', () => {
    expect(normalizeAssistantRecentModelRoutes(['openrouter:qwen/qwen3.7-plus'], 0)).toEqual([
      'openrouter:qwen/qwen3.7-plus',
    ]);
  });
});

// ---------------------------------------------------------------------------
// getNextAssistantRecentModelRoute
// ---------------------------------------------------------------------------

describe('getNextAssistantRecentModelRoute branch coverage', () => {
  it('returns null when recentModelIds is empty', () => {
    expect(getNextAssistantRecentModelRoute({ recentModelIds: [] })).toBeNull();
  });

  it('returns null when every recent id is filtered out as non-explicit', () => {
    expect(
      getNextAssistantRecentModelRoute({ recentModelIds: ['plain-model-name', ''] }),
    ).toBeNull();
  });

  it('defaults direction to 1 and advances to the next route', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openrouter:qwen/qwen3.7-plus',
        recentModelIds: RECENTS,
      }),
    ).toBe('openrouter:deepseek/deepseek-v4-pro');
  });

  it('moves backward with direction -1 from the middle of the list', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openrouter:deepseek/deepseek-v4-pro',
        direction: -1,
        recentModelIds: RECENTS,
      }),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });

  it('wraps to the last route with direction -1 from the first index', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openrouter:qwen/qwen3.7-plus',
        direction: -1,
        recentModelIds: RECENTS,
      }),
    ).toBe('gemini:gemini-3.1-flash-lite');
  });

  it('wraps to the first route with direction 1 from the last index', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'gemini:gemini-3.1-flash-lite',
        direction: 1,
        recentModelIds: RECENTS,
      }),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });

  it('treats an undefined currentModel as empty and returns the first route', () => {
    expect(getNextAssistantRecentModelRoute({ recentModelIds: RECENTS })).toBe(
      'openrouter:qwen/qwen3.7-plus',
    );
  });

  it('treats a whitespace-only currentModel as empty (currentIndex < 0, direction 1)', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: '   ',
        recentModelIds: RECENTS,
      }),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });

  it('matches the current model after trimming surrounding whitespace', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: ' openrouter:qwen/qwen3.7-plus ',
        direction: 1,
        recentModelIds: RECENTS,
      }),
    ).toBe('openrouter:deepseek/deepseek-v4-pro');
  });

  it('returns the last route when current is absent and direction is -1', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openai:gpt-4o',
        direction: -1,
        recentModelIds: RECENTS,
      }),
    ).toBe('gemini:gemini-3.1-flash-lite');
  });

  it('returns null when the only recent route matches the current model', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openrouter:qwen/qwen3.7-plus',
        recentModelIds: ['openrouter:qwen/qwen3.7-plus'],
      }),
    ).toBeNull();
  });

  it('returns the sole recent route when current is absent and it is the only entry', () => {
    expect(
      getNextAssistantRecentModelRoute({
        currentModel: 'openai:gpt-4o',
        recentModelIds: ['openrouter:qwen/qwen3.7-plus'],
      }),
    ).toBe('openrouter:qwen/qwen3.7-plus');
  });
});
