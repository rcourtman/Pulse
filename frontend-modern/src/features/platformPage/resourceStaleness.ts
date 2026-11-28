import type { Resource } from '@/types/resource';
import { getResourceSourceStatus, hasImpairedResourceSource } from '@/utils/resourceSourceHealth';
import { asTrimmedString } from '@/utils/stringUtils';
import { buildProviderStaleNotice } from '@/utils/truenasDatasetState';

export interface ResourceStaleness {
  // Backend-formatted age of the last report, e.g. "9m" or "1h"; null when
  // the backend did not say.
  age: string | null;
  label: string;
}

export type PlatformStaleNoticePlatform = 'docker' | 'kubernetes' | 'truenas' | 'vmware';

export interface PlatformResourceStaleNotice {
  label: string;
  detail: string;
}

const DOCKER_ERROR_SOURCE_STATUSES = new Set(['error', 'failed', 'unauthorized']);

const normalizeToken = (value: unknown): string =>
  typeof value === 'string'
    ? value
        .trim()
        .toLowerCase()
        .replace(/[\s_-]/g, '')
    : '';

export const ensureStaleNoticeSentence = (value: string): string => {
  const trimmed = value.trim();
  if (!trimmed) return '';
  return /[.!?]$/.test(trimmed) ? trimmed : `${trimmed}.`;
};

// The backend owns staleness: a resource whose sources stopped reporting
// carries a `telemetry_stale` health reason with the age, either as the reason
// for a `stale` verdict or behind an alert that outranks it. Tables use this
// to stop presenting the last received metrics as live.
export const getResourceStaleness = (
  resource: Pick<Resource, 'health'> | undefined | null,
): ResourceStaleness | null => {
  const health = resource?.health;
  if (!health) return null;
  const reason = health.reasons?.find((entry) => entry.code === 'telemetry_stale');
  if (!reason && health.verdict !== 'stale') return null;
  const age = reason?.detail?.trim() || null;
  return { age, label: age ? `No report for ${age}` : 'No recent report' };
};

// Shared stale-reading notice for cross-platform record tables (Alerts/Health
// and Activity) without coupling shared primitives to every platform page model.
export const getPlatformResourceStaleNotice = (
  platform: PlatformStaleNoticePlatform,
  resource: Resource | undefined | null,
): PlatformResourceStaleNotice | null => {
  if (!resource) return null;
  const healthStale = getResourceStaleness(resource);
  const age = healthStale?.age ?? null;

  if (platform === 'docker') {
    const dockerSource = getResourceSourceStatus(resource, 'docker');
    const agentSource = getResourceSourceStatus(resource, 'agent');
    const dockerStatus = normalizeToken(dockerSource?.status);
    const agentStatus = normalizeToken(agentSource?.status);
    const isStale = Boolean(healthStale) || dockerStatus === 'stale' || agentStatus === 'stale';
    const isError =
      DOCKER_ERROR_SOURCE_STATUSES.has(dockerStatus) ||
      DOCKER_ERROR_SOURCE_STATUSES.has(agentStatus);
    if (!isStale && !isError) return null;
    const sourceError = asTrimmedString(dockerSource?.error) || asTrimmedString(agentSource?.error);
    const base =
      isError && !isStale
        ? 'Docker collection error — showing the last report received.'
        : age
          ? `No Docker report for ${age} — showing the last values received.`
          : 'Showing the last Docker report — no fresh update received recently.';
    return {
      label: 'Stale data',
      detail: sourceError ? `${base} Last error: ${sourceError}` : base,
    };
  }

  if (platform === 'kubernetes') {
    const impaired = hasImpairedResourceSource(resource, 'kubernetes');
    if (!healthStale && !impaired) return null;
    const sourceError = asTrimmedString(getResourceSourceStatus(resource, 'kubernetes')?.error);
    const base = age
      ? `No Kubernetes report for ${age} — showing the last values received.`
      : 'Showing the last Kubernetes report — no fresh update received recently.';
    return {
      label: 'Stale data',
      detail: sourceError ? `${base} Last error: ${sourceError}` : base,
    };
  }

  const providerLabel = platform === 'truenas' ? 'TrueNAS' : 'vCenter';
  const sourceEntry =
    platform === 'truenas'
      ? (resource.sourceStatus?.truenas ?? getResourceSourceStatus(resource, 'truenas'))
      : (resource.sourceStatus?.vmware ??
        resource.sourceStatus?.['vmware-vsphere'] ??
        getResourceSourceStatus(resource, 'vmware-vsphere'));
  const providerNotice = buildProviderStaleNotice(sourceEntry, providerLabel);
  if (providerNotice) {
    const detail =
      age && !providerNotice.detail.includes(age)
        ? `${providerNotice.detail} (last report ${age} ago)`
        : providerNotice.detail;
    return { label: providerNotice.badge, detail };
  }
  if (!healthStale) return null;
  return {
    label: 'Stale data',
    detail: age
      ? `No ${providerLabel} report for ${age} — showing the last values received.`
      : `${providerLabel} has not updated this recently.`,
  };
};
