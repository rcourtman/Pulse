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
        label: 'Patrol Paused',
        title: 'Patrol paused',
        description:
          normalizedBlockedReason ||
          'Pulse Patrol cannot start new verification until the blocking condition is cleared.',
        tone: 'warning',
      };
    case 'disabled':
      return {
        label: 'Patrol Disabled',
        title: 'Patrol disabled',
        description: 'Enable Patrol to resume continuous verification.',
        tone: 'info',
      };
    case 'running':
      return {
        label: 'Patrol Running',
        title: 'Patrol running',
        description: 'Pulse Patrol is actively verifying your infrastructure.',
        tone: 'info',
      };
    case 'unavailable':
      return {
        label: 'Patrol Unavailable',
        title: 'Patrol unavailable',
        description: 'Pulse Patrol is not ready yet. Check Patrol provider settings and runtime availability.',
        tone: 'error',
      };
    case 'active':
    default:
      return {
        label: 'Patrol enabled',
        title: 'Patrol enabled',
        description: 'Pulse Patrol is ready to continuously verify your infrastructure.',
        tone: 'info',
      };
  }
}
