import { Show, createSignal, createEffect, onMount } from 'solid-js';
import { useNavigate } from '@solidjs/router';

/**
 * ⚠️ MIGRATION SCAFFOLDING - TEMPORARY COMPONENT
 *
 * This banner exists only to handle migration from legacy SSH-in-container
 * to the secure pulse-sensor-proxy architecture introduced in v4.23.0.
 *
 * REMOVAL CRITERIA: Remove after v5.0 or when backend telemetry shows <1% detection rate
 * for 30+ days. This component serves no functional purpose beyond migration assistance.
 *
 * Can be disabled by setting PULSE_LEGACY_DETECTION=false on backend.
 */
export function LegacySSHBanner() {
  const navigate = useNavigate();
  const [isVisible, setIsVisible] = createSignal(false);
  const [isDismissed, setIsDismissed] = createSignal(false);

  // Check if previously dismissed
  onMount(() => {
    const dismissed = localStorage.getItem('legacySSHBannerDismissed');
    if (dismissed === 'true') {
      setIsDismissed(true);
      setIsVisible(false);
    }
  });

  // Check health endpoint for legacy SSH detection
  createEffect(async () => {
    if (isDismissed()) return;

    try {
      const response = await fetch('/api/health');
      const health = await response.json();

      if (health.legacySSHDetected && health.recommendProxyUpgrade) {
        setIsVisible(true);
        // Log banner impression for telemetry (removal criteria tracking)
        console.info('[Migration] Legacy SSH banner shown to user');
      }
    } catch (_error) {
      // Silently fail - health check failures shouldn't break the UI
    }
  });

  const handleDismiss = () => {
    setIsDismissed(true);
    setIsVisible(false);
    // Store dismissal in localStorage so it persists
    localStorage.setItem('legacySSHBannerDismissed', 'true');
    // Log dismissal for telemetry
    console.info('[Migration] Legacy SSH banner dismissed by user');
  };

  return (
    <Show when={isVisible()}>
      <div class="bg-orange-50 dark:bg-orange-900/20 border-b border-orange-200 dark:border-orange-800 text-orange-800 dark:text-orange-200 relative animate-slideDown">
        <div class="px-4 py-2">
          <div class="flex items-center justify-between gap-4">
            <div class="flex items-center gap-3">
              {/* Warning icon */}
              <svg
                class="w-5 h-5 flex-shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <line x1="12" y1="9" x2="12" y2="13"></line>
                <line x1="12" y1="17" x2="12.01" y2="17"></line>
              </svg>

              <div class="flex items-center gap-3 flex-wrap">
                <div class="text-sm space-x-1">
                  <span class="font-medium">Legacy temperature monitoring detected.</span>
                  <span>Remove each node and re-add it using the installer script in Settings → Nodes (advanced: rerun the host installer script directly).</span>
                  <a
                    href="https://github.com/rcourtman/Pulse/blob/main/docs/PULSE_SENSOR_PROXY_HARDENING.md#upgrading-existing-installations"
                    target="_blank"
                    rel="noreferrer"
                    class="underline decoration-dashed underline-offset-2 hover:decoration-solid"
                  >
                    View upgrade guide ↗
                  </a>
                </div>
                <button
                  onClick={() => navigate('/settings/nodes')}
                  class="px-3 py-1 text-xs font-medium bg-orange-600 hover:bg-orange-700 dark:bg-orange-700 dark:hover:bg-orange-800 text-white rounded transition-colors"
                >
                  Go to Nodes →
                </button>
              </div>
            </div>

            {/* Dismiss button */}
            <button
              onClick={handleDismiss}
              class="p-1 hover:bg-orange-100 dark:hover:bg-orange-800/30 rounded transition-colors flex-shrink-0"
              title="Dismiss"
            >
              <svg
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
