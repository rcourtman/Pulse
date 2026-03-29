export function getDashboardInfrastructureEmptyState() {
  return {
    title: 'No infrastructure hosts connected',
    description:
      'To start using Pulse, first add your infrastructure in Settings → Infrastructure → Install on a host. If you want an API-backed platform integration instead, use Settings → Infrastructure → Platform connections.',
    actionLabel: 'Open infrastructure setup',
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
    title: 'No resources yet',
    description:
      'Start by opening Settings → Infrastructure → Install on a host and connecting the first system you want Pulse to monitor. Your dashboard overview will appear here once that system starts reporting.',
    actionLabel: 'Open infrastructure install',
  } as const;
}
