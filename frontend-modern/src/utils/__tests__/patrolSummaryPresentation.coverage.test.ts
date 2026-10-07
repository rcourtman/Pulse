import { describe, expect, it } from 'vitest';

import type { PatrolRunRecord } from '@/api/patrol';

import {
  getPatrolRecencyPresentation,
  getPatrolRunCoverage,
} from '@/utils/patrolSummaryPresentation';

// Ten minutes after the default fixture run completed.
const NOW = Date.parse('2026-07-10T09:15:00Z');

// Minimal typed PatrolRunRecord fixtures.
function makeRun(overrides: Partial<PatrolRunRecord> = {}): PatrolRunRecord {
  return {
    id: 'run-1',
    started_at: '2026-07-10T09:00:00Z',
    completed_at: '2026-07-10T09:05:00Z',
    duration_ms: 300000,
    type: 'patrol',
    resources_checked: 0,
    nodes_checked: 0,
    guests_checked: 0,
    docker_checked: 0,
    storage_checked: 0,
    hosts_checked: 0,
    truenas_checked: 0,
    pbs_checked: 0,
    pmg_checked: 0,
    kubernetes_checked: 0,
    new_findings: 0,
    existing_findings: 0,
    rejected_findings: 0,
    resolved_findings: 0,
    auto_fix_count: 0,
    findings_summary: '',
    error_count: 0,
    status: 'healthy',
    triage_flags: 0,
    tool_call_count: 0,
    ...overrides,
  };
}

function successfulFullRun(resourcesChecked = 50): PatrolRunRecord {
  return makeRun({
    type: 'patrol',
    resources_checked: resourcesChecked,
    error_count: 0,
    status: 'issues_found',
  });
}

// ===========================================================================
// getPatrolRunCoverage — the latest completed full run decides; errors,
// zero-resource runs, limited-only history, and missing history.
// ===========================================================================

describe('getPatrolRunCoverage — latest completed full run', () => {
  it('is complete when the full run ended cleanly and checked resources', () => {
    expect(getPatrolRunCoverage([successfulFullRun(58)], NOW)).toBe('complete');
  });

  it('is complete when the full run checked a single resource', () => {
    expect(getPatrolRunCoverage([successfulFullRun(1)], NOW)).toBe('complete');
  });

  it('is unproven when the clean full run checked zero resources', () => {
    expect(getPatrolRunCoverage([successfulFullRun(0)], NOW)).toBe('unproven');
  });

  it('is incomplete when the full run ended with errors after checking resources', () => {
    expect(
      getPatrolRunCoverage(
        [makeRun({ resources_checked: 30, error_count: 2, status: 'error' })],
        NOW,
      ),
    ).toBe('incomplete');
  });

  it('is incomplete when the full run ended with errors and checked nothing', () => {
    expect(
      getPatrolRunCoverage(
        [makeRun({ resources_checked: 0, error_count: 1, status: 'error' })],
        NOW,
      ),
    ).toBe('incomplete');
  });

  it('detects errors via status "error" even when error_count is 0', () => {
    expect(
      getPatrolRunCoverage(
        [makeRun({ resources_checked: 30, error_count: 0, status: 'error' })],
        NOW,
      ),
    ).toBe('incomplete');
  });

  it('detects errors case-insensitively via status "ERROR" as wrong-typed input', () => {
    const run = makeRun({ resources_checked: 30, error_count: 0 });
    (run as unknown as { status: string }).status = ' ERROR ';
    expect(getPatrolRunCoverage([run], NOW)).toBe('incomplete');
  });

  it('reads only the latest completed full run when an older one errored', () => {
    expect(
      getPatrolRunCoverage(
        [
          successfulFullRun(40),
          makeRun({ id: 'run-older', resources_checked: 40, error_count: 3, status: 'error' }),
        ],
        NOW,
      ),
    ).toBe('complete');
  });

  it('reads only the latest completed full run when an older one was clean', () => {
    expect(
      getPatrolRunCoverage(
        [
          makeRun({ id: 'run-newer', resources_checked: 40, error_count: 1, status: 'error' }),
          successfulFullRun(40),
        ],
        NOW,
      ),
    ).toBe('incomplete');
  });

  it('skips a full run still in progress', () => {
    expect(
      getPatrolRunCoverage(
        [
          makeRun({ id: 'run-in-progress', completed_at: '  ', error_count: 1, status: 'error' }),
          successfulFullRun(40),
        ],
        NOW,
      ),
    ).toBe('complete');
  });

  it('lets later clean targeted runs leave a clean full run complete', () => {
    expect(
      getPatrolRunCoverage(
        [
          makeRun({ id: 'run-scoped', type: 'scoped', resources_checked: 1 }),
          successfulFullRun(40),
        ],
        NOW,
      ),
    ).toBe('complete');
  });
});

describe('getPatrolRunCoverage — what a clean full run can vouch for', () => {
  it('is incomplete when a targeted run after the clean full run failed', () => {
    expect(
      getPatrolRunCoverage(
        [
          makeRun({
            id: 'run-scoped-failed',
            type: 'scoped',
            started_at: '2026-07-10T09:08:00Z',
            completed_at: '2026-07-10T09:09:00Z',
            resources_checked: 0,
            error_count: 1,
            status: 'error',
          }),
          successfulFullRun(40),
        ],
        NOW,
      ),
    ).toBe('incomplete');
  });

  it('is complete when the failure came before the clean full run', () => {
    expect(
      getPatrolRunCoverage(
        [
          successfulFullRun(40),
          makeRun({
            id: 'run-scoped-failed',
            type: 'scoped',
            started_at: '2026-07-10T08:00:00Z',
            completed_at: '2026-07-10T08:01:00Z',
            error_count: 1,
            status: 'error',
          }),
        ],
        NOW,
      ),
    ).toBe('complete');
  });

  it('is unproven when the clean full run finished more than 24 hours ago', () => {
    expect(
      getPatrolRunCoverage(
        [successfulFullRun(40)],
        Date.parse('2026-07-10T09:05:00Z') + 24 * 60 * 60 * 1000 + 1,
      ),
    ).toBe('unproven');
  });

  it('is complete when the clean full run finished exactly 24 hours ago', () => {
    expect(
      getPatrolRunCoverage(
        [successfulFullRun(40)],
        Date.parse('2026-07-10T09:05:00Z') + 24 * 60 * 60 * 1000,
      ),
    ).toBe('complete');
  });

  it('is unproven when the clean full run has an unparseable completion time', () => {
    expect(
      getPatrolRunCoverage([makeRun({ resources_checked: 40, completed_at: 'soon' })], NOW),
    ).toBe('unproven');
  });
});

describe('getPatrolRunCoverage — no completed full run', () => {
  it('is incomplete when only targeted runs completed', () => {
    expect(getPatrolRunCoverage([makeRun({ type: 'scoped', resources_checked: 1 })], NOW)).toBe(
      'incomplete',
    );
  });

  it('is incomplete when only follow-up checks completed', () => {
    expect(
      getPatrolRunCoverage([makeRun({ type: 'verification', resources_checked: 1 })], NOW),
    ).toBe('incomplete');
  });

  it('is unproven when no runs exist', () => {
    expect(getPatrolRunCoverage(undefined, NOW)).toBe('unproven');
    expect(getPatrolRunCoverage([], NOW)).toBe('unproven');
  });

  it('is unproven when the only run is not completed', () => {
    expect(getPatrolRunCoverage([makeRun({ completed_at: undefined })], NOW)).toBe('unproven');
  });
});

// ===========================================================================
// getPatrolRecencyPresentation — timestamp fallback logic branches,
// formatRecencyResourcesCheckedLabel singular case.
// ===========================================================================

describe('getPatrolRecencyPresentation — timestamp fallback logic', () => {
  it('prefers lastPatrolAt when lastActivityAt is an unparseable date', () => {
    expect(
      getPatrolRecencyPresentation({
        lastPatrolAt: '2026-07-10T09:57:00Z',
        lastActivityAt: 'not-a-date',
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-07-10T09:57:00Z',
    });
  });

  it('prefers lastPatrolAt when it is newer than or equal to lastActivityAt', () => {
    expect(
      getPatrolRecencyPresentation({
        lastPatrolAt: '2026-07-10T10:00:00Z',
        lastActivityAt: '2026-07-10T09:00:00Z',
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-07-10T10:00:00Z',
    });
  });

  it('prefers lastActivityAt when lastPatrolAt is an unparseable date', () => {
    expect(
      getPatrolRecencyPresentation({
        lastPatrolAt: 'not-a-date',
        lastActivityAt: '2026-07-10T09:00:00Z',
      }),
    ).toEqual({
      label: 'Last activity',
      timestamp: '2026-07-10T09:00:00Z',
    });
  });

  it('returns last activity when only lastActivityAt is provided', () => {
    expect(getPatrolRecencyPresentation({ lastActivityAt: '2026-07-10T09:00:00Z' })).toEqual({
      label: 'Last activity',
      timestamp: '2026-07-10T09:00:00Z',
    });
  });

  it('returns last check when only lastPatrolAt is provided', () => {
    expect(getPatrolRecencyPresentation({ lastPatrolAt: '2026-07-10T09:57:00Z' })).toEqual({
      label: 'Last check',
      timestamp: '2026-07-10T09:57:00Z',
    });
  });

  it('returns a timestamp-less last-activity label when nothing is provided', () => {
    expect(getPatrolRecencyPresentation({})).toEqual({
      label: 'Last activity',
    });
  });

  it('trims whitespace from timestamps before comparing', () => {
    expect(
      getPatrolRecencyPresentation({
        lastPatrolAt: '  2026-07-10T09:57:00Z  ',
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-07-10T09:57:00Z',
    });
  });
});

describe('getPatrolRecencyPresentation — resourcesCheckedLabel', () => {
  it('uses "checked 1 resource" (singular) for a successful full run with one resource', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          makeRun({
            type: 'patrol',
            resources_checked: 1,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-07-10T09:05:00Z',
      resourcesChecked: 1,
      resourcesCheckedLabel: 'checked 1 resource',
    });
  });

  it('uses "checked" for a scoped run regardless of resources', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          makeRun({
            type: 'scoped',
            resources_checked: 3,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }).resourcesCheckedLabel,
    ).toBe('checked 3 resources');
  });

  it('omits resourcesCheckedLabel when the completed run checked zero resources', () => {
    const result = getPatrolRecencyPresentation({
      runs: [
        makeRun({
          type: 'patrol',
          resources_checked: 0,
          error_count: 0,
          status: 'healthy',
        }),
      ],
    });
    expect(result.resourcesChecked).toBeUndefined();
    expect(result.resourcesCheckedLabel).toBeUndefined();
  });

  it('uses "checked" for a full run that ended with errors', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          makeRun({
            type: 'patrol',
            resources_checked: 5,
            error_count: 1,
            status: 'error',
          }),
        ],
      }).resourcesCheckedLabel,
    ).toBe('checked 5 resources');
  });
});

// ===========================================================================
// normalizeRunType (private) — exercised through run-type classification in
// the run-history coverage verdict. Covers case/whitespace/empty
// normalization and the resulting full/limited classification.
// ===========================================================================

describe('normalizeRunType (via run-type classification)', () => {
  it('treats an empty type as a full run', () => {
    expect(getPatrolRunCoverage([makeRun({ type: '', resources_checked: 5 })], NOW)).toBe(
      'complete',
    );
  });

  it('treats undefined type as a full run', () => {
    const run = makeRun({ resources_checked: 5 });
    delete (run as Partial<PatrolRunRecord>).type;
    expect(getPatrolRunCoverage([run], NOW)).toBe('complete');
  });

  it('treats "PATROL" (uppercase) as a full run', () => {
    expect(getPatrolRunCoverage([makeRun({ type: 'PATROL', resources_checked: 5 })], NOW)).toBe(
      'complete',
    );
  });

  it('treats "  Full  " (whitespace, mixed case) as a full run', () => {
    expect(getPatrolRunCoverage([makeRun({ type: '  Full  ', resources_checked: 5 })], NOW)).toBe(
      'complete',
    );
  });

  it('classifies "SCOPED" (uppercase) as a limited run', () => {
    expect(getPatrolRunCoverage([makeRun({ type: 'SCOPED', resources_checked: 1 })], NOW)).toBe(
      'incomplete',
    );
  });
});
