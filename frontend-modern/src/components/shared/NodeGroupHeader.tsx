import { Component, Show } from 'solid-js';
import type { Node } from '@/types/api';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';
import { StatusDot } from '@/components/shared/StatusDot';
import { getNodeStatusIndicator } from '@/utils/status';

interface NodeGroupHeaderProps {
  node: Node;
  colspan?: number;
  renderAs?: 'tr' | 'div';
}

export const NodeGroupHeader: Component<NodeGroupHeaderProps> = (props) => {
  const nodeStatus = () => getNodeStatusIndicator(props.node);
  const isOnline = () => nodeStatus().variant === 'success';
  const nodeUrl = () => props.node.guestURL || props.node.host || `https://${props.node.name}:8006`;
  const displayName = () => getNodeDisplayName(props.node);
  const showActualName = () => hasAlternateDisplayName(props.node);

  const InnerContent = () => (
    <div
      class={`flex flex-wrap items-center gap-3 ${isOnline() ? '' : 'opacity-60'}`}
      title={nodeStatus().label}
    >
      <StatusDot
        variant={nodeStatus().variant}
        title={nodeStatus().label}
        ariaLabel={nodeStatus().label}
        size="xs"
      />
      <a
        href={nodeUrl()}
        target="_blank"
        rel="noopener noreferrer"
        class="transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
        title={`Open ${props.node.name} web interface`}
      >
        {displayName()}
      </a>
      <Show when={showActualName()}>
        <span class="text-[10px] text-muted">({props.node.name})</span>
      </Show>

      <Show when={props.node.isClusterMember !== undefined}>
        <span
          class={`rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${props.node.isClusterMember
            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300'
            : 'bg-slate-200 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
            }`}
        >
          {props.node.isClusterMember ? props.node.clusterName : 'Standalone'}
        </span>
      </Show>

      <Show when={props.node.linkedHostAgentId}>
        <span
          class="inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-400"
          title="Pulse host agent is installed on this node for enhanced metrics (temperatures, detailed disks, RAID status)"
        >
          Agent
        </span>
      </Show>
    </div>
  );

  return (
    <Show
      when={props.renderAs === 'tr'}
      fallback={
        <div class="bg-surface-alt w-full">
          <div class="py-0.5 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-base-content">
            <InnerContent />
          </div>
        </div>
      }
    >
      <tr class="bg-surface-alt">
        <td
          colspan={props.colspan}
          class="py-0.5 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-base-content"
        >
          <InnerContent />
        </td>
      </tr>
    </Show>
  );
};
