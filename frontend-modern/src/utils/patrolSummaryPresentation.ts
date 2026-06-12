import type { PatrolRunRecord, PatrolRuntimeState } from '@/api/patrol';
import type { UnifiedFinding } from '@/stores/aiIntelligence';
import type { IntelligenceHealthScore } from '@/types/aiIntelligence';
import {
  isPatrolRuntimeFinding,
  normalizePatrolRuntimeFindingLabel,
} from '@/utils/aiFindingPresentation';
import {
  formatPatrolActivityBreakdown,
  getPatrolActivityBreakdown,
} from '@/utils/patrolRunPresentation';
import { getPatrolProviderSettingsAction } from '@/utils/patrolRuntimeActions';
import type { SemanticTone } from '@/utils/semanticTonePresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import type { StatusIndicatorVariant } from '@/utils/status';

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

export interface PatrolAssessmentShellPresentation {
  headerClass: string;
  badgeVariant: StatusIndicatorVariant;
  iconClass: string;
  iconContainerClass: string;
}

export interface PatrolAssessmentAction {
  label: string;
  href: string;
}

export interface PatrolSummaryMetricState {
  primaryLabel: string;
  primaryValue: number;
  primarySeverity: 'critical' | 'warning';
  secondaryLabel: string;
  secondaryValue: number;
  secondarySeverity: 'critical' | 'warning';
  criticalLabel: string;
  criticalValue: number;
  fixedLabel: string;
  fixedValue: number;
}

export interface PatrolVerificationPresentation {
  title: string;
  description: string;
  compactLabel: string;
  tone: SemanticTone;
  lastFullRunAt?: string;
  activityMixLabel?: string;
}

export interface PatrolRecencyPresentation {
  label: string;
  timestamp?: string;
  // resourcesChecked is the raw coverage signal for the most recent
  // completed run. resourcesCheckedLabel is the operator-facing phrase; it
  // says "verified" only for successful full patrols and uses "checked" for
  // limited or errored activity.
  resourcesChecked?: number;
  resourcesCheckedLabel?: string;
}

type PatrolAssessmentFinding = Pick<
  UnifiedFinding,
  'resourceId' | 'resourceName' | 'title' | 'severity' | 'status'
>;

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

const ASSESSMENT_SHELL_PRESENTATION: Record<SemanticTone, PatrolAssessmentShellPresentation> = {
  success: {
    headerClass: 'bg-emerald-50/60 dark:bg-emerald-950/30',
    badgeVariant: 'success',
    iconClass: 'text-emerald-600 dark:text-emerald-300',
    iconContainerClass:
      'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/40',
  },
  warning: {
    headerClass: 'bg-amber-50/70 dark:bg-amber-950/30',
    badgeVariant: 'warning',
    iconClass: 'text-amber-600 dark:text-amber-300',
    iconContainerClass: 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/40',
  },
  error: {
    headerClass: 'bg-red-50/70 dark:bg-red-950/30',
    badgeVariant: 'danger',
    iconClass: 'text-red-600 dark:text-red-300',
    iconContainerClass: 'border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/40',
  },
  info: {
    headerClass: 'bg-blue-50/70 dark:bg-blue-950/30',
    badgeVariant: 'info',
    iconClass: 'text-blue-600 dark:text-blue-300',
    iconContainerClass: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950/40',
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

export function getPatrolAssessmentShellPresentation(
  tone: SemanticTone = 'info',
): PatrolAssessmentShellPresentation {
  return ASSESSMENT_SHELL_PRESENTATION[tone] || ASSESSMENT_SHELL_PRESENTATION.info;
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

function formatRuntimeIssueCount(count: number, severity: 'critical' | 'warning'): string {
  const severityLabel = severity === 'critical' ? 'critical ' : '';
  return `${count} active ${severityLabel}Patrol runtime issue${count === 1 ? '' : 's'}`;
}

function getRuntimeFindingSummaryLabel(finding: PatrolAssessmentFinding | undefined): string {
  const title = String(finding?.title || '').trim();
  if (!title) {
    return 'a Patrol runtime issue';
  }

  return normalizePatrolRuntimeFindingLabel(title) || title;
}

function classifyActiveFindings(activeFindings: PatrolAssessmentFinding[] | undefined) {
  const runtimeCritical = (activeFindings ?? []).filter(
    (finding) =>
      finding.status === 'active' &&
      finding.severity === 'critical' &&
      isPatrolRuntimeFinding(finding),
  ).length;
  const runtimeWarning = (activeFindings ?? []).filter(
    (finding) =>
      finding.status === 'active' &&
      finding.severity === 'warning' &&
      isPatrolRuntimeFinding(finding),
  ).length;
  const infrastructureCritical = (activeFindings ?? []).filter(
    (finding) =>
      finding.status === 'active' &&
      finding.severity === 'critical' &&
      !isPatrolRuntimeFinding(finding),
  ).length;
  const infrastructureWarning = (activeFindings ?? []).filter(
    (finding) =>
      finding.status === 'active' &&
      finding.severity === 'warning' &&
      !isPatrolRuntimeFinding(finding),
  ).length;

  return {
    runtimeCritical,
    runtimeWarning,
    infrastructureCritical,
    infrastructureWarning,
    runtimeTotal: runtimeCritical + runtimeWarning,
    infrastructureTotal: infrastructureCritical + infrastructureWarning,
  };
}

export function getPatrolSummaryMetricState(args: {
  activeFindings?: PatrolAssessmentFinding[];
  fixedCount?: number;
}): PatrolSummaryMetricState {
  const classified = classifyActiveFindings(args.activeFindings);
  const hasRuntimeIssues = classified.runtimeTotal > 0;

  return {
    primaryLabel: hasRuntimeIssues ? 'Infrastructure findings' : 'Active findings',
    primaryValue: classified.infrastructureTotal,
    primarySeverity: classified.infrastructureCritical > 0 ? 'critical' : 'warning',
    secondaryLabel: hasRuntimeIssues ? 'Runtime issues' : 'Warnings',
    secondaryValue: hasRuntimeIssues ? classified.runtimeTotal : classified.infrastructureWarning,
    secondarySeverity: classified.runtimeCritical > 0 ? 'critical' : 'warning',
    criticalLabel: 'Critical',
    criticalValue: classified.infrastructureCritical + classified.runtimeCritical,
    fixedLabel: 'Fixed',
    fixedValue: args.fixedCount ?? 0,
  };
}

export function getPatrolScoreChipLabel(args: {
  overallHealth?: IntelligenceHealthScore;
  activeFindings?: PatrolAssessmentFinding[];
}): string {
  const classified = classifyActiveFindings(args.activeFindings);
  const hasCoverageGap = Boolean(
    args.overallHealth?.factors.some((factor) => factor.category === 'coverage'),
  );

  if (hasCoverageGap || (classified.infrastructureTotal === 0 && classified.runtimeTotal > 0)) {
    return 'Assessment';
  }

  return 'Health';
}

export function getPatrolAssessmentAction(args: {
  activeFindings?: PatrolAssessmentFinding[];
}): PatrolAssessmentAction | undefined {
  const classified = classifyActiveFindings(args.activeFindings);
  if (classified.infrastructureTotal === 0 && classified.runtimeTotal > 0) {
    return getPatrolProviderSettingsAction();
  }

  return undefined;
}

function joinAssessmentParts(parts: string[]): string {
  if (parts.length <= 1) return parts[0] ?? '';
  if (parts.length === 2) return `${parts[0]} and ${parts[1]}`;
  return `${parts.slice(0, -1).join(', ')}, and ${parts.at(-1)}`;
}

function predictionReadsAsAllClear(prediction: string | undefined): boolean {
  const normalized = String(prediction || '')
    .trim()
    .toLowerCase();
  if (!normalized) return false;

  return (
    normalized.includes('healthy with no significant issue') ||
    normalized.includes('no significant issues detected') ||
    normalized.includes('no active issues') ||
    normalized.includes('no issues detected') ||
    normalized.includes('all clear')
  );
}

function predictionReadsAsCoverageGap(prediction: string | undefined): boolean {
  const normalized = String(prediction || '')
    .trim()
    .toLowerCase();
  if (!normalized) return false;

  return (
    normalized.includes('coverage is incomplete') ||
    normalized.includes('coverage incomplete') ||
    normalized.includes('not fully verified') ||
    normalized.includes('limited to scoped runs') ||
    normalized.includes('limited to targeted') ||
    normalized.includes('runs encountered errors') ||
    normalized.includes('ended with errors') ||
    normalized.includes('summary may be incomplete')
  );
}

function hasSuccessfulFullCoverageRun(runs: PatrolRunRecord[] | undefined): boolean {
  const recentFullRun = (runs ?? [])
    .filter((run) => isCompletedPatrolRun(run))
    .find((run) => isFullPatrolRun(run));
  return Boolean(
    recentFullRun && !hasRunErrors(recentFullRun) && (recentFullRun.resources_checked || 0) > 0,
  );
}

function getFindingAssessmentDescription(args: {
  criticalFindings?: number;
  warningFindings?: number;
  overallHealth?: IntelligenceHealthScore;
  activeFindings?: PatrolAssessmentFinding[];
  runs?: PatrolRunRecord[];
}): string {
  const criticalFindings = args.criticalFindings ?? 0;
  const warningFindings = args.warningFindings ?? 0;
  const hasVerifiedFullCoverage = hasSuccessfulFullCoverageRun(args.runs);
  const hasCoverageGapFactor = Boolean(
    args.overallHealth?.factors.some((factor) => factor.category === 'coverage'),
  );
  const hasCoverageGap = hasCoverageGapFactor && !hasVerifiedFullCoverage;
  const classified = classifyActiveFindings(args.activeFindings);
  const activeRuntimeFindings = (args.activeFindings ?? []).filter(
    (finding) => finding.status === 'active' && isPatrolRuntimeFinding(finding),
  );
  const shouldUsePrediction = (prediction: string | undefined): prediction is string => {
    if (!prediction) return false;
    if (predictionReadsAsAllClear(prediction)) return false;
    return !(hasVerifiedFullCoverage && predictionReadsAsCoverageGap(prediction));
  };

  if (classified.infrastructureTotal === 0 && classified.runtimeTotal === 1) {
    const runtimeFinding = activeRuntimeFindings[0];
    const runtimeLabel = getRuntimeFindingSummaryLabel(runtimeFinding);
    const runtimeSummary =
      runtimeFinding?.severity === 'critical'
        ? `Patrol is currently blocked by a critical runtime issue: ${runtimeLabel}.`
        : `Patrol has an active runtime issue: ${runtimeLabel}.`;

    if (hasCoverageGap) {
      return `${runtimeSummary} Recent coverage is also incomplete, so the rest of your infrastructure is not fully verified.`;
    }

    const prediction = args.overallHealth?.prediction?.trim();
    if (shouldUsePrediction(prediction)) {
      return prediction;
    }

    return `${runtimeSummary} Review the Patrol runtime issue for more detail.`;
  }

  const findingSummaryParts: string[] = [];

  if (classified.infrastructureCritical > 0) {
    findingSummaryParts.push(
      `${formatFindingCount(classified.infrastructureCritical, 'critical')} in your infrastructure`,
    );
  }
  if (classified.infrastructureWarning > 0) {
    findingSummaryParts.push(
      `${formatFindingCount(classified.infrastructureWarning, 'warning')} in your infrastructure`,
    );
  }
  if (classified.runtimeCritical > 0) {
    findingSummaryParts.push(formatRuntimeIssueCount(classified.runtimeCritical, 'critical'));
  }
  if (classified.runtimeWarning > 0) {
    findingSummaryParts.push(formatRuntimeIssueCount(classified.runtimeWarning, 'warning'));
  }

  const findingSummary =
    findingSummaryParts.length > 0
      ? `Patrol surfaced ${joinAssessmentParts(findingSummaryParts)}.`
      : criticalFindings > 0
        ? `Patrol surfaced ${formatFindingCount(criticalFindings, 'critical')}.`
        : `Patrol surfaced ${formatFindingCount(warningFindings, 'warning')}.`;

  if (hasCoverageGap) {
    return `${findingSummary} Recent coverage is also incomplete, so the rest of your infrastructure is not fully verified.`;
  }

  const prediction = args.overallHealth?.prediction?.trim();
  if (shouldUsePrediction(prediction)) {
    return prediction;
  }

  return `${findingSummary} Review the active findings for more detail.`;
}

function normalizeRunType(type: string | undefined): string {
  return String(type || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');
}

function isFullPatrolRun(run: PatrolRunRecord): boolean {
  const normalized = normalizeRunType(run.type);
  return normalized === '' || normalized === 'full' || normalized === 'patrol';
}

function isScopedPatrolRun(run: PatrolRunRecord): boolean {
  return normalizeRunType(run.type) === 'scoped';
}

function isVerificationPatrolRun(run: PatrolRunRecord): boolean {
  return normalizeRunType(run.type) === 'verification';
}

function getVerificationActivityMixLabel(runs: PatrolRunRecord[]): string | undefined {
  const latestCompletedRun = runs.find((run) => isCompletedPatrolRun(run));
  const referenceTimestamp = latestCompletedRun?.completed_at || latestCompletedRun?.started_at;
  if (!referenceTimestamp) {
    return undefined;
  }

  const breakdown = getPatrolActivityBreakdown(runs, new Date(referenceTimestamp));
  const scopedRuns =
    breakdown.alertTriggeredRuns +
    breakdown.anomalyTriggeredRuns +
    breakdown.alertClearedRuns +
    breakdown.verificationChecks +
    breakdown.otherScopedRuns;
  if (breakdown.totalRuns <= 1 || scopedRuns <= 0) {
    return undefined;
  }

  const label = formatPatrolActivityBreakdown(breakdown);
  return label || undefined;
}

function isCompletedPatrolRun(run: PatrolRunRecord): boolean {
  return Boolean(run.completed_at?.trim());
}

function hasRunErrors(run: PatrolRunRecord): boolean {
  return (
    run.error_count > 0 ||
    String(run.status || '')
      .trim()
      .toLowerCase() === 'error'
  );
}

function formatRecencyResourcesCheckedLabel(run: PatrolRunRecord): string | undefined {
  const resourcesChecked = run.resources_checked || 0;
  if (resourcesChecked <= 0) {
    return undefined;
  }

  const verb = isFullPatrolRun(run) && !hasRunErrors(run) ? 'verified' : 'checked';
  return `${verb} ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}`;
}

export function getPatrolAssessmentPresentation(args: {
  overallHealth?: IntelligenceHealthScore;
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
  criticalFindings?: number;
  warningFindings?: number;
  activeFindings?: PatrolAssessmentFinding[];
  runs?: PatrolRunRecord[];
}): PatrolAssessmentPresentation {
  const classified = classifyActiveFindings(args.activeFindings);
  const hasVerifiedFullCoverage = hasSuccessfulFullCoverageRun(args.runs);

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
    if (classified.infrastructureTotal === 0 && classified.runtimeTotal > 0) {
      return {
        title: 'Critical Patrol runtime issue',
        description: getFindingAssessmentDescription(args),
        eyebrow: 'Patrol assessment',
        compactLabel: 'Patrol runtime issue',
        tone: 'error',
      };
    }

    return {
      title: 'Critical issues detected',
      description: getFindingAssessmentDescription(args),
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'error',
    };
  }

  if ((args.warningFindings ?? 0) > 0) {
    if (classified.infrastructureTotal === 0 && classified.runtimeTotal > 0) {
      return {
        title: 'Patrol runtime issue',
        description: getFindingAssessmentDescription(args),
        eyebrow: 'Patrol assessment',
        compactLabel: 'Patrol runtime issue',
        tone: 'warning',
      };
    }

    return {
      title: 'Issues detected',
      description: getFindingAssessmentDescription(args),
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'warning',
    };
  }

  if (
    args.overallHealth?.factors.some((factor) => factor.category === 'coverage') &&
    !hasVerifiedFullCoverage
  ) {
    return {
      title: 'Coverage incomplete',
      description: getCoverageDescription(args.overallHealth),
      eyebrow: 'Patrol assessment',
      compactLabel: 'Coverage incomplete',
      tone: getHealthSummaryTone(args.overallHealth),
    };
  }

  if (args.overallHealth && args.overallHealth.grade !== 'A') {
    const prediction = args.overallHealth.prediction?.trim();
    return {
      title: 'Health requires attention',
      description:
        prediction && !(hasVerifiedFullCoverage && predictionReadsAsCoverageGap(prediction))
          ? prediction
          : 'Patrol assessment still needs attention.',
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
  const activityMixLabel = getVerificationActivityMixLabel(completedRuns);
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
        activityMixLabel,
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
      activityMixLabel,
    };
  }

  const recentLimitedRun = completedRuns.find((run) => !isFullPatrolRun(run));
  if (recentLimitedRun) {
    const resourcesChecked = recentLimitedRun.resources_checked || 0;
    let description =
      'Recent activity was limited to targeted Patrol checks, so Patrol has not recently re-verified your full infrastructure.';

    if (isVerificationPatrolRun(recentLimitedRun)) {
      description =
        resourcesChecked > 0
          ? `Recent activity was limited to verification checks over ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}, so Patrol has not recently re-verified your full infrastructure.`
          : 'Recent activity was limited to verification checks, so Patrol has not recently re-verified your full infrastructure.';
    } else if (isScopedPatrolRun(recentLimitedRun)) {
      description =
        resourcesChecked > 0
          ? `Recent activity was limited to scoped ${recentLimitedRun.trigger_reason ? String(recentLimitedRun.trigger_reason).replace(/_/g, ' ') : 'patrol'} runs over ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}, so Patrol has not recently re-verified your full infrastructure.`
          : 'Recent activity was limited to scoped patrol runs, so Patrol has not recently re-verified your full infrastructure.';
    } else if (resourcesChecked > 0) {
      description = `Recent activity was limited to targeted Patrol checks over ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}, so Patrol has not recently re-verified your full infrastructure.`;
    }

    return {
      title: 'No recent full patrol',
      description,
      compactLabel: 'Partial verification',
      tone: 'warning',
      activityMixLabel,
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
  lastActivityAt?: string;
}): PatrolRecencyPresentation {
  const latestCompletedRun = (args.runs ?? []).find((run) => isCompletedPatrolRun(run));
  if (latestCompletedRun?.completed_at) {
    const resourcesChecked = latestCompletedRun.resources_checked || 0;
    const resourcesCheckedLabel = formatRecencyResourcesCheckedLabel(latestCompletedRun);
    return {
      label: isFullPatrolRun(latestCompletedRun) ? 'Last full patrol' : 'Last activity',
      timestamp: latestCompletedRun.completed_at,
      resourcesChecked: resourcesChecked > 0 ? resourcesChecked : undefined,
      resourcesCheckedLabel,
    };
  }

  const lastPatrolAt = args.lastPatrolAt?.trim();
  const lastActivityAt = args.lastActivityAt?.trim();

  if (lastActivityAt && lastPatrolAt) {
    const activityMs = Date.parse(lastActivityAt);
    const patrolMs = Date.parse(lastPatrolAt);
    if (Number.isNaN(activityMs) && !Number.isNaN(patrolMs)) {
      return {
        label: 'Last full patrol',
        timestamp: lastPatrolAt,
      };
    }
    if (!Number.isNaN(activityMs) && !Number.isNaN(patrolMs) && patrolMs >= activityMs) {
      return {
        label: 'Last full patrol',
        timestamp: lastPatrolAt,
      };
    }
    return {
      label: 'Last activity',
      timestamp: lastActivityAt,
    };
  }

  if (lastActivityAt) {
    return {
      label: 'Last activity',
      timestamp: lastActivityAt,
    };
  }

  if (lastPatrolAt) {
    return {
      label: 'Last full patrol',
      timestamp: lastPatrolAt,
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
