import { Component, Show } from 'solid-js';
import type { Node } from '@/types/api';

interface NodeGroupHeaderProps {
  node: Node;
  colspan: number;
}

export const NodeGroupHeader: Component<NodeGroupHeaderProps> = (props) => {
  const isOnline = () => props.node.status === 'online' && (props.node.uptime || 0) > 0;
  const nodeUrl = () => props.node.host || `https://${props.node.name}:8006`;

  return (
    <tr class="relative">
      <td colspan={props.colspan} class="py-1 px-3">
        <div class="flex items-center gap-3 rounded-lg border border-slate-200 dark:border-slate-700 bg-gray-100 dark:bg-gray-900 shadow-sm px-3 py-1">
          <span
            class={`h-2 w-2 rounded-full flex-shrink-0 ${
              isOnline() ? 'bg-green-500' : 'bg-red-500'
            }`}
          ></span>
          <div class="flex w-full flex-wrap items-center gap-4 text-[11px] sm:text-xs text-slate-600 dark:text-slate-200">
            <div class="flex flex-wrap items-center gap-3">
              <a
                href={nodeUrl()}
                target="_blank"
                rel="noopener noreferrer"
                class="text-slate-800 dark:text-slate-50 hover:text-sky-600 dark:hover:text-sky-400 transition-colors duration-150 cursor-pointer font-semibold text-sm sm:text-base"
                title={`Open ${props.node.name} web interface`}
              >
                {props.node.name}
              </a>
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
          </div>
        </div>
      </td>
    </tr>
  );
};
