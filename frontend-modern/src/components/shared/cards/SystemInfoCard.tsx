import { Component, Show } from 'solid-js';
import type { Agent, Node } from '@/types/api';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
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
          <InfoCardKeyValueRow
            label="Node"
            value={node.name}
            valueClass="select-all truncate"
            valueTitle={node.name}
          />
          <InfoCardKeyValueRow
            label="Version"
            value={node.pveVersion}
            valueClass="truncate"
            valueTitle={node.pveVersion}
          />
          <InfoCardKeyValueRow
            label="Kernel"
            value={node.kernelVersion}
            valueClass="truncate"
            valueTitle={node.kernelVersion}
          />
          <Show when={node.uptime}>
            <InfoCardKeyValueRow label="Uptime" value={formatUptime(node.uptime!)} />
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
        <InfoCardKeyValueRow
          label="Hostname"
          value={agentInfo.hostname}
          valueClass="select-all truncate"
          valueTitle={agentInfo.hostname}
        />
        <InfoCardKeyValueRow
          label="OS"
          value={
            agentInfo.osName
              ? `${agentInfo.osName}${agentInfo.osVersion ? ` ${agentInfo.osVersion}` : ''}`
              : agentInfo.platform || 'Unknown'
          }
          valueClass="truncate"
          valueTitle={
            agentInfo.osName
              ? `${agentInfo.osName}${agentInfo.osVersion ? ` ${agentInfo.osVersion}` : ''}`
              : agentInfo.platform || 'Unknown'
          }
        />
        <InfoCardKeyValueRow
          label="Kernel"
          value={agentInfo.kernelVersion}
          valueClass="truncate"
          valueTitle={agentInfo.kernelVersion}
        />
        <InfoCardKeyValueRow label="Architecture" value={agentInfo.architecture} />
        <Show when={agentInfo.uptimeSeconds}>
          <InfoCardKeyValueRow label="Uptime" value={formatUptime(agentInfo.uptimeSeconds!)} />
        </Show>
      </div>
    </InfoCardFrame>
  );
};
