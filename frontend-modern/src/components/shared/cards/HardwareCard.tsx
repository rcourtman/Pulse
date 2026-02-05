import { Component } from 'solid-js';
import { Host, Node } from '@/types/api';
import { formatBytes } from '@/utils/format';

type HardwareCardProps =
  | { variant: 'node'; node: Node }
  | { variant: 'host'; host: Host };

export const HardwareCard: Component<HardwareCardProps> = (props) => {
  if (props.variant === 'node') {
    const node = props.node;
    return (
      <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Hardware</div>
        <div class="space-y-1.5 text-[11px]">
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">CPU Model</span>
            <div class="font-medium text-gray-700 dark:text-gray-200 text-right truncate max-w-[150px]" title={node.cpuInfo?.model || 'Unknown'}>
              {node.cpuInfo?.model || 'Unknown'}
            </div>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">Cores</span>
            <span class="font-medium text-gray-700 dark:text-gray-200">{node.cpuInfo?.cores || 0}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">Memory</span>
            <span class="font-medium text-gray-700 dark:text-gray-200">
              {formatBytes(node.memory?.total || 0)}
            </span>
          </div>
        </div>
      </div>
    );
  }

  const host = props.host;
  return (
    <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
      <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Hardware</div>
      <div class="space-y-1.5 text-[11px]">
        <div class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-gray-400">CPU</span>
          <span class="font-medium text-gray-700 dark:text-gray-200">{host.cpuCount} Cores</span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-gray-400">Memory</span>
          <span class="font-medium text-gray-700 dark:text-gray-200">
            {formatBytes(host.memory?.total || 0)}
          </span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-gray-400">Agent</span>
          <span class="font-medium text-gray-700 dark:text-gray-200">{host.agentVersion}</span>
        </div>
      </div>
    </div>
  );
};
