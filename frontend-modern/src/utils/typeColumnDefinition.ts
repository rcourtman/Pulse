import type { ColumnDef } from '@/hooks/useColumnVisibility';

type CanonicalTypeColumnOptions = Pick<
  ColumnDef,
  'icon' | 'width' | 'minWidth' | 'maxWidth' | 'flex' | 'sortKey' | 'defaultHidden'
>;

export const createCanonicalTypeColumn = (
  options: CanonicalTypeColumnOptions = {},
): ColumnDef => ({
  id: 'type',
  label: 'Type',
  toggleable: true,
  ...options,
});
