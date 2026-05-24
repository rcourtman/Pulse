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
  | 'network'
  | 'network-share'
  | 'network-endpoint'
  | 'pbs'
  | 'pmg'
  | 'pod'
  | 'disk';

export function toReportingResourceType(resourceType: ResourceType): ReportingResourceType {
  switch (resourceType) {
    case 'jail':
      return 'system-container';
    case 'docker-service':
    case 'docker-image':
    case 'docker-task':
    case 'docker-swarm-node':
    case 'docker-secret':
    case 'docker-config':
      return 'app-container';
    case 'docker-volume':
      return 'storage';
    case 'docker-network':
      return 'network';
    case 'k8s-cluster':
    case 'k8s-node':
    case 'pod':
    case 'k8s-deployment':
    case 'k8s-replicaset':
    case 'k8s-service':
    case 'k8s-namespace':
    case 'k8s-statefulset':
    case 'k8s-daemonset':
    case 'k8s-job':
    case 'k8s-cronjob':
    case 'k8s-ingress':
    case 'k8s-endpoint-slice':
    case 'k8s-network-policy':
    case 'k8s-persistent-volume':
    case 'k8s-persistent-volume-claim':
    case 'k8s-storage-class':
    case 'k8s-configmap':
    case 'k8s-secret':
    case 'k8s-serviceaccount':
    case 'k8s-resource-quota':
    case 'k8s-limit-range':
    case 'k8s-pod-disruption-budget':
    case 'k8s-horizontal-pod-autoscaler':
    case 'k8s-event':
      return 'k8s';
    case 'physical_disk':
      return 'disk';
    case 'ceph':
      return 'storage';
    default:
      return resourceType;
  }
}
