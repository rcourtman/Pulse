export type SemanticTone = 'success' | 'warning' | 'error' | 'info';

export interface SemanticTonePresentation {
  panelClass: string;
  iconClass: string;
}

const SEMANTIC_TONE_PRESENTATION: Record<SemanticTone, SemanticTonePresentation> = {
  success: {
    panelClass: 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900',
    iconClass: 'text-green-600 dark:text-green-400',
  },
  warning: {
    panelClass: 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900',
    iconClass: 'text-amber-600 dark:text-amber-400',
  },
  error: {
    panelClass: 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900',
    iconClass: 'text-red-600 dark:text-red-400',
  },
  info: {
    panelClass: 'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900',
    iconClass: 'text-blue-600 dark:text-blue-400',
  },
};

export function getSemanticTonePresentation(tone: SemanticTone = 'info'): SemanticTonePresentation {
  return SEMANTIC_TONE_PRESENTATION[tone] || SEMANTIC_TONE_PRESENTATION.info;
}
