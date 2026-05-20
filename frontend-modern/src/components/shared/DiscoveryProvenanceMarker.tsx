import { Show, type Component } from 'solid-js';
import ScanSearchIcon from 'lucide-solid/icons/scan-search';

import {
  getDiscoveryProvenanceBadgeClass,
  getDiscoveryProvenanceIconClass,
  getDiscoveryProvenanceLabel,
  getDiscoveryProvenanceTitle,
} from '@/utils/discoveryPresentation';

interface DiscoveryProvenanceMarkerProps {
  showLabel?: boolean;
  label?: string;
  title?: string;
  class?: string;
  testId?: string;
}

export const DiscoveryProvenanceMarker: Component<DiscoveryProvenanceMarkerProps> = (props) => {
  const showLabel = () => props.showLabel !== false;
  const title = () => props.title || getDiscoveryProvenanceTitle();
  const label = () => props.label || getDiscoveryProvenanceLabel();
  const className = () =>
    props.class ||
    (showLabel() ? getDiscoveryProvenanceBadgeClass() : getDiscoveryProvenanceIconClass());

  return (
    <span class={className()} title={title()} aria-label={title()} data-testid={props.testId}>
      <ScanSearchIcon class="h-3 w-3" aria-hidden="true" />
      <Show when={showLabel()}>
        <span>{label()}</span>
      </Show>
    </span>
  );
};

export default DiscoveryProvenanceMarker;
