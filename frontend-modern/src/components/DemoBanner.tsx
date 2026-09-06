import InfoIcon from 'lucide-solid/icons/info';
import { createSignal, onMount, Show } from 'solid-js';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { presentationPolicyIsDemoMode } from '@/stores/sessionPresentationPolicy';

const DEMO_INSTALL_URL = 'https://pulserelay.pro/#setup';

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
        <span>Demo instance with mock data (read-only)</span>
        <span aria-hidden="true"> · </span>
        {/* The public demo is where pulserelay.pro sends curious visitors, and
            without this it was a dead end: no way back except browser history.
            Install guidance, not an upsell, so it stays in demo mode. */}
        <a
          href={DEMO_INSTALL_URL}
          target="_blank"
          rel="noopener noreferrer"
          class="font-medium underline underline-offset-2 hover:no-underline"
        >
          Run Pulse on your own hardware
        </a>
      </InlineNotice>
    </Show>
  );
}
