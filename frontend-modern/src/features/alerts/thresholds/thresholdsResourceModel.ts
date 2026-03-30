import { unwrap } from 'solid-js/store';
import { getPreferredResourceHostname } from '@/utils/resourceIdentity';

import { requiresGovernedResourceDisplay } from '@/types/resource';
import type { Resource } from '@/types/resource';
import {
  getAgentDiscoveryResourceId,
  isAppContainerDiscoveryResourceType,
} from '@/utils/discoveryTarget';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';

import type { GroupHeaderMeta } from './tableTypes';
import type { Override, ThresholdsTableProps } from './types';

export interface ThresholdsDataInputs {
  props: ThresholdsTableProps;
  editingId: () => string | null;
  searchTerm: () => string;
}

export const platformData = (resource: Resource): Record<string, unknown> | undefined =>
  resource.platformData ? (unwrap(resource.platformData) as Record<string, unknown>) : undefined;

export const readRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

export const readString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

export const uniqueIds = (...values: unknown[]): string[] => {
  const ids: string[] = [];
  const seen = new Set<string>();
  values.forEach((value) => {
    const normalized = readString(value);
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    ids.push(normalized);
  });
  return ids;
};

export const createOverridesMap = (overrides: Override[] | undefined) =>
  new Map((overrides ?? []).map((override) => [override.id, override]));

export const hasThresholdDiff = (
  override: Override | undefined,
  defaults: Record<string, number | undefined>,
) =>
  Boolean(
    override?.thresholds &&
      Object.keys(override.thresholds).some((key) => {
        const thresholdKey = key as keyof Override['thresholds'];
        return (
          override.thresholds[thresholdKey] !== undefined &&
          override.thresholds[thresholdKey] !== defaults[key]
        );
      }),
  );

export function hostOverrideIdCandidates(resource: Resource): string[] {
  const data = platformData(resource);
  const agent = readRecord(data?.agent);
  const discoveryTarget = resource.discoveryTarget ?? null;
  return uniqueIds(
    getAgentDiscoveryResourceId(discoveryTarget),
    discoveryTarget?.agentId,
    resource.agent?.agentId,
    agent?.agentId,
    data?.agentId,
    resource.id,
  );
}

export const hostActionId = (resource: Resource): string =>
  hostOverrideIdCandidates(resource)[0] || resource.id;

export const dockerHostOverrideIdCandidates = (resource: Resource): string[] => {
  const data = platformData(resource);
  const docker = readRecord(data?.docker);
  const discoveryTarget = resource.discoveryTarget;
  return uniqueIds(
    isAppContainerDiscoveryResourceType(discoveryTarget?.resourceType)
      ? discoveryTarget?.resourceId
      : undefined,
    docker?.hostSourceId,
    data?.hostSourceId,
    discoveryTarget?.agentId,
    resource.id,
  );
};

export const dockerContainerOverrideIdCandidates = (host: Resource, shortId: string): string[] =>
  uniqueIds(
    ...dockerHostOverrideIdCandidates(host).map((hostId) => `docker:${hostId}/${shortId}`),
  );

export const findOverrideByCandidates = (
  overridesMap: Map<string, Override>,
  candidates: string[],
): Override | undefined => {
  for (const candidate of candidates) {
    const override = overridesMap.get(candidate);
    if (override) {
      return override;
    }
  }
  return undefined;
};

export const getFriendlyNodeName = (value: string, clusterName?: string): string => {
  if (!value) return value;

  const clusterLower = clusterName?.toLowerCase().trim();

  const normalizeToken = (token?: string | null): string => {
    if (!token) return '';
    let result = token
      .replace(/\(.*?\)/g, ' ')
      .replace(/\s+/g, ' ')
      .trim();
    if (clusterLower) {
      result = result
        .split(' ')
        .filter((part) => part.toLowerCase() !== clusterLower)
        .join(' ')
        .trim();
    }
    if (!result) return '';
    const firstWord = result.split(/\s+/)[0] || result;
    const withoutDomain = firstWord.includes('.') ? (firstWord.split('.')[0] ?? firstWord) : firstWord;
    return withoutDomain.trim();
  };

  const parentheticalMatch = value.match(/\(([^)]+)\)/);
  const parentheticalRaw = parentheticalMatch?.[1]?.trim();

  let base = normalizeToken(value);
  if (!base) {
    base = value.trim();
  }

  const parenthetical = normalizeToken(parentheticalRaw);
  if (parenthetical && parenthetical.toLowerCase() !== base.toLowerCase()) {
    return parenthetical;
  }

  return base;
};

export const getFriendlyAlertNodeName = (
  value: string,
  policy?: Resource['policy'],
  clusterName?: string,
): string => (requiresGovernedResourceDisplay(policy) ? value : getFriendlyNodeName(value, clusterName));

export function buildNodeHeaderMeta(node: Resource) {
  const data = platformData(node);
  const clusterName = (data?.clusterName as string | undefined) ?? undefined;
  const isClusterMember =
    (data?.isClusterMember as boolean | undefined) ?? Boolean(node.clusterId);

  const originalDisplayName = getAlertResourceDisplayLabel(node);
  const friendlyName = getFriendlyAlertNodeName(originalDisplayName, node.policy, clusterName);

  const guestUrlValue = typeof data?.guestURL === 'string' ? data.guestURL.trim() : '';
  const hostValue = typeof data?.host === 'string' ? data.host.trim() : '';

  let host: string | undefined;
  if (guestUrlValue && guestUrlValue !== '') {
    host = guestUrlValue.startsWith('http') ? guestUrlValue : `https://${guestUrlValue}`;
  } else if (hostValue && hostValue !== '') {
    host = hostValue.startsWith('http')
      ? hostValue
      : `https://${hostValue.includes(':') ? hostValue : `${hostValue}:8006`}`;
  } else if (node.name) {
    host = `https://${node.name.includes(':') ? node.name : `${node.name}:8006`}`;
  }

  const headerMeta: GroupHeaderMeta = {
    type: 'node',
    displayName: friendlyName,
    rawName: originalDisplayName,
    host,
    status: node.status,
    clusterName: isClusterMember ? clusterName?.trim() || 'Cluster' : undefined,
    isClusterMember,
  };

  const keys = new Set<string>();
  [node.name, originalDisplayName, friendlyName].forEach((value) => {
    if (value && value.trim()) {
      keys.add(value.trim());
    }
  });

  return { headerMeta, keys };
}

export function buildAgentHeaderMeta(agent: Resource) {
  const displayName = getAlertResourceDisplayLabel(agent);
  const rawName = getPreferredResourceHostname(agent) || agent.name || agent.id;

  const headerMeta: GroupHeaderMeta = {
    type: 'agent',
    displayName,
    rawName,
    status: agent.status,
  };

  const keys = new Set<string>();
  [displayName, rawName, agent.id].forEach((value) => {
    if (value && value.trim()) {
      keys.add(value.trim());
    }
  });

  return { headerMeta, keys };
}

export const agentDiskResourceId = (agentId: string, mountpoint: string, device?: string): string => {
  let label = (mountpoint?.trim() || device?.trim() || 'disk').toLowerCase();
  label = label
    .replace(/[^a-z0-9]/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-|-$/g, '');
  if (!label) label = 'unknown';
  return `agent:${agentId}/disk:${label}`;
};

export const storageCoords = (resource: Resource): { node: string; instance: string } => {
  const data = platformData(resource);
  if (resource.type === 'datastore') {
    const instance =
      (data?.pbsInstanceId as string | undefined) || resource.parentId || resource.platformId || 'pbs';
    const node = (data?.pbsInstanceName as string | undefined) || instance;
    return { node, instance };
  }
  return {
    node: (data?.node as string | undefined) || '',
    instance: (data?.instance as string | undefined) || resource.platformId || '',
  };
};

export const normalizeStorageStatus = (status: string | undefined): string => {
  switch ((status ?? '').toLowerCase()) {
    case 'online':
    case 'running':
    case 'available':
      return 'available';
    default:
      return 'offline';
  }
};
