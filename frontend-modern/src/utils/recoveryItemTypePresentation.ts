import type { RecoveryExternalRef } from '@/types/recovery';
import { getResourceTypePresentation } from '@/utils/resourceTypePresentation';
import { getWorkloadTypePresentation } from '@/utils/workloadTypePresentation';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

export interface RecoveryItemTypePresentation {
  key: string;
  label: string;
  badgeClasses: string;
}

const BADGE_BASE_CLASSES =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';
const DEFAULT_BADGE_TONE_CLASSES = 'bg-surface-alt text-base-content';
const DEFAULT_BADGE_CLASSES = `${BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`;

interface RecoveryItemTypeLike {
  display?: {
    itemType?: string | null;
    subjectType?: string | null;
  } | null;
  itemRef?: RecoveryExternalRef | null;
  subjectRef?: RecoveryExternalRef | null;
}

const getRecoveryItemTypeValue = (
  value: RecoveryItemTypeLike | null | undefined,
): string =>
  String(
    value?.display?.itemType ||
      value?.display?.subjectType ||
      value?.itemRef?.type ||
      value?.subjectRef?.type ||
      '',
  );

export const normalizeRecoveryItemTypeQueryValue = (
  value: string | null | undefined,
): string => {
  const normalized = String(value || '').trim().toLowerCase();
  switch (normalized) {
    case '':
    case 'all':
      return '';
    case 'proxmox-vm':
    case 'proxmox-vm-backup':
    case 'vm':
    case 'vm-backup':
      return 'vm';
    case 'proxmox-lxc':
    case 'lxc':
    case 'ct':
    case 'container':
    case 'system-container':
      return 'system-container';
    case 'docker-container':
    case 'docker':
    case 'app-container':
      return 'app-container';
    case 'oci-container':
      return 'oci-container';
    case 'k8s-pod':
    case 'pod':
      return 'pod';
    case 'k8s-pvc':
    case 'pvc':
      return 'pvc';
    case 'truenas-dataset':
    case 'dataset':
      return 'dataset';
    case 'velero-backup':
      return 'velero-backup';
    case 'proxmox-guest':
    case 'guest':
      return 'guest';
    default:
      if (normalized.startsWith('proxmox-')) return normalized.slice('proxmox-'.length);
      if (normalized.startsWith('truenas-')) return normalized.slice('truenas-'.length);
      if (normalized.startsWith('k8s-')) return normalized.slice('k8s-'.length);
      return normalized;
  }
};

export const getRecoveryItemTypePresentation = (
  value: string | null | undefined,
): RecoveryItemTypePresentation | null => {
  const key = normalizeRecoveryItemTypeQueryValue(value);
  if (!key) return null;

  switch (key) {
    case 'vm': {
      const presentation = getWorkloadTypePresentation('vm');
      return {
        key,
        label: presentation.label,
        badgeClasses: `${BADGE_BASE_CLASSES} ${presentation.className}`,
      };
    }
    case 'system-container': {
      const presentation = getWorkloadTypePresentation('system-container');
      return {
        key,
        label: presentation.label,
        badgeClasses: `${BADGE_BASE_CLASSES} ${presentation.className}`,
      };
    }
    case 'app-container': {
      const presentation = getWorkloadTypePresentation('app-container', {
        label: 'App Container',
        title: 'Application Container',
      });
      return {
        key,
        label: presentation.label,
        badgeClasses: `${BADGE_BASE_CLASSES} ${presentation.className}`,
      };
    }
    case 'oci-container': {
      const presentation = getWorkloadTypePresentation('oci-container');
      return {
        key,
        label: presentation.label,
        badgeClasses: `${BADGE_BASE_CLASSES} ${presentation.className}`,
      };
    }
    case 'pod': {
      const presentation = getWorkloadTypePresentation('pod');
      return {
        key,
        label: presentation.label,
        badgeClasses: `${BADGE_BASE_CLASSES} ${presentation.className}`,
      };
    }
    case 'pvc': {
      const presentation = getResourceTypePresentation('k8s-pvc');
      return {
        key,
        label: presentation?.label || 'PVC',
        badgeClasses: `${BADGE_BASE_CLASSES} ${presentation?.badgeClasses || DEFAULT_BADGE_TONE_CLASSES}`,
      };
    }
    default: {
      const presentation = getResourceTypePresentation(key);
      if (presentation) {
        const label =
          presentation.label === key
            ? titleCaseDelimitedLabel(key, {
                fallback: 'Unknown',
                preserveShortAllCaps: true,
              })
            : presentation.label;
        return {
          key,
          label,
          badgeClasses: `${BADGE_BASE_CLASSES} ${presentation.badgeClasses}`,
        };
      }
      return {
        key,
        label: titleCaseDelimitedLabel(key, {
          fallback: 'Unknown',
          preserveShortAllCaps: true,
        }),
        badgeClasses: DEFAULT_BADGE_CLASSES,
      };
    }
  }
};

export const getRecoveryItemTypeLabel = (value: string | null | undefined): string =>
  getRecoveryItemTypePresentation(value)?.label || '';

export const getRecoveryItemTypeBadgeClass = (value: string | null | undefined): string =>
  getRecoveryItemTypePresentation(value)?.badgeClasses || DEFAULT_BADGE_CLASSES;

export const getRecoveryRollupItemTypeKey = (
  rollup: RecoveryItemTypeLike | null | undefined,
): string => normalizeRecoveryItemTypeQueryValue(getRecoveryItemTypeValue(rollup));

export const getRecoveryPointItemTypeKey = (
  point: RecoveryItemTypeLike | null | undefined,
): string => normalizeRecoveryItemTypeQueryValue(getRecoveryItemTypeValue(point));
