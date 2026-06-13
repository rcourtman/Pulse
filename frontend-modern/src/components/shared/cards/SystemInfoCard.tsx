import { Component, Show } from 'solid-js';
import type { Agent, Node } from '@/types/api';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { formatUptime } from '@/utils/format';

type SystemInfoCardProps = { variant: 'node'; node: Node } | { variant: 'agent'; agent: Agent };

export const SystemInfoCard: Component<SystemInfoCardProps> = (props) => {
  if (props.variant === 'node') {
    const node = props.node;
    return (
      <InfoCardFrame>
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
      </InfoCardFrame>
    );
  }

  const agentInfo = props.agent;
  return (
    <InfoCardFrame>
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        System
      </div>
      <div class="space-y-1.5 text-[11px]">
        <div class="flex items-center justify-between gap-2 min-w-0">
          <span class="text-muted shrink-0">Hostname</span>
          <span
            class="font-medium text-base-content select-all truncate"
            title={agentInfo.hostname}
          >
            {agentInfo.hostname}
          </span>
        </div>
        <div class="flex items-center justify-between gap-2 min-w-0">
          <span class="text-muted shrink-0">OS</span>
          <span
            class="font-medium text-base-content truncate"
            title={
              agentInfo.osName
                ? `${agentInfo.osName}${agentInfo.osVersion ? ` ${agentInfo.osVersion}` : ''}`
                : agentInfo.platform || 'Unknown'
            }
          >
            {agentInfo.osName
              ? `${agentInfo.osName}${agentInfo.osVersion ? ` ${agentInfo.osVersion}` : ''}`
              : agentInfo.platform || 'Unknown'}
          </span>
        </div>
        <div class="flex items-center justify-between gap-2 min-w-0">
          <span class="text-muted shrink-0">Kernel</span>
          <span class="font-medium text-base-content truncate" title={agentInfo.kernelVersion}>
            {agentInfo.kernelVersion}
          </span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">Architecture</span>
          <span class="font-medium text-base-content">{agentInfo.architecture}</span>
        </div>
        <Show when={agentInfo.uptimeSeconds}>
          <div class="flex items-center justify-between">
            <span class="text-muted">Uptime</span>
            <span class="font-medium text-base-content">
              {formatUptime(agentInfo.uptimeSeconds!)}
            </span>
          </div>
        </Show>
      </div>
    </InfoCardFrame>
  );
};
