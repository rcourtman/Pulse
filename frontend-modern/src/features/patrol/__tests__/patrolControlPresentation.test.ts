import { describe, expect, it } from 'vitest';

import type { PatrolAutonomyLevel } from '@/api/patrol';
import {
  getPatrolProInvestigationHandoff,
  getPatrolQueueBadgeLabel,
  getPatrolQueueWorkspaceDescription,
  getPatrolReadyWorkDetail,
  getPatrolSetupIssueReason,
  PATROL_AUTONOMY_POLICY_PRESENTATION,
  PATROL_WORKSPACE_QUEUE_TITLE,
} from '../patrolControlPresentation';

describe('patrolControlPresentation', () => {
  it('keeps the four Patrol mode options in user-facing language', () => {
    expect(PATROL_AUTONOMY_POLICY_PRESENTATION).toEqual({
      monitor: {
        label: 'Watch only',
        detail: 'Patrol reports issues without making changes.',
        compactLabel: 'Watch only',
      },
      approval: {
        label: 'Ask first',
        detail: 'Patrol investigates issues and prepares fixes. You approve every change.',
        compactLabel: 'Ask first',
      },
      assisted: {
        label: 'Safe auto-fix',
        detail: 'Patrol fixes safe policy-allowed issues. It asks before anything riskier.',
        compactLabel: 'Safe auto-fix',
      },
      full: {
        label: 'Autopilot',
        detail:
          'Patrol handles policy-approved issues automatically. It asks only when policy requires approval.',
        compactLabel: 'Autopilot',
      },
    });
  });

  it.each([
    [
      'monitor',
      'Patrol is ready to check infrastructure and show issues.',
      'Issues Patrol finds that need you appear here.',
    ],
    [
      'approval',
      'Patrol is ready to watch and investigate. You approve every change.',
      'Investigations and approval requests that need you appear here.',
    ],
    [
      'assisted',
      'Patrol is ready to watch, investigate, and handle safe fixes when policy allows it.',
      'Issues Patrol is handling appear here. Approval requests appear when needed.',
    ],
    [
      'full',
      'Patrol is ready to watch, investigate, and act automatically within your policy.',
      'Issues Patrol is handling appear here. Approval requests appear when policy requires them.',
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
      'Patrol is ready to check infrastructure and show issues.',
    );
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'full',
        autonomyLocked: true,
      }),
    ).toBe('Issues Patrol finds that need you appear here.');
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
    ).toBe('2 issues on 1 affected resource.');
    expect(
      getPatrolQueueWorkspaceDescription({
        autonomyLevel: 'approval',
        findingCount: 1,
        affectedResourceCount: 1,
      }),
    ).toBe('1 issue on 1 affected resource.');
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
