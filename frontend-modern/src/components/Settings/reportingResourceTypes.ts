import type { ResourceType } from '@/types/resource';

export type ReportingResourceType =
  | 'agent'
  | 'docker-host'
  | 'vm'
  | 'system-container'
  | 'app-container'
  | 'oci-container'
  | 'k8s'
  | 'storage'
  | 'datastore'
  | 'pool'
  | 'dataset'
  | 'pbs'
  | 'pmg'
  | 'pod'
  | 'disk';

export function toReportingResourceType(resourceType: ResourceType): ReportingResourceType {
  switch (resourceType) {
    case 'k8s-cluster':
    case 'k8s-node':
    case 'pod':
      return 'k8s';
    default:
      return resourceType;
  }
}
