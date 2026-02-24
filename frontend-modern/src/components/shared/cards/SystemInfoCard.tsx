import { Component, Show } from 'solid-js';
import { Host, Node } from '@/types/api';
import { formatUptime } from '@/utils/format';

type SystemInfoCardProps = { variant: 'node'; node: Node } | { variant: 'host'; host: Host };

export const SystemInfoCard: Component<SystemInfoCardProps> = (props) => {
  if (props.variant === 'node') {
    const node = props.node;
    return (
      <div class="rounded border border-border bg-surface p-3 shadow-sm">
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          System
        </div>
        <div class="space-y-1.5 text-[11px]">
          <div class="flex items-center justify-between gap-2 min-w-0">
            <span class="text-muted shrink-0">Node</span>
            <span class="font-medium text-base-content select-all truncate" title={node.name}>
              {node.name}
            </span>
          </div>
          <div class="flex items-center justify-between gap-2 min-w-0">
            <span class="text-muted shrink-0">Version</span>
            <span class="font-medium text-base-content truncate" title={node.pveVersion}>
              {node.pveVersion}
            </span>
          </div>
          <div class="flex items-center justify-between gap-2 min-w-0">
            <span class="text-muted shrink-0">Kernel</span>
            <span class="font-medium text-base-content truncate" title={node.kernelVersion}>
              {node.kernelVersion}
            </span>
          </div>
          <Show when={node.uptime}>
            <div class="flex items-center justify-between">
              <span class="text-muted">Uptime</span>
              <span class="font-medium text-base-content">{formatUptime(node.uptime!)}</span>
            </div>
          </Show>
        </div>
      </div>
    );
  }

  const host = props.host;
  return (
    <div class="rounded border border-border bg-surface p-3 shadow-sm">
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        System
      </div>
      <div class="space-y-1.5 text-[11px]">
        <div class="flex items-center justify-between gap-2 min-w-0">
          <span class="text-muted shrink-0">Hostname</span>
          <span class="font-medium text-base-content select-all truncate" title={host.hostname}>
            {host.hostname}
          </span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">Platform</span>
          <span class="font-medium text-base-content capitalize">{host.platform || 'Unknown'}</span>
        </div>
        <div class="flex items-center justify-between gap-2 min-w-0">
          <span class="text-muted shrink-0">OS</span>
          <span
            class="font-medium text-base-content truncate"
            title={`${host.osName} ${host.osVersion}`}
          >
            {host.osName} {host.osVersion}
          </span>
        </div>
        <div class="flex items-center justify-between gap-2 min-w-0">
          <span class="text-muted shrink-0">Kernel</span>
          <span class="font-medium text-base-content truncate" title={host.kernelVersion}>
            {host.kernelVersion}
          </span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">Architecture</span>
          <span class="font-medium text-base-content">{host.architecture}</span>
        </div>
        <Show when={host.uptimeSeconds}>
          <div class="flex items-center justify-between">
            <span class="text-muted">Uptime</span>
            <span class="font-medium text-base-content">{formatUptime(host.uptimeSeconds!)}</span>
          </div>
        </Show>
      </div>
    </div>
  );
};
