export function getInfrastructureEmptyState() {
  return {
    title: 'No infrastructure resources yet',
    description:
      'Add Proxmox VE nodes or install the Pulse agent on your infrastructure to start monitoring.',
    actionLabel: 'Add Infrastructure',
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
