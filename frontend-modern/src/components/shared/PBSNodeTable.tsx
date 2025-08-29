import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import type { PBSInstance } from '@/types/api';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { formatBytes, formatUptime } from '@/utils/format';

interface PBSNodeTableProps {
  pbsInstances?: PBSInstance[];
  backupCounts?: Record<string, number>;
  selectedNode: string | null;
  onNodeClick: (nodeId: string) => void;
  onNamespaceClick?: (instanceName: string, datastoreName: string, namespace: string) => void;
  currentTab?: 'dashboard' | 'storage' | 'backups';
  filteredBackups?: any[];
  searchTerm?: string;
}

export const PBSNodeTable: Component<PBSNodeTableProps> = (props) => {
  // Track which PBS instances are expanded to show datastores/namespaces
  const [expandedInstances, setExpandedInstances] = createSignal<Set<string>>(new Set());
  
  const toggleExpanded = (instanceName: string) => {
    const expanded = new Set(expandedInstances());
    if (expanded.has(instanceName)) {
      expanded.delete(instanceName);
    } else {
      expanded.add(instanceName);
    }
    setExpandedInstances(expanded);
  };
  
  const isExpanded = (instanceName: string) => expandedInstances().has(instanceName);
  
  // Check if a namespace is currently selected/filtered
  const isNamespaceSelected = (instanceName: string, datastoreName: string, namespace: string) => {
    if (!props.searchTerm) return false;
    const expectedFilter = `pbs:${instanceName}:${datastoreName}:${namespace}`;
    return props.searchTerm === expectedFilter;
  };
  
  // Filter and sort PBS instances - but don't hide them when filtering, similar to PVE nodes
  const sortedInstances = createMemo(() => {
    if (!props.pbsInstances) return [];
    
    let instances = [...props.pbsInstances];
    
    // For PBS instances, always show all of them
    // The selection/highlighting will indicate which one is filtered
    // This keeps the PBS cards as a stable navigation element
    
    return instances.sort((a, b) => {
      // Healthy/online instances first
      const aOnline = a.status === 'healthy' || a.status === 'online';
      const bOnline = b.status === 'healthy' || b.status === 'online';
      if (aOnline !== bOnline) return aOnline ? -1 : 1;
      
      // Then by name
      return a.name.localeCompare(b.name);
    });
  });

  if (sortedInstances().length === 0) return null;

  return (
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
      <div class="overflow-x-auto" style="overflow-x: auto; scrollbar-width: none; -ms-overflow-style: none;">
        <style>{`
          .overflow-x-auto::-webkit-scrollbar { display: none; }
        `}</style>
        <table class="w-full" style="min-width: 1000px;">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 200px;">
                PBS Instances
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 80px;">
                Status
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap">
                Version
              </th>
              <th class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 100px;">
                Datastores
              </th>
              <th class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 80px;">
                Backups
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 180px;">
                CPU
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 180px;">
                Memory
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 180px;">
                Storage Used
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap">
                Uptime
              </th>
            </tr>
          </thead>
          <tbody>
            <For each={sortedInstances()}>
              {(pbs) => {
                const isOnline = () => pbs.status === 'healthy' || pbs.status === 'online';
                const cpuPercent = () => Math.round(pbs.cpu || 0);
                const memPercent = () => pbs.memoryTotal > 0 ? Math.round((pbs.memoryUsed / pbs.memoryTotal) * 100) : 0;
                
                // Calculate total storage across all datastores
                const totalStorage = createMemo(() => {
                  if (!pbs.datastores) return { used: 0, total: 0, percent: 0 };
                  
                  const totals = pbs.datastores.reduce((acc, ds) => {
                    acc.used += ds.used || 0;
                    acc.total += ds.total || 0;
                    return acc;
                  }, { used: 0, total: 0 });
                  
                  const percent = totals.total > 0 ? Math.round((totals.used / totals.total) * 100) : 0;
                  return { ...totals, percent };
                });
                
                const isSelected = () => props.selectedNode === pbs.name;
                const isClickable = props.currentTab === 'backups';
                
                const hasDatastoresWithNamespaces = () => {
                  return pbs.datastores?.some(ds => ds.namespaces && ds.namespaces.length > 0);
                };
                
                return (
                  <>
                  <tr 
                    class={`
                      border-b border-gray-100 dark:border-gray-700/50 
                      transition-all duration-150 ease-in-out h-8 relative
                      ${isClickable ? 'hover:bg-gray-50 dark:hover:bg-gray-700/30 cursor-pointer hover:scale-[1.01] hover:shadow-md hover:z-10 hover:border-l-4 hover:border-l-blue-500 dark:hover:border-l-blue-400' : ''}
                      ${!isOnline() ? 'opacity-60' : ''}
                      ${isSelected() && isClickable ? 'bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 scale-[1.005] shadow-sm border-l-4 border-l-blue-600 dark:border-l-blue-500' : ''}
                    `}
                    onClick={(e) => {
                      // If clicking the expand button, don't trigger node click
                      if ((e.target as HTMLElement).closest('.expand-button')) {
                        return;
                      }
                      isClickable && props.onNodeClick(pbs.name);
                    }}
                  >
                    <td class="p-1 px-2 whitespace-nowrap">
                      <div class="flex items-center gap-2">
                        <Show when={hasDatastoresWithNamespaces()}>
                          <button
                            class="expand-button p-0.5 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                            onClick={(e) => {
                              e.stopPropagation();
                              toggleExpanded(pbs.name);
                            }}
                            title={isExpanded(pbs.name) ? "Collapse" : "Expand"}
                          >
                            <svg
                              class={`w-3 h-3 text-gray-500 dark:text-gray-400 transition-transform ${
                                isExpanded(pbs.name) ? 'rotate-90' : ''
                              }`}
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M9 5l7 7-7 7"
                              />
                            </svg>
                          </button>
                        </Show>
                        <span class={`h-2 w-2 rounded-full ${isOnline() ? 'bg-green-500' : 'bg-red-500'}`}></span>
                        <span class="text-sm font-medium text-gray-900 dark:text-gray-100" style="white-space: nowrap;">
                          {pbs.name}
                        </span>
                      </div>
                    </td>
                    <td class="p-1 px-2">
                      <span class={`text-xs ${isOnline() ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                        {pbs.status}
                      </span>
                    </td>
                    <td class="p-1 px-2">
                      <span class="text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap">
                        <Show when={pbs.version} fallback="-">
                          {pbs.version}
                        </Show>
                      </span>
                    </td>
                    <td class="p-1 px-2 text-center">
                      <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
                        {pbs.datastores?.length || 0}
                      </span>
                    </td>
                    <td class="p-1 px-2 text-center">
                      <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
                        {props.backupCounts?.[pbs.name] || 0}
                      </span>
                    </td>
                    <td class="p-1 px-2" style="min-width: 180px;">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={cpuPercent()} 
                          label={`${cpuPercent()}%`}
                          type="cpu"
                        />
                      </Show>
                    </td>
                    <td class="p-1 px-2" style="min-width: 180px;">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={memPercent()} 
                          label={`${memPercent()}%`}
                          sublabel={`${formatBytes(pbs.memoryUsed)} / ${formatBytes(pbs.memoryTotal)}`}
                          type="memory"
                        />
                      </Show>
                    </td>
                    <td class="p-1 px-2" style="min-width: 180px;">
                      <Show when={isOnline() && totalStorage().total > 0} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={totalStorage().percent} 
                          label={`${totalStorage().percent}%`}
                          sublabel={`${formatBytes(totalStorage().used)} / ${formatBytes(totalStorage().total)}`}
                          type="disk"
                        />
                      </Show>
                    </td>
                    <td class="p-1 px-2">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
                        <Show when={isOnline() && pbs.uptime} fallback="-">
                          {formatUptime(pbs.uptime)}
                        </Show>
                      </span>
                    </td>
                  </tr>
                  
                  {/* Expandable rows for datastores and namespaces */}
                  <Show when={isExpanded(pbs.name) && pbs.datastores}>
                    <For each={pbs.datastores}>
                      {(datastore) => (
                        <>
                          {/* Datastore row */}
                          <tr class="bg-gray-50 dark:bg-gray-800/50 border-b border-gray-100 dark:border-gray-700/30">
                            <td colspan="9" class="px-8 py-1">
                              <div class="flex items-center justify-between">
                                <div class="flex items-center gap-3">
                                  <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
                                  </svg>
                                  <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                                    {datastore.name}
                                  </span>
                                  <span class="text-xs text-gray-500 dark:text-gray-400">
                                    ({datastore.namespaces?.length || 0} namespaces)
                                  </span>
                                </div>
                                <div class="flex items-center gap-4 text-xs">
                                  <div class="flex items-center gap-1">
                                    <span class="text-gray-500">Used:</span>
                                    <span class="font-medium text-gray-700 dark:text-gray-300">
                                      {formatBytes(datastore.used || 0)}
                                    </span>
                                  </div>
                                  <div class="flex items-center gap-1">
                                    <span class="text-gray-500">Total:</span>
                                    <span class="font-medium text-gray-700 dark:text-gray-300">
                                      {formatBytes(datastore.total || 0)}
                                    </span>
                                  </div>
                                </div>
                              </div>
                            </td>
                          </tr>
                          
                          {/* Namespace rows */}
                          <Show when={datastore.namespaces && datastore.namespaces.length > 0}>
                            <For each={datastore.namespaces}>
                              {(namespace) => {
                                const isSelected = () => isNamespaceSelected(pbs.name, datastore.name, namespace.path || '/');
                                
                                return (
                                  <tr 
                                    class={`
                                      cursor-pointer transition-all duration-150 border-b border-gray-50 dark:border-gray-800
                                      ${isSelected() 
                                        ? 'bg-blue-100 dark:bg-blue-900/40 hover:bg-blue-150 dark:hover:bg-blue-900/50 font-medium' 
                                        : 'bg-gray-25 dark:bg-gray-850 hover:bg-blue-50 dark:hover:bg-blue-900/20'}
                                    `}
                                    onClick={() => {
                                      if (props.onNamespaceClick) {
                                        props.onNamespaceClick(pbs.name, datastore.name, namespace.path || '/');
                                      }
                                    }}
                                  >
                                    <td colspan="9" class="px-12 py-0.5">
                                      <div class="flex items-center gap-2">
                                        <svg 
                                          class={`w-3 h-3 ${isSelected() ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400'}`} 
                                          fill="none" 
                                          viewBox="0 0 24 24" 
                                          stroke="currentColor"
                                        >
                                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h10M7 12h10m-7 5h4" />
                                        </svg>
                                        <span class={`text-xs ${
                                          isSelected() 
                                            ? 'text-blue-700 dark:text-blue-300' 
                                            : 'text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400'
                                        }`}>
                                          {namespace.path || '/ (root)'}
                                        </span>
                                        <Show when={isSelected()}>
                                          <span class="text-xs text-blue-600 dark:text-blue-400 ml-auto mr-2">
                                            (filtering)
                                          </span>
                                        </Show>
                                      </div>
                                    </td>
                                  </tr>
                                );
                              }}
                            </For>
                          </Show>
                        </>
                      )}
                    </For>
                  </Show>
                  </>
                );
              }}
            </For>
          </tbody>
        </table>
      </div>
    </div>
  );
};