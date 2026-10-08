export type AIControlLevel = 'read_only' | 'controlled';

export interface AIChatControlLevelPresentation {
  label: string;
  description: string;
  pillClassName: string;
  dotClassName: string;
  selectedClassName: string;
}

// The server migrates the retired 'suggest' and 'autonomous' levels to
// controlled; mirror that so a stale value never shows as Read-only.
export function normalizeAIControlLevel(value?: string): AIControlLevel {
  if (value === 'controlled' || value === 'suggest' || value === 'autonomous') {
    return 'controlled';
  }
  return 'read_only';
}

export const AI_CONTROL_LEVEL_PANEL_CLASS =
  'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/25';

export const AI_CONTROL_LEVEL_ASK_FIRST_BADGE_CLASS =
  'bg-amber-100 dark:bg-amber-900/25 text-amber-700 dark:text-amber-300';

export function getAIControlLevelDescription(level: AIControlLevel): string {
  switch (level) {
    case 'controlled':
      return 'Assistant can plan actions, such as restarting a container, and saves each plan to Actions for you to review and run.';
    default:
      return 'Assistant answers questions and cannot plan actions.';
  }
}

export function getAIChatControlLevelPresentation(
  level: AIControlLevel,
): AIChatControlLevelPresentation {
  switch (level) {
    case 'controlled':
      return {
        label: 'Ask first',
        description: 'Plans actions for your review',
        pillClassName:
          'border-amber-200 text-amber-700 bg-amber-50 dark:border-amber-800 dark:text-amber-200 dark:bg-amber-900/25',
        dotClassName: 'bg-amber-500',
        selectedClassName: 'bg-amber-50 dark:bg-amber-900/25',
      };
    default:
      return {
        label: 'Read-only',
        description: 'Answers questions only',
        pillClassName: 'border-border text-muted bg-surface',
        dotClassName: 'bg-slate-400',
        selectedClassName: 'bg-surface-alt',
      };
  }
}
