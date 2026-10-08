import type { PatrolRunRecord, PatrolRuntimeState } from '@/api/patrol';
import {
  formatPatrolActivityBreakdown,
  getPatrolActivityBreakdown,
} from '@/utils/patrolRunPresentation';
import type { SemanticTone } from '@/utils/semanticTonePresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';

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
  // completed run. resourcesCheckedLabel is the operator-facing phrase:
  // "checked N resources" for any completed run with a positive count,
  // whatever its type or error state. It never says "verified".
  resourcesChecked?: number;
  resourcesCheckedLabel?: string;
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

  return `checked ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}`;
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
        title: 'Patrol check needs review',
        description:
          resourcesChecked > 0
            ? `The most recent Patrol check covered ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'} but ended with ${recentFullRun.error_count} error${recentFullRun.error_count === 1 ? '' : 's'}.`
            : 'The most recent Patrol check ended with errors.',
        compactLabel: 'Check needs review',
        tone: 'warning',
        lastFullRunAt: recentFullRun.completed_at,
        activityMixLabel,
      };
    }

    return {
      title: 'Recently checked',
      description:
        resourcesChecked > 0
          ? `The most recent Patrol check completed successfully and covered ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}.`
          : 'The most recent Patrol check completed successfully.',
      compactLabel: 'Recently checked',
      tone: 'success',
      lastFullRunAt: recentFullRun.completed_at,
      activityMixLabel,
    };
  }

  const recentLimitedRun = completedRuns.find((run) => !isFullPatrolRun(run));
  if (recentLimitedRun) {
    const resourcesChecked = recentLimitedRun.resources_checked || 0;
    let description =
      'Recent activity only checked part of your infrastructure. Run Patrol to check everything.';

    if (isVerificationPatrolRun(recentLimitedRun)) {
      description =
        resourcesChecked > 0
          ? `Recent follow-up checks covered ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}. Run Patrol to check everything.`
          : 'Recent follow-up checks did not cover your full infrastructure. Run Patrol to check everything.';
    } else if (isScopedPatrolRun(recentLimitedRun)) {
      description =
        resourcesChecked > 0
          ? `Recent targeted checks covered ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}. Run Patrol to check everything.`
          : 'Recent targeted checks did not cover your full infrastructure. Run Patrol to check everything.';
    } else if (resourcesChecked > 0) {
      description = `Recent targeted checks covered ${resourcesChecked} resource${resourcesChecked === 1 ? '' : 's'}. Run Patrol to check everything.`;
    }

    return {
      title: 'Needs full check',
      description,
      compactLabel: 'Partial check',
      tone: 'warning',
      activityMixLabel,
    };
  }

  return {
    title: 'Run Patrol to check',
    description: 'Patrol has not completed a check yet.',
    compactLabel: 'Check pending',
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
      label: 'Last check',
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
        label: 'Last check',
        timestamp: lastPatrolAt,
      };
    }
    if (!Number.isNaN(activityMs) && !Number.isNaN(patrolMs) && patrolMs >= activityMs) {
      return {
        label: 'Last check',
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
      label: 'Last check',
      timestamp: lastPatrolAt,
    };
  }

  return {
    label: 'Last activity',
  };
}
