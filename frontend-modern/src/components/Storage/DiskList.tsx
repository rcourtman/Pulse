import { Component, For, Show, createMemo } from 'solid-js';
import { formatBytes } from '@/utils/format';
import type { PhysicalDisk } from '@/types/api';

interface DiskListProps {
  disks: PhysicalDisk[];
  selectedNode: string | null;
  searchTerm: string;
}

export const DiskList: Component<DiskListProps> = (props) => {
  // Filter disks based on selected node and search term
  const filteredDisks = createMemo(() => {
    let disks = props.disks || [];
    
    // Filter by node if selected
    if (props.selectedNode) {
      disks = disks.filter(d => d.node === props.selectedNode);
    }
    
    // Filter by search term
    if (props.searchTerm) {
      const term = props.searchTerm.toLowerCase();
      disks = disks.filter(d => 
        d.model.toLowerCase().includes(term) ||
        d.devPath.toLowerCase().includes(term) ||
        d.serial.toLowerCase().includes(term) ||
        d.node.toLowerCase().includes(term)
      );
    }
    
    // Sort by node and devPath - create a copy to avoid mutating store
    return [...disks].sort((a, b) => {
      if (a.node !== b.node) return a.node.localeCompare(b.node);
      return a.devPath.localeCompare(b.devPath);
    });
  });
  
  // Get health status color and badge
  const getHealthStatus = (disk: PhysicalDisk) => {
    if (disk.health === 'PASSED') {
      // Check wearout for SSDs
      if (disk.wearout > 0 && disk.wearout < 10) {
        return { 
          color: 'text-yellow-700 dark:text-yellow-400', 
          bgColor: 'bg-yellow-100 dark:bg-yellow-900/30',
          text: 'LOW LIFE' 
        };
      }
      return { 
        color: 'text-green-700 dark:text-green-400', 
        bgColor: 'bg-green-100 dark:bg-green-900/30',
        text: 'HEALTHY' 
      };
    } else if (disk.health === 'FAILED') {
      return { 
        color: 'text-red-700 dark:text-red-400', 
        bgColor: 'bg-red-100 dark:bg-red-900/30',
        text: 'FAILED' 
      };
    }
    return { 
      color: 'text-gray-700 dark:text-gray-400', 
      bgColor: 'bg-gray-100 dark:bg-gray-700',
      text: 'UNKNOWN' 
    };
  };
  
  // Get disk type badge color
  const getDiskTypeBadge = (type: string) => {
    switch (type.toLowerCase()) {
      case 'nvme':
        return 'bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300';
      case 'sata':
        return 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300';
      case 'sas':
        return 'bg-indigo-100 dark:bg-indigo-900/30 text-indigo-800 dark:text-indigo-300';
      default:
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
    }
  };
  
  return (
    <div>
      <Show when={filteredDisks().length === 0}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center text-gray-500">
            No physical disks found
            {props.selectedNode && ` for node ${props.selectedNode}`}
            {props.searchTerm && ` matching "${props.searchTerm}"`}
          </div>
        </div>
      </Show>
      
      <Show when={filteredDisks().length > 0}>
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
          <div class="overflow-x-auto">
            <table class="w-full">
              <thead>
                <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Node</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Device</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Model</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Type</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Health</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">SSD Life</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden sm:table-cell">Temp</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Size</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={filteredDisks()}>
                  {(disk) => {
                    const health = getHealthStatus(disk);
                    
                    return (
                      <tr class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                        <td class="px-2 py-1.5 text-xs">
                          <span class="font-medium text-gray-900 dark:text-gray-100">{disk.node}</span>
                        </td>
                        <td class="px-2 py-1.5 text-xs">
                          <span class="font-mono text-gray-600 dark:text-gray-400">{disk.devPath}</span>
                        </td>
                        <td class="px-2 py-1.5 text-xs">
                          <span class="text-gray-700 dark:text-gray-300">{disk.model || 'Unknown'}</span>
                        </td>
                        <td class="px-2 py-1.5 text-xs">
                          <span class={`inline-block px-1.5 py-0.5 text-[10px] font-medium rounded ${getDiskTypeBadge(disk.type)}`}>
                            {disk.type.toUpperCase()}
                          </span>
                        </td>
                        <td class="px-2 py-1.5 text-xs">
                          <span class={`inline-block px-1.5 py-0.5 text-[10px] font-medium rounded ${health.bgColor} ${health.color}`}>
                            {health.text}
                          </span>
                        </td>
                        <td class="px-2 py-1.5 text-xs">
                          <Show when={disk.wearout > 0} fallback={<span class="text-gray-400">-</span>}>
                            <div class="relative w-24 h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600">
                              <div 
                                class={`absolute top-0 left-0 h-full ${
                                  disk.wearout >= 50 ? 'bg-green-500/60 dark:bg-green-500/50' :
                                  disk.wearout >= 20 ? 'bg-yellow-500/60 dark:bg-yellow-500/50' :
                                  disk.wearout >= 10 ? 'bg-orange-500/60 dark:bg-orange-500/50' :
                                  'bg-red-500/60 dark:bg-red-500/50'
                                }`}
                                style={{ width: `${disk.wearout}%` }}
                              />
                              <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                                <span class="whitespace-nowrap px-0.5">
                                  {disk.wearout}%
                                </span>
                              </span>
                            </div>
                          </Show>
                        </td>
                        <td class="px-2 py-1.5 text-xs hidden sm:table-cell">
                          <Show when={disk.temperature > 0} fallback={<span class="text-gray-400">-</span>}>
                            <span class={`font-medium ${
                              disk.temperature > 70 ? 'text-red-600 dark:text-red-400' :
                              disk.temperature > 60 ? 'text-yellow-600 dark:text-yellow-400' :
                              'text-gray-600 dark:text-gray-400'
                            }`}>
                              {disk.temperature}°C
                            </span>
                          </Show>
                        </td>
                        <td class="px-2 py-1.5 text-xs">
                          <span class="text-gray-700 dark:text-gray-300">{formatBytes(disk.size)}</span>
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
    </div>
  );
};