import type { PatrolRunRecord, PatrolRuntimeState } from '@/api/patrol';
import type { FindingsFilter } from '@/utils/aiFindingPresentation';
import { getFindingEmptyStateCopy } from '@/utils/aiFindingPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import type { SemanticTone } from '@/utils/semanticTonePresentation';
import type { IntelligenceHealthScore } from '@/types/aiIntelligence';
import { getPatrolRunCoverageSummary, isPatrolRunHealthy } from '@/utils/patrolRunPresentation';
import type { PatrolRunCoverage } from '@/utils/patrolSummaryPresentation';
import { getPatrolRunCoverage } from '@/utils/patrolSummaryPresentation';

export function getInvestigationMessagesState(loading: boolean, hasMessages: boolean) {
  if (loading) {
    return {
      text: 'Loading messages...',
      empty: false,
    } as const;
  }

  if (!hasMessages) {
    return {
      text: 'No investigation messages available.',
      empty: true,
    } as const;
  }

  return {
    text: '',
    empty: false,
  } as const;
}

export function getRunHistoryEmptyState() {
  return {
    text: 'No Patrol checks yet. Run Patrol to start history.',
  } as const;
}

export function getInvestigationSectionState(loading: boolean, hasInvestigation: boolean) {
  if (loading) {
    return {
      text: 'Loading investigation...',
      empty: false,
    } as const;
  }

  if (!hasInvestigation) {
    return {
      text: 'No investigation yet. Patrol adds notes after it runs in a mode that investigates.',
      empty: true,
    } as const;
  }

  return {
    text: '',
    empty: false,
  } as const;
}

export interface PatrolFindingsEmptyStateCopy {
  title: string;
  body?: string;
  tone: SemanticTone;
}

type PatrolRunSnapshotEmptyStateArgs = Pick<
  PatrolRunRecord,
  | 'resources_checked'
  | 'scope_resource_ids'
  | 'effective_scope_resource_ids'
  | 'finding_ids'
  | 'status'
  | 'error_count'
>;

const DEGRADED_COVERAGE_EMPTY_STATE_BODY = 'Run Patrol to check everything and refresh open work.';
const DEGRADED_HEALTH_EMPTY_STATE_BODY =
  'No current issues are listed, but Patrol health needs review.';
const HISTORICAL_REGRESSION_EMPTY_STATE_BODY = 'Past issues are in History if you need the record.';
const PATROL_QUEUE_CLEAR_TITLE = 'No current issues';
const PATROL_QUEUE_RECHECK_TITLE = 'Check needed';
const PATROL_QUEUE_REVIEW_TITLE = 'Patrol needs review';
const PATROL_QUEUE_RUNNING_TITLE = 'Patrol is checking now';
const PATROL_QUEUE_RUNNING_BODY =
  'If Patrol finds an issue or needs approval, it will add it here.';

function getHealthDegradedTone(overallHealth: IntelligenceHealthScore): SemanticTone {
  return overallHealth.grade === 'D' || overallHealth.grade === 'F' ? 'error' : 'warning';
}

function hasCoverageFactor(overallHealth: IntelligenceHealthScore | undefined): boolean {
  return Boolean(overallHealth?.factors.some((factor) => factor.category === 'coverage'));
}

// Run history decides coverage before the health summary does. The summary
// keeps its `Recent Patrol errors` coverage factor until three clean full runs
// lead its window or the errored runs leave it (summarizeRecentPatrolCoverage
// in internal/ai/intelligence.go), so a clean full run that supersedes those
// errors must outrank that factor, or the queue keeps asking for a check after
// Patrol has already checked everything.
function hasIncompleteCoverageEvidence(
  overallHealth: IntelligenceHealthScore | undefined,
  runCoverage: PatrolRunCoverage,
): boolean {
  if (runCoverage !== 'unproven') {
    return runCoverage === 'incomplete';
  }
  return hasCoverageFactor(overallHealth);
}

// Once run history proves coverage, a grade that only the coverage factor
// lowered is not degraded health.
function hasDegradedHealth(
  overallHealth: IntelligenceHealthScore,
  runCoverage: PatrolRunCoverage,
): boolean {
  if (overallHealth.grade === 'A') {
    return false;
  }
  if (runCoverage !== 'complete') {
    return true;
  }
  return overallHealth.factors.some(
    (factor) => factor.category !== 'coverage' && factor.impact < 0,
  );
}

function hasHistoricalRegressions(count: number | undefined): boolean {
  return typeof count === 'number' && Number.isFinite(count) && count > 0;
}

function getLatestRunCoverageContext(runs: PatrolRunRecord[] | undefined): string | undefined {
  if (!runs || runs.length === 0) {
    return undefined;
  }
  const latestRun = runs[0];
  const coverageSummary = getPatrolRunCoverageSummary(latestRun);
  if (!coverageSummary) {
    return undefined;
  }
  return `${coverageSummary}.`;
}

function getPatrolRunSnapshotEmptyState(
  run: PatrolRunSnapshotEmptyStateArgs,
): PatrolFindingsEmptyStateCopy {
  const coverageSummary = getPatrolRunCoverageSummary(run);
  const coveragePrefix = coverageSummary ? `${coverageSummary}. ` : '';

  if (!isPatrolRunHealthy(run.status, run.error_count)) {
    return {
      title: 'No findings recorded for this run',
      body: `${coveragePrefix}This run recorded no Patrol findings, but it ended with issues requiring review.`,
      tone: 'warning',
    };
  }

  return {
    title: 'No findings recorded for this run',
    body: `${coveragePrefix}This run recorded no Patrol findings.`,
    tone: 'info',
  };
}

function getPatrolRunSnapshotUnavailableEmptyState(): PatrolFindingsEmptyStateCopy {
  return {
    title: 'Finding record unavailable for this run',
    body: 'This older Patrol run has no finding record, so Patrol cannot show its issue list.',
    tone: 'warning',
  };
}

export function getPatrolFindingsEmptyState(args: {
  filter: FindingsFilter;
  overallHealth?: IntelligenceHealthScore;
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
  historicalRegressionCount?: number;
  coverageStale?: boolean;
  runs?: PatrolRunRecord[];
  runSnapshot?: PatrolRunSnapshotEmptyStateArgs;
  nowMs?: number;
}): PatrolFindingsEmptyStateCopy {
  if (
    args.filter === 'all' &&
    args.runSnapshot &&
    Array.isArray(args.runSnapshot.finding_ids) &&
    args.runSnapshot.finding_ids.length === 0
  ) {
    return getPatrolRunSnapshotEmptyState(args.runSnapshot);
  }

  if (args.filter === 'all' && args.runSnapshot && args.runSnapshot.finding_ids === undefined) {
    return getPatrolRunSnapshotUnavailableEmptyState();
  }

  if (args.filter !== 'active') {
    return {
      ...getFindingEmptyStateCopy(args.filter),
      tone: 'info',
    };
  }

  if (
    args.runtimeState === 'blocked' ||
    args.runtimeState === 'disabled' ||
    args.runtimeState === 'unavailable'
  ) {
    const runtime = getPatrolRuntimePresentation(args.runtimeState, args.blockedReason);
    return {
      title: runtime.title,
      body: runtime.description,
      tone: runtime.tone,
    };
  }

  if (args.runtimeState === 'running') {
    return {
      title: PATROL_QUEUE_RUNNING_TITLE,
      body: PATROL_QUEUE_RUNNING_BODY,
      tone: 'info',
    };
  }

  if (args.coverageStale) {
    return {
      title: PATROL_QUEUE_RECHECK_TITLE,
      body: 'Patrol coverage is stale. Run Patrol to check current infrastructure state.',
      tone: 'warning',
    };
  }

  const runCoverage = getPatrolRunCoverage(args.runs, args.nowMs);
  if (hasIncompleteCoverageEvidence(args.overallHealth, runCoverage)) {
    return {
      title: PATROL_QUEUE_RECHECK_TITLE,
      body: DEGRADED_COVERAGE_EMPTY_STATE_BODY,
      tone: args.overallHealth ? getHealthDegradedTone(args.overallHealth) : 'warning',
    };
  }

  if (args.overallHealth && hasDegradedHealth(args.overallHealth, runCoverage)) {
    return {
      title: PATROL_QUEUE_REVIEW_TITLE,
      body: DEGRADED_HEALTH_EMPTY_STATE_BODY,
      tone: getHealthDegradedTone(args.overallHealth),
    };
  }

  if (hasHistoricalRegressions(args.historicalRegressionCount)) {
    return {
      title: PATROL_QUEUE_CLEAR_TITLE,
      body: HISTORICAL_REGRESSION_EMPTY_STATE_BODY,
      tone: 'info',
    };
  }

  const coverageContext = getLatestRunCoverageContext(args.runs);
  return {
    title: PATROL_QUEUE_CLEAR_TITLE,
    body: coverageContext,
    tone: 'success',
  };
}
