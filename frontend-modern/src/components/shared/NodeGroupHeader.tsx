import { Component, Show, type JSX } from 'solid-js';
import type { Node } from '@/types/api';
import { getNodeDisplayName, getNodeExternalUrl, hasAlternateDisplayName } from '@/utils/nodes';
import { StatusDot } from '@/components/shared/StatusDot';
import { getNodeStatusIndicator } from '@/utils/status';
import {
  GROUPED_TABLE_ROW_BADGE_CLASS,
  getGroupedTableRowCellClass,
  getGroupedTableRowClass,
} from './groupedTableRowPresentation';
import { ResourceNameWithWebInterfaceLink } from './WebInterfaceLink';

interface NodeGroupHeaderProps {
  node: Node;
  colspan?: number;
  leadingAction?: JSX.Element;
  renderAs?: 'tr' | 'div';
  trClass?: string;
  trProps?: JSX.HTMLAttributes<HTMLTableRowElement> &
    Partial<Record<`data-${string}`, string | undefined>>;
}

export const NodeGroupHeader: Component<NodeGroupHeaderProps> = (props) => {
  const nodeStatus = () => getNodeStatusIndicator(props.node);
  const isOnline = () => nodeStatus().variant === 'success';
  const nodeIsStale = () => {
    const connection = (props.node.connectionHealth || '').trim().toLowerCase();
    const status = (props.node.status || '').trim().toLowerCase();
    return connection === 'stale' || status === 'stale';
  };
  const nodeUrl = () => getNodeExternalUrl(props.node);
  const displayName = () => getNodeDisplayName(props.node);
  const showActualName = () => hasAlternateDisplayName(props.node);

  const InnerContent = () => (
    <div
      class={`flex flex-wrap items-center gap-3 ${isOnline() ? '' : 'opacity-60'}`}
      title={nodeStatus().label}
    >
      {props.leadingAction}
      <StatusDot
        variant={nodeStatus().variant}
        title={nodeStatus().label}
        ariaLabel={nodeStatus().label}
        size="xs"
      />
      <ResourceNameWithWebInterfaceLink
        name={displayName()}
        url={nodeUrl()}
        nameClass="text-base-content"
        title={`Open ${props.node.name} web interface`}
      />
      <Show when={showActualName()}>
        <span class="text-[10px] text-muted">({props.node.name})</span>
      </Show>

      <Show when={props.node.isClusterMember !== undefined}>
        <span
          class={
            props.node.isClusterMember
              ? GROUPED_TABLE_ROW_BADGE_CLASS
              : 'rounded-sm px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted'
          }
        >
          {props.node.isClusterMember ? props.node.clusterName : 'Standalone'}
        </span>
      </Show>

      <Show when={nodeIsStale()}>
        <span
          data-node-group-stale
          class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap text-amber-800 dark:bg-amber-900/30 dark:text-amber-300"
          title="Node telemetry is stale. Workload readings below reflect the last poll received."
        >
          Stale data
        </span>
      </Show>

      <Show when={props.node.linkedAgentId}>
        <span
          class="inline-flex items-center rounded-sm px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-emerald-100 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-400"
          title="Pulse agent is installed on this node for enhanced metrics (temperatures, detailed disks, RAID status)"
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
          <div class={getGroupedTableRowCellClass()}>
            <InnerContent />
          </div>
        </div>
      }
    >
      <tr class={getGroupedTableRowClass(props.trClass)} {...props.trProps}>
        <td colspan={props.colspan} class={getGroupedTableRowCellClass()}>
          <InnerContent />
        </td>
      </tr>
    </Show>
  );
};
