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
    
    // Sort by node and devPath
    return disks.sort((a, b) => {
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
        return 'bg-purple-100 text-purple-800';
      case 'sata':
        return 'bg-blue-100 text-blue-800';
      case 'sas':
        return 'bg-indigo-100 text-indigo-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };
  
  return (
    <div class="space-y-4">
      <Show when={filteredDisks().length === 0}>
        <div class="text-center py-8 text-gray-500">
          No physical disks found
          {props.selectedNode && ` for node ${props.selectedNode}`}
          {props.searchTerm && ` matching "${props.searchTerm}"`}
        </div>
      </Show>
      
      <For each={filteredDisks()}>
        {(disk) => {
          const health = getHealthStatus(disk);
          
          return (
            <div class="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow">
              <div class="flex items-start justify-between">
                <div class="flex-1">
                  {/* Header with model and health */}
                  <div class="flex items-center gap-3 mb-2">
                    <span class={`px-2 py-0.5 text-xs font-medium rounded ${health.bgColor} ${health.color}`}>
                      {health.text}
                    </span>
                    <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                      {disk.model || 'Unknown Model'}
                    </h3>
                    <span class={`px-2 py-1 text-xs font-medium rounded-full ${getDiskTypeBadge(disk.type)}`}>
                      {disk.type.toUpperCase()}
                    </span>
                  </div>
                  
                  {/* Disk details */}
                  <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                    <div>
                      <span class="text-gray-500">Node:</span>
                      <span class="ml-2 font-medium">{disk.node}</span>
                    </div>
                    <div>
                      <span class="text-gray-500">Path:</span>
                      <span class="ml-2 font-mono text-xs">{disk.devPath}</span>
                    </div>
                    <div>
                      <span class="text-gray-500">Size:</span>
                      <span class="ml-2 font-medium">{formatBytes(disk.size)}</span>
                    </div>
                    <div>
                      <span class="text-gray-500">Usage:</span>
                      <span class="ml-2">{disk.used || 'Unknown'}</span>
                    </div>
                  </div>
                  
                  {/* Additional metrics for SSDs */}
                  <Show when={disk.wearout > 0}>
                    <div class="mt-3 space-y-2">
                      <div class="flex items-center gap-2">
                        <span class="text-sm text-gray-500">SSD Life Remaining:</span>
                        <div class="flex-1 max-w-xs">
                          <div class="bg-gray-200 rounded-full h-2">
                            <div 
                              class={`h-2 rounded-full transition-all ${
                                disk.wearout >= 50 ? 'bg-green-500' :
                                disk.wearout >= 20 ? 'bg-yellow-500' :
                                disk.wearout >= 10 ? 'bg-orange-500' :
                                'bg-red-500'
                              }`}
                              style={`width: ${disk.wearout}%`}
                            />
                          </div>
                        </div>
                        <span class="text-sm font-medium">{disk.wearout}%</span>
                      </div>
                    </div>
                  </Show>
                  
                  {/* Temperature if available */}
                  <Show when={disk.temperature > 0}>
                    <div class="mt-2 text-sm">
                      <span class="text-gray-500">Temperature:</span>
                      <span class={`ml-2 font-medium ${
                        disk.temperature > 70 ? 'text-red-500' :
                        disk.temperature > 60 ? 'text-yellow-500' :
                        'text-green-500'
                      }`}>
                        {disk.temperature}°C
                      </span>
                    </div>
                  </Show>
                  
                  {/* Serial number (smaller, muted) */}
                  <div class="mt-2 text-xs text-gray-400">
                    Serial: {disk.serial}
                  </div>
                </div>
              </div>
            </div>
          );
        }}
      </For>
    </div>
  );
};