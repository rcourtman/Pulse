import type { ResourceType } from '@/types/resource';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

export interface ResourceTypePresentation {
  label: string;
  badgeClasses: string;
}

const DEFAULT_BADGE_CLASSES = 'bg-surface-alt text-base-content';

const RESOURCE_TYPE_PRESENTATION: Partial<Record<ResourceType | string, ResourceTypePresentation>> =
  {
    agent: {
      label: 'Agent',
      badgeClasses: 'bg-blue-500 text-blue-300',
    },
    'docker-host': {
      label: 'Container Runtime',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-cluster': {
      label: 'K8s Cluster',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-node': {
      label: 'K8s Node',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    vm: {
      label: 'VM',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'system-container': {
      label: 'Container',
      badgeClasses: 'bg-blue-500 text-blue-300',
    },
    'oci-container': {
      label: 'Container',
      badgeClasses: 'bg-blue-500 text-blue-300',
    },
    'app-container': {
      label: 'App Container',
      badgeClasses: 'bg-blue-500 text-blue-300',
    },
    pod: {
      label: 'Pod',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    jail: {
      label: 'Jail',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'docker-service': {
      label: 'Docker Service',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'docker-image': {
      label: 'Image',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'docker-volume': {
      label: 'Volume',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'docker-network': {
      label: 'Network',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'docker-task': {
      label: 'Swarm Task',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'docker-swarm-node': {
      label: 'Swarm Node',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-deployment': {
      label: 'K8s Deployment',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-replicaset': {
      label: 'ReplicaSet',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-service': {
      label: 'K8s Service',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-namespace': {
      label: 'Namespace',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-statefulset': {
      label: 'StatefulSet',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-daemonset': {
      label: 'DaemonSet',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-job': {
      label: 'Job',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-cronjob': {
      label: 'CronJob',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-ingress': {
      label: 'Ingress',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-endpoint-slice': {
      label: 'EndpointSlice',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-network-policy': {
      label: 'NetworkPolicy',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-persistent-volume': {
      label: 'PV',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-persistent-volume-claim': {
      label: 'PVC',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-storage-class': {
      label: 'StorageClass',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-configmap': {
      label: 'ConfigMap',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-serviceaccount': {
      label: 'ServiceAccount',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'k8s-event': {
      label: 'Event',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    storage: {
      label: 'Storage',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    datastore: {
      label: 'Datastore',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    pool: {
      label: 'Pool',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    dataset: {
      label: 'Dataset',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    pbs: {
      label: 'PBS',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    pmg: {
      label: 'PMG',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    physical_disk: {
      label: 'Physical Disk',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'network-share': {
      label: 'Network Share',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    ceph: {
      label: 'Ceph',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
    'network-endpoint': {
      label: 'Network Endpoint',
      badgeClasses: DEFAULT_BADGE_CLASSES,
    },
  };

const EXTERNAL_TYPE_PRESENTATION: Record<string, ResourceTypePresentation> = {
  'proxmox-vm': RESOURCE_TYPE_PRESENTATION.vm!,
  'proxmox-vm-backup': RESOURCE_TYPE_PRESENTATION.vm!,
  'proxmox-lxc': {
    label: 'LXC',
    badgeClasses: RESOURCE_TYPE_PRESENTATION['system-container']!.badgeClasses,
  },
  'proxmox-guest': {
    label: 'Guest',
    badgeClasses: DEFAULT_BADGE_CLASSES,
  },
  'k8s-pvc': {
    label: 'PVC',
    badgeClasses: DEFAULT_BADGE_CLASSES,
  },
  'k8s-pod': RESOURCE_TYPE_PRESENTATION.pod!,
  'velero-backup': {
    label: 'Velero',
    badgeClasses: DEFAULT_BADGE_CLASSES,
  },
  'docker-container': {
    label: 'Container',
    badgeClasses: RESOURCE_TYPE_PRESENTATION['app-container']!.badgeClasses,
  },
  'truenas-dataset': RESOURCE_TYPE_PRESENTATION.dataset!,
  'truenas-replication-task': {
    label: 'Replication',
    badgeClasses: DEFAULT_BADGE_CLASSES,
  },
};

export const canonicalResourceTypeForDisplay = (value: string): string =>
  canonicalizeFrontendResourceType(value) || value.trim().toLowerCase();

export const getResourceTypePresentation = (
  resourceType?: ResourceType | string,
): ResourceTypePresentation | null => {
  if (!resourceType) return null;
  const rawType = resourceType.trim().toLowerCase();
  const externalPresentation = EXTERNAL_TYPE_PRESENTATION[rawType];
  if (externalPresentation) return externalPresentation;
  const canonicalType = canonicalResourceTypeForDisplay(resourceType);
  const presentation = RESOURCE_TYPE_PRESENTATION[canonicalType];
  if (presentation) return presentation;
  return {
    label: canonicalType,
    badgeClasses: DEFAULT_BADGE_CLASSES,
  };
};

export const getResourceTypeLabel = (resourceType?: ResourceType | string): string | null =>
  getResourceTypePresentation(resourceType)?.label || null;
