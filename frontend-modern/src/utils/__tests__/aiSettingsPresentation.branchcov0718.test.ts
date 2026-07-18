import { describe, expect, it } from 'vitest';

import {
  getAICredentialsClearErrorMessage,
  getAIChatSessionsLoadErrorMessage,
  getAIModelsLoadErrorMessage,
  getAISessionSummarizeErrorMessage,
  getAISettingsReadinessPresentation,
  getAISettingsSaveErrorMessage,
  getAISettingsToggleErrorMessage,
} from '@/utils/aiSettingsPresentation';

// Supplemental branch-coverage suite. The sibling aiSettingsPresentation.test.ts
// already pins the canonical copy and exercises the happy arms of every error
// helper (`undefined` and a populated detail string). This file targets the
// *residual* arms:
//   * getAISettingsReadinessPresentation — the singular `providerCount === 1`
//     arm of the `providerCount !== 1 ? 's' : ''` ternary, plus the
//     `configured === false` short-circuit when non-zero counts are supplied,
//     and the zero/one model-count boundary.
//   * The optional-message error helpers — null, empty-string and
//     whitespace-only inputs that route through `(message || '').trim()` to the
//     fallback (`detail || fallback`) arm, plus the trimmed-non-empty arm and,
//     for getAISettingsSaveErrorMessage, the custom-fallback + truthy-message
//     pairing.

describe('getAISettingsReadinessPresentation — branch coverage', () => {
  it('singularises "provider" when configured with exactly one provider (false arm of !== 1)', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: true,
        providerCount: 1,
        modelCount: 5,
      }),
    ).toEqual({
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: 'Ready • 1 provider • 5 models',
    });
  });

  it('keeps the singular form alongside a zero model-count boundary', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: true,
        providerCount: 1,
        modelCount: 0,
      }),
    ).toEqual({
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: 'Ready • 1 provider • 0 models',
    });
  });

  it('pluralises when configured with zero providers (0 !== 1 truthy arm boundary)', () => {
    // providerCount === 0 still satisfies `!== 1`, exercising the truthy arm at
    // its lower boundary. Documents that this branch keys only off unity.
    expect(
      getAISettingsReadinessPresentation({
        configured: true,
        providerCount: 0,
        modelCount: 3,
      }),
    ).toEqual({
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: 'Ready • 0 providers • 3 models',
    });
  });

  it('renders the unit model count without singularising the model noun', () => {
    // The model count is interpolated verbatim (no pluralisation logic); locks
    // the boundary so a future "models" → "model" tweak cannot pass silently.
    expect(
      getAISettingsReadinessPresentation({
        configured: true,
        providerCount: 3,
        modelCount: 1,
      }),
    ).toEqual({
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: 'Ready • 3 providers • 1 models',
    });
  });

  it('short-circuits to the not-configured presentation even when provider/model counts are non-zero', () => {
    // The `if (configured)` false arm must ignore the supplied counts entirely;
    // a configured=false state with stray positive counts still yields the
    // amber "Configure at least one provider..." copy.
    expect(
      getAISettingsReadinessPresentation({
        configured: false,
        providerCount: 4,
        modelCount: 9,
      }),
    ).toEqual({
      containerClassName: 'bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200',
      dotClassName: 'bg-amber-400',
      summary: 'Configure at least one provider above to enable Pulse Assistant and Patrol.',
    });
  });
});

describe('getAIModelsLoadErrorMessage — branch coverage', () => {
  it('falls back when message is null (null arm of `message || ""`)', () => {
    expect(getAIModelsLoadErrorMessage(null)).toBe('Unable to load models.');
  });

  it('falls back when message is the empty string (trim() === "" arm)', () => {
    expect(getAIModelsLoadErrorMessage('')).toBe('Unable to load models.');
  });

  it('falls back when message is whitespace-only (trim() collapses to "")', () => {
    expect(getAIModelsLoadErrorMessage('   \t  ')).toBe('Unable to load models.');
  });

  it('returns the trimmed detail when the input has surrounding whitespace (truthy trim() arm)', () => {
    expect(getAIModelsLoadErrorMessage('  Network request failed  ')).toBe(
      'Network request failed',
    );
  });
});

describe('getAIChatSessionsLoadErrorMessage — branch coverage', () => {
  it('falls back when message is null', () => {
    expect(getAIChatSessionsLoadErrorMessage(null)).toBe('Unable to load chat sessions.');
  });

  it('falls back when message is the empty string', () => {
    expect(getAIChatSessionsLoadErrorMessage('')).toBe('Unable to load chat sessions.');
  });

  it('falls back when message is whitespace-only', () => {
    expect(getAIChatSessionsLoadErrorMessage('\n\t ')).toBe('Unable to load chat sessions.');
  });

  it('returns the trimmed detail for a padded non-empty message', () => {
    expect(getAIChatSessionsLoadErrorMessage('  Session API offline  ')).toBe(
      'Session API offline',
    );
  });
});

describe('getAISessionSummarizeErrorMessage — branch coverage', () => {
  it('falls back when message is null', () => {
    expect(getAISessionSummarizeErrorMessage(null)).toBe('Unable to summarize the session.');
  });

  it('falls back when message is the empty string', () => {
    expect(getAISessionSummarizeErrorMessage('')).toBe('Unable to summarize the session.');
  });

  it('falls back when message is whitespace-only', () => {
    expect(getAISessionSummarizeErrorMessage('   ')).toBe('Unable to summarize the session.');
  });

  it('returns the trimmed detail for a padded non-empty message', () => {
    expect(getAISessionSummarizeErrorMessage('  provider offline  ')).toBe('provider offline');
  });
});

describe('getAISettingsSaveErrorMessage — branch coverage', () => {
  it('falls back to the default fallback when message is null', () => {
    expect(getAISettingsSaveErrorMessage(null)).toBe(
      'Unable to save Provider & Models settings.',
    );
  });

  it('falls back to the default fallback when message is the empty string', () => {
    expect(getAISettingsSaveErrorMessage('')).toBe(
      'Unable to save Provider & Models settings.',
    );
  });

  it('falls back to the default fallback when message is whitespace-only', () => {
    expect(getAISettingsSaveErrorMessage('  \t  ')).toBe(
      'Unable to save Provider & Models settings.',
    );
  });

  it('honours the custom fallback when message is null (right operand of `detail || fallback`)', () => {
    expect(getAISettingsSaveErrorMessage(null, 'Unable to save Patrol settings.')).toBe(
      'Unable to save Patrol settings.',
    );
  });

  it('honours the custom fallback when message is whitespace-only', () => {
    expect(getAISettingsSaveErrorMessage('   ', 'Unable to save Patrol settings.')).toBe(
      'Unable to save Patrol settings.',
    );
  });

  it('prefers a truthy trimmed message over the custom fallback (left operand wins)', () => {
    // Confirms the `detail || fallback` ordering: a real error message always
    // wins over any caller-supplied fallback string.
    expect(
      getAISettingsSaveErrorMessage('  bad request  ', 'Unable to save Patrol settings.'),
    ).toBe('bad request');
  });
});

describe('getAICredentialsClearErrorMessage — branch coverage', () => {
  it('falls back when message is null', () => {
    expect(getAICredentialsClearErrorMessage(null)).toBe('Unable to clear credentials.');
  });

  it('falls back when message is the empty string', () => {
    expect(getAICredentialsClearErrorMessage('')).toBe('Unable to clear credentials.');
  });

  it('falls back when message is whitespace-only', () => {
    expect(getAICredentialsClearErrorMessage('\t\n  ')).toBe('Unable to clear credentials.');
  });

  it('returns the trimmed detail for a padded non-empty message', () => {
    expect(getAICredentialsClearErrorMessage('  permission denied  ')).toBe(
      'permission denied',
    );
  });
});

describe('getAISettingsToggleErrorMessage — branch coverage', () => {
  it('falls back when message is null', () => {
    expect(getAISettingsToggleErrorMessage(null)).toBe(
      'Unable to update Pulse Intelligence settings.',
    );
  });

  it('falls back when message is the empty string', () => {
    expect(getAISettingsToggleErrorMessage('')).toBe(
      'Unable to update Pulse Intelligence settings.',
    );
  });

  it('falls back when message is whitespace-only', () => {
    expect(getAISettingsToggleErrorMessage('   ')).toBe(
      'Unable to update Pulse Intelligence settings.',
    );
  });

  it('returns the trimmed detail for a padded non-empty message', () => {
    expect(getAISettingsToggleErrorMessage('  rate limited  ')).toBe('rate limited');
  });
});
