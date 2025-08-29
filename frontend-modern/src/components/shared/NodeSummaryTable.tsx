import { Component, For, Show, createMemo } from 'solid-js';
import type { Node, VM, Container, Storage, PBSInstance } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { MetricBar } from '@/components/Dashboard/MetricBar';

interface NodeSummaryTableProps {
  nodes: Node[];
  pbsInstances?: PBSInstance[];
  vms?: VM[];
  containers?: Container[];
  storage?: Storage[];
  backupCounts?: Record<string, number>;
  currentTab: 'dashboard' | 'storage' | 'backups';
  selectedNode: string | null;
  onNodeClick: (nodeId: string, nodeType: 'pve' | 'pbs') => void;
}

export const NodeSummaryTable: Component<NodeSummaryTableProps> = (props) => {
  // Combine and sort nodes based on tab
  const sortedItems = createMemo(() => {
    const items: Array<{ type: 'pve' | 'pbs'; data: Node | PBSInstance }> = [];
    
    // Add PVE nodes (shown on all tabs)
    if (props.nodes) {
      props.nodes.forEach(node => items.push({ type: 'pve', data: node }));
    }
    
    // Add PBS instances (shown on all tabs)
    if (props.pbsInstances) {
      props.pbsInstances.forEach(pbs => items.push({ type: 'pbs', data: pbs }));
    }
    
    // Sort by type (PVE first) then by status then by name
    return items.sort((a, b) => {
      // PVE nodes come before PBS
      if (a.type !== b.type) return a.type === 'pve' ? -1 : 1;
      
      // Then by online status
      const aOnline = a.type === 'pve' 
        ? (a.data as Node).status === 'online'
        : ((a.data as PBSInstance).status === 'healthy' || (a.data as PBSInstance).status === 'online');
      const bOnline = b.type === 'pve'
        ? (b.data as Node).status === 'online'
        : ((b.data as PBSInstance).status === 'healthy' || (b.data as PBSInstance).status === 'online');
      if (aOnline !== bOnline) return aOnline ? -1 : 1;
      
      // Then by name
      return a.data.name.localeCompare(b.data.name);
    });
  });
  
  // Get column header based on tab
  const getCountHeader = () => {
    switch (props.currentTab) {
      case 'dashboard': return ['VMs', 'Containers'];
      case 'storage': return ['Storage'];
      case 'backups': return ['Backups'];
      default: return [];
    }
  };
  
  // Get count values for a node
  const getNodeCounts = (item: { type: 'pve' | 'pbs'; data: Node | PBSInstance }) => {
    if (item.type === 'pbs') {
      // PBS instances only show backup count
      return [props.backupCounts?.[item.data.name] || 0];
    }
    
    const node = item.data as Node;
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

  // Don't return null - let the table render even if empty
  // This prevents the table from disappearing on refresh while data loads

  return (
    <div class="mb-4 bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
      <div class="overflow-x-auto" style="overflow-x: auto;">
        <table style="width: 1000px; table-layout: fixed;">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 200px; white-space: nowrap;">
                {props.currentTab === 'backups' ? 'Node / PBS' : 'Node'}
              </th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 80px; white-space: nowrap;">Status</th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 100px; white-space: nowrap;">Uptime</th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 200px; white-space: nowrap;">CPU</th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 200px; white-space: nowrap;">Memory</th>
              <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 200px; white-space: nowrap;">
                {props.currentTab === 'backups' && props.pbsInstances ? 'Storage / Disk' : 'Disk'}
              </th>
              <For each={getCountHeader()}>
                {(header) => (
                  <th class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 80px; white-space: nowrap;">{header}</th>
                )}
              </For>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <For each={sortedItems()}>
              {(item) => {
                const isPVE = item.type === 'pve';
                const node = isPVE ? item.data as Node : null;
                const pbs = !isPVE ? item.data as PBSInstance : null;
                
                const isOnline = () => isPVE 
                  ? node!.status === 'online' && node!.uptime > 0
                  : (pbs!.status === 'healthy' || pbs!.status === 'online');
                
                const cpuPercent = () => isPVE 
                  ? Math.round((node!.cpu || 0) * 100)
                  : Math.round(pbs!.cpu || 0);
                
                const memPercent = () => isPVE 
                  ? Math.round(node!.memory?.usage || 0)
                  : (pbs!.memoryTotal ? Math.round((pbs!.memoryUsed / pbs!.memoryTotal) * 100) : 0);
                
                const diskPercent = () => {
                  if (isPVE) {
                    return node!.disk ? Math.round((node!.disk.used / node!.disk.total) * 100) : 0;
                  } else {
                    // Calculate total storage for PBS
                    if (!pbs!.datastores) return 0;
                    const totals = pbs!.datastores.reduce((acc, ds) => {
                      acc.used += ds.used || 0;
                      acc.total += ds.total || 0;
                      return acc;
                    }, { used: 0, total: 0 });
                    return totals.total > 0 ? Math.round((totals.used / totals.total) * 100) : 0;
                  }
                };
                
                const getDiskSublabel = () => {
                  if (isPVE && node!.disk) {
                    return `${formatBytes(node!.disk.used)}/${formatBytes(node!.disk.total)}`;
                  } else if (!isPVE && pbs!.datastores) {
                    const totals = pbs!.datastores.reduce((acc, ds) => {
                      acc.used += ds.used || 0;
                      acc.total += ds.total || 0;
                      return acc;
                    }, { used: 0, total: 0 });
                    return `${formatBytes(totals.used)}/${formatBytes(totals.total)}`;
                  }
                  return undefined;
                };
                
                const nodeId = isPVE ? node!.name : pbs!.name;
                const isSelected = () => props.selectedNode === nodeId;
                
                return (
                  <tr 
                    class={`hover:bg-gray-50 dark:hover:bg-gray-700/30 cursor-pointer transition-colors ${
                      isSelected() ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                    }`}
                    onClick={() => props.onNodeClick(nodeId, item.type)}
                  >
                    <td class="px-2 py-0.5" style="white-space: nowrap;">
                      <div class="flex items-center gap-1" style="display: flex; white-space: nowrap;">
                        <a 
                          href={isPVE ? (node!.host || `https://${node!.name}:8006`) : (pbs!.host || `https://${pbs!.name}:8007`)}
                          target="_blank"
                          onClick={(e) => e.stopPropagation()}
                          class="font-medium text-[11px] text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 whitespace-nowrap"
                        >
                          {item.data.name}
                        </a>
                        <Show when={isPVE && node!.isClusterMember !== undefined}>
                          <span class={`text-[9px] px-1 py-0 rounded text-[8px] font-medium whitespace-nowrap ${
                            node!.isClusterMember 
                              ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' 
                              : 'bg-gray-100 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400'
                          }`}>
                            {node!.isClusterMember ? node!.clusterName : 'Standalone'}
                          </span>
                        </Show>
                        <Show when={!isPVE}>
                          <span class="text-[9px] px-1 py-0 rounded text-[8px] font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
                            PBS
                          </span>
                        </Show>
                        <Show when={!isPVE && pbs!.version}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400">
                            v{pbs!.version}
                          </span>
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-0.5" style="white-space: nowrap;">
                      <div class="flex items-center gap-1" style="display: flex; white-space: nowrap;">
                        <span class={`h-2 w-2 flex-shrink-0 rounded-full ${
                          isOnline() ? 'bg-green-500' : 'bg-red-500'
                        }`} />
                        <span class="text-xs text-gray-600 dark:text-gray-400">
                          {isOnline() ? 'Online' : 'Offline'}
                        </span>
                      </div>
                    </td>
                    <td class="px-2 py-0.5 whitespace-nowrap">
                      <span class={`text-xs ${
                        isPVE && node!.uptime < 3600 ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'
                      }`}>
                        <Show when={isOnline() && (isPVE ? node!.uptime : pbs!.uptime)} fallback="-">
                          {formatUptime(isPVE ? node!.uptime : pbs!.uptime)}
                        </Show>
                      </span>
                    </td>
                    <td class="px-2 py-0.5" style="width: 200px;">
                      <MetricBar 
                        value={cpuPercent()} 
                        label={`${cpuPercent()}%`}
                        sublabel={isPVE && node!.cpuInfo?.cores ? `${node!.cpuInfo.cores} cores` : undefined}
                        type="cpu"
                      />
                    </td>
                    <td class="px-2 py-0.5" style="width: 200px;">
                      <MetricBar 
                        value={memPercent()} 
                        label={`${memPercent()}%`}
                        sublabel={isPVE && node!.memory 
                          ? `${formatBytes(node!.memory.used)}/${formatBytes(node!.memory.total)}`
                          : (!isPVE && pbs!.memoryTotal 
                            ? `${formatBytes(pbs!.memoryUsed)}/${formatBytes(pbs!.memoryTotal)}`
                            : undefined)}
                        type="memory"
                      />
                    </td>
                    <td class="px-2 py-0.5" style="width: 200px;">
                      <MetricBar 
                        value={diskPercent()} 
                        label={`${diskPercent()}%`}
                        sublabel={getDiskSublabel()}
                        type="disk"
                      />
                    </td>
                    <For each={getNodeCounts(item)}>
                      {(count) => (
                        <td class="px-2 py-0.5 whitespace-nowrap text-center">
                          <span class="text-xs text-gray-700 dark:text-gray-300">{count}</span>
                        </td>
                      )}
                    </For>
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