import type { DeployTargetStatus } from '@/types/agentDeploy';

export interface DeployStatusPresentation {
  label: string;
  className: string;
}

const DEPLOY_STATUS_PRESENTATION: Record<DeployTargetStatus, DeployStatusPresentation> = {
  pending: { label: 'Pending', className: 'bg-surface-alt text-muted' },
  preflighting: {
    label: 'Checking',
    className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 animate-pulse',
  },
  ready: {
    label: 'Ready',
    className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
  },
  installing: {
    label: 'Installing',
    className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 animate-pulse',
  },
  enrolling: {
    label: 'Enrolling',
    className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 animate-pulse',
  },
  verifying: {
    label: 'Verifying',
    className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 animate-pulse',
  },
  succeeded: {
    label: 'Deployed',
    className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
  },
  failed_retryable: {
    label: 'Failed',
    className: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
  },
  failed_permanent: {
    label: 'Failed',
    className: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
  },
  skipped_already_agent: {
    label: 'Already monitored',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
  skipped_license: {
    label: 'License limit',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
  canceled: { label: 'Canceled', className: 'bg-surface-alt text-muted' },
};

export const getDeployStatusPresentation = (
  status?: DeployTargetStatus | null,
): DeployStatusPresentation => DEPLOY_STATUS_PRESENTATION[status ?? 'pending'] ?? DEPLOY_STATUS_PRESENTATION.pending;
