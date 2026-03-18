import type { ColumnDef } from '@/hooks/useColumnVisibility';
import {
  TYPE_COLUMN_ID,
  TYPE_COLUMN_LABEL,
  TYPE_COLUMN_SORT_KEY,
  TYPE_COLUMN_WIDTH,
} from '@/utils/typeColumnContract';

const createCanonicalTypeColumn = (
  options: Pick<ColumnDef, 'defaultHidden' | 'width' | 'sortKey'> = {},
): ColumnDef => {
  return {
    id: TYPE_COLUMN_ID,
    label: TYPE_COLUMN_LABEL,
    toggleable: true,
    ...options,
  };
};

export const createVisibleCanonicalTypeColumn = (): ColumnDef =>
  createCanonicalTypeColumn({
    width: TYPE_COLUMN_WIDTH,
    sortKey: TYPE_COLUMN_SORT_KEY,
  });

export const createHiddenCanonicalTypeColumn = (): ColumnDef =>
  createCanonicalTypeColumn({
    defaultHidden: true,
    width: TYPE_COLUMN_WIDTH,
    sortKey: TYPE_COLUMN_SORT_KEY,
  });
