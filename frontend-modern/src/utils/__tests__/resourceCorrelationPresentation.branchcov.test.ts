import { describe, expect, it } from 'vitest';

import type { ResourceCorrelation } from '@/types/aiIntelligence';
import type { ResourceRelationship } from '@/types/resource';
import {
  formatResourceCorrelationEndpoint,
  formatResourceCorrelationPattern,
  formatResourceCorrelationSummary,
  formatResourceRelationshipEndpoint,
  formatResourceRelationshipSummary,
  sortResourceCorrelations,
  sortResourceRelationships,
} from '@/utils/resourceCorrelationPresentation';

const makeCorrelation = (overrides: Partial<ResourceCorrelation> = {}): ResourceCorrelation => ({
  source_id: 'storage-1',
  source_name: 'Storage 1',
  source_type: 'storage',
  target_id: 'vm-42',
  target_name: 'VM 42',
  target_type: 'vm',
  event_pattern: 'disk_full -> restart',
  occurrences: 2,
  avg_delay: 5_000_000_000,
  confidence: 0.875,
  last_seen: '2026-03-18T12:00:00Z',
  description: 'Disk pressure often precedes restarts',
  ...overrides,
});

const makeRelationship = (overrides: Partial<ResourceRelationship> = {}): ResourceRelationship => ({
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

// humanizeCorrelationToken is a private (non-exported) function in the module.
// It is reachable through formatResourceCorrelationPattern, which delegates
// directly: humanizeCorrelationToken(correlation.event_pattern).
describe('humanizeCorrelationToken (via formatResourceCorrelationPattern)', () => {
  it('returns the fallback label when event_pattern is undefined', () => {
    expect(
      formatResourceCorrelationPattern(
        makeCorrelation({ event_pattern: undefined as unknown as string }),
      ),
    ).toBe('Correlation');
  });

  it('returns the fallback label when event_pattern is empty', () => {
    expect(formatResourceCorrelationPattern(makeCorrelation({ event_pattern: '' }))).toBe(
      'Correlation',
    );
  });

  it('returns the fallback label when event_pattern is whitespace-only', () => {
    expect(formatResourceCorrelationPattern(makeCorrelation({ event_pattern: '   ' }))).toBe(
      'Correlation',
    );
  });

  it('lowercases an all-caps token (with A-Z letters) before humanizing', () => {
    expect(formatResourceCorrelationPattern(makeCorrelation({ event_pattern: 'API_ERROR' }))).toBe(
      'Api Error',
    );
  });

  it('keeps a digits-only token untouched (no A-Z, so the lowercase arm is skipped)', () => {
    expect(formatResourceCorrelationPattern(makeCorrelation({ event_pattern: '404' }))).toBe('404');
  });
});

describe('formatResourceCorrelationEndpoint (branch coverage)', () => {
  it('falls back to source_id when source_name is empty', () => {
    expect(
      formatResourceCorrelationEndpoint(
        makeCorrelation({ source_name: '', source_id: 'sid-9' }),
        'source',
      ),
    ).toBe('sid-9');
  });

  it('falls back to target_id when target_name is empty', () => {
    expect(
      formatResourceCorrelationEndpoint(
        makeCorrelation({ target_name: '', target_id: 'tid-7' }),
        'target',
      ),
    ).toBe('tid-7');
  });

  it('returns Unknown resource when source name and id are both whitespace-only', () => {
    expect(
      formatResourceCorrelationEndpoint(
        makeCorrelation({ source_name: '  ', source_id: ' \t' }),
        'source',
      ),
    ).toBe('Unknown resource');
  });

  it('returns Unknown resource when target name and id are both empty', () => {
    expect(
      formatResourceCorrelationEndpoint(
        makeCorrelation({ target_name: '', target_id: '' }),
        'target',
      ),
    ).toBe('Unknown resource');
  });

  it('trims whitespace around a resolved source name', () => {
    expect(
      formatResourceCorrelationEndpoint(makeCorrelation({ source_name: '  Web Node  ' }), 'source'),
    ).toBe('Web Node');
  });
});

describe('formatResourceCorrelationSummary (branch coverage)', () => {
  it('uses the singular occurrence when occurrences is exactly 1', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({
          occurrences: 1,
          avg_delay: 0,
          confidence: undefined as unknown as number,
        }),
      ),
    ).toBe('1 occurrence');
  });

  it('omits the occurrence part when occurrences is 0', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ occurrences: 0, avg_delay: 0, confidence: 0.5 }),
      ),
    ).toBe('50% confidence');
  });

  it('parses avg_delay from a Go duration string', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ occurrences: 3, avg_delay: '5s', confidence: 0.9 }),
      ),
    ).toBe('3 occurrences · avg delay 5s · 90% confidence');
  });

  it('omits the delay when the Go duration string is unparseable', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ occurrences: 1, avg_delay: 'not-a-duration', confidence: NaN }),
      ),
    ).toBe('1 occurrence');
  });

  it('omits the delay when a numeric avg_delay rounds to zero', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ occurrences: 2, avg_delay: 1, confidence: NaN }),
      ),
    ).toBe('2 occurrences');
  });

  it('omits confidence when it is not finite and parses a minute duration string', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({ occurrences: 1, avg_delay: '1m', confidence: Infinity }),
      ),
    ).toBe('1 occurrence · avg delay 1m');
  });

  it('returns an empty string when no parts apply', () => {
    expect(
      formatResourceCorrelationSummary(
        makeCorrelation({
          occurrences: 0,
          avg_delay: '',
          confidence: undefined as unknown as number,
        }),
      ),
    ).toBe('');
  });
});

describe('sortResourceCorrelations (branch coverage)', () => {
  it('returns a new empty array for empty input', () => {
    expect(sortResourceCorrelations([])).toEqual([]);
  });

  it('does not mutate the original array', () => {
    const input = [
      makeCorrelation({ source_id: 'low', confidence: 0.1 }),
      makeCorrelation({ source_id: 'high', confidence: 0.9 }),
    ];
    sortResourceCorrelations(input);
    expect(input.map((c) => c.source_id)).toEqual(['low', 'high']);
  });

  it('treats zero confidence as 0 in the confidence comparison', () => {
    const sorted = sortResourceCorrelations([
      makeCorrelation({ source_id: 'zero', confidence: 0 }),
      makeCorrelation({ source_id: 'high', confidence: 0.5 }),
    ]);
    expect(sorted.map((c) => c.source_id)).toEqual(['high', 'zero']);
  });

  it('breaks a confidence tie using last_seen recency', () => {
    const sorted = sortResourceCorrelations([
      makeCorrelation({
        source_id: 'older',
        confidence: 0.5,
        last_seen: '2026-01-01T00:00:00Z',
      }),
      makeCorrelation({
        source_id: 'newer',
        confidence: 0.5,
        last_seen: '2026-06-01T00:00:00Z',
      }),
    ]);
    expect(sorted.map((c) => c.source_id)).toEqual(['newer', 'older']);
  });

  it('treats a missing last_seen as time zero when confidence ties', () => {
    const sorted = sortResourceCorrelations([
      makeCorrelation({ source_id: 'no-date', confidence: 0.5, last_seen: '' }),
      makeCorrelation({
        source_id: 'has-date',
        confidence: 0.5,
        last_seen: '2026-06-01T00:00:00Z',
      }),
    ]);
    expect(sorted.map((c) => c.source_id)).toEqual(['has-date', 'no-date']);
  });

  it('keeps relative order when confidence and dates are equivalent (both invalid)', () => {
    const sorted = sortResourceCorrelations([
      makeCorrelation({ source_id: 'a', confidence: 0.5, last_seen: 'not-a-date' }),
      makeCorrelation({ source_id: 'b', confidence: 0.5, last_seen: 'also-bad' }),
    ]);
    expect(sorted.map((c) => c.source_id)).toEqual(['a', 'b']);
  });
});

describe('formatResourceRelationshipEndpoint (branch coverage)', () => {
  it('returns Unknown resource when sourceId is empty', () => {
    expect(formatResourceRelationshipEndpoint(makeRelationship({ sourceId: '' }), 'source')).toBe(
      'Unknown resource',
    );
  });

  it('returns Unknown resource when targetId is whitespace-only', () => {
    expect(
      formatResourceRelationshipEndpoint(makeRelationship({ targetId: '   ' }), 'target'),
    ).toBe('Unknown resource');
  });

  it('trims a resolved targetId', () => {
    expect(
      formatResourceRelationshipEndpoint(makeRelationship({ targetId: '  vm-99  ' }), 'target'),
    ).toBe('vm-99');
  });
});

describe('formatResourceRelationshipSummary (branch coverage)', () => {
  it('omits confidence when it is NaN and shows discoverer and Historical', () => {
    expect(
      formatResourceRelationshipSummary(
        makeRelationship({ confidence: NaN, active: false, discoverer: 'k8s_adapter' }),
      ),
    ).toBe('K8s Adapter · Historical');
  });

  it('filters out the fallback discoverer label when discoverer is empty', () => {
    expect(
      formatResourceRelationshipSummary(makeRelationship({ confidence: 0.5, discoverer: '' })),
    ).toBe('50% confidence');
  });

  it('filters out the fallback discoverer label when discoverer is undefined', () => {
    expect(
      formatResourceRelationshipSummary(
        makeRelationship({
          confidence: 0.5,
          discoverer: undefined as unknown as string,
          active: true,
        }),
      ),
    ).toBe('50% confidence');
  });

  it('humanizes an all-caps discoverer via the lowercase arm', () => {
    expect(
      formatResourceRelationshipSummary(makeRelationship({ confidence: 0.4, discoverer: 'SNMP' })),
    ).toBe('40% confidence · Snmp');
  });

  it('does not add Historical when active is true', () => {
    expect(
      formatResourceRelationshipSummary(
        makeRelationship({ confidence: 0.5, active: true, discoverer: 'snmp' }),
      ),
    ).toBe('50% confidence · Snmp');
  });

  it('does not add Historical when active is not strictly false', () => {
    expect(
      formatResourceRelationshipSummary(
        makeRelationship({
          confidence: undefined as unknown as number,
          active: undefined as unknown as boolean,
          discoverer: '',
        }),
      ),
    ).toBe('');
  });
});

describe('sortResourceRelationships (branch coverage)', () => {
  it('returns a new empty array for empty input', () => {
    expect(sortResourceRelationships([])).toEqual([]);
  });

  it('does not mutate the original array', () => {
    const input = [
      makeRelationship({ sourceId: 'inactive', active: false }),
      makeRelationship({ sourceId: 'active', active: true }),
    ];
    sortResourceRelationships(input);
    expect(input.map((r) => r.sourceId)).toEqual(['inactive', 'active']);
  });

  it('places an inactive relationship after an active one via the left.active falsy arm', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({ sourceId: 'sleeping', active: false, confidence: 1 }),
      makeRelationship({ sourceId: 'live', active: true, confidence: 0.1 }),
    ]);
    expect(sorted.map((r) => r.sourceId)).toEqual(['live', 'sleeping']);
  });

  it('breaks an active-group tie using confidence with the zero-coalesce arm', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({ sourceId: 'zero-conf', active: true, confidence: 0 }),
      makeRelationship({ sourceId: 'half-conf', active: true, confidence: 0.5 }),
    ]);
    expect(sorted.map((r) => r.sourceId)).toEqual(['half-conf', 'zero-conf']);
  });

  it('falls back to observedAt when lastSeenAt is missing', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({
        sourceId: 'observed-only',
        active: true,
        confidence: 0.5,
        lastSeenAt: '',
        observedAt: '2026-01-01T00:00:00Z',
      }),
      makeRelationship({
        sourceId: 'recent',
        active: true,
        confidence: 0.5,
        lastSeenAt: '2026-06-01T00:00:00Z',
        observedAt: '2020-01-01T00:00:00Z',
      }),
    ]);
    expect(sorted.map((r) => r.sourceId)).toEqual(['recent', 'observed-only']);
  });

  it('treats missing lastSeenAt and observedAt as time zero', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({
        sourceId: 'no-times',
        active: true,
        confidence: 0.5,
        lastSeenAt: '',
        observedAt: '',
      }),
      makeRelationship({
        sourceId: 'timed',
        active: true,
        confidence: 0.5,
        lastSeenAt: '2026-06-01T00:00:00Z',
      }),
    ]);
    expect(sorted.map((r) => r.sourceId)).toEqual(['timed', 'no-times']);
  });

  it('keeps relative order when active, confidence, and times are equivalent', () => {
    const sorted = sortResourceRelationships([
      makeRelationship({
        sourceId: 'first',
        active: true,
        confidence: 0.5,
        lastSeenAt: 'bad',
        observedAt: 'bad',
      }),
      makeRelationship({
        sourceId: 'second',
        active: true,
        confidence: 0.5,
        lastSeenAt: 'bad',
        observedAt: 'bad',
      }),
    ]);
    expect(sorted.map((r) => r.sourceId)).toEqual(['first', 'second']);
  });
});
