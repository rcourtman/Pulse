import { ThresholdsTableAgentDisksSection } from './ThresholdsTableAgentDisksSection';
import { ThresholdsTableAgentSensorsSection } from './ThresholdsTableAgentSensorsSection';
import { ThresholdsTableAgentsResourcesSection } from './ThresholdsTableAgentsResourcesSection';
import { ThresholdsTableSMARTDefaultsCard } from './ThresholdsTableSMARTDefaultsCard';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableAgentsTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      {/* The groups and their default limits lead; disk health rules follow. */}
      <ThresholdsTableAgentsResourcesSection {...props} />
      <ThresholdsTableAgentDisksSection {...props} />
      <ThresholdsTableAgentSensorsSection {...props} />
      <ThresholdsTableSMARTDefaultsCard {...props} />
    </>
  );
}
