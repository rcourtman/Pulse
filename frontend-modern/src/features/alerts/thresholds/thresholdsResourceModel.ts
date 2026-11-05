import { unwrap } from 'solid-js/store';
import { getPreferredResourceHostname } from '@/utils/resourceIdentity';

import type { Resource } from '@/types/resource';
import { getAgentDiscoveryResourceId } from '@/utils/discoveryTarget';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';
import {
  buildDockerHostIdentities,
  getDockerHostRowName,
} from '@/features/docker/dockerHostIdentity';

import type { Alert } from '@/types/api';
import type { GroupHeaderMeta, Resource as TableResource } from './tableTypes';
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

// A row matches the resource search by the name it shows or the full name it
// stands for: host rows shorten `titan.example.com` to `titan`, and a link
// that searches for the full name must still find the row.
// A qualified name as it reads on the row, and as it reads in the row's controls.
const qualifiedNames = (name?: string, qualifier?: string): string[] =>
  name && qualifier ? [`${name} ${qualifier}`, `${name} (${qualifier})`] : [];

export const matchesThresholdResourceSearch = (
  resource: { name?: string; displayName?: string; rawName?: string; qualifier?: string },
  search: string,
): boolean => {
  if (!search) return true;
  return [
    resource.name,
    resource.displayName,
    resource.rawName,
    ...qualifiedNames(resource.name, resource.qualifier),
  ].some((value) => value?.toLowerCase().includes(search));
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

// Single implementation shared with the overrides projection so the lookup
// chain (stable name key first, legacy ID forms trailing) cannot drift.
export {
  dockerContainerOverrideIdCandidates,
  dockerHostOverrideIdCandidates,
} from '../alertOverridesModel';

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

// A name is the one its owner reads elsewhere in Pulse, so it stays whole. Only a
// hostname gives up its domain, and an address has none to give up.
const HOSTNAME_WITH_DOMAIN = /^([^\s.]+)\.\S*[a-z]\S*$/i;

export const getFriendlyHostName = (rawName: string): string => {
  const name = rawName.trim();
  return name.match(HOSTNAME_WITH_DOMAIN)?.[1] ?? name;
};

const NAME_WITH_PARENTHETICAL = /^(.+?)\s*\(([^()]+)\)$/;

// A node row shows its cluster on its own, so `Prod (pve1)` and `pve1 (Prod)` drop
// the cluster's half. Any other parenthetical or cluster word is part of the name.
export const getFriendlyNodeName = (value: string, clusterName?: string): string => {
  if (!value) return value;

  const name = value.trim();
  const [, outer = '', inner = ''] = name.match(NAME_WITH_PARENTHETICAL) ?? [];
  const outerKey = outer.toLowerCase();
  const innerKey = inner.trim().toLowerCase();
  const clusterKey = clusterName?.trim().toLowerCase();

  if (outer && (outerKey === clusterKey || outerKey === innerKey)) {
    return getFriendlyHostName(inner);
  }
  if (inner && innerKey === clusterKey) {
    return getFriendlyHostName(outer);
  }
  return getFriendlyHostName(name);
};

export const getFriendlyAlertNodeName = (value: string, clusterName?: string): string =>
  getFriendlyNodeName(value, clusterName);

export interface ThresholdsDockerHostLabel {
  /** The host's full name. */
  rawName: string;
  /** Its name on this page: shortened, unless that makes it read like another host's. */
  name: string;
  /** What tells it from hosts with its hostname, from the Docker page's rule. */
  qualifier?: string;
  /** Its Containers group, named so each host keeps its own. */
  groupKey: string;
  /** Lowercased text a search for this host matches on its containers. */
  searchText: string;
}

export const buildThresholdsDockerHostLabels = (
  hosts: readonly Resource[],
): ((host: Resource) => ThresholdsDockerHostLabel) => {
  const identities = buildDockerHostIdentities(hosts);
  const rawNamesByFriendlyName = new Map<string, Set<string>>();
  for (const host of hosts) {
    const rawName = getAlertResourceDisplayLabel(host);
    const friendlyName = getFriendlyHostName(rawName);
    const rawNames = rawNamesByFriendlyName.get(friendlyName) ?? new Set<string>();
    rawNames.add(rawName);
    rawNamesByFriendlyName.set(friendlyName, rawNames);
  }

  return (host) => {
    const rawName = getAlertResourceDisplayLabel(host);
    const friendlyName = getFriendlyHostName(rawName);
    const name = (rawNamesByFriendlyName.get(friendlyName)?.size ?? 0) > 1 ? rawName : friendlyName;
    const qualifier = getDockerHostRowName(host, identities).qualifier ?? undefined;
    return {
      rawName,
      name,
      qualifier,
      groupKey: qualifier ? `${name} (${qualifier})` : name,
      searchText: [rawName, name, ...qualifiedNames(name, qualifier)].join('\n').toLowerCase(),
    };
  };
};

export function buildNodeHeaderMeta(node: Resource) {
  const data = platformData(node);
  const clusterName = (data?.clusterName as string | undefined) ?? undefined;
  const isClusterMember = (data?.isClusterMember as boolean | undefined) ?? Boolean(node.clusterId);

  const originalDisplayName = getAlertResourceDisplayLabel(node);
  const friendlyName = getFriendlyAlertNodeName(originalDisplayName, clusterName);

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

export const agentDiskResourceId = (
  agentId: string,
  mountpoint: string,
  device?: string,
): string => {
  let label = (mountpoint?.trim() || device?.trim() || 'disk').toLowerCase();
  label = label
    .replace(/[^a-z0-9]/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-|-$/g, '');
  if (!label) label = 'unknown';
  return `agent:${agentId}/disk:${label}`;
};

// The resource ID a machine's own alerts are raised on (internal/alerts host.go
// hostResourceID). Its threshold row and override are keyed on the bare host ID.
export const agentAlertResourceId = (hostId: string): string => {
  const trimmed = hostId.trim();
  if (trimmed.startsWith('agent:')) return trimmed;
  return `agent:${trimmed || 'unknown'}`;
};

export interface ActiveMetricAlertIndex {
  /** Metrics with an open alert, by the resource ID the alert was raised on. */
  byResource: Map<string, Set<string>>;
  /** Metrics with an open alert on a machine's disks, by the machine's "agent:<host>" key. */
  byMachine: Map<string, Set<string>>;
}

const addIndexedMetric = (index: Map<string, Set<string>>, key: string, metric: string) => {
  const metrics = index.get(key);
  if (metrics) metrics.add(metric);
  else index.set(key, new Set([metric]));
};

// The live alert map is keyed by alert ID ("<resource>::metric-threshold:<metric>"
// for threshold alerts), which a row cannot rebuild. Rows match on the fields
// each alert carries instead: the resource it was raised on and its metric type.
export const indexActiveMetricAlerts = (
  activeAlerts: Record<string, Alert> | undefined,
): ActiveMetricAlertIndex => {
  const byResource = new Map<string, Set<string>>();
  const byMachine = new Map<string, Set<string>>();
  Object.values(activeAlerts ?? {}).forEach((alert) => {
    const resourceId = alert?.resourceId?.trim();
    const metric = alert?.type?.trim();
    if (!resourceId || !metric) return;
    addIndexedMetric(byResource, resourceId, metric);
    // "agent:<host>/disk:<mount>", "agent:<host>/disk_temp:<device>". Other
    // children (sensors, RAID arrays) have limits of their own.
    const childSeparator = resourceId.indexOf('/');
    if (!resourceId.startsWith('agent:') || childSeparator < 0) return;
    const child = resourceId.slice(childSeparator + 1);
    if (child.startsWith('disk:') || child.startsWith('disk_temp:')) {
      addIndexedMetric(byMachine, resourceId.slice(0, childSeparator), metric);
    }
  });
  return { byResource, byMachine };
};

// Whether a threshold row has an open alert on one of its metrics. A machine
// row's Disk % and Disk Temp limits are what its disks' alerts judge, and
// those alerts are raised on the disks, so a machine row also answers for them.
export const thresholdRowHasActiveAlert = (
  index: ActiveMetricAlertIndex,
  row: Pick<TableResource, 'id' | 'alertResourceIds' | 'type'>,
  metric: string,
): boolean => {
  const alertResourceIds = row.alertResourceIds?.length ? row.alertResourceIds : [row.id];
  return alertResourceIds.some(
    (alertResourceId) =>
      index.byResource.get(alertResourceId)?.has(metric) ||
      (row.type === 'agent' && index.byMachine.get(alertResourceId)?.has(metric)),
  );
};

export const storageCoords = (resource: Resource): { node: string; instance: string } => {
  const data = platformData(resource);
  if (resource.type === 'datastore') {
    const instance =
      (data?.pbsInstanceId as string | undefined) ||
      resource.parentId ||
      resource.platformId ||
      'pbs';
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
