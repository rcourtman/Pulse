import { getInfrastructureSourceStrategyDescription } from '@/utils/infrastructureSettingsPresentation';

export function getDashboardInfrastructureEmptyState() {
  return {
    title: 'No infrastructure sources connected',
    description: getInfrastructureSourceStrategyDescription(),
    actionLabel: 'Add infrastructure source',
  } as const;
}

export function getDashboardGuestsEmptyState(search: string) {
  const normalized = search.trim();
  return {
    title: 'No guests found',
    description:
      normalized.length > 0
        ? `No guests match your search "${normalized}"`
        : 'No guests match your current filters',
  } as const;
}

export function getDashboardLoadingState(reconnecting: boolean) {
  return {
    title: 'Loading dashboard data...',
    description: reconnecting
      ? 'Reconnecting to monitoring service…'
      : 'Connecting to monitoring service',
  } as const;
}

export function getDashboardDisconnectedState(reconnecting: boolean) {
  return {
    title: 'Connection lost',
    description: reconnecting
      ? 'Attempting to reconnect…'
      : 'Unable to connect to the backend server',
    actionLabel: reconnecting ? undefined : 'Reconnect now',
  } as const;
}

export function getDashboardDisconnectedBannerState(reconnecting: boolean) {
  return {
    title: 'Connection lost',
    description: reconnecting
      ? 'Real-time data is reconnecting. Showing last-known state.'
      : 'Real-time data is currently unavailable. Showing last-known state.',
    actionLabel: 'Reconnect',
  } as const;
}

export function getDashboardUnavailableState() {
  return {
    title: 'Dashboard unavailable',
    description: 'Real-time dashboard data is currently unavailable. Reconnect to try again.',
    actionLabel: 'Reconnect',
  } as const;
}

export function getDashboardNoResourcesState() {
  return {
    title: 'Connect your first infrastructure source',
    description:
      'The dashboard appears after Pulse receives its first monitored system. Add an infrastructure source with API inventory, Agent telemetry, or both, then this page becomes the live estate overview.',
    actionLabel: 'Add infrastructure source',
  } as const;
}
