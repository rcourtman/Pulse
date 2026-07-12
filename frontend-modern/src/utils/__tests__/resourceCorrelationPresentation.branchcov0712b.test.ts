import { describe, expect, it } from 'vitest';

import type { ResourceCorrelation } from '@/types/aiIntelligence';
import type { ResourceRelationship } from '@/types/resource';
import {
  formatResourceCorrelationSummary,
  sortResourceCorrelations,
  sortResourceRelationships,
} from '@/utils/resourceCorrelationPresentation';

// Branch-coverage companion to resourceCorrelationPresentation.branchcov.test.ts.
// Focuses exclusively on the still-under-exercised branches of:
//   - parseGoDurationMs (private; reached via formatResourceCorrelationSummary
//     when `avg_delay` is a Go duration string)
//   - sortResourceCorrelations
//   - sortResourceRelationships
const makeCorrelation = (
  overrides: Partial<ResourceCorrelation> = {},
): ResourceCorrelation => ({
  source_id: 'storage-1',
  source_name: 'Storage 1',
  source_type: 'storage',
  target_id: 'vm-42',
  target_name: 'VM 42',
  target_type: 'vm',
  event_pattern: 'disk_full -> restart',
  occurrences: 2,
  avg_delay: 0,
  confidence: NaN,
  last_seen: '2026-03-18T12:00:00Z',
  description: 'Disk pressure often precedes restarts',
  ...overrides,
});

const makeRelationship = (
  overrides: Partial<ResourceRelationship> = {},
): ResourceRelationship => ({
  sourceId: 'node:pve-1',
  targetId: 'vm-42',
  type: 'runs_on',
  confidence: 0.98,
  active: true,
  discoverer: 'proxmox_adapter',
  observedAt: '2026-03-18T12:00:00Z',
  lastSeenAt: '2026-03-18T12:05:00Z',
  ...overrides,
});

// parseGoDurationMs is a private (non-exported) function in the module. It is
// reachable through formatResourceCorrelationSummary, which delegates to it
// whenever `correlation.avg_delay` is a string rather than a number.
describe('parseGoDurationMs (via formatResourceCorrelationSummary)', () => {
  // Uses occurrences: 2 and confidence: NaN so the only varying summary part is
  // the formatted delay, making assertions unambiguous.

  it('returns null (omitting delay) for a whitespace-only string via the empty-normalized early return', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '   ' })),
    ).toBe('2 occurrences');
  });

  it('trims surrounding whitespace before parsing', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '  5s  ' })),
    ).toBe('2 occurrences · avg delay 5s');
  });

  it('parses the h (hour) unit', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '2h' })),
    ).toBe('2 occurrences · avg delay 120m');
  });

  it('parses a fractional amount using the decimal part of the duration pattern', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '1.0s' })),
    ).toBe('2 occurrences · avg delay 1s');
  });

  it('parses the us (microseconds) unit', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '2000us' })),
    ).toBe('2 occurrences · avg delay 2ms');
  });

  it('parses the µs (unicode microseconds) unit', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '2000µs' })),
    ).toBe('2 occurrences · avg delay 2ms');
  });

  it('parses the ns (nanoseconds) unit', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ avg_delay: '3000000ns' }),
      ),
    ).toBe('2 occurrences · avg delay 3ms');
  });

  it('accumulates multiple unit tokens in a single duration string', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '1h30m' })),
    ).toBe('2 occurrences · avg delay 90m');
  });

  it('returns null when the matched amount is not finite (defensive continue branch)', () => {
    // 400 nines overflow Number.parseFloat to Infinity, exercising the
    // `if (!Number.isFinite(amount)) continue;` branch.
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ avg_delay: `${'9'.repeat(400)}s` }),
      ),
    ).toBe('2 occurrences');
  });

  it('returns null when the accumulated total overflows to Infinity', () => {
    // A finite amount (1e300) multiplied by the hour factor overflows totalNs
    // to Infinity, exercising the `!Number.isFinite(totalNs)` arm of the
    // terminal guard.
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ avg_delay: `${'9'.repeat(300)}h` }),
      ),
    ).toBe('2 occurrences');
  });

  it('returns null when the parsed duration is zero (totalNs <= 0 guard)', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '0s' })),
    ).toBe('2 occurrences');
  });

  it('returns null when no unit token matches at all (!matched guard)', () => {
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: 'abc' })),
    ).toBe('2 occurrences');
  });

  it('documents the current behaviour that an `ms` token is consumed as minutes', () => {
    // The duration regex alternation is `h|m|s|ms|us|µs|ns`; because `m`
    // precedes `ms`, the input `5ms` matches the `m` alternative first (as
    // `5m`) and the trailing `s` is left unparsed. The result is therefore
    // 5 minutes, not 5 milliseconds. This is reported as a suspected source
    // bug; the assertion pins the actual current behaviour.
    expect(
      formatResourceCorrelationSummary(makeCorrelation({ avg_delay: '5ms' })),
    ).toBe('2 occurrences · avg delay 5m');
  });
});

describe('sortResourceCorrelations (branch coverage)', () => {
  it('sorts by descending confidence, then descending last_seen, coalescing undefined/NaN confidence to 0', () => {
    const sorted = sortResourceCorrelations([
      makeCorrelation({
        source_id: 'A-mid',
        confidence: 0.9,
        last_seen: '2026-06-01T00:00:00Z',
      }),
      makeCorrelation({
        source_id: 'B-undef',
        confidence: undefined as unknown as number,
        last_seen: '2026-05-01T00:00:00Z',
      }),
      makeCorrelation({
        source_id: 'C-nan-invalid-date',
        confidence: NaN,
        last_seen: 'garbage',
      }),
      makeCorrelation({
        source_id: 'D-high-conf-newer',
        confidence: 0.9,
        last_seen: '2026-07-01T00:00:00Z',
      }),
    ]);

    // Confidence tiers: 0.9 (A, D) > 0 (B undefined, C NaN).
    // Within 0.9: D (2026-07) is newer than A (2026-06).
    // Within 0: B has a finite date (2026-05) while C's date is invalid (-> 0).
    expect(sorted.map((c) => c.source_id)).toEqual([
      'D-high-conf-newer',
      'A-mid',
      'B-undef',
      'C-nan-invalid-date',
    ]);
  });

  it('treats both invalid last_seen dates as time zero, preserving input order on a full tie', () => {
    const sorted = sortResourceCorrelations([
      makeCorrelation({ source_id: 'first', confidence: 0.4, last_seen: 'nope' }),
      makeCorrelation({
        source_id: 'second',
        confidence: 0.4,
        last_seen: 'also-nope',
      }),
    ]);
    expect(sorted.map((c) => c.source_id)).toEqual(['first', 'second']);
  });

  it('returns a new array instance (does not return the input reference)', () => {
    const input: readonly ResourceCorrelation[] = [
      makeCorrelation({ source_id: 'only', confidence: 0.1 }),
    ];
    const sorted = sortResourceCorrelations(input);
    expect(sorted).not.toBe(input);
    expect(sorted.map((c) => c.source_id)).toEqual(['only']);
  });
});

describe('sortResourceRelationships (branch coverage)', () => {
  it('keeps an already-active-first pair stable via the left.active truthy (-1) arm', () => {
    // Input order is [active, inactive]; for a two-element array the comparator
    // is invoked as (active, inactive), so left.active is truthy and the
    // `left.active ? -1 : 1` expression evaluates the -1 branch.
    const sorted = sortResourceRelationships([
      makeRelationship({ sourceId: 'live', active: true, confidence: 0.1 }),
      makeRelationship({ sourceId: 'sleeping', active: false, confidence: 1 }),
    ]);
    expect(sorted.map((r) => r.sourceId)).toEqual(['live', 'sleeping']);
  });

  it('treats an undefined active value as falsy, sorting it below a true-active relationship', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({
        sourceId: 'undefined-active',
        active: undefined as unknown as boolean,
        confidence: 1,
      }),
      makeRelationship({ sourceId: 'true-active', active: true, confidence: 0 }),
    ]);
    expect(sorted.map((r) => r.sourceId)).toEqual([
      'true-active',
      'undefined-active',
    ]);
  });

  it('sorts active before inactive, then by confidence, then by observedAt fallback', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({
        sourceId: 'A-undef-conf',
        active: true,
        confidence: undefined as unknown as number,
        lastSeenAt: '2026-06-01T00:00:00Z',
        observedAt: '2026-06-01T00:00:00Z',
      }),
      makeRelationship({
        sourceId: 'B-nan-conf-observed',
        active: true,
        confidence: NaN,
        lastSeenAt: '',
        observedAt: '2026-05-01T00:00:00Z',
      }),
      makeRelationship({
        sourceId: 'C-high-conf-bad-date',
        active: true,
        confidence: 0.9,
        lastSeenAt: 'garbage',
        observedAt: 'garbage',
      }),
      makeRelationship({
        sourceId: 'D-inactive',
        active: false,
        confidence: 1,
      }),
    ]);

    // Active (A, B, C) before inactive (D).
    // Among active: confidence tiers 0.9 (C) > 0 (A undefined, B NaN).
    // A vs B tie at 0: A uses lastSeenAt 2026-06; B falls back to observedAt
    // 2026-05. A is newer. C's date is invalid -> 0 but it already wins on
    // confidence.
    expect(sorted.map((r) => r.sourceId)).toEqual([
      'C-high-conf-bad-date',
      'A-undef-conf',
      'B-nan-conf-observed',
      'D-inactive',
    ]);
  });

  it('falls back to observedAt and then to time zero across a three-way time tie-break', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({
        sourceId: 'no-times',
        active: true,
        confidence: 0.5,
        lastSeenAt: '',
        observedAt: '',
      }),
      makeRelationship({
        sourceId: 'observed-only',
        active: true,
        confidence: 0.5,
        lastSeenAt: '',
        observedAt: '2026-04-01T00:00:00Z',
      }),
      makeRelationship({
        sourceId: 'last-seen',
        active: true,
        confidence: 0.5,
        lastSeenAt: '2026-05-01T00:00:00Z',
        observedAt: '2020-01-01T00:00:00Z',
      }),
    ]);
    // All active, all confidence 0.5 -> time tie-break, newest first.
    // last-seen (2026-05) > observed-only (2026-04) > no-times (0).
    expect(sorted.map((r) => r.sourceId)).toEqual([
      'last-seen',
      'observed-only',
      'no-times',
    ]);
  });

  it('returns a new array instance (does not return the input reference)', () => {
    const input: readonly ResourceRelationship[] = [
      makeRelationship({ sourceId: 'only', active: true, confidence: 0.1 }),
    ];
    const sorted = sortResourceRelationships(input);
    expect(sorted).not.toBe(input);
    expect(sorted.map((r) => r.sourceId)).toEqual(['only']);
  });
});
