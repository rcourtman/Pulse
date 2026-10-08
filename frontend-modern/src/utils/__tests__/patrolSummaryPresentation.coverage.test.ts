import { describe, expect, it } from 'vitest';

import type { PatrolRunRecord } from '@/api/patrol';

import {
  getPatrolRecencyPresentation,
  getPatrolVerificationPresentation,
} from '@/utils/patrolSummaryPresentation';

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
// getPatrolVerificationPresentation — runtime-state branches, full-run-with-
// errors, zero-resource full run, limited-run variants (verification/scoped/
// unknown), no-completed-runs, and getVerificationActivityMixLabel undefined
// paths.
// ===========================================================================

describe('getPatrolVerificationPresentation — runtime state', () => {
  it('maps the blocked state to the paused runtime presentation', () => {
    expect(getPatrolVerificationPresentation({ runtimeState: 'blocked' })).toEqual({
      title: 'Patrol paused',
      description: 'Patrol cannot check infrastructure until the blocking condition is cleared.',
      compactLabel: 'Patrol paused',
      tone: 'warning',
    });
  });

  it('maps the disabled state to the disabled runtime presentation', () => {
    expect(getPatrolVerificationPresentation({ runtimeState: 'disabled' })).toEqual({
      title: 'Patrol disabled',
      description: 'Enable Patrol to resume checks.',
      compactLabel: 'Patrol disabled',
      tone: 'info',
    });
  });

  it('maps the unavailable state to the unavailable runtime presentation', () => {
    expect(getPatrolVerificationPresentation({ runtimeState: 'unavailable' })).toEqual({
      title: 'Patrol unavailable',
      description: 'Patrol is not ready yet. Check Provider & Models and runtime availability.',
      compactLabel: 'Patrol unavailable',
      tone: 'error',
    });
  });
});

describe('getPatrolVerificationPresentation — full run with errors (hasRunErrors)', () => {
  it('reports a needs-review check when the full run has errors and covered resources', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'patrol',
            resources_checked: 10,
            error_count: 2,
            status: 'error',
          }),
        ],
      }),
    ).toEqual({
      title: 'Patrol check needs review',
      description: 'The most recent Patrol check covered 10 resources but ended with 2 errors.',
      compactLabel: 'Check needs review',
      tone: 'warning',
      lastFullRunAt: '2026-07-10T09:05:00Z',
    });
  });

  it('reports a needs-review check with a generic message when errors occurred and zero resources', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'patrol',
            resources_checked: 0,
            error_count: 1,
            status: 'error',
          }),
        ],
      }),
    ).toEqual({
      title: 'Patrol check needs review',
      description: 'The most recent Patrol check ended with errors.',
      compactLabel: 'Check needs review',
      tone: 'warning',
      lastFullRunAt: '2026-07-10T09:05:00Z',
    });
  });

  it('detects errors via status "error" even when error_count is 0', () => {
    const result = getPatrolVerificationPresentation({
      runs: [
        makeRun({
          type: 'patrol',
          resources_checked: 5,
          error_count: 0,
          status: 'error',
        }),
      ],
    });
    expect(result.title).toBe('Patrol check needs review');
  });

  it('detects errors case-insensitively via status "ERROR" as wrong-typed input', () => {
    const run = makeRun({
      type: 'patrol',
      resources_checked: 5,
      error_count: 0,
      status: 'ERROR' as unknown as PatrolRunRecord['status'],
    });
    expect(getPatrolVerificationPresentation({ runs: [run] }).title).toBe(
      'Patrol check needs review',
    );
  });
});

describe('getPatrolVerificationPresentation — successful full run', () => {
  it('reports a successful check with zero resources', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'patrol',
            resources_checked: 0,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }),
    ).toEqual({
      title: 'Recently checked',
      description: 'The most recent Patrol check completed successfully.',
      compactLabel: 'Recently checked',
      tone: 'success',
      lastFullRunAt: '2026-07-10T09:05:00Z',
    });
  });

  it('reports a successful check with a single resource (singular)', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'patrol',
            resources_checked: 1,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }).description,
    ).toBe('The most recent Patrol check completed successfully and covered 1 resource.');
  });
});

describe('getPatrolVerificationPresentation — limited runs', () => {
  it('reports follow-up checks with zero resources', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'verification',
            resources_checked: 0,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }).description,
    ).toBe(
      'Recent follow-up checks did not cover your full infrastructure. Run Patrol to check everything.',
    );
  });

  it('reports targeted checks with zero resources', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'scoped',
            resources_checked: 0,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }).description,
    ).toBe(
      'Recent targeted checks did not cover your full infrastructure. Run Patrol to check everything.',
    );
  });

  it('reports an unknown-type limited run with resources using the targeted fallback', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'custom',
            resources_checked: 3,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }).description,
    ).toBe('Recent targeted checks covered 3 resources. Run Patrol to check everything.');
  });

  it('uses the default limited description for an unknown type with zero resources', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          makeRun({
            type: 'custom',
            resources_checked: 0,
            error_count: 0,
            status: 'healthy',
          }),
        ],
      }).description,
    ).toBe(
      'Recent activity only checked part of your infrastructure. Run Patrol to check everything.',
    );
  });
});

describe('getPatrolVerificationPresentation — no completed runs', () => {
  it('reports a pending check when no runs exist', () => {
    expect(getPatrolVerificationPresentation({})).toEqual({
      title: 'Run Patrol to check',
      description: 'Patrol has not completed a check yet.',
      compactLabel: 'Check pending',
      tone: 'info',
    });
  });

  it('reports a pending check when the only run is not completed', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [makeRun({ completed_at: '' })],
      }),
    ).toEqual({
      title: 'Run Patrol to check',
      description: 'Patrol has not completed a check yet.',
      compactLabel: 'Check pending',
      tone: 'info',
    });
  });
});

describe('getVerificationActivityMixLabel (via getPatrolVerificationPresentation)', () => {
  it('omits activityMixLabel when there is only a single completed run', () => {
    const result = getPatrolVerificationPresentation({
      runs: [successfulFullRun(10)],
    });
    expect(result.activityMixLabel).toBeUndefined();
  });

  it('omits activityMixLabel when all completed runs are full patrols', () => {
    const result = getPatrolVerificationPresentation({
      runs: [
        makeRun({
          id: 'run-a',
          started_at: '2026-07-10T10:00:00Z',
          completed_at: '2026-07-10T10:05:00Z',
          type: 'patrol',
          resources_checked: 40,
        }),
        makeRun({
          id: 'run-b',
          started_at: '2026-07-10T09:00:00Z',
          completed_at: '2026-07-10T09:05:00Z',
          type: 'full',
          resources_checked: 40,
        }),
      ],
    });
    expect(result.activityMixLabel).toBeUndefined();
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
// verification and recency presentations. Covers case/whitespace/empty
// normalization and the resulting full/scoped/verification classification.
// ===========================================================================

describe('normalizeRunType (via run-type classification)', () => {
  it('treats an empty type as a full run', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [makeRun({ type: '', resources_checked: 5, error_count: 0 })],
      }).title,
    ).toBe('Recently checked');
  });

  it('treats undefined type as a full run', () => {
    const run = makeRun({ resources_checked: 5, error_count: 0 });
    delete (run as Partial<PatrolRunRecord>).type;
    expect(getPatrolVerificationPresentation({ runs: [run] }).title).toBe('Recently checked');
  });

  it('treats "PATROL" (uppercase) as a full run', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [makeRun({ type: 'PATROL', resources_checked: 5, error_count: 0 })],
      }).title,
    ).toBe('Recently checked');
  });

  it('treats "  Full  " (whitespace, mixed case) as a full run', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [makeRun({ type: '  Full  ', resources_checked: 5, error_count: 0 })],
      }).title,
    ).toBe('Recently checked');
  });

  it('classifies "SCOPED" (uppercase) as a scoped run', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [makeRun({ type: 'SCOPED', resources_checked: 1, error_count: 0 })],
      }).description,
    ).toContain('targeted checks');
  });

  it('classifies "verification" as a verification run', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [makeRun({ type: 'verification', resources_checked: 1, error_count: 0 })],
      }).description,
    ).toContain('follow-up checks');
  });
});
