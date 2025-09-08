import { Component, createMemo, Show } from 'solid-js';
import type { PhysicalDisk } from '@/types/api';

interface DiskHealthSummaryProps {
  disks: PhysicalDisk[];
}

export const DiskHealthSummary: Component<DiskHealthSummaryProps> = (props) => {
  const diskStats = createMemo(() => {
    const disks = props.disks || [];
    const total = disks.length;
    const healthy = disks.filter(d => d.health === 'PASSED').length;
    const failing = disks.filter(d => d.health === 'FAILED').length;
    const unknown = disks.filter(d => d.health === 'UNKNOWN' || !d.health).length;
    const lowLife = disks.filter(d => d.wearout > 0 && d.wearout < 10).length;
    const avgWearout = disks.filter(d => d.wearout > 0).reduce((sum, d) => sum + d.wearout, 0) / 
                       disks.filter(d => d.wearout > 0).length || 0;
    
    // Group by node
    const byNode: Record<string, number> = {};
    disks.forEach(d => {
      byNode[d.node] = (byNode[d.node] || 0) + 1;
    });
    
    return {
      total,
      healthy,
      failing,
      unknown,
      lowLife,
      avgWearout,
      byNode
    };
  });
  
  const healthColor = createMemo(() => {
    const stats = diskStats();
    if (stats.failing > 0) return 'text-red-600 dark:text-red-400';
    if (stats.lowLife > 0) return 'text-yellow-600 dark:text-yellow-400';
    if (stats.unknown > 0) return 'text-gray-600 dark:text-gray-400';
    return 'text-green-600 dark:text-green-400';
  });
  
  const healthBg = createMemo(() => {
    const stats = diskStats();
    if (stats.failing > 0) return 'bg-red-50 dark:bg-red-950/20 border-red-200 dark:border-red-800';
    if (stats.lowLife > 0) return 'bg-yellow-50 dark:bg-yellow-950/20 border-yellow-200 dark:border-yellow-800';
    return 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700';
  });
  
  return (
    <Show when={diskStats().total > 0}>
      <div class={`rounded-lg p-4 border ${healthBg()}`}>
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
            Disk Health Summary
          </h3>
          <span class={`text-2xl font-bold ${healthColor()}`}>
            {diskStats().healthy}/{diskStats().total}
          </span>
        </div>
        
        <div class="space-y-2">
          {/* Health Status */}
          <div class="flex items-center justify-between text-xs">
            <span class="text-gray-600 dark:text-gray-400">Status</span>
            <div class="flex gap-2">
              <Show when={diskStats().healthy > 0}>
                <span class="px-1.5 py-0.5 text-xs rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400">
                  {diskStats().healthy} healthy
                </span>
              </Show>
              <Show when={diskStats().failing > 0}>
                <span class="px-1.5 py-0.5 text-xs rounded bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400">
                  {diskStats().failing} failed
                </span>
              </Show>
              <Show when={diskStats().lowLife > 0}>
                <span class="px-1.5 py-0.5 text-xs rounded bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400">
                  {diskStats().lowLife} low
                </span>
              </Show>
              <Show when={diskStats().unknown > 0}>
                <span class="px-1.5 py-0.5 text-xs rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-400">
                  {diskStats().unknown} unknown
                </span>
              </Show>
            </div>
          </div>
          
          {/* Average SSD Life */}
          <Show when={diskStats().avgWearout > 0}>
            <div class="flex items-center justify-between text-xs">
              <span class="text-gray-600 dark:text-gray-400">Avg SSD Life</span>
              <div class="flex items-center gap-2">
                <div class="w-24 bg-gray-200 dark:bg-gray-700 rounded-full h-1.5">
                  <div 
                    class={`h-1.5 rounded-full transition-all ${
                      diskStats().avgWearout >= 50 ? 'bg-green-500' :
                      diskStats().avgWearout >= 20 ? 'bg-yellow-500' :
                      diskStats().avgWearout >= 10 ? 'bg-orange-500' :
                      'bg-red-500'
                    }`}
                    style={`width: ${diskStats().avgWearout}%`}
                  />
                </div>
                <span class="text-gray-700 dark:text-gray-300 font-medium">
                  {Math.round(diskStats().avgWearout)}%
                </span>
              </div>
            </div>
          </Show>
          
          {/* Disk Distribution */}
          <div class="pt-2 mt-2 border-t border-gray-200 dark:border-gray-700">
            <div class="text-xs text-gray-600 dark:text-gray-400 mb-1">Distribution</div>
            <div class="flex flex-wrap gap-1">
              {Object.entries(diskStats().byNode).map(([node, count]) => (
                <span class="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-xs">
                  {node}: {count}
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>
    </Show>
  );
};