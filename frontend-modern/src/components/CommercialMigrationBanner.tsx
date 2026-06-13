import { createEffect, createMemo, createSignal, onCleanup, onMount, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import { InlineNotice } from '@/components/shared/InlineNotice';
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

  return (
    <Show when={!dismissed() && notice()}>
      {(activeNotice) => (
        <InlineNotice
          role={migration()?.state === 'failed' ? 'alert' : 'status'}
          tone={migration()?.state === 'failed' ? 'danger' : 'warning'}
          layout="banner"
          icon={<AlertTriangleIcon class="h-4 w-4" aria-hidden="true" />}
          actionLabel="Open license settings"
          actionOnClick={() => navigate(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE)}
          onDismiss={handleDismiss}
          dismissLabel="Dismiss commercial migration notice"
          dismissTitle="Dismiss for this session"
        >
          <span class="font-medium">{activeNotice().title}.</span> {activeNotice().body}
        </InlineNotice>
      )}
    </Show>
  );
}
