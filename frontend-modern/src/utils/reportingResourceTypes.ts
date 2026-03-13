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
    case 'truenas':
      return 'agent';
    case 'jail':
      return 'system-container';
    case 'docker-service':
      return 'app-container';
    case 'k8s-cluster':
    case 'k8s-node':
    case 'pod':
    case 'k8s-deployment':
    case 'k8s-service':
      return 'k8s';
    case 'physical_disk':
      return 'disk';
    case 'ceph':
      return 'storage';
    default:
      return resourceType;
  }
}
