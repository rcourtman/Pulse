import InfoIcon from 'lucide-solid/icons/info';
import { createSignal, onMount, Show } from 'solid-js';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { presentationPolicyIsDemoMode } from '@/stores/sessionPresentationPolicy';

export function DemoBanner() {
  const [dismissed, setDismissed] = createSignal(false);

  onMount(() => {
    if (sessionStorage.getItem('demoBannerDismissed') === 'true') {
      setDismissed(true);
    }
  });

  const handleDismiss = () => {
    setDismissed(true);
    // Remember dismissal for this session only
    sessionStorage.setItem('demoBannerDismissed', 'true');
  };

  return (
    <Show when={presentationPolicyIsDemoMode() && !dismissed()}>
      <InlineNotice
        role="status"
        tone="info"
        layout="banner"
        icon={<InfoIcon class="h-4 w-4" aria-hidden="true" />}
        onDismiss={handleDismiss}
        dismissLabel="Dismiss demo banner"
        dismissTitle="Dismiss"
      >
        Demo instance with mock data (read-only)
      </InlineNotice>
    </Show>
  );
}
