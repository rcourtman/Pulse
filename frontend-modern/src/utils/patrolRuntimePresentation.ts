import type { PatrolRuntimeState } from '@/api/patrol';
import type { SemanticTone } from '@/utils/semanticTonePresentation';

export interface PatrolRuntimePresentation {
  label: string;
  title: string;
  description: string;
  tone: SemanticTone;
}

export function getPatrolRuntimePresentation(
  runtimeState: PatrolRuntimeState | undefined,
  blockedReason?: string,
): PatrolRuntimePresentation {
  switch (runtimeState) {
    case 'blocked':
      return {
        label: 'Patrol Paused',
        title: 'Patrol paused',
        description:
          blockedReason?.trim() ||
          'Pulse Patrol cannot start new analysis until the blocking condition is cleared.',
        tone: 'warning',
      };
    case 'disabled':
      return {
        label: 'Patrol Disabled',
        title: 'Patrol disabled',
        description: 'Enable Patrol to resume monitoring and analysis.',
        tone: 'info',
      };
    case 'running':
      return {
        label: 'Patrol Running',
        title: 'Patrol running',
        description: 'Pulse Patrol is actively analyzing your infrastructure.',
        tone: 'info',
      };
    case 'unavailable':
      return {
        label: 'Patrol Unavailable',
        title: 'Patrol unavailable',
        description: 'Pulse Patrol is not ready yet. Check AI settings and runtime availability.',
        tone: 'error',
      };
    case 'active':
    default:
      return {
        label: 'Patrol enabled',
        title: 'Patrol enabled',
        description: 'Pulse Patrol is available to monitor and analyze your infrastructure.',
        tone: 'info',
      };
  }
}
