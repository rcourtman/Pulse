import { createMemo } from 'solid-js';
import { unwrap } from 'solid-js/store';
import type { Resource } from '@/types/resource';
import type { GroupHeaderMeta } from '@/components/Alerts/ResourceTable';
import type { Resource as TableResource } from '@/components/Alerts/ResourceTable';
import {
  PMG_THRESHOLD_COLUMNS,
  PMG_KEY_TO_NORMALIZED,
  PMG_NORMALIZED_TO_KEY,
  DEFAULT_SNAPSHOT_WARNING,
  DEFAULT_SNAPSHOT_CRITICAL,
  DEFAULT_SNAPSHOT_WARNING_SIZE,
  DEFAULT_SNAPSHOT_CRITICAL_SIZE,
  DEFAULT_BACKUP_WARNING,
  DEFAULT_BACKUP_CRITICAL,
  DEFAULT_BACKUP_FRESH_HOURS,
  DEFAULT_BACKUP_STALE_HOURS,
} from '../constants';
import type { PMGThresholdDefaults, SnapshotAlertConfig, BackupAlertConfig } from '@/types/alerts';
import type { ThresholdsTableProps, Override } from '../types';

export function useThresholdsData(
  props: ThresholdsTableProps,
  editingId: () => string | null,
  searchTerm: () => string,
  pd: (r: Resource) => Record<string, unknown> | undefined,
  buildNodeHeaderMeta: (node: Resource) => { headerMeta: GroupHeaderMeta; keys: Set<string> },
  getFriendlyNodeName: (value: string, clusterName?: string) => string,
) {
  // Passed-in blocks:
  const nodesWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));

    const nodes = (props.nodes ?? []).map((node) => {
      const override = overridesMap.get(node.id);
      const data = pd(node);
      const clusterName = (data?.clusterName as string | undefined) ?? undefined;
      const isClusterMember =
        (data?.isClusterMember as boolean | undefined) ?? Boolean(node.clusterId);

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        (override as Override | undefined)?.thresholds &&
        Object.keys((override as Override).thresholds).some((key: string) => {
          const k = key as keyof Override['thresholds'];
          return (
            (override as Override).thresholds[k] !== undefined &&
            (override as Override).thresholds[k] !== (props.nodeDefaults as any)[k]
          );
        });

      const note =
        typeof (override as Override | undefined)?.note === 'string'
          ? (override as Override).note
          : undefined;
      const hasNote = Boolean(note && note.trim().length > 0);

      const originalDisplayName = node.displayName?.trim() || node.name;
      const friendlyName = getFriendlyNodeName(originalDisplayName, clusterName);
      const rawName = node.name;
      const sanitizedName = friendlyName || originalDisplayName || rawName.split('.')[0] || rawName;
      // Build a best-effort management URL for the node
      // Prioritize guestURL over host (same as NodeGroupHeader)
      const guestUrlValue =
        typeof data?.guestURL === 'string' ? (data.guestURL as string).trim() : '';
      const hostValue =
        (typeof data?.host === 'string' ? (data.host as string).trim() : '') || rawName;
      let normalizedHost: string;
      if (guestUrlValue && guestUrlValue !== '') {
        normalizedHost = guestUrlValue.startsWith('http')
          ? guestUrlValue
          : `https://${guestUrlValue}`;
      } else {
        normalizedHost =
          hostValue.startsWith('http://') || hostValue.startsWith('https://')
            ? hostValue
            : `https://${hostValue.includes(':') ? hostValue : `${hostValue}:8006`}`;
      }

      return {
        id: node.id,
        name: sanitizedName,
        displayName: sanitizedName,
        rawName: originalDisplayName,
        host: normalizedHost,
        type: 'node' as const,
        resourceType: 'Node',
        status: node.status,
        uptime: node.uptime,
        cpu: (node.cpu?.current ?? 0) / 100,
        memory: node.memory?.current,
        hasOverride:
          hasCustomThresholds ||
          hasNote ||
          Boolean((override as Override | undefined)?.disableConnectivity) ||
          false,
        disabled: false,
        disableConnectivity: (override as Override | undefined)?.disableConnectivity || false,
        thresholds: (override as Override | undefined)?.thresholds || {},
        defaults: props.nodeDefaults,
        clusterName: isClusterMember ? clusterName?.trim() : undefined,
        isClusterMember,
        instance: node.platformId,
        note,
      } satisfies TableResource;
    });

    if (search) {
      return nodes.filter((n) => n.name.toLowerCase().includes(search));
    }
    return nodes;
  }, []);

  const hostAgentsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));
    const seen = new Set<string>();

    const hosts: TableResource[] = (props.hosts ?? []).map((host) => {
      const override = overridesMap.get(host.id);
      const hasCustomThresholds =
        (override as Override | undefined)?.thresholds &&
        Object.keys((override as Override).thresholds).some((key: string) => {
          const k = key as keyof Override['thresholds'];
          return (
            (override as Override).thresholds[k] !== undefined &&
            (override as Override).thresholds[k] !== (props.hostDefaults as any)[k]
          );
        });

      const displayName =
        host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const status = host.status;

      seen.add(host.id);

      return {
        id: host.id,
        name: displayName,
        displayName,
        rawName: host.identity?.hostname ?? host.name,
        type: 'hostAgent' as const,
        resourceType: 'Host Agent',
        node: host.identity?.hostname ?? host.name,
        instance: (pd(host)?.platform as string) || (pd(host)?.osName as string) || '',
        status,
        hasOverride:
          hasCustomThresholds ||
          Boolean((override as Override | undefined)?.disabled) ||
          Boolean((override as Override | undefined)?.disableConnectivity),
        disabled: (override as Override | undefined)?.disabled || false,
        disableConnectivity: (override as Override | undefined)?.disableConnectivity || false,
        thresholds: (override as Override | undefined)?.thresholds || {},
        defaults: props.hostDefaults,
      } satisfies TableResource;
    });

    (props.overrides() ?? [])
      .filter(
        (override) =>
          (override as Override).type === 'hostAgent' && !seen.has((override as Override).id),
      )
      .forEach((override) => {
        const name = (override as Override).name?.trim() || (override as Override).id;
        hosts.push({
          id: (override as Override).id,
          name,
          displayName: name,
          rawName: name,
          type: 'hostAgent' as const,
          resourceType: 'Host Agent',
          node: '',
          instance: '',
          status: 'unknown',
          hasOverride: true,
          disabled: (override as Override).disabled || false,
          disableConnectivity: (override as Override).disableConnectivity || false,
          thresholds: (override as Override).thresholds || {},
          defaults: props.hostDefaults,
        } satisfies TableResource);
      });

    if (search) {
      return hosts.filter((host) => host.name.toLowerCase().includes(search));
    }

    return hosts;
  }, []);

  // Helper function to create host disk resource ID (matches backend sanitizeHostComponent)
  const hostDiskResourceID = (hostId: string, mountpoint: string, device?: string): string => {
    // Use mountpoint if available, otherwise device
    let label = (mountpoint?.trim() || device?.trim() || 'disk').toLowerCase();
    // Replicate backend sanitizeHostComponent: keep a-z 0-9, replace everything else with '-', collapse consecutive hyphens
    label = label
      .replace(/[^a-z0-9]/g, '-')
      .replace(/-{2,}/g, '-')
      .replace(/^-|-$/g, '');
    if (!label) label = 'unknown';
    return `host:${hostId}/disk:${label}`;
  };

  // Process host disks with their overrides
  const hostDisksWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));
    const seen = new Set<string>();
    const disks: TableResource[] = [];

    // Extract disks from all hosts
    (props.hosts ?? []).forEach((host) => {
      const hostDisplayName =
        host.displayName?.trim() || host.identity?.hostname || host.name || host.id;

      const disksForHost = (pd(host)?.disks ?? []) as Array<{
        mountpoint?: string;
        device?: string;
        used?: number;
        total?: number;
        type?: string;
      }>;

      disksForHost.forEach((disk) => {
        const diskLabel = disk.mountpoint?.trim() || disk.device?.trim() || 'disk';
        const resourceId = hostDiskResourceID(host.id, disk.mountpoint || '', disk.device);
        const override = overridesMap.get(resourceId);

        const hasCustomThresholds =
          (override as Override | undefined)?.thresholds?.disk !== undefined &&
          (override as Override).thresholds.disk !== props.hostDefaults.disk;

        seen.add(resourceId);

        disks.push({
          id: resourceId,
          name: diskLabel,
          displayName: diskLabel,
          rawName: disk.device || diskLabel,
          type: 'hostDisk' as const,
          resourceType: 'Host Disk',
          host: host.id,
          node: hostDisplayName,
          instance: disk.type || '',
          status: host.status,
          hasOverride: hasCustomThresholds || Boolean((override as Override | undefined)?.disabled),
          disabled: (override as Override | undefined)?.disabled || false,
          thresholds: (override as Override | undefined)?.thresholds || {},
          defaults: { disk: props.hostDefaults.disk },
          subtitle: `${((disk.used || 0) / 1024 / 1024 / 1024).toFixed(1)} / ${((disk.total || 0) / 1024 / 1024 / 1024).toFixed(1)} GB`,
        } satisfies TableResource);
      });
    });

    // Include any hostDisk overrides for disks that are no longer present
    (props.overrides() ?? [])
      .filter(
        (override) =>
          (override as Override).type === 'hostDisk' && !seen.has((override as Override).id),
      )
      .forEach((override) => {
        const name = (override as Override).name || (override as Override).id;
        disks.push({
          id: (override as Override).id,
          name,
          displayName: name,
          rawName: name,
          type: 'hostDisk' as const,
          resourceType: 'Host Disk',
          host: '',
          node: 'Unknown Host',
          instance: '',
          status: 'unknown',
          hasOverride: true,
          disabled: (override as Override).disabled || false,
          thresholds: (override as Override).thresholds || {},
          defaults: { disk: props.hostDefaults.disk },
        });
      });

    if (search) {
      return disks.filter(
        (d) => d.name.toLowerCase().includes(search) || d.node?.toLowerCase().includes(search),
      );
    }

    return disks;
  }, []);

  // Group host disks by their host
  const hostDisksGroupedByHost = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    hostDisksWithOverrides().forEach((disk) => {
      const key = disk.node?.trim() || 'Unknown Host';
      if (!grouped[key]) {
        grouped[key] = [];
      }
      grouped[key].push(disk);
    });

    // Sort disks within each host by name
    Object.values(grouped).forEach((resources) => {
      resources.sort((a, b) => a.name.localeCompare(b.name));
    });

    return grouped;
  });

  // Process Docker hosts with their overrides (primarily for connectivity toggles)

  const dockerHostsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));
    const seen = new Set<string>();

    const hosts: TableResource[] = (props.dockerHosts ?? []).map((host) => {
      const originalName =
        host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const friendlyName = getFriendlyNodeName(originalName);
      const override = overridesMap.get(host.id);
      const disableConnectivity = (override as Override | undefined)?.disableConnectivity || false;
      const status = host.status;

      seen.add(host.id);

      return {
        id: host.id,
        name: friendlyName,
        displayName: friendlyName,
        rawName: originalName,
        type: 'dockerHost' as const,
        resourceType: 'Container Host',
        node: host.identity?.hostname ?? host.name,
        instance: (pd(host)?.platform as string) || (pd(host)?.osName as string) || '',
        status,
        hasOverride: disableConnectivity,
        disableConnectivity,
        thresholds: (override as Override | undefined)?.thresholds || {},
        defaults: {},
        editable: false,
      } satisfies TableResource;
    });

    // Include any overrides referencing Docker hosts that are no longer reporting
    (props.overrides() ?? [])
      .filter(
        (override) =>
          (override as Override).type === 'dockerHost' && !seen.has((override as Override).id),
      )
      .forEach((override) => {
        const originalName = (override as Override).name || (override as Override).id;
        const friendlyName = getFriendlyNodeName(originalName);
        hosts.push({
          id: (override as Override).id,
          name: friendlyName,
          displayName: friendlyName,
          rawName: originalName,
          type: 'dockerHost',
          resourceType: 'Container Host',
          node: (override as Override).node || '',
          instance: (override as Override).instance || '',
          status: 'unknown',
          hasOverride: true,
          disableConnectivity: (override as Override).disableConnectivity || false,
          thresholds: (override as Override).thresholds || {},
          defaults: {},
          editable: false,
        });
      });

    if (search) {
      return hosts.filter((host) => host.name.toLowerCase().includes(search));
    }
    return hosts;
  }, []);

  const dockerContainersByHostId = createMemo(() => {
    const map = new Map<string, Resource[]>();
    (props.allResources ?? []).forEach((resource) => {
      if (resource.type !== 'docker-container') return;
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

  // Process Docker containers grouped by host

  const dockerContainersGroupedByHost = createMemo<Record<string, TableResource[]>>((prev = {}) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));
    const groups: Record<string, TableResource[]> = {};
    const seen = new Set<string>();

    (props.dockerHosts ?? []).forEach((host) => {
      const hostLabel = host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const friendlyHostName = getFriendlyNodeName(hostLabel);
      const hostLabelLower = hostLabel.toLowerCase();
      const friendlyHostNameLower = friendlyHostName.toLowerCase();

      const hostHostname = host.identity?.hostname ?? host.name;
      const containers = dockerContainersByHostId().get(host.id) ?? [];

      containers.forEach((container) => {
        const shortId = container.id.includes('/')
          ? (container.id.split('/').pop() ?? container.id)
          : container.id;
        const resourceId = `docker:${host.id}/${shortId}`;
        const override = overridesMap.get(resourceId);
        const overrideSeverity = (override as Override | undefined)?.poweredOffSeverity;

        const defaults = props.dockerDefaults as Record<string, number | undefined>;
        const hasCustomThresholds =
          (override as Override | undefined)?.thresholds &&
          Object.keys((override as Override).thresholds).some((key: string) => {
            const k = key as keyof Override['thresholds'];
            return (
              (override as Override).thresholds[k] !== undefined &&
              (override as Override).thresholds[k] !== defaults?.[k as keyof typeof defaults]
            );
          });

        const hasOverride =
          hasCustomThresholds ||
          (override as Override | undefined)?.disabled ||
          (override as Override | undefined)?.disableConnectivity ||
          overrideSeverity !== undefined ||
          false;

        const containerName = container.name?.replace(/^\/+/, '') || shortId;
        const containerNameLower = containerName.toLowerCase();
        const image = (pd(container)?.image as string) ?? '';
        const imageLower = image.toLowerCase();

        const matchesSearch =
          !search ||
          containerNameLower.includes(search) ||
          hostLabelLower.includes(search) ||
          friendlyHostNameLower.includes(search) ||
          imageLower.includes(search);
        if (!matchesSearch) {
          return;
        }

        const status = container.status;
        const groupKey = friendlyHostName || hostLabel;

        const resource: TableResource = {
          id: resourceId,
          name: containerName,
          type: 'dockerContainer',
          resourceType: 'Container',
          node: groupKey,
          instance: hostHostname,
          status,
          hasOverride,
          disabled: (override as Override | undefined)?.disabled || false,
          disableConnectivity: (override as Override | undefined)?.disableConnectivity || false,
          thresholds: (override as Override | undefined)?.thresholds || {},
          defaults: props.dockerDefaults,
          hostId: host.id,
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

    // Include overrides for Docker containers that aren't currently reporting
    (props.overrides() ?? [])
      .filter(
        (override) =>
          (override as Override).type === 'dockerContainer' && !seen.has((override as Override).id),
      )
      .forEach((override) => {
        const fallbackName =
          (override as Override).name ||
          (override as Override).id.split('/').pop() ||
          (override as Override).id;
        const group = 'Unassigned Containers';
        if (!groups[group]) {
          groups[group] = [];
        }
        groups[group].push({
          id: (override as Override).id,
          name: fallbackName,
          type: 'dockerContainer',
          resourceType: 'Container',
          status: 'unknown',
          hasOverride: true,
          disabled: (override as Override).disabled || false,
          disableConnectivity: (override as Override).disableConnectivity || false,
          thresholds: (override as Override).thresholds || {},
          defaults: props.dockerDefaults,
          poweredOffSeverity: (override as Override).poweredOffSeverity,
        });
      });

    Object.keys(groups).forEach((group) => {
      groups[group].sort((a, b) => a.name.localeCompare(b.name));
    });

    if (!search) {
      return groups;
    }

    // With search applied, remove empty groups (should already be filtered)
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
    (props.dockerHosts ?? []).reduce(
      (sum, host) => sum + (dockerContainersByHostId().get(host.id)?.length ?? 0),
      0,
    ),
  );

  const dockerHostGroupMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.dockerHosts ?? []).forEach((host) => {
      const originalName =
        host.displayName?.trim() || host.identity?.hostname || host.name || host.id;
      const friendlyName = getFriendlyNodeName(originalName);
      const headerMeta: GroupHeaderMeta = {
        displayName: friendlyName,
        rawName: originalName,
        status: host.status,
      };

      const hostname = host.identity?.hostname ?? host.name;
      [friendlyName, originalName, hostname, host.id]
        .filter((key: string): key is string => Boolean(key && key.trim()))
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

  const snapshotFactoryConfig = () =>
    props.snapshotFactoryDefaults ?? {
      enabled: false,
      warningDays: DEFAULT_SNAPSHOT_WARNING,
      criticalDays: DEFAULT_SNAPSHOT_CRITICAL,
      warningSizeGiB: DEFAULT_SNAPSHOT_WARNING_SIZE,
      criticalSizeGiB: DEFAULT_SNAPSHOT_CRITICAL_SIZE,
    };

  const sanitizeSnapshotConfig = (config: SnapshotAlertConfig): SnapshotAlertConfig => {
    let warning = Math.max(0, Math.round(config.warningDays ?? 0));
    let critical = Math.max(0, Math.round(config.criticalDays ?? 0));

    if (critical > 0 && warning > critical) {
      warning = critical;
    }
    if (critical === 0 && warning > 0) {
      critical = warning;
    }

    const rawWarningSize = Number.isFinite(config.warningSizeGiB)
      ? Number(config.warningSizeGiB)
      : DEFAULT_SNAPSHOT_WARNING_SIZE;
    const rawCriticalSize = Number.isFinite(config.criticalSizeGiB)
      ? Number(config.criticalSizeGiB)
      : DEFAULT_SNAPSHOT_CRITICAL_SIZE;

    const roundSize = (value: number) => Math.round(Math.max(0, value) * 10) / 10;

    let warningSize = roundSize(rawWarningSize);
    let criticalSize = roundSize(rawCriticalSize);

    if (criticalSize > 0 && warningSize > criticalSize) {
      warningSize = criticalSize;
    }
    if (criticalSize === 0 && warningSize > 0) {
      criticalSize = warningSize;
    }

    return {
      enabled: !!config.enabled,
      warningDays: warning,
      criticalDays: critical,
      warningSizeGiB: warningSize,
      criticalSizeGiB: criticalSize,
    };
  };

  const backupFactoryConfig = () =>
    props.backupFactoryDefaults ?? {
      enabled: false,
      warningDays: DEFAULT_BACKUP_WARNING,
      criticalDays: DEFAULT_BACKUP_CRITICAL,
      freshHours: DEFAULT_BACKUP_FRESH_HOURS,
      staleHours: DEFAULT_BACKUP_STALE_HOURS,
      alertOrphaned: true,
      ignoreVMIDs: [],
    };

  const sanitizeBackupConfig = (config: BackupAlertConfig): BackupAlertConfig => {
    let warning = Math.max(0, Math.round(config.warningDays ?? 0));
    let critical = Math.max(0, Math.round(config.criticalDays ?? 0));
    let fresh = Math.max(0, Math.round(config.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS));
    let stale = Math.max(0, Math.round(config.staleHours ?? DEFAULT_BACKUP_STALE_HOURS));
    const alertOrphaned = config.alertOrphaned ?? true;
    const ignoreVMIDs = Array.from(
      new Set(
        (config.ignoreVMIDs ?? []).map((value) => value.trim()).filter((value) => value.length > 0),
      ),
    );

    if (critical > 0 && warning > critical) {
      warning = critical;
    }
    if (critical === 0 && warning > 0) {
      critical = warning;
    }

    // Ensure stale is at least fresh
    if (stale < fresh) {
      stale = fresh;
    }

    return {
      enabled: !!config.enabled,
      warningDays: warning,
      criticalDays: critical,
      freshHours: fresh,
      staleHours: stale,
      alertOrphaned,
      ignoreVMIDs,
    };
  };

  const snapshotDefaultsRecord = createMemo(() => {
    const current = props.snapshotDefaults();
    return {
      'warning days': current.warningDays ?? 0,
      'critical days': current.criticalDays ?? 0,
      'warning size (gib)': current.warningSizeGiB ?? 0,
      'critical size (gib)': current.criticalSizeGiB ?? 0,
    };
  });

  const snapshotFactoryDefaultsRecord = createMemo(() => {
    const factory = snapshotFactoryConfig();
    return {
      'warning days': factory.warningDays ?? DEFAULT_SNAPSHOT_WARNING,
      'critical days': factory.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL,
      'warning size (gib)': factory.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE,
      'critical size (gib)': factory.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE,
    };
  });

  const backupDefaultsRecord = createMemo(() => {
    const current = props.backupDefaults();
    return {
      'fresh hours': current.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
      'stale hours': current.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
      'warning days': current.warningDays ?? 0,
      'critical days': current.criticalDays ?? 0,
    };
  });

  const backupFactoryDefaultsRecord = createMemo(() => {
    const factory = backupFactoryConfig();
    return {
      'fresh hours': factory.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
      'stale hours': factory.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
      'warning days': factory.warningDays ?? DEFAULT_BACKUP_WARNING,
      'critical days': factory.criticalDays ?? DEFAULT_BACKUP_CRITICAL,
    };
  });

  const snapshotOverridesCount = createMemo(() => {
    const current = props.snapshotDefaults();
    const factory = snapshotFactoryConfig();
    const differs =
      current.enabled !== factory.enabled ||
      (current.warningDays ?? DEFAULT_SNAPSHOT_WARNING) !==
        (factory.warningDays ?? DEFAULT_SNAPSHOT_WARNING) ||
      (current.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL) !==
        (factory.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL) ||
      (current.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE) !==
        (factory.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE) ||
      (current.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE) !==
        (factory.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE);
    return differs ? 1 : 0;
  });

  const backupOverridesCount = createMemo(() => {
    const backupCurrent = props.backupDefaults();
    const backupFactory = backupFactoryConfig();
    const currentIgnore = backupCurrent.ignoreVMIDs ?? [];
    const factoryIgnore = backupFactory.ignoreVMIDs ?? [];
    const ignoreDiff =
      currentIgnore.length !== factoryIgnore.length ||
      currentIgnore.some((value, index) => value !== factoryIgnore[index]);
    return backupCurrent.enabled !== backupFactory.enabled ||
      (backupCurrent.warningDays ?? DEFAULT_BACKUP_WARNING) !==
        (backupFactory.warningDays ?? DEFAULT_BACKUP_WARNING) ||
      (backupCurrent.criticalDays ?? DEFAULT_BACKUP_CRITICAL) !==
        (backupFactory.criticalDays ?? DEFAULT_BACKUP_CRITICAL) ||
      (backupCurrent.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS) !==
        (backupFactory.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS) ||
      (backupCurrent.staleHours ?? DEFAULT_BACKUP_STALE_HOURS) !==
        (backupFactory.staleHours ?? DEFAULT_BACKUP_STALE_HOURS) ||
      (backupCurrent.alertOrphaned ?? true) !== (backupFactory.alertOrphaned ?? true) ||
      ignoreDiff
      ? 1
      : 0;
  });

  // Process guests with their overrides and group by node
  const guestsGroupedByNode = createMemo<Record<string, TableResource[]>>((prev = {}) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));

    const guests = (props.allGuests() ?? []).map((guest) => {
      const gpd = guest.platformData
        ? (unwrap(guest.platformData) as Record<string, unknown>)
        : undefined;
      const vmid = (gpd?.vmid as number | undefined) ?? undefined;
      const node = (gpd?.node as string | undefined) ?? '';
      const instance = (gpd?.instance as string | undefined) ?? guest.platformId ?? '';
      const guestId = guest.id;
      const override = overridesMap.get(guestId);
      const overrideSeverity = (override as Override | undefined)?.poweredOffSeverity;

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        (override as Override | undefined)?.thresholds &&
        Object.keys((override as Override).thresholds).some((key: string) => {
          const k = key as keyof Override['thresholds'];
          return (
            (override as Override).thresholds[k] !== undefined &&
            (override as Override).thresholds[k] !== (props.guestDefaults as any)[k]
          );
        });

      // A guest has an override if it has custom thresholds OR is disabled OR has connectivity disabled
      const hasOverride =
        hasCustomThresholds ||
        (override as Override | undefined)?.disabled ||
        (override as Override | undefined)?.disableConnectivity ||
        overrideSeverity !== undefined ||
        false;

      return {
        id: guestId,
        name: guest.name,
        type: 'guest' as const,
        resourceType: guest.type === 'vm' ? 'VM' : 'CT',
        vmid,
        node,
        instance,
        status: guest.status,
        hasOverride: hasOverride,
        disabled: (override as Override | undefined)?.disabled || false,
        disableConnectivity: (override as Override | undefined)?.disableConnectivity || false,
        thresholds: (override as Override | undefined)?.thresholds || {},
        defaults: props.guestDefaults,
        backup: (override as Override | undefined)?.backup || props.backupDefaults(),
        snapshot: (override as Override | undefined)?.snapshot || props.snapshotDefaults(),
        poweredOffSeverity: overrideSeverity,
      };
    });

    const filteredGuests = search
      ? guests.filter(
          (g) =>
            g.name.toLowerCase().includes(search) ||
            g.vmid?.toString().includes(search) ||
            g.node?.toLowerCase().includes(search),
        )
      : guests;

    // Group by instance (not node - node is just the hostname which may be duplicated)
    // Instance is the disambiguated name like "px1" or "px1 (10.0.2.224)"
    const grouped: Record<string, TableResource[]> = {};
    filteredGuests.forEach((guest) => {
      const groupKey = guest.instance || guest.node || 'Unknown';
      if (!grouped[groupKey]) {
        grouped[groupKey] = [];
      }
      grouped[groupKey].push(guest);
    });

    // Sort guests within each group by vmid
    Object.keys(grouped).forEach((node) => {
      grouped[node].sort((a, b) => {
        if (a.vmid && b.vmid) return a.vmid - b.vmid;
        return a.name.localeCompare(b.name);
      });
    });

    return grouped;
  }, {});

  const guestsFlat = createMemo<TableResource[]>(() =>
    Object.values(guestsGroupedByNode() ?? {}).flat(),
  );

  const guestGroupHeaderMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.nodes ?? []).forEach((node) => {
      const { headerMeta, keys } = buildNodeHeaderMeta(node);
      keys.forEach((key: string) => {
        meta[key] = headerMeta;
      });
    });
    return meta;
  });

  // Process PBS servers with their overrides

  const pbsServersWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));

    // Get PBS instances from props
    const pbsInstances = props.pbsInstances || [];

    const pbsServers = pbsInstances.map((pbs) => {
      // Offline PBS instances report zero metrics; keep them visible so connectivity toggles stay usable
      // PBS IDs already have "pbs-" prefix from backend, don't double it
      const pbsId = pbs.id;
      const override = overridesMap.get(pbsId);

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        (override as Override | undefined)?.thresholds &&
        Object.keys((override as Override).thresholds).some((key: string) => {
          const k = key as keyof Override['thresholds'];
          // PBS uses pbsDefaults for CPU/Memory (not nodeDefaults)
          return (
            (override as Override).thresholds[k] !== undefined &&
            (override as Override).thresholds[k] !==
              (props.pbsDefaults?.[k as keyof typeof props.pbsDefaults] ?? (k === 'cpu' ? 80 : 85))
          );
        });

      const disableConnectivity = (override as Override | undefined)?.disableConnectivity || false;
      const hasOverride = hasCustomThresholds || disableConnectivity;

      return {
        id: pbsId,
        name: pbs.name,
        type: 'pbs' as const,
        resourceType: 'PBS',
        host: pbs.host,
        status: pbs.status,
        cpu: pbs.cpu,
        memory: pbs.memory,
        memoryUsed: pbs.memoryUsed,
        memoryTotal: pbs.memoryTotal,
        uptime: pbs.uptime,
        hasOverride,
        disabled: false,
        disableConnectivity,
        thresholds: (override as Override | undefined)?.thresholds || {},
        defaults: {
          cpu: props.pbsDefaults?.cpu ?? 80,
          memory: props.pbsDefaults?.memory ?? 85,
        },
      };
    });

    if (search) {
      return pbsServers.filter(
        (p) => p.name.toLowerCase().includes(search) || p.host?.toLowerCase().includes(search),
      );
    }
    return pbsServers;
  }, []);

  const pmgGlobalDefaults = createMemo<Record<string, number>>(() => {
    const defaults = props.pmgThresholds();
    const record: Record<string, number> = {};
    PMG_THRESHOLD_COLUMNS.forEach(({ key, normalized }: { key: any; normalized: any }) => {
      const value = defaults[key as keyof PMGThresholdDefaults];
      record[normalized] = typeof value === 'number' && Number.isFinite(value) ? value : 0;
    });
    return record;
  });

  const pmgServersWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));

    // Get PMG instances from props
    const pmgInstances = props.pmgInstances || [];
    const defaultThresholds = pmgGlobalDefaults();

    const pmgServers = pmgInstances.map((pmg) => {
      // PMG IDs should already have appropriate prefix from backend
      const pmgId = pmg.id;
      const override = overridesMap.get(pmgId);

      const thresholdOverrides: Record<string, number> = {};
      const overrideThresholds = ((override as Override | undefined)?.thresholds ?? {}) as Record<
        string,
        unknown
      >;
      Object.entries(overrideThresholds).forEach(([rawKey, rawValue]) => {
        if (typeof rawValue !== 'number' || Number.isNaN(rawValue)) return;
        const normalizedKey =
          PMG_KEY_TO_NORMALIZED.get(rawKey as keyof PMGThresholdDefaults) ||
          (PMG_NORMALIZED_TO_KEY.has(rawKey) ? rawKey : undefined);
        if (!normalizedKey) return;
        thresholdOverrides[normalizedKey] = rawValue;
      });

      const hasOverride =
        (override as Override | undefined)?.disableConnectivity ||
        (override as Override | undefined)?.disabled ||
        Object.keys(thresholdOverrides).length > 0 ||
        false;

      return {
        id: pmgId,
        name: pmg.name,
        type: 'pmg' as const,
        resourceType: 'PMG',
        host: pmg.host,
        status: pmg.status,
        hasOverride,
        disabled: (override as Override | undefined)?.disabled || false,
        disableConnectivity: (override as Override | undefined)?.disableConnectivity || false,
        thresholds: thresholdOverrides,
        defaults: { ...defaultThresholds },
      };
    });

    if (search) {
      return pmgServers.filter(
        (p) => p.name.toLowerCase().includes(search) || p.host?.toLowerCase().includes(search),
      );
    }
    return pmgServers;
  }, []);

  const storageCoords = (r: Resource): { node: string; instance: string } => {
    const data = pd(r);
    if (r.type === 'datastore') {
      const instance =
        (data?.pbsInstanceId as string | undefined) || r.parentId || r.platformId || 'pbs';
      const node = (data?.pbsInstanceName as string | undefined) || instance;
      return { node, instance };
    }
    return {
      node: (data?.node as string | undefined) || '',
      instance: (data?.instance as string | undefined) || r.platformId || '',
    };
  };

  const normalizeStorageStatus = (status: string | undefined): string => {
    switch ((status ?? '').toLowerCase()) {
      case 'online':
      case 'running':
      case 'available':
        return 'available';
      default:
        return 'offline';
    }
  };

  // Process storage with their overrides

  const storageWithOverrides = createMemo<TableResource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o: Override) => [o.id, o]));

    const storageDevices = (props.storage ?? []).map((storage) => {
      const override = overridesMap.get(storage.id);
      const coords = storageCoords(storage);

      // Storage only has usage threshold
      const hasCustomThresholds =
        (override as Override | undefined)?.thresholds?.usage !== undefined &&
        (override as Override).thresholds.usage !== props.storageDefault();

      // A storage device has an override if it has custom thresholds OR is disabled
      const hasOverride =
        hasCustomThresholds || (override as Override | undefined)?.disabled || false;

      return {
        id: storage.id,
        name: storage.name,
        type: 'storage' as const,
        resourceType: 'Storage',
        node: coords.node,
        instance: coords.instance,
        status: normalizeStorageStatus(storage.status),
        hasOverride: hasOverride,
        disabled: (override as Override | undefined)?.disabled || false,
        thresholds: (override as Override | undefined)?.thresholds || {},
        defaults: { usage: props.storageDefault() },
      };
    });

    if (search) {
      return storageDevices.filter(
        (s) => s.name.toLowerCase().includes(search) || s.node?.toLowerCase().includes(search),
      );
    }
    return storageDevices;
  }, []);

  const storageGroupedByNode = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    storageWithOverrides().forEach((storage) => {
      const key = storage.node?.trim() || 'Unassigned';
      if (!grouped[key]) {
        grouped[key] = [];
      }
      grouped[key].push(storage);
    });

    Object.values(grouped).forEach((resources) => {
      resources.sort((a, b) => a.name.localeCompare(b.name));
    });

    return grouped;
  });

  return {
    nodesWithOverrides,
    hostAgentsWithOverrides,
    hostDisksWithOverrides,
    hostDisksGroupedByHost,
    dockerHostsWithOverrides,
    dockerContainersByHostId,
    dockerContainersGroupedByHost,
    dockerContainersFlat,
    totalDockerContainers,
    dockerHostGroupMeta,
    snapshotFactoryConfig,
    sanitizeSnapshotConfig,
    backupFactoryConfig,
    sanitizeBackupConfig,
    snapshotDefaultsRecord,
    snapshotFactoryDefaultsRecord,
    backupDefaultsRecord,
    backupFactoryDefaultsRecord,
    snapshotOverridesCount,
    backupOverridesCount,
    guestsGroupedByNode,
    guestsFlat,
    guestGroupHeaderMeta,
    pbsServersWithOverrides,
    pmgGlobalDefaults,
    pmgServersWithOverrides,
    storageWithOverrides,
    storageGroupedByNode,
  };
}
