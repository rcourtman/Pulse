import type { Component } from 'solid-js';
import { createMemo } from 'solid-js';
import { getServiceHealthPresentation } from '@/utils/serviceHealthPresentation';

export const ServiceHealthBadge: Component<{ status: string; health?: string }> = (props) => {
  const statusInfo = createMemo(() => getServiceHealthPresentation(props.status, props.health));

  return (
    <span
      class={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase ${statusInfo().bg} ${statusInfo().text}`}
    >
      <span class={`w-1.5 h-1.5 rounded-full ${statusInfo().dot}`} />
      {statusInfo().label}
    </span>
  );
};

export default ServiceHealthBadge;
