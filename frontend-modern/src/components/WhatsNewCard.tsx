import { Show, createEffect, createSignal } from 'solid-js';
import { updateStore } from '@/stores/updates';
import { UpdatesAPI } from '@/api/updates';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { buildReleaseNotesUrl, normalizeReleaseVersion } from '@/components/updateVersion';
import { extractHighlights, isReleaseVersion } from '@/components/whatsNewModel';
import { renderMarkdown } from '@/components/AI/aiChatUtils';
import { logger } from '@/utils/logger';

const readLastSeenVersion = (): string | null => {
  try {
    return localStorage.getItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN);
  } catch {
    return null;
  }
};

const markVersionSeen = (version: string) => {
  try {
    localStorage.setItem(STORAGE_KEYS.WHATS_NEW_LAST_SEEN, version);
  } catch {
    // Private mode / storage disabled: the banner simply won't persist state.
  }
};

/**
 * Post-update "What's New" banner. Shows once after the running version
 * changes, and only when that release has a curated `## Highlights` section
 * in its GitHub release notes. Dismissing (or a highlights-free release)
 * records the version so the banner stays quiet until the next update.
 */
export function WhatsNewCard() {
  const [visible, setVisible] = createSignal(false);
  const [version, setVersion] = createSignal('');
  const [highlightsHtml, setHighlightsHtml] = createSignal('');
  let checked = false;

  const loadNotes = async (currentVersion: string) => {
    try {
      const notes = await UpdatesAPI.getReleaseNotes();
      // Mid-update the backend can briefly disagree with the UI about the
      // running version; don't show notes for the wrong release.
      if (normalizeReleaseVersion(notes.version) !== currentVersion) {
        return;
      }
      const highlights = extractHighlights(notes.releaseNotes);
      if (!highlights) {
        markVersionSeen(currentVersion);
        return;
      }
      setHighlightsHtml(renderMarkdown(highlights));
      setVersion(currentVersion);
      setVisible(true);
    } catch (error) {
      if ((error as { status?: number }).status === 404) {
        // No published release for this build — stop asking.
        markVersionSeen(currentVersion);
        return;
      }
      // Transient failure: leave last-seen untouched so the next load retries.
      logger.warn("Failed to load release notes for What's New banner", error);
    }
  };

  createEffect(() => {
    const info = updateStore.versionInfo();
    if (!info || checked) return;
    checked = true;

    if (info.isDevelopment || info.isSourceBuild || !isReleaseVersion(info.version)) {
      return;
    }

    const currentVersion = normalizeReleaseVersion(info.version);
    if (!currentVersion) return;

    const lastSeen = readLastSeenVersion();
    if (!lastSeen) {
      // First run (fresh install or first load after this feature shipped):
      // record the baseline silently instead of greeting users with a banner.
      markVersionSeen(currentVersion);
      return;
    }
    if (normalizeReleaseVersion(lastSeen) === currentVersion) {
      return;
    }

    void loadNotes(currentVersion);
  });

  const dismiss = () => {
    markVersionSeen(version());
    setVisible(false);
  };

  return (
    <Show when={visible()}>
      <div
        data-testid="whats-new-banner"
        class="bg-emerald-50 dark:bg-emerald-900 border-b border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-100 relative animate-slideDown"
      >
        <div class="px-4 py-2">
          <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-3 min-w-0">
              {/* Sparkle icon */}
              <svg
                class="w-4 h-4 flex-shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9L12 3z"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path d="M19 15l.9 2.1L22 18l-2.1.9L19 21l-.9-2.1L16 18l2.1-.9L19 15z" />
              </svg>
              <span class="text-sm font-medium truncate">
                Pulse updated to v{version()} — here's what's new
              </span>
              <a
                href={buildReleaseNotesUrl(version())}
                target="_blank"
                rel="noopener noreferrer"
                class="text-emerald-600 dark:text-emerald-300 underline text-sm hidden sm:inline hover:text-emerald-700 dark:hover:text-emerald-200 flex-shrink-0"
              >
                Full release notes →
              </a>
            </div>
            <button
              onClick={dismiss}
              class="px-3 py-1 text-sm font-medium text-white bg-emerald-600 hover:bg-emerald-700 rounded transition-colors flex-shrink-0"
            >
              Got it
            </button>
          </div>
          <div
            class="text-sm mt-1 pl-7 pr-2 max-h-48 overflow-y-auto [&_ul]:list-disc [&_ul]:pl-4 [&_ol]:list-decimal [&_ol]:pl-4 [&_li]:mt-0.5 [&_p]:mt-1 [&_a]:underline [&_code]:font-mono [&_code]:text-xs"
            // eslint-disable-next-line solid/no-innerhtml -- renderMarkdown sanitizes via DOMPurify
            innerHTML={highlightsHtml()}
          />
        </div>
      </div>
    </Show>
  );
}
