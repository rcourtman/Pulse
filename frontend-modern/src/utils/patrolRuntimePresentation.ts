import type { PatrolRuntimeState } from '@/api/patrol';
import type { SemanticTone } from '@/utils/semanticTonePresentation';

export interface PatrolRuntimePresentation {
  label: string;
  title: string;
  description: string;
  tone: SemanticTone;
}

const RETIRED_HOSTED_PATROL_BLOCKED_REASON =
  'Connect your own AI provider or local model to use Pulse Patrol.';

export function normalizePatrolRuntimeBlockedReason(blockedReason?: string): string {
  const reason = blockedReason?.trim() ?? '';
  if (!reason) {
    return '';
  }

  if (/\bquickstart\b/i.test(reason) || /\bhosted\b/i.test(reason)) {
    return RETIRED_HOSTED_PATROL_BLOCKED_REASON;
  }

  return reason;
}

export function getPatrolRuntimePresentation(
  runtimeState: PatrolRuntimeState | undefined,
  blockedReason?: string,
): PatrolRuntimePresentation {
  const normalizedBlockedReason = normalizePatrolRuntimeBlockedReason(blockedReason);

  switch (runtimeState) {
    case 'blocked':
      return {
        label: 'Patrol paused',
        title: 'Patrol paused',
        description:
          normalizedBlockedReason ||
          'Patrol cannot check infrastructure until the blocking condition is cleared.',
        tone: 'warning',
      };
    case 'disabled':
      return {
        label: 'Patrol disabled',
        title: 'Patrol disabled',
        description: 'Enable Patrol to resume checks.',
        tone: 'info',
      };
    case 'running':
      return {
        label: 'Patrol enabled',
        title: 'Patrol running',
        description: 'Patrol is checking your infrastructure now.',
        tone: 'info',
      };
    case 'unavailable':
      return {
        label: 'Patrol unavailable',
        title: 'Patrol unavailable',
        description: 'Patrol is not ready yet. Check Provider & Models and runtime availability.',
        tone: 'error',
      };
    case 'active':
    default:
      return {
        label: 'Patrol enabled',
        title: 'Patrol enabled',
        description: 'Patrol is ready to check your infrastructure.',
        tone: 'info',
      };
  }
}
