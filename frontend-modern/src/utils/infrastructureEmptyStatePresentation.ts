import { getInfrastructureSourceStrategyDescription } from '@/utils/infrastructureSettingsPresentation';

export function getInfrastructureEmptyState() {
  return {
    title: 'No infrastructure sources yet',
    description: getInfrastructureSourceStrategyDescription(),
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
