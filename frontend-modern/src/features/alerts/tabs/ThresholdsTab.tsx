import { createMemo } from 'solid-js';

import { ThresholdsTable } from '@/components/Alerts/ThresholdsTable';

import {
  buildThresholdsTableProps,
  type ThresholdsTabProps,
} from '../thresholds/thresholdsTabModel';

export function ThresholdsTab(props: ThresholdsTabProps) {
  const tableProps = createMemo(() => buildThresholdsTableProps(props));

  return <ThresholdsTable {...tableProps()} />;
}
