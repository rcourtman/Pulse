import { ThresholdsTableDockerContainersSection } from './ThresholdsTableDockerContainersSection';
import { ThresholdsTableDockerHostsSection } from './ThresholdsTableDockerHostsSection';
import { ThresholdsTableDockerIgnoredPrefixesSection } from './ThresholdsTableDockerIgnoredPrefixesSection';
import { ThresholdsTableDockerServiceGapSection } from './ThresholdsTableDockerServiceGapSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      <ThresholdsTableDockerIgnoredPrefixesSection {...props} />
      <ThresholdsTableDockerServiceGapSection {...props} />
      <ThresholdsTableDockerHostsSection {...props} />
      <ThresholdsTableDockerContainersSection {...props} />
    </>
  );
}
