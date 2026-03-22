import type { ThresholdsTableProps } from './types';
import type { ThresholdsTableState } from './hooks/useThresholdsTableState';

export interface ThresholdsTableSectionProps {
  state: ThresholdsTableState;
  tableProps: ThresholdsTableProps;
}
