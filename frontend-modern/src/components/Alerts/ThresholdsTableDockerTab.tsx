import { Show } from 'solid-js';

import { ThresholdsTableDockerContainersSection } from './ThresholdsTableDockerContainersSection';
import { ThresholdsTableDockerHostsSection } from './ThresholdsTableDockerHostsSection';
import { ThresholdsTableDockerIgnoredPrefixesSection } from './ThresholdsTableDockerIgnoredPrefixesSection';
import { ThresholdsTableDockerServiceGapSection } from './ThresholdsTableDockerServiceGapSection';
import { ThresholdsTableDockerUpdateAlertsSection } from './ThresholdsTableDockerUpdateAlertsSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      {/* The groups and their default limits lead; Docker-only rules follow. */}
      <ThresholdsTableDockerHostsSection {...props} />
      <ThresholdsTableDockerContainersSection {...props} />
      <Show when={props.state.hasDockerSpecificControls()}>
        <ThresholdsTableDockerIgnoredPrefixesSection {...props} />
        <ThresholdsTableDockerUpdateAlertsSection {...props} />
        <ThresholdsTableDockerServiceGapSection {...props} />
      </Show>
    </>
  );
}
