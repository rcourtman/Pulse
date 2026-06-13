import type { JSX } from 'solid-js';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import { LoadingSpinner } from './LoadingSpinner';

interface DiscoveryLoadingFallbackProps {
  text?: string;
  class?: string;
}

export function DiscoveryLoadingFallback(props: DiscoveryLoadingFallbackProps): JSX.Element {
  const text = () => props.text ?? getDiscoveryLoadingState().text;

  return (
    <div
      role="status"
      aria-live="polite"
      class={['flex items-center justify-center py-8', props.class ?? '']
        .filter(Boolean)
        .join(' ')}
    >
      <LoadingSpinner size="xl" tone="info" />
      <span class="ml-2 text-sm text-muted">{text()}</span>
    </div>
  );
}

export default DiscoveryLoadingFallback;
