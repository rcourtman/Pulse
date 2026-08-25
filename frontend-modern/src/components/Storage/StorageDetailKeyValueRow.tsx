import type { Component } from 'solid-js';
import {
  STORAGE_DETAIL_KEY_CLASS,
  STORAGE_DETAIL_KEY_VALUE_ROW_CLASS,
  STORAGE_DETAIL_VALUE_CLASS,
} from '@/features/storageBackups/detailPresentation';

type StorageDetailKeyValueRowProps = {
  label: string;
  value: string;
};

export const StorageDetailKeyValueRow: Component<StorageDetailKeyValueRowProps> = (props) => (
  <div
    class={`${STORAGE_DETAIL_KEY_VALUE_ROW_CLASS} col-span-2 min-w-0 items-start gap-3 sm:col-span-1`}
  >
    <span class={`${STORAGE_DETAIL_KEY_CLASS} shrink-0`}>{props.label}</span>
    <span class={`${STORAGE_DETAIL_VALUE_CLASS} min-w-0 break-words text-right`}>
      {props.value}
    </span>
  </div>
);

export default StorageDetailKeyValueRow;
