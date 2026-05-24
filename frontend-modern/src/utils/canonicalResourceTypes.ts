import type { ResourceType } from '@/types/resource';

export const CANONICAL_RESOURCE_TYPES = [
  'agent',
  'docker-host',
  'k8s-cluster',
  'k8s-node',
  'vm',
  'system-container',
  'app-container',
  'oci-container',
  'pod',
  'jail',
  'docker-service',
  'docker-image',
  'docker-volume',
  'docker-network',
  'docker-task',
  'docker-swarm-node',
  'k8s-deployment',
  'k8s-replicaset',
  'k8s-service',
  'k8s-namespace',
  'k8s-statefulset',
  'k8s-daemonset',
  'k8s-job',
  'k8s-cronjob',
  'k8s-ingress',
  'k8s-endpoint-slice',
  'k8s-network-policy',
  'k8s-persistent-volume',
  'k8s-persistent-volume-claim',
  'k8s-storage-class',
  'k8s-configmap',
  'k8s-secret',
  'k8s-serviceaccount',
  'k8s-resource-quota',
  'k8s-limit-range',
  'k8s-pod-disruption-budget',
  'k8s-horizontal-pod-autoscaler',
  'k8s-event',
  'storage',
  'datastore',
  'pool',
  'dataset',
  'pbs',
  'pmg',
  'physical_disk',
  'network-share',
  'ceph',
  'network-endpoint',
] as const satisfies readonly ResourceType[];

export const INVALID_RESOURCE_TYPE_ERROR = `Invalid resource type. Valid types: ${CANONICAL_RESOURCE_TYPES.join(', ')}`;

export const normalizeCanonicalResourceTypeInput = (value: string): string =>
  value.trim().toLowerCase();

export const isCanonicalResourceType = (value: string): value is ResourceType =>
  (CANONICAL_RESOURCE_TYPES as readonly string[]).includes(value);
