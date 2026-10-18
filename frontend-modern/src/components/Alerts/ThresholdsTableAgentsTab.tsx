import { ThresholdsTableAgentDisksSection } from './ThresholdsTableAgentDisksSection';
import { ThresholdsTableAgentsResourcesSection } from './ThresholdsTableAgentsResourcesSection';
import { ThresholdsTableSMARTDefaultsCard } from './ThresholdsTableSMARTDefaultsCard';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableAgentsTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      {/* The groups and their default limits lead; disk health rules follow. */}
      <ThresholdsTableAgentsResourcesSection {...props} />
      <ThresholdsTableAgentDisksSection {...props} />
      <ThresholdsTableSMARTDefaultsCard {...props} />
    </>
  );
}
