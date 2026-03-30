import { describe, expect, it } from 'vitest';
import {
  getPatrolAssessmentPresentation,
  getPatrolAssessmentAction,
  getPatrolRecencyPresentation,
  getPatrolScoreChipLabel,
  getPatrolSummaryMetricState,
  getPatrolVerificationPresentation,
  getPatrolNoIssuesPresentation,
  getPatrolSummaryPresentation,
  PATROL_NO_ISSUES_LABEL,
} from '@/utils/patrolSummaryPresentation';

describe('getPatrolSummaryPresentation', () => {
  it('returns active critical presentation', () => {
    expect(getPatrolSummaryPresentation('critical', true)).toEqual({
      iconClass: 'text-red-500 dark:text-red-400',
      iconContainerClass: 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800',
      valueClass: 'text-red-600 dark:text-red-400',
    });
  });

  it('returns quiet presentation when the metric is zero', () => {
    expect(getPatrolSummaryPresentation('warning', false)).toEqual({
      iconClass: 'text-muted',
      iconContainerClass: 'bg-surface border-border',
      valueClass: 'text-muted',
    });
  });

  it('exports canonical patrol empty-state copy', () => {
    expect(PATROL_NO_ISSUES_LABEL).toBe('No issues found');
  });

  it('promotes coverage gaps into the primary patrol assessment state', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 70,
          grade: 'C',
          trend: 'stable',
          factors: [
            {
              name: 'Patrol coverage incomplete',
              impact: -0.35,
              description: 'Patrol coverage is incomplete.',
              category: 'coverage',
            },
          ],
          prediction: 'Patrol coverage is incomplete.',
        },
      }),
    ).toEqual({
      title: 'Coverage incomplete',
      description: 'Patrol coverage is incomplete.',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Coverage incomplete',
      tone: 'warning',
    });
  });

  it('reports issues detected when active warning findings exist', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 84,
          grade: 'B',
          trend: 'stable',
          factors: [],
          prediction: 'Warnings require review.',
        },
        warningFindings: 2,
      }),
    ).toEqual({
      title: 'Issues detected',
      description: 'Warnings require review.',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'warning',
    });
  });

  it('combines active findings with the coverage caveat when both are present', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 60,
          grade: 'C',
          trend: 'stable',
          factors: [
            {
              name: 'Patrol coverage incomplete',
              impact: -0.35,
              description: 'Patrol coverage is incomplete.',
              category: 'coverage',
            },
          ],
          prediction:
            'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
        },
        warningFindings: 1,
      }),
    ).toEqual({
      title: 'Issues detected',
      description:
        'Patrol surfaced 1 active warning finding. Recent coverage is also incomplete, so the rest of your infrastructure is not fully verified.',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'warning',
    });
  });

  it('classifies patrol-owned service failures as runtime issues instead of infrastructure issues', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 60,
          grade: 'C',
          trend: 'stable',
          factors: [
            {
              name: 'Patrol coverage incomplete',
              impact: -0.35,
              description: 'Patrol coverage is incomplete.',
              category: 'coverage',
            },
          ],
          prediction:
            'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
        },
        warningFindings: 1,
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Insufficient API credits',
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Patrol runtime issue',
      description:
        'Patrol has an active runtime issue: Insufficient API credits. Recent coverage is also incomplete, so the rest of your infrastructure is not fully verified.',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Patrol runtime issue',
      tone: 'warning',
    });
  });

  it('uses assessment labeling for coverage or runtime-limited score states', () => {
    expect(
      getPatrolScoreChipLabel({
        overallHealth: {
          score: 60,
          grade: 'C',
          trend: 'stable',
          factors: [
            {
              name: 'Patrol coverage incomplete',
              impact: -0.35,
              description: 'Patrol coverage is incomplete.',
              category: 'coverage',
            },
          ],
          prediction: 'Patrol coverage is incomplete.',
        },
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Insufficient API credits',
          },
        ] as never,
      }),
    ).toBe('Assessment');
  });

  it('offers a direct AI settings action for Patrol runtime issues', () => {
    expect(
      getPatrolAssessmentAction({
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Insufficient API credits',
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Open AI Settings',
      href: '/settings/system-ai',
    });
  });

  it('does not offer an AI settings action for infrastructure findings', () => {
    expect(
      getPatrolAssessmentAction({
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'vm-100',
            resourceName: 'web-1',
            title: 'Disk nearly full',
          },
        ] as never,
      }),
    ).toBeUndefined();
  });

  it('splits infrastructure findings from patrol runtime issues in supporting metrics', () => {
    expect(
      getPatrolSummaryMetricState({
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Insufficient API credits',
          },
        ] as never,
        fixedCount: 0,
      }),
    ).toEqual({
      primaryLabel: 'Infrastructure findings',
      primaryValue: 0,
      primarySeverity: 'warning',
      secondaryLabel: 'Runtime issues',
      secondaryValue: 1,
      secondarySeverity: 'warning',
      criticalLabel: 'Critical',
      criticalValue: 0,
      fixedLabel: 'Fixed',
      fixedValue: 0,
    });
  });

  it('keeps active findings and warnings labels when only infrastructure findings are active', () => {
    expect(
      getPatrolSummaryMetricState({
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'vm-101',
            resourceName: 'db-01',
            title: 'Disk nearly full',
          },
        ] as never,
        fixedCount: 2,
      }),
    ).toEqual({
      primaryLabel: 'Active findings',
      primaryValue: 1,
      primarySeverity: 'warning',
      secondaryLabel: 'Warnings',
      secondaryValue: 1,
      secondarySeverity: 'warning',
      criticalLabel: 'Critical',
      criticalValue: 0,
      fixedLabel: 'Fixed',
      fixedValue: 2,
    });
  });

  it('keeps health labeling for verified healthy states', () => {
    expect(
      getPatrolScoreChipLabel({
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
        activeFindings: [],
      }),
    ).toBe('Health');
  });

  it('keeps no-issues copy only for fully healthy patrol states', () => {
    expect(
      getPatrolNoIssuesPresentation({
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
      }),
    ).toEqual({
      label: 'No issues found',
      tone: 'success',
    });
  });

  it('reports recent successful full patrol verification', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:57:00Z',
            duration_ms: 420000,
            type: 'patrol',
            resources_checked: 58,
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
            existing_findings: 1,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'issues_found',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Recently verified',
      description: 'The most recent full patrol completed successfully and checked 58 resources.',
      compactLabel: 'Recently verified',
      tone: 'success',
      lastFullRunAt: '2026-03-12T09:57:00Z',
    });
  });

  it('adds an activity mix when scoped runs make recent verification look busy', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-scoped-alert',
            started_at: '2026-03-12T10:00:00Z',
            completed_at: '2026-03-12T10:01:00Z',
            duration_ms: 60000,
            type: 'scoped',
            trigger_reason: 'alert_fired',
            resources_checked: 1,
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
            finding_ids: [],
            error_count: 0,
            status: 'healthy',
            triage_flags: 0,
            tool_call_count: 0,
          },
          {
            id: 'run-scoped-anomaly',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
            duration_ms: 60000,
            type: 'scoped',
            trigger_reason: 'anomaly',
            resources_checked: 1,
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
            finding_ids: [],
            error_count: 0,
            status: 'healthy',
            triage_flags: 0,
            tool_call_count: 0,
          },
          {
            id: 'run-full',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:57:00Z',
            duration_ms: 420000,
            type: 'patrol',
            resources_checked: 58,
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
            existing_findings: 1,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'issues_found',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Recently verified',
      description: 'The most recent full patrol completed successfully and checked 58 resources.',
      compactLabel: 'Recently verified',
      tone: 'success',
      lastFullRunAt: '2026-03-12T09:57:00Z',
      activityMixLabel: '1 full, 1 alert-triggered, 1 anomaly-triggered',
    });
  });

  it('reports partial verification when only scoped runs are recent', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
            duration_ms: 60000,
            type: 'scoped',
            trigger_reason: 'alert_fired',
            resources_checked: 1,
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
            finding_ids: [],
            error_count: 1,
            status: 'error',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'No recent full patrol',
      description:
        'Recent activity was limited to scoped alert fired runs over 1 resource, so Patrol has not recently re-verified your full infrastructure.',
      compactLabel: 'Partial verification',
      tone: 'warning',
    });
  });

  it('reports limited verification when only verification checks are recent', () => {
    expect(
      getPatrolVerificationPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
            duration_ms: 60000,
            type: 'verification',
            trigger_reason: 'verification',
            resources_checked: 1,
            nodes_checked: 1,
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
            resolved_findings: 1,
            auto_fix_count: 0,
            findings_summary: 'Verification: issue resolved',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'healthy',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      title: 'No recent full patrol',
      description:
        'Recent activity was limited to verification checks over 1 resource, so Patrol has not recently re-verified your full infrastructure.',
      compactLabel: 'Partial verification',
      tone: 'warning',
    });
  });

  it('labels scoped recency as activity rather than patrol', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:58:00Z',
            completed_at: '2026-03-12T09:59:00Z',
            duration_ms: 60000,
            type: 'scoped',
            trigger_reason: 'alert_fired',
            resources_checked: 1,
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
            finding_ids: [],
            error_count: 1,
            status: 'error',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last activity',
      timestamp: '2026-03-12T09:59:00Z',
    });
  });

  it('labels full patrol recency explicitly when the latest completed run is full', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-1',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:57:00Z',
            duration_ms: 420000,
            type: 'patrol',
            resources_checked: 58,
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
            existing_findings: 1,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 0,
            status: 'issues_found',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last full patrol',
      timestamp: '2026-03-12T09:57:00Z',
    });
  });

  it('prefers explicit last activity transport over last full patrol transport when no run history is loaded', () => {
    expect(
      getPatrolRecencyPresentation({
        lastPatrolAt: '2026-03-12T09:57:00Z',
        lastActivityAt: '2026-03-12T09:59:00Z',
      }),
    ).toEqual({
      label: 'Last activity',
      timestamp: '2026-03-12T09:59:00Z',
    });
  });
});
