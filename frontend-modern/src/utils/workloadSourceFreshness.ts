import type { ResourceSourceStatus } from '@/types/resource';
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import { buildProviderStaleNotice, type ProviderStaleNotice } from '@/utils/truenasDatasetState';

export interface WorkloadSourceFreshnessTarget {
  platformType?: string;
  platformScopes?: string[];
  workloadType?: string;
  sourceStatus?: Record<string, ResourceSourceStatus>;
}

const PROVIDER_LABELS: Record<string, string> = {
  'proxmox-pve': 'Proxmox VE',
  'vmware-vsphere': 'vCenter',
  docker: 'Docker',
  kubernetes: 'Kubernetes',
  truenas: 'TrueNAS',
  agent: 'Pulse Agent',
};

const findSourceStatus = (
  sourceStatus: Record<string, ResourceSourceStatus>,
  normalizedSource: string,
): ResourceSourceStatus | undefined => {
  for (const [key, entry] of Object.entries(sourceStatus)) {
    const normalizedKey = normalizeSourcePlatformKey(key) || key.trim().toLowerCase();
    if (normalizedKey === normalizedSource) return entry;
  }
  return undefined;
};

const resolvePrimaryPlatformSource = (guest: WorkloadSourceFreshnessTarget): string | null => {
  const fromPlatform = normalizeSourcePlatformKey(guest.platformType);
  if (fromPlatform) return fromPlatform;
  for (const scope of guest.platformScopes ?? []) {
    const normalized = normalizeSourcePlatformKey(scope);
    if (normalized) return normalized;
  }
  if (guest.workloadType === 'pod') return 'kubernetes';
  if (guest.workloadType === 'app-container') return 'docker';
  return 'proxmox-pve';
};

export const getWorkloadSourceStaleNotice = (
  guest: WorkloadSourceFreshnessTarget | undefined | null,
): ProviderStaleNotice | null => {
  const sourceStatus = guest?.sourceStatus;
  if (!guest || !sourceStatus) return null;
  const primary = resolvePrimaryPlatformSource(guest);
  if (!primary) return null;
  const entry = findSourceStatus(sourceStatus, primary);
  if (!entry) return null;
  return buildProviderStaleNotice(entry, PROVIDER_LABELS[primary] ?? 'Monitoring source');
};

export const formatWorkloadStaleDetail = (notice: ProviderStaleNotice): string =>
  /[.!?]$/.test(notice.detail) ? notice.detail : `${notice.detail}.`;
