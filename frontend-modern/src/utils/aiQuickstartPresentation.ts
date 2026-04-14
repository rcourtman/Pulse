export interface AIQuickstartCreditsPresentation {
  className: string;
  summary: string;
  title: string;
}

export const PATROL_QUICKSTART_EXHAUSTED_REASON =
  'Quickstart credits exhausted. Connect your API key to continue using Patrol.';

export function isPatrolQuickstartExhaustedReason(reason?: string | null): boolean {
  return reason?.trim() === PATROL_QUICKSTART_EXHAUSTED_REASON;
}

export function getAIQuickstartCreditsPresentation(
  remaining: number,
  total: number,
): AIQuickstartCreditsPresentation {
  if (remaining > 0) {
    return {
      className:
        'bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300',
      summary: `Patrol quickstart: ${remaining}/${total} runs left`,
      title: `${remaining} of ${total} Patrol quickstart runs remaining on this activated or trial-backed install. No API key needed for initial Patrol quickstart.`,
    };
  }

  return {
    className:
      'bg-amber-50 dark:bg-amber-950 border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-300',
    summary: 'Patrol quickstart exhausted',
    title:
      'Patrol quickstart on this activated or trial-backed install is exhausted. Connect your API key to continue using Patrol.',
  };
}
