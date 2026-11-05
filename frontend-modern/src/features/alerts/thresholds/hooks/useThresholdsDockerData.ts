import { createMemo } from 'solid-js';

import { getPreferredResourceHostname } from '@/utils/resourceIdentity';
import type { Resource } from '@/types/resource';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';

import type { GroupHeaderMeta, Resource as TableResource } from '../tableTypes';
import { ThresholdsDataInputs } from '../thresholdsResourceModel';
import {
  buildThresholdsDockerHostLabels,
  createOverridesMap,
  dockerContainerOverrideIdCandidates,
  dockerHostOverrideIdCandidates,
  findOverrideByCandidates,
  getFriendlyDockerHostName,
  hasThresholdDiff,
  matchesThresholdResourceSearch,
  platformData,
} from '../thresholdsResourceModel';

const UNASSIGNED_CONTAINERS = 'Unassigned Containers';

export function useThresholdsDockerData(inputs: ThresholdsDataInputs) {
  const { props, editingId, searchTerm } = inputs;

  const dockerHostLabels = createMemo(() =>
    buildThresholdsDockerHostLabels(props.containerRuntimes ?? []),
  );

  const dockerHostsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const seen = new Set<string>();

    const hosts: TableResource[] = (props.containerRuntimes ?? []).map((host) => {
      const idCandidates = dockerHostOverrideIdCandidates(host);
      const label = dockerHostLabels()(host);
      const override = findOverrideByCandidates(overridesMap, idCandidates);
      const resourceId = override?.id || idCandidates[0] || host.id;
      const disableConnectivity = override?.disableConnectivity || false;

      seen.add(resourceId);

      return {
        id: resourceId,
        name: label.name,
        displayName: label.name,
        rawName: label.rawName,
        qualifier: label.qualifier,
        type: 'dockerHost' as const,
        resourceType: 'Container Runtime',
        node: getPreferredResourceHostname(host),
        instance:
          (platformData(host)?.platform as string) || (platformData(host)?.osName as string) || '',
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
        const friendlyName = getFriendlyDockerHostName(originalName);
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

    return search ? hosts.filter((host) => matchesThresholdResourceSearch(host, search)) : hosts;
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
      const hostLabel = dockerHostLabels()(host);
      const hostHostname = getPreferredResourceHostname(host);
      const containers = dockerContainersByHostId().get(host.id) ?? [];

      containers.forEach((container) => {
        const containerId = container.docker?.containerId?.trim();
        const shortId = containerId ? containerId.slice(0, 12) : container.id;
        const resourceIdCandidates = dockerContainerOverrideIdCandidates(host, container);
        const override = findOverrideByCandidates(overridesMap, resourceIdCandidates);
        // The row keys on the stable name-based candidate so it survives
        // container recreates. Every candidate is marked seen, even for a row
        // the search hides: its override belongs to a live container, whichever
        // ID it is stored under, and must not surface as Unassigned Containers.
        const resourceId = resourceIdCandidates[0] || `docker:${dockerHostIdForActions}/${shortId}`;
        resourceIdCandidates.forEach((candidate) => seen.add(candidate));
        seen.add(resourceId);
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
          hostLabel.searchText.includes(search) ||
          image.toLowerCase().includes(search);
        if (!matchesSearch) return;

        const groupKey = hostLabel.groupKey;
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
          overrideIdCandidates: resourceIdCandidates,
          overrideStorageId: resourceId,
        };

        if (!groups[groupKey]) {
          groups[groupKey] = [];
        }
        groups[groupKey].push(resource);
      });
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'dockerContainer' && !seen.has(override.id))
      .forEach((override) => {
        const fallbackName = override.name || override.id.split('/').pop() || override.id;
        // Like a live row, it matches by its own name or its group's.
        const matchesSearch =
          !search ||
          fallbackName.toLowerCase().includes(search) ||
          UNASSIGNED_CONTAINERS.toLowerCase().includes(search);
        if (!matchesSearch) return;

        if (!groups[UNASSIGNED_CONTAINERS]) {
          groups[UNASSIGNED_CONTAINERS] = [];
        }
        groups[UNASSIGNED_CONTAINERS].push({
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

    return groups;
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
      const label = dockerHostLabels()(host);
      meta[label.groupKey] = {
        displayName: label.name,
        rawName: label.rawName,
        qualifier: label.qualifier,
        status: host.status,
      };
    });

    meta[UNASSIGNED_CONTAINERS] = {
      displayName: UNASSIGNED_CONTAINERS,
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
