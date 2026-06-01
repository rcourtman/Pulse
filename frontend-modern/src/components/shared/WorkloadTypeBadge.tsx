import { createMemo, type Component } from 'solid-js';
import type { WorkloadType } from '@/types/workloads';
import { getWorkloadTypeBadge } from '@/components/shared/workloadTypeBadges';

export const WORKLOAD_TYPE_BADGE_CLASS =
  'inline-flex items-center rounded px-1 py-0.5 text-[10px] font-medium whitespace-nowrap';

export interface WorkloadTypeBadgeProps {
  type: string | WorkloadType | null | undefined;
  label?: string;
  title?: string;
  class?: string;
}

export const WorkloadTypeBadge: Component<WorkloadTypeBadgeProps> = (props) => {
  const badge = createMemo(() =>
    getWorkloadTypeBadge(props.type, {
      label: props.label,
      title: props.title,
    }),
  );

  return (
    <span
      class={`${WORKLOAD_TYPE_BADGE_CLASS} ${badge().className} ${props.class ?? ''}`.trim()}
      title={badge().title}
    >
      {badge().label}
    </span>
  );
};

export default WorkloadTypeBadge;
