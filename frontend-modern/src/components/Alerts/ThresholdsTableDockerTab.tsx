import { Show } from 'solid-js';

import { ThresholdsTableDockerContainersSection } from './ThresholdsTableDockerContainersSection';
import { ThresholdsTableDockerHostsSection } from './ThresholdsTableDockerHostsSection';
import { ThresholdsTableDockerIgnoredPrefixesSection } from './ThresholdsTableDockerIgnoredPrefixesSection';
import { ThresholdsTableDockerServiceGapSection } from './ThresholdsTableDockerServiceGapSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      <Show when={props.state.hasDockerSpecificControls()}>
        <ThresholdsTableDockerIgnoredPrefixesSection {...props} />
        <ThresholdsTableDockerServiceGapSection {...props} />
      </Show>
      <ThresholdsTableDockerHostsSection {...props} />
      <ThresholdsTableDockerContainersSection {...props} />
    </>
  );
}
