import { describe, expect, it } from 'vitest';

import type { PatrolAutonomyLevel } from '@/api/patrol';
import {
  getPatrolProInvestigationHandoff,
  getPatrolQueueBadgeLabel,
  getPatrolQueueWorkspaceDescription,
  getMonitorContextPatrolProtectionPosture,
  getPatrolReadyWorkDetail,
  getPatrolSetupIssueReason,
  getPatrolWorkspaceWorkGroups,
  isPatrolCoverageStale,
  PATROL_AUTONOMY_POLICY_PRESENTATION,
  PATROL_WORKSPACE_QUEUE_TITLE,
} from '../patrolControlPresentation';

describe('patrolControlPresentation', () => {
  it('keeps the four Patrol mode options in user-facing language', () => {
    expect(PATROL_AUTONOMY_POLICY_PRESENTATION).toEqual({
      monitor: {
        label: 'Watch only',
        detail: 'Patrol checks infrastructure and reports issues only; it does not start fixes.',
        compactLabel: 'Watch only',
      },
      approval: {
        label: 'Ask first',
        detail:
          'Patrol investigates and prepares fixes, but every change waits for your approval.',
        compactLabel: 'Ask first',
      },
      assisted: {
        label: 'Safe auto-fix',
        detail:
          'Patrol can run low- or medium-risk fixes allowed by policy; higher-risk work still asks first.',
        compactLabel: 'Safe auto-fix',
      },
      full: {
        label: 'Autopilot',
        detail:
          'Patrol can act automatically within policy and still asks when approval is required.',
        compactLabel: 'Autopilot',
      },
    });
  });

  it.each([
    [
      'monitor',
      'Patrol is ready to check infrastructure and list current issues.',
      'Current Patrol issues appear here.',
    ],
    [
      'approval',
      'Patrol is ready to check, investigate, and ask before any change.',
      'Current issues, investigations, and approvals appear here.',
    ],
    [
      'assisted',
      'Patrol is ready to check, investigate, and fix safe issues when policy allows it.',
      'Current issues, fixes, and approvals appear here.',
    ],
    [
      'full',
      'Patrol is ready to check, investigate, and act automatically within your policy.',
      'Current issues, automatic work, and approvals appear here.',
    ],
  ] satisfies Array<[PatrolAutonomyLevel, string, string]>)(
    'describes %s mode without generic control-level wording',
    (autonomyLevel, currentWorkDetail, workspaceDescription) => {
      expect(PATROL_WORKSPACE_QUEUE_TITLE).toBe('Open work');
      expect(getPatrolReadyWorkDetail({ autonomyLevel })).toBe(currentWorkDetail);
      expect(getPatrolQueueWorkspaceDescription({ autonomyLevel })).toBe(workspaceDescription);
    },
  );

  it('does not advertise investigation or fixes while Patrol mode is locked', () => {
    expect(getPatrolReadyWorkDetail({ autonomyLevel: 'full', autonomyLocked: true })).toBe(
      'Patrol is ready to check infrastructure and list current issues.',
    );
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'full',
        autonomyLocked: true,
      }),
    ).toBe('Current Patrol issues appear here.');
  });

  it('summarizes the open queue by affected resource instead of raw findings', () => {
    expect(getPatrolQueueBadgeLabel({ findingCount: 2, affectedResourceCount: 1 })).toBe(
      '1 resource',
    );
    expect(getPatrolQueueBadgeLabel({ findingCount: 3, affectedResourceCount: 2 })).toBe(
      '2 resources',
    );
    expect(getPatrolQueueBadgeLabel({ findingCount: 2, affectedResourceCount: 0 })).toBeUndefined();
    expect(getPatrolQueueBadgeLabel({ findingCount: 0, affectedResourceCount: 0 })).toBeUndefined();
  });

  it('uses the grouped queue count when open findings exist', () => {
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'monitor',
        findingCount: 2,
        affectedResourceCount: 1,
      }),
    ).toBe(
      'Patrol found 2 issues on 1 affected resource. Open a row to review evidence and record the outcome.',
    );
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'approval',
        findingCount: 1,
        affectedResourceCount: 1,
      }),
    ).toBe(
      'Patrol found 1 issue on 1 affected resource. Open a row to review evidence, approve any change, and verify the outcome.',
    );
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'assisted',
        findingCount: 2,
        affectedResourceCount: 2,
      }),
    ).toBe(
      'Patrol found 2 issues on 2 affected resources. Open a row to see safe fixes, approval requests, and verification.',
    );
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'full',
        findingCount: 3,
        affectedResourceCount: 2,
      }),
    ).toBe(
      'Patrol found 3 issues on 2 affected resources. Open a row to see automatic actions, policy approvals, and verification.',
    );
  });

  it('includes work-type composition in the description when notable types exist', () => {
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'monitor',
        findingCount: 3,
        affectedResourceCount: 2,
        workTypeComposition: {
          total: 3,
          approval: 1,
          failed: 0,
          inProgress: 0,
          recurring: 1,
          newIssues: 1,
        },
      }),
    ).toBe(
      'Patrol found 3 issues on 2 affected resources — 1 needs approval, 1 recurring. Open a row to review evidence and record the outcome.',
    );
  });

  it('omits the composition clause when all findings are new', () => {
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'monitor',
        findingCount: 2,
        affectedResourceCount: 2,
        workTypeComposition: {
          total: 2,
          approval: 0,
          failed: 0,
          inProgress: 0,
          recurring: 0,
          newIssues: 2,
        },
      }),
    ).toBe(
      'Patrol found 2 issues on 2 affected resources. Open a row to review evidence and record the outcome.',
    );
  });

  it('does not add work-group chrome for ordinary active findings alone', () => {
    expect(
      getPatrolWorkspaceWorkGroups({
        workTypeComposition: {
          total: 1,
          approval: 0,
          failed: 0,
          inProgress: 0,
          recurring: 0,
          newIssues: 1,
        },
      }),
    ).toEqual([]);
  });

  it('groups approvals, failed checks, recurring issues, and stale protection as Patrol work', () => {
    expect(
      getPatrolWorkspaceWorkGroups({
        latestRun: {
          error_count: 1,
          resources_checked: 4,
          status: 'error',
        },
        nowMs: Date.parse('2026-06-30T15:00:00Z'),
        patrolStatus: {
          error_count: 1,
          next_patrol_at: '2026-06-30T14:05:00Z',
          running: false,
        },
        pendingApprovalCount: 2,
        workTypeComposition: {
          total: 3,
          approval: 1,
          failed: 1,
          inProgress: 0,
          recurring: 1,
          newIssues: 1,
        },
      }),
    ).toEqual([
      {
        detail: 'Approve or reject requested fixes from the issue rows.',
        id: 'approvals',
        label: '2 approvals waiting',
        tone: 'warning',
      },
      {
        detail: 'Review the failed action and verification state in the affected issue row.',
        id: 'failed-actions',
        label: '1 failed action',
        tone: 'danger',
      },
      {
        detail: 'Patrol checked 4 resources but ended with runtime issues.',
        id: 'failed-check',
        label: 'Latest check needs review',
        tone: 'danger',
      },
      {
        detail: 'Current work includes issues that reappeared after earlier resolution.',
        id: 'recurring',
        label: '1 recurring issue',
        tone: 'warning',
      },
      {
        detail:
          'Patrol has not completed a fresh full check; run Patrol to refresh current coverage.',
        id: 'stale-protection',
        label: 'Coverage stale',
        tone: 'warning',
      },
    ]);
  });

  it('treats old Patrol evidence as stale relative to the configured interval', () => {
    const nowMs = Date.parse('2026-06-30T15:00:00Z');

    expect(
      isPatrolCoverageStale({
        latestRun: { completed_at: '2026-06-28T12:00:00Z' },
        nowMs,
        patrolStatus: {
          interval_ms: 6 * 60 * 60 * 1000,
          running: false,
        },
      }),
    ).toBe(true);
    expect(
      isPatrolCoverageStale({
        latestRun: { completed_at: '2026-06-30T12:00:00Z' },
        nowMs,
        patrolStatus: {
          interval_ms: 6 * 60 * 60 * 1000,
          running: false,
        },
      }),
    ).toBe(false);
  });

  it('does not call future or running scheduled checks stale protection', () => {
    expect(
      getPatrolWorkspaceWorkGroups({
        nowMs: Date.parse('2026-06-30T13:00:00Z'),
        patrolStatus: {
          error_count: 0,
          next_patrol_at: '2026-06-30T14:05:00Z',
          running: false,
        },
      }),
    ).toEqual([]);
    expect(
      getPatrolWorkspaceWorkGroups({
        nowMs: Date.parse('2026-06-30T15:00:00Z'),
        patrolStatus: {
          error_count: 0,
          next_patrol_at: '2026-06-30T14:05:00Z',
          running: true,
        },
      }),
    ).toEqual([]);
  });

  it('surfaces monitor-context posture without reusing the Patrol empty-work labels', () => {
    expect(
      getMonitorContextPatrolProtectionPosture({
        findingCount: 0,
        latestRun: {
          error_count: 0,
          resources_checked: 4,
          status: 'healthy',
        },
        monitoredResourceCount: 4,
        nowMs: Date.parse('2026-06-30T13:00:00Z'),
        patrolStatus: {
          enabled: true,
          error_count: 0,
          findings_count: 0,
          healthy: true,
          next_patrol_at: '2026-06-30T14:05:00Z',
          resources_checked: 4,
          running: false,
          runtime_state: 'active',
        },
        pendingApprovalCount: 0,
        workTypeComposition: {
          total: 0,
          approval: 0,
          failed: 0,
          inProgress: 0,
          recurring: 0,
          newIssues: 0,
        },
      }),
    ).toEqual([
      {
        detail: 'Latest Patrol evidence is available while you review this monitor view.',
        id: 'coverage',
        label: 'Patrol checked 4 resources',
        tone: 'success',
      },
      {
        detail: 'Current Patrol findings and approvals stay in Patrol; none are waiting now.',
        id: 'open-work',
        label: 'No Patrol work waiting',
        tone: 'success',
      },
      {
        detail: 'Patrol is scheduled to check monitored resources again.',
        id: 'schedule',
        label: 'Next check scheduled',
        tone: 'info',
      },
    ]);
  });

  it('does not surface monitor-context posture without monitor resources or when Patrol has work', () => {
    const baseInput = {
      latestRun: {
        error_count: 0,
        resources_checked: 4,
        status: 'healthy',
      },
      nowMs: Date.parse('2026-06-30T13:00:00Z'),
      patrolStatus: {
        enabled: true,
        error_count: 0,
        findings_count: 0,
        healthy: true,
        next_patrol_at: '2026-06-30T14:05:00Z',
        resources_checked: 4,
        running: false,
        runtime_state: 'active',
      },
    } as const;

    expect(
      getMonitorContextPatrolProtectionPosture({
        ...baseInput,
        monitoredResourceCount: 0,
      }),
    ).toEqual([]);

    expect(
      getMonitorContextPatrolProtectionPosture({
        ...baseInput,
        findingCount: 1,
        monitoredResourceCount: 4,
      }),
    ).toEqual([]);

    expect(
      getMonitorContextPatrolProtectionPosture({
        ...baseInput,
        monitoredResourceCount: 4,
        pendingApprovalCount: 1,
      }),
    ).toEqual([]);

    expect(
      getMonitorContextPatrolProtectionPosture({
        ...baseInput,
        latestRun: {
          error_count: 1,
          resources_checked: 4,
          status: 'error',
        },
        monitoredResourceCount: 4,
      }),
    ).toEqual([]);
  });

  it('keeps setup-only issue reasons short and actionable', () => {
    expect(
      getPatrolSetupIssueReason({
        setupFindingTitle: 'Provider connection issue',
        readinessSummary: 'The selected Patrol model is a reasoning-only model family.',
      }),
    ).toBe('Provider connection issue');
    expect(
      getPatrolSetupIssueReason({
        readinessSummary:
          'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      }),
    ).toBe('Selected model cannot run Patrol tools.');
    expect(getPatrolSetupIssueReason({ triggerDisabledReason: 'Patrol is paused' })).toBe(
      'Patrol is paused',
    );
    expect(getPatrolSetupIssueReason({})).toBe('Provider settings need attention');
  });

  it('surfaces the Pulse Pro at-need handoff only for actionable active findings on plan-locked installs', () => {
    const destination = { href: '/settings/billing', external: false };
    const base = {
      autoFixLocked: true,
      commercialSurfacesHidden: false,
      upgradePromptsHidden: false,
      upgradeDestination: destination,
    };

    expect(
      getPatrolProInvestigationHandoff({ ...base, severity: 'critical', status: 'active' }),
    ).toMatchObject({
      detail: 'Pulse Pro can investigate and fix issues like this.',
      actionLabel: 'Learn about Pulse Pro',
      destination,
    });
    expect(
      getPatrolProInvestigationHandoff({ ...base, severity: 'warning', status: 'active' }),
    ).toBeDefined();

    expect(
      getPatrolProInvestigationHandoff({ ...base, severity: 'info', status: 'active' }),
    ).toBeUndefined();
    expect(
      getPatrolProInvestigationHandoff({ ...base, severity: 'critical', status: 'resolved' }),
    ).toBeUndefined();
    expect(
      getPatrolProInvestigationHandoff({
        ...base,
        autoFixLocked: false,
        severity: 'critical',
        status: 'active',
      }),
    ).toBeUndefined();
    expect(
      getPatrolProInvestigationHandoff({
        ...base,
        commercialSurfacesHidden: true,
        severity: 'critical',
        status: 'active',
      }),
    ).toBeUndefined();
  });

  it('keeps the at-need capability line but drops the upgrade action when upgrade prompts are hidden', () => {
    const handoff = getPatrolProInvestigationHandoff({
      autoFixLocked: true,
      commercialSurfacesHidden: false,
      upgradePromptsHidden: true,
      upgradeDestination: { href: '/settings/billing', external: false },
      severity: 'critical',
      status: 'active',
    });
    expect(handoff).toMatchObject({
      detail: 'Pulse Pro can investigate and fix issues like this.',
    });
    expect(handoff).not.toHaveProperty('actionLabel');
    expect(handoff).not.toHaveProperty('destination');
  });
});
