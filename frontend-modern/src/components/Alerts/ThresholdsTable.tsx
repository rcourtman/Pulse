import { createSignal, createMemo, Show, onMount, onCleanup } from 'solid-js';
import type { VM, Container, Node, Alert, Storage, PBSInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import { ResourceTable, Resource } from './ResourceTable';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

interface Override {
  id: string;
  name: string;
  type: 'guest' | 'node' | 'storage' | 'pbs';
  resourceType?: string;
  vmid?: number;
  node?: string;
  instance?: string;
  disabled?: boolean;
  disableConnectivity?: boolean; // For nodes only - disable offline alerts
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
  pbsInstances?: PBSInstance[]; // PBS instances from state
  guestDefaults: SimpleThresholds;
  setGuestDefaults: (
    value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>),
  ) => void;
  nodeDefaults: SimpleThresholds;
  setNodeDefaults: (
    value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>),
  ) => void;
  storageDefault: () => number;
  setStorageDefault: (value: number) => void;
  timeThreshold: () => number;
  setTimeThreshold: (value: number) => void;
  timeThresholds: () => { guest: number; node: number; storage: number; pbs: number };
  setTimeThresholds: (value: { guest: number; node: number; storage: number; pbs: number }) => void;
  setHasUnsavedChanges: (value: boolean) => void;
  activeAlerts?: Record<string, Alert>;
}

export function ThresholdsTable(props: ThresholdsTableProps) {
  const [searchTerm, setSearchTerm] = createSignal('');
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<
    Record<string, number | undefined>
  >({});

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
    if (metric === 'cpu' || metric === 'memory' || metric === 'disk' || metric === 'usage') {
      return `${value}%`;
    }

    // Temperature in Celsius
    if (metric === 'temperature') {
      return `${value}°C`;
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
  const nodesWithOverrides = createMemo<Resource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map(props.overrides().map((o) => [o.id, o]));

    const nodes = props.nodes.map((node) => {
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

      return {
        id: node.id,
        name: node.name,
        type: 'node' as const,
        resourceType: 'Node',
        status: node.status,
        hasOverride: hasCustomThresholds || false,
        disabled: false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.nodeDefaults,
      };
    });

    if (search) {
      return nodes.filter((n) => n.name.toLowerCase().includes(search));
    }
    return nodes;
  }, []);

  // Process guests with their overrides and group by node
  const guestsGroupedByNode = createMemo<Record<string, Resource[]>>((prev = {}) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map(props.overrides().map((o) => [o.id, o]));

    const guests = props.allGuests().map((guest) => {
      const guestId = guest.id || `${guest.instance}-${guest.node}-${guest.vmid}`;
      const override = overridesMap.get(guestId);

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

      // A guest has an override if it has custom thresholds OR is disabled
      const hasOverride = hasCustomThresholds || override?.disabled || false;

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
        thresholds: override?.thresholds || {},
        defaults: props.guestDefaults,
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

  // Process PBS servers with their overrides
  const pbsServersWithOverrides = createMemo<Resource[]>((prev = []) => {
    // If we're currently editing, return the previous value to avoid re-renders
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = new Map(props.overrides().map((o) => [o.id, o]));

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
    const overridesMap = new Map(props.overrides().map((o) => [o.id, o]));

    const storageDevices = props.storage.map((storage) => {
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
    const allGuests = Object.values(guestsGroupedByNode()).flat();
    const allResources = [
      ...nodesWithOverrides(),
      ...allGuests,
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

    // If no overrides, just cancel the edit
    if (Object.keys(overrideThresholds).length === 0) {
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
      type: resource.type as 'guest' | 'node' | 'storage' | 'pbs',
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: resource.disabled,
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
    const hysteresisThresholds: RawOverrideConfig = {};
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

  const removeOverride = (resourceId: string) => {
    props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

    const newRawConfig = { ...props.rawOverridesConfig() };
    delete newRawConfig[resourceId];
    props.setRawOverridesConfig(newRawConfig);

    props.setHasUnsavedChanges(true);
  };

  const toggleDisabled = (resourceId: string, forceState?: boolean) => {
    // Flatten grouped guests to find the resource
    const allGuests = Object.values(guestsGroupedByNode()).flat();
    const allResources = [...allGuests, ...storageWithOverrides(), ...pbsServersWithOverrides()];
    const resource = allResources.find((r) => r.id === resourceId);
    if (
      !resource ||
      (resource.type !== 'guest' && resource.type !== 'storage' && resource.type !== 'pbs')
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

    props.setHasUnsavedChanges(true);
  };

  const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {
    // Find the resource - could be a node or PBS server
    const nodes = nodesWithOverrides();
    const pbsServers = pbsServersWithOverrides();
    const resource = [...nodes, ...pbsServers].find((r) => r.id === resourceId);
    if (!resource || (resource.type !== 'node' && resource.type !== 'pbs')) return;

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
        type: resource.type as 'node' | 'guest' | 'storage',
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
  };

  return (
    <div class="space-y-6">
      {/* Global Settings Section */}
      <Card padding="none">
        <div class="p-4">
          <SectionHeader
            title="Global default thresholds"
            description="Default thresholds that apply to all resources unless overridden"
            size="md"
          />
        </div>

        <div class="border-t border-gray-200 dark:border-gray-700 p-4 space-y-4">
          {/* Threshold inputs in a responsive layout */}
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
              Default thresholds for all resources. Individual resources can override these values
              below.
              <span class="ml-2 text-blue-600 dark:text-blue-400">
                Enter 0 or -1 to disable specific alerts.
              </span>
            </p>
            <div class="grid gap-4 md:grid-cols-2">
              <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 p-3">
                <h4 class="text-sm font-medium text-gray-700 dark:text-gray-200 mb-3">
                  VMs & Containers
                </h4>
                <div class="grid gap-3 sm:grid-cols-2">
                  <div class="space-y-1">
                    <label
                      for="guest-cpu"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>CPU</span>
                      <span class="text-[10px] font-normal text-gray-400">%</span>
                    </label>
                    <input
                      id="guest-cpu"
                      type="number"
                      min="-1"
                      max="100"
                      value={props.guestDefaults.cpu ?? 0}
                      title="Enter 0 or -1 to disable CPU alerts"
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setGuestDefaults((prev) => ({
                          ...prev,
                          cpu: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="guest-memory"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Memory</span>
                      <span class="text-[10px] font-normal text-gray-400">%</span>
                    </label>
                    <input
                      id="guest-memory"
                      type="number"
                      min="-1"
                      max="100"
                      value={props.guestDefaults.memory ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setGuestDefaults((prev) => ({
                          ...prev,
                          memory: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="guest-disk"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Disk</span>
                      <span class="text-[10px] font-normal text-gray-400">%</span>
                    </label>
                    <input
                      id="guest-disk"
                      type="number"
                      min="-1"
                      max="100"
                      value={props.guestDefaults.disk ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setGuestDefaults((prev) => ({
                          ...prev,
                          disk: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="guest-disk-read"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Disk Read</span>
                      <span class="text-[10px] font-normal text-gray-400">MB/s</span>
                    </label>
                    <input
                      id="guest-disk-read"
                      type="number"
                      min="-1"
                      max="10000"
                      value={props.guestDefaults.diskRead ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setGuestDefaults((prev) => ({
                          ...prev,
                          diskRead: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="guest-disk-write"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Disk Write</span>
                      <span class="text-[10px] font-normal text-gray-400">MB/s</span>
                    </label>
                    <input
                      id="guest-disk-write"
                      type="number"
                      min="-1"
                      max="10000"
                      value={props.guestDefaults.diskWrite ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setGuestDefaults((prev) => ({
                          ...prev,
                          diskWrite: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="guest-network-in"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Net In</span>
                      <span class="text-[10px] font-normal text-gray-400">MB/s</span>
                    </label>
                    <input
                      id="guest-network-in"
                      type="number"
                      min="-1"
                      max="10000"
                      value={props.guestDefaults.networkIn ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setGuestDefaults((prev) => ({
                          ...prev,
                          networkIn: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="guest-network-out"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Net Out</span>
                      <span class="text-[10px] font-normal text-gray-400">MB/s</span>
                    </label>
                    <input
                      id="guest-network-out"
                      type="number"
                      min="-1"
                      max="10000"
                      value={props.guestDefaults.networkOut ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setGuestDefaults((prev) => ({
                          ...prev,
                          networkOut: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                </div>
              </div>
              <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 p-3">
                <h4 class="text-sm font-medium text-gray-700 dark:text-gray-200 mb-3">
                  Proxmox Nodes
                </h4>
                <div class="grid gap-3 sm:grid-cols-2">
                  <div class="space-y-1">
                    <label
                      for="node-cpu"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>CPU</span>
                      <span class="text-[10px] font-normal text-gray-400">%</span>
                    </label>
                    <input
                      id="node-cpu"
                      type="number"
                      min="-1"
                      max="100"
                      value={props.nodeDefaults.cpu ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setNodeDefaults((prev) => ({
                          ...prev,
                          cpu: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="node-memory"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Memory</span>
                      <span class="text-[10px] font-normal text-gray-400">%</span>
                    </label>
                    <input
                      id="node-memory"
                      type="number"
                      min="-1"
                      max="100"
                      value={props.nodeDefaults.memory ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setNodeDefaults((prev) => ({
                          ...prev,
                          memory: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="node-disk"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Disk</span>
                      <span class="text-[10px] font-normal text-gray-400">%</span>
                    </label>
                    <input
                      id="node-disk"
                      type="number"
                      min="-1"
                      max="100"
                      value={props.nodeDefaults.disk ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setNodeDefaults((prev) => ({
                          ...prev,
                          disk: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                  <div class="space-y-1">
                    <label
                      for="node-temperature"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Temperature</span>
                      <span class="text-[10px] font-normal text-gray-400">°C</span>
                    </label>
                    <input
                      id="node-temperature"
                      type="number"
                      min="-1"
                      max="150"
                      value={props.nodeDefaults.temperature ?? 0}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setNodeDefaults((prev) => ({
                          ...prev,
                          temperature: Number.isNaN(value) ? 0 : value,
                        }));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                </div>
              </div>
              <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 p-3 md:col-span-2">
                <h4 class="text-sm font-medium text-gray-700 dark:text-gray-200 mb-3">
                  Storage
                </h4>
                <div class="grid gap-3 sm:grid-cols-3 lg:grid-cols-4">
                  <div class="space-y-1">
                    <label
                      for="storage-usage"
                      class="text-xs font-medium text-gray-600 dark:text-gray-300 flex items-center justify-between"
                    >
                      <span>Usage</span>
                      <span class="text-[10px] font-normal text-gray-400">%</span>
                    </label>
                    <input
                      id="storage-usage"
                      type="number"
                      min="-1"
                      max="100"
                      value={props.storageDefault()}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        props.setStorageDefault(Number.isNaN(value) ? 0 : value);
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-1.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                               bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
          {/* Alert delay settings per resource type */}
          <div class="pt-3 border-t border-gray-200 dark:border-gray-700">
            <div class="mb-2">
              <h4 class="text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">
                Alert Delay (seconds above threshold before triggering)
              </h4>
              <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
                <div class="flex items-center gap-2">
                  <label class="text-xs text-gray-500 dark:text-gray-400 min-w-[80px]">
                    VMs/Containers:
                  </label>
                  <input
                    type="number"
                    min="0"
                    max="300"
                    value={props.timeThresholds().guest}
                    onInput={(e) => {
                      props.setTimeThresholds({
                        ...props.timeThresholds(),
                        guest: parseInt(e.currentTarget.value) || 0,
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class="w-14 px-1 py-0.5 text-xs text-center border border-gray-300 dark:border-gray-600 rounded
                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                  />
                </div>
                <div class="flex items-center gap-2">
                  <label class="text-xs text-gray-500 dark:text-gray-400 min-w-[80px]">
                    Nodes:
                  </label>
                  <input
                    type="number"
                    min="0"
                    max="300"
                    value={props.timeThresholds().node}
                    onInput={(e) => {
                      props.setTimeThresholds({
                        ...props.timeThresholds(),
                        node: parseInt(e.currentTarget.value) || 0,
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class="w-14 px-1 py-0.5 text-xs text-center border border-gray-300 dark:border-gray-600 rounded
                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                  />
                </div>
                <div class="flex items-center gap-2">
                  <label class="text-xs text-gray-500 dark:text-gray-400 min-w-[80px]">
                    Storage:
                  </label>
                  <input
                    type="number"
                    min="0"
                    max="300"
                    value={props.timeThresholds().storage}
                    onInput={(e) => {
                      props.setTimeThresholds({
                        ...props.timeThresholds(),
                        storage: parseInt(e.currentTarget.value) || 0,
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class="w-14 px-1 py-0.5 text-xs text-center border border-gray-300 dark:border-gray-600 rounded
                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                  />
                </div>
                <div class="flex items-center gap-2">
                  <label class="text-xs text-gray-500 dark:text-gray-400 min-w-[80px]">PBS:</label>
                  <input
                    type="number"
                    min="0"
                    max="300"
                    value={props.timeThresholds().pbs}
                    onInput={(e) => {
                      props.setTimeThresholds({
                        ...props.timeThresholds(),
                        pbs: parseInt(e.currentTarget.value) || 0,
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class="w-14 px-1 py-0.5 text-xs text-center border border-gray-300 dark:border-gray-600 rounded
                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                  />
                </div>
              </div>
            </div>

            {/* Reset button row */}
            <div class="flex justify-end pt-2">
              <button
                onClick={() => {
                  props.setGuestDefaults({
                    cpu: 80,
                    memory: 85,
                    disk: 90,
                    diskRead: 150,
                    diskWrite: 150,
                    networkIn: 200,
                    networkOut: 200,
                  });
                  props.setNodeDefaults({
                    cpu: 80,
                    memory: 85,
                    disk: 90,
                    temperature: 80,
                  });
                  props.setStorageDefault(85);
                  props.setTimeThreshold(0);
                  props.setTimeThresholds({ guest: 10, node: 15, storage: 30, pbs: 30 });
                  props.setHasUnsavedChanges(true);
                }}
                class="flex items-center gap-1 px-2 py-0.5 text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded transition-colors"
                title="Reset all values to factory defaults"
              >
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                  />
                </svg>
                Reset defaults
              </button>
            </div>
          </div>
        </div>
      </Card>

      {/* Search Bar */}
      <div class="relative">
        <input
          ref={searchInputRef}
          type="text"
          placeholder="Search resources..."
          value={searchTerm()}
          onInput={(e) => setSearchTerm(e.currentTarget.value)}
          class="w-full px-4 py-2 pl-10 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        />
        <svg
          class="absolute left-3 top-2.5 w-4 h-4 text-gray-400"
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

      {/* Nodes Table */}
      <Show when={nodesWithOverrides().length > 0}>
        <ResourceTable
          title="Proxmox Nodes"
          resources={nodesWithOverrides()}
          columns={['CPU %', 'Memory %', 'Disk %']}
          activeAlerts={props.activeAlerts}
          onEdit={startEditing}
          onSaveEdit={saveEdit}
          onCancelEdit={cancelEdit}
          onRemoveOverride={removeOverride}
          onToggleNodeConnectivity={toggleNodeConnectivity}
          editingId={editingId}
          editingThresholds={editingThresholds}
          setEditingThresholds={setEditingThresholds}
          formatMetricValue={formatMetricValue}
          hasActiveAlert={hasActiveAlert}
        />
      </Show>

      {/* PBS Servers Table */}
      <Show when={pbsServersWithOverrides().length > 0}>
        <ResourceTable
          title="PBS Servers"
          resources={pbsServersWithOverrides()}
          columns={['CPU %', 'Memory %']}
          activeAlerts={props.activeAlerts}
          onEdit={startEditing}
          onSaveEdit={saveEdit}
          onCancelEdit={cancelEdit}
          onRemoveOverride={removeOverride}
          onToggleNodeConnectivity={toggleNodeConnectivity}
          editingId={editingId}
          editingThresholds={editingThresholds}
          setEditingThresholds={setEditingThresholds}
          formatMetricValue={formatMetricValue}
          hasActiveAlert={hasActiveAlert}
        />
      </Show>

      {/* Guests Table */}
      <Show when={Object.keys(guestsGroupedByNode()).length > 0}>
        <ResourceTable
          title="VMs & Containers"
          groupedResources={guestsGroupedByNode()}
          columns={[
            'CPU %',
            'Memory %',
            'Disk %',
            'Disk R MB/s',
            'Disk W MB/s',
            'Net In MB/s',
            'Net Out MB/s',
          ]}
          activeAlerts={props.activeAlerts}
          onEdit={startEditing}
          onSaveEdit={saveEdit}
          onCancelEdit={cancelEdit}
          onRemoveOverride={removeOverride}
          onToggleDisabled={toggleDisabled}
          editingId={editingId}
          editingThresholds={editingThresholds}
          setEditingThresholds={setEditingThresholds}
          formatMetricValue={formatMetricValue}
          hasActiveAlert={hasActiveAlert}
        />
      </Show>

      {/* Storage Table */}
      <Show when={storageWithOverrides().length > 0}>
        <ResourceTable
          title="Storage Devices"
          resources={storageWithOverrides()}
          columns={['Usage %']}
          activeAlerts={props.activeAlerts}
          onEdit={startEditing}
          onSaveEdit={saveEdit}
          onCancelEdit={cancelEdit}
          onRemoveOverride={removeOverride}
          onToggleDisabled={toggleDisabled}
          editingId={editingId}
          editingThresholds={editingThresholds}
          setEditingThresholds={setEditingThresholds}
          formatMetricValue={formatMetricValue}
          hasActiveAlert={hasActiveAlert}
        />
      </Show>
    </div>
  );
}
