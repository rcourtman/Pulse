import { createSignal, onMount, Show } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

export function DemoBanner() {
  const [isDemoMode, setIsDemoMode] = createSignal(false);
  const [dismissed, setDismissed] = createSignal(false);

  onMount(async () => {
    // Check if we're in demo mode by trying a test request
    try {
      const response = await apiFetch('/api/health');
      const demoHeader = response.headers.get('X-Demo-Mode');
      if (demoHeader === 'true') {
        setIsDemoMode(true);
      }
    } catch (error) {
      // Non-fatal: banner remains hidden when demo detection cannot be verified.
      logger.debug('[DemoBanner] Failed to check demo mode', error);
    }
  });

  const handleDismiss = () => {
    setDismissed(true);
    // Remember dismissal for this session only
    sessionStorage.setItem('demoBannerDismissed', 'true');
  };

  // Check if already dismissed this session
  onMount(() => {
    if (sessionStorage.getItem('demoBannerDismissed') === 'true') {
      setDismissed(true);
    }
  });

  return (
    <Show when={isDemoMode() && !dismissed()}>
      <div class="bg-blue-50 dark:bg-blue-900/20 border-b border-blue-200 dark:border-blue-800 px-3 py-2">
        <div class="container mx-auto flex items-center justify-between text-sm">
          <div class="flex items-center gap-2 text-blue-700 dark:text-blue-300">
            <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
            </svg>
            <span>
              Demo instance with mock data (read-only)
            </span>
          </div>
          <button
            onClick={handleDismiss}
            class="p-1 hover:bg-blue-100 dark:hover:bg-blue-800/50 rounded text-blue-600 dark:text-blue-400 transition-colors"
            title="Dismiss"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
    </Show>
  );
}
