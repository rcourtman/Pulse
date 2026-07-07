import type { Accessor } from 'solid-js';

import type { ThresholdsTableProps } from './types';

export interface ThresholdsTabProps extends Omit<
  ThresholdsTableProps,
  | 'guestDefaults'
  | 'nodeDefaults'
  | 'pbsDefaults'
  | 'kubernetesDefaults'
  | 'trueNASDefaults'
  | 'trueNASDiskDefaults'
  | 'vmwareDefaults'
  | 'agentDefaults'
  | 'diskTempByType'
  | 'dockerDefaults'
> {
  guestDefaults: Accessor<ThresholdsTableProps['guestDefaults']>;
  nodeDefaults: Accessor<ThresholdsTableProps['nodeDefaults']>;
  pbsDefaults: Accessor<NonNullable<ThresholdsTableProps['pbsDefaults']>>;
  kubernetesDefaults: Accessor<NonNullable<ThresholdsTableProps['kubernetesDefaults']>>;
  trueNASDefaults: Accessor<NonNullable<ThresholdsTableProps['trueNASDefaults']>>;
  trueNASDiskDefaults: Accessor<NonNullable<ThresholdsTableProps['trueNASDiskDefaults']>>;
  vmwareDefaults: Accessor<NonNullable<ThresholdsTableProps['vmwareDefaults']>>;
  agentDefaults: Accessor<ThresholdsTableProps['agentDefaults']>;
  diskTempByType: Accessor<ThresholdsTableProps['diskTempByType']>;
  dockerDefaults: Accessor<ThresholdsTableProps['dockerDefaults']>;
}
