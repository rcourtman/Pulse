import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import type { PBSInstance, PMGInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';
import { getActionableAgentIdFromResource, hasAgentFacet } from '@/utils/agentResources';
import { isAppContainerDiscoveryResourceType } from '@/utils/discoveryTarget';
import { pbsInstanceFromResource, pmgInstanceFromResource } from '@/utils/resourceStateAdapters';

import {
  extractTriggerValues,
  getAlertResourceDisplayLabel,
  guessNumericId,
  platformData,
} from './helpers';
import type { Override } from './types';

export interface AlertOverridesStateProps {
  allResources: Accessor<Resource[]>;
  byType: (resourceType: ResourceType) => Resource[];
  children: (resourceId: string) => Resource[];
  hasUnsavedChanges: Accessor<boolean>;
  setOverviewOverrides: (value: Override[]) => void;
}

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

const normalizeRawOverridesConfig = (
  rawOverrides: Record<string, RawOverrideConfig>,
): Record<string, RawOverrideConfig> => {
  const cleanedOverrides: Record<string, RawOverrideConfig> = {};

  for (const [key, value] of Object.entries(rawOverrides)) {
    const diskMatch = key.match(/^(agent:.+\/disk:)(.+)$/);
    if (diskMatch) {
      const normalized =
        diskMatch[2]
          .toLowerCase()
          .replace(/[^a-z0-9]/g, '-')
          .replace(/-{2,}/g, '-')
          .replace(/^-|-$/g, '') || 'unknown';
      cleanedOverrides[diskMatch[1] + normalized] = value;
      continue;
    }

    cleanedOverrides[key] = value;
  }

  return cleanedOverrides;
};

export function useAlertOverridesState(props: AlertOverridesStateProps) {
  const [overrides, setOverrides] = createSignal<Override[]>([]);
  const [rawOverridesConfig, setRawOverridesConfig] = createSignal<
    Record<string, RawOverrideConfig>
  >({});

  const pd = platformData;

  const hostOverrideIdCandidates = (resource: Resource): string[] => {
    const data = pd(resource);
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

  const dockerHostOverrideIdCandidates = (resource: Resource): string[] => {
    const data = pd(resource);
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

  const dockerContainerOverrideIdCandidates = (host: Resource, shortId: string): string[] =>
    uniqueIds(
      ...dockerHostOverrideIdCandidates(host).map((hostId) => `docker:${hostId}/${shortId}`),
    );

  const allGuests = createMemo(
    () => [
      ...props.byType('vm'),
      ...props.byType('system-container'),
      ...props.byType('oci-container'),
    ],
    [],
    {
      equals: (prev, next) => {
        if (prev.length !== next.length) return false;
        return prev.every(
          (current, index) => current.id === next[index].id && current.name === next[index].name,
        );
      },
    },
  );

  const agentResources = createMemo(() =>
    props.allResources().filter(
      (resource) =>
        (resource.type === 'agent' ||
          resource.type === 'pbs' ||
          resource.type === 'pmg' ||
          resource.type === 'truenas') &&
        hasAgentFacet(resource),
    ),
  );

  const pbsInstances = createMemo<PBSInstance[]>(() =>
    props
      .allResources()
      .filter((resource) => resource.type === 'pbs')
      .map(pbsInstanceFromResource)
      .filter((resource): resource is PBSInstance => Boolean(resource)),
  );

  const pbsInstanceById = createMemo(
    () => new Map(pbsInstances().map((instance) => [instance.id, instance])),
  );

  const pmgInstances = createMemo<PMGInstance[]>(() =>
    props
      .allResources()
      .filter((resource) => resource.type === 'pmg')
      .map(pmgInstanceFromResource)
      .filter((resource): resource is PMGInstance => Boolean(resource)),
  );

  createEffect(() => {
    if (props.hasUnsavedChanges()) {
      return;
    }

    const rawConfig = rawOverridesConfig();
    if (Object.keys(rawConfig).length === 0) {
      if (overrides().length > 0) {
        setOverrides([]);
      }
      return;
    }

    const nodeResources = props.byType('agent');
    const vmResources = props.byType('vm');
    const containerResources = [
      ...props.byType('system-container'),
      ...props.byType('oci-container'),
    ];
    const storageResources = props
      .allResources()
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore');
    const agentResourceList = agentResources();
    const dockerHostResources = props.byType('docker-host');
    const overridesList: Override[] = [];
    const dockerHostMap = new Map<string, Resource>();
    const dockerContainerMap = new Map<
      string,
      { host: Resource; container: Resource; containerShortId: string }
    >();
    const agentMap = new Map<string, Resource>();

    const storageCoords = (resource: Resource): { node: string; instance: string } => {
      const data = pd(resource);
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

    dockerHostResources.forEach((host) => {
      dockerHostOverrideIdCandidates(host).forEach((id) => {
        dockerHostMap.set(id, host);
      });

      const containers = props
        .children(host.id)
        .filter((resource) => resource.type === 'app-container');

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

    Object.entries(rawConfig).forEach(([key, thresholds]) => {
      const dockerHost = dockerHostMap.get(key);
      if (dockerHost) {
        overridesList.push({
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
        overridesList.push({
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
          overridesList.push({
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

        overridesList.push({
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
        overridesList.push({
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
        const data = pd(agentResource);
        const agent = asRecord(data?.agent);
        overridesList.push({
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
        const pbs = pbsInstanceById().get(key);
        if (pbs) {
          overridesList.push({
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
        overridesList.push({
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
        overridesList.push({
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
        vmResources.find((resource) => resource.id === key) ||
        containerResources.find((resource) => resource.id === key);
      if (!guest) {
        return;
      }

      const data = pd(guest);
      overridesList.push({
        id: key,
        name: getAlertResourceDisplayLabel(guest),
        type: 'guest',
        resourceType: guest.type === 'vm' ? 'VM' : 'Container',
        vmid: (data?.vmid as number | undefined) ?? guessNumericId(guest.id),
        node: (data?.node as string | undefined) ?? '',
        instance: (data?.instance as string | undefined) ?? guest.platformId,
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

    const currentOverrides = overrides();
    const hasChanged =
      overridesList.length !== currentOverrides.length ||
      overridesList.some((newOverride) => {
        const existing = currentOverrides.find((override) => override.id === newOverride.id);
        if (!existing) return true;
        return (
          JSON.stringify(newOverride.thresholds) !== JSON.stringify(existing.thresholds) ||
          Boolean(newOverride.disableConnectivity) !== Boolean(existing.disableConnectivity) ||
          Boolean(newOverride.disabled) !== Boolean(existing.disabled) ||
          (newOverride.poweredOffSeverity ?? null) !== (existing.poweredOffSeverity ?? null) ||
          JSON.stringify(newOverride.backup ?? null) !== JSON.stringify(existing.backup ?? null) ||
          JSON.stringify(newOverride.snapshot ?? null) !==
            JSON.stringify(existing.snapshot ?? null)
        );
      });

    if (hasChanged) {
      setOverrides(overridesList);
    }
  });

  createEffect(() => {
    props.setOverviewOverrides(overrides());
  });

  const replaceRawOverridesConfig = (value: Record<string, RawOverrideConfig>) => {
    setRawOverridesConfig(normalizeRawOverridesConfig(value));
  };

  return {
    overrides,
    setOverrides,
    rawOverridesConfig,
    setRawOverridesConfig,
    replaceRawOverridesConfig,
    allGuests,
    agentResources,
    pbsInstances,
    pmgInstances,
  };
}
