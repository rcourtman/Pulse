import type { Component } from 'solid-js';
import type { DeployTargetStatus } from '@/types/agentDeploy';
import { getDeployStatusPresentation } from '@/utils/deployStatusPresentation';

interface DeployStatusBadgeProps {
  status: DeployTargetStatus;
}

export const DeployStatusBadge: Component<DeployStatusBadgeProps> = (props) => {
  const config = () => getDeployStatusPresentation(props.status);

  return (
    <span
      class={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium whitespace-nowrap ${config().className}`}
    >
      {config().label}
    </span>
  );
};
