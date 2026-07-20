import { describe, expect, it } from 'vitest';

import type { PatrolRunRecord } from '@/api/patrol';
import type { IntelligenceHealthScore } from '@/types/aiIntelligence';

import {
  getPatrolAssessmentPresentation,
  getPatrolAssessmentShellPresentation,
  getPatrolCompactAssessmentLabel,
  getPatrolRecencyPresentation,
  getPatrolSummaryMetricState,
  getPatrolVerificationPresentation,
} from '@/utils/patrolSummaryPresentation';

// ---------------------------------------------------------------------------
// Fixture builders — minimal, typed objects that satisfy the module's
// internal PatrolAssessmentFinding / PatrolRunRecord / IntelligenceHealthScore
// shapes without needing the non-exported PatrolAssessmentFinding alias.
// ---------------------------------------------------------------------------

type AssessmentFinding = NonNullable<
  Parameters<typeof getPatrolCompactAssessmentLabel>[0]['activeFindings']
>[number];

function makeFinding(overrides: Partial<AssessmentFinding> = {}): AssessmentFinding {
  return {
    resourceId: 'vm-100',
    resourceName: 'web-1',
    title: 'Disk nearly full',
    severity: 'warning',
    status: 'active',
    ...overrides,
  };
}

function makeRuntimeFinding(overrides: Partial<AssessmentFinding> = {}): AssessmentFinding {
  return makeFinding({
    resourceId: 'ai-service',
    resourceName: 'Pulse Patrol Service',
    title: 'Pulse Patrol: Provider connection issue',
    ...overrides,
  });
}

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

function makeHealth(overrides: Partial<IntelligenceHealthScore> = {}): IntelligenceHealthScore {
  return {
    score: 90,
    grade: 'A',
    trend: 'stable',
    factors: [],
    prediction: 'Infrastructure is healthy with no significant issues detected.',
    ...overrides,
  };
}

function coverageFactor() {
  return {
    name: 'Patrol coverage incomplete',
    impact: -0.35,
    description: 'Patrol coverage is incomplete.',
    category: 'coverage',
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
// getPatrolAssessmentShellPresentation — covers all SemanticTone map entries
// and the fallback for an unknown tone.
// ===========================================================================

describe('getPatrolAssessmentShellPresentation', () => {
  it('maps the success tone to the emerald shell', () => {
    expect(getPatrolAssessmentShellPresentation('success')).toEqual({
      headerClass: 'bg-emerald-50/60 dark:bg-emerald-950/30',
      badgeVariant: 'success',
      iconClass: 'text-emerald-600 dark:text-emerald-300',
      iconContainerClass:
        'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/40',
    });
  });

  it('maps the error tone to the red shell with a danger badge', () => {
    expect(getPatrolAssessmentShellPresentation('error')).toEqual({
      headerClass: 'bg-red-50/70 dark:bg-red-950/30',
      badgeVariant: 'danger',
      iconClass: 'text-red-600 dark:text-red-300',
      iconContainerClass: 'border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/40',
    });
  });

  it('maps an explicit info tone to the blue shell', () => {
    expect(getPatrolAssessmentShellPresentation('info')).toEqual({
      headerClass: 'bg-blue-50/70 dark:bg-blue-950/30',
      badgeVariant: 'info',
      iconClass: 'text-blue-600 dark:text-blue-300',
      iconContainerClass: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950/40',
    });
  });

  it('falls back to the info shell for an unrecognised tone string', () => {
    expect(getPatrolAssessmentShellPresentation('critical' as never)).toEqual(
      getPatrolAssessmentShellPresentation('info'),
    );
  });
});

// ===========================================================================
// getPatrolSummaryMetricState — covers classifyActiveFindings severity
// branches and default fallbacks.
// ===========================================================================

describe('getPatrolSummaryMetricState', () => {
  it('flags primarySeverity as critical when an infrastructure critical finding is active', () => {
    const state = getPatrolSummaryMetricState({
      activeFindings: [
        makeFinding({ severity: 'critical', resourceId: 'vm-1', title: 'Host down' }),
      ],
    });
    expect(state.primarySeverity).toBe('critical');
    expect(state.primaryValue).toBe(1);
    expect(state.criticalValue).toBe(1);
    expect(state.secondarySeverity).toBe('warning');
  });

  it('flags secondarySeverity as critical when the runtime issue is critical', () => {
    const state = getPatrolSummaryMetricState({
      activeFindings: [makeRuntimeFinding({ severity: 'critical' })],
    });
    expect(state.secondarySeverity).toBe('critical');
    expect(state.secondaryValue).toBe(1);
    expect(state.criticalValue).toBe(1);
    expect(state.primaryLabel).toBe('Infrastructure findings');
  });

  it('defaults fixedValue to 0 when fixedCount is omitted', () => {
    expect(getPatrolSummaryMetricState({}).fixedValue).toBe(0);
  });

  it('ignores findings whose status is not active', () => {
    const state = getPatrolSummaryMetricState({
      activeFindings: [
        makeFinding({ status: 'resolved', severity: 'critical' }),
        makeRuntimeFinding({ status: 'dismissed', severity: 'critical' }),
      ],
    });
    expect(state.primaryValue).toBe(0);
    expect(state.secondaryValue).toBe(0);
    expect(state.criticalValue).toBe(0);
  });

  it('combines mixed infrastructure and runtime severities into the critical total', () => {
    const state = getPatrolSummaryMetricState({
      activeFindings: [
        makeFinding({ severity: 'critical', resourceId: 'vm-1' }),
        makeRuntimeFinding({ severity: 'critical' }),
        makeFinding({ severity: 'warning', resourceId: 'vm-2' }),
      ],
    });
    expect(state.criticalValue).toBe(2);
    expect(state.primarySeverity).toBe('critical');
    expect(state.secondarySeverity).toBe('critical');
  });
});

// ===========================================================================
// getPatrolCompactAssessmentLabel — covers formatIssueLabel singular/plural,
// totalActive fallback, health-score suppression by absence.
// ===========================================================================

describe('getPatrolCompactAssessmentLabel', () => {
  it('lists both critical and warning infrastructure findings when both are active', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'Issues detected',
        activeFindings: [
          makeFinding({ severity: 'critical', resourceId: 'vm-1', title: 'Host down' }),
          makeFinding({ severity: 'warning', resourceId: 'vm-2', title: 'Disk full' }),
        ],
      }),
    ).toBe('1 critical issue · 1 warning issue');
  });

  it('pluralises infrastructure critical issues', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'Issues detected',
        activeFindings: [
          makeFinding({ severity: 'critical', resourceId: 'vm-1' }),
          makeFinding({ severity: 'critical', resourceId: 'vm-2' }),
        ],
      }),
    ).toBe('2 critical issues');
  });

  it('pluralises Patrol runtime issues when more than one is active', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'Issues detected',
        activeFindings: [
          makeRuntimeFinding({ severity: 'warning', title: 'Pulse Patrol: Issue A' }),
          makeRuntimeFinding({ severity: 'warning', title: 'Pulse Patrol: Issue B' }),
        ],
      }),
    ).toBe('2 Patrol runtime issues');
  });

  it('falls back to the totalActive count when classification yields no parts', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'Issues detected',
        totalActive: 3,
        activeFindings: [makeFinding({ status: 'resolved', severity: 'warning' })],
      }),
    ).toBe('3 active issues');
  });

  it('omits the health-score segment when overallHealth is absent', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'No active issues',
      }),
    ).toBe('No active issues');
  });

  it('rounds the health score to the nearest whole number', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'No active issues',
        overallHealth: makeHealth({ score: 72.4 }),
      }),
    ).toBe('No active issues · health score 72/100');
  });

  it('pluralises historical regressions', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'No active issues',
        historicalRegressionCount: 3,
      }),
    ).toBe('No active issues · 3 past regressions');
  });

  it('combines findings, regressions, and health score into one compact label', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'Issues detected',
        overallHealth: makeHealth({ score: 60, grade: 'C' }),
        activeFindings: [makeFinding({ severity: 'critical', resourceId: 'vm-1' })],
        historicalRegressionCount: 1,
      }),
    ).toBe('1 critical issue · 1 past regression · health score 60/100');
  });
});

// ===========================================================================
// getPatrolAssessmentPresentation — runtime-state branches, critical-finding
// branches, coverage-gap health tones (getHealthSummaryTone), and
// getCoverageDescription fallback.
// ===========================================================================

describe('getPatrolAssessmentPresentation — runtime state', () => {
  it('maps the blocked state to the paused runtime presentation', () => {
    expect(getPatrolAssessmentPresentation({ runtimeState: 'blocked' })).toEqual({
      title: 'Patrol paused',
      description: 'Patrol cannot check infrastructure until the blocking condition is cleared.',
      eyebrow: 'Patrol paused',
      compactLabel: 'Patrol paused',
      tone: 'warning',
    });
  });

  it('maps the disabled state to the disabled runtime presentation', () => {
    expect(getPatrolAssessmentPresentation({ runtimeState: 'disabled' })).toEqual({
      title: 'Patrol disabled',
      description: 'Enable Patrol to resume checks.',
      eyebrow: 'Patrol disabled',
      compactLabel: 'Patrol disabled',
      tone: 'info',
    });
  });

  it('maps the unavailable state to the unavailable runtime presentation', () => {
    expect(getPatrolAssessmentPresentation({ runtimeState: 'unavailable' })).toEqual({
      title: 'Patrol unavailable',
      description: 'Patrol is not ready yet. Check Provider & Models and runtime availability.',
      eyebrow: 'Patrol unavailable',
      compactLabel: 'Patrol unavailable',
      tone: 'error',
    });
  });

  it('passes the blocked reason through to the assessment description', () => {
    expect(
      getPatrolAssessmentPresentation({
        runtimeState: 'blocked',
        blockedReason: '  provider offline  ',
      }).description,
    ).toBe('provider offline');
  });
});

describe('getPatrolAssessmentPresentation — critical findings', () => {
  it('reports critical issues detected for infrastructure critical findings', () => {
    expect(
      getPatrolAssessmentPresentation({
        criticalFindings: 1,
        activeFindings: [
          makeFinding({ severity: 'critical', resourceId: 'vm-1', title: 'Host down' }),
        ],
      }),
    ).toEqual({
      title: 'Critical issues detected',
      description:
        'Patrol surfaced 1 active critical finding in your infrastructure. Review the active findings for more detail.',
      eyebrow: 'Status',
      compactLabel: 'Issues detected',
      tone: 'error',
    });
  });

  it('reports a critical Patrol runtime issue when only a runtime finding is critical', () => {
    expect(
      getPatrolAssessmentPresentation({
        criticalFindings: 1,
        activeFindings: [makeRuntimeFinding({ severity: 'critical' })],
      }),
    ).toEqual({
      title: 'Critical Patrol runtime issue',
      description:
        'Patrol is currently blocked by a critical runtime issue: Provider connection issue. Review the Patrol runtime issue for more detail.',
      eyebrow: 'Status',
      compactLabel: 'Patrol runtime issue',
      tone: 'error',
    });
  });
});

describe('getPatrolAssessmentPresentation — coverage gap health tones (getHealthSummaryTone)', () => {
  it('uses the error tone when health grade is D', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 45,
          grade: 'D',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete.',
        }),
      }),
    ).toMatchObject({ title: 'Coverage incomplete', tone: 'error' });
  });

  it('uses the error tone when health grade is F', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 25,
          grade: 'F',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete.',
        }),
      }),
    ).toMatchObject({ title: 'Coverage incomplete', tone: 'error' });
  });

  it('uses the warning tone when health grade is C with a coverage gap', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 70,
          grade: 'C',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete.',
        }),
      }),
    ).toMatchObject({ title: 'Coverage incomplete', tone: 'warning' });
  });
});

describe('getPatrolAssessmentPresentation — getCoverageDescription fallback', () => {
  it('falls back to the default message when the prediction is whitespace-only', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 50,
          grade: 'D',
          factors: [coverageFactor()],
          prediction: '   ',
        }),
      }).description,
    ).toBe('Patrol has not finished a current check.');
  });
});

describe('getPatrolAssessmentPresentation — health requires attention', () => {
  it('uses a usable prediction as the description for non-A health', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 80,
          grade: 'B',
          prediction: 'Some warnings need review.',
        }),
      }),
    ).toEqual({
      title: 'Health requires attention',
      description: 'Some warnings need review.',
      eyebrow: 'Status',
      compactLabel: 'Health requires attention',
      tone: 'warning',
    });
  });

  it('suppresses a coverage-gap prediction after a verified full run', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 80,
          grade: 'B',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete. Run Patrol to check everything.',
        }),
        runs: [successfulFullRun()],
      }),
    ).toEqual({
      title: 'Health requires attention',
      description: 'Patrol still needs attention.',
      eyebrow: 'Status',
      compactLabel: 'Health requires attention',
      tone: 'warning',
    });
  });

  it('falls back when the health prediction is empty', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({ score: 80, grade: 'B', prediction: '' }),
      }).description,
    ).toBe('Patrol still needs attention.');
  });

  it('keeps a non-coverage-gap prediction even after a verified full run', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 80,
          grade: 'B',
          factors: [coverageFactor()],
          prediction: 'A few warnings were detected.',
        }),
        runs: [successfulFullRun()],
      }).description,
    ).toBe('A few warnings were detected.');
  });
});

describe('getPatrolAssessmentPresentation — default success', () => {
  it('returns the default success shell when no health is provided', () => {
    expect(getPatrolAssessmentPresentation({})).toEqual({
      title: 'No active issues detected',
      description: 'Infrastructure is healthy with no significant issues detected.',
      eyebrow: 'Status',
      compactLabel: 'No active issues',
      tone: 'success',
    });
  });

  it('falls back to the default description when grade-A health has an empty prediction', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({ grade: 'A', prediction: '' }),
      }).description,
    ).toBe('Infrastructure is healthy with no significant issues detected.');
  });
});

// ===========================================================================
// getFindingAssessmentDescription (private) — exercised through
// getPatrolAssessmentPresentation. Covers single-runtime-issue branches,
// joinAssessmentParts with 1/2/3+ parts, formatRuntimeIssueCount,
// getRuntimeFindingSummaryLabel, shouldUsePrediction,
// predictionReadsAsAllClear, predictionReadsAsCoverageGap,
// criticalFindings/warningFindings fallbacks.
// ===========================================================================

describe('getFindingAssessmentDescription — single runtime issue', () => {
  it('uses the critical-severity runtime summary for a single critical runtime issue', () => {
    expect(
      getPatrolAssessmentPresentation({
        criticalFindings: 1,
        activeFindings: [makeRuntimeFinding({ severity: 'critical' })],
      }).description,
    ).toContain('Patrol is currently blocked by a critical runtime issue');
  });

  it('appends the coverage-gap caveat for a single critical runtime issue', () => {
    expect(
      getPatrolAssessmentPresentation({
        criticalFindings: 1,
        overallHealth: makeHealth({
          score: 50,
          grade: 'D',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete.',
        }),
        activeFindings: [makeRuntimeFinding({ severity: 'critical' })],
      }).description,
    ).toBe(
      'Patrol is currently blocked by a critical runtime issue: Provider connection issue. Recent coverage is incomplete. Run Patrol to check everything.',
    );
  });

  it('uses a usable prediction for a single runtime issue', () => {
    expect(
      getPatrolAssessmentPresentation({
        warningFindings: 1,
        overallHealth: makeHealth({
          score: 85,
          grade: 'B',
          prediction: 'A runtime hiccup was observed.',
        }),
        activeFindings: [makeRuntimeFinding({ severity: 'warning' })],
      }).description,
    ).toBe('A runtime hiccup was observed.');
  });

  it('falls back to "a Patrol runtime issue" when the runtime finding has no title', () => {
    expect(
      getPatrolAssessmentPresentation({
        warningFindings: 1,
        activeFindings: [makeRuntimeFinding({ title: '' })],
      }).description,
    ).toBe(
      'Patrol has an active runtime issue: a Patrol runtime issue. Review the Patrol runtime issue for more detail.',
    );
  });

  it('keeps an unnormalised runtime title that has no Pulse Patrol prefix', () => {
    expect(
      getPatrolAssessmentPresentation({
        warningFindings: 1,
        activeFindings: [makeRuntimeFinding({ title: 'Some custom runtime error' })],
      }).description,
    ).toContain('Some custom runtime error');
  });
});

describe('getFindingAssessmentDescription — multi-finding joinAssessmentParts', () => {
  it('joins two summary parts with "and"', () => {
    expect(
      getPatrolAssessmentPresentation({
        warningFindings: 2,
        activeFindings: [
          makeFinding({ severity: 'critical', resourceId: 'vm-1', title: 'Host down' }),
          makeFinding({ severity: 'warning', resourceId: 'vm-2', title: 'Disk full' }),
        ],
      }).description,
    ).toBe(
      'Patrol surfaced 1 active critical finding in your infrastructure and 1 active warning finding in your infrastructure. Review the active findings for more detail.',
    );
  });

  it('joins four summary parts with an Oxford comma', () => {
    expect(
      getPatrolAssessmentPresentation({
        criticalFindings: 2,
        activeFindings: [
          makeFinding({ severity: 'critical', resourceId: 'vm-1', title: 'Host down' }),
          makeFinding({ severity: 'warning', resourceId: 'vm-2', title: 'Disk full' }),
          makeRuntimeFinding({ severity: 'critical' }),
          makeRuntimeFinding({
            severity: 'warning',
            title: 'Pulse Patrol: Slow response',
          }),
        ],
      }).description,
    ).toBe(
      'Patrol surfaced 1 active critical finding in your infrastructure, 1 active warning finding in your infrastructure, 1 active critical Patrol runtime issue, and 1 active Patrol runtime issue. Review the active findings for more detail.',
    );
  });

  it('uses the criticalFindings fallback when no active findings classify', () => {
    expect(
      getPatrolAssessmentPresentation({
        criticalFindings: 3,
        activeFindings: [makeFinding({ status: 'resolved', severity: 'critical' })],
      }).description,
    ).toBe(
      'Patrol surfaced 3 active critical findings. Review the active findings for more detail.',
    );
  });

  it('uses the warningFindings fallback when no active findings classify and no criticals exist', () => {
    expect(
      getPatrolAssessmentPresentation({
        warningFindings: 2,
      }).description,
    ).toBe(
      'Patrol surfaced 2 active warning findings. Review the active findings for more detail.',
    );
  });
});

describe('predictionReadsAsAllClear (via getFindingAssessmentDescription)', () => {
  it.each([
    ['healthy with no significant issue'],
    ['no significant issues detected'],
    ['no active issues'],
    ['no issues detected'],
    ['all clear'],
    ['ALL CLEAR'], // case-insensitive
  ])('suppresses the all-clear prediction "%s" for a single runtime issue', (prediction) => {
    const description = getPatrolAssessmentPresentation({
      warningFindings: 1,
      overallHealth: makeHealth({ score: 95, grade: 'A', prediction }),
      activeFindings: [makeRuntimeFinding({ severity: 'warning' })],
    }).description;
    expect(description).not.toBe(prediction);
    expect(description).toContain('Patrol has an active runtime issue');
  });

  it('returns false for an empty prediction so the runtime summary is used', () => {
    const description = getPatrolAssessmentPresentation({
      warningFindings: 1,
      overallHealth: makeHealth({ score: 95, grade: 'A', prediction: '' }),
      activeFindings: [makeRuntimeFinding({ severity: 'warning' })],
    }).description;
    expect(description).toContain('Review the Patrol runtime issue for more detail.');
  });
});

describe('predictionReadsAsCoverageGap (via getPatrolAssessmentPresentation)', () => {
  it.each([
    ['coverage is incomplete'],
    ['coverage incomplete'],
    ['not fully verified'],
    ['limited to scoped runs'],
    ['limited to targeted'],
    ['runs encountered errors'],
    ['ended with errors'],
    ['current issue list'],
    ['current full issue list'],
    ['current health may be incomplete'],
    ['summary may be incomplete'],
  ])('suppresses the coverage-gap prediction "%s" after a verified full run', (prediction) => {
    const description = getPatrolAssessmentPresentation({
      overallHealth: makeHealth({
        score: 80,
        grade: 'B',
        factors: [coverageFactor()],
        prediction,
      }),
      runs: [successfulFullRun()],
    }).description;
    expect(description).toBe('Patrol still needs attention.');
  });
});

// ===========================================================================
// hasSuccessfulFullCoverageRun (private) — exercised through
// getPatrolAssessmentPresentation's coverage-gap branch.
// ===========================================================================

describe('hasSuccessfulFullCoverageRun (via coverage-gap branch)', () => {
  it('treats a full run with errors as not successful so the coverage gap stays', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 70,
          grade: 'C',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete.',
        }),
        runs: [
          makeRun({
            type: 'patrol',
            resources_checked: 50,
            error_count: 3,
            status: 'error',
          }),
        ],
      }),
    ).toMatchObject({ title: 'Coverage incomplete' });
  });

  it('treats a full run with zero resources as not successful so the coverage gap stays', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 70,
          grade: 'C',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete.',
        }),
        runs: [makeRun({ type: 'patrol', resources_checked: 0, error_count: 0 })],
      }),
    ).toMatchObject({ title: 'Coverage incomplete' });
  });

  it('treats a scoped-only run as not a full coverage run', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 70,
          grade: 'C',
          factors: [coverageFactor()],
          prediction: 'Coverage is incomplete.',
        }),
        runs: [makeRun({ type: 'scoped', resources_checked: 5, error_count: 0 })],
      }),
    ).toMatchObject({ title: 'Coverage incomplete' });
  });

  it('clears the coverage gap after a successful full run', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: makeHealth({
          score: 85,
          grade: 'B',
          factors: [coverageFactor()],
          prediction: 'A few warnings were detected.',
        }),
        runs: [successfulFullRun(60)],
      }),
    ).toMatchObject({ title: 'Health requires attention' });
  });
});

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
// formatRecencyResourcesCheckedLabel singular verified case.
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
  it('uses "verified 1 resource" (singular) for a successful full run with one resource', () => {
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
      resourcesCheckedLabel: 'verified 1 resource',
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
