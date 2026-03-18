import type { WorkloadType } from '@/types/workloads';
import {
  type WorkloadTypePresentation as WorkloadTypeBadge,
  getWorkloadTypePresentation,
} from '@/utils/workloadTypePresentation';

export const getWorkloadTypeBadge = (
  rawType: string | WorkloadType | null | undefined,
  overrides?: Partial<Pick<WorkloadTypeBadge, 'label' | 'title'>>,
): WorkloadTypeBadge => {
  return getWorkloadTypePresentation(rawType, overrides);
};
