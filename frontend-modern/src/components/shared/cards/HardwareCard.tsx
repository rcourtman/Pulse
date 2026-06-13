import { Component } from 'solid-js';
import type { Agent, Node } from '@/types/api';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { formatBytes } from '@/utils/format';

type HardwareCardProps = { variant: 'node'; node: Node } | { variant: 'agent'; agent: Agent };

export const HardwareCard: Component<HardwareCardProps> = (props) => {
  if (props.variant === 'node') {
    const node = props.node;
    return (
      <InfoCardFrame>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          Hardware
        </div>
        <div class="space-y-1.5 text-[11px]">
          <div class="flex items-center justify-between">
            <span class="text-muted">CPU Model</span>
            <div
              class="font-medium text-base-content text-right truncate max-w-[150px]"
              title={node.cpuInfo?.model || 'Unknown'}
            >
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
      </InfoCardFrame>
    );
  }

  const agentInfo = props.agent;
  return (
    <InfoCardFrame>
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        Hardware
      </div>
      <div class="space-y-1.5 text-[11px]">
        <div class="flex items-center justify-between">
          <span class="text-muted">CPU</span>
          <span class="font-medium text-base-content">{agentInfo.cpuCount} Cores</span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">Memory</span>
          <span class="font-medium text-base-content">
            {formatBytes(agentInfo.memory?.total || 0)}
          </span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">Agent</span>
          <span class="font-medium text-base-content">{agentInfo.agentVersion}</span>
        </div>
      </div>
    </InfoCardFrame>
  );
};
