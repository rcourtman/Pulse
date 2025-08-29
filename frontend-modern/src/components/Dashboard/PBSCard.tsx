import { Component, Show, createMemo } from 'solid-js';
import type { PBSInstance } from '@/types/api';
import { formatUptime, formatBytes } from '@/utils/format';

interface PBSCardProps {
  instance: PBSInstance;
  isSelected?: boolean;
}

const PBSCard: Component<PBSCardProps> = (props) => {
  const isOnline = () => props.instance.status === 'online';
  
  const hasSystemStats = () => {
    return props.instance.cpu > 0 || props.instance.memory > 0 || props.instance.uptime > 0;
  };

  // Detect if PBS is running in Docker (no system stats available)
  const isDockerPBS = () => {
    // PBS in Docker typically has "docker" in the name
    // pbs-docker has datastores but is still Docker, so check the name
    return props.instance.status === 'online' && 
           props.instance.version && 
           !hasSystemStats() &&
           props.instance.name.toLowerCase().includes('docker');
  };

  // Calculate percentages
  const cpuPercent = createMemo(() => Math.round(props.instance.cpu || 0));
  
  const memPercent = createMemo(() => Math.round(props.instance.memory || 0));
  
  const diskPercent = createMemo(() => {
    if (!props.instance.datastores || props.instance.datastores.length === 0) return 0;
    
    let totalUsed = 0;
    let totalSpace = 0;
    
    props.instance.datastores.forEach(ds => {
      totalUsed += ds.used || 0;
      totalSpace += ds.total || 0;
    });
    
    return totalSpace > 0 ? Math.round((totalUsed / totalSpace) * 100) : 0;
  });

  // Calculate total disk usage from datastores
  const diskUsage = createMemo(() => {
    if (!props.instance.datastores || props.instance.datastores.length === 0) {
      return { used: 0, total: 0 };
    }
    
    let totalUsed = 0;
    let totalSpace = 0;
    
    props.instance.datastores.forEach(ds => {
      totalUsed += ds.used || 0;
      totalSpace += ds.total || 0;
    });
    
    return { used: totalUsed, total: totalSpace };
  });

  // Helper function to create progress bar with text overlay (matching NodeCard)
  const createProgressBar = (percentage: number, text: string, colorClass: string) => {
    const bgColorClass = 'bg-gray-200 dark:bg-gray-600';
    const progressColorClass = {
      'red': 'bg-red-500/60 dark:bg-red-500/50',
      'yellow': 'bg-yellow-500/60 dark:bg-yellow-500/50',
      'green': 'bg-green-500/60 dark:bg-green-500/50'
    }[colorClass] || 'bg-gray-500/60 dark:bg-gray-500/50';
    
    return (
      <div class={`relative w-[180px] h-3.5 rounded overflow-hidden ${bgColorClass}`}>
        <div class={`absolute top-0 left-0 h-full ${progressColorClass}`} style={{ width: `${percentage}%` }} />
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
          <span class="truncate px-1">{text}</span>
        </span>
      </div>
    );
  };

  // Get color based on percentage and metric type
  const getColor = (percentage: number, metric: 'cpu' | 'memory' | 'disk') => {
    if (metric === 'cpu') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    } else if (metric === 'memory') {
      if (percentage >= 85) return 'red';
      if (percentage >= 75) return 'yellow';
      return 'green';
    } else if (metric === 'disk') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    }
    return 'green';
  };

  // Format text for progress bars
  const cpuText = () => `${cpuPercent()}%`;
  
  const memoryText = () => {
    if (!props.instance.memoryTotal || props.instance.memoryTotal === 0) return `${memPercent()}%`;
    return `${memPercent()}% (${formatBytes(props.instance.memoryUsed)}/${formatBytes(props.instance.memoryTotal)})`;
  };
  
  const diskText = () => {
    const usage = diskUsage();
    if (usage.total === 0) return '0%';
    return `${diskPercent()}% (${formatBytes(usage.used)}/${formatBytes(usage.total)})`;
  };

  const getBorderClass = () => {
    // Selected PBS gets blue ring
    if (props.isSelected) {
      return 'ring-2 ring-blue-500 border border-gray-200 dark:border-gray-700';
    }
    // Offline PBS gets red ring
    if (!isOnline()) {
      return 'ring-2 ring-red-500 border border-gray-200 dark:border-gray-700';
    }
    // Normal PBS gets standard border
    return 'border border-gray-200 dark:border-gray-700';
  };

  return (
    <div class={`bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 flex flex-col gap-1 w-[250px] ${getBorderClass()}`}>
      {/* Header */}
      <div class="flex justify-between items-center">
        <h3 class="text-sm font-semibold truncate text-gray-800 dark:text-gray-200 flex items-center gap-2">
          <a 
            href={props.instance.host} 
            target="_blank" 
            rel="noopener noreferrer" 
            class="hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer"
            title={`Open ${props.instance.name} web interface`}
          >
            {props.instance.name}
          </a>
        </h3>
        <div class="flex items-center">
          <span class={`h-2.5 w-2.5 rounded-full mr-1.5 flex-shrink-0 ${
            isOnline() ? 'bg-green-500' : 'bg-red-500'
          }`} />
          <span class="text-xs capitalize text-gray-600 dark:text-gray-400">
            {isOnline() ? 'online' : props.instance.status || 'unknown'}
          </span>
        </div>
      </div>

      <Show when={hasSystemStats()}>
        {/* CPU */}
        <div class="text-[11px] text-gray-600 dark:text-gray-400">
          <span class="font-medium">CPU:</span>
          {createProgressBar(cpuPercent(), cpuText(), getColor(cpuPercent(), 'cpu'))}
        </div>

        {/* Memory */}
        <div class="text-[11px] text-gray-600 dark:text-gray-400">
          <span class="font-medium">Mem:</span>
          {createProgressBar(memPercent(), memoryText(), getColor(memPercent(), 'memory'))}
        </div>

        {/* Disk */}
        <Show when={diskUsage().total > 0}>
          <div class="text-[11px] text-gray-600 dark:text-gray-400">
            <span class="font-medium">Disk:</span>
            {createProgressBar(diskPercent(), diskText(), getColor(diskPercent(), 'disk'))}
          </div>
        </Show>

        {/* Footer Info */}
        <div class="flex justify-between text-[11px] text-gray-500 dark:text-gray-400 pt-0.5">
          <span>Uptime: {formatUptime(props.instance.uptime)}</span>
          <span>PBS v{props.instance.version}</span>
        </div>
      </Show>

      <Show when={!hasSystemStats()}>
        <Show when={isDockerPBS()}>
          {/* For Docker PBS, show available datastore info */}
          <div class="text-[11px] text-gray-600 dark:text-gray-400">
            <Show when={diskUsage().total > 0}>
              <div>
                <span class="font-medium">Storage:</span>
                {createProgressBar(diskPercent(), diskText(), getColor(diskPercent(), 'disk'))}
              </div>
            </Show>
            <Show when={props.instance.datastores && props.instance.datastores.length > 0}>
              <div class="flex justify-between pt-0.5">
                <span>
                  <span class="font-medium">Datastores: </span>
                  <span class="text-gray-500">{props.instance.datastores.length}</span>
                </span>
                <Show when={(() => {
                  const namespaceCount = props.instance.datastores?.reduce((acc, ds) => 
                    acc + (ds.namespaces?.filter(ns => ns.path !== '').length || 0), 0) || 0;
                  return namespaceCount > 0;
                })()}>
                  <span class="text-gray-500">
                    {(() => {
                      const count = props.instance.datastores?.reduce((acc, ds) => 
                        acc + (ds.namespaces?.filter(ns => ns.path !== '').length || 0), 0) || 0;
                      return `${count} namespace${count !== 1 ? 's' : ''}`;
                    })()}
                  </span>
                </Show>
              </div>
            </Show>
            <div class="flex justify-between text-gray-500 dark:text-gray-400 pt-0.5">
              <span>PBS v{props.instance.version}</span>
              <span class="text-[10px]">(Docker)</span>
            </div>
          </div>
        </Show>
        <Show when={!isDockerPBS()}>
          {/* For non-Docker PBS, show available info */}
          <div class="text-[11px] text-gray-600 dark:text-gray-400">
            {/* Show disk usage if datastores exist */}
            <Show when={diskUsage().total > 0}>
              <div>
                <span class="font-medium">Storage:</span>
                {createProgressBar(diskPercent(), diskText(), getColor(diskPercent(), 'disk'))}
              </div>
            </Show>
            
            <Show when={props.instance.datastores && props.instance.datastores.length > 0}>
              <div class="flex justify-between pt-0.5">
                <span>
                  <span class="font-medium">Datastores: </span>
                  <span class="text-gray-500">{props.instance.datastores.length}</span>
                </span>
                <Show when={(() => {
                  const namespaceCount = props.instance.datastores?.reduce((acc, ds) => 
                    acc + (ds.namespaces?.filter(ns => ns.path !== '').length || 0), 0) || 0;
                  return namespaceCount > 0;
                })()}>
                  <span class="text-gray-500">
                    {(() => {
                      const count = props.instance.datastores?.reduce((acc, ds) => 
                        acc + (ds.namespaces?.filter(ns => ns.path !== '').length || 0), 0) || 0;
                      return `${count} namespace${count !== 1 ? 's' : ''}`;
                    })()}
                  </span>
                </Show>
              </div>
            </Show>
            
            <div class="flex justify-between text-[11px] text-gray-500 dark:text-gray-400 pt-0.5">
              <span>PBS v{props.instance.version}</span>
              <span class="text-[10px] italic">No Sys.Audit permission</span>
            </div>
          </div>
        </Show>
      </Show>
    </div>
  );
};

export default PBSCard;