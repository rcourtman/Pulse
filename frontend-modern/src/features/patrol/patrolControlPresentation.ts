import type { PatrolAutonomyLevel } from '@/api/patrol';
import { getPatrolFindingIssueCountLabel } from '@/utils/aiFindingPresentation';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

export const PATROL_AUTONOMY_POLICY_PRESENTATION: Record<
  PatrolAutonomyLevel,
  { label: string; detail: string; compactLabel?: string }
> = {
  monitor: {
    label: 'Watch only',
    detail: 'Patrol checks and reports issues without making changes.',
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
};

export const PATROL_WORKSPACE_HISTORY_DESCRIPTION = 'Past checks and what Patrol recorded.';

export const PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION = 'What Patrol found during this run.';

export const PATROL_WORKSPACE_SETUP_TITLE = 'Fix provider';

export const PATROL_WORKSPACE_SETUP_DESCRIPTION =
  'Open Provider & Models, then run Patrol from this page.';

export const PATROL_WORKSPACE_QUEUE_TITLE = 'Open work';

export const PATROL_WORKSPACE_RUN_RECORD_TITLE = 'Check details';

interface PatrolControlCopyInput {
  autonomyLevel?: PatrolAutonomyLevel;
  autonomyLocked?: boolean;
}

interface PatrolQueueCountInput {
  affectedResourceCount?: number;
  findingCount?: number;
}

interface PatrolSetupIssueReasonInput {
  setupFindingTitle?: string;
  readinessSummary?: string;
  triggerDisabledReason?: string;
  blockedReason?: string;
}

function normalizeCount(value: number | undefined): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.trunc(value));
}

function normalizeText(value: string | undefined): string {
  return value?.trim() ?? '';
}

function formatCount(count: number, singular: string, plural = `${singular}s`): string {
  return `${count} ${count === 1 ? singular : plural}`;
}

function getPatrolQueueActionDetail(input: PatrolControlCopyInput): string {
  if (input.autonomyLocked) {
    return 'Open a row to review evidence and record the outcome.';
  }

  switch (input.autonomyLevel) {
    case 'approval':
      return 'Open a row to review evidence, approve any change, and verify the outcome.';
    case 'assisted':
      return 'Open a row to see safe fixes, approval requests, and verification.';
    case 'full':
      return 'Open a row to see automatic actions, policy approvals, and verification.';
    case 'monitor':
    default:
      return 'Open a row to review evidence and record the outcome.';
  }
}

export function getPatrolSetupIssueReason(input: PatrolSetupIssueReasonInput): string {
  const setupFindingTitle = normalizeText(input.setupFindingTitle);
  if (setupFindingTitle) return setupFindingTitle;

  const readinessSummary = normalizeText(input.readinessSummary);
  if (readinessSummary) {
    if (
      /\btool(?:_| )?calls?\b/i.test(readinessSummary) ||
      /\brun tools?\b/i.test(readinessSummary)
    ) {
      return 'Selected model cannot run Patrol tools.';
    }
    return readinessSummary;
  }

  return (
    normalizeText(input.triggerDisabledReason) ||
    normalizeText(input.blockedReason) ||
    'Provider settings need attention'
  );
}

export function getPatrolReadyWorkDetail(input: PatrolControlCopyInput): string {
  if (input.autonomyLocked) {
    return 'Patrol is ready to check infrastructure and list current issues.';
  }

  switch (input.autonomyLevel) {
    case 'approval':
      return 'Patrol is ready to check, investigate, and ask before any change.';
    case 'assisted':
      return 'Patrol is ready to check, investigate, and fix safe issues when policy allows it.';
    case 'full':
      return 'Patrol is ready to check, investigate, and act automatically within your policy.';
    case 'monitor':
    default:
      return 'Patrol is ready to check infrastructure and list current issues.';
  }
}

export function getPatrolQueueBadgeLabel(input: PatrolQueueCountInput): string | undefined {
  const affectedResourceCount = normalizeCount(input.affectedResourceCount);
  if (affectedResourceCount <= 0) {
    return undefined;
  }
  return formatCount(affectedResourceCount, 'resource');
}

export function getPatrolQueueWorkspaceDescription(
  input: PatrolControlCopyInput & PatrolQueueCountInput,
): string {
  const findingCount = normalizeCount(input.findingCount);
  const affectedResourceCount = normalizeCount(input.affectedResourceCount);
  if (findingCount > 0 && affectedResourceCount > 0) {
    return `Patrol found ${getPatrolFindingIssueCountLabel(findingCount)} on ${formatCount(
      affectedResourceCount,
      'affected resource',
    )}. ${getPatrolQueueActionDetail(input)}`;
  }

  if (input.autonomyLocked) {
    return 'Patrol lists current issues here after each check. History keeps past outcomes.';
  }

  switch (input.autonomyLevel) {
    case 'approval':
      return 'Patrol lists investigations, approvals, and verification results here.';
    case 'assisted':
      return 'Patrol lists issues it can fix, approvals it needs, and verification results here.';
    case 'full':
      return 'Patrol lists automatic work, policy approvals, and verification results here.';
    case 'monitor':
    default:
      return 'Patrol lists current issues here after each check. History keeps past outcomes.';
  }
}

const PATROL_PRO_HANDOFF_ACTIONABLE_SEVERITIES = new Set(['critical', 'warning']);

export interface PatrolProInvestigationHandoff {
  detail: string;
  actionLabel?: string;
  destination?: UpgradeDestination;
}

export interface PatrolProInvestigationHandoffInput {
  autoFixLocked: boolean;
  commercialSurfacesHidden: boolean;
  upgradePromptsHidden: boolean;
  upgradeDestination: UpgradeDestination;
  severity?: string;
  status?: string;
}

export function getPatrolProInvestigationHandoff(
  input: PatrolProInvestigationHandoffInput,
): PatrolProInvestigationHandoff | undefined {
  if (!input.autoFixLocked) return undefined;
  if (input.commercialSurfacesHidden) return undefined;
  if (input.status && input.status !== 'active') return undefined;
  if (!input.severity || !PATROL_PRO_HANDOFF_ACTIONABLE_SEVERITIES.has(input.severity)) {
    return undefined;
  }

  const handoff: PatrolProInvestigationHandoff = {
    detail: 'Pulse Pro can investigate and fix issues like this.',
  };
  if (!input.upgradePromptsHidden) {
    handoff.actionLabel = 'Learn about Pulse Pro';
    handoff.destination = input.upgradeDestination;
  }
  return handoff;
}
