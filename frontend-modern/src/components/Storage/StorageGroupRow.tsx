import { Component, Show } from 'solid-js';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import { formatPercent } from '@/utils/format';
import type { StorageGroupedRecords, StorageGroupKey } from './useStorageModel';

interface StorageGroupRowProps {
  group: StorageGroupedRecords;
  groupBy: StorageGroupKey;
  expanded: boolean;
  onToggle: () => void;
}

const HEALTH_DOT: Record<string, string> = {
  healthy: 'bg-green-500',
  warning: 'bg-yellow-500',
  critical: 'bg-red-500',
  offline: 'bg-gray-400',
  unknown: 'bg-gray-300',
};

export const StorageGroupRow: Component<StorageGroupRowProps> = (props) => {
  return (
    <tr
      class="cursor-pointer select-none bg-gray-50/80 dark:bg-gray-800/50 hover:bg-gray-100/80 dark:hover:bg-gray-700/40 transition-colors border-b border-gray-200 dark:border-gray-700"
      onClick={() => props.onToggle()}
    >
      <td colSpan={99} class="px-1.5 sm:px-2 py-1">
        <div class="flex items-center gap-3">
          {/* Expand chevron */}
          <svg
            class={`w-3.5 h-3.5 text-gray-500 dark:text-gray-400 transition-transform duration-150 flex-shrink-0 ${
              props.expanded ? 'rotate-90' : ''
            }`}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M9 18l6-6-6-6" />
          </svg>

          {/* Group label */}
          <span class="text-[11px] font-semibold text-gray-800 dark:text-gray-200 min-w-0 truncate max-w-[200px]">
            {props.group.key}
          </span>

          {/* Aggregate capacity bar */}
          <Show when={props.group.stats.totalBytes > 0}>
            <div class="w-48 flex-shrink-0 hidden sm:block">
              <EnhancedStorageBar
                used={props.group.stats.usedBytes}
                total={props.group.stats.totalBytes}
                free={Math.max(0, props.group.stats.totalBytes - props.group.stats.usedBytes)}
              />
            </div>
            <span class="text-xs font-medium text-gray-600 dark:text-gray-400 hidden sm:inline">
              {formatPercent(props.group.stats.usagePercent)}
            </span>
          </Show>

          {/* Pool count */}
          <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
            {props.group.items.length} {props.group.items.length === 1 ? 'pool' : 'pools'}
          </span>

          {/* Health dots */}
          <div class="flex items-center gap-1.5 ml-auto">
            <Show when={props.group.stats.byHealth.healthy > 0}>
              <span class="flex items-center gap-0.5">
                <span class={`w-2 h-2 rounded-full ${HEALTH_DOT.healthy}`} />
                <span class="text-[10px] text-gray-500 dark:text-gray-400">{props.group.stats.byHealth.healthy}</span>
              </span>
            </Show>
            <Show when={props.group.stats.byHealth.warning > 0}>
              <span class="flex items-center gap-0.5">
                <span class={`w-2 h-2 rounded-full ${HEALTH_DOT.warning}`} />
                <span class="text-[10px] text-yellow-600 dark:text-yellow-400">{props.group.stats.byHealth.warning}</span>
              </span>
            </Show>
            <Show when={props.group.stats.byHealth.critical > 0}>
              <span class="flex items-center gap-0.5">
                <span class={`w-2 h-2 rounded-full ${HEALTH_DOT.critical}`} />
                <span class="text-[10px] text-red-600 dark:text-red-400">{props.group.stats.byHealth.critical}</span>
              </span>
            </Show>
            <Show when={props.group.stats.byHealth.offline > 0}>
              <span class="flex items-center gap-0.5">
                <span class={`w-2 h-2 rounded-full ${HEALTH_DOT.offline}`} />
                <span class="text-[10px] text-gray-500 dark:text-gray-400">{props.group.stats.byHealth.offline}</span>
              </span>
            </Show>
            <Show when={props.group.stats.byHealth.unknown > 0}>
              <span class="flex items-center gap-0.5">
                <span class={`w-2 h-2 rounded-full ${HEALTH_DOT.unknown}`} />
                <span class="text-[10px] text-gray-500 dark:text-gray-400">{props.group.stats.byHealth.unknown}</span>
              </span>
            </Show>
          </div>
        </div>
      </td>
    </tr>
  );
};
