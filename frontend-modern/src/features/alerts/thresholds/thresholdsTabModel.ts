import type { Accessor } from 'solid-js';

import type { ThresholdsTableProps } from './types';

export interface ThresholdsTabProps
  extends Omit<
    ThresholdsTableProps,
    'guestDefaults' | 'nodeDefaults' | 'pbsDefaults' | 'agentDefaults' | 'dockerDefaults'
  > {
  guestDefaults: Accessor<ThresholdsTableProps['guestDefaults']>;
  nodeDefaults: Accessor<ThresholdsTableProps['nodeDefaults']>;
  pbsDefaults: Accessor<NonNullable<ThresholdsTableProps['pbsDefaults']>>;
  agentDefaults: Accessor<ThresholdsTableProps['agentDefaults']>;
  dockerDefaults: Accessor<ThresholdsTableProps['dockerDefaults']>;
}

export function buildThresholdsTableProps(props: ThresholdsTabProps): ThresholdsTableProps {
  return {
    ...props,
    guestDefaults: props.guestDefaults(),
    nodeDefaults: props.nodeDefaults(),
    pbsDefaults: props.pbsDefaults(),
    agentDefaults: props.agentDefaults(),
    dockerDefaults: props.dockerDefaults(),
  };
}
