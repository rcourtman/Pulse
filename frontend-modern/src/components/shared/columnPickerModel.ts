import type { ColumnDef } from '@/hooks/useColumnVisibility';

export const COLUMN_PICKER_BUTTON_LABEL = 'Columns';
export const COLUMN_PICKER_BUTTON_TITLE = 'Choose which columns to display';
export const COLUMN_PICKER_PANEL_TITLE = 'Show Columns';
export const COLUMN_PICKER_RESET_LABEL = 'Show all';
export const COLUMN_PICKER_EMPTY_LABEL = 'No columns available to toggle';

export function getHiddenColumnCount(
  columns: ColumnDef[],
  isHidden: (id: string) => boolean,
): number {
  return columns.filter((column) => isHidden(column.id)).length;
}

export function shouldShowColumnPickerReset(
  onReset: (() => void) | undefined,
  hiddenCount: number,
): boolean {
  return Boolean(onReset) && hiddenCount > 0;
}

export function getColumnPickerOptionTextClass(isChecked: boolean): string {
  return `text-sm ${isChecked ? 'text-base-content' : 'text-muted'}`;
}
