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
  <div class={STORAGE_DETAIL_KEY_VALUE_ROW_CLASS}>
    <span class={STORAGE_DETAIL_KEY_CLASS}>{props.label}</span>
    <span class={STORAGE_DETAIL_VALUE_CLASS}>{props.value}</span>
  </div>
);

export default StorageDetailKeyValueRow;
