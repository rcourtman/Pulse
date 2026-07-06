import type { PatrolAutonomyLevel, PatrolRunRecord, PatrolStatus } from '@/api/patrol';
import type { MetadataBadgeTone } from '@/components/shared/MetadataBadge';
import {
  getPatrolFindingIssueCountLabel,
  getPatrolWorkTypeCompositionClause,
} from '@/utils/aiFindingPresentation';
import type { PatrolWorkTypeComposition } from '@/utils/aiFindingPresentation';
import { getPatrolRunCoverageSummary } from '@/utils/patrolRunPresentation';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

export const PATROL_AUTONOMY_POLICY_PRESENTATION: Record<
  PatrolAutonomyLevel,
  { label: string; detail: string; compactLabel?: string }
> = {
  monitor: {
    label: 'Watch only',
    detail: 'Patrol checks infrastructure and reports issues only; it does not start fixes.',
    compactLabel: 'Watch only',
  },
  approval: {
    label: 'Ask first',
    detail: 'Patrol investigates and prepares fixes, but every change waits for your approval.',
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
    detail: 'Patrol can act automatically within policy and still asks when approval is required.',
    compactLabel: 'Autopilot',
  },
};

export const PATROL_WORKSPACE_HISTORY_DESCRIPTION = 'Past checks and what Patrol recorded.';

export const PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION = 'What Patrol found during this run.';

export const PATROL_WORKSPACE_SETUP_TITLE = 'Check Patrol model';

export const PATROL_WORKSPACE_SETUP_DESCRIPTION =
  'Provider checks can pass while Patrol still needs a tool-call check. Open Provider & Models, then click Check Patrol model.';

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

export interface MonitorContextPatrolPostureSummary {
  detail: string;
  id: 'coverage' | 'open-work' | 'schedule';
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

interface MonitorContextPatrolPostureInput {
  findingCount?: number;
  latestRun?: Pick<
    PatrolRunRecord,
    | 'effective_scope_resource_ids'
    | 'error_count'
    | 'resources_checked'
    | 'scope_resource_ids'
    | 'status'
  > | null;
  nowMs?: number;
  patrolStatus?: Pick<
    PatrolStatus,
    | 'enabled'
    | 'error_count'
    | 'findings_count'
    | 'healthy'
    | 'next_patrol_at'
    | 'resources_checked'
    | 'running'
    | 'runtime_state'
  > | null;
  pendingApprovalCount?: number;
  workTypeComposition?: PatrolWorkTypeComposition;
  monitoredResourceCount?: number;
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

function isActivePatrolRuntime(
  status: Pick<PatrolStatus, 'runtime_state'> | null | undefined,
): boolean {
  return !status?.runtime_state || status.runtime_state === 'active';
}

function getPatrolCoverageLabel(
  latestRun:
    | Pick<
        PatrolRunRecord,
        'effective_scope_resource_ids' | 'resources_checked' | 'scope_resource_ids'
      >
    | null
    | undefined,
  status: Pick<PatrolStatus, 'resources_checked'> | null | undefined,
): string | undefined {
  const latestRunCoverage = latestRun ? getPatrolRunCoverageSummary(latestRun) : '';
  if (latestRunCoverage) return latestRunCoverage;

  const statusResourcesChecked = normalizeCount(status?.resources_checked);
  if (statusResourcesChecked > 0) {
    return `Checked ${formatCount(statusResourcesChecked, 'resource')}`;
  }

  return undefined;
}

function formatMonitorCoverageLabel(coverageLabel: string | undefined): string {
  if (!coverageLabel) return 'Patrol coverage needs refresh';
  return `Patrol ${coverageLabel.charAt(0).toLowerCase()}${coverageLabel.slice(1)}`;
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
    return 'Current Patrol issues appear here.';
  }

  switch (input.autonomyLevel) {
    case 'approval':
      return 'Current issues, investigations, and approvals appear here.';
    case 'assisted':
      return 'Current issues, fixes, and approvals appear here.';
    case 'full':
      return 'Current issues, automatic work, and approvals appear here.';
    case 'monitor':
    default:
      return 'Current Patrol issues appear here.';
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

function shouldSuppressMonitorContextPatrolPosture(input: MonitorContextPatrolPostureInput): boolean {
  const composition = input.workTypeComposition;
  const pendingApprovalCount = normalizeCount(input.pendingApprovalCount);
  const findingCount = normalizeCount(input.findingCount);
  const failedActionCount = normalizeCount(composition?.failed);
  const approvalWorkCount = normalizeCount(composition?.approval);
  const inProgressWorkCount = normalizeCount(composition?.inProgress);
  const recurringIssueCount = normalizeCount(composition?.recurring);
  const latestRun = input.latestRun ?? null;
  const patrolStatus = input.patrolStatus ?? null;
  const statusErrorCount = normalizeCount(patrolStatus?.error_count);
  const statusFindingCount = normalizeCount(patrolStatus?.findings_count);

  return (
    findingCount > 0 ||
    pendingApprovalCount > 0 ||
    failedActionCount > 0 ||
    approvalWorkCount > 0 ||
    inProgressWorkCount > 0 ||
    recurringIssueCount > 0 ||
    statusFindingCount > 0 ||
    patrolStatus?.running ||
    statusErrorCount > 0 ||
    hasFailedPatrolCheck(latestRun) ||
    !isActivePatrolRuntime(patrolStatus) ||
    isScheduledPatrolOverdue(patrolStatus, input.nowMs ?? Date.now())
  );
}

export function getMonitorContextPatrolProtectionPosture(
  input: MonitorContextPatrolPostureInput,
): MonitorContextPatrolPostureSummary[] {
  if (normalizeCount(input.monitoredResourceCount) <= 0) {
    return [];
  }

  const patrolStatus = input.patrolStatus ?? null;
  if ((!input.latestRun && !patrolStatus) || shouldSuppressMonitorContextPatrolPosture(input)) {
    return [];
  }

  const healthyTone: MetadataBadgeTone = patrolStatus?.healthy === false ? 'warning' : 'success';
  const coverageLabel = getPatrolCoverageLabel(input.latestRun ?? null, patrolStatus);
  const summaries: MonitorContextPatrolPostureSummary[] = [
    {
      id: 'coverage',
      label: formatMonitorCoverageLabel(coverageLabel),
      detail: coverageLabel
        ? 'Latest Patrol evidence is available while you review this monitor view.'
        : 'Run Patrol to refresh current coverage for monitored resources.',
      tone: coverageLabel ? healthyTone : 'warning',
    },
    {
      id: 'open-work',
      label: 'No Patrol work waiting',
      detail: 'Current Patrol findings and approvals stay in Patrol; none are waiting now.',
      tone: healthyTone,
    },
  ];

  if (patrolStatus?.enabled === false) {
    summaries.push({
      id: 'schedule',
      label: 'Scheduled checks paused',
      detail: 'Run Patrol manually or enable scheduled checks to keep coverage fresh.',
      tone: 'warning',
    });
  } else if (patrolStatus?.next_patrol_at) {
    summaries.push({
      id: 'schedule',
      label: 'Next check scheduled',
      detail: 'Patrol is scheduled to check monitored resources again.',
      tone: 'info',
    });
  } else {
    summaries.push({
      id: 'schedule',
      label: 'Ready to run Patrol',
      detail: 'Run Patrol from the Patrol page any time to refresh coverage.',
      tone: 'info',
    });
  }

  return summaries;
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
