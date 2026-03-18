import type { NormalizedHealth } from './models';
import type { StatusIndicatorVariant } from '@/utils/status';

export interface StorageHealthPresentation {
  label: string;
  variant: StatusIndicatorVariant;
  dotClass: string;
  countClass: string;
}

const STORAGE_HEALTH_PRESENTATION: Record<NormalizedHealth, StorageHealthPresentation> = {
  healthy: {
    label: 'Healthy',
    variant: 'success',
    dotClass: 'bg-green-500',
    countClass: 'text-muted',
  },
  warning: {
    label: 'Warning',
    variant: 'warning',
    dotClass: 'bg-yellow-500',
    countClass: 'text-yellow-600 dark:text-yellow-400',
  },
  critical: {
    label: 'Critical',
    variant: 'danger',
    dotClass: 'bg-red-500',
    countClass: 'text-red-600 dark:text-red-400',
  },
  offline: {
    label: 'Offline',
    variant: 'muted',
    dotClass: 'bg-slate-400',
    countClass: 'text-muted',
  },
  unknown: {
    label: 'Unknown',
    variant: 'muted',
    dotClass: 'bg-slate-300',
    countClass: 'text-muted',
  },
};

export const getStorageHealthPresentation = (health: NormalizedHealth): StorageHealthPresentation =>
  STORAGE_HEALTH_PRESENTATION[health];
