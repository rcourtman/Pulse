import { ThresholdsTableAgentDisksSection } from './ThresholdsTableAgentDisksSection';
import { ThresholdsTableAgentsResourcesSection } from './ThresholdsTableAgentsResourcesSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableAgentsTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      <ThresholdsTableAgentsResourcesSection {...props} />
      <ThresholdsTableAgentDisksSection {...props} />
    </>
  );
}
