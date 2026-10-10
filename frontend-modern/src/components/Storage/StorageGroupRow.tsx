import { Component, For, Show } from 'solid-js';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import type { StorageGroupedRecords } from './useStorageModel';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import {
  buildStorageGroupRowPresentation,
  STORAGE_GROUP_ROW_CELL_CLASS,
  STORAGE_GROUP_ROW_CLASS,
  STORAGE_GROUP_ROW_CONTENT_CLASS,
  STORAGE_GROUP_ROW_HEALTH_COUNT_CLASS,
  STORAGE_GROUP_ROW_HEALTH_DOT_CLASS,
  STORAGE_GROUP_ROW_HEALTH_ITEM_CLASS,
  STORAGE_GROUP_ROW_HEALTH_WRAP_CLASS,
  STORAGE_GROUP_ROW_LABEL_CLASS,
  STORAGE_GROUP_ROW_POOL_COUNT_CLASS,
  STORAGE_GROUP_ROW_USAGE_LABEL_CLASS,
  STORAGE_GROUP_ROW_USAGE_WRAP_CLASS,
} from '@/features/storageBackups/groupPresentation';

interface StorageGroupRowProps {
  group: StorageGroupedRecords;
  summaryGroupId: string | null;
  expanded: boolean;
  onToggle: () => void;
}

// A group header is a disclosure row: clicking anywhere on it opens or closes
// the group, exactly like its chevron. data-summary-group-id keeps that click
// from clearing an open pool elsewhere in the table.
export const StorageGroupRow: Component<StorageGroupRowProps> = (props) => {
  const row = () => buildStorageGroupRowPresentation(props.group);

  return (
    <tr
      class={STORAGE_GROUP_ROW_CLASS}
      data-summary-group-id={props.summaryGroupId ?? undefined}
      onClick={() => props.onToggle()}
    >
      <td colSpan={99} class={STORAGE_GROUP_ROW_CELL_CLASS}>
        <div class={STORAGE_GROUP_ROW_CONTENT_CLASS}>
          <SummaryRowActionButton
            kind="disclosure"
            subjectLabel={row().label}
            expanded={props.expanded}
            hideWhenRowTappableOnMobile
            onAction={props.onToggle}
            class="inline-flex items-center justify-center"
          />

          {/* Group label */}
          <span class={STORAGE_GROUP_ROW_LABEL_CLASS}>{row().label}</span>

          {/* Aggregate capacity bar */}
          <Show when={row().showUsage}>
            <div class={STORAGE_GROUP_ROW_USAGE_WRAP_CLASS}>
              <EnhancedStorageBar
                used={props.group.stats.usedBytes}
                total={props.group.stats.totalBytes}
                free={Math.max(0, props.group.stats.totalBytes - props.group.stats.usedBytes)}
              />
            </div>
            <span class={STORAGE_GROUP_ROW_USAGE_LABEL_CLASS}>{row().usagePercentLabel}</span>
          </Show>

          {/* Pool count */}
          <span class={STORAGE_GROUP_ROW_POOL_COUNT_CLASS}>{row().poolCountLabel}</span>

          {/* Health dots */}
          <div class={STORAGE_GROUP_ROW_HEALTH_WRAP_CLASS}>
            <For each={row().healthCounts}>
              {(item) => (
                <span class={STORAGE_GROUP_ROW_HEALTH_ITEM_CLASS}>
                  <span
                    class={`${STORAGE_GROUP_ROW_HEALTH_DOT_CLASS} ${item.dotClass}`}
                    title={item.label}
                  />
                  <span class={`${STORAGE_GROUP_ROW_HEALTH_COUNT_CLASS} ${item.countClass}`}>
                    {item.count}
                  </span>
                </span>
              )}
            </For>
          </div>
        </div>
      </td>
    </tr>
  );
};
