import { Show, createSignal, createEffect, onCleanup } from 'solid-js';
import {
  createLocalStorageBooleanSignal,
  createLocalStorageStringSignal,
  STORAGE_KEYS,
} from '@/utils/localStorage';
import { useWebSocket } from '@/contexts/appRuntime';
import { ActionIconButton, Button } from '@/components/shared/Button';
import GithubIcon from 'lucide-solid/icons/github';
import StarIcon from 'lucide-solid/icons/star';
import XIcon from 'lucide-solid/icons/x';

const GITHUB_REPO_URL = 'https://github.com/rcourtman/Pulse';

function getTodayDateString(): string {
  return new Date().toISOString().split('T')[0]; // YYYY-MM-DD
}

export function GitHubStarBanner() {
  const { initialDataReceived, state } = useWebSocket();

  // Track if user has dismissed the modal (permanent)
  const [dismissed, setDismissed] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.GITHUB_STAR_DISMISSED,
    false,
  );

  // Track the first date user had infrastructure connected
  const [firstSeenDate, setFirstSeenDate] = createLocalStorageStringSignal(
    STORAGE_KEYS.GITHUB_STAR_FIRST_SEEN,
    '',
  );

  // Track snooze date (when "Maybe later" was clicked, don't show again until this date)
  const [snoozedUntil, setSnoozedUntil] = createLocalStorageStringSignal(
    STORAGE_KEYS.GITHUB_STAR_SNOOZED_UNTIL,
    '',
  );

  const [showModal, setShowModal] = createSignal(false);

  // Check if user qualifies to see the modal
  createEffect(() => {
    // Already dismissed? Don't show
    if (dismissed()) {
      setShowModal(false);
      return;
    }

    if (!initialDataReceived()) {
      setShowModal(false);
      return;
    }

    // Check if user has connected infrastructure
    const hasInfrastructure = (state.resources || []).length > 0;

    if (!hasInfrastructure) {
      setShowModal(false);
      return;
    }

    const today = getTodayDateString();
    const firstSeen = firstSeenDate();

    // First time seeing infrastructure? Record the date, don't show yet
    if (!firstSeen) {
      setFirstSeenDate(today);
      setShowModal(false);
      return;
    }

    // Still within snooze period? Don't show
    const snoozeDate = snoozedUntil();
    if (snoozeDate && today < snoozeDate) {
      setShowModal(false);
      return;
    }

    // Returning user (different day than first seen)? Show the modal
    if (firstSeen !== today) {
      setShowModal(true);
    }
  });

  const handleDismiss = () => {
    setDismissed(true);
    setShowModal(false);
  };

  const handleStarClick = () => {
    window.open(GITHUB_REPO_URL, '_blank', 'noopener,noreferrer');
    // Auto-dismiss - trust that they starred
    setDismissed(true);
    setShowModal(false);
  };

  const handleMaybeLater = () => {
    // Snooze for 7 days before showing again
    const snoozeDate = new Date();
    snoozeDate.setDate(snoozeDate.getDate() + 7);
    setSnoozedUntil(snoozeDate.toISOString().split('T')[0]);
    setShowModal(false);
  };

  createEffect(() => {
    if (!showModal()) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      handleMaybeLater();
    };
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });

  return (
    <Show when={showModal()}>
      <section
        class="fixed left-4 right-20 bottom-[calc(5rem+env(safe-area-inset-bottom,0px))] z-30 max-w-md overflow-hidden rounded-lg border border-border bg-surface text-base-content shadow-xl md:right-auto md:bottom-4"
        aria-labelledby="github-star-title"
        aria-live="polite"
      >
        <div class="flex items-start gap-3 p-4">
          <div
            class="relative mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-surface-hover"
            aria-hidden="true"
          >
            <GithubIcon class="h-5 w-5 text-base-content" />
            <div class="absolute -right-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full bg-yellow-400 shadow-sm">
              <StarIcon class="h-3 w-3 text-yellow-800" />
            </div>
          </div>

          <div class="min-w-0 flex-1">
            <div class="flex items-start gap-2">
              <div class="min-w-0 flex-1">
                <h2 id="github-star-title" class="text-sm font-semibold text-base-content">
                  Enjoying Pulse?
                </h2>
                <p class="mt-1 text-xs leading-5 text-muted">
                  Pulse is built and maintained by an independent developer. If it's been useful for
                  monitoring your infrastructure, a GitHub star helps more than you'd think.
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
              <Button
                onClick={handleStarClick}
                variant="primary"
                size="mdCompact"
                class="gap-2"
                type="button"
              >
                <StarIcon class="h-4 w-4" aria-hidden="true" />
                Star on GitHub
              </Button>
              <Button onClick={handleMaybeLater} variant="ghost" size="mdCompact" type="button">
                Maybe later
              </Button>
            </div>
          </div>
        </div>
      </section>
    </Show>
  );
}
