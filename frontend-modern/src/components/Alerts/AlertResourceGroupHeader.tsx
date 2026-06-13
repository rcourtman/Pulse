import { Show } from 'solid-js';

import { GROUPED_TABLE_ROW_BADGE_CLASS } from '@/components/shared/groupedTableRowPresentation';
import { WebInterfaceNameLink } from '@/components/shared/WebInterfaceNameLink';
import type { GroupHeaderMeta } from '@/features/alerts/thresholds/tableTypes';

interface AlertResourceGroupHeaderProps {
  groupKey: string;
  meta?: GroupHeaderMeta;
}

export function AlertResourceGroupHeader(props: AlertResourceGroupHeaderProps) {
  const groupLabel = () => props.meta?.displayName || props.meta?.rawName || props.groupKey;

  return (
    <Show when={props.meta?.type === 'agent'} fallback={<span>{groupLabel()}</span>}>
      <div class="flex flex-wrap items-center gap-3">
        <WebInterfaceNameLink
          name={groupLabel()}
          url={props.meta?.host}
          class="text-base-content transition-colors duration-150 hover:text-sky-600 dark:hover:text-sky-400"
          fallbackClass=""
          title={`Open ${groupLabel()} web interface`}
        />
        <Show when={props.meta?.clusterName}>
          <span class={GROUPED_TABLE_ROW_BADGE_CLASS}>{props.meta?.clusterName}</span>
        </Show>
      </div>
    </Show>
  );
}
