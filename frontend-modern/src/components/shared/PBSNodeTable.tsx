import { Component, For, Show, createMemo } from 'solid-js';
import type { PBSInstance } from '@/types/api';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { formatBytes, formatUptime } from '@/utils/format';

interface PBSNodeTableProps {
  pbsInstances?: PBSInstance[];
  backupCounts?: Record<string, number>;
  selectedNode: string | null;
  onNodeClick: (nodeId: string) => void;
  currentTab?: 'dashboard' | 'storage' | 'backups';
  filteredBackups?: any[];
}

export const PBSNodeTable: Component<PBSNodeTableProps> = (props) => {
  // Filter and sort PBS instances
  const sortedInstances = createMemo(() => {
    if (!props.pbsInstances) return [];
    
    let instances = [...props.pbsInstances];
    
    // If we have filtered backups in backups tab, only show PBS instances with matching backups
    if (props.currentTab === 'backups' && props.filteredBackups !== undefined) {
      const pbsWithBackups = new Set<string>();
      
      props.filteredBackups.forEach(b => {
        // PBS backups can have node as the PBS instance name or 'PBS' generic
        // Check if it's a PBS backup (has datastore or node is PBS instance)
        if (b.datastore || b.node === 'PBS' || b.backupType === 'remote') {
          if (b.node === 'PBS' || !b.node) {
            // Generic PBS backup, show all PBS instances
            props.pbsInstances?.forEach(pbs => pbsWithBackups.add(pbs.name));
          } else if (props.pbsInstances?.some(pbs => pbs.name === b.node)) {
            // Specific PBS instance
            pbsWithBackups.add(b.node);
          }
        }
      });
      
      // If we have any PBS backups, filter to matching instances
      if (pbsWithBackups.size > 0) {
        instances = instances.filter(pbs => pbsWithBackups.has(pbs.name));
      } else {
        // No PBS backups found in the filtered results, hide PBS table
        instances = [];
      }
    }
    
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
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="border-b border-gray-200 dark:border-gray-700">
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                PBS Instances
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Status
              </th>
              <th class="px-2 py-1 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Datastores
              </th>
              <th class="px-2 py-1 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Backups
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[180px]">
                CPU
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[180px]">
                Memory
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[180px]">
                Storage Used
              </th>
              <th class="px-2 py-1 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
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
                
                return (
                  <tr 
                    class={`
                      border-b border-gray-100 dark:border-gray-700/50 transition-colors
                      ${isClickable ? 'hover:bg-gray-50 dark:hover:bg-gray-700/30 cursor-pointer' : ''}
                      ${!isOnline() ? 'opacity-60' : ''}
                      ${isSelected() && isClickable ? 'bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30' : ''}
                    `}
                    onClick={() => isClickable && props.onNodeClick(pbs.name)}
                  >
                    <td class="px-2 py-0.5">
                      <div class="flex items-center gap-2">
                        <span class={`h-2 w-2 rounded-full ${isOnline() ? 'bg-green-500' : 'bg-red-500'}`}></span>
                        <span class="text-sm font-medium text-gray-900 dark:text-gray-100">
                          {pbs.name}
                          <span class="ml-2 text-xs px-1.5 py-0.5 bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300 rounded">
                            PBS
                          </span>
                        </span>
                      </div>
                    </td>
                    <td class="px-2 py-0.5">
                      <span class={`text-xs ${isOnline() ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                        {pbs.status}
                      </span>
                    </td>
                    <td class="px-2 py-0.5 text-center">
                      <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
                        {pbs.datastores?.length || 0}
                      </span>
                    </td>
                    <td class="px-2 py-0.5 text-center">
                      <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
                        {props.backupCounts?.[pbs.name] || 0}
                      </span>
                    </td>
                    <td class="px-2 py-0.5 w-[180px]">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={cpuPercent()} 
                          label={`${cpuPercent()}%`}
                          type="cpu"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5 w-[180px]">
                      <Show when={isOnline()} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={memPercent()} 
                          label={`${memPercent()}%`}
                          sublabel={`${formatBytes(pbs.memoryUsed)} / ${formatBytes(pbs.memoryTotal)}`}
                          type="memory"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5 w-[180px]">
                      <Show when={isOnline() && totalStorage().total > 0} fallback={<span class="text-xs text-gray-400">-</span>}>
                        <MetricBar 
                          value={totalStorage().percent} 
                          label={`${totalStorage().percent}%`}
                          sublabel={`${formatBytes(totalStorage().used)} / ${formatBytes(totalStorage().total)}`}
                          type="disk"
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
                        <Show when={isOnline() && pbs.uptime} fallback="-">
                          {formatUptime(pbs.uptime)}
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