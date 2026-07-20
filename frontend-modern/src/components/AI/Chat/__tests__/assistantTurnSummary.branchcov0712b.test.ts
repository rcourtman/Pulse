import { describe, expect, it } from 'vitest';
import { formatAssistantTurnDuration, getAssistantTurnSummary } from '../assistantTurnSummary';
import type { ChatMessage } from '../types';

// Branch-coverage companion to assistantTurnSummary.test.ts. Each test below
// drives a specific arm (early return, ternary branch, optional chain, `||`
// fallback, guard) of the three named functions and asserts against concrete
// outputs rather than truthiness. assistantTokenSummary is module-private, so
// every one of its branches is exercised through getAssistantTurnSummary and
// observed via the embedded label/title text (model + completedAt are omitted
// to isolate the token-summary output where useful).

const baseAssistant = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'asst-x',
  role: 'assistant',
  content: '',
  timestamp: new Date('2026-06-06T12:00:00Z'),
  isStreaming: false,
  ...overrides,
});

// ---------------------------------------------------------------------------
// formatAssistantTurnDuration
// ---------------------------------------------------------------------------

describe('formatAssistantTurnDuration branch coverage', () => {
  it("returns '' when completedAt is omitted (the !completedAt guard)", () => {
    expect(formatAssistantTurnDuration(new Date('2026-06-06T12:00:00Z'))).toBe('');
  });

  it("returns '' when the computed duration is non-finite (invalid completedAt -> NaN)", () => {
    // new Date('not-a-date') is an Invalid Date whose getTime() is NaN, so
    // durationMs is NaN and the !Number.isFinite arm fires.
    expect(
      formatAssistantTurnDuration(new Date('2026-06-06T12:00:00Z'), new Date('not-a-date')),
    ).toBe('');
  });

  it("formats a sub-minute duration as plain seconds ('Ns')", () => {
    expect(
      formatAssistantTurnDuration(
        new Date('2026-06-06T12:00:00Z'),
        new Date('2026-06-06T12:00:05Z'),
      ),
    ).toBe('5s');
  });

  it("formats an exact minute boundary with no seconds ('Xm', seconds ternary falsy arm)", () => {
    expect(
      formatAssistantTurnDuration(
        new Date('2026-06-06T12:00:00Z'),
        new Date('2026-06-06T12:02:00Z'),
      ),
    ).toBe('2m');
  });

  it("formats hours with leftover minutes ('Xh Ym', remainingMinutes ternary truthy arm)", () => {
    // 1h 2m 5s -> minutes 62, seconds 5 -> hours 1, remainingMinutes 2.
    expect(
      formatAssistantTurnDuration(
        new Date('2026-06-06T12:00:00Z'),
        new Date('2026-06-06T13:02:05Z'),
      ),
    ).toBe('1h 2m');
  });

  it("formats an exact hour with no leftover minutes ('Xh', remainingMinutes ternary falsy arm)", () => {
    // exactly 3600s -> minutes 60, seconds 0 -> hours 1, remainingMinutes 0.
    expect(
      formatAssistantTurnDuration(
        new Date('2026-06-06T12:00:00Z'),
        new Date('2026-06-06T13:00:00Z'),
      ),
    ).toBe('1h');
  });
});

// ---------------------------------------------------------------------------
// assistantTokenSummary (module-private — reached via getAssistantTurnSummary).
// model + completedAt omitted so label/title carry only the token summary.
// ---------------------------------------------------------------------------

describe('assistantTokenSummary branch coverage', () => {
  it("emits the singular 'token' word when the total is exactly 1", () => {
    expect(getAssistantTurnSummary(baseAssistant({ tokens: { input: 0, output: 1 } }))).toEqual({
      label: 'Last turn: 1 token',
      title: 'Last assistant turn summary: Usage: 1 total, 0 input, 1 output',
    });
  });

  it('clamps the context percentage to 100 when input exceeds the limit (Math.min arm)', () => {
    expect(
      getAssistantTurnSummary(
        baseAssistant({ tokens: { input: 50000, output: 100, contextLimit: 100 } }),
      ),
    ).toEqual({
      label: 'Last turn: 50,100 tokens (100% of context)',
      title:
        'Last assistant turn summary: Usage: 50,100 total, 50,000 input, 100 output, context 50,000 of 100 (100%)',
    });
  });

  it('drops the context detail when input is 0 even with a contextLimit present (input>0 false arm)', () => {
    expect(
      getAssistantTurnSummary(
        baseAssistant({ tokens: { input: 0, output: 5, contextLimit: 1000 } }),
      ),
    ).toEqual({
      label: 'Last turn: 5 tokens',
      title: 'Last assistant turn summary: Usage: 5 total, 0 input, 5 output',
    });
  });

  it('drops the context detail when contextLimit is 0 (contextLimit>0 false arm)', () => {
    expect(
      getAssistantTurnSummary(baseAssistant({ tokens: { input: 10, output: 5, contextLimit: 0 } })),
    ).toEqual({
      label: 'Last turn: 15 tokens',
      title: 'Last assistant turn summary: Usage: 15 total, 10 input, 5 output',
    });
  });
});

// ---------------------------------------------------------------------------
// getAssistantTurnSummary
// ---------------------------------------------------------------------------

describe('getAssistantTurnSummary branch coverage', () => {
  it('falls back to the raw model id when no getModelRouteLabel option is supplied (?. absent + || model)', () => {
    expect(
      getAssistantTurnSummary(
        baseAssistant({
          model: 'openrouter:qwen/qwen3.7-plus',
          tokens: { input: 10, output: 5 },
        }),
      ),
    ).toEqual({
      label: 'Last turn: openrouter:qwen/qwen3.7-plus · 15 tokens',
      title:
        'Last assistant turn summary: Model: openrouter:qwen/qwen3.7-plus. Usage: 15 total, 10 input, 5 output',
    });
  });

  it('falls back to the raw model id when getModelRouteLabel returns an empty string (|| model falsy arm)', () => {
    expect(
      getAssistantTurnSummary(
        baseAssistant({ model: 'direct:local', tokens: { input: 2, output: 3 } }),
        { getModelRouteLabel: () => '' },
      ),
    ).toEqual({
      label: 'Last turn: direct:local · 5 tokens',
      title: 'Last assistant turn summary: Model: direct:local. Usage: 5 total, 2 input, 3 output',
    });
  });

  it("omits the model part when model is whitespace-only (model?.trim()='' -> model falsy -> '')", () => {
    expect(
      getAssistantTurnSummary(
        baseAssistant({
          model: '   ',
          completedAt: new Date('2026-06-06T12:00:03Z'),
          tokens: { input: 1, output: 1 },
        }),
        { getModelRouteLabel: () => 'Should Not Appear' },
      ),
    ).toEqual({
      label: 'Last turn: 3s · 2 tokens',
      title: 'Last assistant turn summary: Duration: 3s. Usage: 2 total, 1 input, 1 output',
    });
  });

  it('omits the model part when model is absent (message.model?. absent short-circuit)', () => {
    expect(
      getAssistantTurnSummary(
        baseAssistant({
          completedAt: new Date('2026-06-06T12:00:03Z'),
          tokens: { input: 10, output: 5 },
        }),
      ),
    ).toEqual({
      label: 'Last turn: 3s · 15 tokens',
      title: 'Last assistant turn summary: Duration: 3s. Usage: 15 total, 10 input, 5 output',
    });
  });

  it('returns null when there is nothing to summarize (no model, no completedAt, tokens?. absent -> output<=0)', () => {
    // Drives: message.tokens?. short-circuit in assistantTokenSummary, the
    // output<=0 -> null guard, and the labelParts.length===0 -> null guard.
    expect(getAssistantTurnSummary(baseAssistant())).toBeNull();
  });
});
