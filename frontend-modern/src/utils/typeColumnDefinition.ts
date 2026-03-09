import type { ColumnDef } from '@/hooks/useColumnVisibility';
import {
  TYPE_COLUMN_ID,
  TYPE_COLUMN_LABEL,
  TYPE_COLUMN_SORT_KEY,
  TYPE_COLUMN_WIDTH,
} from '@/utils/typeColumnContract';

type CanonicalTypeColumnOptions = Pick<
  ColumnDef,
  'icon' | 'width' | 'minWidth' | 'maxWidth' | 'flex' | 'sortKey'
>;

type CanonicalTypeColumnVisibility = 'visible' | 'hidden';

const createCanonicalTypeColumn = (
  options: CanonicalTypeColumnOptions & {
    defaultVisibility?: CanonicalTypeColumnVisibility;
  } = {},
): ColumnDef => {
  const { defaultVisibility = 'visible', ...columnOptions } = options;

  return {
    id: TYPE_COLUMN_ID,
    label: TYPE_COLUMN_LABEL,
    toggleable: true,
    defaultHidden: defaultVisibility === 'hidden',
    ...columnOptions,
  };
};

export const createVisibleCanonicalTypeColumn = (
  options: CanonicalTypeColumnOptions = {},
): ColumnDef =>
  createCanonicalTypeColumn({
    width: TYPE_COLUMN_WIDTH,
    sortKey: TYPE_COLUMN_SORT_KEY,
    ...options,
    defaultVisibility: 'visible',
  });

export const createHiddenCanonicalTypeColumn = (
  options: CanonicalTypeColumnOptions = {},
): ColumnDef =>
  createCanonicalTypeColumn({
    ...options,
    defaultVisibility: 'hidden',
  });
