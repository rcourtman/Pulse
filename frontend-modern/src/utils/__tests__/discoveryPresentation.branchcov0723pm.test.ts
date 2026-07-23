import { describe, expect, it } from 'vitest';

import type { ResourceDiscovery } from '@/types/discovery';
import {
  getDiscoveryIdentifiedSummary,
  getDiscoverySuggestedURLCodeClass,
  getDiscoverySuggestedURLFallback,
  getDiscoverySuggestedURLHeadingClass,
  getDiscoverySuggestedURLTextClass,
} from '@/utils/discoveryPresentation';

// Baseline discovery record carries no meaningful signal: every "unknown" /
// empty value, no ports, paths, facts, url or probe. Mirrors the fixture in
// discoveryPresentation.branchcov.test.ts so behaviour stays consistent.
const makeDiscovery = (overrides: Partial<ResourceDiscovery> = {}): ResourceDiscovery =>
  ({
    id: 'disc-1',
    resource_type: 'docker',
    resource_id: 'res-1',
    target_id: 'agent-1',
    hostname: 'host',
    service_type: 'unknown',
    service_name: 'unknown',
    service_version: 'unknown',
    category: 'unknown',
    cli_access: '',
    facts: [],
    config_paths: [],
    data_paths: [],
    log_paths: [],
    ports: [],
    user_notes: '',
    user_secrets: {},
    confidence: 0,
    ai_reasoning: '',
    discovered_at: '',
    updated_at: '',
    scan_duration: 0,
    ...overrides,
  }) as ResourceDiscovery;

// ---------------------------------------------------------------------------
// Three presentation class helpers are never called by any existing test.
// They take no arguments and have no internal branches, so "all inputs" is a
// single arm each: assert the exact returned class string.
// ---------------------------------------------------------------------------

describe('getDiscoverySuggestedURLHeadingClass (function + branch coverage)', () => {
  it('returns the canonical heading class string and takes no inputs', () => {
    expect(getDiscoverySuggestedURLHeadingClass()).toBe(
      'text-[11px] font-medium uppercase tracking-wide text-blue-800 dark:text-blue-200 mb-1',
    );
  });
});

describe('getDiscoverySuggestedURLTextClass (function + branch coverage)', () => {
  it('returns the canonical body-text class string and takes no inputs', () => {
    expect(getDiscoverySuggestedURLTextClass()).toBe('text-blue-700 dark:text-blue-300');
  });
});

describe('getDiscoverySuggestedURLCodeClass (function + branch coverage)', () => {
  it('returns the canonical mono code class string and takes no inputs', () => {
    expect(getDiscoverySuggestedURLCodeClass()).toBe(
      'min-w-0 flex-1 rounded bg-blue-100 px-2 py-1.5 text-xs text-blue-800 dark:bg-blue-950 dark:text-blue-100 font-mono break-all',
    );
  });
});

// ---------------------------------------------------------------------------
// Branch coverage: line 143 `const serviceName = (discovery.service_name || '').trim();`
// The makeDiscovery default ships service_name='unknown' (truthy), so the
// fallback arm only fires for a null/empty name. The record must stay
// meaningful via another signal or getDiscoveryIdentifiedSummary short-circuits
// to null before line 143 executes; a single port supplies that signal here.
// ---------------------------------------------------------------------------

describe('getDiscoveryIdentifiedSummary: (service_name || "") fallback arm (branch coverage)', () => {
  // Only genuinely falsy values flip the `||` to its fallback arm. (A
  // whitespace-only string is truthy and so never reaches this branch.)
  it.each<[string, unknown]>([
    ['null', null],
    ['undefined', undefined],
    ['empty string', ''],
  ])(
    'falls back to "" and labels "Unidentified service" when service_name is %s',
    (_label, name) => {
      const summary = getDiscoveryIdentifiedSummary(
        makeDiscovery({
          service_name: name as unknown as string,
          ports: [{ port: 80, protocol: 'tcp', process: 'nginx', address: '0.0.0.0' }],
        }),
      );
      expect(summary).not.toBeNull();
      // The fallback arm produced '' which isMeaningfulDiscoveryText rejects,
      // so the summary surfaces the placeholder name rather than the raw input.
      expect(summary?.serviceName).toBe('Unidentified service');
      expect(summary?.portCount).toBe(1);
      // service_type/version/category stay "unknown" so they are dropped too.
      expect(summary?.serviceType).toBeUndefined();
      expect(summary?.serviceVersion).toBeUndefined();
      expect(summary?.category).toBeUndefined();
    },
  );

  it('keeps the raw service_name when it is a meaningful truthy value (truthy-arm regression guard)', () => {
    const summary = getDiscoveryIdentifiedSummary(makeDiscovery({ service_name: 'Nginx' }));
    expect(summary?.serviceName).toBe('Nginx');
  });

  it('treats a whitespace-only service_name as truthy at the ||, then trims it to empty (no-trim-before-|| guard)', () => {
    // service_name='   ' is truthy, so the || keeps it; the subsequent .trim()
    // then empties it and isMeaningfulDiscoveryText rejects the empty result.
    // This documents that the trim happens AFTER the ||, not before.
    const summary = getDiscoveryIdentifiedSummary(
      makeDiscovery({
        service_name: '   ',
        ports: [{ port: 80, protocol: 'tcp', process: 'nginx', address: '0.0.0.0' }],
      }),
    );
    expect(summary?.serviceName).toBe('Unidentified service');
  });
});

// ---------------------------------------------------------------------------
// Branch coverage: line 302 `description: diagnostic || ''` in
// getDiscoverySuggestedURLFallback. Existing tests only pass a truthy 'diag',
// so exercise every falsy shape the public signature accepts.
// ---------------------------------------------------------------------------

describe('getDiscoverySuggestedURLFallback: (diagnostic || "") fallback arm (branch coverage)', () => {
  // Only genuinely falsy values flip the `||` to its fallback arm. (A
  // whitespace-only string is truthy — see the dedicated test below.)
  it.each<[string, string | null | undefined]>([
    ['undefined', undefined],
    ['null', null],
    ['empty string', ''],
  ])(
    'collapses a %s diagnostic to an empty description while keeping the title',
    (_label, diagnostic) => {
      expect(getDiscoverySuggestedURLFallback(diagnostic)).toEqual({
        title: 'No suggested URL available',
        description: '',
      });
    },
  );

  // Suspected source finding (reported, not fixed): unlike
  // normalizeDiscoverySuggestedUrl / cli_access handling, this fallback does
  // NOT trim — a whitespace-only diagnostic survives verbatim because it is
  // truthy at the ||.
  it('forwards a whitespace-only diagnostic verbatim (documents the missing trim)', () => {
    expect(getDiscoverySuggestedURLFallback('   ')).toEqual({
      title: 'No suggested URL available',
      description: '   ',
    });
  });
});

// ---------------------------------------------------------------------------
// Suspected source finding (reported, NOT fixed):
// Line 176 `suggestedUrlReasonTitle: suggestedUrlReason.title || undefined`.
// The falsy arm of this || is unreachable through the public API. Reasoning:
//   - Line 176 only executes after `getDiscoveryIdentifiedSummary` has passed
//     the `if (!discovery) return null` guard at line 141, so discovery is a
//     non-null object when `getDiscoverySuggestedURLReason(discovery)` runs.
//   - Inside getDiscoverySuggestedURLReason, the only path returning an empty
//     title is `if (!discovery) return { text: '', title: '' }`, which cannot
//     fire here. For any non-null discovery, `title` resolves to either
//     `${label}: ${detail}` or `label`, and `label` always falls back to the
//     non-empty default 'Discovery heuristic' from getDiscoveryURLSuggestionSourceLabel.
//   - Therefore suggestedUrlReason.title is always a non-empty (truthy) string
//     at line 176, and the `|| undefined` short-circuit arm is dead code.
// No test is written for it because faking it would require constructing an
// impossible state. Documented here and in the report.
// ---------------------------------------------------------------------------
