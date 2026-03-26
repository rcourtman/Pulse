import { getRecoveryLocationFacetLabel } from '@/utils/recoveryLocationPresentation';

export type RecoveryFilterChipKind = 'day' | 'cluster' | 'item-type' | 'node' | 'namespace';

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
  'item-type': {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-fuchsia-100 dark:hover:bg-fuchsia-900`,
    className: `${CHIP_BASE_CLASS} border-fuchsia-200 bg-fuchsia-50 text-fuchsia-700 dark:border-fuchsia-700 dark:bg-fuchsia-900 dark:text-fuchsia-200`,
    label: 'Item Type',
  },
  namespace: {
    clearButtonClass: `${CLEAR_BUTTON_BASE_CLASS} hover:bg-violet-100 dark:hover:bg-violet-900`,
    className: `${CHIP_BASE_CLASS} border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-700 dark:bg-violet-900 dark:text-violet-200`,
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
