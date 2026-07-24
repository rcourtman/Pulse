import { Show } from 'solid-js';

import { GROUPED_TABLE_ROW_BADGE_CLASS } from '@/components/shared/groupedTableRowPresentation';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
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
        <ResourceNameWithWebInterfaceLink
          name={groupLabel()}
          url={props.meta?.host}
          title={`Open ${groupLabel()} web interface`}
        />
        <Show when={props.meta?.clusterName}>
          <span class={GROUPED_TABLE_ROW_BADGE_CLASS}>{props.meta?.clusterName}</span>
        </Show>
      </div>
    </Show>
  );
}
