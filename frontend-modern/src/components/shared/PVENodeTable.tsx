import { Component, For, Show, createMemo } from 'solid-js';
import type { Node } from '@/types/api';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { formatBytes, formatUptime } from '@/utils/format';

interface PVENodeTableProps {
  nodes: Node[];
  vms?: any[];
  containers?: any[];
  storage?: any[];
  backupCounts?: Record<string, number>;
  currentTab: 'dashboard' | 'storage' | 'backups';
  selectedNode: string | null;
  onNodeClick: (nodeId: string) => void;
  searchTerm?: string;
  filteredBackups?: any[];
}

export const PVENodeTable: Component<PVENodeTableProps> = (props) => {
  
  // Check if we have active filtering
  const hasActiveFilter = createMemo(() => {
    // Check based on current tab
    switch (props.currentTab) {
      case 'dashboard':
        return props.vms !== undefined || props.containers !== undefined;
      case 'storage':
        return props.storage !== undefined;
      case 'backups':
        // For backups, only consider it filtered if there's a search term or filtered backups is explicitly set and different from all backups
        // We can't easily detect if it's filtered, so we'll rely on search term
        return false; // Let the filtering logic handle it based on filteredBackups content
      default:
        return false;
    }
  });

  // Filter and sort nodes based on current tab
  const sortedNodes = createMemo(() => {
    if (!props.nodes) return [];
    
    let nodes = [...props.nodes];
    
    // Special handling for backups tab - only hide nodes that have no backups at all
    if (props.currentTab === 'backups') {
      // Filter nodes to only show those with backups
      // First, check if nodes have any backups at all (from backupCounts)
      nodes = nodes.filter(node => {
        const backupCount = props.backupCounts?.[node.name] || 0;
        return backupCount > 0;
      });
      // Don't further filter based on search - let highlighting handle that
    }
    // For other tabs, ALWAYS show all nodes - don't filter them out
    // The node cards should remain as a stable navigation element
    // Only the data below gets filtered
    
    return nodes.sort((a, b) => {
      // Online nodes first
      if (a.status !== b.status) {
        return a.status === 'online' ? -1 : 1;
      }
      // Then by name
      return a.name.localeCompare(b.name);
    });
  });

  // Get column headers based on tab
  const getCountHeaders = () => {
    switch (props.currentTab) {
      case 'dashboard': return ['VMs', 'LXCs'];
      case 'storage': return ['Storages'];
      case 'backups': return ['Backups'];
      default: return [];
    }
  };

  // Get count values for a node
  const getNodeCounts = (node: Node) => {
    switch (props.currentTab) {
      case 'dashboard':
        const vmCount = props.vms?.filter(vm => vm.node === node.name).length || 0;
        const containerCount = props.containers?.filter(ct => ct.node === node.name).length || 0;
        return [vmCount, containerCount];
      case 'storage':
        const storageCount = props.storage?.filter(s => s.node === node.name).length || 0;
        return [storageCount];
      case 'backups':
        // If we have filtered backups, count all types for display
        // but remember that PBS backups are shared across nodes
        if (props.filteredBackups !== undefined) {
          // Count all backups that match this node (including PBS for display purposes)
          const backupCount = props.filteredBackups.filter(b => b.node === node.name).length;
          return [backupCount];
        }
        return [props.backupCounts?.[node.name] || 0];
      default:
        return [];
    }
  };

  // Create a reactive memo for whether to show the table
  // Check sortedNodes to hide table when filtering results in no nodes
  const showTable = createMemo(() => {
    const nodes = sortedNodes();
    return nodes && nodes.length > 0;
  });
  
  // Table uses 100% width with fixed layout
  
  return (
    <Show when={showTable()}>
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
        <div class="overflow-x-auto" style="overflow-x: auto; scrollbar-width: none; -ms-overflow-style: none;">
          <style>{`
            .overflow-x-auto::-webkit-scrollbar { display: none; }
          `}</style>
          <table class="w-full" style="min-width: 1000px;">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 200px;">
                PVE Nodes
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 100px;">
                Cluster
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap">
                Version
              </th>
              <For each={getCountHeaders()}>
                {(header) => (
                  <th class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 80px;">
                    {header}
                  </th>
                )}
              </For>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 180px;">
                CPU
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 180px;">
                Memory
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap" style="min-width: 180px;">
                Storage
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap">
                Uptime
              </th>
            </tr>
          </thead>
          <tbody>
            <For each={sortedNodes()}>
              {(node) => {
                const isOnline = () => node.status === 'online';
                
                // Get filtered guest count for display
                const getFilteredGuestCount = () => {
                  if (props.currentTab === 'dashboard' && hasActiveFilter()) {
                    const nodeVms = props.vms?.filter(vm => vm.node === node.name) || [];
                    const nodeContainers = props.containers?.filter(ct => ct.node === node.name) || [];
                    return nodeVms.length + nodeContainers.length;
                  }
                  return null;
                };
                
                // Always show actual node metrics, not calculated from filtered guests
                const cpuPercent = () => Math.round(node.cpu || 0);
                const memPercent = () => node.memory?.total > 0 ? Math.round((node.memory.used / node.memory.total) * 100) : 0;
                const storagePercent = () => node.disk?.total > 0 ? Math.round((node.disk.used / node.disk.total) * 100) : 0;
                const filteredGuestCount = createMemo(() => getFilteredGuestCount());
                
                const counts = getNodeCounts(node);
                // Check if this node is selected
                const isSelected = () => props.selectedNode === node.name;
                
                return (
                  <tr 
                    class={`
                      border-b border-gray-100 dark:border-gray-700/50
                      transition-all duration-200 ease-out cursor-pointer h-8
                      hover:bg-gray-50 dark:hover:bg-gray-700/30
                      ${!isOnline() ? 'opacity-60' : ''}
                      ${isSelected() 
                        ? 'bg-blue-50 dark:bg-blue-900/20 border-l-4 border-l-blue-500 dark:border-l-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30' 
                        : ''
                      }
                    `}
                    onClick={() => props.onNodeClick(node.name)}
                  >
                    <td class="p-1 px-2 whitespace-nowrap">
                      <div class="flex items-center gap-2">
                        <span class={`h-2 w-2 rounded-full flex-shrink-0 ${isOnline() ? 'bg-green-500' : 'bg-red-500'}`}></span>
                        <span class={`text-sm truncate ${isSelected() ? 'font-semibold text-blue-700 dark:text-blue-300' : 'font-medium text-gray-900 dark:text-gray-100'}`} title={node.name}>{node.name}</span>
                      </div>
                    </td>
                    <td class="p-1 px-2">
                      <span class="text-xs text-gray-600 dark:text-gray-400 block truncate" title={node.clusterName || ''}>
                        {node.clusterName || '-'}
                      </span>
                    </td>
                    <td class="p-1 px-2">
                      <span class="text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap" title={node.pveVersion || ''}>
                        <Show when={node.pveVersion} fallback="-">
                          {node.pveVersion.split('/')[1] || node.pveVersion}
                        </Show>
                      </span>
                    </td>
                    <For each={counts}>
                      {(count) => (
                        <td class="p-1 px-2 text-center">
                          <span class="text-xs font-medium text-gray-700 dark:text-gray-300 whitespace-nowrap">
                            {count}
                          </span>
                        </td>
                      )}
                    </For>
                    <td class="p-1 px-2" style="min-width: 180px;">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={cpuPercent()} 
                          label={`${cpuPercent()}%`}
                          sublabel={node.cpuInfo?.cores ? `${node.cpuInfo.cores} cores` : undefined}
                          type="cpu"
                        />
                      </Show>
                    </td>
                    <td class="p-1 px-2" style="min-width: 180px;">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={memPercent()} 
                          label={`${memPercent()}%`}
                          sublabel={`${formatBytes(node.memory.used)} / ${formatBytes(node.memory.total)}`}
                          type="memory"
                        />
                      </Show>
                    </td>
                    <td class="p-1 px-2" style="min-width: 180px;">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={storagePercent()} 
                          label={`${storagePercent()}%`}
                          sublabel={`${formatBytes(node.disk.used)} / ${formatBytes(node.disk.total)}`}
                          type="disk"
                        />
                      </Show>
                    </td>
                    <td class="p-1 px-2">
                      <span class="text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap">
                        <Show when={isOnline() && node.uptime} fallback="-">
                          {formatUptime(node.uptime)}
                        </Show>
                      </span>
                    </td>
                  </tr>
                );
              }}
            </For>
          </tbody>
        </table>
      </div>
    </div>
    </Show>
  );
};