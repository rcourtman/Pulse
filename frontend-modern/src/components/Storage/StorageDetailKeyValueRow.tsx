import type { Component } from 'solid-js';
import { InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
import {
  STORAGE_DETAIL_KEY_CLASS,
  STORAGE_DETAIL_VALUE_CLASS,
} from '@/features/storageBackups/detailPresentation';

type StorageDetailKeyValueRowProps = {
  label: string;
  value: string;
};

export const StorageDetailKeyValueRow: Component<StorageDetailKeyValueRowProps> = (props) => (
  <InfoCardKeyValueRow
    class="col-span-2 sm:col-span-1"
    desktopAt="sm"
    label={props.label}
    labelClass={STORAGE_DETAIL_KEY_CLASS}
    value={props.value}
    valueClass={`${STORAGE_DETAIL_VALUE_CLASS} break-words`}
    valueTitle={props.value}
  />
);

export default StorageDetailKeyValueRow;
