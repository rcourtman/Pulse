import { Show } from 'solid-js';

import type { GroupHeaderMeta } from '@/features/alerts/thresholds/tableTypes';

interface AlertResourceGroupHeaderProps {
  groupKey: string;
  meta?: GroupHeaderMeta;
}

export function AlertResourceGroupHeader(props: AlertResourceGroupHeaderProps) {
  const groupLabel = () => props.meta?.displayName || props.meta?.rawName || props.groupKey;

  return (
    <Show
      when={props.meta?.type === 'agent'}
      fallback={<span class="text-xs font-medium text-muted">{groupLabel()}</span>}
    >
      <div class="flex flex-wrap items-center gap-3">
        <Show
          when={props.meta?.host}
          fallback={<span class="text-sm font-medium text-base-content">{groupLabel()}</span>}
        >
          {(host) => (
            <a
              href={host()}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              class="text-sm font-medium text-base-content transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
              title={`Open ${groupLabel()} web interface`}
            >
              {groupLabel()}
            </a>
          )}
        </Show>
        <Show when={props.meta?.clusterName}>
          <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
            {props.meta?.clusterName}
          </span>
        </Show>
      </div>
    </Show>
  );
}
