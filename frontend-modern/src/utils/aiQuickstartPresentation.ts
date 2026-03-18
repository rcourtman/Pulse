export interface AIQuickstartCreditsPresentation {
  className: string;
  summary: string;
  title: string;
}

export function getAIQuickstartCreditsPresentation(
  remaining: number,
  total: number,
): AIQuickstartCreditsPresentation {
  if (remaining > 0) {
    return {
      className:
        'bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300',
      summary: `${remaining}/${total} quickstart credits`,
      title: `${remaining}/${total} free quickstart patrol runs remaining. No API key needed.`,
    };
  }

  return {
    className:
      'bg-amber-50 dark:bg-amber-950 border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-300',
    summary: 'Credits exhausted — connect API key',
    title: 'Quickstart credits exhausted. Connect your API key to continue using AI Patrol.',
  };
}
