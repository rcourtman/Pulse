import type { Component } from 'solid-js';
import {
  STORAGE_DETAIL_CARD_CLASS,
  STORAGE_DETAIL_INLINE_LABEL_CLASS,
} from '@/features/storageBackups/detailPresentation';

type StorageDetailMetricCardProps = {
  label: string;
  value: string;
  valueClass: string;
};

export const StorageDetailMetricCard: Component<StorageDetailMetricCardProps> = (props) => (
  <div class={STORAGE_DETAIL_CARD_CLASS}>
    <div class={`${STORAGE_DETAIL_INLINE_LABEL_CLASS} mb-0.5`}>{props.label}</div>
    <div class={`text-sm font-semibold ${props.valueClass}`}>{props.value}</div>
  </div>
);

export default StorageDetailMetricCard;
