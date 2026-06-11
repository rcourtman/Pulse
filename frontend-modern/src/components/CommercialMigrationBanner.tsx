import { createEffect, createMemo, createSignal, onCleanup, onMount, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import {
  commercialPosture,
  commercialPostureLoaded,
  loadCommercialPosture,
} from '@/stores/licenseCommercial';
import { sessionPresentationPolicyResolved } from '@/stores/sessionPresentationPolicy';
import { getCommercialMigrationNotice } from '@/utils/licensePresentation';
import { SELF_HOSTED_PRO_BILLING_PLAN_ROUTE } from '@/utils/pricingHandoff';

const DISMISSED_KEY_PREFIX = 'commercialMigrationBannerDismissed:';
const PENDING_RECHECK_INTERVAL_MS = 60_000;

/**
 * Global banner for an unresolved paid v5 license migration. The Settings
 * licence panel already explains the state in detail, but a paying customer
 * who upgraded should not have to discover missing Pro features before
 * learning their migration needs attention.
 */
export function CommercialMigrationBanner() {
  const navigate = useNavigate();
  const [dismissedState, setDismissedState] = createSignal<string | null>(null);

  const migration = createMemo(() => commercialPosture()?.commercial_migration);
  const notice = createMemo(() => getCommercialMigrationNotice(migration()));

  // Dismissal is per session and per migration state, so a pending → failed
  // transition resurfaces the banner. The sessionStorage check lives in the
  // memo because posture arrives asynchronously after mount.
  const dismissed = createMemo(() => {
    const state = migration()?.state;
    if (!state) return false;
    return (
      dismissedState() === state || sessionStorage.getItem(DISMISSED_KEY_PREFIX + state) === 'true'
    );
  });

  // The app shell only loads commercial posture when upgrade prompts are
  // enabled, but a migration warning is service degradation, not an upgrade
  // nag — load the banner's own data dependency regardless.
  createEffect(() => {
    if (sessionPresentationPolicyResolved() && !commercialPostureLoaded()) {
      void loadCommercialPosture();
    }
  });

  onMount(() => {
    // A pending migration is retried by the backend in the background, so
    // re-check periodically and let the banner clear itself on success.
    const timer = setInterval(() => {
      if (commercialPosture()?.commercial_migration?.state === 'pending') {
        void loadCommercialPosture(true);
      }
    }, PENDING_RECHECK_INTERVAL_MS);
    onCleanup(() => clearInterval(timer));
  });

  const handleDismiss = () => {
    const state = migration()?.state;
    if (!state) return;
    setDismissedState(state);
    sessionStorage.setItem(DISMISSED_KEY_PREFIX + state, 'true');
  };

  const toneClasses = createMemo(() =>
    migration()?.state === 'failed'
      ? 'bg-red-50 dark:bg-red-900 border-b border-red-200 dark:border-red-800 text-red-800 dark:text-red-100'
      : 'bg-amber-50 dark:bg-amber-900 border-b border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-100',
  );
  const buttonClasses = createMemo(() =>
    migration()?.state === 'failed'
      ? 'border-red-300 dark:border-red-700 hover:bg-red-100 dark:hover:bg-red-800'
      : 'border-amber-300 dark:border-amber-700 hover:bg-amber-100 dark:hover:bg-amber-800',
  );

  return (
    <Show when={!dismissed() && notice()}>
      {(activeNotice) => (
        <div class={`px-3 py-2 ${toneClasses()}`}>
          <div class="container mx-auto flex items-center justify-between gap-3 text-sm">
            <div class="flex items-center gap-2 min-w-0">
              <svg class="w-4 h-4 shrink-0" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fill-rule="evenodd"
                  d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                  clip-rule="evenodd"
                />
              </svg>
              <span class="truncate">
                <span class="font-medium">{activeNotice().title}.</span> {activeNotice().body}
              </span>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <button
                onClick={() => navigate(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE)}
                class={`px-2 py-1 rounded border transition-colors whitespace-nowrap ${buttonClasses()}`}
              >
                Open license settings
              </button>
              <button
                onClick={handleDismiss}
                class="p-1 rounded transition-colors opacity-70 hover:opacity-100"
                title="Dismiss for this session"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </div>
        </div>
      )}
    </Show>
  );
}
