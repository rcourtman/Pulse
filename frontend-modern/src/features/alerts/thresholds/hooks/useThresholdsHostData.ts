import { createMemo } from 'solid-js';

import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';

import type { Resource as TableResource } from '../tableTypes';
import { ThresholdsDataInputs } from '../thresholdsResourceModel';
import {
  agentDiskResourceId,
  createOverridesMap,
  findOverrideByCandidates,
  getFriendlyAlertNodeName,
  hasThresholdDiff,
  hostActionId,
  hostOverrideIdCandidates,
  platformData,
  readRecord,
  readString,
} from '../thresholdsResourceModel';

export function useThresholdsHostData(inputs: ThresholdsDataInputs) {
  const { props, editingId, searchTerm } = inputs;

  const nodesWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());

    const nodes = (props.nodes ?? []).map((node) => {
      const override = overridesMap.get(node.id);
      const data = platformData(node);
      const clusterName = (data?.clusterName as string | undefined) ?? undefined;
      const isClusterMember =
        (data?.isClusterMember as boolean | undefined) ?? Boolean(node.clusterId);
      const hasCustomThresholds = hasThresholdDiff(
        override,
        props.nodeDefaults as Record<string, number | undefined>,
      );
      const note = typeof override?.note === 'string' ? override.note : undefined;
      const hasNote = Boolean(note && note.trim().length > 0);

      const originalDisplayName = getAlertResourceDisplayLabel(node);
      const friendlyName = getFriendlyAlertNodeName(originalDisplayName, node.policy, clusterName);
      const rawName = node.name;
      const sanitizedName = friendlyName || originalDisplayName || rawName.split('.')[0] || rawName;
      const guestUrlValue = typeof data?.guestURL === 'string' ? data.guestURL.trim() : '';
      const hostValue = (typeof data?.host === 'string' ? data.host.trim() : '') || rawName;
      const normalizedHost =
        guestUrlValue && guestUrlValue !== ''
          ? guestUrlValue.startsWith('http')
            ? guestUrlValue
            : `https://${guestUrlValue}`
          : hostValue.startsWith('http://') || hostValue.startsWith('https://')
            ? hostValue
            : `https://${hostValue.includes(':') ? hostValue : `${hostValue}:8006`}`;

      return {
        id: node.id,
        name: sanitizedName,
        displayName: sanitizedName,
        rawName: originalDisplayName,
        host: normalizedHost,
        type: 'agent' as const,
        resourceType: 'Agent',
        status: node.status,
        uptime: node.uptime,
        cpu: (node.cpu?.current ?? 0) / 100,
        memory: node.memory?.current,
        hasOverride: hasCustomThresholds || hasNote || Boolean(override?.disableConnectivity),
        disabled: false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.nodeDefaults,
        clusterName: isClusterMember ? clusterName?.trim() : undefined,
        isClusterMember,
        instance: node.platformId,
        note,
      } satisfies TableResource;
    });

    return search ? nodes.filter((node) => node.name.toLowerCase().includes(search)) : nodes;
  }, []);

  const agentsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const seen = new Set<string>();

    const agents: TableResource[] = (props.agents ?? []).map((agentResource) => {
      const idCandidates = hostOverrideIdCandidates(agentResource);
      const override = findOverrideByCandidates(overridesMap, idCandidates);
      const resourceId = override?.id || idCandidates[0] || agentResource.id;
      const hasCustomThresholds = hasThresholdDiff(
        override,
        props.agentDefaults as Record<string, number | undefined>,
      );
      const displayName = getAlertResourceDisplayLabel(agentResource);
      const data = platformData(agentResource);
      const agentData = readRecord(data?.agent);

      seen.add(resourceId);

      return {
        id: resourceId,
        name: displayName,
        displayName,
        rawName: agentResource.identity?.hostname ?? agentResource.name,
        type: 'agent' as const,
        resourceType: 'Agent',
        node: displayName,
        instance:
          readString(agentData?.platform) ||
          readString(agentData?.osName) ||
          readString(data?.platform) ||
          readString(data?.osName) ||
          '',
        status: agentResource.status,
        hasOverride:
          hasCustomThresholds || Boolean(override?.disabled) || Boolean(override?.disableConnectivity),
        disabled: override?.disabled || false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.agentDefaults,
      } satisfies TableResource;
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'agent' && !seen.has(override.id))
      .forEach((override) => {
        const name = override.name?.trim() || override.id;
        agents.push({
          id: override.id,
          name,
          displayName: name,
          rawName: name,
          type: 'agent' as const,
          resourceType: 'Agent',
          node: '',
          instance: '',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          disableConnectivity: override.disableConnectivity || false,
          thresholds: override.thresholds || {},
          defaults: props.agentDefaults,
        } satisfies TableResource);
      });

    return search ? agents.filter((agent) => agent.name.toLowerCase().includes(search)) : agents;
  }, []);

  const agentDisksWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const seen = new Set<string>();
    const disks: TableResource[] = [];

    (props.agents ?? []).forEach((agentResource) => {
      const agentDisplayName = getAlertResourceDisplayLabel(agentResource);
      const agentIdCandidates = hostOverrideIdCandidates(agentResource);
      const agentIdForActions = hostActionId(agentResource);
      const data = platformData(agentResource);
      const platformAgent = readRecord(data?.agent);
      const disksFromPlatformRoot = Array.isArray(data?.disks) ? data.disks : null;
      const disksFromPlatformAgent = Array.isArray(platformAgent?.disks) ? platformAgent.disks : null;
      const disksFromResourceAgent = Array.isArray(agentResource.agent?.disks)
        ? agentResource.agent.disks
        : null;
      const disksForAgent = (disksFromPlatformRoot ||
        disksFromPlatformAgent ||
        disksFromResourceAgent ||
        []) as Array<{
        mountpoint?: string;
        device?: string;
        used?: number;
        total?: number;
        type?: string;
      }>;

      disksForAgent.forEach((disk) => {
        const diskLabel = disk.mountpoint?.trim() || disk.device?.trim() || 'disk';
        const resourceIdCandidates = agentIdCandidates.map((agentId) =>
          agentDiskResourceId(agentId, disk.mountpoint || '', disk.device),
        );
        const override = findOverrideByCandidates(overridesMap, resourceIdCandidates);
        const resourceId = override?.id || resourceIdCandidates[0];
        if (!resourceId) return;

        const hasCustomThresholds = hasThresholdDiff(override, {
          disk: props.agentDefaults.disk,
        });

        seen.add(resourceId);

        disks.push({
          id: resourceId,
          name: diskLabel,
          displayName: diskLabel,
          rawName: disk.device || diskLabel,
          type: 'agentDisk' as const,
          resourceType: 'Agent Disk',
          host: agentIdForActions,
          node: agentDisplayName,
          instance: disk.type || '',
          status: agentResource.status,
          hasOverride: hasCustomThresholds || Boolean(override?.disabled),
          disabled: override?.disabled || false,
          thresholds: override?.thresholds || {},
          defaults: { disk: props.agentDefaults.disk },
          subtitle: `${((disk.used || 0) / 1024 / 1024 / 1024).toFixed(1)} / ${((disk.total || 0) / 1024 / 1024 / 1024).toFixed(1)} GB`,
        } satisfies TableResource);
      });
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'agentDisk' && !seen.has(override.id))
      .forEach((override) => {
        const name = override.name || override.id;
        disks.push({
          id: override.id,
          name,
          displayName: name,
          rawName: name,
          type: 'agentDisk' as const,
          resourceType: 'Agent Disk',
          host: '',
          node: 'Unknown Agent',
          instance: '',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          thresholds: override.thresholds || {},
          defaults: { disk: props.agentDefaults.disk },
        });
      });

    return search
      ? disks.filter(
          (disk) => disk.name.toLowerCase().includes(search) || disk.node?.toLowerCase().includes(search),
        )
      : disks;
  }, []);

  const agentDisksGroupedByAgent = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    agentDisksWithOverrides().forEach((disk) => {
      const key = disk.node?.trim() || 'Unknown Agent';
      if (!grouped[key]) {
        grouped[key] = [];
      }
      grouped[key].push(disk);
    });

    Object.values(grouped).forEach((resources) => {
      resources.sort((a, b) => a.name.localeCompare(b.name));
    });

    return grouped;
  });

  return {
    nodesWithOverrides,
    agentsWithOverrides,
    agentDisksWithOverrides,
    agentDisksGroupedByAgent,
  };
}
