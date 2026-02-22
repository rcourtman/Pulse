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
      <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">Hardware</div>
        <div class="space-y-1.5 text-[11px]">
          <div class="flex items-center justify-between">
            <span class="text-muted">CPU Model</span>
            <div class="font-medium text-base-content text-right truncate max-w-[150px]" title={node.cpuInfo?.model || 'Unknown'}>
              {node.cpuInfo?.model || 'Unknown'}
            </div>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-muted">Cores</span>
            <span class="font-medium text-base-content">{node.cpuInfo?.cores || 0}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-muted">Memory</span>
            <span class="font-medium text-base-content">
              {formatBytes(node.memory?.total || 0)}
            </span>
          </div>
        </div>
      </div>
    );
  }

  const host = props.host;
  return (
    <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">Hardware</div>
      <div class="space-y-1.5 text-[11px]">
        <div class="flex items-center justify-between">
          <span class="text-muted">CPU</span>
          <span class="font-medium text-base-content">{host.cpuCount} Cores</span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">Memory</span>
          <span class="font-medium text-base-content">
            {formatBytes(host.memory?.total || 0)}
          </span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">Agent</span>
          <span class="font-medium text-base-content">{host.agentVersion}</span>
        </div>
      </div>
    </div>
  );
};
