import { ThresholdsTableAgentDisksSection } from './ThresholdsTableAgentDisksSection';
import { ThresholdsTableAgentsResourcesSection } from './ThresholdsTableAgentsResourcesSection';
import { ThresholdsTableSMARTDefaultsCard } from './ThresholdsTableSMARTDefaultsCard';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableAgentsTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      <ThresholdsTableSMARTDefaultsCard {...props} />
      <ThresholdsTableAgentsResourcesSection {...props} />
      <ThresholdsTableAgentDisksSection {...props} />
    </>
  );
}
