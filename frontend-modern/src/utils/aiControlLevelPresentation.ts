export type AIControlLevel = 'read_only' | 'controlled';

export interface AIChatControlLevelPresentation {
  label: string;
  description: string;
  pillClassName: string;
  dotClassName: string;
  selectedClassName: string;
}

// Legacy saved values still mean action planning, never permission to execute.
// Mirror the server's interactive projection without rewriting the preference.
export function normalizeAIControlLevel(value?: string): AIControlLevel {
  if (value === 'controlled' || value === 'suggest' || value === 'autonomous') {
    return 'controlled';
  }
  return 'read_only';
}

export function getAIControlLevelPanelClass(_level: AIControlLevel): string {
  return 'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/25';
}

export function getAIControlLevelBadgeClass(level: AIControlLevel): string {
  switch (level) {
    case 'controlled':
      return 'bg-amber-100 dark:bg-amber-900/25 text-amber-700 dark:text-amber-300';
    default:
      return 'bg-blue-100 dark:bg-blue-900/25 text-blue-700 dark:text-blue-300';
  }
}

export function getAIControlLevelDescription(level: AIControlLevel): string {
  switch (level) {
    case 'controlled':
      return 'Assistant can plan infrastructure actions and saves each plan to Actions for you to review and run. Chat does not execute the plan.';
    default:
      return 'Assistant can query and explain only; it cannot plan infrastructure actions.';
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
        description: 'Observes only',
        pillClassName: 'border-border text-muted bg-surface',
        dotClassName: 'bg-slate-400',
        selectedClassName: 'bg-surface-alt',
      };
  }
}
