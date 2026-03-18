import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

const normalize = (value: string | null | undefined): string => (value || '').trim().toLowerCase();

export const canonicalizeAlertTargetType = (raw?: string): string | undefined => {
  const normalized = normalize(raw);
  if (!normalized) return undefined;
  if (normalized === 'k8s' || normalized === 'kubernetes') {
    return undefined;
  }
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
    normalized.startsWith('k8s-') ||
    normalized.startsWith('pod:') ||
    normalized.includes(':pod:')
  ) {
    return 'pod';
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
