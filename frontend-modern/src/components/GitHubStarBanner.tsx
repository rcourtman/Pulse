import { createSignal, createEffect, Show } from 'solid-js';
import { createLocalStorageBooleanSignal, createLocalStorageStringSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { useResources } from '@/hooks/useResources';
import GithubIcon from 'lucide-solid/icons/github';
import StarIcon from 'lucide-solid/icons/star';
import XIcon from 'lucide-solid/icons/x';

const GITHUB_REPO_URL = 'https://github.com/rcourtman/Pulse';

function getTodayDateString(): string {
  return new Date().toISOString().split('T')[0]; // YYYY-MM-DD
}

export function GitHubStarBanner() {
  const { resources } = useResources();

  // Track if user has dismissed the modal (permanent)
  const [dismissed, setDismissed] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.GITHUB_STAR_DISMISSED,
    false
  );

  // Track the first date user had infrastructure connected
  const [firstSeenDate, setFirstSeenDate] = createLocalStorageStringSignal(
    STORAGE_KEYS.GITHUB_STAR_FIRST_SEEN,
    ''
  );

  // Track snooze date (when "Maybe later" was clicked, don't show again until this date)
  const [snoozedUntil, setSnoozedUntil] = createLocalStorageStringSignal(
    STORAGE_KEYS.GITHUB_STAR_SNOOZED_UNTIL,
    ''
  );

  const [showModal, setShowModal] = createSignal(false);

  // Check if user qualifies to see the modal
  createEffect(() => {
    // Already dismissed? Don't show
    if (dismissed()) {
      setShowModal(false);
      return;
    }

    // Check if user has connected infrastructure
    const hasInfrastructure = resources().length > 0;

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

  return (
    <Show when={showModal()}>
      <div class="fixed inset-0 bg-black flex items-center justify-center z-50 p-4">
        <div class="bg-surface rounded-md shadow-sm max-w-md w-full overflow-hidden">
          {/* Header with close button */}
          <div class="flex justify-end p-3 pb-0">
            <button
              onClick={handleDismiss}
              class="p-1.5 hover:bg-surface-hover rounded-md text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
              title="Don't show again"
              aria-label="Close and don't show again"
            >
              <XIcon class="w-5 h-5" />
            </button>
          </div>

          {/* Content */}
          <div class="px-6 pb-6 text-center">
            {/* Icon */}
            <div class="flex justify-center mb-4">
              <div class="relative">
                <div class="w-16 h-16 bg-surface-hover rounded-full flex items-center justify-center">
                  <GithubIcon class="w-8 h-8 text-base-content" />
                </div>
                <div class="absolute -top-1 -right-1 w-7 h-7 bg-yellow-400 rounded-full flex items-center justify-center shadow-sm">
                  <StarIcon class="w-4 h-4 text-yellow-800" />
                </div>
              </div>
            </div>

            {/* Text */}
            <h2 class="text-xl font-semibold text-base-content mb-2">
              Enjoying Pulse?
            </h2>
            <p class="text-muted mb-6 leading-relaxed">
              Pulse is built and maintained by an independent developer. If it's been useful for monitoring your infrastructure, a GitHub star helps more than you'd think.
            </p>

            {/* Buttons */}
            <div class="flex flex-col gap-3">
              <button
                onClick={handleStarClick}
                class="w-full inline-flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-base hover:bg-slate-800 dark:text-slate-900 dark:hover:bg-white rounded-md transition-colors"
              >
                <StarIcon class="w-4 h-4" />
                Star on GitHub
              </button>
              <button
                onClick={handleMaybeLater}
                class="w-full px-4 py-2 text-sm text-muted hover:text-slate-700 dark:hover:text-slate-300 transition-colors"
              >
                Maybe later
              </button>
            </div>
          </div>
        </div>
      </div>
    </Show>
  );
}
