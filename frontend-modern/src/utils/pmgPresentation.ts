import { getInfrastructureSettingsLocationLabel } from '@/utils/infrastructureSettingsPresentation';

export const PMG_EMPTY_STATE_TITLE = 'No Mail Gateways configured';

export const PMG_EMPTY_STATE_DESCRIPTION =
  `Add a Proxmox Mail Gateway via ${getInfrastructureSettingsLocationLabel()} to start collecting mail analytics and security metrics.`;

export const PMG_LOADING_STATE_TITLE = 'Loading mail gateway data...';

export const PMG_LOADING_STATE_DESCRIPTION = 'Connecting to the monitoring service.';

export const PMG_DISCONNECTED_STATE_TITLE = 'Connection lost';

export const PMG_SEARCH_PLACEHOLDER = 'Search gateways...';

export function getPMGDisconnectedState(reconnecting: boolean) {
  return {
    title: PMG_DISCONNECTED_STATE_TITLE,
    description: reconnecting
      ? 'Attempting to reconnect…'
      : 'Unable to connect to the backend server',
    actionLabel: reconnecting ? undefined : 'Reconnect now',
  };
}

export function getPMGSearchEmptyState(term: string) {
  return {
    description: `No gateways match "${term}"`,
    actionLabel: 'Clear search',
  };
}
