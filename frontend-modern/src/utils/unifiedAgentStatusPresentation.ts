import { isConnectedHealthStatus } from '@/utils/status';

export type UnifiedAgentMonitoringState = 'active' | 'removed';

export interface UnifiedAgentStatusPresentation {
  badgeClass: string;
  label: string;
}

export const MONITORING_STOPPED_STATUS_LABEL = 'Monitoring stopped';
export const ALLOW_RECONNECT_LABEL = 'Allow reconnect';

export function getUnifiedAgentStatusPresentation(
  state: UnifiedAgentMonitoringState,
  healthStatus?: string | null,
): UnifiedAgentStatusPresentation {
  if (state === 'removed') {
    return {
      badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
      label: MONITORING_STOPPED_STATUS_LABEL,
    };
  }

  if (isConnectedHealthStatus(healthStatus)) {
    return {
      badgeClass: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
      label: healthStatus || 'unknown',
    };
  }

  return {
    badgeClass: 'bg-surface-alt text-base-content',
    label: healthStatus || 'unknown',
  };
}
