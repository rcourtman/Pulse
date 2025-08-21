import { createSignal, createMemo, For, Show, onMount, onCleanup } from 'solid-js';
import type { VM, Container, Node, Alert, Storage } from '@/types/api';

interface Override {
  id: string;
  name: string;
  type: 'guest' | 'node' | 'storage';
  resourceType?: string;
  vmid?: number;
  node?: string;
  instance?: string;
  disabled?: boolean;
  disableConnectivity?: boolean;  // For nodes only - disable offline alerts
  thresholds: {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
    usage?: number;  // For storage devices
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
  [key: string]: number | undefined;  // Add index signature for compatibility
}

interface ThresholdsTableProps {
  overrides: () => Override[];
  setOverrides: (overrides: Override[]) => void;
  rawOverridesConfig: () => Record<string, any>;
  setRawOverridesConfig: (config: Record<string, any>) => void;
  allGuests: () => (VM | Container)[];
  nodes: Node[];
  storage: Storage[];
  guestDefaults: SimpleThresholds;
  setGuestDefaults: (value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>)) => void;
  nodeDefaults: SimpleThresholds;
  setNodeDefaults: (value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>)) => void;
  storageDefault: () => number;
  setStorageDefault: (value: number) => void;
  timeThreshold: () => number;
  setTimeThreshold: (value: number) => void;
  setHasUnsavedChanges: (value: boolean) => void;
  activeAlerts?: Record<string, Alert>;
}

export function ThresholdsTable(props: ThresholdsTableProps) {
  const [searchTerm, setSearchTerm] = createSignal('');
  const [showGlobalSettings, setShowGlobalSettings] = createSignal(true);
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<Record<string, any>>({});
  
  let searchInputRef: HTMLInputElement | undefined;
  
  // Set up keyboard shortcuts
  onMount(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Skip if user is typing in an input or textarea (unless it's Escape)
      const target = e.target as HTMLElement;
      const isInInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.contentEditable === 'true';
      
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
    
    // Percentage-based metrics
    if (metric === 'cpu' || metric === 'memory' || metric === 'disk' || metric === 'usage') {
      return `${value}%`;
    }
    
    // MB/s metrics - show "Off" for 0 values
    if (metric === 'diskRead' || metric === 'diskWrite' || metric === 'networkIn' || metric === 'networkOut') {
      return value === 0 ? 'Off' : `${value} MB/s`;
    }
    
    return String(value);
  };
  
  // Check if there's an active alert for a resource/metric
  const hasActiveAlert = (resourceId: string, metric: string): boolean => {
    if (!props.activeAlerts) return false;
    const alertKey = `${resourceId}-${metric}`;
    return alertKey in props.activeAlerts;
  };
  
  // Component for metric value with active alert indicator
  const MetricValueWithHeat = (props: { 
    resourceId: string; 
    metric: string; 
    value: number; 
    isOverridden: boolean 
  }) => (
    <div class="flex items-center justify-center gap-1">
      <span class={`text-sm ${
        props.isOverridden 
          ? 'text-gray-900 dark:text-gray-100 font-medium' 
          : 'text-gray-400 dark:text-gray-500'
      }`}>
        {formatMetricValue(props.metric, props.value)}
      </span>
      <Show when={hasActiveAlert(props.resourceId, props.metric)}>
        <div 
          class="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse"
          title="Active alert"
        />
      </Show>
    </div>
  );
  
  // Combine all resources (guests and nodes) with their overrides
  const resourcesWithOverrides = createMemo(() => {
    const search = searchTerm().toLowerCase();
    const overridesMap = new Map(props.overrides().map(o => [o.id, o]));
    
    // Process guests
    const guests = props.allGuests().map(guest => {
      const guestId = guest.id || `${guest.instance}-${guest.name}-${guest.vmid}`;
      const override = overridesMap.get(guestId);
      
      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds = override?.thresholds && 
        Object.keys(override.thresholds).some(key => {
          const k = key as keyof typeof override.thresholds;
          return override.thresholds[k] !== undefined && 
                 override.thresholds[k] !== (props.guestDefaults as any)[k];
        });
      
      return {
        id: guestId,
        name: guest.name,
        type: 'guest' as const,
        resourceType: guest.type === 'qemu' ? 'VM' : 'CT',
        vmid: guest.vmid,
        node: guest.node,
        instance: guest.instance,
        status: guest.status,
        hasOverride: hasCustomThresholds || false,  // Only true if thresholds differ
        disabled: override?.disabled || false,
        thresholds: override?.thresholds || {},
        defaults: props.guestDefaults
      };
    });
    
    // Process nodes
    const nodes = props.nodes.map(node => {
      const override = overridesMap.get(node.id);
      
      // Check if any threshold values actually differ from defaults
      const hasCustomThresholds = override?.thresholds && 
        Object.keys(override.thresholds).some(key => {
          const k = key as keyof typeof override.thresholds;
          return override.thresholds[k] !== undefined && 
                 override.thresholds[k] !== (props.nodeDefaults as any)[k];
        });
      
      return {
        id: node.id,
        name: node.name,
        type: 'node' as const,
        resourceType: 'Node',
        status: node.status,
        hasOverride: hasCustomThresholds || false,  // Only true if thresholds differ
        disabled: false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: override?.thresholds || {},
        defaults: props.nodeDefaults
      };
    });
    
    // Process storage devices
    const storageDevices = props.storage.map(storage => {
      const override = overridesMap.get(storage.id);
      
      // Storage only has usage threshold
      const hasCustomThresholds = override?.thresholds?.usage !== undefined && 
                                   override.thresholds.usage !== props.storageDefault();
      
      return {
        id: storage.id,
        name: storage.name,
        type: 'storage' as const,
        resourceType: 'Storage',
        node: storage.node,
        instance: storage.instance,
        status: storage.status,
        hasOverride: hasCustomThresholds || false,
        disabled: override?.disabled || false,
        thresholds: override?.thresholds || {},
        defaults: { usage: props.storageDefault() }
      };
    });
    
    // Combine and filter
    const allResources = [...guests, ...nodes, ...storageDevices];
    
    if (search) {
      return allResources.filter(r => 
        r.name.toLowerCase().includes(search) ||
        ('vmid' in r && r.vmid && r.vmid.toString().includes(search)) ||
        ('node' in r && r.node && r.node.toLowerCase().includes(search))
      );
    }
    
    return allResources;
  });
  
  // Group resources by node for display
  const groupedResources = createMemo(() => {
    const groups: Record<string, any[]> = {};
    
    resourcesWithOverrides().forEach(resource => {
      let groupKey: string;
      if (resource.type === 'node') {
        groupKey = 'Nodes';
      } else if (resource.type === 'storage') {
        groupKey = 'Storage';
      } else {
        // Group all guests together, but we'll show the node within the row
        groupKey = 'Guests';
      }
      
      if (!groups[groupKey]) {
        groups[groupKey] = [];
      }
      groups[groupKey].push(resource);
    });
    
    // Sort resources within each group
    Object.keys(groups).forEach(key => {
      groups[key] = groups[key].sort((a, b) => {
        // For nodes, just sort by name
        if (a.type === 'node' && b.type === 'node') {
          return a.name.localeCompare(b.name);
        }
        
        // For guests, sort by node first, then by vmid
        if (a.type === 'guest' && b.type === 'guest') {
          const nodeCompare = (a.node || '').localeCompare(b.node || '');
          if (nodeCompare !== 0) return nodeCompare;
          if ('vmid' in a && 'vmid' in b && a.vmid && b.vmid) return a.vmid - b.vmid;
          return a.name.localeCompare(b.name);
        }
        
        // For storage, sort by node first, then by name
        if (a.type === 'storage' && b.type === 'storage') {
          const nodeCompare = (a.node || '').localeCompare(b.node || '');
          if (nodeCompare !== 0) return nodeCompare;
          return a.name.localeCompare(b.name);
        }
        
        // Default comparison
        return a.name.localeCompare(b.name);
      });
    });
    
    return groups;
  });
  
  const startEditing = (resourceId: string, currentThresholds: any, defaults: any) => {
    setEditingId(resourceId);
    // Merge defaults with overrides for editing
    const mergedThresholds = { ...defaults, ...currentThresholds };
    setEditingThresholds(mergedThresholds);
  };
  
  const saveEdit = (resourceId: string) => {
    const resource = resourcesWithOverrides().find(r => r.id === resourceId);
    if (!resource) return;
    
    const editedThresholds = editingThresholds();
    const defaultThresholds = resource.defaults;
    
    // Only include values that differ from defaults
    const overrideThresholds: Record<string, any> = {};
    Object.keys(editedThresholds).forEach(key => {
      const editedValue = editedThresholds[key];
      const defaultValue = defaultThresholds[key as keyof typeof defaultThresholds];
      if (editedValue !== defaultValue && editedValue !== undefined && editedValue !== '') {
        overrideThresholds[key] = editedValue;
      }
    });
    
    // If no overrides, just cancel the edit
    if (Object.keys(overrideThresholds).length === 0) {
      // If there was an existing override, remove it
      if (resource.hasOverride) {
        const newOverrides = props.overrides().filter(o => o.id !== resourceId);
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
      type: resource.type as 'guest' | 'node' | 'storage',
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: resource.disabled,
      thresholds: overrideThresholds
    };
    
    // Update overrides list
    const existingIndex = props.overrides().findIndex(o => o.id === resourceId);
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
    Object.entries(overrideThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = { 
          trigger: value, 
          clear: Math.max(0, (value as number) - 5) 
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
    props.setOverrides(props.overrides().filter(o => o.id !== resourceId));
    
    const newRawConfig = { ...props.rawOverridesConfig() };
    delete newRawConfig[resourceId];
    props.setRawOverridesConfig(newRawConfig);
    
    props.setHasUnsavedChanges(true);
  };
  
  const toggleDisabled = (resourceId: string) => {
    const resource = resourcesWithOverrides().find(r => r.id === resourceId);
    if (!resource || resource.type !== 'guest') return;
    
    // Get existing override if it exists
    const existingOverride = props.overrides().find(o => o.id === resourceId);
    
    console.log('Toggle disabled for:', resourceId);
    console.log('Existing override:', existingOverride);
    console.log('Existing thresholds:', existingOverride?.thresholds);
    console.log('Threshold keys:', existingOverride ? Object.keys(existingOverride.thresholds || {}) : 'no override');
    
    // Determine the current disabled state from the override (or false if no override)
    const currentDisabledState = existingOverride?.disabled || false;
    const newDisabledState = !currentDisabledState;
    
    console.log('Current disabled state:', currentDisabledState);
    console.log('New disabled state:', newDisabledState);
    
    // Clean the thresholds to exclude 'disabled' if it got in there
    const cleanThresholds: any = { ...(existingOverride?.thresholds || {}) };
    delete cleanThresholds.disabled;
    
    // If enabling (disabled = false) and no custom thresholds exist, remove the override entirely
    if (!newDisabledState && (!existingOverride || Object.keys(cleanThresholds).length === 0)) {
      console.log('REMOVING OVERRIDE - enabling with no custom thresholds');
      // Remove the override completely
      props.setOverrides(props.overrides().filter(o => o.id !== resourceId));
      
      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      console.log('UPDATING OVERRIDE - either disabling or has custom thresholds');
      
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: newDisabledState,
        thresholds: cleanThresholds  // Only keep actual threshold overrides
      };
      
      const existingIndex = props.overrides().findIndex(o => o.id === resourceId);
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
      
      // Only add threshold overrides that differ from defaults
      Object.entries(override.thresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = { 
            trigger: value, 
            clear: Math.max(0, (value as number) - 5) 
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
  
  const toggleNodeConnectivity = (nodeId: string) => {
    console.log('toggleNodeConnectivity called for:', nodeId);
    const node = resourcesWithOverrides().find(r => r.id === nodeId);
    console.log('Found node:', node);
    if (!node || node.type !== 'node') return;
    
    // Get existing override if it exists
    const existingOverride = props.overrides().find(o => o.id === nodeId);
    console.log('Existing override:', existingOverride);
    
    // Determine the current state
    const currentDisableConnectivity = existingOverride?.disableConnectivity || false;
    const newDisableConnectivity = !currentDisableConnectivity;
    console.log('Current state:', currentDisableConnectivity, 'New state:', newDisableConnectivity);
    
    // Clean the thresholds to exclude any unwanted fields
    const cleanThresholds: any = { ...(existingOverride?.thresholds || {}) };
    delete cleanThresholds.disabled;
    delete cleanThresholds.disableConnectivity;
    
    // If enabling connectivity alerts (disableConnectivity = false) and no custom thresholds exist, remove the override entirely
    if (!newDisableConnectivity && Object.keys(cleanThresholds).length === 0) {
      // Remove the override completely
      props.setOverrides(props.overrides().filter(o => o.id !== nodeId));
      
      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[nodeId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      // Update or create the override
      const override: Override = {
        id: nodeId,
        name: node.name,
        type: node.type,
        resourceType: node.resourceType,
        disableConnectivity: newDisableConnectivity,
        thresholds: cleanThresholds
      };
      
      // Update overrides list
      const existingIndex = props.overrides().findIndex(o => o.id === nodeId);
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
            clear: Math.max(0, (value as number) - 5) 
          };
        }
      });
      
      if (newDisableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      }
      
      newRawConfig[nodeId] = hysteresisThresholds;
      props.setRawOverridesConfig(newRawConfig);
    }
    
    props.setHasUnsavedChanges(true);
  };
  
  return (
    <div class="space-y-6">
      {/* Global Settings Section */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm">
        <div 
          class="p-4 flex items-center justify-between cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-900/50"
          onClick={() => setShowGlobalSettings(!showGlobalSettings())}
        >
          <div>
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Global Default Thresholds</h3>
            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
              Default thresholds that apply to all resources unless overridden
            </p>
          </div>
          <svg 
            class={`w-5 h-5 text-gray-500 transition-transform ${showGlobalSettings() ? 'rotate-180' : ''}`}
            fill="none" 
            stroke="currentColor" 
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
        
        <Show when={showGlobalSettings()}>
          <div class="border-t border-gray-200 dark:border-gray-700 p-4 space-y-4">
            {/* Threshold table */}
            <div>
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
                Default thresholds for all resources. Individual resources can override these values below.
              </p>
              <div class="overflow-x-auto">
                <table class="w-full text-sm">
                  <thead>
                    <tr class="border-b border-gray-200 dark:border-gray-700">
                      <th class="text-left py-2 px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Resource Type</th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">CPU<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">%</span></th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Memory<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">%</span></th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Disk<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">%</span></th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Storage<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">%</span></th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Disk Read<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">MB/s</span></th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Disk Write<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">MB/s</span></th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Net In<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">MB/s</span></th>
                      <th class="text-center px-3 text-xs font-medium text-gray-600 dark:text-gray-400">Net Out<br/><span class="text-[10px] font-normal text-gray-500 dark:text-gray-500">MB/s</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr class="border-b border-gray-100 dark:border-gray-700/50">
                      <td class="py-3 px-3 font-medium text-gray-700 dark:text-gray-300 text-sm">VMs & Containers</td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.guestDefaults.cpu || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, cpu: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.guestDefaults.memory || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, memory: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.guestDefaults.disk || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, disk: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="10000"
                          value={props.guestDefaults.diskRead || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, diskRead: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="10000"
                          value={props.guestDefaults.diskWrite || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, diskWrite: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="10000"
                          value={props.guestDefaults.networkIn || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, networkIn: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="10000"
                          value={props.guestDefaults.networkOut || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, networkOut: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                    </tr>
                    <tr class="border-b border-gray-100 dark:border-gray-700/50">
                      <td class="py-3 px-3 font-medium text-gray-700 dark:text-gray-300 text-sm">Proxmox Nodes</td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.nodeDefaults.cpu || 0}
                          onInput={(e) => {
                            props.setNodeDefaults((prev) => ({...prev, cpu: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.nodeDefaults.memory || 0}
                          onInput={(e) => {
                            props.setNodeDefaults((prev) => ({...prev, memory: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.nodeDefaults.disk || 0}
                          onInput={(e) => {
                            props.setNodeDefaults((prev) => ({...prev, disk: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                    </tr>
                    <tr>
                      <td class="py-3 px-3 font-medium text-gray-700 dark:text-gray-300 text-sm">Storage</td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.storageDefault()}
                          onInput={(e) => {
                            props.setStorageDefault(parseInt(e.currentTarget.value) || 0);
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-16 px-2 py-1 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                      <td class="text-center px-3 py-3"><span class="text-gray-400 dark:text-gray-500">-</span></td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            
            {/* Additional settings row */}
            <div class="flex items-center justify-between pt-2 border-t border-gray-200 dark:border-gray-700">
              <div class="flex items-center gap-2">
                <label class="text-xs text-gray-500 dark:text-gray-400">
                  Alert delay:
                </label>
                <input
                  type="number"
                  min="0"
                  max="300"
                  value={props.timeThreshold()}
                  onInput={(e) => {
                    props.setTimeThreshold(parseInt(e.currentTarget.value) || 0);
                    props.setHasUnsavedChanges(true);
                  }}
                  class="w-14 px-1 py-0.5 text-xs border border-gray-300 dark:border-gray-600 rounded
                         bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                />
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  seconds before alerting
                </span>
              </div>
              
              <button 
                onClick={() => {
                  props.setGuestDefaults({
                    cpu: 80,
                    memory: 85,
                    disk: 90,
                    diskRead: 150,
                    diskWrite: 150,
                    networkIn: 200,
                    networkOut: 200
                  });
                  props.setNodeDefaults({
                    cpu: 80,
                    memory: 85,
                    disk: 90
                  });
                  props.setStorageDefault(85);
                  props.setTimeThreshold(0);
                  props.setHasUnsavedChanges(true);
                }}
                class="flex items-center gap-1 px-2 py-0.5 text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded transition-colors"
                title="Reset all values to factory defaults"
              >
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Reset defaults
              </button>
            </div>
          </div>
        </Show>
      </div>
      
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
        <svg class="absolute left-3 top-2.5 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <Show when={searchTerm()}>
          <button
            onClick={() => setSearchTerm('')}
            class="absolute right-3 top-2.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </Show>
      </div>
      
      {/* Resources Table */}
      <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Resource
                </th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Type
                </th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  CPU %
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Memory %
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Disk %
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Storage %
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Disk R<br/><span class="text-[10px] font-normal">MB/s</span>
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Disk W<br/><span class="text-[10px] font-normal">MB/s</span>
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Net In<br/><span class="text-[10px] font-normal">Mbps</span>
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Net Out<br/><span class="text-[10px] font-normal">Mbps</span>
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Alerts
                </th>
                <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <Show when={Object.keys(groupedResources()).length === 0} fallback={
                <For each={Object.entries(groupedResources()).sort(([a], [b]) => {
                  // Define the order: Nodes, Guests, Storage
                  const order = ['Nodes', 'Guests', 'Storage'];
                  const aIndex = order.indexOf(a);
                  const bIndex = order.indexOf(b);
                  
                  // If both are in the order array, sort by their position
                  if (aIndex !== -1 && bIndex !== -1) {
                    return aIndex - bIndex;
                  }
                  
                  // If only one is in the order array, it comes first
                  if (aIndex !== -1) return -1;
                  if (bIndex !== -1) return 1;
                  
                  // Otherwise, sort alphabetically
                  return a.localeCompare(b);
                })}>
                  {([groupName, resources]) => (
                    <>
                      {/* Group header */}
                      <tr class="bg-gray-50 dark:bg-gray-700/50">
                        <td colspan="13" class="px-3 py-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                          {groupName}
                        </td>
                      </tr>
                      {/* Resource rows */}
                      <For each={resources}>
                        {(resource) => {
                          const isEditing = () => editingId() === resource.id;
                          const thresholds = () => isEditing() ? editingThresholds() : resource.thresholds;
                          const displayValue = (metric: string) => {
                            if (isEditing()) return thresholds()[metric] || resource.defaults[metric] || '';
                            // Show override value or default
                            return resource.thresholds[metric] || resource.defaults[metric] || 0;
                          };
                          const shouldShowMetric = (metric: string) => {
                            // Nodes don't have I/O metrics
                            if (resource.type === 'node' && 
                                (metric === 'diskRead' || metric === 'diskWrite' || 
                                 metric === 'networkIn' || metric === 'networkOut')) {
                              return false;
                            }
                            // Storage only has usage metric
                            if (resource.type === 'storage') {
                              return metric === 'usage';
                            }
                            return true;
                          };
                          const isOverridden = (metric: string) => {
                            return resource.thresholds[metric] !== undefined && resource.thresholds[metric] !== null;
                          };
                          
                          return (
                            <tr class={`hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors ${resource.disabled ? 'opacity-40' : ''}`}>
                              <td class="px-3 py-1.5">
                                <div class="flex items-center gap-2">
                                  <span class={`text-sm font-medium ${resource.disabled ? 'text-gray-500 dark:text-gray-500' : 'text-gray-900 dark:text-gray-100'}`}>
                                    {resource.name}
                                  </span>
                                  <Show when={'vmid' in resource && resource.vmid}>
                                    <span class="text-xs text-gray-500">({resource.vmid})</span>
                                  </Show>
                                  <Show when={(resource.type === 'guest' || resource.type === 'storage') && 'node' in resource && resource.node}>
                                    <span class="text-xs text-gray-500">on {resource.node}</span>
                                  </Show>
                                  <Show when={(() => {
                                    const override = props.overrides().find(o => o.id === resource.id);
                                    if (!override) return false;
                                    // Show badge if there are threshold overrides or connectivity is disabled for nodes
                                    return Object.keys(override.thresholds).length > 0 || 
                                           (resource.type === 'node' && resource.disableConnectivity);
                                  })()}>
                                    <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                                      Custom
                                    </span>
                                  </Show>
                                </div>
                              </td>
                              <td class="px-3 py-1.5">
                                <span class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
                                  resource.type === 'node' ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300' :
                                  resource.type === 'storage' ? 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300' :
                                  resource.resourceType === 'VM' ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' :
                                  'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                                }`}>
                                  {resource.resourceType}
                                </span>
                              </td>
                              <td class="px-3 py-1.5">
                                <span class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
                                  resource.status === 'online' || resource.status === 'running' ?
                                    'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300' :
                                  resource.status === 'stopped' ?
                                    'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300' :
                                    'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
                                }`}>
                                  {resource.status}
                                </span>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={resource.type === 'storage'} fallback={
                                  <Show when={isEditing()} fallback={
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="cpu"
                                      value={displayValue('cpu')}
                                      isOverridden={isOverridden('cpu')}
                                    />
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="100"
                                      value={thresholds().cpu || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        cpu: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                }>
                                  <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={resource.type === 'storage'} fallback={
                                  <Show when={isEditing()} fallback={
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="memory"
                                      value={displayValue('memory')}
                                      isOverridden={isOverridden('memory')}
                                    />
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="100"
                                      value={thresholds().memory || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        memory: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                }>
                                  <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={resource.type === 'storage'} fallback={
                                  <Show when={isEditing()} fallback={
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="disk"
                                      value={displayValue('disk')}
                                      isOverridden={isOverridden('disk')}
                                    />
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="100"
                                      value={thresholds().disk || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        disk: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                }>
                                  <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={resource.type === 'storage'} fallback={
                                  <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                }>
                                  <Show when={isEditing()} fallback={
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="usage"
                                      value={displayValue('usage')}
                                      isOverridden={isOverridden('usage')}
                                    />
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="100"
                                      value={thresholds().usage || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        usage: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={isEditing()} fallback={
                                  <Show when={shouldShowMetric('diskRead')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="diskRead"
                                      value={displayValue('diskRead')}
                                      isOverridden={isOverridden('diskRead')}
                                    />
                                  </Show>
                                }>
                                  <Show when={shouldShowMetric('diskRead')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="10000"
                                      value={thresholds().diskRead || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        diskRead: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={isEditing()} fallback={
                                  <Show when={shouldShowMetric('diskWrite')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="diskWrite"
                                      value={displayValue('diskWrite')}
                                      isOverridden={isOverridden('diskWrite')}
                                    />
                                  </Show>
                                }>
                                  <Show when={shouldShowMetric('diskWrite')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="10000"
                                      value={thresholds().diskWrite || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        diskWrite: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={isEditing()} fallback={
                                  <Show when={shouldShowMetric('networkIn')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="networkIn"
                                      value={displayValue('networkIn')}
                                      isOverridden={isOverridden('networkIn')}
                                    />
                                  </Show>
                                }>
                                  <Show when={shouldShowMetric('networkIn')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="10000"
                                      value={thresholds().networkIn || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        networkIn: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={isEditing()} fallback={
                                  <Show when={shouldShowMetric('networkOut')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <MetricValueWithHeat 
                                      resourceId={resource.id}
                                      metric="networkOut"
                                      value={displayValue('networkOut')}
                                      isOverridden={isOverridden('networkOut')}
                                    />
                                  </Show>
                                }>
                                  <Show when={shouldShowMetric('networkOut')} fallback={
                                    <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                                  }>
                                    <input
                                      type="number"
                                      min="0"
                                      max="10000"
                                      value={thresholds().networkOut || ''}
                                      onInput={(e) => setEditingThresholds({
                                        ...editingThresholds(),
                                        networkOut: parseInt(e.currentTarget.value) || undefined
                                      })}
                                      class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                    />
                                  </Show>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={resource.type === 'guest'}>
                                  <button
                                    onClick={() => toggleDisabled(resource.id)}
                                    class={`px-2 py-0.5 text-xs font-medium rounded transition-colors ${
                                      (() => {
                                        const override = props.overrides().find(o => o.id === resource.id);
                                        return override?.disabled;
                                      })()
                                        ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800/50'
                                        : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800/50'
                                    }`}
                                  >
                                    {(() => {
                                      const override = props.overrides().find(o => o.id === resource.id);
                                      return override?.disabled ? 'Disabled' : 'Enabled';
                                    })()}
                                  </button>
                                </Show>
                                <Show when={resource.type === 'node'}>
                                  <button
                                    onClick={() => toggleNodeConnectivity(resource.id)}
                                    class={`px-2 py-0.5 text-xs font-medium rounded transition-colors ${
                                      resource.disableConnectivity
                                        ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800/50'
                                        : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800/50'
                                    }`}
                                    title="Toggle connectivity alerts for this node"
                                  >
                                    {resource.disableConnectivity ? 'No Offline' : 'Alert Offline'}
                                  </button>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5">
                                <div class="flex items-center justify-center gap-1">
                                  <Show when={isEditing()} fallback={
                                    <>
                                      <button
                                        onClick={() => startEditing(resource.id, resource.thresholds, resource.defaults)}
                                        class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                        title="Edit thresholds"
                                      >
                                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                                            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                        </svg>
                                      </button>
                                      <Show when={props.overrides().find(o => o.id === resource.id)}>
                                        <button
                                          onClick={() => removeOverride(resource.id)}
                                          class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                                          title="Remove all overrides"
                                        >
                                          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                                              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                          </svg>
                                        </button>
                                      </Show>
                                    </>
                                  }>
                                    <button
                                      onClick={() => saveEdit(resource.id)}
                                      class="p-1 text-green-600 hover:text-green-700 dark:text-green-400 dark:hover:text-green-300"
                                      title="Save"
                                    >
                                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                      </svg>
                                    </button>
                                    <button
                                      onClick={cancelEdit}
                                      class="p-1 text-gray-600 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                                      title="Cancel"
                                    >
                                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                      </svg>
                                    </button>
                                  </Show>
                                </div>
                              </td>
                            </tr>
                          );
                        }}
                      </For>
                    </>
                  )}
                </For>
              }>
                <tr>
                  <td colspan="13" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                    No resources found
                  </td>
                </tr>
              </Show>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}