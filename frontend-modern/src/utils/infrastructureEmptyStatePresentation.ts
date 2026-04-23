export function getInfrastructureEmptyState() {
  return {
    title: 'No infrastructure sources yet',
    description:
      'Start in Settings → Infrastructure by choosing a source strategy. Connect a platform API for inventory and health, install Pulse Agent for host telemetry, or use both when you want full coverage.',
    actionLabel: 'Add infrastructure source',
  } as const;
}

export function getInfrastructureFilterEmptyState() {
  return {
    title: 'No resources match filters',
    description: 'Try adjusting the search, source, or status filters.',
    actionLabel: 'Clear filters',
  } as const;
}

export function getInfrastructureLoadFailureState() {
  return {
    title: 'Unable to load infrastructure',
    description: 'We couldn’t fetch unified resources. Check connectivity or retry.',
    actionLabel: 'Retry',
  } as const;
}
