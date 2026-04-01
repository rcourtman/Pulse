import type { PBSInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';
import { getActionableAgentIdFromResource, isTrueNASSystemResource } from '@/utils/agentResources';
import { isAppContainerDiscoveryResourceType } from '@/utils/discoveryTarget';

import {
  extractTriggerValues,
  getAlertResourceDisplayLabel,
  guessNumericId,
  platformData,
} from './helpers';
import {
  getGuestOverrideIdentity,
  guestOverrideIdCandidates,
  guestOverrideStorageId,
  normalizeGuestOverrideKey,
} from './guestOverrideIdentity';
import type { Override } from './types';

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const uniqueIds = (...values: unknown[]): string[] => {
  const ids: string[] = [];
  const seen = new Set<string>();

  values.forEach((value) => {
    const normalized = asString(value);
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    ids.push(normalized);
  });

  return ids;
};

export const normalizeRawOverridesConfig = (
  rawOverrides: Record<string, RawOverrideConfig>,
): Record<string, RawOverrideConfig> => {
  const cleanedOverrides: Record<string, RawOverrideConfig> = {};
  const priorityByKey = new Map<string, number>();

  for (const [key, value] of Object.entries(rawOverrides)) {
    const normalizedGuestKey = normalizeGuestOverrideKey(key);
    const diskMatch = normalizedGuestKey.match(/^(agent:.+\/disk:)(.+)$/);
    let normalizedKey = normalizedGuestKey;
    if (diskMatch) {
      const normalized =
        diskMatch[2]
          .toLowerCase()
          .replace(/[^a-z0-9]/g, '-')
          .replace(/-{2,}/g, '-')
          .replace(/^-|-$/g, '') || 'unknown';
      normalizedKey = diskMatch[1] + normalized;
    }

    const priority = normalizedKey === key ? 1 : 0;
    const existingPriority = priorityByKey.get(normalizedKey);
    if (existingPriority !== undefined && existingPriority > priority) {
      continue;
    }

    priorityByKey.set(normalizedKey, priority);
    cleanedOverrides[normalizedKey] = value;
  }

  return cleanedOverrides;
};

export const hostOverrideIdCandidates = (resource: Resource): string[] => {
  const data = platformData(resource);
  const agent = asRecord(data?.agent);
  return uniqueIds(
    getActionableAgentIdFromResource(resource),
    resource.discoveryTarget?.agentId,
    resource.agent?.agentId,
    agent?.agentId,
    data?.agentId,
    resource.id,
  );
};

export const dockerHostOverrideIdCandidates = (resource: Resource): string[] => {
  const data = platformData(resource);
  const docker = asRecord(data?.docker);
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
  uniqueIds(...dockerHostOverrideIdCandidates(host).map((hostId) => `docker:${hostId}/${shortId}`));

export const buildContainerRuntimeResources = ({
  allResources,
  dockerHostResources,
}: {
  allResources: Resource[];
  dockerHostResources: Resource[];
}): Resource[] => {
  const resourceById = new Map(allResources.map((resource) => [resource.id, resource]));
  const runtimes: Resource[] = [];
  const seen = new Set<string>();

  const addRuntime = (resource: Resource | undefined) => {
    if (!resource || seen.has(resource.id)) return;
    seen.add(resource.id);
    runtimes.push(resource);
  };

  dockerHostResources.forEach(addRuntime);
  allResources.forEach((resource) => {
    if (isTrueNASSystemResource(resource)) {
      addRuntime(resource);
    }
  });
  allResources.forEach((resource) => {
    if (resource.type !== 'app-container' || !resource.parentId) return;
    addRuntime(resourceById.get(resource.parentId));
  });

  return runtimes;
};

interface BuildProjectedOverridesArgs {
  rawConfig: Record<string, RawOverrideConfig>;
  nodeResources: Resource[];
  vmResources: Resource[];
  containerResources: Resource[];
  storageResources: Resource[];
  agentResourceList: Resource[];
  containerRuntimeResources: Resource[];
  getChildren: (resourceId: string) => Resource[];
  pbsInstanceById: Map<string, PBSInstance>;
}

export const buildProjectedOverrides = ({
  rawConfig,
  nodeResources,
  vmResources,
  containerResources,
  storageResources,
  agentResourceList,
  containerRuntimeResources,
  getChildren,
  pbsInstanceById,
}: BuildProjectedOverridesArgs): Override[] => {
  const overridesList: Override[] = [];
  const overrideIndexByID = new Map<string, number>();
  const dockerHostMap = new Map<string, Resource>();
  const dockerContainerMap = new Map<
    string,
    { host: Resource; container: Resource; containerShortId: string }
  >();
  const agentMap = new Map<string, Resource>();
  const guestMap = new Map<string, Resource>();

  const upsertProjectedOverride = (override: Override) => {
    const existingIndex = overrideIndexByID.get(override.id);
    if (existingIndex !== undefined) {
      overridesList[existingIndex] = override;
      return;
    }
    overrideIndexByID.set(override.id, overridesList.length);
    overridesList.push(override);
  };

  const storageCoords = (resource: Resource): { node: string; instance: string } => {
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

  containerRuntimeResources.forEach((host) => {
    dockerHostOverrideIdCandidates(host).forEach((id) => {
      dockerHostMap.set(id, host);
    });

    const containers = getChildren(host.id).filter((resource) => resource.type === 'app-container');

    containers.forEach((container) => {
      const shortId = container.id.includes('/') ? container.id.split('/').pop()! : container.id;
      dockerContainerOverrideIdCandidates(host, shortId).forEach((resourceId) => {
        dockerContainerMap.set(resourceId, { host, container, containerShortId: shortId });
      });
    });
  });

  agentResourceList.forEach((agentResource) => {
    hostOverrideIdCandidates(agentResource).forEach((id) => {
      agentMap.set(id, agentResource);
    });
  });

  [...vmResources, ...containerResources].forEach((guest) => {
    guestOverrideIdCandidates(guest).forEach((candidate) => {
      guestMap.set(candidate, guest);
    });
  });

  Object.entries(rawConfig).forEach(([key, thresholds]) => {
    const dockerHost = dockerHostMap.get(key);
    if (dockerHost) {
      upsertProjectedOverride({
        id: key,
        name: getAlertResourceDisplayLabel(dockerHost),
        type: 'dockerHost',
        resourceType: 'Container Runtime',
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const dockerContainer = dockerContainerMap.get(key);
    if (dockerContainer) {
      const { host, container, containerShortId } = dockerContainer;
      const containerName = getAlertResourceDisplayLabel(container, containerShortId);
      upsertProjectedOverride({
        id: key,
        name: containerName,
        type: 'dockerContainer',
        resourceType: 'Container',
        node: getAlertResourceDisplayLabel(host),
        instance: getAlertResourceDisplayLabel(host),
        disabled: thresholds.disabled || false,
        disableConnectivity: thresholds.disableConnectivity || false,
        poweredOffSeverity:
          thresholds.poweredOffSeverity === 'critical'
            ? 'critical'
            : thresholds.poweredOffSeverity === 'warning'
              ? 'warning'
              : undefined,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    if (key.startsWith('docker:')) {
      const [, rest] = key.split(':', 2);
      const [hostId, containerId] = (rest || '').split('/', 2);
      if (containerId) {
        upsertProjectedOverride({
          id: key,
          name: containerId,
          type: 'dockerContainer',
          resourceType: 'Container',
          node: hostId,
          disabled: thresholds.disabled || false,
          disableConnectivity: thresholds.disableConnectivity || false,
          poweredOffSeverity:
            thresholds.poweredOffSeverity === 'critical'
              ? 'critical'
              : thresholds.poweredOffSeverity === 'warning'
                ? 'warning'
                : undefined,
          thresholds: extractTriggerValues(thresholds),
        });
        return;
      }

      upsertProjectedOverride({
        id: key,
        name: hostId || key,
        type: 'dockerHost',
        resourceType: 'Container Runtime',
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const diskMatch = key.match(/^agent:(.+)\/disk:(.+)$/);
    if (diskMatch) {
      const [, agentId, diskLabel] = diskMatch;
      const agent = agentMap.get(agentId);
      upsertProjectedOverride({
        id: key,
        name: diskLabel.replace(/-/g, '/'),
        type: 'agentDisk',
        resourceType: 'Agent Disk',
        node: agent ? getAlertResourceDisplayLabel(agent) : agentId,
        disabled: thresholds.disabled || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const agentResource = agentMap.get(key);
    if (agentResource) {
      const displayName = getAlertResourceDisplayLabel(agentResource);
      const data = platformData(agentResource);
      const agent = asRecord(data?.agent);
      upsertProjectedOverride({
        id: key,
        name: displayName,
        type: 'agent',
        resourceType: 'Agent',
        node: displayName,
        instance:
          asString(agent?.platform) ||
          asString(agent?.osName) ||
          asString(data?.platform) ||
          asString(data?.osName) ||
          '',
        disabled: thresholds.disabled || false,
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    if (key.startsWith('pbs-')) {
      const pbs = pbsInstanceById.get(key);
      if (pbs) {
        upsertProjectedOverride({
          id: key,
          name: pbs.name,
          type: 'pbs',
          resourceType: 'PBS',
          disableConnectivity: thresholds.disableConnectivity || false,
          thresholds: extractTriggerValues(thresholds),
        });
      }
      return;
    }

    const node = nodeResources.find((resource) => resource.id === key);
    if (node) {
      upsertProjectedOverride({
        id: key,
        name: getAlertResourceDisplayLabel(node),
        type: 'agent',
        resourceType: 'Agent',
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const storage = storageResources.find((resource) => resource.id === key);
    if (storage) {
      const coords = storageCoords(storage);
      upsertProjectedOverride({
        id: key,
        name: getAlertResourceDisplayLabel(storage),
        type: 'storage',
        resourceType: 'Storage',
        node: coords.node,
        instance: coords.instance,
        disabled: thresholds.disabled || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const guest =
      guestMap.get(key) ||
      vmResources.find((resource) => resource.id === key) ||
      containerResources.find((resource) => resource.id === key);
    if (!guest) {
      return;
    }

    const guestIdentity = getGuestOverrideIdentity(guest);
    upsertProjectedOverride({
      id: guestOverrideStorageId(guest) || key,
      name: getAlertResourceDisplayLabel(guest),
      type: 'guest',
      resourceType: guest.type === 'vm' ? 'VM' : 'Container',
      vmid: guestIdentity?.vmid ?? guessNumericId(guest.id),
      node: guestIdentity?.node ?? '',
      instance: guestIdentity?.instance ?? guest.platformId,
      disabled: thresholds.disabled || false,
      disableConnectivity: thresholds.disableConnectivity || false,
      poweredOffSeverity:
        thresholds.poweredOffSeverity === 'critical'
          ? 'critical'
          : thresholds.poweredOffSeverity === 'warning'
            ? 'warning'
            : undefined,
      thresholds: extractTriggerValues(thresholds),
      backup: thresholds.backup,
      snapshot: thresholds.snapshot,
    });
  });

  return overridesList;
};
