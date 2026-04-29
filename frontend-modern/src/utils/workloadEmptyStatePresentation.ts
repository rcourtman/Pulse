import { getInfrastructureSourceStrategyDescription } from '@/utils/infrastructureSettingsPresentation';

export function getWorkloadsInfrastructureEmptyState() {
  return {
    title: 'No infrastructure sources connected',
    description: getInfrastructureSourceStrategyDescription(),
    actionLabel: 'Add infrastructure source',
  } as const;
}

export function getWorkloadsGuestsEmptyState(search: string) {
  const normalized = search.trim();
  return {
    title: 'No guests found',
    description:
      normalized.length > 0
        ? `No guests match your search "${normalized}"`
        : 'No guests match your current filters',
  } as const;
}

export function getWorkloadsLoadingState(reconnecting: boolean) {
  return {
    title: 'Loading workloads...',
    description: reconnecting
      ? 'Reconnecting to monitoring service…'
      : 'Connecting to monitoring service',
  } as const;
}

export function getWorkloadsDisconnectedState(reconnecting: boolean) {
  return {
    title: 'Connection lost',
    description: reconnecting
      ? 'Attempting to reconnect…'
      : 'Unable to connect to the backend server',
    actionLabel: reconnecting ? undefined : 'Reconnect now',
  } as const;
}

export function getWorkloadsDisconnectedBannerState(reconnecting: boolean) {
  return {
    title: 'Connection lost',
    description: reconnecting
      ? 'Real-time data is reconnecting. Showing last-known state.'
      : 'Real-time data is currently unavailable. Showing last-known state.',
    actionLabel: 'Reconnect',
  } as const;
}

export function getWorkloadsUnavailableState() {
  return {
    title: 'Workloads unavailable',
    description: 'Real-time workload data is currently unavailable. Reconnect to try again.',
    actionLabel: 'Reconnect',
  } as const;
}

export function getWorkloadsNoResourcesState() {
  return {
    title: 'Connect your first infrastructure source',
    description:
      'Workloads appear after Pulse receives its first monitored system. Add an infrastructure source with API inventory, Agent telemetry, or both, then return here to inspect running VMs, containers, and pods.',
    actionLabel: 'Add infrastructure source',
  } as const;
}
