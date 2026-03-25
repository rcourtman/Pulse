import type { PatrolRuntimeState } from '@/api/patrol';
import type { FindingsFilter } from '@/utils/aiFindingPresentation';
import { getFindingEmptyStateCopy } from '@/utils/aiFindingPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import type { SemanticTone } from '@/utils/semanticTonePresentation';
import type { IntelligenceHealthScore } from '@/types/aiIntelligence';

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
    text: 'No patrol runs yet. Trigger a run to populate history.',
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
      text: 'No investigation data available. Enable patrol autonomy to investigate findings.',
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

const HEALTHY_PATROL_EMPTY_STATE_BODY = 'Your infrastructure looks healthy!';

function getHealthDegradedTone(overallHealth: IntelligenceHealthScore): SemanticTone {
  return overallHealth.grade === 'D' || overallHealth.grade === 'F' ? 'error' : 'warning';
}

function shouldSuppressHealthyEmptyState(overallHealth: IntelligenceHealthScore | undefined): boolean {
  if (!overallHealth) {
    return false;
  }

  if (overallHealth.factors.some((factor) => factor.category === 'coverage')) {
    return true;
  }

  return overallHealth.grade !== 'A';
}

export function getPatrolFindingsEmptyState(args: {
  filter: FindingsFilter;
  overallHealth?: IntelligenceHealthScore;
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
}): PatrolFindingsEmptyStateCopy {
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

  if (shouldSuppressHealthyEmptyState(args.overallHealth)) {
    return {
      title: 'No active findings',
      body:
        args.overallHealth?.prediction?.trim() ||
        'Patrol has not surfaced active findings, but overall infrastructure health is not fully verified.',
      tone: getHealthDegradedTone(args.overallHealth!),
    };
  }

  return {
    title: 'No active findings',
    body: HEALTHY_PATROL_EMPTY_STATE_BODY,
    tone: 'success',
  };
}
