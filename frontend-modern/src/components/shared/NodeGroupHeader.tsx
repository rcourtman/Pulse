import { Component, Show } from 'solid-js';
import type { Node } from '@/types/api';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';

interface NodeGroupHeaderProps {
  node: Node;
  colspan: number;
}

export const NodeGroupHeader: Component<NodeGroupHeaderProps> = (props) => {
  const isOnline = () => props.node.status === 'online' && (props.node.uptime || 0) > 0;
  const nodeUrl = () => props.node.host || `https://${props.node.name}:8006`;
  const displayName = () => getNodeDisplayName(props.node);
  const showActualName = () => hasAlternateDisplayName(props.node);

  return (
    <tr class="bg-gray-50 dark:bg-gray-900/40">
      <td
        colspan={props.colspan}
        class="py-1 pr-2 pl-3 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-100"
      >
        <div
          class={`flex flex-wrap items-center gap-3 ${
            isOnline() ? '' : 'opacity-60'
          }`}
          title={isOnline() ? 'Online' : 'Offline'}
        >
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
            <span class="text-[10px] text-slate-500 dark:text-slate-400">({props.node.name})</span>
          </Show>

          <Show when={props.node.isClusterMember !== undefined}>
            <span
              class={`rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${
                props.node.isClusterMember
                  ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
                  : 'bg-slate-200 text-slate-600 dark:bg-slate-700/60 dark:text-slate-300'
              }`}
            >
              {props.node.isClusterMember ? props.node.clusterName : 'Standalone'}
            </span>
          </Show>
        </div>
      </td>
    </tr>
  );
};
