import type { WorkloadType } from '@/types/workloads';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

export interface WorkloadTypePresentation {
  label: string;
  pluralLabel: string;
  title: string;
  className: string;
}

type WorkloadTypePresentationKey =
  WorkloadType | 'system-container' | 'app-container' | 'agent' | 'pod';

const PRESENTATION_MAP: Record<WorkloadTypePresentationKey, WorkloadTypePresentation> = {
  vm: {
    label: 'VM',
    pluralLabel: 'VMs',
    title: 'Virtual Machine',
    className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  },
  'system-container': {
    label: 'LXC',
    pluralLabel: 'LXC',
    title: 'System Container',
    className: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  },
  'app-container': {
    label: 'Container',
    pluralLabel: 'Containers',
    title: 'Application Container',
    className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
  },
  pod: {
    label: 'Pod',
    pluralLabel: 'Pods',
    title: 'Kubernetes Pod',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
  agent: {
    label: 'Agent',
    pluralLabel: 'Agents',
    title: 'Agent',
    className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  },
};

const DEFAULT_PRESENTATION: WorkloadTypePresentation = {
  label: 'Unknown',
  pluralLabel: 'Unknown',
  title: 'Unknown workload type',
  className: 'bg-surface-alt text-base-content',
};

export const normalizeWorkloadTypePresentationKey = (
  value: string | null | undefined,
): WorkloadTypePresentationKey | null => {
  const canonical = canonicalizeFrontendResourceType(value);
  if (!canonical) return null;
  // OCI containers are a flavor of system-container, not a distinct workload kind.
  // Callers differentiate via IsOCI / osTemplate and override the title at the call site.
  if (canonical === 'oci-container') return 'system-container';
  if (
    canonical === 'vm' ||
    canonical === 'system-container' ||
    canonical === 'app-container' ||
    canonical === 'pod' ||
    canonical === 'agent'
  ) {
    return canonical;
  }
  return null;
};

export const getWorkloadTypePresentation = (
  rawType: string | WorkloadType | null | undefined,
  overrides?: Partial<Pick<WorkloadTypePresentation, 'label' | 'pluralLabel' | 'title'>>,
): WorkloadTypePresentation => {
  const normalized = normalizeWorkloadTypePresentationKey(rawType);
  const fallbackLabel =
    overrides?.label ||
    titleCaseDelimitedLabel((rawType || '').toString()) ||
    DEFAULT_PRESENTATION.label;

  if (!normalized) {
    return {
      ...DEFAULT_PRESENTATION,
      label: fallbackLabel,
      pluralLabel: overrides?.pluralLabel || fallbackLabel,
      title: overrides?.title || fallbackLabel,
    };
  }

  const base = PRESENTATION_MAP[normalized];
  return {
    ...base,
    label: overrides?.label || base.label,
    pluralLabel: overrides?.pluralLabel || base.pluralLabel,
    title: overrides?.title || base.title,
  };
};

export const getWorkloadTypeLabel = (rawType: string | WorkloadType | null | undefined): string =>
  getWorkloadTypePresentation(rawType).label;
