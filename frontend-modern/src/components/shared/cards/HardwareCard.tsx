import { Component } from 'solid-js';
import type { Agent, Node } from '@/types/api';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
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
          <InfoCardKeyValueRow
            label="CPU Model"
            value={node.cpuInfo?.model || 'Unknown'}
            valueClass="truncate"
            valueTitle={node.cpuInfo?.model || 'Unknown'}
          />
          <InfoCardKeyValueRow label="Cores" value={node.cpuInfo?.cores || 0} />
          <InfoCardKeyValueRow label="Memory" value={formatBytes(node.memory?.total || 0)} />
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
        <InfoCardKeyValueRow label="CPU" value={`${agentInfo.cpuCount} Cores`} />
        <InfoCardKeyValueRow label="Memory" value={formatBytes(agentInfo.memory?.total || 0)} />
        <InfoCardKeyValueRow label="Agent" value={agentInfo.agentVersion} />
      </div>
    </InfoCardFrame>
  );
};
