export type AIControlLevel = 'read_only' | 'controlled' | 'autonomous';

export interface AIChatControlLevelPresentation {
  label: string;
  description: string;
  pillClassName: string;
  dotClassName: string;
  selectedClassName: string;
}

export function normalizeAIControlLevel(value?: string): AIControlLevel {
  if (value === 'controlled' || value === 'autonomous' || value === 'read_only') {
    return value;
  }
  if (value === 'suggest') {
    return 'controlled';
  }
  return 'read_only';
}

export function getAIControlLevelPanelClass(level: AIControlLevel): string {
  return level === 'autonomous'
    ? 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900'
    : 'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900';
}

export function getAIControlLevelBadgeClass(level: AIControlLevel): string {
  switch (level) {
    case 'autonomous':
      return 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300';
    case 'controlled':
      return 'bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300';
    default:
      return 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300';
  }
}

export function getAIControlLevelDescription(level: AIControlLevel): string {
  switch (level) {
    case 'controlled':
      return 'Assistant asks before chat-only actions. Infrastructure work stays with Patrol mode.';
    case 'autonomous':
      return 'Assistant may take eligible chat-only actions allowed by policy. Infrastructure work stays with Patrol mode.';
    default:
      return 'Assistant can query and explain only. Infrastructure work stays with Patrol mode.';
  }
}

export function getAIChatControlLevelPresentation(
  level: AIControlLevel,
): AIChatControlLevelPresentation {
  switch (level) {
    case 'autonomous':
      return {
        label: 'Chat actions',
        description: 'Eligible chat-only actions',
        pillClassName:
          'border-red-200 text-red-700 bg-red-50 dark:border-red-800 dark:text-red-200 dark:bg-red-900',
        dotClassName: 'bg-red-500',
        selectedClassName: 'bg-red-50 dark:bg-red-900',
      };
    case 'controlled':
      return {
        label: 'Ask first',
        description: 'Asks before chat-only actions',
        pillClassName:
          'border-amber-200 text-amber-700 bg-amber-50 dark:border-amber-800 dark:text-amber-200 dark:bg-amber-900',
        dotClassName: 'bg-amber-500',
        selectedClassName: 'bg-amber-50 dark:bg-amber-900',
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
