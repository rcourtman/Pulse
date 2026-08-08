import { Show, createSignal, createEffect, onCleanup } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import {
  createLocalStorageBooleanSignal,
  createLocalStorageStringSignal,
  STORAGE_KEYS,
} from '@/utils/localStorage';
import { useWebSocket } from '@/contexts/appRuntime';
import { sessionCapabilities } from '@/stores/sessionCapabilities';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import {
  getSelfHostedBillingHref,
  PURCHASE_HANDOFF_SOURCE_ESTATE_CARD,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
} from '@/utils/pricingHandoff';
import { ActionIconButton, Button } from '@/components/shared/Button';
import BriefcaseIcon from 'lucide-solid/icons/briefcase';
import XIcon from 'lucide-solid/icons/x';

function getTodayDateString(): string {
  return new Date().toISOString().split('T')[0];
}

// Deliberately local rather than in the STORAGE_KEYS registry: these keys are
// read nowhere else, and utils/localStorage.ts sits inside the
// deployment-installability verification blast radius.
const BUSINESS_ESTATE_DISMISSED_KEY = 'pulse-business-estate-dismissed';
const BUSINESS_ESTATE_FIRST_SEEN_KEY = 'pulse-business-estate-first-seen';

/**
 * One-shot commercial prompt for free installs whose monitored estate crosses
 * the backend's business-scale thresholds (sessionCapabilities.businessEstate).
 * Deliberately stricter than the GitHub star prompt: every action dismisses
 * permanently — there is no snooze loop, so an install sees this card once.
 * It also waits until the star prompt has been interacted with, so the two
 * cards never stack in the same corner.
 */
export function BusinessEstateCard() {
  const navigate = useNavigate();
  const { initialDataReceived } = useWebSocket();

  const [dismissed, setDismissed] = createLocalStorageBooleanSignal(
    BUSINESS_ESTATE_DISMISSED_KEY,
    false,
  );
  const [firstSeenDate, setFirstSeenDate] = createLocalStorageStringSignal(
    BUSINESS_ESTATE_FIRST_SEEN_KEY,
    '',
  );
  const [starDismissed] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.GITHUB_STAR_DISMISSED,
    false,
  );
  const [starSnoozedUntil] = createLocalStorageStringSignal(
    STORAGE_KEYS.GITHUB_STAR_SNOOZED_UNTIL,
    '',
  );

  const [showCard, setShowCard] = createSignal(false);

  createEffect(() => {
    if (dismissed()) {
      setShowCard(false);
      return;
    }
    if (!initialDataReceived()) {
      setShowCard(false);
      return;
    }
    if (presentationPolicyHidesUpgradePrompts()) {
      setShowCard(false);
      return;
    }
    if (sessionCapabilities().businessEstate !== true) {
      setShowCard(false);
      return;
    }

    // The star prompt owns this corner until the user has interacted with it
    // (dismissed or snoozed). Never show two asks at once.
    const starInteracted = starDismissed() || starSnoozedUntil() !== '';
    if (!starInteracted) {
      setShowCard(false);
      return;
    }

    const today = getTodayDateString();
    const firstSeen = firstSeenDate();

    // First qualifying day: record it, stay quiet. Never prompt on the same
    // day the estate first crossed the threshold (or during initial setup).
    if (!firstSeen) {
      setFirstSeenDate(today);
      setShowCard(false);
      return;
    }

    if (firstSeen !== today) {
      setShowCard(true);
    }
  });

  const handleDismiss = () => {
    setDismissed(true);
    setShowCard(false);
  };

  const handleSeePlans = () => {
    setDismissed(true);
    setShowCard(false);
    navigate(
      getSelfHostedBillingHref('plan', {
        intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
        source: PURCHASE_HANDOFF_SOURCE_ESTATE_CARD,
      }),
    );
  };

  createEffect(() => {
    if (!showCard()) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      handleDismiss();
    };
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });

  return (
    <Show when={showCard()}>
      <section
        class="fixed left-4 right-20 bottom-[calc(5rem+env(safe-area-inset-bottom,0px))] z-30 max-w-md overflow-hidden rounded-lg border border-border bg-surface text-base-content shadow-xl md:right-auto md:bottom-4"
        aria-labelledby="business-estate-title"
        aria-live="polite"
      >
        <div class="flex items-start gap-3 p-4">
          <div
            class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-surface-hover"
            aria-hidden="true"
          >
            <BriefcaseIcon class="h-5 w-5 text-base-content" />
          </div>

          <div class="min-w-0 flex-1">
            <div class="flex items-start gap-2">
              <div class="min-w-0 flex-1">
                <h2 id="business-estate-title" class="text-sm font-semibold text-base-content">
                  Monitoring a business environment?
                </h2>
                <p class="mt-1 text-xs leading-5 text-muted">
                  Pulse is free and stays free. If it's earning its keep at work, the business plans
                  fund its development, and MSPs get a free 60-day evaluation.
                </p>
              </div>
              <ActionIconButton
                onClick={handleDismiss}
                label="Close and don't show again"
                title="Don't show again"
                tone="muted"
                size="sm"
                type="button"
              >
                <XIcon class="h-4 w-4" aria-hidden="true" />
              </ActionIconButton>
            </div>
            <div class="mt-3 flex flex-wrap gap-2">
              <Button onClick={handleSeePlans} variant="primary" size="mdCompact" type="button">
                See business plans
              </Button>
              <Button onClick={handleDismiss} variant="ghost" size="mdCompact" type="button">
                This is a homelab
              </Button>
            </div>
          </div>
        </div>
      </section>
    </Show>
  );
}
