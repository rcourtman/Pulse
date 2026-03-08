import type { ResourceType } from '@/types/resource';

export const CANONICAL_RESOURCE_TYPES = [
  'agent',
  'docker-host',
  'k8s-cluster',
  'k8s-node',
  'truenas',
  'vm',
  'system-container',
  'app-container',
  'oci-container',
  'pod',
  'jail',
  'docker-service',
  'k8s-deployment',
  'k8s-service',
  'storage',
  'datastore',
  'pool',
  'dataset',
  'pbs',
  'pmg',
  'physical_disk',
  'ceph',
] as const satisfies readonly ResourceType[];

export const INVALID_RESOURCE_TYPE_ERROR = `Invalid resource type. Valid types: ${CANONICAL_RESOURCE_TYPES.join(', ')}`;

export const normalizeCanonicalResourceTypeInput = (value: string): string =>
  value.trim().toLowerCase();

export const isCanonicalResourceType = (value: string): value is ResourceType =>
  (CANONICAL_RESOURCE_TYPES as readonly string[]).includes(value);
