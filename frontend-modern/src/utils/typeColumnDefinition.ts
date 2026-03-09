import type { ColumnDef } from '@/hooks/useColumnVisibility';

type CanonicalTypeColumnOptions = Pick<
  ColumnDef,
  'icon' | 'width' | 'minWidth' | 'maxWidth' | 'flex' | 'sortKey'
>;

type CanonicalTypeColumnVisibility = 'visible' | 'hidden';

export const createCanonicalTypeColumn = (
  options: CanonicalTypeColumnOptions & {
    defaultVisibility?: CanonicalTypeColumnVisibility;
  } = {},
): ColumnDef => {
  const { defaultVisibility = 'visible', ...columnOptions } = options;

  return {
    id: 'type',
    label: 'Type',
    toggleable: true,
    defaultHidden: defaultVisibility === 'hidden',
    ...columnOptions,
  };
};

export const createVisibleCanonicalTypeColumn = (
  options: CanonicalTypeColumnOptions = {},
): ColumnDef =>
  createCanonicalTypeColumn({
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
