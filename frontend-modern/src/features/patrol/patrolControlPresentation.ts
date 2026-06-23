import type { PatrolAutonomyLevel } from '@/api/patrol';
import { getPatrolFindingIssueCountLabel } from '@/utils/aiFindingPresentation';

export const PATROL_AUTONOMY_POLICY_PRESENTATION: Record<
  PatrolAutonomyLevel,
  { label: string; detail: string; compactLabel?: string }
> = {
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
};

export const PATROL_WORKSPACE_HISTORY_DESCRIPTION = 'Past Patrol checks.';

export const PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION = 'What Patrol found during this run.';

export const PATROL_WORKSPACE_SETUP_TITLE = 'Fix provider';

export const PATROL_WORKSPACE_SETUP_DESCRIPTION =
  'Open Provider & Models, then run Patrol from this page.';

export const PATROL_WORKSPACE_QUEUE_TITLE = 'Current work';

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
    return 'Patrol is ready to check infrastructure and show issues.';
  }

  switch (input.autonomyLevel) {
    case 'approval':
      return 'Patrol is ready to watch and investigate. You approve every change.';
    case 'assisted':
      return 'Patrol is ready to watch, investigate, and handle safe fixes when policy allows it.';
    case 'full':
      return 'Patrol is ready to watch, investigate, and act automatically within your policy.';
    case 'monitor':
    default:
      return 'Patrol is ready to check infrastructure and show issues.';
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
    return `${getPatrolFindingIssueCountLabel(findingCount)} on ${formatCount(
      affectedResourceCount,
      'affected resource',
    )}.`;
  }

  if (input.autonomyLocked) {
    return 'Problems Patrol finds appear here.';
  }

  switch (input.autonomyLevel) {
    case 'approval':
      return 'Investigations and approval requests appear here.';
    case 'assisted':
      return 'Issues Patrol is handling appear here. Approval requests appear when needed.';
    case 'full':
      return 'Issues Patrol is handling appear here. Approval requests appear when policy requires them.';
    case 'monitor':
    default:
      return 'Problems Patrol finds appear here.';
  }
}
