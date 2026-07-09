import { describe, expect, it } from 'vitest';
import {
  getPatrolAssessmentPresentation,
  getPatrolAssessmentAction,
  getPatrolAssessmentShellPresentation,
  getPatrolCompactAssessmentLabel,
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
    expect(PATROL_NO_ISSUES_LABEL).toBe('No active issues');
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
      eyebrow: 'Status',
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
      eyebrow: 'Status',
      compactLabel: 'Issues detected',
      tone: 'warning',
    });
  });

  it('surfaces an active running Patrol instead of a stale all-clear summary', () => {
    expect(
      getPatrolAssessmentPresentation({
        runtimeState: 'running',
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
      }),
    ).toEqual({
      title: 'Patrol running',
      description: 'Patrol is checking your infrastructure now.',
      eyebrow: 'Status',
      compactLabel: 'Patrol is checking now',
      tone: 'info',
    });
  });

  it('does not reuse an all-clear health prediction while active infrastructure findings exist', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 95,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
        warningFindings: 1,
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'vm-100',
            resourceName: 'web-1',
            title: 'Backup age drift',
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Issues detected',
      description:
        'Patrol surfaced 1 active warning finding in your infrastructure. Review the active findings for more detail.',
      eyebrow: 'Status',
      compactLabel: 'Issues detected',
      tone: 'warning',
    });
  });

  it('does not reuse an all-clear health prediction while a Patrol runtime issue is active', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 95,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
        warningFindings: 1,
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Provider billing or quota issue',
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Patrol runtime issue',
      description:
        'Patrol has an active runtime issue: Provider billing or quota issue. Review the Patrol runtime issue for more detail.',
      eyebrow: 'Status',
      compactLabel: 'Patrol runtime issue',
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
            'Recent Patrol runs encountered errors, so the current health summary may be incomplete.',
        },
        warningFindings: 1,
      }),
    ).toEqual({
      title: 'Issues detected',
      description:
        'Patrol surfaced 1 active warning finding. Recent coverage is incomplete. Run Patrol to check everything.',
      eyebrow: 'Status',
      compactLabel: 'Issues detected',
      tone: 'warning',
    });
  });

  it('drops stale coverage caveats after a successful full patrol verified resources', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 85,
          grade: 'B',
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
            'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
        },
        warningFindings: 1,
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'backup-1',
            resourceName: 'delly',
            title: 'Backup failed',
          },
        ] as never,
        runs: [
          {
            completed_at: '2026-03-12T10:00:00Z',
            type: 'patrol',
            resources_checked: 58,
            error_count: 0,
            status: 'issues_found',
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Issues detected',
      description:
        'Patrol surfaced 1 active warning finding in your infrastructure. Review the active findings for more detail.',
      eyebrow: 'Status',
      compactLabel: 'Issues detected',
      tone: 'warning',
    });
  });

  it('keeps Patrol runtime issues distinct from infrastructure warning findings', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 65,
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
            'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
        },
        warningFindings: 2,
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Provider connection issue',
          },
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'backup-1',
            resourceName: 'delly',
            title: 'Backup failed',
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Issues detected',
      description:
        'Patrol surfaced 1 active warning finding in your infrastructure and 1 active Patrol runtime issue. Recent coverage is incomplete. Run Patrol to check everything.',
      eyebrow: 'Status',
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
            'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
        },
        warningFindings: 1,
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Provider billing or quota issue',
          },
        ] as never,
      }),
    ).toEqual({
      title: 'Patrol runtime issue',
      description:
        'Patrol has an active runtime issue: Provider billing or quota issue. Recent coverage is incomplete. Run Patrol to check everything.',
      eyebrow: 'Status',
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
            title: 'Pulse Patrol: Provider billing or quota issue',
          },
        ] as never,
      }),
    ).toBe('Assessment');
  });

  it('offers a direct Patrol provider settings action for Patrol runtime issues', () => {
    expect(
      getPatrolAssessmentAction({
        activeFindings: [
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Provider billing or quota issue',
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Check Patrol model',
      href: '/settings/pulse-intelligence/patrol',
    });
  });

  it('uses the shared neutral shell with a warning accent for degraded patrol summaries', () => {
    expect(getPatrolAssessmentShellPresentation('warning')).toEqual({
      headerClass: 'bg-amber-50/70 dark:bg-amber-950/30',
      badgeVariant: 'warning',
      iconClass: 'text-amber-600 dark:text-amber-300',
      iconContainerClass: 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/40',
    });
  });

  it('defaults patrol shell styling to the informational accent', () => {
    expect(getPatrolAssessmentShellPresentation()).toEqual({
      headerClass: 'bg-blue-50/70 dark:bg-blue-950/30',
      badgeVariant: 'info',
      iconClass: 'text-blue-600 dark:text-blue-300',
      iconContainerClass: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950/40',
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
            title: 'Pulse Patrol: Provider billing or quota issue',
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

  it('names compact assessment evidence instead of exposing bare counters', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'No active issues',
        overallHealth: {
          score: 95,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
        activeFindings: [],
        historicalRegressionCount: 1,
      }),
    ).toBe('No active issues · 1 past regression · health score 95/100');
  });

  it('uses issue nouns for compact active finding summaries', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'Issues detected',
        overallHealth: {
          score: 84,
          grade: 'B',
          trend: 'stable',
          factors: [],
          prediction: 'Warnings require review.',
        },
        activeFindings: [
          {
            status: 'active',
            severity: 'critical',
            resourceId: 'vm-100',
            resourceName: 'web-1',
            title: 'High CPU usage',
          },
          {
            status: 'active',
            severity: 'warning',
            resourceId: 'ai-service',
            resourceName: 'Pulse Patrol Service',
            title: 'Pulse Patrol: Provider connection issue',
          },
        ] as never,
      }),
    ).toBe('1 critical issue · 1 Patrol runtime issue · health score 84/100');
  });

  it('can suppress stale health scores while Patrol is still running', () => {
    expect(
      getPatrolCompactAssessmentLabel({
        assessmentLabel: 'Patrol is checking now',
        overallHealth: {
          score: 65,
          grade: 'C',
          trend: 'stable',
          factors: [],
          prediction: 'Previous assessment needs attention.',
        },
        includeHealthScore: false,
      }),
    ).toBe('Patrol is checking now');
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
      label: 'No active issues',
      tone: 'success',
    });
  });

  it('reports a recent successful Patrol check', () => {
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
      title: 'Recently checked',
      description: 'The most recent Patrol check completed successfully and covered 58 resources.',
      compactLabel: 'Recently checked',
      tone: 'success',
      lastFullRunAt: '2026-03-12T09:57:00Z',
    });
  });

  it('adds a check mix when targeted runs make recent activity look busy', () => {
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
      title: 'Recently checked',
      description: 'The most recent Patrol check completed successfully and covered 58 resources.',
      compactLabel: 'Recently checked',
      tone: 'success',
      lastFullRunAt: '2026-03-12T09:57:00Z',
      activityMixLabel: '1 full check, 1 alert-triggered check, 1 anomaly-triggered check',
    });
  });

  it('reports a partial check when only targeted runs are recent', () => {
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
      title: 'Needs full check',
      description: 'Recent targeted checks covered 1 resource. Run Patrol to check everything.',
      compactLabel: 'Partial check',
      tone: 'warning',
    });
  });

  it('reports a partial check when only follow-up checks are recent', () => {
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
      title: 'Needs full check',
      description: 'Recent follow-up checks covered 1 resource. Run Patrol to check everything.',
      compactLabel: 'Partial check',
      tone: 'warning',
    });
  });

  it('labels targeted recency as the last check', () => {
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
      label: 'Last check',
      timestamp: '2026-03-12T09:59:00Z',
      resourcesChecked: 1,
      resourcesCheckedLabel: 'checked 1 resource',
    });
  });

  it('labels completed Patrol recency as the last check', () => {
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
      label: 'Last check',
      timestamp: '2026-03-12T09:57:00Z',
      resourcesChecked: 58,
      resourcesCheckedLabel: 'verified 58 resources',
    });
  });

  it('uses checked coverage wording when a full patrol ends with errors', () => {
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-error',
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
            new_findings: 1,
            existing_findings: 0,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: '1 warning',
            finding_ids: ['finding-1'],
            error_count: 1,
            status: 'error',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-03-12T09:57:00Z',
      resourcesChecked: 58,
      resourcesCheckedLabel: 'checked 58 resources',
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

  it('omits resourcesChecked when the most recent completed run reports zero coverage', () => {
    // A run that completed without checking any resources (e.g. an early
    // failure or a no-op trigger fire) should not surface a "verified 0
    // resources" line on the page header — that reads as an alarm signal
    // when it's just a degenerate run. The presentation must omit the
    // field instead of returning resourcesChecked: 0 so render code's
    // truthy <Show> gate cleanly hides the coverage span.
    expect(
      getPatrolRecencyPresentation({
        runs: [
          {
            id: 'run-empty',
            started_at: '2026-03-12T09:50:00Z',
            completed_at: '2026-03-12T09:51:00Z',
            duration_ms: 60000,
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
            finding_ids: [],
            error_count: 0,
            status: 'no_issues',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ] as never,
      }),
    ).toEqual({
      label: 'Last check',
      timestamp: '2026-03-12T09:51:00Z',
    });
  });
});
