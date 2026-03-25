import type { PatrolRunRecord, PatrolRuntimeState } from '@/api/patrol';
import type { IntelligenceHealthScore } from '@/types/aiIntelligence';
import type { SemanticTone } from '@/utils/semanticTonePresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';

export type PatrolSummaryTone = 'critical' | 'warning' | 'success';

export interface PatrolSummaryPresentation {
  iconClass: string;
  iconContainerClass: string;
  valueClass: string;
}

export interface PatrolNoIssuesPresentation {
  label: string;
  tone: SemanticTone;
}

export interface PatrolAssessmentPresentation {
  title: string;
  description: string;
  eyebrow: string;
  compactLabel: string;
  tone: SemanticTone;
}

export interface PatrolVerificationPresentation {
  title: string;
  description: string;
  compactLabel: string;
  tone: SemanticTone;
  lastFullRunAt?: string;
}

export interface PatrolRecencyPresentation {
  label: string;
  timestamp?: string;
}

export const PATROL_NO_ISSUES_LABEL = 'No issues found';

const QUIET_ICON_CONTAINER = 'bg-surface border-border';
const QUIET_ICON = 'text-muted';
const QUIET_VALUE = 'text-muted';

const ACTIVE_PRESENTATION: Record<PatrolSummaryTone, PatrolSummaryPresentation> = {
  critical: {
    iconClass: 'text-red-500 dark:text-red-400',
    iconContainerClass: 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800',
    valueClass: 'text-red-600 dark:text-red-400',
  },
  success: {
    iconClass: 'text-green-500 dark:text-green-400',
    iconContainerClass: 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800',
    valueClass: 'text-green-600 dark:text-green-400',
  },
  warning: {
    iconClass: 'text-amber-500 dark:text-amber-400',
    iconContainerClass: 'bg-amber-50 dark:bg-amber-900 border-amber-200 dark:border-amber-800',
    valueClass: 'text-amber-600 dark:text-amber-400',
  },
};

export function getPatrolSummaryPresentation(
  tone: PatrolSummaryTone,
  active: boolean,
): PatrolSummaryPresentation {
  if (!active) {
    return {
      iconClass: QUIET_ICON,
      iconContainerClass: QUIET_ICON_CONTAINER,
      valueClass: QUIET_VALUE,
    };
  }
  return ACTIVE_PRESENTATION[tone];
}

function getHealthSummaryTone(overallHealth: IntelligenceHealthScore): SemanticTone {
  return overallHealth.grade === 'D' || overallHealth.grade === 'F' ? 'error' : 'warning';
}

function getCoverageDescription(overallHealth: IntelligenceHealthScore | undefined): string {
  return (
    overallHealth?.prediction?.trim() ||
    'Patrol coverage is incomplete, so overall infrastructure health is not fully verified.'
  );
}

function formatFindingCount(count: number, severity: 'critical' | 'warning'): string {
  return `${count} active ${severity} finding${count === 1 ? '' : 's'}`;
}

function getFindingAssessmentDescription(args: {
  criticalFindings?: number;
  warningFindings?: number;
  overallHealth?: IntelligenceHealthScore;
}): string {
  const criticalFindings = args.criticalFindings ?? 0;
  const warningFindings = args.warningFindings ?? 0;
  const hasCoverageGap = Boolean(
    args.overallHealth?.factors.some((factor) => factor.category === 'coverage'),
  );

  const findingSummary =
    criticalFindings > 0
      ? `Patrol surfaced ${formatFindingCount(criticalFindings, 'critical')}.`
      : `Patrol surfaced ${formatFindingCount(warningFindings, 'warning')}.`;

  if (hasCoverageGap) {
    return `${findingSummary} Recent coverage is also incomplete, so the rest of your infrastructure is not fully verified.`;
  }

  return (
    args.overallHealth?.prediction?.trim() ||
    `${findingSummary} Review the active findings for more detail.`
  );
}

function normalizeRunType(type: string | undefined): string {
  return String(type || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');
}

function isFullPatrolRun(run: PatrolRunRecord): boolean {
  return normalizeRunType(run.type) !== 'scoped';
}

function isCompletedPatrolRun(run: PatrolRunRecord): boolean {
  return Boolean(run.completed_at?.trim());
}

function hasRunErrors(run: PatrolRunRecord): boolean {
  return run.error_count > 0 || String(run.status || '').trim().toLowerCase() === 'error';
}

export function getPatrolAssessmentPresentation(args: {
  overallHealth?: IntelligenceHealthScore;
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
  criticalFindings?: number;
  warningFindings?: number;
}): PatrolAssessmentPresentation {
  if (
    args.runtimeState === 'blocked' ||
    args.runtimeState === 'disabled' ||
    args.runtimeState === 'unavailable'
  ) {
    const runtime = getPatrolRuntimePresentation(args.runtimeState, args.blockedReason);
    return {
      title: runtime.label,
      description: runtime.description,
      eyebrow: runtime.title,
      compactLabel: runtime.label,
      tone: runtime.tone,
    };
  }

  if ((args.criticalFindings ?? 0) > 0) {
    return {
      title: 'Critical issues detected',
      description: getFindingAssessmentDescription(args),
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'error',
    };
  }

  if ((args.warningFindings ?? 0) > 0) {
    return {
      title: 'Issues detected',
      description: getFindingAssessmentDescription(args),
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'warning',
    };
  }

  if (args.overallHealth?.factors.some((factor) => factor.category === 'coverage')) {
    return {
      title: 'Coverage incomplete',
      description: getCoverageDescription(args.overallHealth),
      eyebrow: 'Patrol assessment',
      compactLabel: 'Coverage incomplete',
      tone: getHealthSummaryTone(args.overallHealth),
    };
  }

  if (args.overallHealth && args.overallHealth.grade !== 'A') {
    return {
      title: 'Health requires attention',
      description:
        args.overallHealth.prediction?.trim() ||
        'Patrol assessment still needs attention.',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Health requires attention',
      tone: getHealthSummaryTone(args.overallHealth),
    };
  }

  return {
    title: 'No active issues detected',
    description:
      args.overallHealth?.prediction?.trim() ||
      'Infrastructure is healthy with no significant issues detected.',
    eyebrow: 'Patrol assessment',
    compactLabel: PATROL_NO_ISSUES_LABEL,
    tone: 'success',
  };
}

export function getPatrolVerificationPresentation(args: {
  runs?: PatrolRunRecord[];
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
}): PatrolVerificationPresentation {
  if (
    args.runtimeState === 'blocked' ||
    args.runtimeState === 'disabled' ||
    args.runtimeState === 'unavailable'
  ) {
    const runtime = getPatrolRuntimePresentation(args.runtimeState, args.blockedReason);
    return {
      title: runtime.label,
      description: runtime.description,
      compactLabel: runtime.label,
      tone: runtime.tone,
    };
  }

  const completedRuns = (args.runs ?? []).filter((run) => isCompletedPatrolRun(run));
  const recentFullRun = completedRuns.find((run) => isFullPatrolRun(run));

  if (recentFullRun) {
    const resourcesChecked = recentFullRun.resources_checked || 0;
    if (hasRunErrors(recentFullRun)) {
      return {
        title: 'Full patrol needs review',
        description:
          resourcesChecked > 0
            ? `The most recent full patrol checked ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'} but ended with ${recentFullRun.error_count} error${recentFullRun.error_count === 1 ? '' : 's'}.`
            : 'The most recent full patrol ended with errors, so Patrol has not fully re-verified your infrastructure.',
        compactLabel: 'Verification limited',
        tone: 'warning',
        lastFullRunAt: recentFullRun.completed_at,
      };
    }

    return {
      title: 'Recently verified',
      description:
        resourcesChecked > 0
          ? `The most recent full patrol completed successfully and checked ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}.`
          : 'The most recent full patrol completed successfully.',
      compactLabel: 'Recently verified',
      tone: 'success',
      lastFullRunAt: recentFullRun.completed_at,
    };
  }

  const recentScopedRun = completedRuns.find((run) => !isFullPatrolRun(run));
  if (recentScopedRun) {
    const resourcesChecked = recentScopedRun.resources_checked || 0;
    return {
      title: 'No recent full patrol',
      description:
        resourcesChecked > 0
          ? `Recent activity was limited to scoped ${recentScopedRun.trigger_reason ? String(recentScopedRun.trigger_reason).replace(/_/g, ' ') : 'patrol'} runs over ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}, so Patrol has not recently re-verified your full infrastructure.`
          : 'Recent activity was limited to scoped patrol runs, so Patrol has not recently re-verified your full infrastructure.',
      compactLabel: 'Partial verification',
      tone: 'warning',
    };
  }

  return {
    title: 'Verification pending',
    description: 'Patrol has not completed a recent full verification run yet.',
    compactLabel: 'Verification pending',
    tone: 'info',
  };
}

export function getPatrolRecencyPresentation(args: {
  runs?: PatrolRunRecord[];
  lastPatrolAt?: string;
}): PatrolRecencyPresentation {
  const latestCompletedRun = (args.runs ?? []).find((run) => isCompletedPatrolRun(run));
  if (latestCompletedRun?.completed_at) {
    return {
      label: isFullPatrolRun(latestCompletedRun) ? 'Last full patrol' : 'Last activity',
      timestamp: latestCompletedRun.completed_at,
    };
  }

  if (args.lastPatrolAt?.trim()) {
    return {
      label: 'Last activity',
      timestamp: args.lastPatrolAt,
    };
  }

  return {
    label: 'Last activity',
  };
}

export function getPatrolNoIssuesPresentation(args: {
  overallHealth?: IntelligenceHealthScore;
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
}): PatrolNoIssuesPresentation {
  const assessment = getPatrolAssessmentPresentation(args);
  return {
    label: assessment.compactLabel,
    tone: assessment.tone,
  };
}
