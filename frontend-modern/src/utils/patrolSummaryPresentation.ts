import type { PatrolRunRecord } from '@/api/patrol';

// What Patrol run history proves about coverage of the whole estate.
// `complete`: the latest completed full patrol ended without errors, checked
// at least one resource, finished within the last 24 hours, and no run after
// it failed. `incomplete`: it ended with errors, a run after it failed, or
// only targeted or follow-up runs have completed. `unproven`: anything else,
// including no completed run.
export type PatrolRunCoverage = 'complete' | 'incomplete' | 'unproven';

// The backend judges coverage over the same 24 hours
// (intelligencePatrolCoverageWindow in internal/ai/intelligence.go), so an
// older clean run cannot vouch for what its coverage factor reports.
const PATROL_RUN_COVERAGE_PROOF_WINDOW_MS = 24 * 60 * 60 * 1000;

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

export function getPatrolRunCoverage(
  runs: PatrolRunRecord[] | undefined,
  nowMs: number = Date.now(),
): PatrolRunCoverage {
  const completedRuns = (runs ?? []).filter((run) => isCompletedPatrolRun(run));
  const latestFullRunIndex = completedRuns.findIndex((run) => isFullPatrolRun(run));

  if (latestFullRunIndex < 0) {
    return completedRuns.length > 0 ? 'incomplete' : 'unproven';
  }
  const latestFullRun = completedRuns[latestFullRunIndex];
  // A clean full run supersedes only the failures before it, not one after.
  if (completedRuns.slice(0, latestFullRunIndex + 1).some((run) => hasRunErrors(run))) {
    return 'incomplete';
  }
  if ((latestFullRun.resources_checked || 0) <= 0) {
    return 'unproven';
  }
  const completedMs = Date.parse(latestFullRun.completed_at);
  if (!Number.isFinite(completedMs) || nowMs - completedMs > PATROL_RUN_COVERAGE_PROOF_WINDOW_MS) {
    return 'unproven';
  }
  return 'complete';
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
