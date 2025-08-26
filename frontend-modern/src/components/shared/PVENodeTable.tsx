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
}

export const PVENodeTable: Component<PVENodeTableProps> = (props) => {
  // Check if we have active filtering (receiving filtered guests)
  const hasActiveFilter = createMemo(() => {
    // If vms or containers props are explicitly passed, we're filtering
    return props.vms !== undefined || props.containers !== undefined;
  });

  // Filter and sort nodes - only show nodes with matching guests when filtering
  const sortedNodes = createMemo(() => {
    if (!props.nodes) return [];
    
    let nodes = [...props.nodes];
    
    // If we have filtered guests, only show nodes that have at least one matching guest
    if (hasActiveFilter() && props.currentTab === 'dashboard') {
      const nodesWithGuests = new Set<string>();
      props.vms?.forEach(vm => nodesWithGuests.add(vm.node));
      props.containers?.forEach(ct => nodesWithGuests.add(ct.node));
      
      // Only show nodes that have filtered guests
      if (nodesWithGuests.size > 0) {
        nodes = nodes.filter(node => nodesWithGuests.has(node.name));
      }
    }
    
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
        return [props.backupCounts?.[node.name] || 0];
      default:
        return [];
    }
  };

  if (sortedNodes().length === 0) return null;

  return (
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="border-b border-gray-200 dark:border-gray-700">
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                PVE Nodes
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Status
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Cluster
              </th>
              <For each={getCountHeaders()}>
                {(header) => (
                  <th class="px-2 py-1 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {header}
                  </th>
                )}
              </For>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[180px]">
                CPU
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[180px]">
                Memory
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[180px]">
                Storage
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Uptime
              </th>
            </tr>
          </thead>
          <tbody>
            <For each={sortedNodes()}>
              {(node) => {
                const isOnline = () => node.status === 'online';
                
                // Calculate metrics based on filtered guests if on dashboard with active filter
                const getFilteredMetrics = () => {
                  if (props.currentTab === 'dashboard' && hasActiveFilter()) {
                    const nodeVms = props.vms?.filter(vm => vm.node === node.name) || [];
                    const nodeContainers = props.containers?.filter(ct => ct.node === node.name) || [];
                    
                    // Calculate total CPU usage from filtered guests
                    let totalCpuUsage = 0;
                    let totalMemUsed = 0;
                    let totalDiskUsed = 0;
                    
                    nodeVms.forEach(vm => {
                      if (vm.cpu) totalCpuUsage += vm.cpu * (vm.cpus || 1);
                      if (vm.mem) totalMemUsed += vm.mem;
                      if (vm.disk) totalDiskUsed += vm.disk;
                    });
                    
                    nodeContainers.forEach(ct => {
                      if (ct.cpu) totalCpuUsage += ct.cpu * (ct.cpus || 1);
                      if (ct.mem) totalMemUsed += ct.mem;
                      if (ct.disk) totalDiskUsed += ct.disk;
                    });
                    
                    // Convert to percentages relative to node capacity
                    const cpuPercent = node.cpuInfo?.cores ? Math.round((totalCpuUsage / node.cpuInfo.cores) * 100) : 0;
                    const memPercent = node.memory?.total ? Math.round((totalMemUsed / node.memory.total) * 100) : 0;
                    const diskPercent = node.disk?.total ? Math.round((totalDiskUsed / node.disk.total) * 100) : 0;
                    
                    return { cpuPercent, memPercent, diskPercent, guestCount: nodeVms.length + nodeContainers.length };
                  }
                  
                  // Default: show node's actual metrics
                  return {
                    cpuPercent: Math.round(node.cpu || 0),
                    memPercent: node.memory?.total > 0 ? Math.round((node.memory.used / node.memory.total) * 100) : 0,
                    diskPercent: node.disk?.total > 0 ? Math.round((node.disk.used / node.disk.total) * 100) : 0,
                    guestCount: null
                  };
                };
                
                const metrics = createMemo(() => getFilteredMetrics());
                const cpuPercent = () => metrics().cpuPercent;
                const memPercent = () => metrics().memPercent;
                const storagePercent = () => metrics().diskPercent;
                const filteredGuestCount = () => metrics().guestCount;
                
                const counts = getNodeCounts(node);
                const isSelected = () => props.selectedNode === node.name;
                
                return (
                  <tr 
                    class={`
                      border-b border-gray-100 dark:border-gray-700/50 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer
                      ${!isOnline() ? 'opacity-60' : ''}
                      ${isSelected() ? 'bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30' : ''}
                    `}
                    onClick={() => props.onNodeClick(node.name)}
                  >
                    <td class="px-2 py-0.5">
                      <div class="flex items-center gap-2">
                        <span class={`h-2 w-2 rounded-full ${isOnline() ? 'bg-green-500' : 'bg-red-500'}`}></span>
                        <span class="text-sm font-medium text-gray-900 dark:text-gray-100">{node.name}</span>
                        <Show when={filteredGuestCount() !== null}>
                          <span class="text-xs text-gray-500 dark:text-gray-400">
                            ({filteredGuestCount()} matched)
                          </span>
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-0.5">
                      <span class={`text-xs ${isOnline() ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                        {node.status}
                      </span>
                    </td>
                    <td class="px-2 py-0.5">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
                        {node.clusterName || '-'}
                      </span>
                    </td>
                    <For each={counts}>
                      {(count) => (
                        <td class="px-2 py-0.5 text-center">
                          <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
                            {count}
                          </span>
                        </td>
                      )}
                    </For>
                    <td class="px-2 py-0.5 w-[180px]">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={cpuPercent()} 
                          label={`${cpuPercent()}%`}
                          sublabel={node.cpuInfo?.cores ? `${node.cpuInfo.cores} cores` : undefined}
                          type="cpu"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5 w-[180px]">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={memPercent()} 
                          label={`${memPercent()}%`}
                          sublabel={`${formatBytes(node.memory.used)} / ${formatBytes(node.memory.total)}`}
                          type="memory"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5 w-[180px]">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={storagePercent()} 
                          label={`${storagePercent()}%`}
                          sublabel={`${formatBytes(node.disk.used)} / ${formatBytes(node.disk.total)}`}
                          type="disk"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
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
  );
};