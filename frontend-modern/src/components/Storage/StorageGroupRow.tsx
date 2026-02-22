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
  offline: 'bg-slate-400',
  unknown: 'bg-slate-300',
};

export const StorageGroupRow: Component<StorageGroupRowProps> = (props) => {
  return (
    <tr
      class="cursor-pointer select-none bg-surface-alt hover:bg-surface-hover transition-colors border-b border-border"
      onClick={() => props.onToggle()}
    >
      <td colSpan={99} class="px-1.5 sm:px-2 py-0.5">
        <div class="flex items-center gap-3">
          {/* Expand chevron */}
          <svg
            class={`w-3.5 h-3.5 text-muted transition-transform duration-150 flex-shrink-0 ${props.expanded ? 'rotate-90' : ''
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
          <span class="text-[11px] font-semibold text-base-content w-[140px] flex-shrink-0 truncate">
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
            <span class="text-xs font-medium text-muted hidden sm:inline">
              {formatPercent(props.group.stats.usagePercent)}
            </span>
          </Show>

          {/* Pool count */}
          <span class="text-xs text-muted whitespace-nowrap">
            {props.group.items.length} {props.group.items.length === 1 ? 'pool' : 'pools'}
          </span>

          {/* Health dots */}
          <div class="flex items-center gap-1.5 ml-auto">
            <Show when={props.group.stats.byHealth.healthy > 0}>
              <span class="flex items-center gap-0.5">
                <span class={`w-2 h-2 rounded-full ${HEALTH_DOT.healthy}`} />
                <span class="text-[10px] text-muted">{props.group.stats.byHealth.healthy}</span>
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
                <span class="text-[10px] text-muted">{props.group.stats.byHealth.offline}</span>
              </span>
            </Show>
            <Show when={props.group.stats.byHealth.unknown > 0}>
              <span class="flex items-center gap-0.5">
                <span class={`w-2 h-2 rounded-full ${HEALTH_DOT.unknown}`} />
                <span class="text-[10px] text-muted">{props.group.stats.byHealth.unknown}</span>
              </span>
            </Show>
          </div>
        </div>
      </td>
    </tr>
  );
};
