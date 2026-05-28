import { ThresholdsTableProxmoxPBSSection } from './ThresholdsTableProxmoxPBSSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTablePBSTab(props: ThresholdsTableSectionProps) {
  return <ThresholdsTableProxmoxPBSSection {...props} />;
}
