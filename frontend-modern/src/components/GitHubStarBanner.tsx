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
const ACTIVE_DAYS_BEFORE_PROMPT = 14;

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

  // Count distinct days with connected infrastructure, then offer one quiet
  // prompt only when the rest of the app shell has yielded the session.
  createEffect(() => {
    if (dismissed()) {
      setShowPrompt(false);
      return;
    }

    if (promptShown()) return;

    if (dialogStackHasBlockingDialog()) {
      blockedByDialogThisSession = true;
      setShowPrompt(false);
      return;
    }

    if (blockedByDialogThisSession) return;

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
        class="fixed left-4 right-20 bottom-[var(--pulse-mobile-nav-height)] z-30 max-w-sm overflow-hidden rounded-lg border border-border bg-surface text-base-content shadow-lg md:right-auto md:bottom-4"
        aria-labelledby="github-star-title"
        aria-live="polite"
      >
        <div class="flex items-start gap-3 p-3">
          <div
            class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-surface-hover"
            aria-hidden="true"
          >
            <GithubIcon class="h-4 w-4 text-base-content" />
          </div>

          <div class="min-w-0 flex-1">
            <div class="flex items-start gap-2">
              <div class="min-w-0 flex-1">
                <h2 id="github-star-title" class="text-sm font-semibold text-base-content">
                  Finding Pulse useful?
                </h2>
                <p class="mt-1 text-xs leading-5 text-muted">
                  Starring the project on GitHub helps others discover it.
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
            <div class="mt-2 flex flex-wrap gap-2">
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
            </div>
          </div>
        </div>
      </section>
    </Show>
  );
}
