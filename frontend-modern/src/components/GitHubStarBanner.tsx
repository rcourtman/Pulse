import { Show, createSignal, createEffect } from 'solid-js';
import {
  createLocalStorageBooleanSignal,
  createLocalStorageNumberSignal,
  createLocalStorageStringSignal,
  reserveLowPriorityNoticeSession,
  STORAGE_KEYS,
} from '@/utils/localStorage';
import { useWebSocket } from '@/contexts/appRuntime';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { dialogStackHasBlockingDialog } from '@/components/shared/useDialogState';
import GithubIcon from 'lucide-solid/icons/github';
import StarIcon from 'lucide-solid/icons/star';
import XIcon from 'lucide-solid/icons/x';

const GITHUB_REPO_URL = 'https://github.com/rcourtman/Pulse';
const ACTIVE_DAYS_BEFORE_PROMPT = 2;

function getTodayDateString(): string {
  return new Date().toISOString().split('T')[0]; // YYYY-MM-DD
}

export function GitHubStarBanner() {
  const { initialDataReceived, state } = useWebSocket();

  // Closing the prompt is permanent; this is gratitude, not a workflow.
  const [dismissed, setDismissed] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.GITHUB_STAR_DISMISSED,
    false,
  );

  // A rendered prompt counts as its single lifetime appearance even if the
  // browser closes before the user chooses an action.
  const [promptShown, setPromptShown] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.GITHUB_STAR_PROMPT_SHOWN,
    false,
  );

  const [activeDays, setActiveDays] = createLocalStorageNumberSignal(
    STORAGE_KEYS.GITHUB_STAR_ACTIVE_DAYS,
    0,
  );

  const [lastActiveDate, setLastActiveDate] = createLocalStorageStringSignal(
    STORAGE_KEYS.GITHUB_STAR_LAST_ACTIVE_DATE,
    '',
  );

  const [showPrompt, setShowPrompt] = createSignal(false);
  let blockedByDialogThisSession = false;

  // The first connected day establishes use. On the first return day, offer
  // one quiet prompt only when the rest of the app shell has yielded the session.
  createEffect(() => {
    if (dismissed()) {
      setShowPrompt(false);
      return;
    }

    if (dialogStackHasBlockingDialog()) {
      blockedByDialogThisSession = true;
      setShowPrompt(false);
      return;
    }

    if (blockedByDialogThisSession) return;
    if (promptShown()) return;

    if (!initialDataReceived()) {
      setShowPrompt(false);
      return;
    }

    const hasInfrastructure = (state.resources || []).length > 0;

    if (!hasInfrastructure) {
      setShowPrompt(false);
      return;
    }

    const today = getTodayDateString();
    if (lastActiveDate() !== today) {
      setLastActiveDate(today);
      setActiveDays(Math.min(activeDays() + 1, ACTIVE_DAYS_BEFORE_PROMPT));
      return;
    }

    if (activeDays() < ACTIVE_DAYS_BEFORE_PROMPT) return;
    if (!reserveLowPriorityNoticeSession('github-star')) return;

    setShowPrompt(true);
    setPromptShown(true);
  });

  const handleDismiss = () => {
    setDismissed(true);
    setShowPrompt(false);
  };

  const handleStarClick = () => {
    window.open(GITHUB_REPO_URL, '_blank', 'noopener,noreferrer');
    // Auto-dismiss - trust that they starred
    setDismissed(true);
    setShowPrompt(false);
  };

  return (
    <Show when={showPrompt()}>
      <section
        class="fixed left-4 right-20 bottom-(--pulse-mobile-nav-height) mb-4 z-30 max-w-sm overflow-hidden rounded-xl border border-blue-200 bg-surface text-base-content shadow-xl dark:border-blue-800 md:right-auto"
        aria-labelledby="github-star-title"
        aria-live="polite"
      >
        <div
          class="h-1 bg-linear-to-r from-blue-500 via-violet-500 to-amber-400"
          aria-hidden="true"
        />
        <div class="bg-linear-to-br from-blue-50 via-surface to-surface p-4 dark:from-blue-950/60">
          <div class="flex items-start justify-between gap-3">
            <div
              class="relative mb-4 flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-blue-100 text-blue-700 dark:bg-blue-900/60 dark:text-blue-200"
              aria-hidden="true"
            >
              <GithubIcon class="h-6 w-6" />
              <div class="absolute -right-2 -top-1 flex h-6 w-6 items-center justify-center rounded-full bg-amber-300 text-amber-900 shadow-xs">
                <StarIcon class="h-3.5 w-3.5 fill-current" />
              </div>
            </div>
            <ActionIconButton
              onClick={handleDismiss}
              label="Close and don't show again"
              title="Don't show again"
              tone="muted"
              size="sm"
              class="min-h-11 min-w-11 sm:min-h-0 sm:min-w-0"
              type="button"
            >
              <XIcon class="h-4 w-4" aria-hidden="true" />
            </ActionIconButton>
          </div>
          <h2 id="github-star-title" class="text-lg font-semibold text-base-content">
            Enjoying Pulse?
          </h2>
          <p class="mt-2 text-sm leading-6 text-muted">
            If Pulse is useful to you, a GitHub star helps others discover it and shows your support
            for the project.
          </p>
          <Button
            onClick={handleStarClick}
            variant="primary"
            size="md"
            class="mt-4 w-full gap-2"
            type="button"
          >
            <StarIcon class="h-4 w-4" aria-hidden="true" />
            Star on GitHub
          </Button>
        </div>
      </section>
    </Show>
  );
}
