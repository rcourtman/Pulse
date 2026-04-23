import { getRecoveryLocationFacetLabel } from '@/utils/recoveryLocationPresentation';

export type RecoveryFilterChipKind =
  | 'day'
  | 'cluster'
  | 'item-type'
  | 'node'
  | 'namespace'
  | 'focused-item';

type RecoveryFilterChipPresentation = {
  clearButtonClass: string;
  className: string;
  label: string;
};

const CHIP_BASE_CLASS = 'inline-flex max-w-full items-center gap-1 rounded border px-2 py-0.5 text-[10px]';
const CLEAR_BUTTON_BASE_CLASS = 'rounded px-1 py-0.5 text-[10px]';

const CHIP_PRESENTATION: Record<RecoveryFilterChipKind, RecoveryFilterChipPresentation> = {
  cluster: {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-cyan-100 dark:hover:bg-cyan-900`,
    className: `${CHIP_BASE_CLASS} border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-700 dark:bg-cyan-900 dark:text-cyan-200`,
    label: getRecoveryLocationFacetLabel('cluster'),
  },
  day: {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-blue-100 dark:hover:bg-blue-900`,
    className: `${CHIP_BASE_CLASS} border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200`,
    label: 'Day',
  },
  'focused-item': {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-amber-100 dark:hover:bg-amber-900`,
    className: `${CHIP_BASE_CLASS} border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200`,
    label: 'Focused Item',
  },
  'item-type': {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-rose-100 dark:hover:bg-rose-900`,
    className: `${CHIP_BASE_CLASS} border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-700 dark:bg-rose-900 dark:text-rose-200`,
    label: 'Item Type',
  },
  namespace: {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-indigo-100 dark:hover:bg-indigo-900`,
    className: `${CHIP_BASE_CLASS} border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-700 dark:bg-indigo-900 dark:text-indigo-200`,
    label: getRecoveryLocationFacetLabel('namespace'),
  },
  node: {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-emerald-100 dark:hover:bg-emerald-900`,
    className: `${CHIP_BASE_CLASS} border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-200`,
    label: getRecoveryLocationFacetLabel('node'),
  },
};

export function getRecoveryFilterChipPresentation(
  kind: RecoveryFilterChipKind,
): RecoveryFilterChipPresentation {
  return CHIP_PRESENTATION[kind];
}
