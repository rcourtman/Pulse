import type { PatrolRuntimeState } from '@/api/patrol';
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
  eyebrow: string;
  compactLabel: string;
  tone: SemanticTone;
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
      eyebrow: runtime.title,
      compactLabel: runtime.label,
      tone: runtime.tone,
    };
  }

  if ((args.criticalFindings ?? 0) > 0) {
    return {
      title: 'Critical issues detected',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'error',
    };
  }

  if ((args.warningFindings ?? 0) > 0) {
    return {
      title: 'Issues detected',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'warning',
    };
  }

  if (args.overallHealth?.factors.some((factor) => factor.category === 'coverage')) {
    return {
      title: 'Coverage incomplete',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Coverage incomplete',
      tone: getHealthSummaryTone(args.overallHealth),
    };
  }

  if (args.overallHealth && args.overallHealth.grade !== 'A') {
    return {
      title: 'Health requires attention',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Health requires attention',
      tone: getHealthSummaryTone(args.overallHealth),
    };
  }

  return {
    title: 'No active issues detected',
    eyebrow: 'Patrol assessment',
    compactLabel: PATROL_NO_ISSUES_LABEL,
    tone: 'success',
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
