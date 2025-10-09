import { createSignal, createMemo, Show, For, onMount, onCleanup } from 'solid-js';

// Workaround for eslint false-positive when `For` is used only in JSX
const __ensureForUsage = For;
void __ensureForUsage;
import type {
  VM,
  Container,
  Node,
  Alert,
  Storage,
  PBSInstance,
  DockerHost,
  DockerContainer,
} from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import { ResourceTable, Resource, GroupHeaderMeta } from './ResourceTable';
type OverrideType =
  | 'guest'
  | 'node'
  | 'storage'
  | 'pbs'
  | 'dockerHost'
  | 'dockerContainer';

type OfflineState = 'off' | 'warning' | 'critical';

interface Override {
  id: string;
  name: string;
  type: OverrideType;
  resourceType?: string;
  vmid?: number;
  node?: string;
  instance?: string;
  disabled?: boolean;
  disableConnectivity?: boolean; // For nodes only - disable offline alerts
  poweredOffSeverity?: 'warning' | 'critical';
  thresholds: {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
    usage?: number; // For storage devices
    temperature?: number; // For nodes only - CPU temperature in °C
  };
}

// Simple threshold object for the UI
interface SimpleThresholds {
  cpu?: number;
  memory?: number;
  disk?: number;
  diskRead?: number;
  diskWrite?: number;
  networkIn?: number;
  networkOut?: number;
  temperature?: number; // For nodes only
  [key: string]: number | undefined; // Add index signature for compatibility
}

interface ThresholdsTableProps {
  overrides: () => Override[];
  setOverrides: (overrides: Override[]) => void;
  rawOverridesConfig: () => Record<string, RawOverrideConfig>;
  setRawOverridesConfig: (config: Record<string, RawOverrideConfig>) => void;
  allGuests: () => (VM | Container)[];
  nodes: Node[];
  storage: Storage[];
  dockerHosts: DockerHost[];
  pbsInstances?: PBSInstance[]; // PBS instances from state
  guestDefaults: SimpleThresholds;
  setGuestDefaults: (
    value: Record<string, number | undefined> | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
  guestDisableConnectivity: () => boolean;
  setGuestDisableConnectivity: (value: boolean) => void;
  guestPoweredOffSeverity: () => 'warning' | 'critical';
  setGuestPoweredOffSeverity: (value: 'warning' | 'critical') => void;
  nodeDefaults: SimpleThresholds;
  setNodeDefaults: (
    value: Record<string, number | undefined> | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
  dockerDefaults: { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number };
  setDockerDefaults: (
    value: { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number } | ((prev: { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number }) => { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number }),
  ) => void;
  storageDefault: () => number;
  setStorageDefault: (value: number) => void;
  timeThresholds: () => { guest: number; node: number; storage: number; pbs: number };
  setTimeThresholds: (value: { guest: number; node: number; storage: number; pbs: number }) => void;
  setHasUnsavedChanges: (value: boolean) => void;
  activeAlerts?: Record<string, Alert>;
  removeAlerts?: (predicate: (alert: Alert) => boolean) => void;
  // Global disable flags
  disableAllNodes: () => boolean;
  setDisableAllNodes: (value: boolean) => void;
  disableAllGuests: () => boolean;
  setDisableAllGuests: (value: boolean) => void;
  disableAllStorage: () => boolean;
  setDisableAllStorage: (value: boolean) => void;
  disableAllPBS: () => boolean;
  setDisableAllPBS: (value: boolean) => void;
  disableAllDockerHosts: () => boolean;
  setDisableAllDockerHosts: (value: boolean) => void;
  disableAllDockerContainers: () => boolean;
  setDisableAllDockerContainers: (value: boolean) => void;
  // Global disable offline alerts flags
  disableAllNodesOffline: () => boolean;
  setDisableAllNodesOffline: (value: boolean) => void;
  disableAllGuestsOffline: () => boolean;
  setDisableAllGuestsOffline: (value: boolean) => void;
  disableAllPBSOffline: () => boolean;
  setDisableAllPBSOffline: (value: boolean) => void;
  disableAllDockerHostsOffline: () => boolean;
  setDisableAllDockerHostsOffline: (value: boolean) => void;
}

export function ThresholdsTable(props: ThresholdsTableProps) {
  const [searchTerm, setSearchTerm] = createSignal('');
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<
    Record<string, number | undefined>
  >({});
  const [activeTab, setActiveTab] = createSignal<'proxmox' | 'docker'>('proxmox');
  let searchInputRef: HTMLInputElement | undefined;

  // Set up keyboard shortcuts
  onMount(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Skip if user is typing in an input or textarea (unless it's Escape)
      const target = e.target as HTMLElement;
      const isInInput =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.contentEditable === 'true';

      // Escape clears search from anywhere
      if (e.key === 'Escape') {
        e.preventDefault();
        setSearchTerm('');
        if (searchInputRef && isInInput) {
          searchInputRef.blur();
        }
        return;
      }

      // Skip other shortcuts if already in an input
      if (isInInput) {
        return;
      }

      // Any letter/number focuses search and starts typing
      if (e.key.length === 1 && e.key.match(/[a-z0-9]/i)) {
        e.preventDefault();
        if (searchInputRef) {
          searchInputRef.focus();
          setSearchTerm(e.key);
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);

    onCleanup(() => {
      document.removeEventListener('keydown', handleKeyDown);
    });
  });

  // Helper function to format values with units
  const formatMetricValue = (metric: string, value: number | undefined): string => {
    if (value === undefined || value === null) return '0';

    // Show "Off" for disabled thresholds (0 or negative values)
    if (value <= 0) return 'Off';

    // Percentage-based metrics
    if (
      metric === 'cpu' ||
      metric === 'memory' ||
      metric === 'disk' ||
      metric === 'usage' ||
      metric === 'memoryWarnPct' ||
      metric === 'memoryCriticalPct'
    ) {
      return `${value}%`;
    }

    // Temperature in Celsius
    if (metric === 'temperature') {
      return `${value}°C`;
    }

    if (metric === 'restartWindow') {
      return `${value}s`;
    }

    if (metric === 'restartCount') {
      return String(value);
    }

    // MB/s metrics
    if (
      metric === 'diskRead' ||
      metric === 'diskWrite' ||
      metric === 'networkIn' ||
      metric === 'networkOut'
    ) {
      return `${value} MB/s`;
    }

    return String(value);
  };

  // Check if there's an active alert for a resource/metric
  const hasActiveAlert = (resourceId: string, metric: string): boolean => {
    if (!props.activeAlerts) return false;
    const alertKey = `${resourceId}-${metric}`;
    return alertKey in props.activeAlerts;
  };

  // Process nodes with their overrides
const getFriendlyNodeName = (value: string, clusterName?: string): string => {
  if (!value) return value;

  const clusterLower = clusterName?.toLowerCase().trim();

  const normalizeToken = (token?: string | null): string => {
    if (!token) return '';
    let result = token.replace(/\(.*?\)/g, ' ').replace(/\s+/g, ' ').trim();
    if (clusterLower) {
      result = result
        .split(' ')
        .filter((part) => part.toLowerCase() !== clusterLower)
        .join(' ')
        .trim();
    }
    if (!result) return '';
    const firstWord = result.split(/\s+/)[0] || result;
    const withoutDomain = firstWord.includes('.') ? firstWord.split('.')[0] ?? firstWord : firstWord;
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

const buildNodeHeaderMeta = (node: Node) => {
  const originalDisplayName = node.displayName?.trim() || node.name;
  const friendlyName = getFriendlyNodeName(originalDisplayName, node.clusterName);
  const hostValue = node.host?.trim();
  let host: string | undefined;
  if (hostValue && hostValue !== '') {
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
    clusterName: node.isClusterMember ? node.clusterName?.trim() || 'Cluster' : undefined,
    isClusterMember: node.isClusterMember ?? false,
  };

  const keys = new Set<string>();
  [node.name, originalDisplayName, friendlyName].forEach((value) => {
    if (value && value.trim()) {
      keys.add(value.trim());
    }
  });

  return { headerMeta, keys };
};

  const nodesWithOverrides = createMemo<Resource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    const nodes = (props.nodes ?? []).map((node) => {
      const override = overridesMap.get(node.id);

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        override?.thresholds &&
        Object.keys(override.thresholds).some((key) => {
          const k = key as keyof typeof override.thresholds;
          return (
            override.thresholds[k] !== undefined &&
            override.thresholds[k] !== (props.nodeDefaults as any)[k]
          );
        });

      const originalDisplayName = node.displayName?.trim() || node.name;
      const friendlyName = getFriendlyNodeName(originalDisplayName, node.clusterName);
      const rawName = node.name;
      const sanitizedName = friendlyName || originalDisplayName || rawName.split('.')[0] || rawName;
      // Build a best-effort management URL for the node
      const hostValue = node.host?.trim() || rawName;
      const normalizedHost = hostValue.startsWith('http://') || hostValue.startsWith('https://')
        ? hostValue
        : `https://${hostValue.includes(':') ? hostValue : `${hostValue}:8006`}`;

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
        cpu: node.cpu,
        memory: node.memory?.usage,
        hasOverride: hasCustomThresholds || false,
        disabled: false,
        disableConnectivity: override?.disableConnectivity || false,
       thresholds: override?.thresholds || {},
       defaults: props.nodeDefaults,
        clusterName: node.isClusterMember ? node.clusterName?.trim() : undefined,
        isClusterMember: node.isClusterMember ?? false,
        instance: node.instance,
      } satisfies Resource;
    });

    if (search) {
      return nodes.filter((n) => n.name.toLowerCase().includes(search));
    }
    return nodes;
  }, []);

  // Process Docker hosts with their overrides (primarily for connectivity toggles)
  const dockerHostsWithOverrides = createMemo<Resource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));
    const seen = new Set<string>();

    const hosts = (props.dockerHosts ?? []).map((host) => {
      const originalName = host.displayName?.trim() || host.hostname || host.id;
      const friendlyName = getFriendlyNodeName(originalName);
      const override = overridesMap.get(host.id);
      const disableConnectivity = override?.disableConnectivity || false;
      const status = host.status || (host.lastSeen ? 'online' : 'offline');

      seen.add(host.id);

      return {
        id: host.id,
        name: friendlyName,
        displayName: friendlyName,
        rawName: originalName,
        type: 'dockerHost' as const,
        resourceType: 'Docker Host',
        node: host.hostname,
        instance: host.displayName,
        status,
        hasOverride: disableConnectivity,
        disableConnectivity,
        thresholds: override?.thresholds || {},
        defaults: {},
      } satisfies Resource;
    });

    // Include any overrides referencing Docker hosts that are no longer reporting
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
          resourceType: 'Docker Host',
          node: override.node || '',
          instance: override.instance || '',
          status: 'unknown',
          hasOverride: true,
          disableConnectivity: override.disableConnectivity || false,
          thresholds: override.thresholds || {},
          defaults: {},
        });
      });

    if (search) {
      return hosts.filter((host) => host.name.toLowerCase().includes(search));
    }
    return hosts;
  }, []);

  // Process Docker containers grouped by host
const dockerContainersGroupedByHost = createMemo<Record<string, Resource[]>>((prev = {}) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));
    const groups: Record<string, Resource[]> = {};
    const seen = new Set<string>();

    const normalizeContainerName = (container: DockerContainer): string => {
      const name = container.name?.trim() || '';
      if (name.startsWith('/')) {
        return name.replace(/^\/+/, '') || (container.id?.slice(0, 12) ?? 'container');
      }
      if (!name) {
        return container.id?.slice(0, 12) ?? 'container';
      }
      return name;
    };

    (props.dockerHosts ?? []).forEach((host) => {
      const hostLabel = host.displayName?.trim() || host.hostname || host.id;
      const friendlyHostName = getFriendlyNodeName(hostLabel);
      const hostLabelLower = hostLabel.toLowerCase();
      const friendlyHostNameLower = friendlyHostName.toLowerCase();

      (host.containers || []).forEach((container) => {
        const containerId = container.id || normalizeContainerName(container);
        const resourceId = `docker:${host.id}/${containerId}`;
        const override = overridesMap.get(resourceId);
        const overrideSeverity = override?.poweredOffSeverity;

        const defaults = props.dockerDefaults as Record<string, number | undefined>;
        const hasCustomThresholds =
          override?.thresholds &&
          Object.keys(override.thresholds).some((key) => {
            const k = key as keyof typeof override.thresholds;
            return (
              override.thresholds[k] !== undefined &&
              override.thresholds[k] !== defaults?.[k as keyof typeof defaults]
            );
          });

        const hasOverride =
          hasCustomThresholds || override?.disabled || override?.disableConnectivity || overrideSeverity !== undefined || false;

        const containerName = normalizeContainerName(container);
        const containerNameLower = containerName.toLowerCase();
        const imageLower = container.image?.toLowerCase() || '';

        const matchesSearch =
          !search ||
          containerNameLower.includes(search) ||
          hostLabelLower.includes(search) ||
          friendlyHostNameLower.includes(search) ||
          imageLower.includes(search);
        if (!matchesSearch) {
          return;
        }

        const status = container.state || container.status || 'unknown';
        const groupKey = friendlyHostName || hostLabel;

        const resource: Resource = {
          id: resourceId,
          name: containerName,
          type: 'dockerContainer',
          resourceType: 'Docker Container',
          node: groupKey,
          instance: host.hostname,
          status,
          hasOverride,
          disabled: override?.disabled || false,
          disableConnectivity: override?.disableConnectivity || false,
          thresholds: override?.thresholds || {},
          defaults: props.dockerDefaults,
          hostId: host.id,
          image: container.image,
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
      .filter((override) => override.type === 'dockerContainer' && !seen.has(override.id))
      .forEach((override) => {
        const fallbackName = override.name || override.id.split('/').pop() || override.id;
        const group = 'Unassigned Docker Containers';
        if (!groups[group]) {
          groups[group] = [];
        }
        groups[group].push({
          id: override.id,
          name: fallbackName,
          type: 'dockerContainer',
          resourceType: 'Docker Container',
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

    // With search applied, remove empty groups (should already be filtered)
    const filteredGroups: Record<string, Resource[]> = {};
    Object.entries(groups).forEach(([group, resources]) => {
      if (resources.length > 0) {
        filteredGroups[group] = resources;
      }
    });
    return filteredGroups;
  }, {});

  const dockerContainersFlat = createMemo<Resource[]>(() =>
    Object.values(dockerContainersGroupedByHost() ?? {}).flat(),
  );

  const totalDockerContainers = createMemo(() =>
    (props.dockerHosts ?? []).reduce((sum, host) => sum + (host.containers?.length ?? 0), 0),
  );

  const dockerHostGroupMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.dockerHosts ?? []).forEach((host) => {
      const originalName = host.displayName?.trim() || host.hostname || host.id;
      const friendlyName = getFriendlyNodeName(originalName);
      const headerMeta: GroupHeaderMeta = {
        displayName: friendlyName,
        rawName: originalName,
        status: host.status || (host.lastSeen ? 'online' : 'offline'),
      };

      [friendlyName, originalName, host.hostname, host.id]
        .filter((key): key is string => Boolean(key && key.trim()))
        .forEach((key) => {
          meta[key.trim()] = headerMeta;
        });
    });

    meta['Unassigned Docker Containers'] = {
      displayName: 'Unassigned Docker Containers',
      status: 'unknown',
    };

    return meta;
  });

  const countOverrides = (resources: Resource[] | undefined) =>
    resources?.filter((resource) => resource.hasOverride || resource.disabled || resource.disableConnectivity)
      .length ?? 0;

  const registerSection = (_key: string) => (_el: HTMLDivElement | null) => {
    /* no-op placeholder for future scroll restoration */
  };

  // Process guests with their overrides and group by node
  const guestsGroupedByNode = createMemo<Record<string, Resource[]>>((prev = {}) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    const guests = (props.allGuests() ?? []).map((guest) => {
      const guestId = guest.id || `${guest.instance}-${guest.node}-${guest.vmid}`;
      const override = overridesMap.get(guestId);
      const overrideSeverity = override?.poweredOffSeverity;

      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds =
        override?.thresholds &&
        Object.keys(override.thresholds).some((key) => {
          const k = key as keyof typeof override.thresholds;
          return (
            override.thresholds[k] !== undefined &&
            override.thresholds[k] !== (props.guestDefaults as any)[k]
          );
        });

      // A guest has an override if it has custom thresholds OR is disabled OR has connectivity disabled
      const hasOverride =
        hasCustomThresholds || override?.disabled || override?.disableConnectivity || overrideSeverity !== undefined || false;

      return {
        id: guestId,
        name: guest.name,
        type: 'guest' as const,
        resourceType: guest.type === 'qemu' ? 'VM' : 'CT',
        vmid: guest.vmid,
        node: guest.node,
        instance: guest.instance,
        status: guest.status,
        hasOverride: hasOverride,
        disabled: override?.disabled || false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.guestDefaults,
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

    // Group by node
    const grouped: Record<string, Resource[]> = {};
    filteredGuests.forEach((guest) => {
      const node = guest.node || 'Unknown';
      if (!grouped[node]) {
        grouped[node] = [];
      }
      grouped[node].push(guest);
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

  const guestsFlat = createMemo<Resource[]>(() =>
    Object.values(guestsGroupedByNode() ?? {}).flat(),
  );

  const guestGroupHeaderMeta = createMemo<Record<string, GroupHeaderMeta>>(() => {
    const meta: Record<string, GroupHeaderMeta> = {};
    (props.nodes ?? []).forEach((node) => {
      const { headerMeta, keys } = buildNodeHeaderMeta(node);
      keys.forEach((key) => {
        meta[key] = headerMeta;
      });
    });
    return meta;
  });

  // Process PBS servers with their overrides
  const pbsServersWithOverrides = createMemo<Resource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    // Get PBS instances from props
    const pbsInstances = props.pbsInstances || [];

    const pbsServers = pbsInstances
      .filter((pbs) => (pbs.cpu || 0) > 0 || (pbs.memory || 0) > 0)
      .map((pbs) => {
        // PBS IDs already have "pbs-" prefix from backend, don't double it
        const pbsId = pbs.id;
        const override = overridesMap.get(pbsId);

        // Check if any threshold values actually differ from defaults
        const hasCustomThresholds =
          override?.thresholds &&
          Object.keys(override.thresholds).some((key) => {
            const k = key as keyof typeof override.thresholds;
            // PBS uses node defaults for CPU/Memory
            return (
              override.thresholds[k] !== undefined &&
              override.thresholds[k] !== props.nodeDefaults[k as keyof typeof props.nodeDefaults]
            );
          });

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
          hasOverride: hasCustomThresholds || false,
          disabled: false,
          disableConnectivity: override?.disableConnectivity || false,
          thresholds: override?.thresholds || {},
          defaults: {
            cpu: props.nodeDefaults.cpu,
            memory: props.nodeDefaults.memory,
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

  // Process storage with their overrides
  const storageWithOverrides = createMemo<Resource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map((props.overrides() ?? []).map((o) => [o.id, o]));

    const storageDevices = (props.storage ?? []).map((storage) => {
      const override = overridesMap.get(storage.id);

      // Storage only has usage threshold
      const hasCustomThresholds =
        override?.thresholds?.usage !== undefined &&
        override.thresholds.usage !== props.storageDefault();

      // A storage device has an override if it has custom thresholds OR is disabled
      const hasOverride = hasCustomThresholds || override?.disabled || false;

      return {
        id: storage.id,
        name: storage.name,
        type: 'storage' as const,
        resourceType: 'Storage',
        node: storage.node,
        instance: storage.instance,
        status: storage.status,
        hasOverride: hasOverride,
        disabled: override?.disabled || false,
        thresholds: override?.thresholds || {},
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

  const storageGroupedByNode = createMemo<Record<string, Resource[]>>(() => {
    const grouped: Record<string, Resource[]> = {};
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

  const summaryItems = createMemo(() => {
    try {
      const items = [
        {
          key: 'nodes' as const,
          label: 'Nodes',
          total: props.nodes?.length ?? 0,
          overrides: countOverrides(nodesWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'dockerHosts' as const,
          label: 'Docker Hosts',
          total: props.dockerHosts?.length ?? 0,
          overrides: countOverrides(dockerHostsWithOverrides()),
          tab: 'docker' as const,
        },
        {
          key: 'storage' as const,
          label: 'Storage',
          total: props.storage?.length ?? 0,
          overrides: countOverrides(storageWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'pbs' as const,
          label: 'PBS Servers',
          total: props.pbsInstances?.length ?? 0,
          overrides: countOverrides(pbsServersWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'dockerContainers' as const,
          label: 'Docker Containers',
          total: totalDockerContainers() ?? 0,
          overrides: countOverrides(dockerContainersFlat()),
          tab: 'docker' as const,
        },
        {
          key: 'guests' as const,
          label: 'VMs & Containers',
          total: props.allGuests?.()?.length ?? 0,
          overrides: countOverrides(guestsFlat()),
          tab: 'proxmox' as const,
        },
      ];

      const filtered = items.filter((item) => item.total > 0 || item.overrides > 0);
      return filtered.filter((item) => item.tab === activeTab());
    } catch (err) {
      console.error('Error in summaryItems memo:', err);
      return [];
    }
  });

  const hasSection = (key: string) => summaryItems()?.some((item) => item.key === key) ?? false;

  const startEditing = (
    resourceId: string,
    currentThresholds: Record<string, number | undefined>,
    defaults: Record<string, number | undefined>,
  ) => {
    setEditingId(resourceId);
    // Merge defaults with overrides for editing
    const mergedThresholds = { ...defaults, ...currentThresholds };
    setEditingThresholds(mergedThresholds);
  };

  const saveEdit = (resourceId: string) => {
    // Flatten grouped guests to find the resource
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const allResources = [
      ...nodesWithOverrides(),
      ...dockerHostsWithOverrides(),
      ...allGuests,
      ...allDockerContainers,
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
    ];
    const resource = allResources.find((r) => r.id === resourceId);
    if (!resource) return;

    const editedThresholds = editingThresholds();
    const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;

    // Only include values that differ from defaults
    const overrideThresholds: Record<string, number> = {};
    Object.keys(editedThresholds).forEach((key) => {
      const editedValue = editedThresholds[key];
      const defaultValue = defaultThresholds[key as keyof typeof defaultThresholds];
      if (editedValue !== undefined && editedValue !== defaultValue) {
        overrideThresholds[key] = editedValue;
      }
    });

    const hasStateOnlyOverride = Boolean(
      resource.disabled ||
        resource.disableConnectivity ||
        resource.poweredOffSeverity !== undefined,
    );

    // If no threshold overrides or state flags remain, remove the override entirely
    if (Object.keys(overrideThresholds).length === 0 && !hasStateOnlyOverride) {
      // If there was an existing override, remove it
      if (resource.hasOverride) {
        const newOverrides = props.overrides().filter((o) => o.id !== resourceId);
        props.setOverrides(newOverrides);

        // Also remove from raw config
        const newRawConfig = { ...props.rawOverridesConfig() };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
      }
      cancelEdit();
      return;
    }

    // Create or update override
    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: resource.disabled,
      disableConnectivity: resource.disableConnectivity,
      poweredOffSeverity: resource.poweredOffSeverity,
      thresholds: overrideThresholds,
    };

    // Update overrides list
    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    // Update raw config
    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const previousRaw = props.rawOverridesConfig()[resourceId];
    const hysteresisThresholds: RawOverrideConfig = {};
    if (previousRaw) {
      if (previousRaw.disabled !== undefined) {
        hysteresisThresholds.disabled = previousRaw.disabled;
      }
      if (previousRaw.disableConnectivity !== undefined) {
        hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
      }
      if (previousRaw.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
      }
    }
    Object.entries(overrideThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = {
          trigger: value,
          clear: Math.max(0, value - 5),
        };
      }
    });
    if (resource.disabled) {
      hysteresisThresholds.disabled = true;
    } else {
      delete hysteresisThresholds.disabled;
    }
    if (resource.disableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
      delete hysteresisThresholds.poweredOffSeverity;
    } else {
      if ((resource.type === 'guest' || resource.type === 'dockerContainer') && props.guestDisableConnectivity()) {
        hysteresisThresholds.disableConnectivity = false;
      } else {
        delete hysteresisThresholds.disableConnectivity;
      }
      if (resource.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = resource.poweredOffSeverity;
      } else {
        delete hysteresisThresholds.poweredOffSeverity;
      }
    }
    newRawConfig[resourceId] = hysteresisThresholds;
    props.setRawOverridesConfig(newRawConfig);

    props.setHasUnsavedChanges(true);
    setEditingId(null);
    setEditingThresholds({});
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditingThresholds({});
  };

  const updateDelay = (key: 'guest' | 'node' | 'storage' | 'pbs', value: number) => {
    const sanitized = Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0;
    const current = props.timeThresholds();
    if (current[key] === sanitized) return;
    props.setTimeThresholds({ ...current, [key]: sanitized });
    props.setHasUnsavedChanges(true);
  };

  const removeOverride = (resourceId: string) => {
    props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

    const newRawConfig = { ...props.rawOverridesConfig() };
    delete newRawConfig[resourceId];
    props.setRawOverridesConfig(newRawConfig);

    props.setHasUnsavedChanges(true);
  };

  const toggleDisabled = (resourceId: string, forceState?: boolean) => {
    // Flatten grouped guests to find the resource
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const allResources = [
      ...allGuests,
      ...allDockerContainers,
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
    ];
    const resource = allResources.find((r) => r.id === resourceId);
    if (
      !resource ||
      (resource.type !== 'guest' &&
        resource.type !== 'storage' &&
        resource.type !== 'pbs' &&
        resource.type !== 'dockerContainer')
    )
      return;

    // Get existing override if it exists
    const existingOverride = props.overrides().find((o) => o.id === resourceId);

    // Determine the current disabled state - check the resource's current state, not the override
    const currentDisabledState = resource.disabled;
    const newDisabledState = forceState !== undefined ? forceState : !currentDisabledState;

    // Clean the thresholds to exclude 'disabled' if it got in there
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;

    // If enabling (disabled = false) and no custom thresholds exist, remove the override entirely
    if (!newDisabledState && (!existingOverride || Object.keys(cleanThresholds).length === 0)) {
      // Remove the override completely
      props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: newDisabledState,
        thresholds: cleanThresholds, // Only keep actual threshold overrides
      };

      const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
      if (existingIndex >= 0) {
        const newOverrides = [...props.overrides()];
        newOverrides[existingIndex] = override;
        props.setOverrides(newOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      // Update raw config
      const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: RawOverrideConfig = {};

      // Only add threshold overrides that differ from defaults
      Object.entries(override.thresholds).forEach(([metric, value]) => {
        if (typeof value === 'number') {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, value - 5),
          };
        }
      });

      if (newDisabledState) {
        hysteresisThresholds.disabled = true;
      }

      newRawConfig[resourceId] = hysteresisThresholds;
      props.setRawOverridesConfig(newRawConfig);
    }

    if (newDisabledState && props.removeAlerts) {
      if (resource.type === 'guest') {
        props.removeAlerts(
          (alert) => alert.resourceId === resourceId && alert.type === 'powered-off',
        );
      } else if (resource.type === 'pbs') {
        const offlineId = `pbs-offline-${resourceId}`;
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.id === offlineId || alert.type === 'offline'),
        );
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
    }

    props.setHasUnsavedChanges(true);
  };

  const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {
    // Find the resource - could be a node, PBS server, or guest
    const nodes = nodesWithOverrides();
    const pbsServers = pbsServersWithOverrides();
    const guests = guestsFlat();
    const dockerHosts = dockerHostsWithOverrides();
    const resource = [...nodes, ...pbsServers, ...guests, ...dockerHosts].find(
      (r) => r.id === resourceId,
    );
    if (
      !resource ||
      (resource.type !== 'node' &&
        resource.type !== 'pbs' &&
        resource.type !== 'guest' &&
        resource.type !== 'dockerHost')
    )
      return;

    // Get existing override if it exists
    const existingOverride = props.overrides().find((o) => o.id === resourceId);

    // Determine the current state - use the resource's computed state, not just the override
    const currentDisableConnectivity = resource.disableConnectivity;
    const newDisableConnectivity =
      forceState !== undefined ? forceState : !currentDisableConnectivity;

    // Clean the thresholds to exclude any unwanted fields
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;
    delete (cleanThresholds as Record<string, unknown>).disableConnectivity;

    // If enabling connectivity alerts (disableConnectivity = false) and no custom thresholds exist, remove the override entirely
    if (!newDisableConnectivity && Object.keys(cleanThresholds).length === 0) {
      // Remove the override completely
      props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      // Update or create the override
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        disableConnectivity: newDisableConnectivity,
        thresholds: cleanThresholds,
      };

      // Update overrides list
      const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
      if (existingIndex >= 0) {
        const newOverrides = [...props.overrides()];
        newOverrides[existingIndex] = override;
        props.setOverrides(newOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      // Update raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: Record<string, any> = {};

      // Add threshold configs
      Object.entries(cleanThresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, (value as number) - 5),
          };
        }
      });

      if (newDisableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      }

      newRawConfig[resourceId] = hysteresisThresholds;
      props.setRawOverridesConfig(newRawConfig);
    }

    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && resource.type === 'dockerHost') {
      const offlineId = `docker-host-offline-${resourceId}`;
      const resourceKey = `docker:${resourceId}`;
      props.removeAlerts(
        (alert) => alert.id === offlineId || alert.resourceId === resourceKey,
      );
    }
  };

  const setOfflineState = (resourceId: string, state: OfflineState) => {
    const guests = guestsFlat();
    const dockerContainers = dockerContainersFlat();
    const resource = [...guests, ...dockerContainers].find((r) => r.id === resourceId);
    if (!resource) return;

    const defaultDisabled = props.guestDisableConnectivity();
    const defaultSeverity = props.guestPoweredOffSeverity();

    const existingOverride = props.overrides().find((o) => o.id === resourceId);
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;
    delete (cleanThresholds as Record<string, unknown>).disableConnectivity;
    delete (cleanThresholds as Record<string, unknown>).poweredOffSeverity;

    const newDisableConnectivity = state === 'off';
    const newSeverity: 'warning' | 'critical' | undefined =
      state === 'off' ? undefined : state === 'critical' ? 'critical' : 'warning';

    const overrideDisabled = existingOverride?.disabled || false;
    const hasThresholds = Object.keys(cleanThresholds).length > 0;

    const differsFromDefaults =
      newDisableConnectivity !== defaultDisabled ||
      (!newDisableConnectivity && newSeverity !== defaultSeverity);

    if (!differsFromDefaults && !hasThresholds && !overrideDisabled && !existingOverride?.disableConnectivity) {
      // Remove override entirely
      if (existingOverride) {
        props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));
        const newRawConfig = { ...props.rawOverridesConfig() };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
      }
      return;
    }

    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: overrideDisabled,
      disableConnectivity: newDisableConnectivity,
      poweredOffSeverity: newDisableConnectivity ? undefined : newSeverity,
      thresholds: cleanThresholds,
    };

    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const hysteresisThresholds: RawOverrideConfig = {};

    Object.entries(cleanThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = {
          trigger: value,
          clear: Math.max(0, value - 5),
        };
      }
    });

    if (overrideDisabled) {
      hysteresisThresholds.disabled = true;
    }

    if (newDisableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
    } else {
      if (defaultDisabled) {
        hysteresisThresholds.disableConnectivity = false;
      }
      if (newSeverity) {
        hysteresisThresholds.poweredOffSeverity = newSeverity;
      }
    }

    if (Object.keys(hysteresisThresholds).length > 0) {
      newRawConfig[resourceId] = hysteresisThresholds;
    } else {
      delete newRawConfig[resourceId];
    }

    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && newDisableConnectivity) {
      if (resource.type === 'guest') {
        props.removeAlerts((alert) => alert.resourceId === resourceId && alert.type === 'powered-off');
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
    }
  };

  return (
    <div class="space-y-6">
      {/* Search Bar */}
      <div class="relative">
        <input
          ref={searchInputRef}
          type="text"
          placeholder="Search resources..."
          value={searchTerm()}
          onInput={(e) => setSearchTerm(e.currentTarget.value)}
          class="w-full pl-10 pr-10 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        />
        <svg
          class="absolute left-3 top-2.5 w-5 h-5 text-gray-400"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <Show when={searchTerm()}>
          <button
            type="button"
            onClick={() => setSearchTerm('')}
            class="absolute right-3 top-2.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </Show>
      </div>

      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700">
        <nav class="-mb-px flex gap-6" aria-label="Tabs">
          <button
            type="button"
            onClick={() => setActiveTab('proxmox')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab() === 'proxmox'
                ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
            }`}
          >
            Proxmox / PBS
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('docker')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab() === 'docker'
                ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
            }`}
          >
            Docker
          </button>
        </nav>
      </div>

      <div class="space-y-6">
        <Show when={activeTab() === 'proxmox'}>
          <Show when={hasSection('nodes')}>
            <div ref={registerSection('nodes')} class="scroll-mt-24">
              <ResourceTable
                title="Proxmox Nodes"
                resources={nodesWithOverrides()}
                columns={['CPU %', 'Memory %', 'Disk %', 'Temp °C']}
                activeAlerts={props.activeAlerts}
                emptyMessage="No nodes match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={props.nodeDefaults}
                setGlobalDefaults={props.setNodeDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllNodes}
                onToggleGlobalDisable={() => props.setDisableAllNodes(!props.disableAllNodes())}
                globalDisableOfflineFlag={props.disableAllNodesOffline}
                onToggleGlobalDisableOffline={() => props.setDisableAllNodesOffline(!props.disableAllNodesOffline())}
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().node}
                onGlobalDelayChange={(value) => updateDelay('node', value)}
              />
            </div>
          </Show>

          <Show when={hasSection('guests')}>
            <div ref={registerSection('guests')} class="scroll-mt-24">
              <ResourceTable
                title="VMs & Containers"
                groupedResources={guestsGroupedByNode()}
                groupHeaderMeta={guestGroupHeaderMeta()}
                columns={['CPU %', 'Memory %', 'Disk %', 'Disk R MB/s', 'Disk W MB/s', 'Net In MB/s', 'Net Out MB/s']}
                activeAlerts={props.activeAlerts}
                emptyMessage="No VMs or containers match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={props.guestDefaults}
                setGlobalDefaults={props.setGuestDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllGuests}
                onToggleGlobalDisable={() => props.setDisableAllGuests(!props.disableAllGuests())}
                globalDisableOfflineFlag={() => props.guestDisableConnectivity()}
                onToggleGlobalDisableOffline={() =>
                  props.setGuestDisableConnectivity(!props.guestDisableConnectivity())
                }
                globalOfflineSeverity={props.guestPoweredOffSeverity()}
                onSetGlobalOfflineState={(state) => {
                  if (state === 'off') {
                    props.setGuestDisableConnectivity(true);
                  } else {
                    props.setGuestDisableConnectivity(false);
                    props.setGuestPoweredOffSeverity(state === 'critical' ? 'critical' : 'warning');
                  }
                  props.setHasUnsavedChanges(true);
                }}
                onSetOfflineState={setOfflineState}
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().guest}
                onGlobalDelayChange={(value) => updateDelay('guest', value)}
              />
            </div>
          </Show>

          <Show when={hasSection('storage')}>
            <div ref={registerSection('storage')} class="scroll-mt-24">
              <ResourceTable
                title="Storage Devices"
                groupedResources={storageGroupedByNode()}
                groupHeaderMeta={guestGroupHeaderMeta()}
                columns={['Usage %']}
                activeAlerts={props.activeAlerts}
                emptyMessage="No storage devices match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                showOfflineAlertsColumn={false}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={{ usage: props.storageDefault() }}
                setGlobalDefaults={(value) => {
                  if (typeof value === 'function') {
                    const newValue = value({ usage: props.storageDefault() });
                    props.setStorageDefault(newValue.usage ?? 85);
                  } else {
                    props.setStorageDefault(value.usage ?? 85);
                  }
                }}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllStorage}
                onToggleGlobalDisable={() => props.setDisableAllStorage(!props.disableAllStorage())}
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().storage}
                onGlobalDelayChange={(value) => updateDelay('storage', value)}
              />
            </div>
          </Show>

          <Show when={hasSection('pbs')}>
            <div ref={registerSection('pbs')} class="scroll-mt-24">
              <ResourceTable
                title="PBS Servers"
                resources={pbsServersWithOverrides()}
                columns={['CPU %', 'Memory %']}
                activeAlerts={props.activeAlerts}
                emptyMessage="No PBS servers match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={{ cpu: props.nodeDefaults.cpu, memory: props.nodeDefaults.memory }}
                setGlobalDefaults={(value) => {
                  if (typeof value === 'function') {
                    const newValue = value({ cpu: props.nodeDefaults.cpu, memory: props.nodeDefaults.memory });
                    props.setNodeDefaults((prev) => ({ ...prev, cpu: newValue.cpu ?? prev.cpu, memory: newValue.memory ?? prev.memory }));
                  } else {
                    props.setNodeDefaults((prev) => ({ ...prev, cpu: value.cpu ?? prev.cpu, memory: value.memory ?? prev.memory }));
                  }
                }}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllPBS}
                onToggleGlobalDisable={() => props.setDisableAllPBS(!props.disableAllPBS())}
                globalDisableOfflineFlag={props.disableAllPBSOffline}
                onToggleGlobalDisableOffline={() => props.setDisableAllPBSOffline(!props.disableAllPBSOffline())}
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().pbs}
                onGlobalDelayChange={(value) => updateDelay('pbs', value)}
              />
            </div>
          </Show>
        </Show>

        <Show when={activeTab() === 'docker'}>
          <Show when={hasSection('dockerHosts')}>
            <div ref={registerSection('dockerHosts')} class="scroll-mt-24">
              <ResourceTable
                title="Docker Hosts"
                resources={dockerHostsWithOverrides()}
                columns={[]}
                activeAlerts={props.activeAlerts}
                emptyMessage="No Docker hosts match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDisableFlag={props.disableAllDockerHosts}
                onToggleGlobalDisable={() => props.setDisableAllDockerHosts(!props.disableAllDockerHosts())}
                globalDisableOfflineFlag={props.disableAllDockerHostsOffline}
                onToggleGlobalDisableOffline={() => props.setDisableAllDockerHostsOffline(!props.disableAllDockerHostsOffline())}
              />
            </div>
          </Show>

          <Show when={hasSection('dockerContainers')}>
            <div ref={registerSection('dockerContainers')} class="scroll-mt-24">
              <ResourceTable
                title="Docker Containers"
                groupedResources={dockerContainersGroupedByHost()}
                groupHeaderMeta={dockerHostGroupMeta()}
                columns={[
                  'CPU %',
                  'Memory %',
                  'Restart Count',
                  'Restart Window (s)',
                  'Memory Warn %',
                  'Memory Critical %',
                ]}
                activeAlerts={props.activeAlerts}
                emptyMessage="No Docker containers match the current filters."
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                showOfflineAlertsColumn={false}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={{
                  cpu: props.dockerDefaults.cpu,
                  memory: props.dockerDefaults.memory,
                  restartCount: props.dockerDefaults.restartCount,
                  restartWindow: props.dockerDefaults.restartWindow,
                  memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                  memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                }}
                setGlobalDefaults={(value) => {
                  const current = {
                    cpu: props.dockerDefaults.cpu,
                    memory: props.dockerDefaults.memory,
                    restartCount: props.dockerDefaults.restartCount,
                    restartWindow: props.dockerDefaults.restartWindow,
                    memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                    memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                  };
                  const next =
                    typeof value === 'function'
                      ? value(current)
                      : { ...current, ...value };

                  props.setDockerDefaults((prev) => ({
                    ...prev,
                    cpu: next.cpu ?? prev.cpu,
                    memory: next.memory ?? prev.memory,
                    restartCount: next.restartCount ?? prev.restartCount,
                    restartWindow: next.restartWindow ?? prev.restartWindow,
                    memoryWarnPct: next.memoryWarnPct ?? prev.memoryWarnPct,
                    memoryCriticalPct: next.memoryCriticalPct ?? prev.memoryCriticalPct,
                  }));
                }}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllDockerContainers}
                onToggleGlobalDisable={() => props.setDisableAllDockerContainers(!props.disableAllDockerContainers())}
                globalDisableOfflineFlag={() => props.guestDisableConnectivity()}
                onToggleGlobalDisableOffline={() =>
                  props.setGuestDisableConnectivity(!props.guestDisableConnectivity())
                }
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().guest}
                onGlobalDelayChange={(value) => updateDelay('guest', value)}
                globalOfflineSeverity={props.guestPoweredOffSeverity()}
                onSetOfflineState={setOfflineState}
              />
            </div>
          </Show>
        </Show>
      </div>
    </div>
  );
}
