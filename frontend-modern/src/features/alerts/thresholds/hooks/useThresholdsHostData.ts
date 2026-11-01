import { createMemo } from 'solid-js';

import {
  parseAgentSensorResourceId,
  agentSensorResourceId,
} from '@/features/alerts/agentSensorIdentity';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';
import { formatTemperature } from '@/utils/temperature';

import type { GroupHeaderMeta, Resource as TableResource } from '../tableTypes';
import { ThresholdsDataInputs } from '../thresholdsResourceModel';
import {
  agentAlertResourceId,
  agentDiskResourceId,
  buildAgentHeaderMeta,
  createOverridesMap,
  findOverrideByCandidates,
  getFriendlyAlertNodeName,
  hasThresholdDiff,
  hostActionId,
  hostOverrideIdCandidates,
  matchesThresholdResourceSearch,
  platformData,
  readRecord,
  readString,
  uniqueIds,
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
      const friendlyName = getFriendlyAlertNodeName(originalDisplayName, clusterName);
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
        // The node's own alerts are raised on its Proxmox node ID. A node merged
        // with its Pulse agent hands CPU, memory and disk to the agent, whose
        // alerts its Machines row shows against the agent's limits.
        alertResourceIds: uniqueIds(node.proxmox?.sourceId, node.id),
        name: sanitizedName,
        displayName: sanitizedName,
        rawName: originalDisplayName,
        host: normalizedHost,
        type: 'agent' as const,
        resourceType: 'Virtualization Host',
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

    return search ? nodes.filter((node) => matchesThresholdResourceSearch(node, search)) : nodes;
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
        alertResourceIds: [
          agentAlertResourceId(
            readString(agentResource.agent?.agentId) || hostActionId(agentResource),
          ),
        ],
        name: displayName,
        displayName,
        rawName: agentResource.identity?.hostname ?? agentResource.name,
        type: 'agent' as const,
        resourceType: 'System',
        node: displayName,
        instance:
          readString(agentData?.platform) ||
          readString(agentData?.osName) ||
          readString(data?.platform) ||
          readString(data?.osName) ||
          '',
        status: agentResource.status,
        hasOverride:
          hasCustomThresholds ||
          Boolean(override?.disabled) ||
          Boolean(override?.disableConnectivity),
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
          resourceType: 'System',
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

    return search
      ? agents.filter((agent) => matchesThresholdResourceSearch(agent, search))
      : agents;
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
      const disksFromPlatformAgent = Array.isArray(platformAgent?.disks)
        ? platformAgent.disks
        : null;
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
          resourceType: 'System Disk',
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
          resourceType: 'System Disk',
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
          (disk) =>
            disk.name.toLowerCase().includes(search) || disk.node?.toLowerCase().includes(search),
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

  // One row per usable temperature sensor a machine reports, plus every sensor
  // that has saved alert settings, so a configured sensor stays editable and
  // removable while it reads invalid, has gone missing or its machine is
  // offline. Sensors have no global default: the presentation default of 0
  // makes an unconfigured row read Off, which is what the alert engine does.
  const agentSensorsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    // Configured sensor IDs parsed once, by host, so each machine reads only
    // its own entries.
    const configuredSensorIdsByHost = new Map<string, string[]>();
    (props.overrides() ?? []).forEach((override) => {
      if (override.type !== 'agentSensor') return;
      const parsed = parseAgentSensorResourceId(override.id);
      if (!parsed) return;
      const ids = configuredSensorIdsByHost.get(parsed.hostId) ?? [];
      ids.push(parsed.sensorId);
      configuredSensorIdsByHost.set(parsed.hostId, ids);
    });
    const seen = new Set<string>();
    const sensors: TableResource[] = [];

    (props.agents ?? []).forEach((agentResource) => {
      const hostId = readString(agentResource.agent?.agentId) || hostActionId(agentResource);
      const agentDisplayName = getAlertResourceDisplayLabel(agentResource);
      const hostReporting =
        agentResource.status !== 'offline' && agentResource.agent?.stale !== true;
      const reported = (agentResource.agent?.sensors?.native ?? []).filter(
        (sensor) => sensor.kind === 'temperature',
      );
      const reportedById = new Map(reported.map((sensor) => [sensor.id, sensor]));
      const labelCounts = new Map<string, number>();
      reported.forEach((sensor) => {
        const labelKey = `${sensor.chip}\u0000${sensor.label}`;
        labelCounts.set(labelKey, (labelCounts.get(labelKey) ?? 0) + 1);
      });

      const sensorIds = new Set(
        reported.filter((sensor) => sensor.status === 'ok').map((sensor) => sensor.id),
      );
      configuredSensorIdsByHost.get(hostId)?.forEach((sensorId) => sensorIds.add(sensorId));

      sensorIds.forEach((sensorId) => {
        const resourceId = agentSensorResourceId(hostId, sensorId);
        const override = overridesMap.get(resourceId);
        const sensor = reportedById.get(sensorId);
        const label = sensor?.label.trim() || sensorId;
        const name =
          sensor && (labelCounts.get(`${sensor.chip}\u0000${sensor.label}`) ?? 0) > 1
            ? `${label} (${sensor.channel})`
            : label;
        const chip = sensor?.chip || sensorId.split('/')[0] || '';
        let readingMessage: string | undefined;
        let reading: string | undefined;
        if (!sensor) {
          readingMessage = 'Not reported by this machine. Saved alert settings are retained.';
        } else if (!hostReporting) {
          readingMessage = 'Machine is not reporting, so there is no current reading.';
        } else if (sensor.status !== 'ok' || typeof sensor.value !== 'number') {
          readingMessage = 'No valid reading from this sensor.';
        } else {
          reading = formatTemperature(sensor.value, { decimals: 1 });
        }

        seen.add(resourceId);
        sensors.push({
          id: resourceId,
          name,
          displayName: name,
          rawName: sensorId,
          type: 'agentSensor' as const,
          resourceType: 'Machine Sensor',
          groupKey: agentResource.id,
          node: agentDisplayName,
          instance: chip,
          status: agentResource.status,
          // The default is presentation-only, so any stored entry is an
          // override: a saved 0 stays visible and removable.
          hasOverride: Boolean(override),
          note: override?.note,
          disabled: override?.disabled || false,
          thresholds: override?.thresholds || {},
          defaults: { temperature: 0 },
          presentationDefaults: true,
          subtitle: reading ? `${reading} · ${chip}` : chip,
          readingMessage,
        } satisfies TableResource);
      });
    });

    (props.overrides() ?? [])
      .filter((override) => override.type === 'agentSensor' && !seen.has(override.id))
      .forEach((override) => {
        const parsed = parseAgentSensorResourceId(override.id);
        const name = parsed?.sensorId || override.name || override.id;
        sensors.push({
          id: override.id,
          name,
          displayName: name,
          rawName: name,
          type: 'agentSensor' as const,
          resourceType: 'Machine Sensor',
          groupKey: parsed?.hostId || override.node || 'Unknown Machine',
          node: override.node || parsed?.hostId || 'Unknown Machine',
          instance: '',
          status: 'unknown',
          hasOverride: true,
          disabled: override.disabled || false,
          thresholds: override.thresholds || {},
          defaults: { temperature: 0 },
          presentationDefaults: true,
          note: override.note,
          readingMessage: 'Machine not found. Saved alert settings are retained.',
        } satisfies TableResource);
      });

    return search
      ? sensors.filter(
          (sensor) =>
            sensor.name.toLowerCase().includes(search) ||
            sensor.rawName?.toLowerCase().includes(search) ||
            sensor.node?.toLowerCase().includes(search),
        )
      : sensors;
  }, []);

  // Grouped by the machine's resource ID, not its display name, so two
  // machines with the same name keep separate sensor lists.
  const agentSensorsGroupedByAgent = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    agentSensorsWithOverrides().forEach((sensor) => {
      const key =
        typeof sensor.groupKey === 'string' && sensor.groupKey
          ? sensor.groupKey
          : 'Unknown Machine';
      (grouped[key] ??= []).push(sensor);
    });
    Object.values(grouped).forEach((resources) => {
      resources.sort((a, b) => a.name.localeCompare(b.name));
    });
    return grouped;
  });

  const agentGroupHeaderMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.agents ?? []).forEach((agent) => {
      const { headerMeta, keys } = buildAgentHeaderMeta(agent);
      keys.forEach((key: string) => {
        meta[key] = headerMeta;
      });
    });
    return meta;
  });

  return {
    nodesWithOverrides,
    agentsWithOverrides,
    agentDisksWithOverrides,
    agentDisksGroupedByAgent,
    agentSensorsWithOverrides,
    agentSensorsGroupedByAgent,
    agentGroupHeaderMeta,
  };
}
