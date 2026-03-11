import type { Resource, ResourceType } from '@/types/resource';

export type ResourcePickerTypeFilter =
  | 'all'
  | 'infrastructure'
  | 'workloads'
  | 'storage'
  | 'recovery';

const RESOURCE_PICKER_TYPE_FILTER_LABELS: Record<ResourcePickerTypeFilter, string> = {
  all: 'All',
  infrastructure: 'Infrastructure',
  workloads: 'Workloads',
  storage: 'Storage',
  recovery: 'Recovery',
};

export const REPORTABLE_RESOURCE_TYPES = new Set<ResourceType>([
  'agent',
  'docker-host',
  'k8s-cluster',
  'k8s-node',
  'vm',
  'system-container',
  'app-container',
  'oci-container',
  'pod',
  'storage',
  'datastore',
  'pool',
  'dataset',
  'pbs',
  'pmg',
]);

const INFRASTRUCTURE_TYPES = new Set<ResourceType>([
  'agent',
  'docker-host',
  'k8s-cluster',
  'k8s-node',
  'pbs',
  'pmg',
]);

const WORKLOAD_TYPES = new Set<ResourceType>([
  'vm',
  'system-container',
  'app-container',
  'oci-container',
  'pod',
]);

const STORAGE_TYPES = new Set<ResourceType>(['storage', 'datastore', 'pool', 'dataset']);

const RECOVERY_TYPES = new Set<ResourceType>(['pbs', 'datastore']);

const SORT_ORDER: Record<string, number> = {
  agent: 0,
  'docker-host': 1,
  k8s: 2,
  pbs: 3,
  pmg: 4,
  vm: 5,
  'system-container': 6,
  'app-container': 7,
  pod: 8,
  storage: 9,
  datastore: 10,
  pool: 11,
  dataset: 12,
};

export const normalizeReportableResourceType = (type: ResourceType): string => {
  if (type === 'system-container' || type === 'oci-container') return 'system-container';
  if (type === 'app-container') return 'app-container';
  if (type === 'docker-host') return 'docker-host';
  if (type === 'k8s-node') return 'node';
  if (type === 'k8s-cluster') return 'k8s';
  return type;
};

export const matchesReportableResourceTypeFilter = (
  resource: Pick<Resource, 'type'>,
  filter: ResourcePickerTypeFilter,
): boolean => {
  if (filter === 'all') return true;
  if (filter === 'infrastructure') return INFRASTRUCTURE_TYPES.has(resource.type);
  if (filter === 'workloads') return WORKLOAD_TYPES.has(resource.type);
  if (filter === 'storage') return STORAGE_TYPES.has(resource.type);
  if (filter === 'recovery') return RECOVERY_TYPES.has(resource.type);
  return true;
};

export const reportableResourceTypeSortOrder = (type: ResourceType): number =>
  SORT_ORDER[normalizeReportableResourceType(type)] ?? 13;

export const getResourcePickerTypeFilterLabel = (filter: ResourcePickerTypeFilter): string =>
  RESOURCE_PICKER_TYPE_FILTER_LABELS[filter];

export const getResourcePickerEmptyState = (hasResources: boolean) =>
  hasResources
    ? {
        title: 'No resources match your filters',
      }
    : {
        title: 'No resources available',
        description: 'Resources appear as Pulse collects infrastructure and workload metrics',
      };

export const RESOURCE_PICKER_TYPE_FILTERS: ResourcePickerTypeFilter[] = [
  'all',
  'infrastructure',
  'workloads',
  'storage',
  'recovery',
];
