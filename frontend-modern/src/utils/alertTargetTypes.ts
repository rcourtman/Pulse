import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

const normalize = (value: string | null | undefined): string => (value || '').trim().toLowerCase();
const canonicalToken = (value: string): string => value.replace(/[\s_]+/g, '-');

const ALERT_TARGET_TYPE_ALIASES: Record<string, string> = {
  k8s: 'k8s-cluster',
  kubernetes: 'k8s-cluster',
  'k8s-cluster': 'k8s-cluster',
  'kubernetes-cluster': 'k8s-cluster',
  'k8s-node': 'k8s-node',
  'kubernetes-node': 'k8s-node',
  'k8s-namespace': 'k8s-namespace',
  'kubernetes-namespace': 'k8s-namespace',
  'k8s-deployment': 'k8s-deployment',
  'kubernetes-deployment': 'k8s-deployment',
  'k8s-pod': 'pod',
  'kubernetes-pod': 'pod',
  truenas: 'truenas',
  'truenas-system': 'truenas',
  'truenas-pool': 'pool',
  'truenas-dataset': 'truenas-dataset',
  'truenas-disk': 'physical_disk',
  vmware: 'vmware-vsphere',
  vsphere: 'vmware-vsphere',
  vmsphere: 'vmware-vsphere',
  'vmware-host': 'vmware-host',
  'vsphere-host': 'vmware-host',
  'vmware-vm': 'vmware-vm',
  'vsphere-vm': 'vmware-vm',
  'vmware-virtual-machine': 'vmware-vm',
  'vsphere-virtual-machine': 'vmware-vm',
  'vmware-datastore': 'vmware-datastore',
  'vsphere-datastore': 'vmware-datastore',
  'vmware-network': 'vmware-network',
  'vsphere-network': 'vmware-network',
};

export const canonicalizeAlertTargetType = (raw?: string): string | undefined => {
  const normalized = normalize(raw);
  if (!normalized) return undefined;
  const alias =
    ALERT_TARGET_TYPE_ALIASES[normalized] ?? ALERT_TARGET_TYPE_ALIASES[canonicalToken(normalized)];
  if (alias) return alias;
  if (normalized === 'docker-container' || normalized === 'docker-service') {
    return 'app-container';
  }
  return canonicalizeFrontendResourceType(raw);
};

export const inferAlertTargetTypeFromResourceId = (resourceID?: string): string | undefined => {
  const normalized = normalize(resourceID);
  if (!normalized) {
    return undefined;
  }

  if (
    normalized.startsWith('vm-') ||
    normalized.startsWith('qemu-') ||
    normalized.includes('/qemu/')
  ) {
    return 'vm';
  }
  if (
    normalized.startsWith('ct-') ||
    normalized.startsWith('lxc-') ||
    normalized.includes('/lxc/')
  ) {
    return 'system-container';
  }
  if (
    normalized.startsWith('docker:') ||
    normalized.startsWith('app-container:') ||
    normalized.includes('/container:')
  ) {
    return 'app-container';
  }
  if (normalized.startsWith('node:')) {
    return 'agent';
  }
  if (normalized.startsWith('storage:')) {
    return 'storage';
  }
  if (normalized.startsWith('disk:')) {
    return 'disk';
  }
  if (normalized.startsWith('pbs:')) {
    return 'pbs';
  }
  if (normalized.startsWith('pmg:')) {
    return 'pmg';
  }
  if (
    normalized.startsWith('k8s-pod:') ||
    normalized.startsWith('k8s-pod-') ||
    normalized.startsWith('pod:') ||
    normalized.includes(':pod:')
  ) {
    return 'pod';
  }
  if (normalized.startsWith('k8s:')) {
    if (normalized.includes(':deployment:')) return 'k8s-deployment';
    if (normalized.includes(':namespace:')) return 'k8s-namespace';
    if (normalized.includes(':node:')) return 'k8s-node';
    return 'k8s-cluster';
  }
  if (normalized.startsWith('truenas-system:') || normalized.startsWith('system:truenas')) {
    return 'truenas';
  }
  if (normalized.startsWith('truenas-pool:')) {
    return 'pool';
  }
  if (normalized.startsWith('truenas-dataset:')) {
    return 'truenas-dataset';
  }
  if (normalized.startsWith('truenas-disk:') || normalized.startsWith('physical-disk:')) {
    return 'physical_disk';
  }
  if (normalized.startsWith('vmware-host:') || normalized.startsWith('vsphere-host:')) {
    return 'vmware-host';
  }
  if (normalized.startsWith('vmware-vm:') || normalized.startsWith('vsphere-vm:')) {
    return 'vmware-vm';
  }
  if (normalized.startsWith('vmware-datastore:') || normalized.startsWith('vsphere-datastore:')) {
    return 'vmware-datastore';
  }
  if (normalized.startsWith('vmware-network:') || normalized.startsWith('vsphere-network:')) {
    return 'vmware-network';
  }
  if (normalized.startsWith('vmware:') || normalized.startsWith('vsphere:')) {
    if (normalized.includes(':host:')) return 'vmware-host';
    if (normalized.includes(':vm:')) return 'vmware-vm';
    if (normalized.includes(':datastore:')) return 'vmware-datastore';
    if (normalized.includes(':network:')) return 'vmware-network';
    return 'vmware-vsphere';
  }

  return undefined;
};

type ResolveAlertTargetTypeInput = {
  alertType?: string | null;
  resourceType?: string | null;
  metadataResourceType?: string | null;
  resourceId?: string | null;
};

export const resolveAlertTargetType = ({
  alertType,
  resourceType,
  metadataResourceType,
  resourceId,
}: ResolveAlertTargetTypeInput): string => {
  const normalizedAlertType = normalize(alertType);
  if (normalizedAlertType.startsWith('node_')) {
    return 'agent';
  }
  if (normalizedAlertType.startsWith('docker_')) {
    return 'app-container';
  }
  if (normalizedAlertType.startsWith('storage_')) {
    return 'storage';
  }

  const fromExplicitType = canonicalizeAlertTargetType(resourceType || undefined);
  if (fromExplicitType) {
    return fromExplicitType;
  }

  const fromMetadataType = canonicalizeAlertTargetType(metadataResourceType || undefined);
  if (fromMetadataType) {
    return fromMetadataType;
  }

  const fromResourceID = inferAlertTargetTypeFromResourceId(resourceId || undefined);
  if (fromResourceID) {
    return fromResourceID;
  }

  return 'agent';
};
