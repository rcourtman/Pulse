import type { Resource } from '@/types/resource';
import type { KnownSourcePlatform } from '@/utils/sourcePlatforms';
import { buildSourcePlatformOptions, type SourcePlatformOption } from '@/utils/sourcePlatformOptions';
import {
  buildStatusOptions,
  collectAvailableSources,
  collectAvailableStatuses,
  filterResources,
  tokenizeSearch,
} from '@/components/Infrastructure/infrastructureSelectors';

export interface InfrastructurePageFilterDerivation {
  activeFilterCount: number;
  availableSources: Set<KnownSourcePlatform>;
  sourceOptions: SourcePlatformOption[];
  statusOptions: Array<{ key: string; label: string }>;
  hasActiveFilters: boolean;
  filteredResources: Resource[];
  hasFilteredResources: boolean;
}

export function buildInfrastructurePageFilterDerivation(
  resources: Resource[],
  selectedSource: string,
  selectedStatus: string,
  searchQuery: string,
): InfrastructurePageFilterDerivation {
  const activeFilterCount =
    (selectedSource !== '' ? 1 : 0) + (selectedStatus !== '' ? 1 : 0);
  const availableSources = collectAvailableSources(resources);
  const sourceKeys = new Set<string>(availableSources);
  if (selectedSource !== '') {
    sourceKeys.add(selectedSource);
  }
  const sourceOptions = buildSourcePlatformOptions(sourceKeys);
  const statusOptions = buildStatusOptions(collectAvailableStatuses(resources));
  const hasActiveFilters =
    selectedSource !== '' || selectedStatus !== '' || searchQuery.trim().length > 0;
  const filteredResources = filterResources(
    resources,
    selectedSource !== '' ? new Set([selectedSource]) : new Set(),
    selectedStatus !== '' ? new Set([selectedStatus]) : new Set(),
    tokenizeSearch(searchQuery),
  );

  return {
    activeFilterCount,
    availableSources,
    sourceOptions,
    statusOptions,
    hasActiveFilters,
    filteredResources,
    hasFilteredResources: filteredResources.length > 0,
  };
}
