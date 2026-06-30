import type { PatrolAutonomyLevel, PatrolRunRecord, PatrolStatus } from '@/api/patrol';
import type { MetadataBadgeTone } from '@/components/shared/MetadataBadge';
import {
  getPatrolFindingIssueCountLabel,
  getPatrolWorkTypeCompositionClause,
} from '@/utils/aiFindingPresentation';
import type { PatrolWorkTypeComposition } from '@/utils/aiFindingPresentation';
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
  workTypeComposition?: PatrolWorkTypeComposition;
}

export interface PatrolWorkspaceWorkGroupSummary {
  detail: string;
  id: 'approvals' | 'failed-actions' | 'failed-check' | 'recurring' | 'stale-protection';
  label: string;
  tone: MetadataBadgeTone;
}

interface PatrolWorkspaceWorkGroupsInput {
  latestRun?: Pick<PatrolRunRecord, 'error_count' | 'resources_checked' | 'status'> | null;
  nowMs?: number;
  patrolStatus?: Pick<PatrolStatus, 'error_count' | 'next_patrol_at' | 'running'> | null;
  pendingApprovalCount?: number;
  workTypeComposition?: PatrolWorkTypeComposition;
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

function normalizeStatus(value: string | undefined): string {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');
}

function hasFailedPatrolCheck(
  run: Pick<PatrolRunRecord, 'error_count' | 'status'> | null | undefined,
): boolean {
  if (!run) return false;
  return normalizeCount(run.error_count) > 0 || normalizeStatus(run.status) === 'error';
}

function isScheduledPatrolOverdue(
  status: Pick<PatrolStatus, 'next_patrol_at' | 'running'> | null | undefined,
  nowMs: number,
): boolean {
  if (!status || status.running || !status.next_patrol_at) return false;
  const nextPatrolAt = Date.parse(status.next_patrol_at);
  return Number.isFinite(nextPatrolAt) && nextPatrolAt < nowMs;
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
    const compositionClause = input.workTypeComposition
      ? getPatrolWorkTypeCompositionClause(input.workTypeComposition)
      : '';
    return `Patrol found ${getPatrolFindingIssueCountLabel(findingCount)} on ${formatCount(
      affectedResourceCount,
      'affected resource',
    )}${compositionClause}. ${getPatrolQueueActionDetail(input)}`;
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

export function getPatrolWorkspaceWorkGroups(
  input: PatrolWorkspaceWorkGroupsInput,
): PatrolWorkspaceWorkGroupSummary[] {
  const groups: PatrolWorkspaceWorkGroupSummary[] = [];
  const composition = input.workTypeComposition;
  const pendingApprovalCount = normalizeCount(input.pendingApprovalCount);
  const failedActionCount = normalizeCount(composition?.failed);
  const recurringIssueCount = normalizeCount(composition?.recurring);
  const latestRun = input.latestRun ?? null;
  const latestRunFailed = hasFailedPatrolCheck(latestRun);
  const statusErrorCount = normalizeCount(input.patrolStatus?.error_count);

  if (pendingApprovalCount > 0) {
    groups.push({
      id: 'approvals',
      label: `${formatCount(pendingApprovalCount, 'approval')} waiting`,
      detail: 'Approve or reject requested fixes from the issue rows.',
      tone: 'warning',
    });
  }

  if (failedActionCount > 0) {
    groups.push({
      id: 'failed-actions',
      label: `${formatCount(failedActionCount, 'failed action')}`,
      detail: 'Review the failed action and verification state in the affected issue row.',
      tone: 'danger',
    });
  }

  if (latestRunFailed || statusErrorCount > 0) {
    const checkedResources = normalizeCount(latestRun?.resources_checked);
    groups.push({
      id: 'failed-check',
      label: 'Latest check needs review',
      detail:
        checkedResources > 0
          ? `Patrol checked ${formatCount(checkedResources, 'resource')} but ended with runtime issues.`
          : 'The last Patrol check ended with runtime issues.',
      tone: 'danger',
    });
  }

  if (recurringIssueCount > 0) {
    groups.push({
      id: 'recurring',
      label: `${formatCount(recurringIssueCount, 'recurring issue')}`,
      detail: 'Current work includes issues that reappeared after earlier resolution.',
      tone: 'warning',
    });
  }

  if (isScheduledPatrolOverdue(input.patrolStatus, input.nowMs ?? Date.now())) {
    groups.push({
      id: 'stale-protection',
      label: 'Check overdue',
      detail: 'The next scheduled Patrol check is overdue; run Patrol when the system is ready.',
      tone: 'warning',
    });
  }

  return groups;
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
