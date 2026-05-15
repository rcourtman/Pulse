import { Component, For, Show, type JSX } from 'solid-js';
import type { Node } from '@/types/api';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';
import { StatusDot } from '@/components/shared/StatusDot';
import { getNodeStatusIndicator } from '@/utils/status';
import { formatUptime } from '@/utils/format';
import { formatTemperature, getCpuTemperature, getTemperatureTextClass } from '@/utils/temperature';
import {
  GROUPED_TABLE_ROW_BADGE_CLASS,
  getGroupedTableRowCellClass,
  getGroupedTableRowClass,
} from './groupedTableRowPresentation';

interface NodeGroupHeaderProps {
  node: Node;
  colspan?: number;
  columns?: Array<{ id: string }>;
  columnCellClass?: (columnId: string, isNameColumn: boolean) => string;
  leadingAction?: JSX.Element;
  renderColumnCell?: (columnId: string, node: Node) => JSX.Element;
  renderAs?: 'tr' | 'div';
  showFactsInName?: boolean;
  trClass?: string;
  trProps?: JSX.HTMLAttributes<HTMLTableRowElement> &
    Partial<Record<`data-${string}`, string | undefined>>;
}

export const NodeGroupHeader: Component<NodeGroupHeaderProps> = (props) => {
  const nodeStatus = () => getNodeStatusIndicator(props.node);
  const isOnline = () => nodeStatus().variant === 'success';
  const nodeUrl = () => props.node.guestURL || props.node.host || `https://${props.node.name}:8006`;
  const displayName = () => getNodeDisplayName(props.node);
  const showActualName = () => hasAlternateDisplayName(props.node);
  const pveVersion = () => {
    const version = (props.node.pveVersion || '').trim();
    if (!version || version.toLowerCase() === 'unknown') return '';
    return (
      version.match(/pve-manager\/([^/\s]+)/i)?.[1] ||
      version.match(/\d+(?:\.\d+)+/)?.[0] ||
      version
    );
  };
  const cpuTemperature = () => getCpuTemperature(props.node.temperature);
  const hasNodeFacts = () =>
    Boolean(pveVersion()) ||
    cpuTemperature() !== null ||
    (typeof props.node.uptime === 'number' && props.node.uptime > 0);

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
      <a
        href={nodeUrl()}
        target="_blank"
        rel="noopener noreferrer"
        class="transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
        title={`Open ${props.node.name} web interface`}
        onClick={(event) => event.stopPropagation()}
      >
        {displayName()}
      </a>
      <Show when={showActualName()}>
        <span class="text-[10px] text-muted">({props.node.name})</span>
      </Show>

      <Show when={props.node.isClusterMember !== undefined}>
        <span
          class={
            props.node.isClusterMember
              ? GROUPED_TABLE_ROW_BADGE_CLASS
              : 'rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted'
          }
        >
          {props.node.isClusterMember ? props.node.clusterName : 'Standalone'}
        </span>
      </Show>

      <Show when={props.node.linkedAgentId}>
        <span
          class="inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-400"
          title="Pulse agent is installed on this node for enhanced metrics (temperatures, detailed disks, RAID status)"
        >
          Agent
        </span>
      </Show>

      <Show when={props.showFactsInName !== false && hasNodeFacts()}>
        <span class="hidden sm:inline-flex flex-wrap items-center gap-3 text-[10px] font-medium text-muted">
          <Show when={pveVersion()}>
            <span title="Proxmox VE version">PVE {pveVersion()}</span>
          </Show>
          <Show when={cpuTemperature() !== null}>
            <span class={getTemperatureTextClass(cpuTemperature())} title="CPU temperature">
              {formatTemperature(cpuTemperature())}
            </span>
          </Show>
          <Show when={typeof props.node.uptime === 'number' && props.node.uptime > 0}>
            <span title="Node uptime">{formatUptime(props.node.uptime, true)}</span>
          </Show>
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
        <Show
          when={(props.columns?.length ?? 0) > 0 && Boolean(props.renderColumnCell)}
          fallback={
            <td colspan={props.colspan} class={getGroupedTableRowCellClass()}>
              <InnerContent />
            </td>
          }
        >
          <For each={props.columns}>
            {(column) => {
              const isNameColumn = () => column.id === 'name';
              return (
                <td
                  data-workload-col={column.id}
                  class={
                    props.columnCellClass?.(column.id, isNameColumn()) ??
                    (isNameColumn()
                      ? getGroupedTableRowCellClass()
                      : 'px-1.5 sm:px-2 py-0.5 align-middle')
                  }
                >
                  <Show
                    when={isNameColumn()}
                    fallback={props.renderColumnCell?.(column.id, props.node)}
                  >
                    <InnerContent />
                  </Show>
                </td>
              );
            }}
          </For>
        </Show>
      </tr>
    </Show>
  );
};
