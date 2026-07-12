import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { getResourceHealthIssuePresentation } from '../resourceHealthPresentation';

/**
 * Branch-coverage suite for the still-uncovered branches in
 * resourceHealthPresentation.ts. `compactHealthLabel` is module-private, so it
 * is driven exclusively through the exported `getResourceHealthIssuePresentation`
 * (by placing the probe string in `incidentSummary`, which becomes `primary`),
 * asserting on the observable `compactLabel` — never re-implemented or imported
 * directly.
 */

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent-tower',
    type: 'agent',
    name: 'Tower',
    displayName: 'Tower',
    platformId: 'tower',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'degraded',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

/**
 * Push `primary` as the sole summary (via `incidentSummary`) on a degraded
 * resource and return the resulting `compactLabel`. Throws if the presentation
 * resolves to null so a branch misconfiguration surfaces as a test failure
 * rather than a silent `undefined` assertion.
 */
const compactLabelFor = (primary: string): string => {
  const result = getResourceHealthIssuePresentation(makeResource({ incidentSummary: primary }));
  if (!result) throw new Error(`expected non-null presentation for primary: ${primary}`);
  return result.compactLabel;
};

describe('compactHealthLabel (via getResourceHealthIssuePresentation)', () => {
  it('returns "Parity unavailable" when both "parity protection" and "unavailable" appear', () => {
    expect(compactLabelFor('Parity protection unavailable')).toBe('Parity unavailable');
  });

  it('returns "Parity missing" when both "parity" and "missing" appear (no "protection")', () => {
    expect(compactLabelFor('Parity disk missing')).toBe('Parity missing');
  });

  it('returns "Array check running" when "array is running check" appears', () => {
    expect(compactLabelFor('The array is running check now')).toBe('Array check running');
  });

  it('returns the summary unchanged when it is short (<= 34) and keyword-free', () => {
    expect(compactLabelFor('Disk failure detected')).toBe('Disk failure detected');
  });

  it('returns the summary unchanged at the exact 34-character boundary', () => {
    const exact = 'b'.repeat(34);
    expect(compactLabelFor(exact)).toBe(exact);
  });

  it('truncates keyword-free summaries longer than 34 characters to 31 chars + "..."', () => {
    expect(compactLabelFor('a'.repeat(40))).toBe(`${'a'.repeat(31)}...`);
  });

  it('prefers the "without parity protection" keyword over the truncation branch', () => {
    // Length > 34 but contains the keyword -> keyword label wins, no truncation.
    const longKeyword = `without parity protection${'X'.repeat(40)}`;
    expect(compactLabelFor(longKeyword)).toBe('No parity');
  });
});

describe('getResourceHealthIssuePresentation', () => {
  it('normalizes a whitespace-padded uppercase status to a recognized attention status', () => {
    // The runtime payload may carry a non-canonical status string even though
    // the TS type is strict, so cast to exercise the defensive trim().toLowerCase().
    const result = getResourceHealthIssuePresentation(
      makeResource({
        status: '  DEGRADED  ' as unknown as Resource['status'],
        incidentSummary: 'Normalized status hit',
      }),
    );
    // '  DEGRADED  ' is not in ATTENTION_STATUSES; only trim().toLowerCase()
    // (-> 'degraded') yields a match, so a non-null result proves normalization.
    expect(result?.primary).toBe('Normalized status hit');
  });

  it('returns null when status is undefined (the `|| ""` empty-string arm)', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({ status: undefined, incidentSummary: 'Present but ignored' }),
    );
    expect(result).toBeNull();
  });

  it('returns null when there are no summaries even with an attention status', () => {
    const result = getResourceHealthIssuePresentation(makeResource({ status: 'warning' }));
    expect(result).toBeNull();
  });

  it('returns null for a non-attention status when summaries are present', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({ status: 'paused', incidentSummary: 'Ignored when paused' }),
    );
    expect(result).toBeNull();
  });

  it('dedupes a case-insensitive duplicate of primary at push time (never reaches details or title)', () => {
    // pushUnique drops 'DISK FAILURE' against the already-pushed primary
    // 'Disk failure' before it can become a detail, so neither `details` nor
    // `title` contain the duplicate. (Consequence: the `details.filter(...)`
    // guard in the return statement is never able to remove anything — see
    // GLM_REPORT.md suspected-dead-code note.)
    const result = getResourceHealthIssuePresentation(
      makeResource({ incidentSummary: 'Disk failure', incidentLabel: 'DISK FAILURE' }),
    );
    expect(result).toStrictEqual({
      primary: 'Disk failure',
      compactLabel: 'Disk failure',
      details: [],
      title: 'Disk failure',
    });
  });

  it('keeps a non-matching detail in both `details` and `title`', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({ incidentSummary: 'Primary issue', incidentLabel: 'Secondary issue' }),
    );
    expect(result?.details).toStrictEqual(['Secondary issue']);
    expect(result?.title).toBe('Primary issue · Secondary issue');
  });

  it('dedupes case-insensitive equal summaries, keeping only the first occurrence', () => {
    // 'BETA' duplicates 'Beta' (already pushed) -> dropped by pushUnique before
    // it could become a second detail.
    const result = getResourceHealthIssuePresentation(
      makeResource({
        incidentSummary: 'Alpha',
        incidentLabel: 'Beta',
        storage: { postureSummary: 'BETA' },
      }),
    );
    expect(result?.details).toStrictEqual(['Beta']);
    expect(result?.title).toBe('Alpha · Beta');
  });

  it('skips whitespace-only summaries so they never become the primary', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({ incidentSummary: '   ', incidentLabel: 'Real issue' }),
    );
    expect(result?.primary).toBe('Real issue');
  });

  it('surfaces reasons from resource.storage.risk', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({
        storage: {
          risk: {
            level: 'critical',
            reasons: [{ code: 'storage_down', severity: 'critical', summary: 'Storage risk reason A' }],
          },
        },
      }),
    );
    expect(result?.primary).toBe('Storage risk reason A');
  });

  it('surfaces reasons from agent.storageRisk', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({
        agent: {
          storageRisk: {
            level: 'critical',
            reasons: [{ code: 'agent_disk', severity: 'critical', summary: 'Agent storage risk reason' }],
          },
        },
      }),
    );
    expect(result?.primary).toBe('Agent storage risk reason');
  });

  it('surfaces all four agent.unraid storage summaries (posture primary, rest as details)', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({
        agent: {
          unraid: {
            postureSummary: 'Unraid posture',
            riskSummary: 'Unraid risk',
            protectionSummary: 'Unraid protection',
            rebuildSummary: 'Unraid rebuild',
          },
        },
      }),
    );
    expect(result?.primary).toBe('Unraid posture');
    expect(result?.details).toStrictEqual(['Unraid risk', 'Unraid protection', 'Unraid rebuild']);
  });

  it('surfaces all four resource.storage summaries (posture primary, rest as details)', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({
        storage: {
          postureSummary: 'Storage posture',
          riskSummary: 'Storage risk',
          protectionSummary: 'Storage protection',
          rebuildSummary: 'Storage rebuild',
        },
      }),
    );
    expect(result?.primary).toBe('Storage posture');
    expect(result?.details).toStrictEqual(['Storage risk', 'Storage protection', 'Storage rebuild']);
  });

  it('surfaces all four agent storage summaries (posture primary, rest as details)', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({
        agent: {
          storagePostureSummary: 'Agent posture',
          storageRiskSummary: 'Agent risk',
          protectionSummary: 'Agent protection',
          rebuildSummary: 'Agent rebuild',
        },
      }),
    );
    expect(result?.primary).toBe('Agent posture');
    expect(result?.details).toStrictEqual(['Agent risk', 'Agent protection', 'Agent rebuild']);
  });

  it('prefers incidentSummary as primary over storage and agent summaries (push order)', () => {
    const result = getResourceHealthIssuePresentation(
      makeResource({
        incidentSummary: 'Top priority',
        storage: { postureSummary: 'Storage summary' },
        agent: { storagePostureSummary: 'Agent summary' },
      }),
    );
    expect(result?.primary).toBe('Top priority');
    expect(result?.details).toStrictEqual(['Storage summary', 'Agent summary']);
  });
});
