import { Show } from 'solid-js';

import { GROUPED_TABLE_ROW_BADGE_CLASS } from '@/components/shared/groupedTableRowPresentation';
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
      fallback={<span>{groupLabel()}</span>}
    >
      <div class="flex flex-wrap items-center gap-3">
        <Show
          when={props.meta?.host}
          fallback={<span>{groupLabel()}</span>}
        >
          {(host) => (
            <a
              href={host()}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              class="text-base-content transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
              title={`Open ${groupLabel()} web interface`}
            >
              {groupLabel()}
            </a>
          )}
        </Show>
        <Show when={props.meta?.clusterName}>
          <span class={GROUPED_TABLE_ROW_BADGE_CLASS}>
            {props.meta?.clusterName}
          </span>
        </Show>
      </div>
    </Show>
  );
}
