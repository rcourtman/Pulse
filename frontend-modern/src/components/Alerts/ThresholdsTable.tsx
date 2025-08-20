import { createSignal, createMemo, For, Show } from 'solid-js';
import type { VM, Container, Node } from '@/types/api';

interface Override {
  id: string;
  name: string;
  type: 'guest' | 'node';
  resourceType?: string;
  vmid?: number;
  node?: string;
  instance?: string;
  disabled?: boolean;
  thresholds: {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
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
  guestDefaults: SimpleThresholds;
  setGuestDefaults: (value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>)) => void;
  nodeDefaults: SimpleThresholds;
  setNodeDefaults: (value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>)) => void;
  storageDefault: () => number;
  setStorageDefault: (value: number) => void;
  timeThreshold: () => number;
  setTimeThreshold: (value: number) => void;
  setHasUnsavedChanges: (value: boolean) => void;
}

export function ThresholdsTable(props: ThresholdsTableProps) {
  const [searchTerm, setSearchTerm] = createSignal('');
  const [showGlobalSettings, setShowGlobalSettings] = createSignal(true);
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<Record<string, any>>({});
  
  // Combine all resources (guests and nodes) with their overrides
  const resourcesWithOverrides = createMemo(() => {
    const search = searchTerm().toLowerCase();
    const overridesMap = new Map(props.overrides().map(o => [o.id, o]));
    
    // Process guests
    const guests = props.allGuests().map(guest => {
      const guestId = guest.id || `${guest.instance}-${guest.name}-${guest.vmid}`;
      const override = overridesMap.get(guestId);
      
      return {
        id: guestId,
        name: guest.name,
        type: 'guest' as const,
        resourceType: guest.type === 'qemu' ? 'VM' : 'CT',
        vmid: guest.vmid,
        node: guest.node,
        instance: guest.instance,
        status: guest.status,
        hasOverride: !!override,
        disabled: override?.disabled || false,
        thresholds: override?.thresholds || props.guestDefaults
      };
    });
    
    // Process nodes
    const nodes = props.nodes.map(node => {
      const override = overridesMap.get(node.id);
      
      return {
        id: node.id,
        name: node.name,
        type: 'node' as const,
        resourceType: 'Node',
        status: node.status,
        hasOverride: !!override,
        disabled: false,
        thresholds: override?.thresholds || props.nodeDefaults
      };
    });
    
    // Combine and filter
    const allResources = [...guests, ...nodes];
    
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
      const groupKey = resource.type === 'node' ? 'Nodes' : ('node' in resource ? resource.node : 'Unknown');
      
      if (!groups[groupKey]) {
        groups[groupKey] = [];
      }
      groups[groupKey].push(resource);
    });
    
    // Sort resources within each group
    Object.keys(groups).forEach(key => {
      groups[key] = groups[key].sort((a, b) => {
        if (a.type === 'node' && b.type !== 'node') return -1;
        if (a.type !== 'node' && b.type === 'node') return 1;
        if ('vmid' in a && 'vmid' in b && a.vmid && b.vmid) return a.vmid - b.vmid;
        return a.name.localeCompare(b.name);
      });
    });
    
    return groups;
  });
  
  const startEditing = (resourceId: string, currentThresholds: any) => {
    setEditingId(resourceId);
    setEditingThresholds(currentThresholds);
  };
  
  const saveEdit = (resourceId: string) => {
    const resource = resourcesWithOverrides().find(r => r.id === resourceId);
    if (!resource) return;
    
    const thresholds = editingThresholds();
    
    // Check if there are any actual changes from the defaults
    const defaultThresholds = resource.type === 'guest' ? props.guestDefaults : props.nodeDefaults;
    const hasChanges = Object.keys(thresholds).some(key => {
      const editedValue = thresholds[key];
      const defaultValue = defaultThresholds[key as keyof typeof defaultThresholds];
      return editedValue !== defaultValue;
    });
    
    // If no changes and no existing override, just cancel the edit
    if (!hasChanges && !resource.hasOverride) {
      cancelEdit();
      return;
    }
    
    // If no changes but there's an existing override, keep it as is
    if (!hasChanges && resource.hasOverride) {
      cancelEdit();
      return;
    }
    
    // Create or update override
    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: resource.disabled,
      thresholds
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
    Object.entries(thresholds).forEach(([metric, value]) => {
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
    
    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: !resource.disabled,
      thresholds: resource.thresholds
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
    Object.entries(resource.thresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = { 
          trigger: value, 
          clear: Math.max(0, (value as number) - 5) 
        };
      }
    });
    hysteresisThresholds.disabled = !resource.disabled;
    newRawConfig[resourceId] = hysteresisThresholds;
    props.setRawOverridesConfig(newRawConfig);
    
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
          <div class="border-t border-gray-200 dark:border-gray-700 p-4">
            {/* Compact grid layout */}
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
              {/* Left column - Time threshold and reset button */}
              <div class="space-y-4">
                <div class="flex items-center gap-3">
                  <label class="text-sm font-medium text-gray-700 dark:text-gray-300 min-w-fit">
                    Time Threshold:
                  </label>
                  <div class="flex items-center gap-2">
                    <input
                      type="number"
                      min="0"
                      max="300"
                      value={props.timeThreshold()}
                      onInput={(e) => {
                        props.setTimeThreshold(parseInt(e.currentTarget.value) || 0);
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-16 px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded
                             bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100
                             focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                    <span class="text-sm text-gray-500 dark:text-gray-400">
                      sec {props.timeThreshold() === 0 ? '(disabled)' : `(wait ${props.timeThreshold()}s before alerting)`}
                    </span>
                  </div>
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
                  class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 font-medium"
                >
                  Reset All to Defaults
                </button>
              </div>
              
              {/* Right column - Threshold values in compact table */}
              <div class="overflow-x-auto">
                <table class="w-full text-sm">
                  <thead>
                    <tr class="border-b border-gray-200 dark:border-gray-700">
                      <th class="text-left py-1 px-2 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Type</th>
                      <th class="text-center px-2 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">CPU %</th>
                      <th class="text-center px-2 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Memory %</th>
                      <th class="text-center px-2 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Disk %</th>
                      <th class="text-center px-2 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Storage %</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td class="py-2 px-2 font-medium text-gray-700 dark:text-gray-300">VMs & Containers</td>
                      <td class="text-center px-2">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.guestDefaults.cpu || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, cpu: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        />
                      </td>
                      <td class="text-center px-2">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.guestDefaults.memory || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, memory: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        />
                      </td>
                      <td class="text-center px-2">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.guestDefaults.disk || 0}
                          onInput={(e) => {
                            props.setGuestDefaults((prev) => ({...prev, disk: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        />
                      </td>
                      <td class="text-center px-2 text-gray-400">-</td>
                    </tr>
                    <tr>
                      <td class="py-2 px-2 font-medium text-gray-700 dark:text-gray-300">Proxmox Nodes</td>
                      <td class="text-center px-2">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.nodeDefaults.cpu || 0}
                          onInput={(e) => {
                            props.setNodeDefaults((prev) => ({...prev, cpu: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        />
                      </td>
                      <td class="text-center px-2">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.nodeDefaults.memory || 0}
                          onInput={(e) => {
                            props.setNodeDefaults((prev) => ({...prev, memory: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        />
                      </td>
                      <td class="text-center px-2">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.nodeDefaults.disk || 0}
                          onInput={(e) => {
                            props.setNodeDefaults((prev) => ({...prev, disk: parseInt(e.currentTarget.value) || 0}));
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        />
                      </td>
                      <td class="text-center px-2 text-gray-400">-</td>
                    </tr>
                    <tr>
                      <td class="py-2 px-2 font-medium text-gray-700 dark:text-gray-300">Storage</td>
                      <td class="text-center px-2 text-gray-400">-</td>
                      <td class="text-center px-2 text-gray-400">-</td>
                      <td class="text-center px-2 text-gray-400">-</td>
                      <td class="text-center px-2">
                        <input
                          type="number"
                          min="0"
                          max="100"
                          value={props.storageDefault()}
                          onInput={(e) => {
                            props.setStorageDefault(parseInt(e.currentTarget.value) || 0);
                            props.setHasUnsavedChanges(true);
                          }}
                          class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                        />
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </Show>
      </div>
      
      {/* Search Bar */}
      <div class="relative">
        <input
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
                  if (a === 'Nodes') return -1;
                  if (b === 'Nodes') return 1;
                  return a.localeCompare(b);
                })}>
                  {([groupName, resources]) => (
                    <>
                      {/* Group header */}
                      <tr class="bg-gray-50 dark:bg-gray-700/50">
                        <td colspan="8" class="px-3 py-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                          {groupName}
                        </td>
                      </tr>
                      {/* Resource rows */}
                      <For each={resources}>
                        {(resource) => {
                          const isEditing = () => editingId() === resource.id;
                          const thresholds = () => isEditing() ? editingThresholds() : resource.thresholds;
                          
                          return (
                            <tr class="hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors">
                              <td class="px-3 py-1.5">
                                <div class="flex items-center gap-2">
                                  <span class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                    {resource.name}
                                  </span>
                                  <Show when={'vmid' in resource && resource.vmid}>
                                    <span class="text-xs text-gray-500">({resource.vmid})</span>
                                  </Show>
                                  <Show when={resource.hasOverride}>
                                    <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                                      Custom
                                    </span>
                                  </Show>
                                </div>
                              </td>
                              <td class="px-3 py-1.5">
                                <span class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
                                  resource.type === 'node' ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300' :
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
                                <Show when={isEditing()} fallback={
                                  <span class="text-sm text-gray-700 dark:text-gray-300">
                                    {thresholds().cpu || '-'}
                                  </span>
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
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={isEditing()} fallback={
                                  <span class="text-sm text-gray-700 dark:text-gray-300">
                                    {thresholds().memory || '-'}
                                  </span>
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
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={isEditing()} fallback={
                                  <span class="text-sm text-gray-700 dark:text-gray-300">
                                    {thresholds().disk || '-'}
                                  </span>
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
                              </td>
                              <td class="px-3 py-1.5 text-center">
                                <Show when={resource.type === 'guest'}>
                                  <button
                                    onClick={() => toggleDisabled(resource.id)}
                                    class={`px-2 py-0.5 text-xs font-medium rounded transition-colors ${
                                      resource.disabled
                                        ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800/50'
                                        : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800/50'
                                    }`}
                                  >
                                    {resource.disabled ? 'Disabled' : 'Enabled'}
                                  </button>
                                </Show>
                                <Show when={resource.type === 'node'}>
                                  <span class="text-xs text-gray-500">-</span>
                                </Show>
                              </td>
                              <td class="px-3 py-1.5">
                                <div class="flex items-center justify-center gap-1">
                                  <Show when={isEditing()} fallback={
                                    <>
                                      <button
                                        onClick={() => startEditing(resource.id, resource.thresholds)}
                                        class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                        title="Edit thresholds"
                                      >
                                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                                            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                        </svg>
                                      </button>
                                      <Show when={resource.hasOverride}>
                                        <button
                                          onClick={() => removeOverride(resource.id)}
                                          class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                                          title="Remove override"
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
                  <td colspan="8" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
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