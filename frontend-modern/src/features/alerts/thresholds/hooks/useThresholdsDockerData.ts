import { createMemo } from 'solid-js';

import { getPreferredResourceHostname } from '@/utils/resourceIdentity';
import type { Resource } from '@/types/resource';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';

import type { GroupHeaderMeta, Resource as TableResource } from '../tableTypes';
import { ThresholdsDataInputs } from '../thresholdsResourceModel';
import {
  createOverridesMap,
  dockerContainerOverrideIdCandidates,
  dockerHostOverrideIdCandidates,
  findOverrideByCandidates,
  getFriendlyAlertNodeName,
  getFriendlyNodeName,
  hasThresholdDiff,
  platformData,
} from '../thresholdsResourceModel';

export function useThresholdsDockerData(inputs: ThresholdsDataInputs) {
  const { props, editingId, searchTerm } = inputs;

  const dockerHostsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const seen = new Set<string>();

    const hosts: TableResource[] = (props.containerRuntimes ?? []).map((host) => {
      const idCandidates = dockerHostOverrideIdCandidates(host);
      const originalName = getAlertResourceDisplayLabel(host);
      const friendlyName = getFriendlyAlertNodeName(originalName, host.policy);
      const override = findOverrideByCandidates(overridesMap, idCandidates);
      const resourceId = override?.id || idCandidates[0] || host.id;
      const disableConnectivity = override?.disableConnectivity || false;

      seen.add(resourceId);

      return {
        id: resourceId,
        name: friendlyName,
        displayName: friendlyName,
        rawName: originalName,
        type: 'dockerHost' as const,
        resourceType: 'Container Runtime',
        node: getPreferredResourceHostname(host),
        instance: (platformData(host)?.platform as string) || (platformData(host)?.osName as string) || '',
        status: host.status,
        hasOverride: disableConnectivity,
        disableConnectivity,
        thresholds: override?.thresholds || {},
        defaults: {},
        editable: false,
      } satisfies TableResource;
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'dockerHost' && !seen.has(override.id))
      .forEach((override) => {
        const originalName = override.name || override.id;
        const friendlyName = getFriendlyNodeName(originalName);
        hosts.push({
          id: override.id,
          name: friendlyName,
          displayName: friendlyName,
          rawName: originalName,
          type: 'dockerHost',
          resourceType: 'Container Runtime',
          node: override.node || '',
          instance: override.instance || '',
          status: 'unknown',
          hasOverride: true,
          disableConnectivity: override.disableConnectivity || false,
          thresholds: override.thresholds || {},
          defaults: {},
          editable: false,
        });
      });

    return search ? hosts.filter((host) => host.name.toLowerCase().includes(search)) : hosts;
  }, []);

  const dockerContainersByHostId = createMemo(() => {
    const map = new Map<string, Resource[]>();
    (props.allResources ?? []).forEach((resource) => {
      if (resource.type !== 'app-container') return;
      const parentId = resource.parentId;
      if (!parentId) return;
      const existing = map.get(parentId);
      if (existing) {
        existing.push(resource);
      } else {
        map.set(parentId, [resource]);
      }
    });
    return map;
  });

  const dockerContainersGroupedByHost = createMemo<Record<string, TableResource[]>>((prev = {}) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const groups: Record<string, TableResource[]> = {};
    const seen = new Set<string>();

    (props.containerRuntimes ?? []).forEach((host) => {
      const dockerHostIds = dockerHostOverrideIdCandidates(host);
      const dockerHostIdForActions = dockerHostIds[0] || host.id;
      const hostLabel = getAlertResourceDisplayLabel(host);
      const friendlyHostName = getFriendlyAlertNodeName(hostLabel, host.policy);
      const hostLabelLower = hostLabel.toLowerCase();
      const friendlyHostNameLower = friendlyHostName.toLowerCase();
      const hostHostname = getPreferredResourceHostname(host);
      const containers = dockerContainersByHostId().get(host.id) ?? [];

      containers.forEach((container) => {
        const shortId = container.id.includes('/')
          ? (container.id.split('/').pop() ?? container.id)
          : container.id;
        const resourceIdCandidates = dockerContainerOverrideIdCandidates(host, shortId);
        const override = findOverrideByCandidates(overridesMap, resourceIdCandidates);
        const resourceId =
          override?.id || resourceIdCandidates[0] || `docker:${dockerHostIdForActions}/${shortId}`;
        const overrideSeverity = override?.poweredOffSeverity;
        const hasCustomThresholds = hasThresholdDiff(
          override,
          props.dockerDefaults as Record<string, number | undefined>,
        );
        const hasOverride =
          hasCustomThresholds ||
          Boolean(override?.disabled) ||
          Boolean(override?.disableConnectivity) ||
          overrideSeverity !== undefined;
        const containerName = getAlertResourceDisplayLabel(container, shortId);
        const image = (platformData(container)?.image as string) ?? '';

        const matchesSearch =
          !search ||
          containerName.toLowerCase().includes(search) ||
          hostLabelLower.includes(search) ||
          friendlyHostNameLower.includes(search) ||
          image.toLowerCase().includes(search);
        if (!matchesSearch) return;

        const groupKey = friendlyHostName || hostLabel;
        const resource: TableResource = {
          id: resourceId,
          name: containerName,
          type: 'dockerContainer',
          resourceType: 'Container',
          node: groupKey,
          instance: hostHostname,
          status: container.status,
          hasOverride,
          disabled: override?.disabled || false,
          disableConnectivity: override?.disableConnectivity || false,
          thresholds: override?.thresholds || {},
          defaults: props.dockerDefaults,
          hostId: dockerHostIdForActions,
          image,
          poweredOffSeverity: overrideSeverity,
        };

        if (!groups[groupKey]) {
          groups[groupKey] = [];
        }
        groups[groupKey].push(resource);
        seen.add(resourceId);
      });
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'dockerContainer' && !seen.has(override.id))
      .forEach((override) => {
        const fallbackName = override.name || override.id.split('/').pop() || override.id;
        const group = 'Unassigned Containers';
        if (!groups[group]) {
          groups[group] = [];
        }
        groups[group].push({
          id: override.id,
          name: fallbackName,
          type: 'dockerContainer',
          resourceType: 'Container',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          disableConnectivity: override.disableConnectivity || false,
          thresholds: override.thresholds || {},
          defaults: props.dockerDefaults,
          poweredOffSeverity: override.poweredOffSeverity,
        });
      });

    Object.keys(groups).forEach((group) => {
      groups[group].sort((a, b) => a.name.localeCompare(b.name));
    });

    if (!search) {
      return groups;
    }

    const filteredGroups: Record<string, TableResource[]> = {};
    Object.entries(groups).forEach(([group, resources]) => {
      if (resources.length > 0) {
        filteredGroups[group] = resources;
      }
    });
    return filteredGroups;
  }, {});

  const dockerContainersFlat = createMemo<TableResource[]>(() =>
    Object.values(dockerContainersGroupedByHost() ?? {}).flat(),
  );

  const totalDockerContainers = createMemo(() =>
    (props.containerRuntimes ?? []).reduce(
      (sum, host) => sum + (dockerContainersByHostId().get(host.id)?.length ?? 0),
      0,
    ),
  );

  const dockerHostGroupMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.containerRuntimes ?? []).forEach((host) => {
      const originalName = getAlertResourceDisplayLabel(host);
      const friendlyName = getFriendlyAlertNodeName(originalName, host.policy);
      const headerMeta: GroupHeaderMeta = {
        displayName: friendlyName,
        rawName: originalName,
        status: host.status,
      };

      const hostname = getPreferredResourceHostname(host);
      [friendlyName, originalName, hostname, host.id]
        .filter((key: string | undefined): key is string => Boolean(key && key.trim()))
        .forEach((key: string) => {
          meta[key.trim()] = headerMeta;
        });
    });

    meta['Unassigned Containers'] = {
      displayName: 'Unassigned Containers',
      status: 'unknown',
    };

    return meta;
  });

  return {
    dockerHostsWithOverrides,
    dockerContainersByHostId,
    dockerContainersGroupedByHost,
    dockerContainersFlat,
    totalDockerContainers,
    dockerHostGroupMeta,
  };
}
