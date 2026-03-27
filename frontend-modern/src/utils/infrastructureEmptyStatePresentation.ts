export function getInfrastructureEmptyState() {
  return {
    title: 'No infrastructure resources yet',
    description:
      'Start by opening Settings → Infrastructure → Install on a host and adding the first system you want Pulse to monitor. If you prefer a direct Proxmox connection instead, use Direct Proxmox.',
    actionLabel: 'Open Infrastructure Install',
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
