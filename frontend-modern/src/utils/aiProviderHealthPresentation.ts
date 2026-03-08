export type AIProviderHealthStatus = 'not_configured' | 'checking' | 'ok' | 'error';

export interface AIProviderHealthPresentation {
  label: string;
  badgeClass: string;
}

const PRESENTATION: Record<AIProviderHealthStatus, AIProviderHealthPresentation> = {
  ok: {
    label: 'Healthy',
    badgeClass: 'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300',
  },
  error: {
    label: 'Issue',
    badgeClass: 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300',
  },
  checking: {
    label: 'Checking...',
    badgeClass: 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300',
  },
  not_configured: {
    label: 'Not checked',
    badgeClass: 'bg-surface-hover text-muted',
  },
};

export const getAIProviderHealthPresentation = (
  status: AIProviderHealthStatus,
): AIProviderHealthPresentation => PRESENTATION[status];
