import { Component, Show, createMemo } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getStoragePoolIssueTextClass,
  getStoragePoolProtectionTextClass,
} from '@/features/storageBackups/rowPresentation';
import {
  buildStoragePoolRowModel,
  getStoragePoolExpandIconClass,
  getStoragePoolImpactTextClass,
  STORAGE_POOL_ROW_CLASS,
  STORAGE_POOL_ROW_EXPANDED_CLASS,
  STORAGE_POOL_ROW_EXPAND_BUTTON_CLASS,
  STORAGE_POOL_ROW_EXPAND_CELL_CLASS,
  STORAGE_POOL_ROW_PLACEHOLDER_CLASS,
  STORAGE_POOL_ROW_HOST_CELL_CLASS,
  STORAGE_POOL_ROW_IMPACT_CELL_CLASS,
  STORAGE_POOL_ROW_ISSUE_CELL_CLASS,
  STORAGE_POOL_ROW_ISSUE_TEXT_CLASS,
  STORAGE_POOL_ROW_NAME_CELL_CLASS,
  STORAGE_POOL_ROW_NAME_TEXT_CLASS,
  STORAGE_POOL_ROW_PROTECTION_CELL_CLASS,
  STORAGE_POOL_ROW_PROTECTION_TEXT_CLASS,
  STORAGE_POOL_ROW_SOURCE_BADGE_CLASS,
  STORAGE_POOL_ROW_SOURCE_CELL_CLASS,
  STORAGE_POOL_ROW_STYLE,
  STORAGE_POOL_ROW_TEXT_TRUNCATE_CLASS,
  STORAGE_POOL_ROW_TYPE_CELL_CLASS,
  STORAGE_POOL_ROW_USAGE_FALLBACK_CLASS,
  STORAGE_POOL_ROW_USAGE_BAR_WRAP_CLASS,
  STORAGE_POOL_ROW_USAGE_CELL_CLASS,
  STORAGE_POOL_ROW_USAGE_WRAP_CLASS,
} from '@/features/storageBackups/storagePoolRowPresentation';
import type { Resource } from '@/types/resource';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import { StoragePoolDetail } from './StoragePoolDetail';

interface StoragePoolRowProps {
  record: StorageRecord;
  expanded: boolean;
  onToggleExpand: () => void;
  rowClass: string;
  rowStyle: Record<string, string>;
  physicalDisks: Resource[];
  alertDataAttrs: {
    'data-row-id': string;
    'data-alert-state': string;
    'data-alert-severity': string;
    'data-resource-highlighted': string;
  };
}

export const StoragePoolRow: Component<StoragePoolRowProps> = (props) => {
  const row = createMemo(() => buildStoragePoolRowModel(props.record));

  return (
    <>
      <tr
        class={`${STORAGE_POOL_ROW_CLASS} ${props.rowClass} ${props.expanded ? STORAGE_POOL_ROW_EXPANDED_CLASS : ''}`}
        style={{ ...props.rowStyle, ...STORAGE_POOL_ROW_STYLE }}
        onClick={props.onToggleExpand}
        {...props.alertDataAttrs}
      >
        <td class={STORAGE_POOL_ROW_NAME_CELL_CLASS}>
          <span class={STORAGE_POOL_ROW_NAME_TEXT_CLASS} title={props.record.name}>
            {props.record.name}
          </span>
        </td>

        <td class={STORAGE_POOL_ROW_SOURCE_CELL_CLASS}>
          <span
            class={`${row().platformToneClass} ${STORAGE_POOL_ROW_SOURCE_BADGE_CLASS}`}
          >
            {row().platformLabel}
          </span>
        </td>

        <td class={STORAGE_POOL_ROW_TYPE_CELL_CLASS}>
          <span class={STORAGE_POOL_ROW_TEXT_TRUNCATE_CLASS} title={row().topologyLabel}>
            {row().topologyLabel}
          </span>
        </td>

        <td class={STORAGE_POOL_ROW_HOST_CELL_CLASS}>
          <span class={STORAGE_POOL_ROW_TEXT_TRUNCATE_CLASS} title={row().hostLabel}>
            {row().hostLabel}
          </span>
        </td>

        <td class={STORAGE_POOL_ROW_PROTECTION_CELL_CLASS}>
          <Show when={row().compactProtection !== '—'} fallback={<span class={STORAGE_POOL_ROW_PLACEHOLDER_CLASS}>—</span>}>
            <span
              class={`${STORAGE_POOL_ROW_PROTECTION_TEXT_CLASS} ${getStoragePoolProtectionTextClass(props.record)}`}
              title={row().compactProtectionTitle || row().compactProtection}
            >
              {row().compactProtection}
            </span>
          </Show>
        </td>

        <td class={STORAGE_POOL_ROW_USAGE_CELL_CLASS}>
          <Show when={row().totalBytes > 0} fallback={<span class={STORAGE_POOL_ROW_USAGE_FALLBACK_CLASS}>n/a</span>}>
            <div class={STORAGE_POOL_ROW_USAGE_WRAP_CLASS}>
              <div class={STORAGE_POOL_ROW_USAGE_BAR_WRAP_CLASS}>
                <EnhancedStorageBar
                  used={row().usedBytes}
                  total={Math.max(row().totalBytes, 0)}
                  free={Math.max(row().freeBytes, 0)}
                  zfsPool={row().zfsPool || undefined}
                />
              </div>
            </div>
          </Show>
        </td>

        <td class={STORAGE_POOL_ROW_IMPACT_CELL_CLASS}>
          <span class={getStoragePoolImpactTextClass(row().compactImpact)} title={row().compactImpact}>
            {row().compactImpact}
          </span>
        </td>

        <td class={STORAGE_POOL_ROW_ISSUE_CELL_CLASS}>
          <Show when={row().compactIssue !== '—'} fallback={<span class={STORAGE_POOL_ROW_PLACEHOLDER_CLASS}>—</span>}>
            <span
              class={`${STORAGE_POOL_ROW_ISSUE_TEXT_CLASS} ${getStoragePoolIssueTextClass(props.record)}`}
              title={row().compactIssueSummary || row().compactIssue}
            >
              {row().compactIssue}
            </span>
          </Show>
        </td>

        <td class={STORAGE_POOL_ROW_EXPAND_CELL_CLASS}>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              props.onToggleExpand();
            }}
            class={STORAGE_POOL_ROW_EXPAND_BUTTON_CLASS}
            aria-label={`Toggle details for ${props.record.name}`}
          >
            <svg
              class={getStoragePoolExpandIconClass(props.expanded)}
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M9 18l6-6-6-6" />
            </svg>
          </button>
        </td>
      </tr>
      <Show when={props.expanded}>
        <StoragePoolDetail record={props.record} physicalDisks={props.physicalDisks} />
      </Show>
    </>
  );
};
