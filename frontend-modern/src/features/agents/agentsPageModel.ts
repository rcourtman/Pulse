import type { Resource } from '@/types/resource';
import {
  buildStatusOptions,
  collectAvailableStatuses,
  filterResources,
  tokenizeSearch,
} from '@/components/Infrastructure/infrastructureSelectors';
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';

export interface AgentsPageModel {
  resources: Resource[];
}

export interface AgentsPageFilterModel {
  activeFilterCount: number;
  statusOptions: Array<{ key: string; label: string }>;
  hasActiveFilters: boolean;
  filteredResources: Resource[];
  hasFilteredResources: boolean;
}

export const isAgentsPageResource = (resource: Resource): boolean =>
  resource.type === 'agent' && normalizeSourcePlatformKey(resource.platformType) === 'agent';

export function buildAgentsPageModel(resources: readonly Resource[]): AgentsPageModel {
  return {
    resources: resources.filter(isAgentsPageResource),
  };
}

export function buildAgentsPageFilterModel(
  resources: Resource[],
  selectedStatus: string,
  searchQuery: string,
): AgentsPageFilterModel {
  const normalizedStatus = selectedStatus.trim().toLowerCase();
  const searchTerms = tokenizeSearch(searchQuery);
  const filteredResources = filterResources(
    resources,
    new Set(),
    normalizedStatus ? new Set([normalizedStatus]) : new Set(),
    searchTerms,
  );

  return {
    activeFilterCount: (normalizedStatus ? 1 : 0) + (searchTerms.length > 0 ? 1 : 0),
    statusOptions: buildStatusOptions(collectAvailableStatuses(resources)),
    hasActiveFilters: Boolean(normalizedStatus) || searchTerms.length > 0,
    filteredResources,
    hasFilteredResources: filteredResources.length > 0,
  };
}
